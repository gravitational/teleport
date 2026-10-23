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
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/srv/mcp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// connIdentityHandler serves connections authenticated with a client's route-bound App certificate. The certificate
// routes the whole connection to one app, so the handler limits and monitors each connection rather than each request.
type connIdentityHandler struct {
	c *ConnectionsHandler

	httpServer *http.Server
	tcpServer  *tcpServer
	mcpServer  *mcp.Server

	connAuthMu sync.Mutex
	// connAuth is used to map an initial failure of authorization to a connection.
	// This will force the HTTP server to serve an error and close the connection.
	connAuth map[net.Conn]error
}

// newConnIdentityHandler creates the handler for route-bound connections, with a server for each app protocol.
func newConnIdentityHandler(c *ConnectionsHandler) (_ *connIdentityHandler, err error) {
	h := &connIdentityHandler{
		c:        c,
		connAuth: make(map[net.Conn]error),
	}

	// Create and configure HTTP server with authorizing middleware.
	h.httpServer = h.newHTTPServer()

	// TCP server will handle TCP applications.
	h.tcpServer, err = h.newTCPServer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

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

// handleConnection serves a connection authenticated with a client's route-bound App certificate. The certificate
// routes to one app, so it limits, authorizes and monitors the whole connection for that app. The caller runs
// the returned cleanup function, if any, after the connection closes.
func (h *connIdentityHandler) handleConnection(ctx context.Context, conn, tc net.Conn, tlsConn *tls.Conn, user authz.IdentityGetter) (func(), error) {
	// Apply connection and rate limiting to all app protocols
	// (HTTP, TCP, MCP) before any protocol-specific handling.
	// Skip limiting when the client IP cannot be extracted (e.g.
	// net.Pipe connections used by integration app proxying).
	var release func()
	if clientIP, err := utils.ClientIPFromConn(conn); err == nil {
		release, err = h.c.limiter.RegisterRequestAndConnection(clientIP)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	defer func() {
		if release != nil {
			release()
		}
	}()

	app, err := h.c.resolveApp(h.c.closeContext, user.GetIdentity().RouteToApp.Name, user.GetIdentity().RouteToApp.PublicAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx = authz.ContextWithUser(ctx, user)
	ctx = authz.ContextWithClientSrcAddr(ctx, conn.RemoteAddr())
	// Authorize the exact application resolved from this connection, so that the
	// app that gets authorized is the same one that gets dispatched below.
	sessionCtx, err := h.c.authorizeContext(ctx, app)

	// The behavior here is a little hard to track. To be clear here, if authorization fails
	// the following will occur:
	// 1. If the application is a TCP application, error out immediately as expected.
	// 2. If the application is an MCP application, let the MCP server handler
	//    returns the error on first request.
	// 3. If the application is an HTTP application, store the error and let the HTTP handler
	//    serve the error directly so that it's properly converted to an HTTP status code.
	//    This will ensure users will get a 403 when authorization fails.
	//
	// Note: LLM doesn't require a special handling. It must work like other
	// HTTP applications.
	if err != nil {
		switch {
		case app.IsTCP():
			return nil, trace.Wrap(err)
		case app.IsMCP():
			return nil, trace.Wrap(h.mcpServer.HandleUnauthorizedConnection(ctx, tlsConn, app, err))
		default:
			h.setConnAuth(tlsConn, err)
		}
	} else {
		ctx, _, err = h.c.cfg.ConnectionMonitor.MonitorConnScoped(ctx, sessionCtx, tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Add user certificate and client source address into the context after the monitor connection
	// initialization to ensure the values are present on the context.
	ctx = authz.ContextWithClientSrcAddr(ctx, conn.RemoteAddr())
	userCert := leafCertFromConn(tlsConn)
	ctx = authz.ContextWithUserCertificate(ctx, userCert)

	// TCP and MCP handlers block until the session is done, so they return
	// (nil, err) and the caller has nothing to clean up. The HTTP handler is
	// asynchronous: handleHTTPApp hands the conn to http.Server.Serve and
	// returns immediately, so it returns a cleanup function and the caller
	// blocks on waitConn.Wait() until the HTTP server closes the conn.
	switch {
	case app.IsTCP():
		identity := sessionCtx.Context.Identity.GetIdentity()
		return nil, trace.Wrap(h.handleTCPApp(ctx, tlsConn, &identity, app))

	case app.IsMCP():
		// TODO (williamo/scopes): change this to a scoped context when we support MCP.
		unscopedAuthCtx, ok := sessionCtx.Context.UnscopedContext()
		if !ok {
			return nil, trace.AccessDenied("MCP application access is not supported for scoped identities")
		}
		sessionCtx := mcp.SessionCtx{
			ClientConn:      tlsConn,
			AuthCtx:         unscopedAuthCtx,
			App:             app,
			UserCertificate: userCert,
		}
		return nil, trace.Wrap(h.mcpServer.HandleSession(ctx, &sessionCtx))

	default:
		// Transfer release ownership to the cleanup function so
		// the deferred release above becomes a no-op.
		releaseConn := release
		release = nil
		cleanup := func() {
			if releaseConn != nil {
				releaseConn()
			}
			h.deleteConnAuth(tlsConn)
		}
		return cleanup, trace.Wrap(h.c.handleHTTPApp(ctx, h.httpServer, tlsConn))
	}
}

// handleTCPApp handles connection for a TCP application.
func (h *connIdentityHandler) handleTCPApp(ctx context.Context, conn net.Conn, identity *tlsca.Identity, app types.Application) error {
	// Access is recorded before the TCP server's target checks, which authorize the dial target rather than access to
	// the application.
	h.c.recordAccess(ctx, identity, app, common.AppAccessIdentitySourceUserCert)
	err := h.tcpServer.handleConnection(ctx, conn, identity, app)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// newTCPServer creates a server that proxies TCP applications.
func (h *connIdentityHandler) newTCPServer() (*tcpServer, error) {
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
	}, nil
}

// newHTTPServer creates an *http.Server that can authorize and forward
// requests to a target application.
func (h *connIdentityHandler) newHTTPServer() *http.Server {
	server := h.c.newAppHTTPServer(&authz.Middleware{
		ClusterName:   h.c.authMiddleware.ClusterName,
		AcceptedUsage: h.c.authMiddleware.AcceptedUsage,
		Handler:       http.HandlerFunc(h.serve),
	})
	server.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, connContextKey, conn)
	}
	return server
}

func (h *connIdentityHandler) serve(w http.ResponseWriter, r *http.Request) {
	// Serve a failure to authorize the connection as an HTTP error. Otherwise authorize again, as policy may have
	// changed since the connection was set up.
	conn, ok := r.Context().Value(connContextKey).(net.Conn)
	if !ok {
		h.c.log.ErrorContext(h.c.closeContext, "Unable to extract connection from context.")
	}
	if err := h.getAndDeleteConnAuth(conn); err != nil {
		h.writeError(w, r, err)
		return
	}
	req, err := h.authorize(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.c.serveAuthorizedHTTP(w, req, h.chunkConfig(req)); err != nil {
		h.writeError(w, r, err)
	}
}

// writeError writes err, and closes the connection afterwards so that the monitor is recreated if needed.
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
	state, err := h.c.accessState(ctx, authCtx)
	if err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	if _, err := h.c.authorizeApp(ctx, authCtx, app, state); err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	return authorizedRequest{r: r, authCtx: authCtx, app: app, identitySource: common.AppAccessIdentitySourceUserCert}, nil
}

// jwtExpiresWithCertEnv makes JWTs expire with the client's certificate rather than with their session chunk, as they
// did before v19
const jwtExpiresWithCertEnv = "TELEPORT_UNSTABLE_APP_JWT_EXPIRES_WITH_CERT"

// chunkConfig keys chunks by app session, which ends when its certificate expires. Its chunks and JWTs never outlive
// the certificate.
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
	// The certificate's issue time is shared by every chunk of the app session. serveSession logs a missing
	// certificate.
	if userCert, _ := authz.UserCertificateFromContext(req.r.Context()); userCert != nil {
		cfg.startTime = userCert.NotBefore
	}
	return cfg
}

func (h *connIdentityHandler) getAndDeleteConnAuth(conn net.Conn) error {
	h.connAuthMu.Lock()
	defer h.connAuthMu.Unlock()
	err := h.connAuth[conn]
	delete(h.connAuth, conn)
	return err
}

func (h *connIdentityHandler) setConnAuth(conn net.Conn, err error) {
	h.connAuthMu.Lock()
	defer h.connAuthMu.Unlock()
	h.connAuth[conn] = err
}

func (h *connIdentityHandler) deleteConnAuth(conn net.Conn) {
	h.connAuthMu.Lock()
	defer h.connAuthMu.Unlock()
	delete(h.connAuth, conn)
}

// leafCertFromConn returns the leaf certificate from the connection.
func leafCertFromConn(tlsConn *tls.Conn) *x509.Certificate {
	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return nil
	}

	return state.PeerCertificates[0]
}
