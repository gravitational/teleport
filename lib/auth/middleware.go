/*
Copyright 2017-2021 Gravitational, Inc.

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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/trace"
	om "github.com/grpc-ecosystem/go-grpc-middleware/providers/openmetrics/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/exp/slices"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// TeleportImpersonateUserHeader is a header that specifies teleport user identity
	// that the proxy is impersonating.
	TeleportImpersonateUserHeader = "Teleport-Impersonate-User"
	// TeleportImpersonateIPHeader is a header that specifies the real user IP address.
	TeleportImpersonateIPHeader = "Teleport-Impersonate-IP"
)

// TLSServerConfig is a configuration for TLS server
type TLSServerConfig struct {
	// Listener is a listener to bind to
	Listener net.Listener
	// TLS is a base TLS configuration
	TLS *tls.Config
	// API is API server configuration
	APIConfig
	// LimiterConfig is limiter config
	LimiterConfig limiter.Config
	// AccessPoint is a caching access point
	AccessPoint AccessCache
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
	c.TLS.ClientAuth = tls.VerifyClientCertIfGiven
	if c.TLS.ClientCAs == nil {
		return trace.BadParameter("missing parameter TLS.ClientCAs")
	}
	if c.TLS.RootCAs == nil {
		return trace.BadParameter("missing parameter TLS.RootCAs")
	}
	if len(c.TLS.Certificates) == 0 {
		return trace.BadParameter("missing parameter TLS.Certificates")
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
	log *logrus.Entry
	// mux is a listener that multiplexes HTTP/2 and HTTP/1.1
	// on different listeners
	mux *multiplexer.TLSListener
}

// NewTLSServer returns new unstarted TLS server
func NewTLSServer(cfg TLSServerConfig) (*TLSServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	// limiter limits requests by frequency and amount of simultaneous
	// connections per client
	limiter, err := limiter.NewLimiter(cfg.LimiterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// sets up grpc metrics interceptor
	grpcMetrics := metrics.CreateGRPCServerMetrics(cfg.Metrics.GRPCServerLatency, prometheus.Labels{teleport.TagServer: "teleport-auth"})
	err = metrics.RegisterPrometheusCollectors(grpcMetrics)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localClusterName, err := cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// authMiddleware authenticates request assuming TLS client authentication
	// adds authentication information to the context
	// and passes it to the API server
	authMiddleware := &Middleware{
		ClusterName:   localClusterName.GetClusterName(),
		AcceptedUsage: cfg.AcceptedUsage,
		Limiter:       limiter,
		GRPCMetrics:   grpcMetrics,
	}

	apiServer, err := NewAPIServer(&cfg.APIConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authMiddleware.Wrap(apiServer)
	// Wrap sets the next middleware in chain to the authMiddleware
	limiter.WrapHandle(authMiddleware)
	// force client auth if given
	cfg.TLS.ClientAuth = tls.VerifyClientCertIfGiven
	cfg.TLS.NextProtos = []string{http2.NextProtoTLS}

	securityHeaderHandler := httplib.MakeSecurityHeaderHandler(limiter)
	tracingHandler := httplib.MakeTracingHandler(securityHeaderHandler, teleport.ComponentAuth)

	server := &TLSServer{
		cfg: cfg,
		httpServer: &http.Server{
			Handler:           tracingHandler,
			ReadHeaderTimeout: apidefaults.DefaultIOTimeout,
			IdleTimeout:       apidefaults.DefaultIdleTimeout,
		},
		log: logrus.WithFields(logrus.Fields{
			trace.Component: cfg.Component,
		}),
	}
	server.cfg.TLS.GetConfigForClient = server.GetConfigForClient

	server.grpcServer, err = NewGRPCServer(GRPCServerConfig{
		TLS:               server.cfg.TLS,
		Middleware:        authMiddleware,
		APIConfig:         cfg.APIConfig,
		UnaryInterceptor:  authMiddleware.UnaryInterceptor(),
		StreamInterceptor: authMiddleware.StreamInterceptor(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server.mux, err = multiplexer.NewTLSListener(multiplexer.TLSListenerConfig{
		Listener: tls.NewListener(cfg.Listener, server.cfg.TLS),
		ID:       cfg.ID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.PluginRegistry != nil {
		if err := cfg.PluginRegistry.RegisterAuthServices(server.grpcServer); err != nil {
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

// Serve starts GRPC and HTTP1.1 services on the mux listener
func (t *TLSServer) Serve() error {
	errC := make(chan error, 2)
	go func() {
		err := t.mux.Serve()
		t.log.WithError(err).Warningf("Mux serve failed.")
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

// GetConfigForClient is getting called on every connection
// and server's GetConfigForClient reloads the list of trusted
// local and remote certificate authorities
func (t *TLSServer) GetConfigForClient(info *tls.ClientHelloInfo) (*tls.Config, error) {
	var clusterName string
	var err error
	switch info.ServerName {
	case "":
		// Client does not use SNI, will validate against all known CAs.
	default:
		clusterName, err = apiutils.DecodeClusterName(info.ServerName)
		if err != nil {
			if !trace.IsNotFound(err) {
				t.log.Warningf("Client sent unsupported cluster name %q, what resulted in error %v.", info.ServerName, err)
				return nil, trace.AccessDenied("access is denied")
			}
		}
	}

	// update client certificate pool based on currently trusted TLS
	// certificate authorities.
	// TODO(klizhentas) drop connections of the TLS cert authorities
	// that are not trusted
	pool, totalSubjectsLen, err := DefaultClientCertPool(t.cfg.AccessPoint, clusterName)
	if err != nil {
		var ourClusterName string
		if clusterName, err := t.cfg.AccessPoint.GetClusterName(); err == nil {
			ourClusterName = clusterName.GetClusterName()
		}
		t.log.Errorf("Failed to retrieve client pool for client %v, client cluster %v, target cluster %v, error:  %v.",
			info.Conn.RemoteAddr().String(), clusterName, ourClusterName, trace.DebugReport(err))
		// this falls back to the default config
		return nil, nil
	}

	// Per https://tools.ietf.org/html/rfc5246#section-7.4.4 the total size of
	// the known CA subjects sent to the client can't exceed 2^16-1 (due to
	// 2-byte length encoding). The crypto/tls stack will panic if this
	// happens. To make the error less cryptic, catch this condition and return
	// a better error.
	//
	// This may happen with a very large (>500) number of trusted clusters, if
	// the client doesn't send the correct ServerName in its ClientHelloInfo
	// (see the switch at the top of this func).
	if totalSubjectsLen >= int64(math.MaxUint16) {
		return nil, trace.BadParameter("number of CAs in client cert pool is too large and cannot be encoded in a TLS handshake; this is due to a large number of trusted clusters; try updating tsh to the latest version; if that doesn't help, remove some trusted clusters")
	}

	tlsCopy := t.cfg.TLS.Clone()
	tlsCopy.ClientCAs = pool
	for _, cert := range tlsCopy.Certificates {
		t.log.Debugf("Server certificate %v.", TLSCertInfo(&cert))
	}
	return tlsCopy, nil
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
	// GRPCMetrics is the configured grpc metrics for the interceptors
	GRPCMetrics *om.ServerMetrics
	// EnableCredentialsForwarding allows the middleware to receive impersonation
	// identity from the client if it presents a valid proxy certificate.
	// This is used by the proxy to forward the identity of the user who
	// connected to the proxy to the next hop.
	EnableCredentialsForwarding bool
}

// Wrap sets next handler in chain
func (a *Middleware) Wrap(h http.Handler) {
	a.Handler = h
}

func getCustomRate(endpoint string) *ratelimit.RateSet {
	switch endpoint {
	// Account recovery RPCs.
	case
		"/proto.AuthService/ChangeUserAuthentication",
		"/proto.AuthService/ChangePassword",
		"/proto.AuthService/GetAccountRecoveryToken",
		"/proto.AuthService/StartAccountRecovery",
		"/proto.AuthService/VerifyAccountRecovery":
		rates := ratelimit.NewRateSet()
		// This limit means: 1 request per minute with bursts up to 10 requests.
		if err := rates.Add(time.Minute, 1, 10); err != nil {
			log.WithError(err).Debugf("Failed to define a custom rate for rpc method %q, using default rate", endpoint)
			return nil
		}
		return rates
	// Passwordless RPCs (potential unauthenticated challenge generation).
	case "/proto.AuthService/CreateAuthenticateChallenge":
		const period = defaults.LimiterPeriod
		const average = defaults.LimiterAverage
		const burst = defaults.LimiterBurst
		rates := ratelimit.NewRateSet()
		if err := rates.Add(period, average, burst); err != nil {
			log.WithError(err).Debugf("Failed to define a custom rate for rpc method %q, using default rate", endpoint)
			return nil
		}
		return rates
	}
	return nil
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
	ctx = authz.ContextWithClientAddr(ctx, peerInfo.Addr)
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

// UnaryInterceptor returns a gPRC unary interceptor which performs rate
// limiting, authenticates requests, and passes the user information as context
// metadata.
func (a *Middleware) UnaryInterceptor() grpc.UnaryServerInterceptor {
	if a.GRPCMetrics != nil {
		return utils.ChainUnaryServerInterceptors(
			otelgrpc.UnaryServerInterceptor(),
			om.UnaryServerInterceptor(a.GRPCMetrics),
			utils.GRPCServerUnaryErrorInterceptor,
			a.Limiter.UnaryServerInterceptorWithCustomRate(getCustomRate),
			a.withAuthenticatedUserUnaryInterceptor,
		)
	}
	return utils.ChainUnaryServerInterceptors(
		otelgrpc.UnaryServerInterceptor(),
		utils.GRPCServerUnaryErrorInterceptor,
		a.Limiter.UnaryServerInterceptorWithCustomRate(getCustomRate),
		a.withAuthenticatedUserUnaryInterceptor,
	)
}

// StreamInterceptor returns a gPRC stream interceptor which performs rate
// limiting, authenticates requests, and passes the user information as context
// metadata.
func (a *Middleware) StreamInterceptor() grpc.StreamServerInterceptor {
	if a.GRPCMetrics != nil {
		return utils.ChainStreamServerInterceptors(
			otelgrpc.StreamServerInterceptor(),
			om.StreamServerInterceptor(a.GRPCMetrics),
			utils.GRPCServerStreamErrorInterceptor,
			a.Limiter.StreamServerInterceptor,
			a.withAuthenticatedUserStreamInterceptor,
		)
	}
	return utils.ChainStreamServerInterceptors(
		otelgrpc.StreamServerInterceptor(),
		utils.GRPCServerStreamErrorInterceptor,
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
			log.Warnf("Failed to parse client certificate %v.", err)
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
		log.Warningf("Restricted certificate of user %q with usage %v rejected while accessing the auth endpoint with acceptable usage %v.",
			identity.Username, identity.Usage, a.AcceptedUsage)
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
		// the local auth server can not truste remote servers
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
			log.Warnf("Ignoring unknown system role: %q", role)
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

		if user, err = a.extractIdentityFromImpersonationHeader(impersonateUser); err != nil {
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
		ctx = authz.ContextWithClientAddr(ctx, clientSrcAddr)
	}
	ctx = authz.ContextWithUser(ctx, user)
	a.Handler.ServeHTTP(w, r.WithContext(ctx))
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
	ctx = authz.ContextWithClientAddr(ctx, remoteAddr)
	ctx = authz.ContextWithUser(ctx, user)
	return ctx, nil
}

// ClientCertPool returns trusted x509 certificate authority pool with CAs provided as caTypes.
// In addition, it returns the total length of all subjects added to the cert pool, allowing
// the caller to validate that the pool doesn't exceed the maximum 2-byte length prefix before
// using it.
func ClientCertPool(client AccessCache, clusterName string, caTypes ...types.CertAuthType) (*x509.CertPool, int64, error) {
	if len(caTypes) == 0 {
		return nil, 0, trace.BadParameter("at least one CA type is required")
	}

	ctx := context.TODO()
	pool := x509.NewCertPool()
	var authorities []types.CertAuthority
	if clusterName == "" {
		for _, caType := range caTypes {
			cas, err := client.GetCertAuthorities(ctx, caType, false)
			if err != nil {
				return nil, 0, trace.Wrap(err)
			}
			authorities = append(authorities, cas...)
		}
	} else {
		for _, caType := range caTypes {
			ca, err := client.GetCertAuthority(
				ctx,
				types.CertAuthID{Type: caType, DomainName: clusterName},
				false)
			if err != nil {
				return nil, 0, trace.Wrap(err)
			}

			authorities = append(authorities, ca)
		}
	}

	var totalSubjectsLen int64
	for _, auth := range authorities {
		for _, keyPair := range auth.GetTrustedTLSKeyPairs() {
			cert, err := tlsca.ParseCertificatePEM(keyPair.Cert)
			if err != nil {
				return nil, 0, trace.Wrap(err)
			}
			log.Debugf("ClientCertPool -> %v", CertInfo(cert))
			pool.AddCert(cert)

			// Each subject in the list gets a separate 2-byte length prefix.
			totalSubjectsLen += 2
			totalSubjectsLen += int64(len(cert.RawSubject))
		}
	}
	return pool, totalSubjectsLen, nil
}

// DefaultClientCertPool returns default trusted x509 certificate authority pool.
func DefaultClientCertPool(client AccessCache, clusterName string) (*x509.CertPool, int64, error) {
	return ClientCertPool(client, clusterName, types.HostCA, types.UserCA)
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
func (a *Middleware) extractIdentityFromImpersonationHeader(impersonate string) (authz.IdentityGetter, error) {
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
	identity, err := authz.UserFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b, err := json.Marshal(identity.GetIdentity())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set(TeleportImpersonateUserHeader, string(b))
	defer req.Header.Del(TeleportImpersonateUserHeader)

	clientSrcAddr, err := authz.ClientAddrFromContext(req.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req.Header.Set(TeleportImpersonateIPHeader, clientSrcAddr.String())
	defer req.Header.Del(TeleportImpersonateIPHeader)

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

	clientSrcAddr, err := authz.ClientAddrFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	headers.Set(TeleportImpersonateIPHeader, clientSrcAddr.String())
	return headers, nil
}
