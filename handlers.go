package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/stvp/rollbar"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go"
	"willnorris.com/go/imageproxy"
)

type ResizeHandler struct {
	RouterGroup *gin.RouterGroup
	S3          *minio.Client
}

func NewResizeHandler(r *gin.RouterGroup, s3 *minio.Client) *ResizeHandler {
	u := &ResizeHandler{
		RouterGroup: r,
		S3:          s3,
	}
	u.Routes()
	return u
}

func (this *ResizeHandler) Routes() {
	this.RouterGroup.GET("/*path", this.Resize)
}

func (this *ResizeHandler) Resize(c *gin.Context) {
	if c.Request.URL.Path == "/favicon.ico" {
		return // ignore favicon requests
	}

	path := c.Param("path")[1:] // strip leading slash

	// first segment should be options
	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 3 {
		c.String(http.StatusBadRequest, "Too few path segments")
		return
	}

	// The url structure is super basic:
	// 1: bucket images.kerrygold.enecdn.io
	// 2: format 600x
	// 3: asset b8a99f3580/Capture0019-7489-Edit.jpg
	options := ParseOptions(parts[1])
	optionsString := options.String()

	fmt.Println(options)

	bucketName := parts[0]
	originalFileName := parts[2]
	resizedFilename := optionsString + "/" + parts[2]

	if cachedFile := this.getCachedFile(bucketName, resizedFilename); cachedFile != nil {
		fileInfo, err := cachedFile.Stat()
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.Header("Content-Type", fileInfo.ContentType)
		//		c.Header("Content-Length", strconv.FormatInt(int64(buf.Len()), 10))
		c.Header("Last-Modified", fileInfo.LastModified.Format(http.TimeFormat))
		c.Status(200)

		return
	}

	resizedFile, fileInfo := this.resizeFile(bucketName, originalFileName, options)

	buf := bytes.NewBuffer(resizedFile)

	c.Header("Content-Type", fileInfo.ContentType)
	c.Header("Content-Length", strconv.FormatInt(int64(buf.Len()), 10))
	c.Header("Last-Modified", fileInfo.LastModified.Format(http.TimeFormat))
	c.Status(200)
	io.Copy(c.Writer, buf)

	go this.uploadFile(bucketName, resizedFilename, fileInfo.ContentType, resizedFile)
}

func (this *ResizeHandler) getCachedFile(bucketName, fileName string) *minio.Object {
	reader, _ := this.S3.GetObject(bucketName, fileName, minio.GetObjectOptions{})

	return reader
}

func (this *ResizeHandler) resizeFile(bucketName, fileName string, options imageproxy.Options) ([]byte, *minio.ObjectInfo) {
	reader, err := this.S3.GetObject(bucketName, fileName, minio.GetObjectOptions{})
	defer reader.Close()

	fileInfo, err := reader.Stat()
	if err != nil {
		//		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, nil
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		//		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, nil
	}

	finalImage, err := imageproxy.Transform(body, options)
	if err != nil {
		//		c.AbortWithError(http.StatusInternalServerError, err)
		return nil, nil
	}

	return finalImage, &fileInfo
}

func (this *ResizeHandler) uploadFile(bucketName, fileName, contentType string, finalImage []byte) {
	log.Printf("Upload started for %s/%s", bucketName, fileName)

	_, err := this.S3.StatObject(bucketName, fileName, minio.StatObjectOptions{})

	if err != nil {
		errResp := err.(minio.ErrorResponse)
		if errResp.Code != "NoSuchKey" {
			// This is a legit error and log it
			errorText := fmt.Errorf(
				"Stat failed for %s/%s. %s: %s",
				bucketName, fileName, errResp.Code, errResp.Message,
			)
			log.Printf(errorText.Error())
			rollbar.Error(rollbar.ERR, errorText)

			return
		}
		// Otherwise carry on! 404 is good!
		// Means this file doesn't exist yet
	}

	//	buf := bytes.NewBuffer(finalImage)
	reader := bytes.NewReader(finalImage)
	_, err = this.S3.PutObject(bucketName, fileName, reader, int64(len(finalImage)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		errorText := fmt.Errorf(
			"Upload failed for %s/%s with message: %s",
			bucketName, fileName, err,
		)
		log.Printf(errorText.Error())
		rollbar.Error(rollbar.ERR, errorText)
		return
	}

	log.Printf("Upload finished for %s/%s", bucketName, fileName)
}
