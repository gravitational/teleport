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
	"context"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
	logger    *slog.Logger
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
			ErrorLog:     log.Default(),
		},
		logger: slog.Default(),
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
	fwd.ReverseProxy.Transport = &roundTripperWithLogger{transport: fwd.transport, logger: fwd.logger}

	return fwd, nil
}

// ServeHTTP implements the http.Handler interface for the Forwarder.
// It sets the ServerContextKey to nil to prevent the reverse proxy to panic
// when the request is served. The panic happens when the request is
// canceled by the client instead of the server, which is a common case
// when the reverse proxy is used to forward requests to long-running
// operations (e.g. kubernetes watch streams).
// https://cs.opensource.google/go/go/+/refs/tags/go1.24.4:src/net/http/httputil/reverseproxy.go;l=556-574;drc=e64f7ef03fdfa1c0d847c21b16c9302cc824e79b
// When the ServerContextKey is set to nil, the reverse proxy will not
// attempt to panic when the request is canceled, and will instead
// return. This allows any upstream logic to continue and clean up
// resources instead of having to handle the panic recovery. This
// is particularly important for Kubernetes Watch streams, where
// a substantial number of goroutines are spawned to handle
// the watch stream, and we want to clean them up gracefully
// instead leaving them hanging around because of a panic.
func (f *Forwarder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(
		context.WithValue(
			r.Context(),
			http.ServerContextKey,
			nil,
		),
	)
	f.ReverseProxy.ServeHTTP(w, r)
}

// Option is a functional option for the forwarder.
type Option func(*Forwarder)

// WithFlushInterval sets the flush interval for the forwarder.
func WithFlushInterval(interval time.Duration) Option {
	return func(rp *Forwarder) {
		rp.FlushInterval = interval
	}
}

// WithLogger sets the logger for the forwarder.
func WithLogger(logger *slog.Logger) Option {
	return func(rp *Forwarder) {
		rp.logger = logger
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
	logger    *slog.Logger
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
		r.logger.ErrorContext(req.Context(), "Error forwarding request",
			"method", req.Method,
			"url", logutils.StringerAttr(req.URL),
			"error", err,
		)
		return rsp, err
	}

	if req.TLS != nil {
		r.logger.InfoContext(req.Context(), "Round trip completed",
			slog.String("method", req.Method),
			slog.Any("url", logutils.StringerAttr(req.URL)),
			slog.Int("code", rsp.StatusCode),
			slog.Duration("duration", time.Now().UTC().Sub(start)),
			slog.Group("tls",
				"version", req.TLS.Version,
				"resume", req.TLS.DidResume,
				"csuite", req.TLS.CipherSuite,
				"server", req.TLS.ServerName,
			),
		)
	} else {
		r.logger.InfoContext(req.Context(), "Round trip completed",
			"method", req.Method,
			"url", logutils.StringerAttr(req.URL),
			"code", rsp.StatusCode,
			"duration", time.Now().UTC().Sub(start),
		)
	}

	return rsp, nil
}
