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
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	appauthz "github.com/gravitational/teleport/lib/srv/app/authz"
	"github.com/gravitational/teleport/lib/srv/app/common"
)

// acceptsForwardedIdentity reports whether requestIdentityHandler is on. The
// heartbeat also checks it, so the agent only advertises the feature when it
// serves it.
func (c *ConnectionsHandler) acceptsForwardedIdentity() bool {
	return c.requestIdentityHandler != nil
}

// requestIdentityHandler serves connections from a local Proxy that forwards a
// client identity with each request. Such a connection carries requests from
// many clients, so the handler admits each request rather than the connection.
type requestIdentityHandler struct {
	c *ConnectionsHandler

	httpServer *http.Server
}

// newRequestIdentityHandler creates the handler for connections from a
// forwarding Proxy.
func newRequestIdentityHandler(c *ConnectionsHandler) *requestIdentityHandler {
	h := &requestIdentityHandler{c: c}
	h.httpServer = h.newHTTPServer(http.HandlerFunc(h.serve))
	return h
}

// accepts reports whether the connection initiator, the identity in a
// connection's client certificate, is a local Proxy that may forward
// client identities.
func (h *requestIdentityHandler) accepts(user authz.IdentityGetter) bool {
	// A Proxy of another cluster is an authz.RemoteBuiltinRole, so it never
	// matches. Leaf clusters are out of scope.
	proxy, ok := user.(authz.BuiltinRole)
	return ok && proxy.Role == types.RoleProxy
}

// handleConnection serves a connection from a forwarding Proxy. A Proxy needs
// nothing from Auth to forward any user's identity, so this is where a lock on
// a compromised Proxy takes effect. A Proxy's authorization can only change
// through a lock, so the monitor closes the connection on one.
func (h *requestIdentityHandler) handleConnection(ctx context.Context, conn net.Conn, proxy authz.IdentityGetter) error {
	authCtx, err := h.c.cfg.Authorizer.AuthorizeScoped(authz.ContextWithUser(ctx, proxy))
	if err != nil {
		return trace.Wrap(err)
	}
	// accepts admits only a Proxy's unscoped built-in role.
	proxyCtx, ok := authCtx.UnscopedContext()
	if !ok {
		return trace.AccessDenied("forwarding Proxy has a scoped identity")
	}
	// The monitor returns the tracking connection under TLS, which carries
	// ciphertext, so the server keeps reading from conn.
	ctx, _, err = h.c.cfg.ConnectionMonitor.MonitorConnLocks(ctx, proxyCtx, conn)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(h.c.handleHTTPApp(ctx, h.httpServer, conn))
}

// forwardedIdleTimeout is how long the server keeps a forwarding Proxy's idle
// connection. The Proxy controls the regular timeout. This is a backstop to
// prevent connections leaking due to a buggy proxy.
const forwardedIdleTimeout = 10 * time.Minute

// newHTTPServer creates the server for connections from a forwarding Proxy.
// authz.Middleware puts the forwarded identity in place of the Proxy's. Tests
// pass their own serve to inspect what the chain admits.
func (h *requestIdentityHandler) newHTTPServer(serve http.Handler) *http.Server {
	server := h.c.newAppHTTPServer(&authz.Middleware{
		ClusterName:                 h.c.clusterName,
		EnableCredentialsForwarding: true,
		Handler:                     h.admit(serve),
	})
	server.IdleTimeout = forwardedIdleTimeout
	return server
}

// admit passes a request to next only if check admits it, and without the
// Proxy's certificate.
func (h *requestIdentityHandler) admit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.check(r); err != nil {
			h.writeProxyError(w, err)
			return
		}
		// authz.Middleware stores the TLS peer's certificate as the user
		// certificate, but here the peer is the Proxy. The client's certificate
		// never reaches this service. Nothing may use the Proxy's certificate
		// in its place, for example as the session start time or to issue a
		// managed upstream client certificate.
		next.ServeHTTP(w, r.WithContext(authz.ContextWithUserCertificate(r.Context(), nil)))
	})
}

// check checks that a request's forwarded identity and client address are
// valid.
func (h *requestIdentityHandler) check(r *http.Request) error {
	user, err := authz.UserFromContext(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := appauthz.ValidateForwardedIdentity(user, h.c.clusterName); err != nil {
		return trace.Wrap(err)
	}
	_, err = forwardedClientIP(r)
	return trace.Wrap(err)
}

// forwardedClientIP returns the IP of the client address that authz.Middleware
// read from the forwarded request. IP pinning needs an IP address and port, so
// it rejects anything else.
func forwardedClientIP(r *http.Request) (string, error) {
	addr, err := authz.ClientSrcAddrFromContext(r.Context())
	if err != nil {
		return "", trace.AccessDenied("forwarded request has no client address")
	}
	// authz.Middleware puts a parsed copy of the address in the context, and
	// the parser also accepts forms such as a URL. r.RemoteAddr keeps the
	// address as sent, and the reverse proxy sets X-Forwarded-For from it. The
	// two only match for a plain host and port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || r.RemoteAddr != addr.String() || net.ParseIP(host) == nil {
		return "", trace.AccessDenied("forwarded request has an invalid client address %q", r.RemoteAddr)
	}
	return host, nil
}

func (h *requestIdentityHandler) serve(w http.ResponseWriter, r *http.Request) {
	req, err := h.authorize(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	cfg, err := h.chunkConfig(req)
	if err != nil {
		h.c.writeHTTPError(w, r, err)
		return
	}
	if err := h.c.serveAuthorizedHTTP(w, req, cfg); err != nil {
		h.c.writeHTTPError(w, r, err)
	}
}

// authorize authorizes a request for the application its forwarded identity
// routes to.
func (h *requestIdentityHandler) authorize(r *http.Request) (authorizedRequest, error) {
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
	// The application and the identity's roles may have changed since the
	// Proxy admitted the connection.
	if _, err := h.c.appAuthorizer.AuthorizeForwarded(ctx, authCtx, app); err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	return authorizedRequest{r: r, authCtx: authCtx, app: app, identitySource: common.AppAccessIdentitySourceProxy}, nil
}

// maxForwardedIdentityAge is how long after its source certificate's NotBefore
// a forwarded identity can start or join a session chunk. Forwarded session
// chunks count event indexes in nanoseconds from NotBefore, and indexes past
// 2^53 lose precision as JSON numbers. A day of margin covers chunks and
// requests already in flight.
const maxForwardedIdentityAge = time.Duration(1<<53) - 24*time.Hour

// chunkConfig gives chunks and their JWTs a fixed lifetime, since a forwarded
// connection may outlive the certificate its identity came from.
func (h *requestIdentityHandler) chunkConfig(req authorizedRequest) (chunkConfig, error) {
	identity := req.authCtx.Identity.GetIdentity()
	// A zero NotBefore is too long ago, and one in the future fails as a
	// route-bound certificate's would at the TLS handshake.
	if age := h.c.cfg.Clock.Since(identity.NotBefore); age < 0 || age > maxForwardedIdentityAge {
		return chunkConfig{}, trace.Errorf("forwarded identity's NotBefore %v is in the future or too long ago for exact event indexes", identity.NotBefore)
	}
	// The Proxy defines the session ID. We guard against a bug on the Proxy
	// that produces conflicting session IDs by combining the session ID with a
	// SHA256 of the forwarded identity, as the Proxy encoded it.
	encoded, err := authz.ForwardedIdentityFromContext(req.r.Context())
	if err != nil {
		return chunkConfig{}, trace.Wrap(err)
	}
	digest := sha256.Sum256([]byte(encoded))
	expires := h.c.cfg.Clock.Now().Add(common.MaxSessionChunkDuration)
	return chunkConfig{
		key:        identity.RouteToApp.SessionID + "/" + hex.EncodeToString(digest[:]),
		expires:    expires,
		jwtExpires: expires.Add(jwtExpiryMargin),
		// As on the route-bound path, every chunk of the session counts event
		// indexes from the source certificate's NotBefore.
		startTime: identity.NotBefore,
	}, nil
}

// writeError gives authorization failures an [common.AppErrorCode]. Unlike
// connIdentityHandler.writeError, it keeps the connection open, because the
// connection carries other clients' requests.
func (h *requestIdentityHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	code, ok := appauthz.WriteForwardedError(w, r, err)
	if !ok {
		// Not an authorization failure, such as an upstream error.
		h.c.writeHTTPError(w, r, err)
		return
	}
	attrs := []slog.Attr{slog.String("code", string(code))}
	// The connection carries other clients' requests, so only the forwarded
	// identity tells whose request was refused.
	if user, userErr := authz.UserFromContext(r.Context()); userErr == nil {
		identity := user.GetIdentity()
		attrs = append(attrs,
			slog.String("user", identity.Username),
			slog.String("app", identity.RouteToApp.Name),
		)
	}
	// The error goes last, since its trace report spans many lines.
	attrs = append(attrs, slog.Any("error", err))
	h.c.log.LogAttrs(r.Context(), slog.LevelWarn, "Failed to serve request", attrs...)
}

// writeProxyError answers a request that the Proxy shouldn't have sent. The
// fault is Teleport's rather than the client's, so it's an internal error with
// no error code.
func (h *requestIdentityHandler) writeProxyError(w http.ResponseWriter, err error) {
	h.c.log.WarnContext(h.c.closeContext, "Rejected a request from a forwarding Proxy", "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
