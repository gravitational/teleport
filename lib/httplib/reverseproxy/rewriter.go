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
	"net"
	"net/http"
	"os"
	"strings"
)

// Rewriter is an interface for rewriting http requests.
type Rewriter interface {
	Rewrite(*http.Request)
}

// NewHeaderRewriter creates a new HeaderRewriter.
func NewHeaderRewriter() *HeaderRewriter {
	h, err := os.Hostname()
	if err != nil {
		h = "localhost"
	}
	return &HeaderRewriter{TrustForwardHeader: true, Hostname: h}
}

// HeaderRewriter re-sets the X-Forwarded-* headers and sets X-Real-IP header.
type HeaderRewriter struct {
	TrustForwardHeader bool
	Hostname           string
}

// Rewrite request headers.
func (rw *HeaderRewriter) Rewrite(req *http.Request) {
	if !rw.TrustForwardHeader {
		for _, h := range XHeaders {
			req.Header.Del(h)
		}
	}

	// Set X-Real-IP header if it is not set to the IP address of the client making the request.
	maybeSetXRealIP(req)

	// Set X-Forwarded-* headers if it is not set to the scheme of the request.
	maybeSetForwarded(req)

	if xfPort := req.Header.Get(XForwardedPort); xfPort == "" {
		req.Header.Set(XForwardedPort, forwardedPort(req))
	}

	if xfHost := req.Header.Get(XForwardedHost); xfHost == "" && req.Host != "" {
		req.Header.Set(XForwardedHost, req.Host)
	}

	if rw.Hostname != "" {
		req.Header.Set(XForwardedServer, rw.Hostname)
	}
}

// forwardedPort returns the port part of the Host header if present, otherwise,
// returns "80" if the scheme is http or "443" if the scheme is https or wss.
func forwardedPort(req *http.Request) string {
	if req == nil {
		return ""
	}

	if _, port, err := net.SplitHostPort(req.Host); err == nil && port != "" {
		return port
	}

	if req.Header.Get(XForwardedProto) == "https" || req.Header.Get(XForwardedProto) == "wss" {
		return "443"
	}

	if req.TLS != nil {
		return "443"
	}

	return "80"
}

// maybeSetXRealIP sets X-Real-IP header if it is not set to the IP address of
// the client making the request.
func maybeSetXRealIP(req *http.Request) {
	if req.Header.Get(XRealIP) != "" {
		return
	}
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		clientIP = ipv6fix(clientIP)
		req.Header.Set(XRealIP, clientIP)
	}
}

// maybeSetForwarded sets X-Forwarded-* headers if it is not set to the
// scheme of the request.
func maybeSetForwarded(req *http.Request) {
	// We need to delete the value because httputil.ReverseProxy
	// appends to the existing value.
	req.Header.Del(XForwardedFor)

	if req.Header.Get(XForwardedProto) != "" {
		return
	}

	if req.TLS != nil {
		req.Header.Set(XForwardedProto, "https")
	} else {
		req.Header.Set(XForwardedProto, "http")
	}
}

// clean up IP in case if it is ipv6 address and it has {zone} information in
// it, like "[fe80::d806:a55d:eb1b:49cc%vEthernet (vmxnet3 Ethernet Adapter - Virtual Switch)]:64692".
func ipv6fix(clientIP string) string {
	return strings.Split(clientIP, "%")[0]
}
