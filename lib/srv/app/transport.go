/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// transportConfig is configuration for a rewriting transport.
type transportConfig struct {
	uri                string
	publicAddr         string
	publicPort         string
	insecureSkipVerify bool
	cipherSuites       []uint16
	jwt                string
	rewrite            *services.Rewrite
	w                  events.StreamWriter
}

// Check validates configuration.
func (c *transportConfig) Check() error {
	if c.w == nil {
		return trace.BadParameter("stream writer missing")
	}
	if c.uri == "" {
		return trace.BadParameter("uri missing")
	}
	if c.publicAddr == "" {
		return trace.BadParameter("public addr missing")
	}
	if c.publicPort == "" {
		return trace.BadParameter("public port missing")
	}
	if c.jwt == "" {
		return trace.BadParameter("jwt missing")
	}

	return nil
}

// transport is a rewriting http.RoundTripper that can audit and forward
// requests to an internal application.
type transport struct {
	closeContext context.Context

	c *transportConfig

	tr http.RoundTripper

	uri *url.URL
}

// newTransport creates a new transport.
func newTransport(ctx context.Context, c *transportConfig) (*transport, error) {
	if err := c.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Parse the target address once then inject it into all requests.
	uri, err := url.Parse(c.uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Clone and configure the transport.
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tr.TLSClientConfig, err = configureTLS(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &transport{
		closeContext: ctx,
		c:            c,
		uri:          uri,
		tr:           tr,
	}, nil
}

// RoundTrip will rewrite the request, forward the request to the target
// application, emit an event to the audit log, then rewrite the response.
func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Perform any request rewriting needed before forwarding the request.
	if err := t.rewriteRequest(r); err != nil {
		return nil, trace.Wrap(err)
	}

	// Forward the request to the target application and emit an audit event.
	resp, err := t.tr.RoundTrip(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Emit the event to the audit log.
	if err := t.emitAuditEvent(r, resp); err != nil {
		return nil, trace.Wrap(err)
	}

	// Perform any response rewriting needed before returning the request.
	if err := t.rewriteResponse(resp); err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// rewriteRequest applies any rewriting rules to request before it's forwarded.
func (t *transport) rewriteRequest(r *http.Request) error {
	// Update the target address of the request so it's forwarded correctly.
	r.URL.Scheme = t.uri.Scheme
	r.URL.Host = t.uri.Host

	// Add in JWT header.
	r.Header.Add(teleport.AppJWTHeader, t.c.jwt)
	r.Header.Add(teleport.AppCFHeader, t.c.jwt)

	return nil
}

// rewriteResponse applies any rewriting rules to the response before returning it.
func (t *transport) rewriteResponse(resp *http.Response) error {
	switch {
	case t.c.rewrite != nil && len(t.c.rewrite.Redirect) > 0:
		err := t.rewriteRedirect(resp)
		if err != nil {
			return trace.Wrap(err)
		}
	default:
	}
	return nil
}

// rewriteRedirect applies redirect rules to the response.
func (t *transport) rewriteRedirect(resp *http.Response) error {
	if isRedirect(resp.StatusCode) {
		// Parse the "Location" header.
		u, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			return trace.Wrap(err)
		}

		// If the redirect location is one of the hosts specified in the list of
		// redirects, rewrite the header.
		if utils.SliceContainsStr(t.c.rewrite.Redirect, host(u.Host)) {
			u.Scheme = "https"
			u.Host = net.JoinHostPort(t.c.publicAddr, t.c.publicPort)
		}
		resp.Header.Set("Location", u.String())
	}
	return nil
}

// emitAuditEvent writes the request and response to audit stream.
func (t *transport) emitAuditEvent(req *http.Request, resp *http.Response) error {
	appSessionRequestEvent := &events.AppSessionRequest{
		Metadata: events.Metadata{
			Type: events.AppSessionRequestEvent,
			Code: events.AppSessionRequestCode,
		},
		StatusCode: uint32(resp.StatusCode),
		Path:       req.URL.Path,
		RawQuery:   req.URL.RawQuery,
	}
	if err := t.c.w.EmitAuditEvent(t.closeContext, appSessionRequestEvent); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// configureTLS creates and configures a *tls.Config will be used for
// mutual authentication.
func configureTLS(c *transportConfig) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(c.cipherSuites)

	// Don't verify the servers certificate if either Teleport was started with
	// the --insecure flag or insecure skip verify was specifically requested in
	// application config.
	tlsConfig.InsecureSkipVerify = (lib.IsInsecureDevMode() || c.insecureSkipVerify)

	return tlsConfig, nil
}

// host returns the host from a host:port string.
func host(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// isRedirect returns true if the status code is a 3xx code.
func isRedirect(code int) bool {
	if code >= http.StatusMultipleChoices && code <= http.StatusPermanentRedirect {
		return true
	}
	return false
}
