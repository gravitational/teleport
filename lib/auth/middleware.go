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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/metrics"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// AccessCacheWithEvents extends the [authclient.AccessCache] interface with [types.Events].
// Useful for trust-related components that need to watch for changes.
type AccessCacheWithEvents interface {
	authclient.AccessCache
	types.Events
}

// TLSServerConfig is a configuration for TLS server
type TLSServerConfig struct {
	// Listener is a listener to bind to
	Listener net.Listener
	// TLS is the server TLS configuration.
	TLS *tls.Config
	// GetClientCertificate returns auth client credentials.
	GetClientCertificate func() (*tls.Certificate, error)
	// API is API server configuration
	APIConfig
	// LimiterConfig is limiter config
	LimiterConfig limiter.Config
	// AccessPoint is a caching access point
	AccessPoint AccessCacheWithEvents
	// Component is used for debugging purposes
	Component string
	// AcceptedUsage restricts authentication
	// to a subset of certificates based on the metadata
	AcceptedUsage []string
	// ID is an optional debugging ID
	ID string
	// Metrics are optional TLSServer metrics
	Metrics *Metrics
}

// CheckAndSetDefaults checks and sets default values
func (c *TLSServerConfig) CheckAndSetDefaults() error {
	if err := c.APIConfig.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Listener == nil {
		return trace.BadParameter("missing parameter Listener")
	}
	if c.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	if c.GetClientCertificate == nil {
		return trace.BadParameter("missing parameter GetClientCertificate")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing parameter AccessPoint")
	}
	if c.Component == "" {
		c.Component = teleport.ComponentAuth
	}
	if c.Metrics == nil {
		c.Metrics = &Metrics{}
	}
	return nil
}

// Metrics handles optional metrics for TLSServerConfig
type Metrics struct {
	GRPCServerLatency bool
}

// TLSServer is TLS auth server
type TLSServer struct {
	// httpServer is HTTP/1.1 part of the server
	httpServer *http.Server
	// grpcServer is GRPC server
	grpcServer *GRPCServer
	// cfg is TLS server configuration used for auth server
	cfg TLSServerConfig
	// log is TLS server logging entry
	log *slog.Logger
	// mux is a listener that multiplexes HTTP/2 and HTTP/1.1
	// on different listeners
	mux *multiplexer.TLSListener
	// clientTLSConfigGenerator pre-generates and caches specialized per-cluster
	// client TLS configs.
	clientTLSConfigGenerator *ClientTLSConfigGenerator
}

// NewTLSServer returns new unstarted TLS server
func NewTLSServer(ctx context.Context, cfg TLSServerConfig) (*TLSServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// limiter limits requests by frequency and amount of simultaneous
	// connections per client
	limiter, err := limiter.NewLimiter(cfg.LimiterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sets up gRPC metrics interceptor
	grpcMetrics := metrics.CreateGRPCServerMetrics(cfg.Metrics.GRPCServerLatency, prometheus.Labels{teleport.TagServer: "teleport-auth"})
	err = metrics.RegisterPrometheusCollectors(grpcMetrics)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localClusterName, err := cfg.AccessPoint.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oldestSupportedVersion := teleport.MinClientSemVer()
	if os.Getenv("TELEPORT_UNSTABLE_ALLOW_OLD_CLIENTS") == "yes" {
		oldestSupportedVersion = nil
	}

	apiServer, err := NewAPIServer(&cfg.APIConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &Middleware{
		Middleware: authz.Middleware{
			ClusterName:   localClusterName.GetClusterName(),
			AcceptedUsage: cfg.AcceptedUsage,
			Handler:       apiServer,
		},
		Limiter:                limiter,
		GRPCMetrics:            grpcMetrics,
		OldestSupportedVersion: oldestSupportedVersion,
		AlertCreator: func(ctx context.Context, a types.ClusterAlert) error {
			return trace.Wrap(cfg.AuthServer.UpsertClusterAlert(ctx, a))
		},
	}

	// Wrap sets the next middleware in chain to the authMiddleware
	limiter.WrapHandle(authMiddleware)

	securityHeaderHandler := httplib.MakeSecurityHeaderHandler(limiter)
	tracingHandler := httplib.MakeTracingHandler(securityHeaderHandler, teleport.ComponentAuth)

	server := &TLSServer{
		cfg: cfg,
		httpServer: &http.Server{
			Handler:           tracingHandler,
			ReadTimeout:       apidefaults.DefaultIOTimeout,
			ReadHeaderTimeout: defaults.ReadHeadersTimeout,
			WriteTimeout:      apidefaults.DefaultIOTimeout,
			IdleTimeout:       apidefaults.DefaultIdleTimeout,
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				return authz.ContextWithConn(ctx, c)
			},
		},
		log: slog.With(teleport.ComponentKey, cfg.Component),
	}

	tlsConfig := cfg.TLS.Clone()
	// force client auth if given
	tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	tlsConfig.NextProtos = []string{http2.NextProtoTLS}

	server.clientTLSConfigGenerator, err = NewClientTLSConfigGenerator(ClientTLSConfigGeneratorConfig{
		TLS:                  tlsConfig,
		ClusterName:          localClusterName.GetClusterName(),
		PermitRemoteClusters: true,
		AccessPoint:          server.cfg.AccessPoint,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig.GetConfigForClient = server.clientTLSConfigGenerator.GetConfigForClient

	server.grpcServer, err = NewGRPCServer(GRPCServerConfig{
		TLS:                tlsConfig,
		Middleware:         authMiddleware,
		APIConfig:          cfg.APIConfig,
		UnaryInterceptors:  authMiddleware.UnaryInterceptors(),
		StreamInterceptors: authMiddleware.StreamInterceptors(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server.mux, err = multiplexer.NewTLSListener(multiplexer.TLSListenerConfig{
		Listener: tls.NewListener(cfg.Listener, tlsConfig),
		ID:       cfg.ID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.PluginRegistry != nil {
		if err := cfg.PluginRegistry.RegisterAuthServices(ctx, server.grpcServer, cfg.GetClientCertificate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return server, nil
}

// Close closes TLS server non-gracefully - terminates in flight connections
func (t *TLSServer) Close() error {
	errC := make(chan error, 2)
	go func() {
		errC <- t.httpServer.Close()
	}()
	go func() {
		t.grpcServer.server.Stop()
		errC <- nil
	}()
	errors := []error{}
	for range 2 {
		errors = append(errors, <-errC)
	}
	errors = append(errors, t.mux.Close())
	errors = append(errors, t.clientTLSConfigGenerator.Close())
	return trace.NewAggregate(errors...)
}

// Shutdown shuts down TLS server
func (t *TLSServer) Shutdown(ctx context.Context) error {
	errC := make(chan error, 2)
	go func() {
		errC <- t.httpServer.Shutdown(ctx)
	}()
	go func() {
		t.grpcServer.server.GracefulStop()
		errC <- nil
	}()
	errors := []error{}
	for range 2 {
		errors = append(errors, <-errC)
	}
	return trace.NewAggregate(errors...)
}

// Serve starts gRPC and HTTP1.1 services on the mux listener
func (t *TLSServer) Serve() error {
	errC := make(chan error, 2)
	go func() {
		err := t.mux.Serve()
		t.log.WarnContext(context.Background(), "Mux serve failed", "error", err)
	}()
	go func() {
		errC <- t.httpServer.Serve(t.mux.HTTP())
	}()
	go func() {
		errC <- t.grpcServer.server.Serve(t.mux.HTTP2())
	}()
	errors := []error{}
	for range 2 {
		errors = append(errors, <-errC)
	}
	return trace.NewAggregate(errors...)
}

// Middleware is authentication middleware checking every request
type Middleware struct {
	authz.Middleware
	// Limiter is a rate and connection limiter
	Limiter *limiter.Limiter
	// GRPCMetrics is the configured gRPC metrics for the interceptors
	GRPCMetrics *grpcprom.ServerMetrics

	// OldestSupportedVersion optionally allows the middleware to reject any connections
	// originated from a client that is using an unsupported version. If not set, then no
	// rejection occurs.
	OldestSupportedVersion *semver.Version
	// AlertCreator if provided is used to generate a cluster alert when any
	// unsupported connections are rejected.
	AlertCreator func(ctx context.Context, a types.ClusterAlert) error

	// lastRejectedAlertTime is the timestamp at which the last alert
	// was created in response to rejecting unsupported clients.
	lastRejectedAlertTime atomic.Int64
}

func getCustomRate(endpoint string) *limiter.RateSet {
	switch endpoint {
	// Account recovery RPCs.
	case
		"/proto.AuthService/ChangeUserAuthentication",
		"/proto.AuthService/ChangePassword",
		"/proto.AuthService/GetAccountRecoveryToken",
		"/proto.AuthService/StartAccountRecovery",
		"/proto.AuthService/VerifyAccountRecovery":
		rates := limiter.NewRateSet()
		// This limit means: 1 request per minute with bursts up to 10 requests.
		if err := rates.Add(time.Minute, 1, 10); err != nil {
			logger.DebugContext(context.Background(), "Failed to define a custom rate for rpc method, using default rate",
				"error", err,
				"rpc_method", endpoint)
			return nil
		}
		return rates
	// Passwordless RPCs (potential unauthenticated challenge generation).
	case "/proto.AuthService/CreateAuthenticateChallenge":
		const period = defaults.LimiterPeriod
		const average = defaults.LimiterAverage
		const burst = defaults.LimiterBurst
		rates := limiter.NewRateSet()
		if err := rates.Add(period, average, burst); err != nil {
			logger.DebugContext(context.Background(), "Failed to define a custom rate for rpc method, using default rate",
				"error", err,
				"rpc_method", endpoint,
			)
			return nil
		}
		return rates
	}
	return nil
}

// ValidateClientVersion inspects the client version for the connection and terminates
// the [IdentityInfo.Conn] if the client is unsupported. Requires the [Middleware.OldestSupportedVersion]
// to be configured before any connection rejection occurs.
func (a *Middleware) ValidateClientVersion(ctx context.Context, info IdentityInfo) error {
	if a.OldestSupportedVersion == nil {
		return nil
	}

	clientVersionString, versionExists := metadata.ClientVersionFromContext(ctx)
	if !versionExists {
		return nil
	}

	ua := metadata.UserAgentFromContext(ctx)

	logger := slog.With(
		"user_agent", ua,
		"identity", info.IdentityGetter.GetIdentity().Username,
		"version", clientVersionString,
		"addr", logutils.StringerAttr(info.Conn.RemoteAddr()),
	)
	clientVersion, err := semver.NewVersion(clientVersionString)
	if err != nil {
		logger.WarnContext(ctx, "Failed to determine client version", "error", err)
		a.displayRejectedClientAlert(ctx, clientVersionString, info.Conn.RemoteAddr(), ua, info.IdentityGetter)
		if err := info.Conn.Close(); err != nil {
			logger.WarnContext(ctx, "Failed to close client connection", "error", err)
		}

		return trace.AccessDenied("client version is unsupported")
	}

	if clientVersion.LessThan(*a.OldestSupportedVersion) {
		logger.InfoContext(ctx, "Terminating connection of client using unsupported version")
		a.displayRejectedClientAlert(ctx, clientVersionString, info.Conn.RemoteAddr(), ua, info.IdentityGetter)

		if err := info.Conn.Close(); err != nil {
			logger.WarnContext(ctx, "Failed to close client connection", "error", err)
		}

		return trace.AccessDenied("client version is unsupported")
	}

	return nil
}

var clientUARegex = regexp.MustCompile(`(tsh|tbot|tctl)\/\d+`)

// displayRejectedClientAlert creates an alert to notify admins that
// unsupported Teleport versions exist in the cluster and are explicitly
// being denied to prevent causing issues. Alerts are limited to being
// created once per day to reduce backend writes if there are a large
// number of unsupported clients constantly being rejected.
func (a *Middleware) displayRejectedClientAlert(ctx context.Context, clientVersion string, addr net.Addr, userAgent string, ident authz.IdentityGetter) {
	if a.AlertCreator == nil {
		return
	}

	now := time.Now()
	lastAlert := a.lastRejectedAlertTime.Load()
	then := time.Unix(0, lastAlert)
	if lastAlert != 0 && now.Before(then.Add(24*time.Hour)) {
		return
	}

	if !a.lastRejectedAlertTime.CompareAndSwap(lastAlert, now.UnixNano()) {
		return
	}

	alertVersion := semver.Version{
		Major: a.OldestSupportedVersion.Major,
		Minor: a.OldestSupportedVersion.Minor,
		Patch: a.OldestSupportedVersion.Patch,
	}

	match := clientUARegex.FindStringSubmatch(userAgent)
	i := ident.GetIdentity()
	br, builtin := ident.(authz.BuiltinRole)
	rbr, remoteBuiltin := ident.(authz.RemoteBuiltinRole)

	var message string
	switch {
	case len(match) > 1: // A match indicates the connection was from a client tool
		message = fmt.Sprintf("Connection from %s v%s by %s was rejected. Connections will be allowed after upgrading %s to v%s or newer", match[1], clientVersion, i.Username, match[1], alertVersion.String())
	case builtin: // If the identity is a builtin then this connection is from an agent
		message = fmt.Sprintf("Connection from %s %s at %s, running an unsupported version of v%s was rejected. Connections will be allowed after upgrading the agent to v%s or newer", br.AdditionalSystemRoles, i.Username, addr.String(), clientVersion, alertVersion.String())
	case remoteBuiltin: // If the identity is a remote builtin then this connection is from an agent, or leaf cluster
		message = fmt.Sprintf("Connection from %s %s at %s in cluster %s, running an unsupported version of v%s was rejected. Connections will be allowed after upgrading the agent to v%s or newer", rbr.Username, i.Username, addr.String(), rbr.ClusterName, clientVersion, alertVersion.String())
	default: // The connection is from an old client tool that does not provide a user agent.
		message = fmt.Sprintf("Connection from tsh, tctl, tbot, or a plugin running v%s by %s was rejected. Connections will be allowed after upgrading to v%s or newer", clientVersion, i.Username, alertVersion.String())
	}

	alert, err := types.NewClusterAlert(
		"rejected-unsupported-connection",
		message,
		types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindToken, types.VerbCreate)),
	)
	if err != nil {
		logger.WarnContext(ctx, "failed to create rejected-unsupported-connection alert", "error", err)
		return
	}

	if err := a.AlertCreator(ctx, alert); err != nil {
		logger.WarnContext(ctx, "failed to persist rejected-unsupported-connection alert", "error", err)
		return
	}
}

// withAuthenticatedUser returns a new context with the ContextUser field set to
// the caller's user identity as authenticated by their client TLS certificate.
func (a *Middleware) withAuthenticatedUser(ctx context.Context) (context.Context, error) {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.AccessDenied("missing authentication")
	}

	var (
		connState      *tls.ConnectionState
		identityGetter authz.IdentityGetter
	)

	switch info := peerInfo.AuthInfo.(type) {
	// IdentityInfo is provided if the grpc server is configured with the
	// TransportCredentials provided in this package.
	case IdentityInfo:
		connState = &info.TLSInfo.State
		identityGetter = info.IdentityGetter

		if err := a.ValidateClientVersion(ctx, info); err != nil {
			return nil, trace.Wrap(err)
		}
	// credentials.TLSInfo is provided if the grpc server is configured with
	// credentials.NewTLS.
	case credentials.TLSInfo:
		user, err := a.GetUser(ctx, info.State)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		connState = &info.State
		identityGetter = user
	default:
		return nil, trace.AccessDenied("missing authentication")
	}

	ctx = authz.ContextWithUserCertificate(ctx, certFromConnState(connState))
	ctx = authz.ContextWithClientSrcAddr(ctx, peerInfo.Addr)
	ctx = authz.ContextWithUser(ctx, identityGetter)

	return ctx, nil
}

func certFromConnState(state *tls.ConnectionState) *x509.Certificate {
	if state == nil || len(state.PeerCertificates) != 1 {
		return nil
	}
	return state.PeerCertificates[0]
}

// withAuthenticatedUserUnaryInterceptor is a gRPC unary server interceptor
// which sets the ContextUser field on the request context to the caller's user
// identity as authenticated by their client TLS certificate.
func (a *Middleware) withAuthenticatedUserUnaryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	ctx, err := a.withAuthenticatedUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return handler(ctx, req)
}

// withAuthenticatedUserUnaryInterceptor is a gRPC stream server interceptor
// which sets the ContextUser field on the request context to the caller's user
// identity as authenticated by their client TLS certificate.
func (a *Middleware) withAuthenticatedUserStreamInterceptor(srv any, serverStream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx, err := a.withAuthenticatedUser(serverStream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	return handler(srv, &authenticatedStream{ctx: ctx, ServerStream: serverStream})
}

// UnaryInterceptors returns the gRPC unary interceptor chain.
func (a *Middleware) UnaryInterceptors() []grpc.UnaryServerInterceptor {
	var is []grpc.UnaryServerInterceptor
	if a.GRPCMetrics != nil {
		is = append(is, a.GRPCMetrics.UnaryServerInterceptor())
	}

	return append(is,
		interceptors.GRPCServerUnaryErrorInterceptor,
		metadata.UnaryServerInterceptor,
		a.Limiter.UnaryServerInterceptorWithCustomRate(getCustomRate),
		a.withAuthenticatedUserUnaryInterceptor,
	)
}

// StreamInterceptors returns the gRPC stream interceptor chain.
func (a *Middleware) StreamInterceptors() []grpc.StreamServerInterceptor {
	var is []grpc.StreamServerInterceptor
	if a.GRPCMetrics != nil {
		is = append(is, a.GRPCMetrics.StreamServerInterceptor())
	}

	return append(is,
		interceptors.GRPCServerStreamErrorInterceptor,
		metadata.StreamServerInterceptor,
		a.Limiter.StreamServerInterceptor,
		a.withAuthenticatedUserStreamInterceptor,
	)
}

// authenticatedStream wraps around the embedded grpc.ServerStream
// provides new context with additional metadata
type authenticatedStream struct {
	ctx context.Context
	grpc.ServerStream
}

// Context specifies stream context with authentication metadata
func (a *authenticatedStream) Context() context.Context {
	return a.ctx
}

func findPrimarySystemRole(roles []string) *types.SystemRole {
	for _, role := range roles {
		systemRole := types.SystemRole(role)
		err := systemRole.Check()
		if err == nil {
			return &systemRole
		}
	}
	return nil
}
