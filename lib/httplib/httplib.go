/*
Copyright 2015 Gravitational, Inc.

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

// Package httplib implements common utility functions for writing
// classic HTTP handlers
package httplib

import (
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// timeoutMessage is a generic "timeout" error message that is displayed as a more user-friendly alternative to
// the timeout errors returned by net/http
const timeoutMessage = "unable to complete the request due to a timeout, please try again in a few minutes"

// HandlerFunc specifies HTTP handler function that returns error
type HandlerFunc func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error)

// StdHandlerFunc specifies HTTP handler function that returns error
type StdHandlerFunc func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// ErrorWriter is a function responsible for writing the error into response
// body.
type ErrorWriter func(w http.ResponseWriter, err error)

// MakeHandler returns a new httprouter.Handle func from a handler func
func MakeHandler(fn HandlerFunc) httprouter.Handle {
	return MakeTracingHandler(MakeHandlerWithErrorWriter(fn, trace.WriteError))
}

// MakeTracingHandler returns a new httprouter.Handle func that wraps the provided handler func
// with one that will add a tracing span for each request.
func MakeTracingHandler(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		handler := otelhttp.NewHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			h(writer, request, p)
		}), r.RequestURI)

		handler.ServeHTTP(w, r)
	}
}

// MakeHandlerWithErrorWriter returns a httprouter.Handle from the HandlerFunc,
// and sends all errors to ErrorWriter.
func MakeHandlerWithErrorWriter(fn HandlerFunc, errWriter ErrorWriter) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// ensure that neither proxies nor browsers cache http traffic
		SetNoCacheHeaders(w.Header())

		out, err := fn(w, r, p)
		if err != nil {
			errWriter(w, err)
			return
		}
		if out != nil {
			roundtrip.ReplyJSON(w, http.StatusOK, out)
		}
	}
}

// MakeStdHandler returns a new http.Handle func from http.HandlerFunc
func MakeStdHandler(fn StdHandlerFunc) http.HandlerFunc {
	return MakeStdHandlerWithErrorWriter(fn, trace.WriteError)
}

// MakeStdHandlerWithErrorWriter returns a http.HandlerFunc from the
// StdHandlerFunc, and sends all errors to ErrorWriter.
func MakeStdHandlerWithErrorWriter(fn StdHandlerFunc, errWriter ErrorWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ensure that neither proxies nor browsers cache http traffic
		SetNoCacheHeaders(w.Header())

		out, err := fn(w, r)
		if err != nil {
			errWriter(w, err)
			return
		}
		if out != nil {
			roundtrip.ReplyJSON(w, http.StatusOK, out)
		}
	}
}

// WithCSRFProtection ensures that request to unauthenticated API is checked against CSRF attacks
func WithCSRFProtection(fn HandlerFunc) httprouter.Handle {
	hanlderFn := MakeHandler(fn)
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		err := csrf.VerifyHTTPHeader(r)
		if err != nil {
			log.Warningf("unable to validate CSRF token %v", err)
			trace.WriteError(w, trace.AccessDenied("access denied"))
			return
		}
		hanlderFn(w, r, p)
	}
}

// ReadJSON reads HTTP json request and unmarshals it
// into passed interface{} obj
func ReadJSON(r *http.Request, val interface{}) error {
	// Check content type to mitigate CSRF attack.
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		log.Warningf("Error parsing media type for reading JSON: %v", err)
		return trace.BadParameter("invalid request")
	}

	if contentType != "application/json" {
		log.Warningf("Invalid HTTP request header content-type %q for reading JSON", contentType)
		return trace.BadParameter("invalid request")
	}

	data, err := utils.ReadAtMost(r.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := json.Unmarshal(data, &val); err != nil {
		return trace.BadParameter("request: %v", err.Error())
	}
	return nil
}

// ConvertResponse converts http error to internal error type
// based on HTTP response code and HTTP body contents
func ConvertResponse(re *roundtrip.Response, err error) (*roundtrip.Response, error) {
	if err != nil {
		if uerr, ok := err.(*url.Error); ok && uerr != nil && uerr.Err != nil {
			return nil, trace.ConnectionProblem(uerr.Err, uerr.Error())
		}
		if nerr, ok := errors.Unwrap(err).(net.Error); ok && nerr.Timeout() {
			// Using `ConnectionProblem` instead of `LimitExceeded` allows us to preserve the original error
			// while adding a more user-friendly message.
			return nil, trace.ConnectionProblem(err, timeoutMessage)
		}
		return nil, trace.ConvertSystemError(err)
	}
	return re, trace.ReadError(re.Code(), re.Bytes())
}

// ParseBool will parse boolean variable from url query
// returns value, ok, error
func ParseBool(q url.Values, name string) (bool, bool, error) {
	stringVal := q.Get(name)
	if stringVal == "" {
		return false, false, nil
	}

	val, err := strconv.ParseBool(stringVal)
	if err != nil {
		return false, false, trace.BadParameter(
			"'%v': expected 'true' or 'false', got %v", name, stringVal)
	}
	return val, true, nil
}

// RewritePair is a rewrite expression
type RewritePair struct {
	// Expr is matching expression
	Expr *regexp.Regexp
	// Replacement is replacement
	Replacement string
}

// Rewrite creates a rewrite pair, panics if in epxression
// is not a valid regular expressoin
func Rewrite(in, out string) RewritePair {
	return RewritePair{
		Expr:        regexp.MustCompile(in),
		Replacement: out,
	}
}

// RewritePaths creates a middleware that rewrites paths in incoming request
func RewritePaths(next http.Handler, rewrites ...RewritePair) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, rewrite := range rewrites {
			req.URL.Path = rewrite.Expr.ReplaceAllString(req.URL.Path, rewrite.Replacement)
		}
		next.ServeHTTP(w, req)
	})
}

// SafeRedirect performs a relative redirect to the URI part of the provided redirect URL
func SafeRedirect(w http.ResponseWriter, r *http.Request, redirectURL string) error {
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		return trace.Wrap(err)
	}
	http.Redirect(w, r, parsedURL.RequestURI(), http.StatusFound)
	return nil
}
