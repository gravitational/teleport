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
	"os"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// RouteFreeConfig holds what a RouteFreeHandler needs.
type RouteFreeConfig struct {
	// Next hands a connection to the Proxy's web listener, which serves every
	// web connection.
	Next func(ctx context.Context, conn net.Conn) error
	// ClusterName is the name of the local cluster.
	ClusterName string
	// AuthorizerOpts build the authorizer for the identity resolution forwards
	// to each candidate app, checking that identity's locks and source IP
	// pinning. It ignores the global device_trust.mode toggle, as the
	// Application Service's authorizer does, since Auth issues App certificates
	// without checking it, so only roles can require a trusted device.
	AuthorizerOpts authz.AuthorizerOpts
	// ClusterGetter finds the local cluster's app servers.
	ClusterGetter reversetunnelclient.ClusterGetter
	// ProxyPublicAddrs are the Proxy's public addresses, under whose hosts
	// apps are addressed.
	ProxyPublicAddrs []utils.NetAddr
	// ClusterFeatures reads the Proxy's cached cluster features. Their App
	// Access entitlement gates this path, as it gates the app sessions Auth
	// creates for route-bound certificates.
	ClusterFeatures func() proto.Features
}

// CheckAndSetDefaults checks that c holds everything a RouteFreeHandler needs.
func (c *RouteFreeConfig) CheckAndSetDefaults() error {
	if c.Next == nil {
		return trace.BadParameter("missing web listener")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if c.ClusterFeatures == nil {
		return trace.BadParameter("missing cluster features")
	}
	return nil
}

// RouteFreeHandler takes the Proxy's web connections ahead of the web
// listener, and decides what the connections whose client certificate does
// not include a RouteToApp may reach.
type RouteFreeHandler struct {
	// enabled is whether this Proxy serves route-free App connections at all.
	// We can remove this and the env var when we ship to production.
	enabled bool
	auth    *authz.Middleware
	// next hands a connection to the web listener.
	next            func(ctx context.Context, conn net.Conn) error
	resolver        *routeFreeResolver
	clusterFeatures func() proto.Features
}

// NewRouteFreeHandler returns a handler that passes every connection on to
// cfg.Next. It wraps none unless common.AppForwardedIdentityEnvVar is set.
//
// A Proxy builds the handler whether or not the env var is set, so it fails
// only on a config the Proxy could never have meant.
func NewRouteFreeHandler(cfg RouteFreeConfig) (*RouteFreeHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	resolver, err := newRouteFreeResolver(routeFreeResolverConfig{
		clusterName:      cfg.ClusterName,
		authorizerOpts:   cfg.AuthorizerOpts,
		clusterGetter:    cfg.ClusterGetter,
		proxyPublicAddrs: cfg.ProxyPublicAddrs,
		logger:           slog.With(teleport.ComponentKey, "alpn:app-route-free"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	enabled, _ := apiutils.ParseBool(os.Getenv(common.AppForwardedIdentityEnvVar))
	return &RouteFreeHandler{
		enabled: enabled,
		auth: &authz.Middleware{
			ClusterName:   cfg.ClusterName,
			AcceptedUsage: []string{teleport.UsageAppsOnly},
		},
		next:            cfg.Next,
		resolver:        resolver,
		clusterFeatures: cfg.ClusterFeatures,
	}, nil
}

// HandleConnection passes conn to the web listener, which serves every web
// connection so that all their requests pass the same middleware. A
// route-free connection goes in a wrapper that carries what the Proxy decided
// it may reach.
func (h *RouteFreeHandler) HandleConnection(ctx context.Context, conn net.Conn) error {
	if state, ok := h.accepts(ctx, conn); ok {
		conn = h.wrap(ctx, conn, state)
	}
	return trace.Wrap(h.next(ctx, conn))
}

// accepts reports whether conn is a route-free App connection. If it is,
// accepts returns its handshake state, which holds exactly one peer
// certificate, the client's, since GetUser rejects intermediaries.
func (h *RouteFreeHandler) accepts(ctx context.Context, conn net.Conn) (tls.ConnectionState, bool) {
	if !h.enabled {
		return tls.ConnectionState{}, false
	}
	tlsConn, ok := conn.(utils.TLSConn)
	if !ok {
		return tls.ConnectionState{}, false
	}
	state := tlsConn.ConnectionState()
	if !h.isRouteFree(ctx, state) {
		return tls.ConnectionState{}, false
	}
	return state, true
}

// wrap resolves the route-free connection conn to an app, and returns the
// connection the web listener serves for it. ctx holds the client's address,
// which source IP pinning checks.
func (h *RouteFreeHandler) wrap(ctx context.Context, conn net.Conn, state tls.ConnectionState) *resolvedConn {
	ctx, cancel := context.WithTimeout(ctx, routeFreeResolveTimeout)
	defer cancel()
	_, _, err := h.resolver.resolve(ctx, state.PeerCertificates[0], state.ServerName)
	if err == nil {
		// The entitlement is checked last, after every check that depends on
		// the client, so that an unlicensed cluster doesn't hide a denial the
		// client can act on.
		err = checkEntitlement(h.clusterFeatures())
	}
	if err == nil {
		err = trace.NotImplemented("resolved route-free App connections aren't served yet")
	}
	return &resolvedConn{Conn: conn, state: state, err: err}
}

// checkEntitlement fails if the cluster's license disables App Access. App
// Access is enabled on every license by default, so a missing entitlement
// deliberately fails open.
func checkEntitlement(features proto.Features) error {
	if entitlement, ok := features.GetEntitlements()[string(entitlements.App)]; ok && !entitlement.GetEnabled() {
		return trace.AccessDenied("this Teleport cluster is not licensed for application access, please contact the cluster administrator")
	}
	return nil
}

// isRouteFree reports whether state belongs to a route-free app connection
func (h *RouteFreeHandler) isRouteFree(ctx context.Context, state tls.ConnectionState) bool {
	if len(state.PeerCertificates) == 0 {
		return false
	}
	identity, err := tlsca.IdentityFromCert(state.PeerCertificates[0])
	if err != nil || identity.RouteToApp != (tlsca.RouteToApp{}) {
		return false
	}
	// Web/API dispatch serves certificates restricted to another service, such
	// as Access Graph and Git. Checking here also keeps GetUser from logging
	// each one as rejected.
	if len(identity.Usage) != 0 && !slices.Equal(identity.Usage, []string{teleport.UsageAppsOnly}) {
		return false
	}
	user, err := h.auth.GetUser(ctx, state)
	if err != nil {
		return false
	}
	// Built-in roles, agents and users of other clusters keep today's path.
	_, ok := user.(authz.LocalUser)
	return ok
}

// resolvedConn is a route-free connection as the web listener serves it, with
// what the Proxy decided when it took the connection. It is built once and
// never changed, and every request on the connection reads it.
type resolvedConn struct {
	net.Conn
	// state is the handshake state of the *tls.Conn underneath.
	state tls.ConnectionState
	// err is why the connection may reach no app.
	err error
}

// ConnectionState returns the connection's handshake state. net/http serves
// HTTP/2 only to a connection that has one, and sets each request's TLS from
// it, so requests on c look like any other with a client certificate.
func (c *resolvedConn) ConnectionState() tls.ConnectionState {
	return c.state
}

// NetConn returns the connection c wraps, as tls.Conn.NetConn does, so that
// the ingress metrics find the *tls.Conn.
func (c *resolvedConn) NetConn() net.Conn {
	return c.Conn
}

// CloseWrite half-closes the connection underneath, which net/http does before
// closing a connection whose request body it didn't read, so that the client
// reads the response before the connection resets.
func (c *resolvedConn) CloseWrite() error {
	closer, ok := findConn[interface{ CloseWrite() error }](c.Conn)
	if !ok {
		return trace.NotImplemented("%T can't half-close", c.Conn)
	}
	return trace.Wrap(closer.CloseWrite())
}

// resolvedConnFromRequest returns the route-free connection r arrived on, which
// the web server puts in every request's context.
func resolvedConnFromRequest(r *http.Request) (*resolvedConn, bool) {
	conn, err := authz.ConnFromContext(r.Context())
	if err != nil {
		return nil, false
	}
	return findConn[*resolvedConn](conn)
}

// findConn returns the first connection of type T in the chain of wrappers
// that conn heads, which it follows through NetConn. The depth cap guards
// against a wrapper that returns itself.
func findConn[T any](conn net.Conn) (T, bool) {
	for range 10 {
		if found, ok := conn.(T); ok {
			return found, true
		}
		unwrapper, ok := conn.(interface{ NetConn() net.Conn })
		if !ok {
			break
		}
		conn = unwrapper.NetConn()
	}
	var zero T
	return zero, false
}

// serveResolved answers a request that arrived on the route-free connection
// conn. A denied connection's first response closes it. For a response with
// Connection: close, the HTTP/2 server sends GOAWAY.
func serveResolved(w http.ResponseWriter, r *http.Request, conn *resolvedConn, logger *slog.Logger) {
	w.Header().Set("Connection", "close")
	code, ok := appauthz.WriteForwardedError(w, r, conn.err)
	if !ok {
		status := trace.ErrorToCode(conn.err)
		http.Error(w, http.StatusText(status), status)
	}
	logger.WarnContext(r.Context(), "Rejected a route-free App connection", "code", code, "error", conn.err)
}
