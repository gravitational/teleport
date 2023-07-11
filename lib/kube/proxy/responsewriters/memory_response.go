// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package responsewriters

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
)

// NewMemoryResponseWriter creates a MemoryResponseWriter that satisfies
// the http.ResponseWriter interface and accumulates the response into a memory
// buffer for later decoding.
func NewMemoryResponseWriter() *MemoryResponseWriter {
	return &MemoryResponseWriter{
		header: make(http.Header),
		buf:    bytes.NewBuffer(make([]byte, 0, 1<<16)),
	}
}

// MemoryResponseWriter  satisfies the http.ResponseWriter interface and
// accumulates the response body and headers into a memory for later usage.
type MemoryResponseWriter struct {
	header http.Header
	buf    *bytes.Buffer
	status int
}

// Write appends b into the memory buffer.
func (f *MemoryResponseWriter) Write(b []byte) (int, error) {
	return f.buf.Write(b)
}

// Header returns the http.Header map.
func (f *MemoryResponseWriter) Header() http.Header {
	return f.header
}

// WriteHeader stores the response status code.
func (f *MemoryResponseWriter) WriteHeader(status int) {
	f.status = status
}

// Buffer exposes the memory buffer.
func (f *MemoryResponseWriter) Buffer() *bytes.Buffer {
	return f.buf
}

// Status returns the http response code.
func (f *MemoryResponseWriter) Status() int {
	// http.ResponseWriter implicitly sets StatusOK, if WriteHeader hasn't been
	// explicitly called.
	if f.status == 0 {
		return http.StatusOK
	}
	return f.status
}

// CopyInto copies the headers, response code and body into the provided response
// writer.
func (f *MemoryResponseWriter) CopyInto(dst http.ResponseWriter) error {
	defer func() {
		if flusher, ok := dst.(http.Flusher); ok {
			flusher.Flush()
		}
	}()
	b := f.buf.Bytes()
	copyHeader(dst.Header(), f.header, len(b))
	dst.WriteHeader(f.Status())

	_, err := dst.Write(b)
	return trace.Wrap(err)
}

// copyHeader copies every header execpt the "Content-Length" because the header
// includes the size of the response with the excluded resources.
// For the "Content-Length" header, we replace the value with the new body size.
func copyHeader(dst, src http.Header, contentLength int) {
	src.Set(reverseproxy.ContentLength, strconv.Itoa(contentLength))
	maps.Copy(dst, src)
}
