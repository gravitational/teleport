/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// httpsConnAuthorizer authorizes incoming HTTPS tunnel connections.
type httpsConnAuthorizer interface {
	GetUser(ctx context.Context, connState tls.ConnectionState) (authz.IdentityGetter, error)
}

// HTTPSTunnelHandler handles connections from ALPN tunnel ProtocolAppHTTPS.
type HTTPSTunnelHandler struct {
	// next handles the HTTP connection after TLS (the inner HTTPS) termination,
	// which should be the Proxy web handler in regular setup. next can be
	// asynchronous and is responsible for closing the connection when
	// successful.
	next func(context.Context, net.Conn) error
	// tlsConfig is used to terminate HTTPS. Should be same as proxy web.
	tlsConfig atomic.Pointer[tls.Config]
	auth      httpsConnAuthorizer
	logger    *slog.Logger
}

// NewHTTPSTunnelHandler creates an alpnproxy.HandlerFunc for handling
// connections from ALPN tunnel ProtocolAppHTTPS.
//
// ┌─────────────────────────────────────────────────────────────────┐
// │ TLS-routing  (ALPN: teleport-app-https, Teleport client cert)   │
// │  ┌───────────────────────────────────────────────────────────┐  │
// │  │ HTTPS  (SNI: app.my-tenant.teleport.sh)                   │  │
// │  └───────────────────────────────────────────────────────────┘  │
// └─────────────────────────────────────────────────────────────────┘
//
// This handler extracts the Teleport identity from the outer layer then
// terminates TLS for the inner layer using the provided server certs (which
// should be the Proxy Web certs).
//
// Then the plain HTTP connection is passed to the "next" handler (which should
// be the Proxy Web handler which forwards the request to web app handler). Note
// that "next" can be asynchronous and is responsible for closing the connection
// if invoked.
func NewHTTPSTunnelHandler(next func(context.Context, net.Conn) error, clusterName string) *HTTPSTunnelHandler {
	return &HTTPSTunnelHandler{
		next: next,
		auth: &authz.Middleware{
			ClusterName:   clusterName,
			AcceptedUsage: []string{teleport.UsageAppsOnly},
		},
		logger: slog.With(teleport.ComponentKey, "alpn:app-https"),
	}
}

// SetTLSConfig sets the TLS configuration used to terminate HTTPS. This helper
// is required as the tls.Config may not be available when the handler is
// created.
func (h *HTTPSTunnelHandler) SetTLSConfig(c *tls.Config) {
	h.tlsConfig.Store(c)
}

// HandleConnection handles an incoming ALPN-tunneled HTTPS app connection.
func (h *HTTPSTunnelHandler) HandleConnection(ctx context.Context, conn net.Conn) (err error) {
	defer func() {
		// Do not forget to close the conn on error.
		if err != nil {
			if cerr := conn.Close(); cerr != nil && !utils.IsOKNetworkError(cerr) {
				h.logger.WarnContext(ctx, "Failed to close app HTTPS tunnel connection", "error", cerr)
			}
		}
	}()

	outerConnWithClientCert, ok := conn.(utils.TLSConn)
	if !ok {
		return trace.BadParameter("expected utils.TLSConn, got %T", conn)
	}

	if err := outerConnWithClientCert.HandshakeContext(ctx); err != nil {
		return trace.ConvertSystemError(err)
	}

	user, err := h.auth.GetUser(ctx, outerConnWithClientCert.ConnectionState())
	if err != nil {
		return trace.Wrap(err)
	}

	innerTLSConfig := h.tlsConfig.Load()
	if innerTLSConfig == nil {
		return trace.BadParameter("missing tls.Config")
	}

	authorizedConn := &httpsTunnelConn{
		TLSConn: outerConnWithClientCert,
		user:    user,
	}

	// Always pass tls.Conn to HTTP servers.
	return h.next(ctx, tls.Server(authorizedConn, innerTLSConfig))
}

type httpsTunnelConn struct {
	utils.TLSConn
	user authz.IdentityGetter
}

// IsHTTPSTunnelConn returns true if the HTTPS request is authorized from the
// client cert in the outer mTLS layer with ALPN ProtocolAppHTTPS.
func IsHTTPSTunnelConn(r *http.Request) bool {
	conn, err := authz.ConnFromContext(r.Context())
	if err != nil {
		return false
	}
	_, ok := findHTTPSTunnelConn(conn)
	return ok
}

func findHTTPSTunnelConn(conn net.Conn) (*httpsTunnelConn, bool) {
	// Cap unwrap depth to guard against cycles or deep wrapping.
	for range 10 {
		if c, ok := conn.(*httpsTunnelConn); ok {
			return c, true
		}
		unwrapper, ok := conn.(interface{ NetConn() net.Conn })
		if !ok {
			return nil, false
		}
		conn = unwrapper.NetConn()
	}
	return nil, false
}

func getIdentityFromHTTPSTunnelRequest(r *http.Request) (*tlsca.Identity, error) {
	conn, err := authz.ConnFromContext(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authorizedConn, ok := findHTTPSTunnelConn(conn)
	if !ok {
		return nil, trace.BadParameter("expected httpsTunnelConn, got %T", conn)
	}

	identity := authorizedConn.user.GetIdentity()
	if _, err := identity.GetRouteToApp(); err != nil {
		return nil, trace.AccessDenied("invalid app tunnel identity: %v", err)
	}
	return &identity, nil
}
