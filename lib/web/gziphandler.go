/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// writerPool is a sync.Pool for shared gzip writers.
// each gzip writer allocates a lot of memory
// so it makes sense to reset the writer and reuse the
// internal buffers to avoid too many objects on the heap
var writerPool = sync.Pool{
	New: func() interface{} {
		gz := gzip.NewWriter(io.Discard)
		return gz
	},
}

func newGzipResponseWriter(w http.ResponseWriter) gzipResponseWriter {
	gz := writerPool.Get().(*gzip.Writer)
	gz.Reset(w)
	return gzipResponseWriter{gz: gz, ResponseWriter: w}
}

type gzipResponseWriter struct {
	gz *gzip.Writer
	http.ResponseWriter
}

// Write uses the Writer part of gzipResponseWriter to write the output.
func (w gzipResponseWriter) Write(b []byte) (int, error) {
	_, haveType := w.Header()["Content-Type"]
	// Explicitly set Content-Type if it has not been set previously
	if !haveType {
		// If no content type, apply sniffing algorithm to un-gzipped body.
		w.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return w.gz.Write(b)
}

// makeGzipHandler adds support for gzip compression for given handler.
func makeGzipHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the client can accept the gzip encoding and that this is not an image asset.
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || isCompressedImageRequest(r) {
			handler.ServeHTTP(w, r)
			return
		}

		// Set the HTTP header indicating encoding.
		w.Header().Set("Content-Encoding", "gzip")
		gzw := newGzipResponseWriter(w)
		defer gzw.gz.Close()
		handler.ServeHTTP(gzw, r)
	})
}

// isCompressedImageRequest checks whether a request is for a png or jpg/jpeg
func isCompressedImageRequest(r *http.Request) bool {
	return strings.HasSuffix(r.URL.Path, ".png") || strings.HasSuffix(r.URL.Path, ".jpg") || strings.HasSuffix(r.URL.Path, ".jpeg")
}
