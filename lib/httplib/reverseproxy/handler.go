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
