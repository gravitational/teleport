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
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/trace"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	appaws "github.com/gravitational/teleport/lib/srv/app/aws"
	appazure "github.com/gravitational/teleport/lib/srv/app/azure"
	"github.com/gravitational/teleport/lib/srv/app/common"
	appgcp "github.com/gravitational/teleport/lib/srv/app/gcp"
	appllm "github.com/gravitational/teleport/lib/srv/app/llm"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/inflighttracker"
)

// ConnMonitor monitors authorized connections and terminates them when
// session controls dictate so.
type ConnMonitor interface {
	MonitorConnScoped(ctx context.Context, scopedCtx *authz.ScopedSessionContext, conn net.Conn) (context.Context, net.Conn, error)
	MonitorConnLocks(ctx context.Context, authzCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error)
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
	Authorizer authz.ScopedAuthorizer

	// HostID is the id of the host where this application agent is running.
	HostID string

	// TargetHostScope is the scope of the host where this application agent is running.
	TargetHostScope string

	// AuthClient is a client directly connected to the Auth server.
	AuthClient authclient.ClientI

	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint authclient.AppsAccessPoint

	// Cloud provides cloud provider access related functionality.
	Cloud Cloud

	// AWSConfigOptions is used to provide additional options when getting
	// config.
	AWSConfigOptions []awsconfig.OptionsFn

	// TLSConfig is the *tls.Config for this server.
	TLSConfig *tls.Config

	// ConnectionMonitor monitors connections and terminates any if
	// any session controls prevent them.
	ConnectionMonitor ConnMonitor

	// CipherSuites is the list of TLS cipher suites that have been configured
	// for this process.
	CipherSuites []uint16

	// ServiceComponent is the Teleport Component identifier used for tracing
	// and logging. Must be one of teleport.Component* values.
	// Eg, teleport.ComponentApp
	ServiceComponent string

	// Logger is the slog.Logger.
	Logger *slog.Logger

	// MCPDemoServer enables the "Teleport Demo" MCP server.
	MCPDemoServer bool

	// InsecureMode defines whether insecure connections are allowed.
	InsecureMode bool

	// TargetHostPolicy restricts application target dials by resolved IP.
	TargetHostPolicy common.TargetHostPolicy

	// AcceptForwardedIdentity allows a local Proxy to forward a client identity
	// with each request instead of presenting it as the connection's client
	// certificate (RFD 0341). Only the Application Service sets it.
	AcceptForwardedIdentity bool
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
	if c.Logger == nil {
		c.Logger = slog.Default().With(teleport.ComponentKey, teleport.Component(c.ServiceComponent))
	}
	if c.Cloud == nil {
		cloud, err := NewCloud(CloudConfig{
			Clock:            c.Clock,
			AWSConfigOptions: c.AWSConfigOptions,
			Logger:           c.Logger,
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
	if err := c.TargetHostPolicy.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ConnectionsHandler handles Connections for the ApplicationService. It
// authenticates requests from the web proxy and forwards them to
// internal applications.
type ConnectionsHandler struct {
	cfg *ConnectionsHandlerConfig
	log *slog.Logger

	// closeContext is for the process and it outlives every connection.
	closeContext context.Context
	// connsCtx is the parent of every connection context and it is canceled by
	// calls to Close.
	connsCtx    context.Context
	connsCancel context.CancelFunc
	// conns counts each connection being served so that Shutdown can drain
	// existing connections and refuse new connections.
	conns inflighttracker.Tracker

	tlsConfig *tls.Config

	// connIdentityHandler serves every connection that requestIdentityHandler
	// doesn't accept.
	connIdentityHandler *connIdentityHandler
	// requestIdentityHandler serves connections from a local Proxy that
	// forwards client identities. It is nil unless AcceptForwardedIdentity
	// is set.
	requestIdentityHandler *requestIdentityHandler

	// cache holds sessionChunk objects for in-flight app sessions.
	cache *utils.FnCache
	// cacheCloseWg prevents closing the app server until all app
	// sessions have been removed from the cache and closed.
	cacheCloseWg sync.WaitGroup

	// accessRecorder emits app.access events for authorized access.
	accessRecorder *common.AccessRecorder

	// appAuthorizer runs App RBAC for connections and requests.
	appAuthorizer appauthz.Authorizer

	awsHandler   http.Handler
	azureHandler http.Handler
	gcpHandler   http.Handler
	llmHandler   *appllm.Handler

	// authMiddleware reads the identity in a client certificate, and rejects a
	// certificate restricted to a service other than apps.
	authMiddleware *authz.Middleware

	proxyPort   string
	clusterName string

	// resolveApp returns a types.Application using the name and public address
	// as matchers.
	resolveApp func(ctx context.Context, name, addr string) (types.Application, error)

	// v9Warned dedupes the v9 enforcement warnings to once per warning kind,
	// user, and app, so one kind does not suppress another. A v9 role governs
	// every request to a shared app, so the agent would otherwise log a warning
	// on every request.
	v9Warned *lru.Cache[v9WarnKey, struct{}]
}

// v9WarnedSize bounds the warning deduplication cache. Every user and app
// combination the agent serves takes an entry, so the cache is bounded
// rather than tracking them all. Evicting the least recently warned entry
// repeats its warning, which is the behavior to prefer over unbounded
// growth.
const v9WarnedSize = 2048

// v9WarnKey identifies one fired v9 enforcement warning in v9Warned.
type v9WarnKey struct{ kind, user, app string }

// v9WarnOnce reports whether the v9 enforcement warning of the given kind
// has not fired yet for the user and app, and marks it fired.
func (c *ConnectionsHandler) v9WarnOnce(kind, user, app string) bool {
	warned, _ := c.v9Warned.ContainsOrAdd(v9WarnKey{kind, user, app}, struct{}{})
	return !warned
}

// NewConnectionsHandler returns a new ConnectionsHandler.
func NewConnectionsHandler(closeContext context.Context, cfg *ConnectionsHandlerConfig) (*ConnectionsHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	awsConfigProvider, err := awsconfig.NewCache(awsconfig.WithDefaults(cfg.AWSConfigOptions...))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsHandler, err := appaws.NewAWSSignerHandler(closeContext, appaws.SignerHandlerConfig{
		AWSConfigProvider: awsConfigProvider,
		Clock:             cfg.Clock,
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

	llmHandler, err := appllm.NewHandler(closeContext, appllm.HandlerConfig{
		Log:               cfg.Logger.With(teleport.ComponentKey, teleport.ComponentLLM),
		AWSConfigProvider: awsConfigProvider,
		RecordingEnabled:  appllm.BeamLLMRecordingEnabled(os.Getenv),
		UsageReporter:     cfg.AuthClient.LLMUsageClient(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := cfg.AccessPoint.GetClusterName(closeContext)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v9Warned, err := lru.New[v9WarnKey, struct{}](v9WarnedSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connsCtx, connsCancel := context.WithCancel(closeContext)
	c := &ConnectionsHandler{
		cfg:          cfg,
		closeContext: closeContext,
		connsCtx:     connsCtx,
		connsCancel:  connsCancel,
		awsHandler:   awsHandler,
		azureHandler: azureHandler,
		gcpHandler:   gcpHandler,
		llmHandler:   llmHandler,
		log:          slog.With(teleport.ComponentKey, cfg.ServiceComponent),
		clusterName:  clusterName.GetClusterName(),
		v9Warned:     v9Warned,
		resolveApp: func(context.Context, string, string) (types.Application, error) {
			return nil, trace.NotFound("no applications are being proxied")
		},
	}
	c.appAuthorizer = appauthz.Authorizer{Logger: c.log, AuthPreferenceGetter: cfg.AccessPoint}

	c.accessRecorder, err = common.NewAccessRecorder(closeContext, cfg.Clock, cfg.HostID, cfg.Emitter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create a new session cache, this holds sessions that can be used to
	// forward requests.
	c.cache, err = utils.NewFnCache(utils.FnCacheConfig{
		TTL:             common.MaxSessionChunkDuration,
		Context:         c.closeContext,
		Clock:           c.cfg.Clock,
		CleanupInterval: time.Second,
		OnExpiry:        c.onSessionExpired,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.authMiddleware = &authz.Middleware{
		ClusterName:   clusterName.GetClusterName(),
		AcceptedUsage: []string{teleport.UsageAppsOnly},
	}

	c.connIdentityHandler, err = newConnIdentityHandler(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.AcceptForwardedIdentity {
		c.requestIdentityHandler = newRequestIdentityHandler(c)
	}

	// Make copy of server's TLS configuration and update it with the specific
	// functionality this server needs, like requiring client certificates.
	c.tlsConfig = CopyAndConfigureTLSForCluster(c.log, c.cfg.AccessPoint, clusterName.GetClusterName(), c.cfg.TLSConfig)

	// Figure out the port the proxy is running on.
	c.proxyPort = c.getProxyPort(c.closeContext)

	go c.expireSessions()

	return c, nil
}

// SetApplicationsProvider sets the internal state for the monitored
// applications. This method must be called before the ConnectionsHandler is
// able to handle connections.
func (c *ConnectionsHandler) SetApplicationsProvider(
	fn func(context.Context, string, string) (types.Application, error)) {
	c.resolveApp = fn
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
	if !c.conns.Add() {
		conn.Close()
		return
	}
	defer c.conns.Done()

	ctx, cancel := context.WithCancelCause(c.connsCtx)
	// HandleConnection owns the context. handleConnection passes cancel
	// to TrackingReadConn, which may call it first with io.EOF when the
	// connection closes. The second cancel call is a no-op.
	defer cancel(nil)

	// Wrap conn to detect when it is closed.
	// Returning early will close conn before it has been serviced.
	// httpServer will initiate the close call.
	waitConn := utils.NewWaitConn(conn)

	if err := c.handleConnection(ctx, cancel, waitConn); err != nil {
		if !utils.IsOKNetworkError(err) {
			c.log.WarnContext(ctx, "Failed to handle client connection.", "error", err)
		}
		if err := conn.Close(); err != nil && !utils.IsOKNetworkError(err) {
			c.log.WarnContext(ctx, "Failed to close client connection.", "error", err)
		}
		return
	}

	// The reverse tunnel closes the connection's channel when this returns, so
	// wait until the connection is closed. An http.Server may still be
	// serving it.
	waitConn.Wait()
}

// chunkConfig is how a handler groups its requests into session chunks. Each
// handler sets it, because an identity taken from a certificate and one
// forwarded with each request last differently. Only key applies to every
// request. The other fields apply only when the request creates a chunk.
type chunkConfig struct {
	// Requests with the same key share a chunk.
	key string
	// expires is when the chunk stops accepting new requests, delayed by
	// however long creating the chunk took, because the cache counts the
	// chunk's lifetime from when it was created.
	expires    time.Time
	jwtExpires time.Time
	// startTime is when the chunk's recording starts counting event indexes.
	// Events keep the identity's session ID rather than the chunk's, so the
	// chunks of one session must share it, or their event indexes collide.
	startTime time.Time
}

// serveSession finds the app session and forwards the request.
func (c *ConnectionsHandler) serveSession(w http.ResponseWriter, req authorizedRequest, cfg chunkConfig, opts ...sessionOpt) error {
	r, app := req.r, req.app
	identity := req.authCtx.Identity.GetIdentity()
	// The user certificate is set on the request context by authz middleware.
	// It's needed to issue managed upstream client certificates.
	userCert, err := authz.UserCertificateFromContext(r.Context())
	if err != nil {
		c.log.WarnContext(r.Context(), "Unable to retrieve session user certificate.", "error", err)
	}

	// Fetch a cached request forwarder (or create one) that lives about 5
	// minutes. Used to stream session chunks to the Audit Log.
	ttl := cfg.expires.Sub(c.cfg.Clock.Now())
	session, err := c.cache.GetWithTTL(r.Context(), cfg.key, ttl, func(ctx context.Context) (*sessionChunk, error) {
		// The FnCache loader runs on a context detached from the request, which
		// drops request-scoped values. Re-inject the user certificate so
		// session opts (e.g. managed upstream client certs) can read it.
		if userCert != nil {
			ctx = authz.ContextWithUserCertificate(ctx, userCert)
		}
		return c.newSessionChunk(ctx, &identity, app, cfg.startTime, opts...)
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
		Identity: &identity,
		App:      app,
		ChunkID:  session.id,
		Audit:    session.audit,
	}

	// Forward request to the target application. Only Teleport may set
	// reserved headers, so they're removed from the application's responses.
	common.WithoutReservedResponseHeaders(session.handler).ServeHTTP(w, common.WithSessionContext(r, sessionCtx))
	return nil
}

// recordAccess emits app.access for authorized access to app. Failing to record
// access does not deny it.
func (c *ConnectionsHandler) recordAccess(ctx context.Context, identity *tlsca.Identity, app types.Application, identitySource common.AppAccessIdentitySource) {
	// Logged at debug because a failed emission is returned to every later
	// access to the session until the deduplication period ends, and the audit
	// emitter already reports its own failures.
	if err := c.accessRecorder.RecordAccess(ctx, identity, app, identitySource); err != nil && ctx.Err() == nil {
		c.log.DebugContext(ctx, "Failed to emit app access audit event", "app", app.GetName(), "error", err)
	}
}

func (c *ConnectionsHandler) Shutdown(ctx context.Context) []error {
	c.conns.Close()
	var errs []error
	if err := c.connIdentityHandler.httpServer.Shutdown(ctx); err != nil {
		errs = append(errs, err)
	}
	if c.requestIdentityHandler != nil {
		if err := c.requestIdentityHandler.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.conns.Wait(ctx); err != nil {
		errs = append(errs, err)
	}
	return append(errs, c.Close(ctx)...)
}

func (c *ConnectionsHandler) Close(ctx context.Context) []error {
	c.conns.Close()
	var errs []error
	// Stop the HTTP servers.
	if err := c.connIdentityHandler.httpServer.Close(); err != nil {
		errs = append(errs, err)
	}
	if c.requestIdentityHandler != nil {
		if err := c.requestIdentityHandler.httpServer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	c.connsCancel()

	c.cache.Shutdown(ctx)
	// Any sessions still in the cache during shutdown are closed in
	// background goroutines. We must wait for sessions to finish closing
	// before proceeding any further.
	c.cacheCloseWg.Wait()

	if c.llmHandler != nil {
		if err := c.llmHandler.Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// authorizedRequest is a request that a handler has authorized for app. They
// travel together so that the serving step can't be given a request with an
// authorization made for another one.
type authorizedRequest struct {
	r       *http.Request
	authCtx *authz.ScopedContext
	app     types.Application
	// identitySource is where the request's identity came from.
	identitySource common.AppAccessIdentitySource
}

// serveAuthorizedHTTP forwards req to its application. Both handlers' requests
// rejoin here, so everything from here on applies to both.
func (c *ConnectionsHandler) serveAuthorizedHTTP(w http.ResponseWriter, req authorizedRequest, cfg chunkConfig) error {
	r, app := req.r, req.app
	identity := req.authCtx.Identity.GetIdentity()
	// Access is recorded before the app_resources rules below, which authorize
	// individual request paths rather than access to the application.
	c.recordAccess(r.Context(), &identity, app, req.identitySource)

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
			return c.serveSession(w, req, cfg, c.withAWSSigner)
		}

		// Request for AWS console access originated from Teleport Proxy WebUI
		// is not signed by SigV4.
		return c.serveAWSWebConsole(w, r, &identity, app)

	case app.IsAzureCloud():
		return c.serveSession(w, req, cfg, c.withAzureHandler)

	case app.IsGCP():
		return c.serveSession(w, req, cfg, c.withGCPHandler)

	case app.IsLLM():
		return c.serveSession(w, req, cfg, c.withLLMHandler)

	default:
		// The default case is a plain HTTP app. app_resources rules are
		// enforced only on the unscoped access path. A scoped identity was
		// already authorized for this app, and the scoped access model does
		// not yet evaluate v9 app_resources, so v9 enforcement is skipped for
		// it rather than denying an authorized request. Making the scoped path
		// v9-aware is follow-up work.
		if unscopedAuthCtx, ok := req.authCtx.UnscopedContext(); ok {
			if deny := c.appResourcesDenial(r, unscopedAuthCtx, app); deny != nil {
				deny.write(w)
				return nil
			}
		}
		return c.serveSession(w, req, cfg, c.withJWTTokenForwarder(cfg.jwtExpires))
	}
}

// getProxyPort tries to figure out the address the proxy is running at.
func (c *ConnectionsHandler) getProxyPort(ctx context.Context) string {
	servers, err := clientutils.CollectWithFallback(ctx, c.cfg.AccessPoint.ListProxyServers, func(context.Context) ([]types.Server, error) {
		//nolint:staticcheck // TODO(kiosion) DELETE IN 21.0.0
		return c.cfg.AccessPoint.GetProxies()
	})
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

	// A scoped app references its integration by scope-qualified name.
	integration, err := apiscopes.ParseOptionallyQualifiedName(app.GetIntegration())
	if err != nil {
		return trace.Wrap(err)
	}

	url, err := c.cfg.Cloud.GetAWSSigninURL(r.Context(), AWSSigninRequest{
		Identity:    identity,
		TargetURL:   app.GetURI(),
		Issuer:      app.GetPublicAddr(),
		ExternalID:  app.GetAWSExternalID(),
		Integration: integration,
		RolesAnywhereMetadata: awsconfig.RolesAnywhereMetadata{
			ProfileARN:                    app.GetAWSRolesAnywhereProfileARN(),
			ProfileAcceptsRoleSessionName: app.GetAWSRolesAnywhereAcceptRoleSessionName(),
			RoleARN:                       identity.RouteToApp.AWSRoleARN,
			IdentityUsername:              identity.Username,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	http.Redirect(w, r, url.SigninURL, http.StatusFound)
	return nil
}

// authorizeContext builds an authorization context and checks access to app.
// The caller must pass the exact application that will be served. On success it
// returns the scoped context bundled with the session controls of the role that
// granted access.
func (c *ConnectionsHandler) authorizeContext(ctx context.Context, app types.Application) (*authz.ScopedSessionContext, error) {
	authCtx, err := c.getAuthContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionControls, err := c.appAuthorizer.Authorize(ctx, authCtx, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authz.ScopedSessionContext{
		Context:         authCtx,
		SessionControls: sessionControls,
	}, nil
}

// getAuthContext checks if the context carries identity information and
// builds an authorization context for it.
func (c *ConnectionsHandler) getAuthContext(ctx context.Context) (*authz.ScopedContext, error) {
	// Only allow local and remote identities to proxy to an application.
	userType, err := authz.UserFromContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch userType.(type) {
	case authz.LocalUser, authz.RemoteUser:
	default:
		return nil, trace.BadParameter("invalid identity: %T", userType)
	}

	authContext, err := c.cfg.Authorizer.AuthorizeScoped(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authContext, nil
}

func (c *ConnectionsHandler) handleConnection(ctx context.Context, cancel context.CancelCauseFunc, conn net.Conn) error {
	tc, err := srv.NewTrackingReadConn(srv.TrackingReadConnConfig{
		Conn:    conn,
		Clock:   c.cfg.Clock,
		Context: ctx,
		Cancel:  cancel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConn, user, err := c.authenticateConn(c.closeContext, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	// A connection from a forwarding Proxy carries requests from many clients,
	// so requestIdentityHandler takes the client's identity from each request.
	// For every other connection, connIdentityHandler takes it from the
	// client certificate.
	if c.acceptsForwardedIdentity() && c.requestIdentityHandler.accepts(user) {
		return trace.Wrap(c.requestIdentityHandler.handleConnection(ctx, tlsConn, user))
	}
	return trace.Wrap(c.connIdentityHandler.handleConnection(ctx, tlsConn, user))
}

// handleHTTPApp handles connection for an HTTP application.
func (c *ConnectionsHandler) handleHTTPApp(ctx context.Context, server *http.Server, conn net.Conn) error {
	// Wrap a TLS authorizing conn in a single-use listener.
	listener := newListener(ctx, conn)

	// Serve will return as soon as tlsConn is running in its own goroutine
	err := server.Serve(listener)
	if err != nil && !errors.Is(err, errListenerConnServed) {
		// okay to ignore errListenerConnServed; it is a signal that our
		// single-use listener has passed the connection to http.Serve
		// and conn is being served. See listener.Accept for details.
		return trace.Wrap(err)
	}

	return nil
}

// newAppHTTPServer creates an *http.Server with the settings shared by
// connIdentityHandler and requestIdentityHandler.
func (c *ConnectionsHandler) newAppHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		// Note: read/write timeouts *should not* be set here because it will
		// break application access.
		Handler:           httplib.MakeTracingHandler(handler, c.cfg.ServiceComponent),
		ReadHeaderTimeout: defaults.ReadHeadersTimeout,
		IdleTimeout:       apidefaults.DefaultIdleTimeout,
		ErrorLog:          log.Default(),
	}
}

// writeHTTPError logs err and converts it to an HTTP response.
func (c *ConnectionsHandler) writeHTTPError(w http.ResponseWriter, r *http.Request, err error) {
	c.log.WarnContext(c.closeContext, "Failed to serve request", "error", err)

	code := trace.ErrorToCode(err)
	switch {
	case errors.Is(err, authz.ErrUserNotFound), errors.Is(err, authz.ErrUserSessionRoleNotFound):
		w.Header().Set(common.TeleportAppErrorCodeHeader, string(common.AppErrorCodeSessionInvalid))
		http.Error(w, http.StatusText(code), code)
	case errors.Is(err, services.ErrTrustedDeviceRequired):
		appauthz.WriteTrustedDeviceRequired(w, r, code)
	case errors.Is(err, services.ErrSessionMFARequired):
		http.Error(w, authclient.ErrNoMFADevices.Error(), code)
	default:
		http.Error(w, http.StatusText(code), code)
	}
}

// authenticateConn completes the TLS handshake and returns the identity in the
// peer's client certificate. Connections from the reverse tunnel are expected
// to use TLS and carry an identity this way.
func (c *ConnectionsHandler) authenticateConn(ctx context.Context, conn net.Conn) (*tls.Conn, authz.IdentityGetter, error) {
	tlsConn := tls.Server(conn, c.tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		return nil, nil, trace.Wrap(err, "TLS handshake failed")
	}

	user, err := c.authMiddleware.GetUser(ctx, tlsConn.ConnectionState())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return tlsConn, user, nil
}

// CopyAndConfigureTLSForCluster copies config and modifies it for Teleport
// application proxy servers in the local cluster clusterName.
func CopyAndConfigureTLSForCluster(log *slog.Logger, client authclient.AccessCache, clusterName string, config *tls.Config) *tls.Config {
	tlsConfig := config.Clone()
	if log == nil {
		log = slog.Default()
	}

	// Require clients to present a certificate.
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	// Configure function that will be used to fetch the CAs that signed the
	// client's certificate.
	tlsConfig.GetConfigForClient = authclient.WithClusterCAs(tlsConfig, client, clusterName, log)

	return tlsConfig
}
