/*
Copyright 2017 Gravitational, Inc.

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

package roundtrip

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// NewTracer specifies function that creates request tracer
type NewTracer func() RequestTracer

// RequestTracer traces request parameters
// and is used for debugging
type RequestTracer interface {
	// Start is called on start of a request
	Start(r *http.Request)
	// Done is called on a completed request
	Done(re *Response, err error) (*Response, error)
}

// NopTracer is a no-op tracer
type NopTracer struct {
}

// Start is called on start of a request
func (*NopTracer) Start(r *http.Request) {
}

// Done is called on a completed request
func (NopTracer) Done(re *Response, err error) (*Response, error) {
	return re, err
}

var nopTracer = &NopTracer{}

// NewNopTracer is a function that returns no-op tracer
// every time when called
func NewNopTracer() RequestTracer {
	return nopTracer
}

// NewWriterTracer is a tracer that writes results to io.Writer
func NewWriterTracer(w io.Writer) *WriterTracer {
	return &WriterTracer{
		Writer: w,
	}
}

// Start is called on start of a request
func (t *WriterTracer) Start(r *http.Request) {
	t.StartTime = time.Now().UTC()
	t.Request.URL = r.URL.String()
	t.Request.Method = r.Method
}

// Done is called on a completed request
func (t *WriterTracer) Done(re *Response, err error) (*Response, error) {
	t.EndTime = time.Now().UTC()
	if err != nil {
		fmt.Fprintf(t, "[TRACE] %v %v %v -> ERR: %v", t.EndTime.Sub(t.StartTime), t.Request.Method, t.Request.URL, err)
		return re, err
	}
	fmt.Fprintf(t, "[TRACE] %v %v %v -> STATUS %v", t.EndTime.Sub(t.StartTime), t.Request.Method, t.Request.URL, re.Code())
	return re, err
}

// WriterTracer is a tracer using Writer to output info
type WriterTracer struct {
	// Writer is io.Writer
	io.Writer
	// StartTime is a start time of a request
	StartTime time.Time
	// EndTime is end time of a request
	EndTime time.Time
	// Request contains information about request
	Request RequestInfo
	// ResponseState contains response status
	ResponseStatus string
	// ResponseError is all about response error
	ResponseError error
}

// RequestInfo contains request information
type RequestInfo struct {
	// Method is request method
	Method string `json:"method"` // Method - request method
	// URL is request URL
	URL string `json:"url"` // URL - Request URL
}
