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

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

// mcpOAuthCredentials is everything tsh needs to authorize requests to an
// OAuth-protected MCP server: the token itself plus the client ID it was
// issued to, which is required to refresh the token later.
type mcpOAuthCredentials struct {
	ClientID string                   `json:"client_id"`
	Token    mcpclienttransport.Token `json:"token"`
}

// mcpOAuthTokenPath returns where the OAuth credentials for the
// (proxy, cluster, app) combination are stored, e.g.
// ~/.tsh/mcp_tokens/example.com/mycluster/linear.json.
func mcpOAuthTokenPath(homePath, proxyHost, cluster, appName string) string {
	return filepath.Join(profile.FullProfilePath(homePath), "mcp_tokens", proxyHost, cluster, appName+".json")
}

// saveMCPOAuthCredentials writes the credentials with owner-only permissions.
// The write is atomic (temp file + rename) so that concurrent readers never
// observe a partially written file.
func saveMCPOAuthCredentials(path string, creds *mcpOAuthCredentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer os.Remove(tmp.Name()) // no-op after successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return trace.ConvertSystemError(err)
	}
	if err := tmp.Close(); err != nil {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(os.Rename(tmp.Name(), path))
}

// loadMCPOAuthCredentials returns trace.NotFound when no credentials are
// stored at the path.
func loadMCPOAuthCredentials(path string) (*mcpOAuthCredentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var creds mcpOAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, trace.Wrap(err)
	}
	return &creds, nil
}

// fileTokenStore exposes the credentials file through mcp-go's TokenStore
// interface, so that its OAuthHandler reads the current token on every
// request and writes refreshed tokens back to the same file.
type fileTokenStore struct {
	path     string
	clientID string
}

func (s *fileTokenStore) GetToken(ctx context.Context) (*mcpclienttransport.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	creds, err := loadMCPOAuthCredentials(s.path)
	if trace.IsNotFound(err) {
		return nil, mcpclienttransport.ErrNoToken
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &creds.Token, nil
}

func (s *fileTokenStore) SaveToken(ctx context.Context, token *mcpclienttransport.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return trace.Wrap(saveMCPOAuthCredentials(s.path, &mcpOAuthCredentials{
		ClientID: s.clientID,
		Token:    *token,
	}))
}

// newMCPOAuthGetAuthHeader returns an Authorization header source backed by
// the app's stored OAuth credentials, or nil if the user has not run
// `tsh mcp login` for this app.
func newMCPOAuthGetAuthHeader(dialer *client.MCPServerDialer, homePath, proxyHost, cluster, appName string) (func(context.Context) (string, error), error) {
	credsPath := mcpOAuthTokenPath(homePath, proxyHost, cluster, appName)
	creds, err := loadMCPOAuthCredentials(credsPath)
	if trace.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	httpClient, err := newMCPOAuthHTTPClient(dialer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oauthHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		ClientID:    creds.ClientID,
		PKCEEnabled: true,
		HTTPClient:  httpClient,
		TokenStore:  &fileTokenStore{path: credsPath, clientID: creds.ClientID},
	})
	oauthHandler.SetBaseURL("http://localhost")
	source := &mcpOAuthHeaderSource{
		appName:   appName,
		credsPath: credsPath,
		refresh:   oauthHandler.RefreshToken,
	}
	return source.GetAuthHeader, nil
}

// mcpOAuthRefreshLockTimeout is how long a process waits for the refresh
// lock. It matches the refresh HTTP client timeout: a process waiting on the
// lock should outlast the lock holder's worst-case refresh rather than give
// up early.
const mcpOAuthRefreshLockTimeout = 30 * time.Second

// mcpOAuthHeaderSource produces the Authorization header for requests to an
// OAuth-protected MCP server, refreshing the stored token when it expires.
//
// Concurrent tsh processes (one per MCP client) share one refresh token,
// which the authorization server may rotate on use: refreshing twice with
// the same refresh token wedges the loser. So refresh is single-flight
// across processes: take an exclusive file lock, re-read the file (another
// process may have refreshed while we waited), and only refresh if the
// stored token is still expired. Same pattern as tsh's kube credentials
// lock and known_hosts locking.
type mcpOAuthHeaderSource struct {
	appName   string
	credsPath string
	// refresh exchanges a refresh token for a new token and persists it.
	// Wired to mcp-go's OAuthHandler.RefreshToken, which saves through
	// fileTokenStore.
	refresh func(context.Context, string) (*mcpclienttransport.Token, error)
}

func (s *mcpOAuthHeaderSource) GetAuthHeader(ctx context.Context) (string, error) {
	// Fast path: a valid token on disk, no locking.
	creds, err := loadMCPOAuthCredentials(s.credsPath)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if !creds.Token.IsExpired() && creds.Token.AccessToken != "" {
		return bearerAuthHeader(&creds.Token), nil
	}

	unlock, err := utils.FSTryWriteLockTimeout(ctx, s.credsPath+".lock", mcpOAuthRefreshLockTimeout)
	if err != nil {
		return "", trace.Wrap(err, "waiting for the MCP OAuth token refresh lock")
	}
	defer unlock()

	// Re-check under the lock: if another process already refreshed while
	// we waited, use its token instead of refreshing again.
	creds, err = loadMCPOAuthCredentials(s.credsPath)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if !creds.Token.IsExpired() && creds.Token.AccessToken != "" {
		return bearerAuthHeader(&creds.Token), nil
	}
	if creds.Token.RefreshToken == "" {
		return "", trace.Wrap(&mcpOAuthLoginRequiredError{appName: s.appName})
	}

	token, err := s.refresh(ctx, creds.Token.RefreshToken)
	if err != nil {
		return "", trace.Wrap(&mcpOAuthLoginRequiredError{appName: s.appName, reason: err})
	}
	return bearerAuthHeader(token), nil
}

// mcpOAuthLoginRequiredError means the stored OAuth token can no longer be
// used (no refresh token, or the refresh was rejected) and the user has to
// run `tsh mcp login` again. makeMCPReconnectUserMessage recognizes it to
// show the exact fix command in the MCP client.
type mcpOAuthLoginRequiredError struct {
	appName string
	reason  error
}

func (e *mcpOAuthLoginRequiredError) Error() string {
	msg := fmt.Sprintf("authentication with MCP server %q has expired, run `tsh mcp login %s` to log in again", e.appName, e.appName)
	if e.reason != nil {
		msg += ": " + e.reason.Error()
	}
	return msg
}

func (e *mcpOAuthLoginRequiredError) Unwrap() error { return e.reason }

// bearerAuthHeader formats the Authorization header value. Per RFC 6749 §5.1
// token_type is case-insensitive; normalize to "Bearer" for strict servers,
// same as mcp-go does.
func bearerAuthHeader(token *mcpclienttransport.Token) string {
	tokenType := token.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return tokenType + " " + token.AccessToken
}

// mcpOAuthProxyMiddleware injects the OAuth token stored by `tsh mcp login`
// into requests forwarded by `tsh proxy mcp`, same as `tsh mcp connect` does
// for the stdio path. Without it, the upstream's 401 passes through and
// OAuth-capable clients start their own OAuth flow against the local port,
// which fails the spec's resource-URL validation (the discovery metadata
// names the real server, not localhost).
type mcpOAuthProxyMiddleware struct {
	alpnproxy.DefaultLocalProxyHTTPMiddleware
	getAuthHeader func(context.Context) (string, error)
}

// newMCPOAuthProxyMiddleware returns the injection middleware for the app,
// or nil if the user has not run `tsh mcp login` for it (callers should keep
// the plain tunnel behavior).
func newMCPOAuthProxyMiddleware(tc *client.TeleportClient, homePath, appName string) (*mcpOAuthProxyMiddleware, error) {
	dialer := client.NewMCPServerDialer(tc, appName)
	getAuthHeader, err := newMCPOAuthGetAuthHeader(dialer, homePath, tc.WebProxyHost(), tc.SiteName, appName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if getAuthHeader == nil {
		return nil, nil
	}
	return &mcpOAuthProxyMiddleware{getAuthHeader: getAuthHeader}, nil
}

func (m *mcpOAuthProxyMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	// A client supplying its own credentials wins, same as -H on connect.
	if req.Header.Get("Authorization") != "" {
		return false
	}
	header, err := m.getAuthHeader(req.Context())
	if err != nil {
		// 403 instead of 401 on purpose: a 401 would trigger OAuth-capable
		// clients into their own (doomed) OAuth flow instead of showing the
		// actionable message.
		http.Error(rw, makeMCPReconnectUserMessage(err), http.StatusForbidden)
		return true
	}
	req.Header.Set("Authorization", header)
	return false
}
