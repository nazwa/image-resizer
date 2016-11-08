// Simplification of willnorris.com/go/imageproxy
// Less options, forced longterm cache,
// local method copied directly to fix external scope access issues
package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/stvp/rollbar"
	"willnorris.com/go/imageproxy"
)

type ImageProxy struct {
	*imageproxy.Proxy
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

	req, err := imageproxy.NewRequest(r, p.DefaultBaseURL)
	if err != nil {
		msg := fmt.Sprintf("invalid request URL: %v", err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	// assign static settings from proxy to req.Options
	req.Options.ScaleUp = p.ScaleUp

	resp, err := p.Client.Get(req.String())
	if err != nil {
		msg := fmt.Sprintf("error fetching remote image: %v", err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	cached := resp.Header.Get(httpcache.XFromCache)
	logger.Infof("request: %v (served from cache: %v)", *req, cached == "1")

	// We want to override cache headers to force long term caching
	//	copyHeader(w, resp, "Cache-Control")
	w.Header().Add("Cache-Control", "public, max-age=31536000")

	copyHeader(w, resp, "Last-Modified")
	copyHeader(w, resp, "Expires")
	copyHeader(w, resp, "Etag")
	copyHeader(w, resp, "Link")

	if is304 := check304(r, resp); is304 {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	copyHeader(w, resp, "Content-Length")
	copyHeader(w, resp, "Content-Type")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// NewProxy constructs a new proxy.  The provided http RoundTripper will be
// used to fetch remote URLs.  If nil is provided, http.DefaultTransport will
// be used.
func NewProxy(transport http.RoundTripper, cache imageproxy.Cache) *ImageProxy {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if cache == nil {
		cache = imageproxy.NopCache
	}
	proxy := ImageProxy{
		Proxy: &imageproxy.Proxy{
			Cache: cache,
		},
	}

	client := new(http.Client)
	client.Transport = &httpcache.Transport{
		Transport:           &imageproxy.TransformingTransport{transport, client},
		Cache:               cache,
		MarkCachedResponses: true,
	}

	proxy.Client = client

	return &proxy
}

// check304 checks whether we should send a 304 Not Modified in response to
// req, based on the response resp.  This is determined using the last modified
// time and the entity tag of resp.
func check304(req *http.Request, resp *http.Response) bool {
	// TODO(willnorris): if-none-match header can be a comma separated list
	// of multiple tags to be matched, or the special value "*" which
	// matches all etags
	etag := resp.Header.Get("Etag")
	if etag != "" && etag == req.Header.Get("If-None-Match") {
		return true
	}

	lastModified, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		return false
	}
	ifModSince, err := time.Parse(time.RFC1123, req.Header.Get("If-Modified-Since"))
	if err != nil {
		return false
	}
	if lastModified.Before(ifModSince) {
		return true
	}

	return false
}

func copyHeader(w http.ResponseWriter, r *http.Response, header string) {
	key := http.CanonicalHeaderKey(header)
	if value, ok := r.Header[key]; ok {
		w.Header()[key] = value
	}
}
