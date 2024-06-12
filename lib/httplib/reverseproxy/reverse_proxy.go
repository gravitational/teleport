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
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

// X-* Header names.
const (
	XForwardedProto  = "X-Forwarded-Proto"
	XForwardedFor    = "X-Forwarded-For"
	XForwardedHost   = "X-Forwarded-Host"
	XForwardedPort   = "X-Forwarded-Port"
	XForwardedServer = "X-Forwarded-Server"
	XRealIP          = "X-Real-Ip"
)

// XHeaders X-* headers.
var XHeaders = []string{
	XForwardedProto,
	XForwardedFor,
	XForwardedHost,
	XForwardedPort,
	XForwardedServer,
	XRealIP,
}

const (
	// ContentLength is the Content-Length header.
	ContentLength = "Content-Length"
)

// Forwarder is a reverse proxy that forwards http requests to another server.
type Forwarder struct {
	passHostHeader bool
	headerRewriter Rewriter
	*httputil.ReverseProxy
	log       logrus.FieldLogger
	transport http.RoundTripper
}

// New returns a new reverse proxy that forwards to the given url.
// If passHostHeader is true, the Host header will be copied from the
// request to the forwarded request. Otherwise, the Host header will be
// set to the host portion of the url.
func New(opts ...Option) (*Forwarder, error) {
	fwd := &Forwarder{
		headerRewriter: NewHeaderRewriter(),
		ReverseProxy: &httputil.ReverseProxy{
			ErrorHandler: DefaultHandler.ServeHTTP,
		},
		log: utils.NewLogger(),
	}
	// Apply options.
	for _, opt := range opts {
		opt(fwd)
	}

	// Rewrite is called by the ReverseProxy to modify the request.
	fwd.Rewrite = func(request *httputil.ProxyRequest) {
		modifyRequest(request.Out)
		if fwd.headerRewriter != nil {
			fwd.headerRewriter.Rewrite(request)
		}
		if !fwd.passHostHeader {
			request.Out.Host = request.Out.URL.Host
		}
	}

	if fwd.transport == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fwd.transport = tr
	}
	// Set the transport for the reverse proxy to use a round tripper
	// that logs the request and response.
	fwd.ReverseProxy.Transport = &roundTripperWithLogger{transport: fwd.transport, log: fwd.log}

	return fwd, nil
}

// Option is a functional option for the forwarder.
type Option func(*Forwarder)

// WithFlushInterval sets the flush interval for the forwarder.
func WithFlushInterval(interval time.Duration) Option {
	return func(rp *Forwarder) {
		rp.FlushInterval = interval
	}
}

// WithLogger sets the logger for the forwarder. It uses the logger.Writer()
// method to get the io.Writer to use for the stdlib logger.
func WithLogger(logger logrus.FieldLogger) Option {
	return func(rp *Forwarder) {
		rp.log = logger
	}
}

// WithRoundTripper sets the round tripper for the forwarder.
func WithRoundTripper(transport http.RoundTripper) Option {
	return func(rp *Forwarder) {
		rp.transport = transport
	}
}

// WithErrorHandler sets the error handler for the forwarder.
func WithErrorHandler(e ErrorHandlerFunc) Option {
	return func(rp *Forwarder) {
		rp.ErrorHandler = e
	}
}

// WithRewriter sets the header rewriter for the forwarder.
func WithRewriter(h Rewriter) Option {
	return func(rp *Forwarder) {
		rp.headerRewriter = h
	}
}

// WithPassHostHeader sets whether the Host header should be passed to the
// forwarded request.
func WithPassHostHeader() Option {
	return func(rp *Forwarder) {
		rp.passHostHeader = true
	}
}

// WithResponseModifier sets the response modifier for the forwarder.
func WithResponseModifier(m func(*http.Response) error) Option {
	return func(rp *Forwarder) {
		rp.ModifyResponse = m
	}
}

// Modify the request to handle the target URL.
func modifyRequest(outReq *http.Request) {
	u := getURLFromRequest(outReq)

	outReq.URL.Path = u.Path
	outReq.URL.RawPath = u.RawPath
	outReq.URL.RawQuery = u.RawQuery
	outReq.RequestURI = "" // Outgoing request should not have RequestURI

	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
}

// getURLFromRequest returns the URL from the request object. If the request
// RequestURI is non-empty and parsable, it will be used. Otherwise, the URL
// will be used.
func getURLFromRequest(req *http.Request) *url.URL {
	// If the Request was created by Go via a real HTTP request,
	// RequestURI will contain the original query string.
	// If the Request was created in code,
	// RequestURI will be empty, and we will use the URL object instead
	u := req.URL
	if req.RequestURI != "" {
		parsedURL, err := url.ParseRequestURI(req.RequestURI)
		if err == nil {
			return parsedURL
		}
	}
	return u
}

type roundTripperWithLogger struct {
	log       logrus.FieldLogger
	transport http.RoundTripper
}

// CloseIdleConnections ensures idle connections of the wrapped
// [http.RoundTripper] are closed.
func (r *roundTripperWithLogger) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := r.transport.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

// RoundTrip forwards the request on to the provided http.RoundTripper and logs
// the request and response.
func (r *roundTripperWithLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	rsp, err := r.transport.RoundTrip(req)
	if err != nil {
		r.log.Errorf("Error forwarding to %v, err: %v", req.URL, err)
		return rsp, err
	}

	if req.TLS != nil {
		r.log.Infof("Round trip: %v %v, code: %v, duration: %v tls:version: %x, tls:resume:%t, tls:csuite:%x, tls:server:%v",
			req.Method, req.URL, rsp.StatusCode, time.Now().UTC().Sub(start),
			req.TLS.Version,
			req.TLS.DidResume,
			req.TLS.CipherSuite,
			req.TLS.ServerName)
	} else {
		r.log.Infof("Round trip: %v %v, code: %v, duration: %v",
			req.Method, req.URL, rsp.StatusCode, time.Now().UTC().Sub(start))
	}

	return rsp, nil
}
