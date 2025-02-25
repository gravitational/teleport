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
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sync/atomic"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// TeleportImpersonateUserHeader is a header that specifies teleport user identity
	// that the proxy is impersonating.
	TeleportImpersonateUserHeader = "Teleport-Impersonate-User"
	// TeleportImpersonateIPHeader is a header that specifies the real user IP address.
	TeleportImpersonateIPHeader = "Teleport-Impersonate-IP"
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

	localClusterName, err := cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oldestSupportedVersion := &teleport.MinClientSemVersion
	if os.Getenv("TELEPORT_UNSTABLE_ALLOW_OLD_CLIENTS") == "yes" {
		oldestSupportedVersion = nil
	}

	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &Middleware{
		ClusterName:            localClusterName.GetClusterName(),
		AcceptedUsage:          cfg.AcceptedUsage,
		Limiter:                limiter,
		GRPCMetrics:            grpcMetrics,
		OldestSupportedVersion: oldestSupportedVersion,
		AlertCreator: func(ctx context.Context, a types.ClusterAlert) error {
			return trace.Wrap(cfg.AuthServer.UpsertClusterAlert(ctx, a))
		},
	}

	apiServer, err := NewAPIServer(&cfg.APIConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authMiddleware.Wrap(apiServer)
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
	for i := 0; i < 2; i++ {
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
	for i := 0; i < 2; i++ {
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
	for i := 0; i < 2; i++ {
		errors = append(errors, <-errC)
	}
	return trace.NewAggregate(errors...)
}

// Middleware is authentication middleware checking every request
type Middleware struct {
	ClusterName string
	// Handler is HTTP handler called after the middleware checks requests
	Handler http.Handler
	// AcceptedUsage restricts authentication
	// to a subset of certificates based on certificate metadata,
	// for example middleware can reject certificates with mismatching usage.
	// If empty, will only accept certificates with non-limited usage,
	// if set, will accept certificates with non-limited usage,
	// and usage exactly matching the specified values.
	AcceptedUsage []string
	// Limiter is a rate and connection limiter
	Limiter *limiter.Limiter
	// GRPCMetrics is the configured gRPC metrics for the interceptors
	GRPCMetrics *grpcprom.ServerMetrics
	// EnableCredentialsForwarding allows the middleware to receive impersonation
	// identity from the client if it presents a valid proxy certificate.
	// This is used by the proxy to forward the identity of the user who
	// connected to the proxy to the next hop.
	EnableCredentialsForwarding bool
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

// Wrap sets next handler in chain
func (a *Middleware) Wrap(h http.Handler) {
	a.Handler = h
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
		user, err := a.GetUser(info.State)
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
func (a *Middleware) withAuthenticatedUserUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx, err := a.withAuthenticatedUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return handler(ctx, req)
}

// withAuthenticatedUserUnaryInterceptor is a gRPC stream server interceptor
// which sets the ContextUser field on the request context to the caller's user
// identity as authenticated by their client TLS certificate.
func (a *Middleware) withAuthenticatedUserStreamInterceptor(srv interface{}, serverStream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx, err := a.withAuthenticatedUser(serverStream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	return handler(srv, &authenticatedStream{ctx: ctx, ServerStream: serverStream})
}

// UnaryInterceptors returns the gRPC unary interceptor chain.
func (a *Middleware) UnaryInterceptors() []grpc.UnaryServerInterceptor {
	is := []grpc.UnaryServerInterceptor{
		//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
		// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
		otelgrpc.UnaryServerInterceptor(),
	}

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
	is := []grpc.StreamServerInterceptor{
		//nolint:staticcheck // SA1019. There is a data race in the stats.Handler that is replacing
		// the interceptor. See https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4576.
		otelgrpc.StreamServerInterceptor(),
	}

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

// GetUser returns authenticated user based on request TLS metadata
func (a *Middleware) GetUser(connState tls.ConnectionState) (authz.IdentityGetter, error) {
	peers := connState.PeerCertificates
	if len(peers) > 1 {
		// when turning intermediaries on, don't forget to verify
		// https://github.com/kubernetes/kubernetes/pull/34524/files#diff-2b283dde198c92424df5355f39544aa4R59
		return nil, trace.AccessDenied("access denied: intermediaries are not supported")
	}

	// with no client authentication in place, middleware
	// assumes not-privileged Nop role.
	// it theoretically possible to use bearer token auth even
	// for connections without auth, but this is not active use-case
	// therefore it is not allowed to reduce scope
	if len(peers) == 0 {
		return authz.BuiltinRole{
			Role:        types.RoleNop,
			Username:    string(types.RoleNop),
			ClusterName: a.ClusterName,
			Identity:    tlsca.Identity{},
		}, nil
	}
	clientCert := peers[0]

	identity, err := tlsca.FromSubject(clientCert.Subject, clientCert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Since 5.0, teleport TLS certs include the origin teleport cluster in the
	// subject (identity). Before 5.0, origin teleport cluster was inferred
	// from the cert issuer.
	certClusterName := identity.TeleportCluster
	if certClusterName == "" {
		certClusterName, err = tlsca.ClusterName(clientCert.Issuer)
		if err != nil {
			logger.WarnContext(context.Background(), "Failed to parse client certificate", "error", err)
			return nil, trace.AccessDenied("access denied: invalid client certificate")
		}
		identity.TeleportCluster = certClusterName
	}
	// If there is any restriction on the certificate usage
	// reject the API server request. This is done so some classes
	// of certificates issued for kubernetes usage by proxy, can not be used
	// against auth server. Later on we can extend more
	// advanced cert usage, but for now this is the safest option.
	if len(identity.Usage) != 0 && !slices.Equal(a.AcceptedUsage, identity.Usage) {
		logger.WarnContext(context.Background(), "Restricted certificate rejected while accessing the auth endpoint",
			"user", identity.Username,
			"cert_usage", identity.Usage,
			"acceptable_usage", a.AcceptedUsage,
		)
		return nil, trace.AccessDenied("access denied: invalid client certificate")
	}

	// this block assumes interactive user from remote cluster
	// based on the remote certificate authority cluster name encoded in
	// x509 organization name. This is a safe check because:
	// 1. Trust and verification is established during TLS handshake
	// by creating a cert pool constructed of trusted certificate authorities
	// 2. Remote CAs are not allowed to have the same cluster name
	// as the local certificate authority
	if certClusterName != a.ClusterName {
		// make sure that this user does not have system role
		// the local auth server can not trust remote servers
		// to issue certificates with system roles (e.g. Admin),
		// to get unrestricted access to the local cluster
		systemRole := findPrimarySystemRole(identity.Groups)
		if systemRole != nil {
			return authz.RemoteBuiltinRole{
				Role:        *systemRole,
				Username:    identity.Username,
				ClusterName: certClusterName,
				Identity:    *identity,
			}, nil
		}
		return newRemoteUserFromIdentity(*identity, certClusterName), nil
	}
	// code below expects user or service from local cluster, to distinguish between
	// interactive users and services (e.g. proxies), the code below
	// checks for presence of system roles issued in certificate identity
	systemRole := findPrimarySystemRole(identity.Groups)
	// in case if the system role is present, assume this is a service
	// agent, e.g. Proxy, connecting to the cluster
	if systemRole != nil {
		return authz.BuiltinRole{
			Role:                  *systemRole,
			AdditionalSystemRoles: extractAdditionalSystemRoles(identity.SystemRoles),
			Username:              identity.Username,
			ClusterName:           a.ClusterName,
			Identity:              *identity,
		}, nil
	}
	// otherwise assume that is a local role, no need to pass the roles
	// as it will be fetched from the local database
	return newLocalUserFromIdentity(*identity), nil
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

func extractAdditionalSystemRoles(roles []string) types.SystemRoles {
	var systemRoles types.SystemRoles
	for _, role := range roles {
		systemRole := types.SystemRole(role)
		err := systemRole.Check()
		if err != nil {
			// ignore unknown system roles rather than rejecting them, since new unknown system
			// roles may be present on certs if we rolled back from a newer version.
			logger.WarnContext(context.Background(), "Ignoring unknown system role", "unknown_role", role)
			continue
		}
		systemRoles = append(systemRoles, systemRole)
	}
	return systemRoles
}

// ServeHTTP serves HTTP requests
func (a *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		trace.WriteError(w, trace.AccessDenied("missing authentication"))
		return
	}
	user, err := a.GetUser(*r.TLS)
	if err != nil {
		trace.WriteError(w, err)
		return
	}

	remoteAddr := r.RemoteAddr
	// If the request is coming from a trusted proxy and the proxy is sending a
	// TeleportImpersonateHeader, we will impersonate the user in the header
	// instead of the user in the TLS certificate.
	// This is used by the proxy to impersonate the end user when making requests
	// without re-signing the client certificate.
	impersonateUser := r.Header.Get(TeleportImpersonateUserHeader)
	if impersonateUser != "" {
		if !isProxyRole(user) {
			trace.WriteError(w, trace.AccessDenied("Credentials forwarding is only permitted for Proxy"))
			return
		}
		// If the service is not configured to allow credentials forwarding, reject the request.
		if !a.EnableCredentialsForwarding {
			trace.WriteError(w, trace.AccessDenied("Credentials forwarding is not permitted by this service"))
			return
		}

		proxyClusterName := user.GetIdentity().TeleportCluster
		if user, err = a.extractIdentityFromImpersonationHeader(proxyClusterName, impersonateUser); err != nil {
			trace.WriteError(w, err)
			return
		}
		remoteAddr = r.Header.Get(TeleportImpersonateIPHeader)
	}

	// If the request is coming from a trusted proxy, we already know the user
	// and we will impersonate him. At this point, we need to remove the
	// TeleportImpersonateHeader from the request, otherwise the proxy will
	// attempt sending the request to upstream servers with the impersonation
	// header from a fake user.
	r.Header.Del(TeleportImpersonateUserHeader)
	r.Header.Del(TeleportImpersonateIPHeader)

	// determine authenticated user based on the request parameters
	ctx := r.Context()
	ctx = authz.ContextWithUserCertificate(ctx, certFromConnState(r.TLS))
	clientSrcAddr, err := utils.ParseAddr(remoteAddr)
	if err == nil {
		ctx = authz.ContextWithClientSrcAddr(ctx, clientSrcAddr)
	}
	ctx = authz.ContextWithUser(ctx, user)
	r = r.WithContext(ctx)
	// set remote address to the one that was passed in the header
	// this is needed because impersonation reuses the same connection
	// and the remote address is not updated from 0.0.0.0:0
	r.RemoteAddr = remoteAddr
	a.Handler.ServeHTTP(w, r)
}

// WrapContextWithUser enriches the provided context with the identity information
// extracted from the provided TLS connection.
func (a *Middleware) WrapContextWithUser(ctx context.Context, conn utils.TLSConn) (context.Context, error) {
	// Perform the handshake if it hasn't been already. Before the handshake we
	// won't have client certs available.
	if !conn.ConnectionState().HandshakeComplete {
		if err := conn.HandshakeContext(ctx); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	return a.WrapContextWithUserFromTLSConnState(ctx, conn.ConnectionState(), conn.RemoteAddr())
}

// WrapContextWithUserFromTLSConnState enriches the provided context with the identity information
// extracted from the provided TLS connection state.
func (a *Middleware) WrapContextWithUserFromTLSConnState(ctx context.Context, tlsState tls.ConnectionState, remoteAddr net.Addr) (context.Context, error) {
	user, err := a.GetUser(tlsState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx = authz.ContextWithUserCertificate(ctx, certFromConnState(&tlsState))
	ctx = authz.ContextWithClientSrcAddr(ctx, remoteAddr)
	ctx = authz.ContextWithUser(ctx, user)
	return ctx, nil
}

// isProxyRole returns true if the certificate role is a proxy role.
func isProxyRole(identity authz.IdentityGetter) bool {
	switch id := identity.(type) {
	case authz.RemoteBuiltinRole:
		return id.Role == types.RoleProxy
	case authz.BuiltinRole:
		return id.Role == types.RoleProxy
	default:
		return false
	}
}

// extractIdentityFromImpersonationHeader extracts the identity from the impersonation
// header and returns it. If the impersonation header holds an identity of a
// system role, an error is returned.
func (a *Middleware) extractIdentityFromImpersonationHeader(proxyCluster string, impersonate string) (authz.IdentityGetter, error) {
	// Unmarshal the impersonated user from the header.
	var impersonatedIdentity tlsca.Identity
	if err := json.Unmarshal([]byte(impersonate), &impersonatedIdentity); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	case findPrimarySystemRole(impersonatedIdentity.Groups) != nil:
		// make sure that this user does not have system role
		// since system roles are not allowed to be impersonated.
		return nil, trace.AccessDenied("can not impersonate a system role")
	case proxyCluster != "" && proxyCluster != a.ClusterName && proxyCluster != impersonatedIdentity.TeleportCluster:
		// If a remote proxy is impersonating a user from a different cluster, we
		// must reject the request. This is because the proxy is not allowed to
		// impersonate a user from a different cluster.
		return nil, trace.AccessDenied("can not impersonate users via a different cluster proxy")
	case impersonatedIdentity.TeleportCluster != a.ClusterName:
		// if the impersonated user is from a different cluster, we need to
		// use him as remote user.
		return newRemoteUserFromIdentity(impersonatedIdentity, impersonatedIdentity.TeleportCluster), nil
	default:
		// otherwise assume that is a local role, no need to pass the roles
		// as it will be fetched from the local database
		return newLocalUserFromIdentity(impersonatedIdentity), nil
	}
}

// newRemoteUserFromIdentity creates a new remote user from the identity.
func newRemoteUserFromIdentity(identity tlsca.Identity, clusterName string) authz.RemoteUser {
	return authz.RemoteUser{
		ClusterName:      clusterName,
		Username:         identity.Username,
		Principals:       identity.Principals,
		KubernetesGroups: identity.KubernetesGroups,
		KubernetesUsers:  identity.KubernetesUsers,
		DatabaseNames:    identity.DatabaseNames,
		DatabaseUsers:    identity.DatabaseUsers,
		RemoteRoles:      identity.Groups,
		Identity:         identity,
	}
}

// newLocalUserFromIdentity creates a new local user from the identity.
func newLocalUserFromIdentity(identity tlsca.Identity) authz.LocalUser {
	return authz.LocalUser{
		Username: identity.Username,
		Identity: identity,
	}
}

// ImpersonatorRoundTripper is a round tripper that impersonates a user with
// the identity provided.
type ImpersonatorRoundTripper struct {
	http.RoundTripper
}

// NewImpersonatorRoundTripper returns a new impersonator round tripper.
func NewImpersonatorRoundTripper(rt http.RoundTripper) *ImpersonatorRoundTripper {
	return &ImpersonatorRoundTripper{
		RoundTripper: rt,
	}
}

// RoundTrip implements http.RoundTripper interface to include the identity
// in the request header.
func (r *ImpersonatorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())

	identity, err := authz.UserFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := json.Marshal(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set(TeleportImpersonateUserHeader, string(b))

	clientSrcAddr, err := authz.ClientSrcAddrFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Set(TeleportImpersonateIPHeader, clientSrcAddr.String())

	return r.RoundTripper.RoundTrip(req)
}

// CloseIdleConnections ensures that the returned [net.RoundTripper]
// has a CloseIdleConnections method.
func (r *ImpersonatorRoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if c, ok := r.RoundTripper.(closeIdler); ok {
		c.CloseIdleConnections()
	}
}

// IdentityForwardingHeaders returns a copy of the provided headers with
// the TeleportImpersonateUserHeader and TeleportImpersonateIPHeader headers
// set to the identity provided.
// The returned headers shouln't be used across requests as they contain
// the client's IP address and the user's identity.
func IdentityForwardingHeaders(ctx context.Context, originalHeaders http.Header) (http.Header, error) {
	identity, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := json.Marshal(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers := originalHeaders.Clone()
	headers.Set(TeleportImpersonateUserHeader, string(b))

	clientSrcAddr, err := authz.ClientSrcAddrFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers.Set(TeleportImpersonateIPHeader, clientSrcAddr.String())
	return headers, nil
}
