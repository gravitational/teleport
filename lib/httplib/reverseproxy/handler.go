/*
Copyright 2023 Gravitational, Inc.

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

package reverseproxy

import (
	"io"
	"net"
	"net/http"
)

// ErrorHandler is an interface for handling errors.
type ErrorHandler interface {
	ServeHTTP(w http.ResponseWriter, req *http.Request, err error)
}

// DefaultHandler is the default error handler.
var DefaultHandler ErrorHandler = &defaultHandler{}

// defaultHandler is the default error handler.
type defaultHandler struct{}

// ServeHTTP writes the error message to the response.
func (e *defaultHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, err error) {
	statusCode := http.StatusInternalServerError
	switch {
	case err == io.EOF:
		statusCode = http.StatusBadGateway
	default:
		switch e, ok := err.(net.Error); {
		case ok && e.Timeout():
			statusCode = http.StatusGatewayTimeout
		case ok:
			statusCode = http.StatusBadGateway
		}
	}
	w.WriteHeader(statusCode)
	w.Write([]byte(http.StatusText(statusCode)))
}

// ErrorHandlerFunc is an adapter to allow the use of ordinary functions as
// error handlers. If f is a function with the appropriate signature,
// ErrorHandlerFunc(f) is a ErrorHandler that calls f.
type ErrorHandlerFunc func(http.ResponseWriter, *http.Request, error)

// ServeHTTP calls f(w, r).
func (f ErrorHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, err error) {
	f(w, r, err)
}
