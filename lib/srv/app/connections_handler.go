/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

// Package app runs the application proxy process. It keeps dynamic labels
// updated, heart beats its presence, checks access controls, and forwards
// connections between the tunnel and the target host.
package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	appaws "github.com/gravitational/teleport/lib/srv/app/aws"
	appazure "github.com/gravitational/teleport/lib/srv/app/azure"
	"github.com/gravitational/teleport/lib/srv/app/common"
	appgcp "github.com/gravitational/teleport/lib/srv/app/gcp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// ConnMonitor monitors authorized connections and terminates them when
// session controls dictate so.
type ConnMonitor interface {
	MonitorConn(ctx context.Context, authzCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error)
}

// ConnectionsHandlerConfig is the configuration for a ConnectionsHandler.
type ConnectionsHandlerConfig struct {
	// Clock is used to control time.
	Clock clockwork.Clock

	// DataDir is the path to the data directory for the server.
	DataDir string

	// Emitter is an event emitter.
	Emitter events.Emitter

	// Authorizer is used to authorize requests.
	Authorizer authz.Authorizer

	// HostID is the id of the host where this application agent is running.
	HostID string

	// AuthClient is a client directly connected to the Auth server.
	AuthClient authclient.ClientI

	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint authclient.AppsAccessPoint

	// Cloud provides cloud provider access related functionality.
	Cloud Cloud

	// AWSSessionProvider is used to provide AWS Sessions.
	AWSSessionProvider awsutils.AWSSessionProvider

	// TLSConfig is the *tls.Config for this server.
	TLSConfig *tls.Config

	// ConnectionMonitor monitors connections and terminates any if
	// any session controls prevent them.
	ConnectionMonitor ConnMonitor

	// CipherSuites is the list of TLS cipher suites that have been configured
	// for this process.
	CipherSuites []uint16

	// ServiceComponent is the Teleport Component identifier used for tracing and logging.
	// Must be one of teleport.Component* values.
	// Eg, teleport.ComponentApp
	ServiceComponent string

	// Logger is the slog.Logger.
	Logger *slog.Logger
}

// CheckAndSetDefaults validates the config values and sets defaults.
func (c *ConnectionsHandlerConfig) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.DataDir == "" {
		return trace.BadParameter("data dir missing")
	}
	if c.HostID == "" {
		return trace.BadParameter("host id missing")
	}
	if c.AuthClient == nil {
		return trace.BadParameter("auth client missing")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("access point missing")
	}
	if c.Emitter == nil {
		return trace.BadParameter("emitter missing")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("authorizer missing")
	}
	if c.TLSConfig == nil {
		return trace.BadParameter("tls config missing")
	}
	if c.AWSSessionProvider == nil {
		return trace.BadParameter("aws session provider missing")
	}
	if c.Cloud == nil {
		cloud, err := NewCloud(CloudConfig{
			Clock:         c.Clock,
			SessionGetter: c.AWSSessionProvider,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		c.Cloud = cloud
	}
	if len(c.CipherSuites) == 0 {
		return trace.BadParameter("ciphersuites missing")
	}
	if c.ServiceComponent == "" {
		return trace.BadParameter("service component missing")
	}
	if c.Logger == nil {
		c.Logger = slog.Default().With(teleport.ComponentKey, teleport.Component(c.ServiceComponent))
	}
	return nil
}

// ConnectionsHandler handles Connections for the ApplicationService.
// It authenticates requests from the web proxy and forwards them to internal applications.
type ConnectionsHandler struct {
	cfg *ConnectionsHandlerConfig
	log *slog.Logger

	closeContext context.Context

	httpServer *http.Server
	tlsConfig  *tls.Config
	tcpServer  *tcpServer

	// cache holds sessionChunk objects for in-flight app sessions.
	cache *utils.FnCache
	// cacheCloseWg prevents closing the app server until all app
	// sessions have been removed from the cache and closed.
	cacheCloseWg sync.WaitGroup

	connAuthMu sync.Mutex
	// connAuth is used to map an initial failure of authorization to a connection.
	// This will force the HTTP server to serve an error and close the connection.
	connAuth map[net.Conn]error

	awsHandler   http.Handler
	azureHandler http.Handler
	gcpHandler   http.Handler

	// authMiddleware allows wrapping connections with identity information.
	authMiddleware *auth.Middleware

	proxyPort string

	// getAppByPublicAddress returns a types.Application using the public address as matcher.
	getAppByPublicAddress func(context.Context, string) (types.Application, error)
}

// NewConnectionsHandler returns a new ConnectionsHandler.
func NewConnectionsHandler(closeContext context.Context, cfg *ConnectionsHandlerConfig) (*ConnectionsHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	awsSigner, err := awsutils.NewSigningService(awsutils.SigningServiceConfig{
		Clock:           cfg.Clock,
		SessionProvider: cfg.AWSSessionProvider,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	awsHandler, err := appaws.NewAWSSignerHandler(closeContext, appaws.SignerHandlerConfig{
		SigningService: awsSigner,
		Clock:          cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	azureHandler, err := appazure.NewAzureHandler(closeContext, appazure.HandlerConfig{
		Log: cfg.Logger.With(teleport.ComponentKey, appazure.ComponentKey),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gcpHandler, err := appgcp.NewGCPHandler(closeContext, appgcp.HandlerConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c := &ConnectionsHandler{
		cfg:          cfg,
		closeContext: closeContext,
		awsHandler:   awsHandler,
		azureHandler: azureHandler,
		gcpHandler:   gcpHandler,
		connAuth:     make(map[net.Conn]error),
		log:          slog.With(teleport.ComponentKey, cfg.ServiceComponent),
		getAppByPublicAddress: func(ctx context.Context, s string) (types.Application, error) {
			return nil, trace.NotFound("no applications are being proxied")
		},
	}

	// Create a new session cache, this holds sessions that can be used to
	// forward requests.
	c.cache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL:             5 * time.Minute,
		Context:         c.closeContext,
		Clock:           c.cfg.Clock,
		CleanupInterval: time.Second,
		OnExpiry:        c.onSessionExpired,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go c.expireSessions()

	clustername, err := c.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create and configure HTTP server with authorizing middleware.
	c.httpServer = c.newHTTPServer(clustername.GetClusterName())

	// TCP server will handle TCP applications.
	tcpServer, err := c.newTCPServer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.tcpServer = tcpServer

	// Make copy of server's TLS configuration and update it with the specific
	// functionality this server needs, like requiring client certificates.
	c.tlsConfig = CopyAndConfigureTLS(c.log, c.cfg.AccessPoint, c.cfg.TLSConfig)

	// Figure out the port the proxy is running on.
	c.proxyPort = c.getProxyPort()

	return c, nil
}

// SetApplicationsProvider sets the internal state for the monitored applications.
// This method must be called before the ConnectionsHandler is able to handle connections.
func (c *ConnectionsHandler) SetApplicationsProvider(fn func(context.Context, string) (types.Application, error)) {
	c.getAppByPublicAddress = fn
}

func (c *ConnectionsHandler) expireSessions() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cache.RemoveExpired()
		case <-c.closeContext.Done():
			return
		}
	}
}

// HandleConnection takes a connection and wraps it in a listener, so it can
// be passed to http.Serve to process as a HTTP request.
func (c *ConnectionsHandler) HandleConnection(conn net.Conn) {
	ctx := context.Background()

	// Wrap conn in a CloserConn to detect when it is closed.
	// Returning early will close conn before it has been serviced.
	// httpServer will initiate the close call.
	closerConn := utils.NewCloserConn(conn)

	cleanup, err := c.handleConnection(closerConn)
	// Make sure that the cleanup function is run
	if cleanup != nil {
		defer cleanup()
	}

	if err != nil {
		if !utils.IsOKNetworkError(err) {
			c.log.WarnContext(ctx, "Failed to handle client connection.", "error", err)
		}
		if err := conn.Close(); err != nil && !utils.IsOKNetworkError(err) {
			c.log.WarnContext(ctx, "Failed to close client connection.", "error", err)
		}
		return
	}

	// Wait for connection to close.
	closerConn.Wait()
}

// serveSession finds the app session and forwards the request.
func (c *ConnectionsHandler) serveSession(w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, app types.Application, opts ...sessionOpt) error {
	// Fetch a cached request forwarder (or create one) that lives about 5
	// minutes. Used to stream session chunks to the Audit Log.
	ttl := min(identity.Expires.Sub(c.cfg.Clock.Now()), 5*time.Minute)
	session, err := utils.FnCacheGetWithTTL(r.Context(), c.cache, identity.RouteToApp.SessionID, ttl, func(ctx context.Context) (*sessionChunk, error) {
		session, err := c.newSessionChunk(ctx, identity, app, c.sessionStartTime(r.Context()), opts...)
		return session, trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := session.acquire(); err != nil {
		return trace.Wrap(err)
	}
	defer session.release()

	// Create session context.
	sessionCtx := &common.SessionContext{
		Identity: identity,
		App:      app,
		ChunkID:  session.id,
		Audit:    session.audit,
	}

	// Forward request to the target application.
	session.handler.ServeHTTP(w, common.WithSessionContext(r, sessionCtx))
	return nil
}

// sessionStartTime fetches the session start time based on the certificate
// valid date.
func (c *ConnectionsHandler) sessionStartTime(ctx context.Context) time.Time {
	if userCert, err := authz.UserCertificateFromContext(ctx); err == nil {
		return userCert.NotBefore
	}

	c.log.WarnContext(ctx, "Unable to retrieve session start time from certificate.")
	return time.Time{}
}

// newTCPServer creates a server that proxies TCP applications.
func (c *ConnectionsHandler) newTCPServer() (*tcpServer, error) {
	return &tcpServer{
		emitter: c.cfg.Emitter,
		hostID:  c.cfg.HostID,
		log:     c.log,
	}, nil
}

// Close performs a graceful shutdown.
func (c *ConnectionsHandler) Close(ctx context.Context) []error {
	var errs []error
	// Stop HTTP server.
	if err := c.httpServer.Close(); err != nil {
		errs = append(errs, err)
	}

	// Close the session cache and its remaining sessions.
	c.cache.Shutdown(c.closeContext)
	// Any sessions still in the cache during shutdown are closed in
	// background goroutines. We must wait for sessions to finish closing
	// before proceeding any further.
	c.cacheCloseWg.Wait()

	return errs
}

func (c *ConnectionsHandler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	// Extract the identity and application being requested from the certificate
	// and check if the caller has access.
	authCtx, app, err := c.authorizeContext(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	identity := authCtx.Identity.GetIdentity()
	switch {
	case app.IsAWSConsole():
		// Requests from AWS applications are signed by AWS Signature Version 4
		// algorithm. AWS CLI and AWS SDKs automatically use SigV4 for all
		// services that support it (All services expect Amazon SimpleDB but
		// this AWS service has been deprecated)
		//
		// Also check header common.TeleportAWSAssumedRole which is added by
		// the local proxy for AWS requests signed by assumed roles.
		if awsutils.IsSignedByAWSSigV4(r) || r.Header.Get(common.TeleportAWSAssumedRole) != "" {
			return c.serveSession(w, r, &identity, app, c.withAWSSigner)
		}

		// Request for AWS console access originated from Teleport Proxy WebUI
		// is not signed by SigV4.
		return c.serveAWSWebConsole(w, r, &identity, app)

	case app.IsAzureCloud():
		return c.serveSession(w, r, &identity, app, c.withAzureHandler)

	case app.IsGCP():
		return c.serveSession(w, r, &identity, app, c.withGCPHandler)

	default:
		return c.serveSession(w, r, &identity, app, c.withJWTTokenForwarder)
	}
}

// getProxyPort tries to figure out the address the proxy is running at.
func (c *ConnectionsHandler) getProxyPort() string {
	servers, err := c.cfg.AccessPoint.GetProxies()
	if err != nil {
		return strconv.Itoa(defaults.HTTPListenPort)
	}
	if len(servers) == 0 {
		return strconv.Itoa(defaults.HTTPListenPort)
	}
	_, port, err := net.SplitHostPort(servers[0].GetPublicAddr())
	if err != nil {
		return strconv.Itoa(defaults.HTTPListenPort)
	}
	return port
}

// serveAWSWebConsole generates a sign-in URL for AWS management console and
// redirects the user to it.
func (c *ConnectionsHandler) serveAWSWebConsole(w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, app types.Application) error {
	c.log.DebugContext(c.closeContext, "Redirect to AWS management console.",
		"username", identity.Username,
		"aws_role_arn", identity.RouteToApp.AWSRoleARN,
	)

	url, err := c.cfg.Cloud.GetAWSSigninURL(r.Context(), AWSSigninRequest{
		Identity:    identity,
		TargetURL:   app.GetURI(),
		Issuer:      app.GetPublicAddr(),
		ExternalID:  app.GetAWSExternalID(),
		Integration: app.GetIntegration(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	http.Redirect(w, r, url.SigninURL, http.StatusFound)
	return nil
}

// authorizeContext will check if the context carries identity information and
// runs authorization checks on it.
func (c *ConnectionsHandler) authorizeContext(ctx context.Context) (*authz.Context, types.Application, error) {
	// Only allow local and remote identities to proxy to an application.
	userType, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	switch userType.(type) {
	case authz.LocalUser, authz.RemoteUser:
	default:
		return nil, nil, trace.BadParameter("invalid identity: %T", userType)
	}

	// Extract authorizing context and identity of the user from the request.
	authContext, err := c.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	identity := authContext.Identity.GetIdentity()

	// Fetch the application and check if the identity has access.
	app, err := c.getAppByPublicAddress(ctx, identity.RouteToApp.PublicAddr)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	authPref, err := c.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// When accessing AWS management console, check permissions to assume
	// requested IAM role as well.
	var matchers []services.RoleMatcher
	if app.IsAWSConsole() {
		matchers = append(matchers, &services.AWSRoleARNMatcher{
			RoleARN: identity.RouteToApp.AWSRoleARN,
		})
	}

	// When accessing Azure API, check permissions to assume
	// requested Azure identity as well.
	if app.IsAzureCloud() {
		matchers = append(matchers, &services.AzureIdentityMatcher{
			Identity: identity.RouteToApp.AzureIdentity,
		})
	}

	// When accessing GCP API, check permissions to assume
	// requested GCP service account as well.
	if app.IsGCP() {
		matchers = append(matchers, &services.GCPServiceAccountMatcher{
			ServiceAccount: identity.RouteToApp.GCPServiceAccount,
		})
	}

	state := authContext.GetAccessState(authPref)
	switch err := authContext.Checker.CheckAccess(
		app,
		state,
		matchers...); {
	case errors.Is(err, services.ErrTrustedDeviceRequired):
		// Let the trusted device error through for clarity.
		return nil, nil, trace.Wrap(services.ErrTrustedDeviceRequired)
	case err != nil:
		c.log.WarnContext(c.closeContext, "Access denied to application.",
			"app", app.GetName(),
			"error", err,
		)
		return nil, nil, utils.OpaqueAccessDenied(err)
	}

	return authContext, app, nil
}

func (c *ConnectionsHandler) handleConnection(conn net.Conn) (func(), error) {
	ctx, cancel := context.WithCancelCause(c.closeContext)
	tc, err := srv.NewTrackingReadConn(srv.TrackingReadConnConfig{
		Conn:    conn,
		Clock:   c.cfg.Clock,
		Context: ctx,
		Cancel:  cancel,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Proxy sends a X.509 client certificate to pass identity information,
	// extract it and run authorization checks on it.
	tlsConn, user, app, err := c.getConnectionInfo(c.closeContext, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx = authz.ContextWithUser(ctx, user)
	ctx = authz.ContextWithClientSrcAddr(ctx, conn.RemoteAddr())
	authCtx, _, err := c.authorizeContext(ctx)

	// The behavior here is a little hard to track. To be clear here, if authorization fails
	// the following will occur:
	// 1. If the application is a TCP application, error out immediately as expected.
	// 2. If the application is an HTTP application, store the error and let the HTTP handler
	//    serve the error directly so that it's properly converted to an HTTP status code.
	//    This will ensure users will get a 403 when authorization fails.
	if err != nil {
		if !app.IsTCP() {
			c.setConnAuth(tlsConn, err)
		} else {
			return nil, trace.Wrap(err)
		}
	} else {
		// Monitor the connection an update the context.
		ctx, _, err = c.cfg.ConnectionMonitor.MonitorConn(ctx, authCtx, tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Add user certificate into the context after the monitor connection
	// initialization to ensure value is present on the context.
	ctx = authz.ContextWithUserCertificate(ctx, leafCertFromConn(tlsConn))

	// Application access supports plain TCP connections which are handled
	// differently than HTTP requests from web apps.
	if app.IsTCP() {
		identity := authCtx.Identity.GetIdentity()
		defer cancel(nil)
		return nil, trace.Wrap(c.handleTCPApp(ctx, tlsConn, &identity, app))
	}

	cleanup := func() {
		cancel(nil)
		c.deleteConnAuth(tlsConn)
	}
	return cleanup, trace.Wrap(c.handleHTTPApp(ctx, tlsConn))
}

// handleHTTPApp handles connection for an HTTP application.
func (c *ConnectionsHandler) handleHTTPApp(ctx context.Context, conn net.Conn) error {
	// Wrap a TLS authorizing conn in a single-use listener.
	listener := newListener(ctx, conn)

	// Serve will return as soon as tlsConn is running in its own goroutine
	err := c.httpServer.Serve(listener)
	if err != nil && !errors.Is(err, errListenerConnServed) {
		// okay to ignore errListenerConnServed; it is a signal that our
		// single-use listener has passed the connection to http.Serve
		// and conn is being served. See listener.Accept for details.
		return trace.Wrap(err)
	}

	return nil
}

// handleTCPApp handles connection for a TCP application.
func (c *ConnectionsHandler) handleTCPApp(ctx context.Context, conn net.Conn, identity *tlsca.Identity, app types.Application) error {
	err := c.tcpServer.handleConnection(ctx, conn, identity, app)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// newHTTPServer creates an *http.Server that can authorize and forward
// requests to a target application.
func (c *ConnectionsHandler) newHTTPServer(clusterName string) *http.Server {
	// Reuse the auth.Middleware to authorize requests but only accept
	// certificates that were specifically generated for applications.

	c.authMiddleware = &auth.Middleware{
		ClusterName:   clusterName,
		AcceptedUsage: []string{teleport.UsageAppsOnly},
	}
	c.authMiddleware.Wrap(c)

	return &http.Server{
		// Note: read/write timeouts *should not* be set here because it will
		// break application access.
		Handler:           httplib.MakeTracingHandler(c.authMiddleware, c.cfg.ServiceComponent),
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		ErrorLog:          log.Default(),
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return context.WithValue(ctx, connContextKey, c)
		},
	}
}

// ServeHTTP will forward the *http.Request to the target application.
func (c *ConnectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// See if the initial auth failed. If it didn't, serve the HTTP regularly, which
	// will include subsequent auth attempts to prevent race-type conditions.
	conn, ok := r.Context().Value(connContextKey).(net.Conn)
	if !ok {
		c.log.ErrorContext(c.closeContext, "Unable to extract connection from context.")
	}
	err := c.getAndDeleteConnAuth(conn)
	if err == nil {
		err = c.serveHTTP(w, r)
	}
	if err != nil {
		c.log.WarnContext(c.closeContext, "Failed to serve request", "error", err)

		// Covert trace error type to HTTP and write response, make sure we close the
		// connection afterwards so that the monitor is recreated if needed.
		code := trace.ErrorToCode(err)

		var text string
		if errors.Is(err, services.ErrTrustedDeviceRequired) {
			// Return a nicer error message for device trust errors.
			text = `Access to this app requires a trusted device.

See https://goteleport.com/docs/admin-guides/access-controls/device-trust/device-management/#troubleshooting for help.
`
		} else {
			text = http.StatusText(code)
		}

		w.Header().Set("Connection", "close")
		http.Error(w, text, code)
	}
}

// getConnectionInfo extracts identity information from the provided
// connection and runs authorization checks on it.
//
// The connection comes from the reverse tunnel and is expected to be TLS and
// carry identity in the client certificate.
func (c *ConnectionsHandler) getConnectionInfo(ctx context.Context, conn net.Conn) (*tls.Conn, authz.IdentityGetter, types.Application, error) {
	tlsConn := tls.Server(conn, c.tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, nil, nil, trace.Wrap(err, "TLS handshake failed")
	}

	user, err := c.authMiddleware.GetUser(tlsConn.ConnectionState())
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	app, err := c.getAppByPublicAddress(ctx, user.GetIdentity().RouteToApp.PublicAddr)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return tlsConn, user, app, nil
}

func (c *ConnectionsHandler) getAndDeleteConnAuth(conn net.Conn) error {
	c.connAuthMu.Lock()
	defer c.connAuthMu.Unlock()
	err := c.connAuth[conn]
	delete(c.connAuth, conn)
	return err
}

func (c *ConnectionsHandler) setConnAuth(conn net.Conn, err error) {
	c.connAuthMu.Lock()
	defer c.connAuthMu.Unlock()
	c.connAuth[conn] = err
}

func (c *ConnectionsHandler) deleteConnAuth(conn net.Conn) {
	c.connAuthMu.Lock()
	defer c.connAuthMu.Unlock()
	delete(c.connAuth, conn)
}

// CopyAndConfigureTLS can be used to copy and modify an existing *tls.Config
// for Teleport application proxy servers.
func CopyAndConfigureTLS(log *slog.Logger, client authclient.AccessCache, config *tls.Config) *tls.Config {
	tlsConfig := config.Clone()
	if log == nil {
		log = slog.Default()
	}

	// Require clients to present a certificate
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CA that signed the
	// client's certificate to verify the chain presented. If the client does not
	// pass in the cluster name, this functions pulls back all CA to try and
	// match the certificate presented against any CA.
	tlsConfig.GetConfigForClient = newGetConfigForClientFn(log, client, tlsConfig)

	return tlsConfig
}

func newGetConfigForClientFn(log *slog.Logger, client authclient.AccessCache, tlsConfig *tls.Config) func(*tls.ClientHelloInfo) (*tls.Config, error) {
	return func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		var clusterName string
		var err error

		// Try and extract the name of the cluster that signed the client's certificate.
		if info.ServerName != "" {
			clusterName, err = apiutils.DecodeClusterName(info.ServerName)
			if err != nil {
				if !trace.IsNotFound(err) {
					log.DebugContext(info.Context(), "Ignoring unsupported cluster name", "cluster_name", info.ServerName)
				}
			}
		}

		// Fetch list of CAs that could have signed this certificate. If clusterName
		// is empty, all CAs that this cluster knows about are returned.
		pool, _, err := authclient.DefaultClientCertPool(info.Context(), client, clusterName)
		if err != nil {
			// If this request fails, return nil and fallback to the default ClientCAs.
			log.DebugContext(info.Context(), "Failed to retrieve client pool", "error", err)
			return nil, nil
		}

		// Don't modify the server's *tls.Config, create one per connection because
		// the requests could be coming from different clusters.
		tlsCopy := tlsConfig.Clone()
		tlsCopy.ClientCAs = pool
		return tlsCopy, nil
	}
}

// leafCertFromConn returns the leaf certificate from the connection.
func leafCertFromConn(tlsConn *tls.Conn) *x509.Certificate {
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil
	}

	return state.PeerCertificates[0]
}
