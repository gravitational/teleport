package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

// Write uses the Writer part of gzipResponseWriter to write the output.
func (w gzipResponseWriter) Write(b []byte) (int, error) {
	// Explicitly set Content-Type, otherwise the browser gets confused.
	if "" == w.Header().Get("Content-Type") {
		// If no content type, apply sniffing algorithm to un-gzipped body.
		w.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return w.Writer.Write(b)
}

// MakeGzipHandler adds support for gzip compression for given handler.
func MakeGzipHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the client can accept the gzip encoding and that this is not an image asset.
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || isCompressedImageRequest(r) {
			handler.ServeHTTP(w, r)
			return
		}

		// Set the HTTP header indicating encoding.
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		handler.ServeHTTP(gzw, r)
	})
}

// isCompressedImageRequest checks whether a request is for a png or jpeg
func isCompressedImageRequest(r *http.Request) bool {
	pathLen := len(r.URL.Path)

	// Path must be at least 5 characters long to be asking for a compressed image
	if pathLen < len("a.png") {
		return false
	}

	b := pathLen
	a := pathLen - len(".png") // or ".jpg"
	if r.URL.Path[a:b] == ".png" || r.URL.Path[a:b] == ".jpg" || r.URL.Path[a-1:b] == ".jpeg" {
		return true
	}

	return false
}
