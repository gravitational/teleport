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

package web

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/utils"
)

const xForwardedForHeader = "X-Forwarded-For"

// NewXForwardedForMiddleware is an HTTP middleware that overwrites client
// source address if X-Forwarded-For is set.
//
// Both hijacked conn and request context are updated. The hijacked conn can be
// used for ALPN connection upgrades or Websocket connections.
func NewXForwardedForMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientSrcAddr, err := parseXForwardedForHeaders(r.RemoteAddr, r.Header.Values(xForwardedForHeader))
		switch {
		// Skip updating client source address if no X-Forwarded-For is
		// present. For example, the request may come from an internal
		// network or the load balancer itself.
		case trace.IsNotFound(err):
			next.ServeHTTP(w, r)

		// Reject the request on error.
		case err != nil:
			trace.WriteError(w, err)

		// Serve with updated client source address.
		default:
			next.ServeHTTP(
				responseWriterWithClientSrcAddr(r.Context(), w, clientSrcAddr),
				requestWithClientSrcAddr(r, clientSrcAddr),
			)
		}
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
		if strings.Contains(xForwardedForHeaders[0], ",") {
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
	ctx := authz.ContextWithClientSrcAddr(r.Context(), clientSrcAddr)
	r = r.WithContext(ctx)
	r.RemoteAddr = clientSrcAddr.String()
	return r
}

func responseWriterWithClientSrcAddr(ctx context.Context, w http.ResponseWriter, clientSrcAddr net.Addr) http.ResponseWriter {
	// Returns the original ResponseWriter if not a http.Hijacker.
	_, ok := w.(http.Hijacker)
	if !ok {
		slog.DebugContext(ctx, "Provided ResponseWriter is not a hijacker")
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
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, trace.BadParameter("provided ResponseWriter is not a hijacker")
	}
	conn, buffer, err := hijacker.Hijack()
	if err != nil {
		return conn, buffer, trace.Wrap(err)
	}

	return utils.NewConnWithSrcAddr(conn, r.remoteAddr), buffer, nil
}
