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

// NewTracer is a constructor function to create new instances of RequestTracer.
type NewTracer func() RequestTracer

// RequestTracer defines an interface to trace HTTP requests for debugging or collecting metrics.
//
// Here's an example tracing a request by wrapping a handler between tracer's Start/Done methods:
//
//  func Handler(args ...) (*Response, error) {
//	  req := NewRequestTracer()
//	  return req.Done(func(*Response, error) {
//		  // Handler implementation
//    })
//  }
type RequestTracer interface {
	// Start starts tracing the specified request and is usually called
	// before any request handling takes place.
	Start(r *http.Request)
	// Done is called to complete tracing of the request previously started with Start.
	// It is designed to match the result of calling RoundTrip API for convenience
	// and does not modify the response argument.
	Done(re *Response, err error) (*Response, error)
}

// NopTracer is a request tracer that does nothing
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

// WriteTracer is a request tracer that outputs collected stats
// into the specified io.Writer
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
	Method string `json:"method"`
	// URL is request URL
	URL string `json:"url"`
}
