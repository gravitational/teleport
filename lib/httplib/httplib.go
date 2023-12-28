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

// Package httplib implements common utility functions for writing
// classic HTTP handlers
package httplib

import (
	"bufio"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	_ http.ResponseWriter = (*ResponseStatusRecorder)(nil)
	_ http.Flusher        = (*ResponseStatusRecorder)(nil)
	_ http.Hijacker       = (*ResponseStatusRecorder)(nil)
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
	return MakeHandlerWithErrorWriter(fn, trace.WriteError)
}

// MakeSecurityHeaderHandler returns a new httprouter.Handle func that wraps the provided handler func
// with one that will ensure the headers from SetDefaultSecurityHeaders are applied.
func MakeSecurityHeaderHandler(h http.Handler) http.Handler {
	handler := func(w http.ResponseWriter, r *http.Request) {
		SetDefaultSecurityHeaders(w.Header())

		h.ServeHTTP(w, r)
	}
	return http.HandlerFunc(handler)
}

// MakeTracingHandler returns a new httprouter.Handle func that wraps the provided handler func
// with one that will add a tracing span for each request.
func MakeTracingHandler(h http.Handler, component string) http.Handler {
	// Wrap the provided handler with one that will inject
	// any propagated tracing context provided via a query parameter
	// if there isn't already a header containing tracing context.
	// This is required for scenarios using web sockets as headers
	// cannot be modified to inject the tracing context.
	handler := func(w http.ResponseWriter, r *http.Request) {
		// ensure headers have priority over query parameters
		if r.Header.Get(tracing.TraceParent) != "" {
			h.ServeHTTP(w, r)
			return
		}

		traceParent := r.URL.Query()[tracing.TraceParent]
		if len(traceParent) > 0 {
			r.Header.Add(tracing.TraceParent, traceParent[0])
		}

		h.ServeHTTP(w, r)
	}

	return otelhttp.NewHandler(http.HandlerFunc(handler), component, otelhttp.WithSpanNameFormatter(tracehttp.HandlerFormatter))
}

// MakeTracingMiddleware returns an HTTP middleware that makes tracing
// handlers.
func MakeTracingMiddleware(component string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return MakeTracingHandler(next, component)
	}
}

// MakeHandlerWithErrorWriter returns a httprouter.Handle from the HandlerFunc,
// and sends all errors to ErrorWriter.
func MakeHandlerWithErrorWriter(fn HandlerFunc, errWriter ErrorWriter) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// ensure that neither proxies nor browsers cache http traffic
		SetNoCacheHeaders(w.Header())
		// ensure that default security headers are set
		SetDefaultSecurityHeaders(w.Header())

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

// MakeStdHandlerWithErrorWriter returns a http.HandlerFunc from the
// StdHandlerFunc, and sends all errors to ErrorWriter.
func MakeStdHandlerWithErrorWriter(fn StdHandlerFunc, errWriter ErrorWriter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// ensure that neither proxies nor browsers cache http traffic
		SetNoCacheHeaders(w.Header())
		// ensure that default security headers are set
		SetDefaultSecurityHeaders(w.Header())

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
	handlerFn := MakeHandler(fn)
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			errHeader := csrf.VerifyHTTPHeader(r)
			errForm := csrf.VerifyFormField(r)
			if errForm != nil && errHeader != nil {
				log.Warningf("unable to validate CSRF token: %v, %v", errHeader, errForm)
				trace.WriteError(w, trace.AccessDenied("access denied"))
				return
			}
		}
		handlerFn(w, r, p)
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
		var uErr *url.Error
		if errors.As(err, &uErr) && uErr.Err != nil {
			return nil, trace.ConnectionProblem(uErr.Err, "")
		}
		var nErr net.Error
		if errors.As(err, &nErr) && nErr.Timeout() {
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

// OriginLocalRedirectURI will take an incoming URL including optionally the host and scheme and return the URI
// associated with the URL.  Additionally, it will ensure that the URI does not include any techniques potentially
// used to redirect to a different origin.
func OriginLocalRedirectURI(redirectURL string) (string, error) {
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		return "", trace.Wrap(err)
	} else if parsedURL.IsAbs() && (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", trace.BadParameter("Invalid scheme: %s", parsedURL.Scheme)
	}

	resultURI := parsedURL.RequestURI()
	if strings.HasPrefix(resultURI, "//") {
		return "", trace.BadParameter("Invalid double slash redirect")
	} else if strings.Contains(resultURI, "@") {
		return "", trace.BadParameter("Basic Auth not allowed in redirect")
	}
	return resultURI, nil
}

// ResponseStatusRecorder is an http.ResponseWriter that records the response status code.
type ResponseStatusRecorder struct {
	http.ResponseWriter
	flusher http.Flusher
	status  int
}

// NewResponseStatusRecorder makes and returns a ResponseStatusRecorder.
func NewResponseStatusRecorder(w http.ResponseWriter) *ResponseStatusRecorder {
	rec := &ResponseStatusRecorder{ResponseWriter: w}
	if flusher, ok := w.(http.Flusher); ok {
		rec.flusher = flusher
	}
	return rec
}

// WriteHeader sends an HTTP response header with the provided
// status code and save the status code in the recorder.
func (r *ResponseStatusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush optionally flushes the inner ResponseWriter if it supports that.
// Otherwise, Flush is a noop.
//
// Flush is optionally used by github.com/gravitational/oxy/forward to flush
// pending data on streaming HTTP responses (like streaming pod logs).
//
// Without this, oxy/forward will handle streaming responses by accumulating
// ~32kb of response in a buffer before flushing it.
func (r *ResponseStatusRecorder) Flush() {
	if r.flusher != nil {
		r.flusher.Flush()
	}
}

// Status returns the recorded status after WriteHeader is called, or StatusOK if WriteHeader hasn't been called
// explicitly.
func (r *ResponseStatusRecorder) Status() int {
	// http.ResponseWriter implicitly sets StatusOK, if WriteHeader hasn't been
	// explicitly called.
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

// Hijack implements the http.Hijacker interface.
func (r *ResponseStatusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}
