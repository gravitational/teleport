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

package githttp

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// ConnMonitor monitors authorized connections and terminates them when
// session controls dictate so.
type ConnMonitor interface {
	MonitorConn(ctx context.Context, authzCtx *authz.Context, conn net.Conn) (context.Context, net.Conn, error)
}

// GitServerGetter looks up git server resources.
type GitServerGetter interface {
	GetGitServer(ctx context.Context, name string) (types.Server, error)
}

// HandlerConfig is the configuration for the git HTTP handler.
type HandlerConfig struct {
	// Clock is used to control time.
	Clock clockwork.Clock
	// Emitter is the audit event emitter.
	Emitter apievents.Emitter
	// Authorizer authorizes requests.
	Authorizer authz.Authorizer
	// AccessPoint is the auth cache.
	AccessPoint services.AuthPreferenceGetter
	// ConnectionMonitor monitors connections and terminates any if
	// session controls prevent them.
	ConnectionMonitor ConnMonitor
	// GitServerGetter looks up git server resources.
	GitServerGetter GitServerGetter
	// ClusterName is the name of the local cluster.
	ClusterName string
	// HostID is the proxy host ID for audit events.
	HostID string
	// Logger is the logger.
	Logger *slog.Logger
}

func (c *HandlerConfig) CheckAndSetDefaults() error {
	if c.Emitter == nil {
		return trace.BadParameter("missing Emitter")
	}
	if c.Authorizer == nil {
		return trace.BadParameter("missing Authorizer")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.ConnectionMonitor == nil {
		return trace.BadParameter("missing ConnectionMonitor")
	}
	if c.GitServerGetter == nil {
		return trace.BadParameter("missing GitServerGetter")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("missing ClusterName")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.ComponentGit)
	}
	return nil
}

// Handler handles HTTP requests for Git HTTPS proxying.
//
// TODO(greedy52) add prometheus metrics.
type Handler struct {
	cfg            HandlerConfig
	authMiddleware *authz.Middleware
}

// NewHandler creates a new Git HTTP handler.
func NewHandler(ctx context.Context, cfg HandlerConfig) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		cfg: cfg,
		authMiddleware: &authz.Middleware{
			ClusterName:   cfg.ClusterName,
			AcceptedUsage: []string{teleport.UsageGitOnly},
		},
	}, nil
}

// ServeHTTP handles incoming Git HTTPS requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.serveHTTP(w, r); err != nil {
		switch {
		case errors.Is(err, context.Canceled), utils.IsOKNetworkError(err):
			h.cfg.Logger.DebugContext(r.Context(), "Git proxy request ended", "error", err)
		default:
			h.cfg.Logger.ErrorContext(r.Context(), "Git proxy request failed", "error", err)
		}
		trace.WriteError(w, err)
	}
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return trace.AccessDenied("missing client certificate")
	}

	// We can assume that r.Context() already has conn and client address values
	// from common web handler setup.
	conn, err := authz.ConnFromContext(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	clientAddr, err := authz.ClientSrcAddrFromContext(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	// Auth
	ctx, err := h.authMiddleware.WrapContextWithUserFromTLSConnState(r.Context(), *r.TLS, clientAddr)
	if err != nil {
		h.emitAuthFailureEvent(r.Context(), unverifiedUserMetadata(r.TLS), err)
		return trace.Wrap(err)
	}
	user, err := authz.UserFromContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	identity := user.GetIdentity()

	if identity.RouteToGit.Name == "" || identity.RouteToGit.SessionID == "" {
		return trace.AccessDenied("missing git routing information")
	}

	logger := h.cfg.Logger.With(
		"user", identity.Username,
		"git_server", identity.RouteToGit.Name,
		"session_id", identity.RouteToGit.SessionID,
	)
	authCtx, gitServer, err := h.authorize(ctx, identity.RouteToGit.Name)
	if err != nil {
		h.emitAuthFailureEvent(ctx, identity.GetUserMetadata(), err)
		return trace.Wrap(err)
	}

	// Monitor the connection for session controls
	ctx, _, err = h.cfg.ConnectionMonitor.MonitorConn(ctx, authCtx, conn)
	if err != nil {
		return trace.Wrap(err)
	}

	r = r.WithContext(ctx)
	switch gitServer.GetSubKind() {
	case types.SubKindGitHub:
		return h.handleGitHub(ctx, logger, w, r, &identity, gitServer)
	default:
		return trace.BadParameter("unsupported git server type %q", gitServer.GetSubKind())
	}
}

// unverifiedUserMetadata returns user metadata from the client certificate.
func unverifiedUserMetadata(tlsState *tls.ConnectionState) apievents.UserMetadata {
	cert := tlsState.PeerCertificates[0]
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return apievents.UserMetadata{User: "unknown"}
	}
	return identity.GetUserMetadata()
}

func (h *Handler) authorize(ctx context.Context, gitServerName string) (*authz.Context, types.Server, error) {
	authCtx, err := h.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	gitServer, err := h.cfg.GitServerGetter.GetGitServer(ctx, gitServerName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	authPref, err := h.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if err := authCtx.Checker.CheckAccess(gitServer, authCtx.GetAccessState(authPref)); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if !gitServerSupportsHTTPS(gitServer) {
		return nil, nil, trace.AccessDenied("git server %q does not allow HTTPS protocol", gitServerName)
	}

	return authCtx, gitServer, nil
}

func gitServerSupportsHTTPS(server types.Server) bool {
	if gh := server.GetGitHub(); gh != nil {
		return slices.Contains(gh.AllowProtocols, types.GitProtocolHTTPS)
	}
	return false
}

// emitAuthFailureEvent emits an auth attempt failure event.
func (h *Handler) emitAuthFailureEvent(ctx context.Context, userMetadata apievents.UserMetadata, err error) {
	if emitErr := h.cfg.Emitter.EmitAuditEvent(ctx, &apievents.AuthAttempt{
		Metadata: apievents.Metadata{
			Type: events.AuthAttemptEvent,
			Code: events.AuthAttemptFailureCode,
		},
		UserMetadata:       userMetadata,
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status: apievents.Status{
			Success: false,
			Error:   err.Error(),
		},
	}); emitErr != nil {
		h.cfg.Logger.WarnContext(ctx, "Failed to emit auth failure event", "error", emitErr)
	}
}
