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

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/uuid"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// HTTPHandlerConfig is the configuration for the git HTTP handler.
type HTTPHandlerConfig struct {
	// IntegrationsClient issues GitHub access tokens.
	IntegrationsClient IntegrationsClient
	// GitServerGetter looks up git server resources for audit metadata.
	GitServerGetter GitServerGetter
	// Authorizer authorizes requests.
	Authorizer authz.Authorizer
	// AccessPoint is the auth cache for auth preference lookups.
	AccessPoint services.AuthPreferenceGetter
	// ClusterName is the name of the local cluster.
	ClusterName string
	// TODO(greedy52) add ConnectionMonitor to terminate connections on
	// lock/cert expiry. Requires hijacking the connection similar to
	// lib/web/addr.go pattern.
	// TODO(greedy52) add ingress reporter for git HTTP connections.
	// Also needed for git SSH (lib/srv/ingress/reporter.go).
	// Emitter is the audit event emitter.
	Emitter apievents.Emitter
	// NewSessionRecorder creates a new session recorder for recording API
	// request/response pairs. If nil, API requests are not recorded.
	NewSessionRecorder func(ctx context.Context, sessionID string) (events.SessionPreparerRecorder, error)
	// HostID is the proxy host ID for audit events.
	HostID string
	// Logger is the logger.
	Logger *slog.Logger
}

// GitServerGetter looks up git server resources.
type GitServerGetter interface {
	GetGitServer(ctx context.Context, name string) (types.Server, error)
}

// IntegrationsClient generates GitHub access tokens.
type IntegrationsClient interface {
	GenerateGitHubAppToken(ctx context.Context, in *integrationv1.GenerateGitHubAppTokenRequest, opts ...grpc.CallOption) (*integrationv1.GenerateGitHubAppTokenResponse, error)
}

// HTTPHandler handles HTTP requests for Git HTTPS proxying (git clone/fetch/push
// and GitHub API). Auth is mTLS via client cert containing RouteToGit and
// UsageGitOnly. RBAC is checked on each request.
type HTTPHandler struct {
	cfg            HTTPHandlerConfig
	authMiddleware *authz.Middleware
	recorders      *utils.FnCache
	closeCtx       context.Context
	cancel         context.CancelFunc
}

// NewHTTPHandler creates a new Git HTTP handler. The closeCtx is used to
// control the handler's lifecycle -- when canceled, background goroutines
// stop and pending session recordings are flushed.
func NewHTTPHandler(closeCtx context.Context, cfg HTTPHandlerConfig) (*HTTPHandler, error) {
	if cfg.IntegrationsClient == nil {
		return nil, trace.BadParameter("missing IntegrationsClient")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, teleport.ComponentGit)
	}

	closeCtx, cancel := context.WithCancel(closeCtx)
	h := &HTTPHandler{
		cfg:      cfg,
		closeCtx: closeCtx,
		cancel:   cancel,
		authMiddleware: &authz.Middleware{
			ClusterName:   cfg.ClusterName,
			AcceptedUsage: []string{teleport.UsageGitOnly},
		},
	}

	// FnCache for session recorders -- one recorder per session ID.
	// When the entry expires (5 min), the recorder closes and uploads.
	recorders, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:             5 * time.Minute,
		CleanupInterval: 1 * time.Minute,
		Context:         context.Background(),
		OnExpiry: func(ctx context.Context, key any, value any) {
			cfg.Logger.DebugContext(ctx, "Session recorder expired, closing and uploading",
				"session_id", key,
			)
			if rec, ok := value.(events.SessionPreparerRecorder); ok {
				if err := rec.Close(ctx); err != nil {
					cfg.Logger.WarnContext(ctx, "Failed to close session recorder", "error", err)
				} else {
					cfg.Logger.DebugContext(ctx, "Session recorder closed successfully",
						"session_id", key,
					)
				}
			} else {
				cfg.Logger.WarnContext(ctx, "Expired value is not a session recorder",
					"session_id", key,
					"type", fmt.Sprintf("%T", value),
				)
			}
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	h.recorders = recorders

	// Background goroutine to periodically expire and upload session recordings.
	// FnCache cleanup is lazy (only on get()), so without this, recordings
	// would never upload after the last request.
	// TODO(greedy52) add jitter to avoid thundering herd when multiple proxies
	// expire recordings at the same time.
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.recorders.RemoveExpired()
			case <-h.closeCtx.Done():
				return
			}
		}
	}()

	return h, nil
}

// Close closes the handler and flushes any pending session recordings.
func (h *HTTPHandler) Close() error {
	h.cancel()
	h.recorders.Shutdown(context.Background())
	return nil
}

// ServeHTTP handles incoming Git HTTPS requests.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.serveHTTP(w, r); err != nil {
		h.cfg.Logger.ErrorContext(r.Context(), "Git proxy request failed", "error", err)
		trace.WriteError(w, err)
	}
}

func (h *HTTPHandler) serveHTTP(w http.ResponseWriter, r *http.Request) error {
	identity, err := getIdentityFromCert(r)
	if err != nil {
		return trace.Wrap(err)
	}

	if isBrowserRequest(r) {
		return trace.AccessDenied("browser access not supported for git proxy")
	}

	routeToGit := identity.RouteToGit
	if routeToGit.GitServerName == "" || routeToGit.SessionID == "" {
		return trace.AccessDenied("missing git routing information")
	}

	if err := validateUpstreamHost(r.Host); err != nil {
		return trace.Wrap(err)
	}

	ctx := r.Context()

	if err := h.authorize(ctx, r, routeToGit.GitServerName); err != nil {
		return trace.Wrap(err)
	}

	tokenResp, err := h.cfg.IntegrationsClient.GenerateGitHubAppToken(ctx, &integrationv1.GenerateGitHubAppTokenRequest{
		SessionId: routeToGit.SessionID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	switch r.Host {
	case "github.com":
		return h.handleGitCommand(ctx, w, r, identity, routeToGit, tokenResp.AccessToken)
	case "api.github.com":
		return h.handleAPI(ctx, w, r, identity, routeToGit, tokenResp.AccessToken)
	default:
		return trace.AccessDenied("unsupported host %q", r.Host)
	}
}

// authorize checks that the user has access to the git server.
func (h *HTTPHandler) authorize(ctx context.Context, r *http.Request, gitServerName string) error {
	if h.cfg.Authorizer == nil {
		return nil
	}

	clientAddr, err := utils.ParseAddr(r.RemoteAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, err = h.authMiddleware.WrapContextWithUserFromTLSConnState(ctx, *r.TLS, clientAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	authCtx, err := h.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	gitServer, err := h.cfg.GitServerGetter.GetGitServer(ctx, gitServerName)
	if err != nil {
		return trace.Wrap(err)
	}

	authPref, err := h.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := authCtx.GetAccessState(authPref)
	if err := authCtx.Checker.CheckAccess(gitServer, state); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// handleGitCommand handles git smart HTTP requests (clone/fetch/push) on github.com.
// All requests are recorded in session chunks. Actual git operations (POST)
// also emit separate GitCommand audit events.
func (h *HTTPHandler) handleGitCommand(ctx context.Context, w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, routeToGit tlsca.RouteToGit, accessToken string) error {
	if err := validateGitRequest(r); err != nil {
		return trace.Wrap(err)
	}

	service, repoPath := parseGitHTTPRequest(r)
	isDiscovery := strings.HasSuffix(r.URL.Path, "/info/refs")

	h.cfg.Logger.DebugContext(ctx, "Proxying git command",
		"user", identity.Username,
		"git_server", routeToGit.GitServerName,
		"method", r.Method,
		"path", r.URL.Path,
		"service", service,
		"discovery", isDiscovery,
	)

	// For push POST operations, tee the request body through the command
	// recorder to extract action details (ref updates).
	command := Command{
		SSHCommand: r.Method + " " + r.URL.Path,
		Service:    service,
		Repository: Repository(repoPath),
	}
	cmdRecorder := NewCommandRecorder(ctx, command)
	if r.Body != nil && service == "git-receive-pack" && r.Method == http.MethodPost {
		r.Body = io.NopCloser(io.TeeReader(r.Body, cmdRecorder))
	}

	rw := newResponseRecorder(w)
	h.forward(rw, r, accessToken)

	// Record all requests (including discovery) in session chunks.
	h.recordRequest(ctx, routeToGit, identity, r, rw.statusCode, "")

	// Emit separate GitCommand audit event for actual operations (not discovery).
	if !isDiscovery {
		h.emitGitCommandEventWithRecorder(ctx, identity, routeToGit, r, rw.statusCode, cmdRecorder)
	}
	return nil
}

// handleAPI handles GitHub API requests on api.github.com.
// Records request/response pairs in session chunks for audit.
func (h *HTTPHandler) handleAPI(ctx context.Context, w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, routeToGit tlsca.RouteToGit, accessToken string) error {
	h.cfg.Logger.DebugContext(ctx, "Proxying API request",
		"user", identity.Username,
		"git_server", routeToGit.GitServerName,
		"method", r.Method,
		"path", r.URL.Path,
	)

	r.Body = http.MaxBytesReader(w, r.Body, teleport.MaxHTTPRequestSize)

	var graphqlBuf *bytes.Buffer
	if r.Method == http.MethodPost && r.URL.Path == "/graphql" && r.Body != nil {
		graphqlBuf = &bytes.Buffer{}
		r.Body = io.NopCloser(io.TeeReader(r.Body, graphqlBuf))
	}

	rw := newResponseRecorder(w)
	h.forward(rw, r, accessToken)

	var graphqlQuery string
	if graphqlBuf != nil {
		graphqlQuery = graphqlBuf.String()
	}
	h.recordRequest(ctx, routeToGit, identity, r, rw.statusCode, graphqlQuery)
	return nil
}

// recordRequest records an HTTP request in the session chunk stream.
func (h *HTTPHandler) recordRequest(ctx context.Context, routeToGit tlsca.RouteToGit, identity *tlsca.Identity, r *http.Request, statusCode int, graphqlQuery string) {
	rec, err := h.getOrCreateRecorder(ctx, routeToGit.SessionID, identity, routeToGit)
	if err != nil {
		h.cfg.Logger.WarnContext(ctx, "Failed to get session recorder", "error", err)
	}

	requestEvent := &apievents.GitHTTPRequest{
		Metadata: apievents.Metadata{
			Type: events.GitHTTPRequestEvent,
			Code: events.GitHTTPRequestCode,
		},
		Method:       r.Method,
		RequestPath:  r.URL.Path,
		StatusCode:   uint32(statusCode),
		GraphQLQuery: graphqlQuery,
	}

	if rec != nil {
		if preparedEvent, err := rec.PrepareSessionEvent(requestEvent); err == nil {
			if err := rec.RecordEvent(ctx, preparedEvent); err != nil {
				h.cfg.Logger.WarnContext(ctx, "Failed to record request event", "error", err)
			}
		}
	} else if h.cfg.Emitter != nil {
		if err := h.cfg.Emitter.EmitAuditEvent(ctx, requestEvent); err != nil {
			h.cfg.Logger.WarnContext(ctx, "Failed to emit request event", "error", err)
		}
	}
}

// getOrCreateRecorder returns a session recorder for the given session ID,
// creating one if it doesn't exist. The recorder is cached and auto-closed
// when the cache entry expires.
func (h *HTTPHandler) getOrCreateRecorder(ctx context.Context, sessionID string, identity *tlsca.Identity, routeToGit tlsca.RouteToGit) (events.SessionPreparerRecorder, error) {
	if h.cfg.NewSessionRecorder == nil {
		return nil, nil
	}

	val, err := utils.FnCacheGet(ctx, h.recorders, sessionID, func(ctx context.Context) (events.SessionPreparerRecorder, error) {
		chunkID := uuid.NewString()
		rec, err := h.cfg.NewSessionRecorder(ctx, chunkID)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Emit session chunk event.
		if h.cfg.Emitter != nil {
			chunkEvent := &apievents.GitSessionChunk{
				Metadata: apievents.Metadata{
					Type: events.GitSessionChunkEvent,
					Code: events.GitSessionChunkCode,
				},
				ServerMetadata: apievents.ServerMetadata{
					ServerVersion: teleport.Version,
					ServerID:      h.cfg.HostID,
				},
				SessionMetadata: apievents.SessionMetadata{
					SessionID: sessionID,
				},
				UserMetadata:   identity.GetUserMetadata(),
				SessionChunkID: chunkID,
				GitMetadata:    h.getGitMetadata(ctx, routeToGit.GitServerName),
			}
			if err := h.cfg.Emitter.EmitAuditEvent(ctx, chunkEvent); err != nil {
				h.cfg.Logger.WarnContext(ctx, "Failed to emit session chunk event", "error", err)
			}
		}

		return rec, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return val, nil
}

func (h *HTTPHandler) forward(w http.ResponseWriter, r *http.Request, accessToken string) {
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "https"
			req.URL.Host = req.Host
			req.URL.User = nil
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			// Git HTTPS (github.com) uses Basic auth with x-access-token.
			// GitHub API (api.github.com) uses token auth.
			if req.Host == "github.com" {
				req.SetBasicAuth("x-access-token", accessToken)
			} else {
				req.Header.Set("Authorization", "token "+accessToken)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			h.cfg.Logger.ErrorContext(req.Context(), "Git proxy upstream error", "error", err)
			trace.WriteError(w, trace.ConnectionProblem(err, "upstream error"))
		},
	}
	proxy.ServeHTTP(w, r)
}

func getIdentityFromCert(r *http.Request) (*tlsca.Identity, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil, trace.AccessDenied("missing client certificate")
	}
	cert := r.TLS.PeerCertificates[0]
	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity, nil
}

func (h *HTTPHandler) getGitMetadata(ctx context.Context, gitServerName string) apievents.GitMetadata {
	meta := apievents.GitMetadata{
		GitServerName: gitServerName,
	}
	if h.cfg.GitServerGetter != nil {
		if server, err := h.cfg.GitServerGetter.GetGitServer(ctx, gitServerName); err == nil {
			if github := server.GetGitHub(); github != nil {
				meta.Organization = github.Organization
				meta.Integration = github.Integration
			}
		}
	}
	return meta
}

func isBrowserRequest(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Mode") != ""
}

// emitGitCommandEvent emits a GitCommand audit event for git HTTP operations.
// Matches the same fields as the SSH GitCommand event from forward.go.
func (h *HTTPHandler) emitGitCommandEventWithRecorder(ctx context.Context, identity *tlsca.Identity, routeToGit tlsca.RouteToGit, r *http.Request, statusCode int, rec CommandRecorder) {
	if h.cfg.Emitter == nil {
		return
	}

	cmd := rec.GetCommand()

	event := &apievents.GitCommand{
		Metadata: apievents.Metadata{
			Type: events.GitCommandEvent,
			Code: events.GitCommandCode,
		},
		UserMetadata: identity.GetUserMetadata(),
		SessionMetadata: apievents.SessionMetadata{
			SessionID: routeToGit.SessionID,
		},
		ServerMetadata: apievents.ServerMetadata{
			ServerVersion: teleport.Version,
			ServerID:      h.cfg.HostID,
		},
		CommandMetadata: apievents.CommandMetadata{
			Command:  cmd.SSHCommand,
			ExitCode: strconv.Itoa(statusCode),
		},
		ConnectionMetadata: apievents.ConnectionMetadata{
			RemoteAddr: r.RemoteAddr,
		},
		GitMetadata:    h.getGitMetadata(ctx, routeToGit.GitServerName),
		Service:        cmd.Service,
		Path:           string(cmd.Repository),
		HttpMethod:     r.Method,
		HttpStatusCode: uint32(statusCode),
	}

	if statusCode >= 400 {
		event.Metadata.Code = events.GitCommandFailureCode
	}

	actions, err := rec.GetActions()
	if err != nil {
		h.cfg.Logger.WarnContext(ctx, "Failed to get actions from git command recorder", "error", err)
	} else {
		event.Actions = actions
	}

	if err := h.cfg.Emitter.EmitAuditEvent(ctx, event); err != nil {
		h.cfg.Logger.WarnContext(ctx, "Failed to emit git command event", "error", err)
	}
}

// parseGitHTTPRequest extracts the git service and repo path from an HTTP
// request URL. Git smart HTTP uses paths like:
//   - /org/repo.git/info/refs?service=git-upload-pack
//   - /org/repo.git/git-upload-pack
//   - /org/repo.git/git-receive-pack
func parseGitHTTPRequest(r *http.Request) (service, repoPath string) {
	path := r.URL.Path

	// Check for info/refs discovery request.
	if strings.HasSuffix(path, "/info/refs") {
		repoPath = strings.TrimSuffix(path, "/info/refs")
		service = r.URL.Query().Get("service")
		return service, strings.TrimPrefix(repoPath, "/")
	}

	// Check for git-upload-pack or git-receive-pack.
	for _, svc := range []string{"git-upload-pack", "git-receive-pack"} {
		if strings.HasSuffix(path, "/"+svc) {
			repoPath = strings.TrimSuffix(path, "/"+svc)
			return svc, strings.TrimPrefix(repoPath, "/")
		}
	}

	return "https", strings.TrimPrefix(path, "/")
}

// validateGitRequest ensures the request is a valid git smart HTTP request.
// Only git-upload-pack (fetch/clone) and git-receive-pack (push) are allowed.
func validateGitRequest(r *http.Request) error {
	path := r.URL.Path

	// Allow info/refs discovery with valid service param.
	if strings.HasSuffix(path, "/info/refs") {
		service := r.URL.Query().Get("service")
		switch service {
		case "git-upload-pack", "git-receive-pack":
			return nil
		default:
			return trace.AccessDenied("unsupported git service %q", service)
		}
	}

	// Allow git-upload-pack and git-receive-pack POST endpoints.
	if strings.HasSuffix(path, "/git-upload-pack") || strings.HasSuffix(path, "/git-receive-pack") {
		return nil
	}

	return trace.AccessDenied("request path %q is not a valid git operation", path)
}

// responseRecorder wraps http.ResponseWriter to capture the status code.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

var allowedGitHubHosts = map[string]bool{
	"github.com":     true,
	"api.github.com": true,
}

// TODO(greedy52) support self-hosted GitHub/GitLab with custom hostnames.
func validateUpstreamHost(host string) error {
	if !allowedGitHubHosts[host] {
		return trace.AccessDenied("upstream host %q is not allowed", host)
	}
	return nil
}
