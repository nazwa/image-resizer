// Simplification of willnorris.com/go/imageproxy
// Less options, forced longterm cache,
// local method copied directly to fix external scope access issues
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/kennygrant/sanitize"
	"github.com/minio/minio-go"
	"github.com/stvp/rollbar"
	"willnorris.com/go/imageproxy"
)

type ImageProxy struct {
}

// ServeHTTP handles image requests.
func (p *ImageProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Errorf("Recovered from panic: %v \n", rec)
			rollbar.RequestError(rollbar.CRIT, r, fmt.Errorf("Recovered from panic: %v \n", rec))
		}
	}()

	if r.URL.Path == "/favicon.ico" {
		return // ignore favicon requests
	}

	if r.URL.Path == "/health-check" {
		fmt.Fprint(w, "OK")
		return
	}

	// Step 1 - Generate resized filename
	// Step 2 - Check if image exists based on etag / filename
	// Step 3 - If yes, serve
	// Step 4 - If no - transform
	// Step 5 - Serve
	// Step 6 - Upload to S3

	path := r.URL.Path[1:]

	options := imageproxy.ParseOptions(path)
	filename := sanitize.BaseName(path)
	bucketName := r.Host

	s3Client, err := minio.New("s3.amazonaws.com", config.Services.S3.AccessKey, config.Services.S3.SecretKey, true)
	if err != nil {
		log.Fatalln(err)
	}

	// Step 2 - check for cached image
	existingObjectReader, err := s3Client.GetObject(bucketName+"/_cache", filename)
	if err == nil {
		defer existingObjectReader.Close()

		w.Header().Set("Cache-Control", "public,max-age:912839172")
		_, err = io.Copy(w, existingObjectReader)
		if err != nil {
			log.Fatal(err)
		}
	}

	r.URL.Host = r.Host
	// Step 4 - Check for source image and transform it
	newR, e := imageproxy.NewRequest(r, nil)
	fmt.Println(newR, e, options)

	//	img, err := Transform(b, opt)
	//	if err != nil {
	//		glog.Errorf("error transforming image: %v", err)
	//		img = b
	//	}

	//	// replay response with transformed image and updated content length
	//	buf := new(bytes.Buffer)
	//	fmt.Fprintf(buf, "%s %s\n", resp.Proto, resp.Status)
	//	resp.Header.WriteSubset(buf, map[string]bool{"Content-Length": true})
	//	fmt.Fprintf(buf, "Content-Length: %d\n\n", len(img))
	//	buf.Write(img)

}

func copyHeader(w http.ResponseWriter, r *http.Response, header string) {
	key := http.CanonicalHeaderKey(header)
	if value, ok := r.Header[key]; ok {
		w.Header()[key] = value
	}
}
