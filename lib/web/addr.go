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

package web

import (
	"bufio"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils"
)

// NewXForwardedForMiddleware is an HTTP middleware that overwrites client
// source address if X-Forwarded-For is set.
//
// Both hijacked conn and request context are updated. The hijacked conn can be
// used for ALPN connection upgrades or Websocket connections.
func NewXForwardedForMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientSrcAddr, err := parseXForwardedForHeaders(r.RemoteAddr, r.Header.Values("X-Forwarded-For"))
		if err != nil {
			// Skip updating client source address if no X-Forwarded-For is
			// present. For example, the request may come from an internal
			// network or the load balancer itself.
			if trace.IsNotFound(err) {
				next.ServeHTTP(w, r)
				return
			}

			trace.WriteError(w, err)
			return
		}

		next.ServeHTTP(
			responseWriterWithClientSrcAddr(w, clientSrcAddr),
			requestWithClientSrcAddr(r, clientSrcAddr),
		)
	})
}

// parseXForwardedForHeaders returns a net.Addr from provided values of X-Forwarded-For.
//
// MDN reference:
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
//
// AWS ALB reference:
// https://docs.aws.amazon.com/elasticloadbalancing/latest/application/x-forwarded-headers.html
func parseXForwardedForHeaders(observedAddr string, xForwardedForHeaders []string) (net.Addr, error) {
	switch len(xForwardedForHeaders) {
	case 0:
		return nil, trace.NotFound("no X-Forwarded-For headers")

	case 1:
		// Reject multiple IPs.
		if _, _, multipleIPs := strings.Cut(xForwardedForHeaders[0], ","); multipleIPs {
			return nil, trace.BadParameter("expect a single IP from X-Forwarded-For but got %v", xForwardedForHeaders)
		}

	default:
		// Reject multiple IPs.
		return nil, trace.BadParameter("expect a single IP from X-Forwarded-For but got %v", xForwardedForHeaders)
	}

	// If forwardedAddr has a port, use that.
	forwardedAddr := strings.TrimSpace(xForwardedForHeaders[0])
	if ipAddrPort, err := netip.ParseAddrPort(forwardedAddr); err == nil {
		return net.TCPAddrFromAddrPort(ipAddrPort), nil
	}

	// If forwardedAddr does not have a port, use port from observedAddr.
	ipAddr, err := netip.ParseAddr(forwardedAddr)
	if err != nil {
		return nil, trace.BadParameter("invalid X-Forwarded-For %v: %v", xForwardedForHeaders, err)
	}

	var port int
	if parsed, err := utils.ParseAddr(observedAddr); err == nil {
		port = parsed.Port(port)
	}

	return net.TCPAddrFromAddrPort(netip.AddrPortFrom(ipAddr, uint16(port))), nil
}

func requestWithClientSrcAddr(r *http.Request, clientSrcAddr net.Addr) *http.Request {
	ctx := utils.ClientSrcAddrContext(r.Context(), clientSrcAddr)
	r = r.WithContext(ctx)
	r.RemoteAddr = clientSrcAddr.String()
	return r
}

func responseWriterWithClientSrcAddr(w http.ResponseWriter, clientSrcAddr net.Addr) http.ResponseWriter {
	// Returns the original ResponseWriter if not a http.Hijacker.
	_, ok := w.(http.Hijacker)
	if !ok {
		logrus.Debug("Provided ResponseWriter is not a hijacker.")
		return w
	}

	return &responseWriterWithRemoteAddr{
		ResponseWriter: w,
		remoteAddr:     clientSrcAddr,
	}
}

// responseWriterWithRemoteAddr is a wrapper of provided http.ResponseWriter
// and overwrites Hijacker interface to return a net.Conn with provided
// remoteAddr.
type responseWriterWithRemoteAddr struct {
	http.ResponseWriter
	remoteAddr net.Addr
}

// Hijack returns a net.Conn with provided remoteAddr.
func (r *responseWriterWithRemoteAddr) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := r.ResponseWriter.(http.Hijacker).Hijack()
	if err != nil {
		return conn, buffer, trace.Wrap(err)
	}

	return utils.NewConnWithSrcAddr(conn, r.remoteAddr), buffer, nil
}
