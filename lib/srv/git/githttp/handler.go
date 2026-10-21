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
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
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

// AccessPoint is the cache interface needed by the git HTTP handler.
type AccessPoint interface {
	services.AuthPreferenceGetter
	GetSessionRecordingConfig(context.Context) (types.SessionRecordingConfig, error)
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
	AccessPoint AccessPoint
	// ConnectionMonitor monitors connections and terminates any if
	// session controls prevent them.
	ConnectionMonitor ConnMonitor
	// GitServerGetter looks up git server resources.
	GitServerGetter GitServerGetter
	// ClusterName is the name of the local cluster.
	ClusterName string
	// HostID is the proxy host ID for audit events.
	HostID string
	// UpstreamAuth provides access tokens for upstream git providers.
	UpstreamAuth UpstreamAuth
	// Transport is the HTTP transport for upstream requests. If nil,
	// http.DefaultTransport is used.
	Transport http.RoundTripper
	// AuthClient is used for streaming session recordings to the auth server.
	AuthClient events.Streamer
	// DataDir is the data directory for temporary session recording storage.
	DataDir string
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
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.DataDir == "" {
		return trace.BadParameter("missing DataDir")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.UpstreamAuth == nil {
		c.UpstreamAuth = upstreamAuth{}
	}
	if c.Transport == nil {
		tr, err := defaults.Transport()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Transport = tr
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
	sessionChunks  *utils.FnCache
	closeContext   context.Context
	closeCancel    context.CancelFunc
	// closeWG tracks background work that Close waits for, like closing
	// expired session chunks.
	closeWG sync.WaitGroup
	// sessionChunkLimiter limits the rate of new session chunks per user to
	// limit memory usage.
	sessionChunkLimiter *limiter.RateLimiter

	// newSessionRecorderFn creates a session recorder. Defaults to
	// newSessionRecorder.
	newSessionRecorderFn func(ctx context.Context, chunkID string) (events.SessionPreparerRecorder, error)
}

// NewHandler creates a new Git HTTP handler.
func NewHandler(ctx context.Context, cfg HandlerConfig) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Each live session chunk holds a session recorder with several hundred KiB
	// of buffers for several minutes. With 20 new chunks per minute and a burst
	// of 40, a single user holds at most ~160 live chunks.
	sessionChunkLimiter, err := limiter.NewRateLimiter(limiter.Config{
		Rates: []limiter.Rate{{
			Period:  defaults.LimiterPeriod,
			Average: defaults.LimiterAverage,
			Burst:   defaults.LimiterBurst,
		}},
		Clock: cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeContext, closeCancel := context.WithCancel(ctx)
	h := &Handler{
		cfg: cfg,
		authMiddleware: &authz.Middleware{
			ClusterName:   cfg.ClusterName,
			AcceptedUsage: []string{teleport.UsageGitOnly},
		},
		closeContext:        closeContext,
		closeCancel:         closeCancel,
		sessionChunkLimiter: sessionChunkLimiter,
	}
	h.newSessionRecorderFn = h.newSessionRecorder

	sessionChunks, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   5 * time.Minute,
		Clock: cfg.Clock,
		OnExpiry: func(ctx context.Context, _ any, val any) {
			if chunk, ok := val.(*sessionChunk); ok {
				h.closeWG.Go(func() {
					if err := chunk.close(); err != nil {
						cfg.Logger.WarnContext(ctx, "Failed to close session recorder on expiry", "error", err)
					}
				})
			}
		},
		Context:     closeContext,
		ReloadOnErr: true,
	})
	if err != nil {
		closeCancel()
		return nil, trace.Wrap(err)
	}
	h.sessionChunks = sessionChunks

	// FnCache only removes expired entries when accessed, so remove them
	// periodically to close session chunks of idle sessions.
	h.closeWG.Go(func() {
		cleanup := interval.New(interval.Config{
			Duration: time.Minute,
			Jitter:   retryutils.SeventhJitter,
			Clock:    cfg.Clock,
		})
		defer cleanup.Stop()
		for {
			select {
			case <-cleanup.Next():
				sessionChunks.RemoveExpired()
			case <-closeContext.Done():
				return
			}
		}
	})
	return h, nil
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
	ctx, monitoredConn, err := h.cfg.ConnectionMonitor.MonitorConn(ctx, authCtx, conn)
	if err != nil {
		return trace.Wrap(err)
	}
	// Track client activity at the HTTP level, so long-running clones and pushes
	// are not treated as idle.
	if tracker, ok := monitoredConn.(activityTracker); ok {
		w, r = trackClientActivity(tracker, w, r)
	}

	// TODO(greedy52) to align with RFD 0341, rename git.session.start to
	// git.access and emit it here.

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
	identity, err := tlsca.IdentityFromCert(cert)
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
		return types.IsGitProtocolAllowed(gh.AllowProtocols, types.GitProtocolHTTPS)
	}
	return false
}

// Close flushes and closes all cached session chunks.
func (h *Handler) Close() error {
	h.closeCancel()
	h.sessionChunks.Shutdown(context.WithoutCancel(h.closeContext))
	h.closeWG.Wait()
	return nil
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

type activityTracker interface {
	UpdateClientActivity()
}

func trackClientActivity(tracker activityTracker, w http.ResponseWriter, r *http.Request) (http.ResponseWriter, *http.Request) {
	r.Body = &activityReadCloser{
		ReadCloser: r.Body,
		tracker:    tracker,
	}
	return &activityResponseWriter{
		ResponseWriter: w,
		tracker:        tracker,
	}, r
}

type activityReadCloser struct {
	io.ReadCloser
	tracker activityTracker
}

func (r *activityReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.tracker.UpdateClientActivity()
	return n, err
}

type activityResponseWriter struct {
	http.ResponseWriter
	tracker activityTracker
}

func (w *activityResponseWriter) Write(p []byte) (int, error) {
	w.tracker.UpdateClientActivity()
	return w.ResponseWriter.Write(p)
}

// Unwrap allows http.ResponseController to reach the underlying writer.
func (w *activityResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
