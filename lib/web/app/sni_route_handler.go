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
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpguts"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// SNIRouteConfig configures an SNIRouteHandler.
type SNIRouteConfig struct {
	// Next serves every connection the handler passes on.
	Next func(ctx context.Context, conn net.Conn) error
	// ClusterName is the name of the local cluster.
	ClusterName string
	// AuthorizerOpts configure the authorizer that checks access to each
	// candidate app.
	AuthorizerOpts authz.AuthorizerOpts
	// ClusterGetter finds the local cluster's app servers.
	ClusterGetter reversetunnelclient.ClusterGetter
	// ProxyPublicAddrs are the Proxy's public addresses, which apps are
	// addressed under.
	ProxyPublicAddrs []utils.NetAddr
	// ClusterFeatures returns the cluster's features, whose App Access
	// entitlement the handler checks.
	ClusterFeatures func() proto.Features
	// ConnectionMonitor applies a connection's controls, such as locks and idle
	// timeouts.
	ConnectionMonitor ConnMonitor
	// MetricsRegistry is where the handler registers its metrics. When nil,
	// they are created but not registered.
	MetricsRegistry *metrics.Registry
}

// ConnMonitor applies a role's connection controls to a connection.
type ConnMonitor interface {
	MonitorConnScoped(ctx context.Context, sessionCtx *authz.ScopedSessionContext, conn net.Conn) (context.Context, net.Conn, error)
}

// CheckAndSetDefaults checks c and sets its defaults.
func (c *SNIRouteConfig) CheckAndSetDefaults() error {
	if c.Next == nil {
		return trace.BadParameter("missing Next")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}
	if c.ClusterFeatures == nil {
		return trace.BadParameter("missing cluster features")
	}
	if c.ConnectionMonitor == nil {
		return trace.BadParameter("missing connection monitor")
	}
	if c.MetricsRegistry == nil {
		c.MetricsRegistry = metrics.NoopRegistry()
	}
	return nil
}

// SNIRouteHandler resolves an App connection's route from its SNI when its
// certificate has no RouteToApp, and passes every connection on to next.
type SNIRouteHandler struct {
	// enabled is whether the handler resolves any routes.
	enabled         bool
	auth            *authz.Middleware
	next            func(ctx context.Context, conn net.Conn) error
	resolver        *routeResolver
	clusterFeatures func() proto.Features
	connMonitor     ConnMonitor
	metrics         *accessMetrics
}

// NewSNIRouteHandler returns a handler for cfg. It resolves no routes unless
// common.AppForwardedIdentityEnvVar is set.
func NewSNIRouteHandler(cfg SNIRouteConfig) (*SNIRouteHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	resolver, err := newRouteResolver(routeResolverConfig{
		clusterName:      cfg.ClusterName,
		authorizerOpts:   cfg.AuthorizerOpts,
		clusterGetter:    cfg.ClusterGetter,
		proxyPublicAddrs: cfg.ProxyPublicAddrs,
		logger:           slog.With(teleport.ComponentKey, "alpn:app-sni-route"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	m, err := newAccessMetrics(cfg.MetricsRegistry)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	enabled, _ := apiutils.ParseBool(os.Getenv(common.AppForwardedIdentityEnvVar))
	return &SNIRouteHandler{
		enabled: enabled,
		auth: &authz.Middleware{
			ClusterName:   cfg.ClusterName,
			AcceptedUsage: []string{teleport.UsageAppsOnly},
		},
		next:            cfg.Next,
		resolver:        resolver,
		clusterFeatures: cfg.ClusterFeatures,
		connMonitor:     cfg.ConnectionMonitor,
		metrics:         m,
	}, nil
}

// HandleConnection passes conn on to next. An SNI-route connection goes in a
// wrapper that carries what the Proxy decided it may reach.
func (h *SNIRouteHandler) HandleConnection(ctx context.Context, conn net.Conn) error {
	if state, ok := h.accepts(ctx, conn); ok {
		conn = h.wrap(ctx, conn, state)
	}
	if err := h.next(ctx, conn); err != nil {
		// Closing the wrapper also stops its connection monitor.
		conn.Close()
		return trace.Wrap(err)
	}
	return nil
}

// accepts reports whether the Proxy resolves conn's route from its SNI, and
// returns conn's handshake state if it does.
func (h *SNIRouteHandler) accepts(ctx context.Context, conn net.Conn) (tls.ConnectionState, bool) {
	if !h.enabled {
		return tls.ConnectionState{}, false
	}
	tlsConn, ok := conn.(utils.TLSConn)
	if !ok {
		return tls.ConnectionState{}, false
	}
	state := tlsConn.ConnectionState()
	if !h.takesSNIRoute(ctx, state) {
		return tls.ConnectionState{}, false
	}
	return state, true
}

// wrap admits conn and returns it wrapped with the result, recording the
// admission in the metrics.
func (h *SNIRouteHandler) wrap(ctx context.Context, conn net.Conn, state tls.ConnectionState) *resolvedConn {
	start := time.Now()
	identity, monitored, err := h.admit(ctx, conn, state)
	h.metrics.sniRouteAdmissionDuration.Observe(time.Since(start).Seconds())
	protocol := sniRouteProtocol(state)
	h.metrics.sniRouteConnections.WithLabelValues(sniRouteResult(err), protocol).Inc()
	if err != nil {
		return &resolvedConn{Conn: conn, state: state, err: err}
	}
	return &resolvedConn{
		Conn:     newGaugedConn(monitored, h.metrics.sniRouteActiveConnections.WithLabelValues(protocol)),
		state:    state,
		identity: identity,
	}
}

// admit resolves the connection's route and applies its connection controls.
// It returns the identity to forward and the monitored connection.
func (h *SNIRouteHandler) admit(ctx context.Context, conn net.Conn, state tls.ConnectionState) (authz.LocalUser, net.Conn, error) {
	resolved, err := h.resolve(ctx, state)
	if err != nil {
		return authz.LocalUser{}, nil, trace.Wrap(err)
	}
	monitored, err := h.monitor(ctx, conn, resolved.sessionCtx)
	if err != nil {
		return authz.LocalUser{}, nil, trace.Wrap(err)
	}
	return resolved.identity, monitored, nil
}

// resolve resolves the connection's route and checks the App Access
// entitlement.
func (h *SNIRouteHandler) resolve(ctx context.Context, state tls.ConnectionState) (*resolvedApp, error) {
	ctx, cancel := context.WithTimeout(ctx, routeResolveTimeout)
	defer cancel()
	resolved, err := h.resolver.resolve(ctx, state.PeerCertificates[0], state.ServerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Checked last, so that it doesn't hide a denial the client can act on.
	if err := checkEntitlement(h.clusterFeatures()); err != nil {
		return nil, trace.Wrap(err)
	}
	return resolved, nil
}

// monitor applies the connection controls of the role that granted access,
// and returns the connection to read through.
func (h *SNIRouteHandler) monitor(ctx context.Context, conn net.Conn, sessionCtx *authz.ScopedSessionContext) (net.Conn, error) {
	_, monitored, err := h.connMonitor.MonitorConnScoped(ctx, sessionCtx, conn)
	return monitored, trace.Wrap(err)
}

// checkEntitlement fails if the cluster's license disables App Access. A
// license missing the entitlement is allowed, since App Access is on by default.
func checkEntitlement(features proto.Features) error {
	if entitlement, ok := features.GetEntitlements()[string(entitlements.App)]; ok && !entitlement.GetEnabled() {
		return trace.AccessDenied("this Teleport cluster is not licensed for application access, please contact the cluster administrator")
	}
	return nil
}

// takesSNIRoute reports whether the Proxy resolves a connection's route from
// its SNI: when its certificate has no RouteToApp and belongs to a local user.
func (h *SNIRouteHandler) takesSNIRoute(ctx context.Context, state tls.ConnectionState) bool {
	if len(state.PeerCertificates) == 0 {
		return false
	}
	identity, err := tlsca.IdentityFromCert(state.PeerCertificates[0])
	if err != nil || identity.RouteToApp != (tlsca.RouteToApp{}) {
		return false
	}
	// A certificate restricted to another service isn't for an App connection.
	if len(identity.Usage) != 0 && !slices.Equal(identity.Usage, []string{teleport.UsageAppsOnly}) {
		return false
	}
	user, err := h.auth.GetUser(ctx, state)
	if err != nil {
		return false
	}
	_, ok := user.(authz.LocalUser)
	return ok
}

// resolvedConn is a connection on the SNI route, with the result of its
// admission. It doesn't change after it's built.
type resolvedConn struct {
	net.Conn
	state tls.ConnectionState
	// identity is the identity forwarded with each request. It's unset when
	// err is set.
	identity authz.LocalUser
	// err is why admission denied the connection.
	err error
}

// ConnectionState returns the connection's handshake state, which net/http
// needs to serve HTTP/2 and to set each request's TLS.
func (c *resolvedConn) ConnectionState() tls.ConnectionState {
	return c.state
}

// clientUser returns the username in the client's certificate.
func (c *resolvedConn) clientUser() string {
	if len(c.state.PeerCertificates) == 0 {
		return ""
	}
	return c.state.PeerCertificates[0].Subject.CommonName
}

// NetConn returns the connection c wraps.
func (c *resolvedConn) NetConn() net.Conn {
	return c.Conn
}

// CloseWrite half-closes the connection underneath.
func (c *resolvedConn) CloseWrite() error {
	closer, ok := findConn[interface{ CloseWrite() error }](c.Conn)
	if !ok {
		return trace.NotImplemented("%T can't half-close", c.Conn)
	}
	return trace.Wrap(closer.CloseWrite())
}

// resolvedConnFromRequest returns the SNI-route connection r arrived on, if
// any.
func resolvedConnFromRequest(r *http.Request) (*resolvedConn, bool) {
	conn, err := authz.ConnFromContext(r.Context())
	if err != nil {
		return nil, false
	}
	return findConn[*resolvedConn](conn)
}

// findConn returns the first connection of type T among conn and the
// connections it wraps. It gives up after 10, in case a wrapper returns itself.
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

// serveResolved forwards a request on the SNI-route connection conn to the
// connection's app, or refuses it.
func (h *Handler) serveResolved(w http.ResponseWriter, r *http.Request, conn *resolvedConn) {
	if conn.err != nil {
		h.metrics.countRequest(routeSNI, sniRouteResult(conn.err))
		w.Header().Set("Connection", "close")
		code, ok := appauthz.WriteForwardedError(w, r, conn.err)
		if !ok {
			status := trace.ErrorToCode(conn.err)
			http.Error(w, http.StatusText(status), status)
		}
		h.logger.WarnContext(r.Context(), "Rejected an SNI-route App connection", "code", code,
			"user", conn.clientUser(), "sni", conn.state.ServerName,
			"client_addr", conn.RemoteAddr(), "error", conn.err)
		return
	}
	// Checked before the authority, since a 421 would only send the client to a
	// new connection that refuses the request too.
	if r.Method == http.MethodConnect || (isUpgrade(r) && !upgradesOnlyToWebSocket(r)) {
		h.metrics.countRequest(routeSNI, refusedNotImplemented)
		http.Error(w, "This connection serves HTTP requests and WebSocket upgrades only.", http.StatusNotImplemented)
		h.logger.DebugContext(r.Context(), "Refused a request an SNI-route App connection doesn't serve",
			"method", r.Method, "upgrade", r.Header.Get("Upgrade"), "client_addr", conn.RemoteAddr())
		return
	}
	if !authorityMatches(r.Host, conn.state.ServerName, h.externalWebPorts) {
		h.metrics.countRequest(routeSNI, refusedMisdirected)
		// Only on HTTP/1. On HTTP/2 the header makes the server send GOAWAY and
		// shut the connection down, but only this stream is misdirected.
		if r.ProtoMajor == 1 {
			w.Header().Set("Connection", "close")
		}
		w.Header().Set(common.TeleportAppErrorCodeHeader, string(common.AppErrorCodeAuthorityMismatch))
		http.Error(w, "This connection serves another host. Open a new connection to the host this request names.", http.StatusMisdirectedRequest)
		h.logger.DebugContext(r.Context(), "Refused a misdirected request on an SNI-route App connection",
			"host", r.Host, "sni", conn.state.ServerName, "client_addr", conn.RemoteAddr())
		return
	}
	// Forward the address admission checked, which X-Forwarded-For may have
	// replaced in r.RemoteAddr.
	ctx := authz.ContextWithUser(r.Context(), conn.identity)
	ctx = authz.ContextWithClientSrcAddr(ctx, conn.RemoteAddr())
	h.agentForwarder.ServeHTTP(w, r.WithContext(ctx))
}

// isUpgrade reports whether r asks to switch protocols, as
// httputil.ReverseProxy decides it.
func isUpgrade(r *http.Request) bool {
	return httpguts.HeaderValuesContainsToken(r.Header["Connection"], "upgrade") && r.Header.Get("Upgrade") != ""
}

// upgradesOnlyToWebSocket reports whether WebSocket is the only protocol r
// offers, since the upstream may switch to any protocol it's offered.
func upgradesOnlyToWebSocket(r *http.Request) bool {
	upgrade := r.Header["Upgrade"]
	return len(upgrade) == 1 && strings.EqualFold(strings.TrimSpace(upgrade[0]), "websocket")
}

// authorityMatches reports whether hostport, a request's host, is sni with no
// port or one of ports. This keeps a client that reuses a connection for
// several hosts from sending another app's requests on it.
func authorityMatches(hostport, sni string, ports []string) bool {
	// No SNI is an IP literal, so the first colon starts the port.
	host, port, _ := strings.Cut(hostport, ":")
	want, ok := normalizeHostname(sni)
	if !ok || want == "" {
		return false
	}
	got, ok := normalizeHostname(host)
	if !ok || got != want {
		return false
	}
	return port == "" || slices.Contains(ports, port)
}

// externalWebPorts returns the ports of addrs, the Proxy's public addresses, or
// of webAddr when there are none.
func externalWebPorts(addrs []utils.NetAddr, webAddr utils.NetAddr) []string {
	var ports []string
	for _, addr := range addrs {
		if port := addr.Port(0); port != 0 {
			ports = append(ports, strconv.Itoa(port))
		}
	}
	if len(addrs) == 0 {
		if port := webAddr.Port(0); port != 0 {
			ports = append(ports, strconv.Itoa(port))
		}
	}
	return ports
}
