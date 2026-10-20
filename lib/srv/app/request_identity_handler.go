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
	"errors"
	"net"
	"net/http"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/componentfeatures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// acceptsForwardedIdentity reports whether requestIdentityHandler is on. The heartbeat also checks it, so the agent
// only advertises the feature when it serves it.
func (c *ConnectionsHandler) acceptsForwardedIdentity() bool {
	return c.requestIdentityHandler != nil
}

// requestIdentityHandler serves connections from a local Proxy that forwards a client identity with each request.
// Such a connection carries requests from many clients, so the handler admits and limits each request rather than the
// connection.
type requestIdentityHandler struct {
	c *ConnectionsHandler

	httpServer *http.Server
}

// newRequestIdentityHandler creates the handler for connections from a forwarding Proxy.
func newRequestIdentityHandler(c *ConnectionsHandler) *requestIdentityHandler {
	h := &requestIdentityHandler{c: c}
	h.httpServer = h.newHTTPServer(http.HandlerFunc(h.serve))
	return h
}

// accepts reports whether the connection initiator, the identity in a connection's client certificate, is a local
// Proxy that may forward client identities.
func (h *requestIdentityHandler) accepts(user authz.IdentityGetter) bool {
	// A Proxy of another cluster is an authz.RemoteBuiltinRole, so it never matches. Leaf clusters are out of scope.
	proxy, ok := user.(authz.BuiltinRole)
	return ok && proxy.Role == types.RoleProxy
}

// handleConnection serves a connection from a Proxy that forwards a client identity with each request.
func (h *requestIdentityHandler) handleConnection(ctx context.Context, conn net.Conn) error {
	return trace.Wrap(h.c.handleHTTPApp(ctx, h.httpServer, conn))
}

// newHTTPServer creates the server for connections from a forwarding Proxy. authz.Middleware puts the forwarded
// identity in place of the Proxy's. The limiter runs after admission, so it only counts admitted requests, keyed by
// the forwarded client. Tests pass their own serve to inspect what the chain admits.
func (h *requestIdentityHandler) newHTTPServer(serve http.Handler) *http.Server {
	return h.c.newAppHTTPServer(&authz.Middleware{
		ClusterName:                 h.c.clusterName,
		EnableCredentialsForwarding: true,
		Handler:                     h.admit(h.limit(serve)),
	})
}

// admit passes a request to next only if check admits it, and without the Proxy's certificate.
func (h *requestIdentityHandler) admit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := h.check(r); err != nil {
			h.writeProxyError(w, err)
			return
		}
		// authz.Middleware stores the TLS peer's certificate as the user certificate, but here the peer is the Proxy.
		// The client's certificate never reaches this service. Nothing may use the Proxy's certificate in its place,
		// for example as the session start time or to issue a managed upstream client certificate.
		next.ServeHTTP(w, r.WithContext(authz.ContextWithUserCertificate(r.Context(), nil)))
	})
}

// limit charges each request to its forwarded client in the handler's limiter. connIdentityHandler charges the same
// limiter per connection, so a client has one budget across both handlers. A Proxy's pooled connections never count
// against a client.
func (h *requestIdentityHandler) limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP, err := forwardedClientIP(r)
		if err != nil {
			h.writeProxyError(w, err)
			return
		}
		release, err := h.c.limiter.RegisterRequestAndConnection(clientIP)
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		defer release()
		next.ServeHTTP(w, r)
	})
}

// check checks that the Proxy that sent a request may still forward identities, and that the forwarded identity and
// client address are valid.
func (h *requestIdentityHandler) check(r *http.Request) error {
	if err := h.authorizeProxy(r); err != nil {
		return trace.Wrap(err)
	}
	user, err := authz.UserFromContext(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateForwardedIdentity(user, h.c.clusterName); err != nil {
		return trace.Wrap(err)
	}
	_, err = forwardedClientIP(r)
	return trace.Wrap(err)
}

// authorizeProxy authorizes the Proxy that sent a request, including checking it against locks, as Auth does on each
// of its calls. A Proxy needs nothing from Auth to forward any user's identity, so this is where a lock on a
// compromised Proxy takes effect. The context now holds the forwarded identity, so this reads the Proxy's identity from
// the connection again.
func (h *requestIdentityHandler) authorizeProxy(r *http.Request) error {
	proxy, err := h.c.authMiddleware.GetUser(r.Context(), *r.TLS)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = h.c.cfg.Authorizer.AuthorizeScoped(authz.ContextWithUser(r.Context(), proxy))
	return trace.Wrap(err)
}

// validateForwardedIdentity checks that a forwarded identity has the shape of a route-bound App certificate for this
// cluster. authz.Middleware only rejects a forwarded identity with a primary system role. It skips the scope pin and
// usage checks that it applies to a client certificate, so this function repeats them.
func validateForwardedIdentity(user authz.IdentityGetter, clusterName string) error {
	localUser, ok := user.(authz.LocalUser)
	if !ok {
		return trace.AccessDenied("forwarded identity must be a local user, got %T", user)
	}
	identity := localUser.Identity

	switch {
	case len(identity.SystemRoles) != 0:
		return trace.AccessDenied("forwarded identity has system roles")
	case identity.ScopePin != nil && identity.ScopePin.GetKind() != scopesv1.PinKind_PIN_KIND_USER:
		return trace.AccessDenied("forwarded identity has a %v scope pin", identity.ScopePin.GetKind())
	case len(identity.Usage) != 0 && !slices.Equal(identity.Usage, []string{teleport.UsageAppsOnly}):
		return trace.AccessDenied("forwarded identity is restricted to %v", identity.Usage)
	case identity.RouteToCluster != "" && identity.RouteToCluster != clusterName:
		return trace.AccessDenied("forwarded identity routes to cluster %q", identity.RouteToCluster)
	case identity.KubernetesCluster != "" || !identity.RouteToDatabase.Empty() ||
		identity.RouteToGit != (tlsca.RouteToGit{}):
		return trace.AccessDenied("forwarded identity routes to another service")
	}

	route := identity.RouteToApp
	switch {
	case route.SessionID == "" || route.Name == "" || route.PublicAddr == "":
		return trace.AccessDenied("forwarded identity has an incomplete app route")
	case route.ClusterName != clusterName:
		return trace.AccessDenied("forwarded identity routes to an app in cluster %q", route.ClusterName)
	case route.AWSRoleARN != "" || route.AzureIdentity != "" || route.GCPServiceAccount != "" ||
		route.TargetPort != 0 || route.AWSCredentialProcessCredentials != "":
		// These fields select an upstream identity or port for app types that requestIdentityHandler does not serve.
		return trace.AccessDenied("forwarded identity has an app route for an unsupported app type")
	}
	return nil
}

// forwardedClientIP returns the IP of the client address that authz.Middleware read from the forwarded request. IP
// pinning needs an IP address and port, and the limiter needs the IP, so it rejects anything else.
func forwardedClientIP(r *http.Request) (string, error) {
	addr, err := authz.ClientSrcAddrFromContext(r.Context())
	if err != nil {
		return "", trace.AccessDenied("forwarded request has no client address")
	}
	// authz.Middleware puts a parsed copy of the address in the context, and the parser also accepts forms such as a
	// URL. r.RemoteAddr keeps the address as sent, and the reverse proxy sets X-Forwarded-For from it. The two only
	// match for a plain host and port.
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
	if err := h.c.serveAuthorizedHTTP(w, req); err != nil {
		h.c.writeHTTPError(w, r, err)
	}
}

var errUnsupportedApplicationType = &trace.AccessDeniedError{
	Message: "application can't be served with a forwarded client identity",
}

// authorize authorizes a request for the application its forwarded identity routes to. Until the identity may
// discover the application, a response must not reveal anything about it, so discovery comes before the checks whose
// errors are specific to the application.
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
	if _, err := authCtx.CheckerContext.Decision(ctx, app.GetScope(), func(checker *services.ScopedAccessChecker) error {
		return trace.Wrap(checker.App().CanAccessApp(app))
	}); err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	// The application may have changed since the Proxy admitted the connection.
	if !componentfeatures.SupportsAppForwardedIdentityV1(app) {
		return authorizedRequest{}, trace.Wrap(errUnsupportedApplicationType)
	}
	state, err := h.c.accessState(ctx, authCtx)
	if err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	// A forwarded identity comes from a certificate that routes to no application, so MFA evidence in it would satisfy
	// per-session MFA for every application the user can access. Only the access state ignores it. Locks, audit events
	// and hardware key enforcement still read the identity's fields.
	state.MFAVerified = false
	if _, err := h.c.authorizeApp(ctx, authCtx, app, state); err != nil {
		return authorizedRequest{}, trace.Wrap(err)
	}
	return authorizedRequest{r: r, authCtx: authCtx, app: app}, nil
}

// writeError gives authorization failures an [common.AppErrorCode], so a client can tell them apart from the
// application's responses. A failure without a more specific code is access-denied, even for an application that
// doesn't exist, so a response never reveals one the client may not discover. Unlike connIdentityHandler.writeError,
// it keeps the connection open, because the connection carries other clients' requests.
func (h *requestIdentityHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var code common.AppErrorCode
	var message string
	switch {
	case errors.Is(err, errUnsupportedApplicationType):
		code = common.AppErrorCodeUnsupportedApplicationType
		message = "This application can't be accessed on this path. Use an application-specific access path."
	case errors.Is(err, services.ErrSessionMFARequired):
		code = common.AppErrorCodeMFARequired
		message = "This application requires per-session MFA, which this access path can't complete."
	// An unmet private key policy is a BadParameter whose message tells tsh to log in again with a hardware key, which
	// a client of this path can't act on.
	case errors.Is(err, services.ErrTrustedDeviceRequired), trace.IsAccessDenied(err), trace.IsNotFound(err),
		keys.IsPrivateKeyPolicyError(err):
		code = common.AppErrorCodeAccessDenied
		message = http.StatusText(http.StatusForbidden)
	default:
		// Not an authorization failure, such as a limit.
		h.c.writeHTTPError(w, r, err)
		return
	}

	h.c.log.WarnContext(h.c.closeContext, "Failed to serve request", "code", code, "error", err)
	w.Header().Set(common.TeleportAppErrorCodeHeader, string(code))
	if errors.Is(err, services.ErrTrustedDeviceRequired) {
		writeTrustedDeviceRequired(w, r, http.StatusForbidden)
		return
	}
	http.Error(w, message, http.StatusForbidden)
}

// writeProxyError answers a request that the Proxy shouldn't have sent. The
// fault is Teleport's rather than the client's, so it's an internal error with
// no error code.
func (h *requestIdentityHandler) writeProxyError(w http.ResponseWriter, err error) {
	h.c.log.WarnContext(h.c.closeContext, "Rejected a request from a forwarding Proxy", "error", err)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
