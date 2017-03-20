package main

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go"
	"github.com/minio/minio-go/pkg/policy"
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

	var options Options

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

	options = ParseOptions(parts[1])

	reader, err := this.S3.GetObject(parts[0], parts[2])
	defer reader.Close()

	fileInfo, err := reader.Stat()
	//	if err != nil {
	//		logger.Error("here", err)
	//		c.AbortWithError(http.StatusInternalServerError, err)
	//		return
	//	}

	finalImage, err := Transform(reader, options)
	if err != nil {
		logger.Error("here 2", err)
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	buf := bytes.NewBuffer(finalImage)

	c.Header("Content-Type", fileInfo.ContentType)
	c.Header("Content-Length", strconv.FormatInt(int64(buf.Len()), 10))
	c.Header("Last-Modified", fileInfo.LastModified.Format(http.TimeFormat))
	c.Status(200)
	io.Copy(c.Writer, buf)

	go func() {
		buf = bytes.NewBuffer(finalImage)
		_, err = this.S3.PutObject(parts[0], parts[1]+"/"+parts[2], buf, fileInfo.ContentType)
		if err != nil {
			return
		}

		err = this.S3.SetBucketPolicy(parts[0], parts[1]+"/"+parts[2], policy.BucketPolicyReadOnly)
		if err != nil {
			return
		}
	}()
}
