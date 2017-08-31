package main

import (
	"bytes"
	"errors"
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

	reader, err := this.S3.GetObject(parts[0], parts[2])
	defer reader.Close()

	fileInfo, err := reader.Stat()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	finalImage, err := imageproxy.Transform(body, options)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	buf := bytes.NewBuffer(finalImage)

	c.Header("Content-Type", fileInfo.ContentType)
	c.Header("Content-Length", strconv.FormatInt(int64(buf.Len()), 10))
	c.Header("Last-Modified", fileInfo.LastModified.Format(http.TimeFormat))
	c.Status(200)
	io.Copy(c.Writer, buf)

	go func(reqCopy *gin.Context) {
		objectName := parts[1] + "/" + parts[2]
		log.Printf("Upload started for %s/%s", parts[0], objectName)

		_, err := this.S3.StatObject(parts[0], objectName)

		if err != nil {
			errResp := err.(minio.ErrorResponse)
			if errResp.Code != "NoSuchKey" {
				// This is a legit error and log it
				errorText := errors.New(
					fmt.Sprintf(
						"Stat failed for %s/%s. %s: %s",
						parts[0], objectName, errResp.Code, errResp.Message,
					),
				)
				log.Printf(errorText.Error())
				rollbar.RequestError(rollbar.ERR, reqCopy.Request, errorText)
				return
			}
			// Otherwise carry on! 404 is good!
		}

		buf = bytes.NewBuffer(finalImage)
		_, err = this.S3.PutObject(parts[0], objectName, buf, fileInfo.ContentType)
		if err != nil {
			errorText := errors.New(
				fmt.Sprintf(
					"Upload failed for %s/%s with message: %s",
					parts[0], objectName, err,
				),
			)
			log.Printf(errorText.Error())
			rollbar.RequestError(rollbar.ERR, reqCopy.Request, errorText)
			return
		}

		log.Printf("Upload finished for %s/%s", parts[0], objectName)
	}(c.Copy())
}
