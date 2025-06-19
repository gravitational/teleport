/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	New: func() any {
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
