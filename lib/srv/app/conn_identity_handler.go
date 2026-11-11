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
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/srv/mcp"
	"github.com/gravitational/teleport/lib/tlsca"
)

// connIdentityHandler serves connections authenticated with a client's
// route-bound App certificate. The certificate routes the whole connection to
// one app, so the handler monitors each connection rather than each request.
type connIdentityHandler struct {
	c *ConnectionsHandler

	httpServer *http.Server
	tcpServer  *tcpServer
	mcpServer  *mcp.Server
}

// newConnIdentityHandler creates the handler for route-bound connections, with
// a server for each app protocol.
func newConnIdentityHandler(c *ConnectionsHandler) (_ *connIdentityHandler, err error) {
	h := &connIdentityHandler{c: c}

	// Create and configure HTTP server with authorizing middleware.
	h.httpServer = h.newHTTPServer()

	// TCP server will handle TCP applications.
	h.tcpServer = h.newTCPServer()

	// Handle MCP servers.
	h.mcpServer, err = mcp.NewServer(mcp.ServerConfig{
		Emitter:          c.cfg.Emitter,
		ParentContext:    c.closeContext,
		HostID:           c.cfg.HostID,
		AccessPoint:      c.cfg.AccessPoint,
		EnableDemoServer: c.cfg.MCPDemoServer,
		CipherSuites:     c.cfg.CipherSuites,
		AuthClient:       c.cfg.AuthClient,
		InsecureMode:     c.cfg.InsecureMode,
		TargetHostPolicy: c.cfg.TargetHostPolicy,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return h, nil
}

// handleConnection serves a connection authenticated with a client's
// route-bound App certificate. The certificate routes to one app, so it
// authorizes and monitors the whole connection for that app, then serves it
// with the server for the app's protocol. TCP and MCP connections are served
// before it returns. HTTP ones are served after, by an http.Server.
func (h *connIdentityHandler) handleConnection(ctx context.Context, conn *tls.Conn, user authz.IdentityGetter) error {
	app, err := h.c.resolveApp(h.c.closeContext, user.GetIdentity().RouteToApp.Name, user.GetIdentity().RouteToApp.PublicAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx = authz.ContextWithUser(ctx, user)
	ctx = authz.ContextWithClientSrcAddr(ctx, conn.RemoteAddr())
	// Authorize the exact application resolved from this connection, so that
	// the app that gets authorized is the same one that gets dispatched below.
	sessionCtx, err := h.c.authorizeContext(ctx, app)

	// The behavior here is a little hard to track. To be clear here, if
	// authorization fails the following will occur:
	// 1. If the application is a TCP application, error out immediately
	//    as expected.
	// 2. If the application is an MCP application, let the MCP server handler
	//    return the error on first request.
	// 3. If the application is an HTTP application, serve from a dedicated
	//    error server, so that it's properly converted to an HTTP status code.
	//    This will ensure users get a 403 when authorization fails.
	//
	// Note: LLM doesn't require a special handling. It must work like other
	// HTTP applications.
	if err != nil {
		switch {
		case app.IsTCP():
			return trace.Wrap(err)
		case app.IsMCP():
			return trace.Wrap(h.mcpServer.HandleUnauthorizedConnection(ctx, conn, app, err))
		default:
			return trace.Wrap(h.c.handleHTTPApp(ctx, h.newErrorServer(ctx, err), conn))
		}
	}
	ctx, _, err = h.c.cfg.ConnectionMonitor.MonitorConnScoped(ctx, sessionCtx, conn)
	if err != nil {
		return trace.Wrap(err)
	}

	// Add user certificate and client source address into the context after the
	// monitor connection initialization to ensure the values are present on
	// the context.
	ctx = authz.ContextWithClientSrcAddr(ctx, conn.RemoteAddr())
	userCert := leafCertFromConn(conn)
	ctx = authz.ContextWithUserCertificate(ctx, userCert)

	switch {
	case app.IsTCP():
		identity := sessionCtx.Context.Identity.GetIdentity()
		return trace.Wrap(h.handleTCPApp(ctx, conn, &identity, app))

	case app.IsMCP():
		// TODO (williamo/scopes): change this to a scoped context when we
		// support MCP.
		unscopedAuthCtx, ok := sessionCtx.Context.UnscopedContext()
		if !ok {
			return trace.AccessDenied("MCP application access is not supported for scoped identities")
		}
		sessionCtx := mcp.SessionCtx{
			ClientConn:      conn,
			AuthCtx:         unscopedAuthCtx,
			App:             app,
			UserCertificate: userCert,
		}
		return trace.Wrap(h.mcpServer.HandleSession(ctx, &sessionCtx))

	default:
		return trace.Wrap(h.c.handleHTTPApp(ctx, h.httpServer, conn))
	}
}

// handleTCPApp handles connection for a TCP application.
func (h *connIdentityHandler) handleTCPApp(ctx context.Context, conn net.Conn, identity *tlsca.Identity, app types.Application) error {
	// Access is recorded before the TCP server's target checks, which authorize
	// the dial target rather than access to the application.
	h.c.recordAccess(ctx, identity, app, common.AppAccessIdentitySourceUserCert)
	err := h.tcpServer.handleConnection(ctx, conn, identity, app)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// newTCPServer creates a server that proxies TCP applications.
func (h *connIdentityHandler) newTCPServer() *tcpServer {
	return &tcpServer{
		clock:            h.c.cfg.Clock,
		emitter:          h.c.cfg.Emitter,
		hostID:           h.c.cfg.HostID,
		log:              h.c.log,
		accessPoint:      h.c.cfg.AccessPoint,
		authClient:       h.c.cfg.AuthClient,
		clusterName:      h.c.clusterName,
		cipherSuites:     h.c.cfg.CipherSuites,
		insecureMode:     h.c.cfg.InsecureMode,
		targetHostPolicy: h.c.cfg.TargetHostPolicy,
	}
}

// newHTTPServer creates an *http.Server that can authorize and forward
// requests to a target application. It only serves authorized connections.
func (h *connIdentityHandler) newHTTPServer() *http.Server {
	return h.c.newAppHTTPServer(&authz.Middleware{
		ClusterName:   h.c.authMiddleware.ClusterName,
		AcceptedUsage: h.c.authMiddleware.AcceptedUsage,
		Handler:       http.HandlerFunc(h.serve),
	})
}

// errorServerTimeout bounds how long a connection that failed authorization
// stays open to receive its error.
const errorServerTimeout = 10 * time.Second

// newErrorServer creates the server for a connection that failed authorization,
// which answers with err and closes the connection. ConnectionsHandler only
// closes the shared servers, so this one closes when the connection's context
// ends, or after errorServerTimeout if the client never sends a request or
// reads the error. Shutdown waits for this connection, so the timeout bounds
// that wait.
func (h *connIdentityHandler) newErrorServer(ctx context.Context, err error) *http.Server {
	server := h.c.newAppHTTPServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.writeError(w, r, err)
	}))
	ctx, cancel := context.WithTimeout(ctx, errorServerTimeout)
	context.AfterFunc(ctx, func() {
		cancel()
		server.Close()
	})
	return server
}

func (h *connIdentityHandler) serve(w http.ResponseWriter, r *http.Request) {
	// Authorize again, as policy may have changed since the connection
	// was authorized.
	req, err := h.authorize(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.c.serveAuthorizedHTTP(w, req, h.chunkConfig(req)); err != nil {
		h.writeError(w, r, err)
	}
}

// writeError writes err and closes the connection, so a client that reconnects
// is authorized again, with a new monitor.
func (h *connIdentityHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Connection", "close")
	h.c.writeHTTPError(w, r, err)
}

func (h *connIdentityHandler) authorize(r *http.Request) (authorizedRequest, error) {
	ctx := r.Context()
	authCtx, err := h.c.getAuthContext(ctx)
	if err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	identity := authCtx.Identity.GetIdentity()
	app, err := h.c.resolveApp(ctx, identity.RouteToApp.Name, identity.RouteToApp.PublicAddr)
	if err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	if _, err := h.c.appAuthorizer.Authorize(ctx, authCtx, app); err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	return authorizedRequest{r: r, authCtx: authCtx, app: app, identitySource: common.AppAccessIdentitySourceUserCert}, nil
}

// jwtExpiresWithCertEnv makes JWTs expire with the client's certificate rather
// than with their session chunk, as they did before v19
const jwtExpiresWithCertEnv = "TELEPORT_UNSTABLE_APP_JWT_EXPIRES_WITH_CERT"

// chunkConfig keys chunks by app session, which ends when its certificate
// expires. Its chunks and JWTs never outlive the certificate.
func (h *connIdentityHandler) chunkConfig(req authorizedRequest) chunkConfig {
	identity := req.authCtx.Identity.GetIdentity()
	now := h.c.cfg.Clock.Now()
	cfg := chunkConfig{
		key:     identity.RouteToApp.SessionID,
		expires: now.Add(min(identity.Expires.Sub(now), common.MaxSessionChunkDuration)),
	}
	cfg.jwtExpires = cfg.expires.Add(jwtExpiryMargin)
	if identity.Expires.Before(cfg.jwtExpires) || os.Getenv(jwtExpiresWithCertEnv) == "yes" {
		cfg.jwtExpires = identity.Expires
	}
	// The certificate's issue time is shared by every chunk of the app session.
	// serveSession logs a missing certificate.
	if userCert, _ := authz.UserCertificateFromContext(req.r.Context()); userCert != nil {
		cfg.startTime = userCert.NotBefore
	}
	return cfg
}

// leafCertFromConn returns the leaf certificate of the connection's verified
// chain, so it never returns a certificate that the handshake didn't verify.
func leafCertFromConn(conn *tls.Conn) *x509.Certificate {
	state := conn.ConnectionState()
	if len(state.VerifiedChains) == 0 || len(state.VerifiedChains[0]) == 0 {
		return nil
	}

	return state.VerifiedChains[0][0]
}
