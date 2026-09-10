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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/client"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	authorizationHeader         = "Authorization"
	mcpOAuthMutationLockTimeout = 30 * time.Second
)

// mcpOAuthCredentials is what `tsh mcp login` stores for one MCP server: the
// OAuth token plus the client registration needed to refresh it or repeat the
// login without registering again. ResourceURI and Issuer bind the
// credentials to the server and authorization server they were issued for,
// so they cannot follow an app name to a replacement upstream.
type mcpOAuthCredentials struct {
	ResourceURI  string                   `json:"resource_uri"`
	Issuer       string                   `json:"issuer"`
	ClientID     string                   `json:"client_id"`
	ClientSecret string                   `json:"client_secret,omitempty"`
	RedirectURI  string                   `json:"redirect_uri,omitempty"`
	Scopes       []string                 `json:"scopes,omitempty"`
	Token        mcpclienttransport.Token `json:"token"`
}

func (c *mcpOAuthCredentials) checkResource(resourceURI string) error {
	if c.ResourceURI == "" || c.Issuer == "" {
		return trace.BadParameter("stored MCP OAuth credentials are missing their resource or issuer binding")
	}
	if c.ResourceURI != resourceURI {
		return trace.BadParameter("stored MCP OAuth credentials are bound to a different resource")
	}
	return nil
}

func (c *mcpOAuthCredentials) checkBinding(resourceURI, issuer string) error {
	if err := c.checkResource(resourceURI); err != nil {
		return err
	}
	if c.Issuer != issuer {
		return trace.BadParameter("stored MCP OAuth credentials are bound to a different issuer")
	}
	return nil
}

func mcpOAuthTokenPath(homePath, proxyHost, username, cluster string, appSQN scopes.QualifiedName) (string, error) {
	baseDir := profile.FullProfilePath(homePath)
	appName := client.ScopedAppName(appSQN)
	path := keypaths.MCPOAuthCredentialsPath(baseDir, proxyHost, username, cluster, appName)
	// The app name is user input; reject path separators and directory escapes.
	if strings.ContainsAny(appName, `/\`) || filepath.Dir(path) != keypaths.MCPOAuthCredentialDir(baseDir, proxyHost, username, cluster) {
		return "", trace.BadParameter("invalid MCP server name %q", appSQN.String())
	}
	return path, nil
}

func mcpOAuthMutationLockPath(homePath string) string {
	return keypaths.MCPOAuthCredentialsLockPath(profile.FullProfilePath(homePath))
}

func saveMCPOAuthCredentials(path string, creds *mcpOAuthCredentials) error {
	if creds.ResourceURI == "" || creds.Issuer == "" {
		return trace.BadParameter("MCP OAuth credentials are missing their resource or issuer binding")
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(writeMCPOAuthCredentialsFile(path, data))
}

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

type fileTokenStore struct {
	path             string
	mutationLockPath string
	resourceURI      string
	issuer           string
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
	if err := creds.checkBinding(s.resourceURI, s.issuer); err != nil {
		return nil, trace.Wrap(err)
	}
	return &creds.Token, nil
}

func (s *fileTokenStore) SaveToken(ctx context.Context, token *mcpclienttransport.Token) error {
	// The provider may have rotated the refresh token when issuing this one,
	// so it must be stored even if the request that triggered the refresh has
	// been abandoned. The lock wait is bounded.
	ctx = context.WithoutCancel(ctx)
	return withMCPOAuthMutationLock(ctx, s.mutationLockPath, func() error {
		creds, err := loadMCPOAuthCredentials(s.path)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := creds.checkBinding(s.resourceURI, s.issuer); err != nil {
			return trace.Wrap(err)
		}
		creds.Token = *token
		return trace.Wrap(saveMCPOAuthCredentials(s.path, creds))
	})
}

func withMCPOAuthMutationLock(ctx context.Context, path string, fn func() error) error {
	if path == "" {
		return trace.BadParameter("missing MCP OAuth mutation lock path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return trace.ConvertSystemError(err)
	}
	unlock, err := utils.FSTryWriteLockTimeout(ctx, path, mcpOAuthMutationLockTimeout)
	if err != nil {
		return trace.Wrap(err, "waiting to store MCP OAuth credentials")
	}
	defer unlock()
	return trace.Wrap(fn())
}

func newMCPOAuthHeaderSource(dialer *client.MCPServerDialer, credsPath, mutationLockPath, appName string, reauthorize mcpOAuthReauthorizeFunc) *mcpOAuthHeaderSource {
	return &mcpOAuthHeaderSource{
		appName:     appName,
		credsPath:   credsPath,
		dialer:      dialer,
		reauthorize: reauthorize,
		refresh: func(ctx context.Context, creds *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
			app, err := dialer.GetApp(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			resourceURI := app.GetURI()
			oauthBaseURL, err := mcpOAuthDiscoveryBaseURL(resourceURI)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			httpClient, err := newMCPOAuthHTTPClient(dialer, resourceURI)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			oauthHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
				ClientID:     creds.ClientID,
				ClientSecret: creds.ClientSecret,
				PKCEEnabled:  true,
				HTTPClient:   httpClient,
				TokenStore: &fileTokenStore{
					path:             credsPath,
					mutationLockPath: mutationLockPath,
					resourceURI:      resourceURI,
					issuer:           creds.Issuer,
				},
			})
			oauthHandler.SetBaseURL(oauthBaseURL)
			metadata, err := getMCPOAuthServerMetadata(ctx, oauthHandler)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if err := creds.checkBinding(resourceURI, metadata.Issuer); err != nil {
				return nil, trace.Wrap(err)
			}
			return oauthHandler.RefreshToken(ctx, creds.Token.RefreshToken)
		},
	}
}

// mcpOAuthHeaderSource produces the Authorization header for an MCP server
// from the credentials stored by `tsh mcp login`, refreshing the token or
// re-running the browser login when it expires. Without usable stored
// credentials it produces no header and lets the server's 401 ask for a
// login. Refresh runs under a cross-process file lock because providers may
// rotate the refresh token on use, so two tsh processes must not both spend it.
type mcpOAuthHeaderSource struct {
	appName     string
	credsPath   string
	dialer      *client.MCPServerDialer
	reauthorize mcpOAuthReauthorizeFunc
	// reauthorizeFailed stops this process from opening another browser after
	// an automatic login already failed; the user is told to log in instead.
	reauthorizeFailed atomic.Bool
	refresh           func(context.Context, *mcpOAuthCredentials) (*mcpclienttransport.Token, error)
}

func (s *mcpOAuthHeaderSource) canReauthorize() bool {
	return s.reauthorize != nil && !s.reauthorizeFailed.Load()
}

// loadCredentials returns the credentials stored for the app's current
// resource URI. A missing, unreadable, or mismatched file is a login-required
// error; a failed app lookup is returned as is so relogin handling sees it.
func (s *mcpOAuthHeaderSource) loadCredentials(ctx context.Context) (*mcpOAuthCredentials, error) {
	creds, err := loadMCPOAuthCredentials(s.credsPath)
	if err != nil {
		return nil, trace.Wrap(&mcpOAuthLoginRequiredError{appName: s.appName, reason: err})
	}
	app, err := s.dialer.GetApp(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := creds.checkResource(app.GetURI()); err != nil {
		return nil, trace.Wrap(&mcpOAuthLoginRequiredError{appName: s.appName, reason: err})
	}
	return creds, nil
}

func (s *mcpOAuthHeaderSource) GetAuthHeader(ctx context.Context) (string, error) {
	creds, err := s.loadCredentials(ctx)
	var loginRequired *mcpOAuthLoginRequiredError
	if errors.As(err, &loginRequired) {
		// Nothing usable is stored: send the request without a token and let
		// the server's 401 come back through RefreshAuthHeader.
		return "", nil
	}
	if err != nil {
		return "", trace.Wrap(err)
	}
	if !creds.Token.IsExpired() && creds.Token.AccessToken != "" {
		return bearerAuthHeader(&creds.Token), nil
	}
	return s.refreshAuthHeader(ctx, "")
}

func (s *mcpOAuthHeaderSource) RefreshAuthHeader(ctx context.Context, rejectedHeader string) (string, error) {
	return s.refreshAuthHeader(ctx, rejectedHeader)
}

func (s *mcpOAuthHeaderSource) refreshAuthHeader(ctx context.Context, rejectedHeader string) (string, error) {
	// The wait outlasts a browser flow in another process, so a waiter gets
	// that process's token instead of opening a second browser.
	unlock, err := utils.FSTryWriteLockTimeout(ctx, s.credsPath+".lock", mcpOAuthAuthorizationTimeout+mcpOAuthRefreshLockTimeout)
	if err != nil {
		return "", trace.Wrap(err, "waiting for the MCP OAuth token refresh lock")
	}
	defer unlock()

	// A waiter must not reuse a rotating refresh token already spent by the
	// previous lock holder.
	creds, err := s.loadCredentials(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	currentHeader := bearerAuthHeader(&creds.Token)
	if !creds.Token.IsExpired() && creds.Token.AccessToken != "" &&
		(rejectedHeader == "" || currentHeader != rejectedHeader) {
		return currentHeader, nil
	}
	var refreshErr error
	if creds.Token.RefreshToken != "" {
		if s.refresh == nil {
			refreshErr = trace.BadParameter("missing MCP OAuth token refresh function")
		} else if token, err := s.refresh(ctx, creds); err != nil {
			refreshErr = err
		} else if token == nil || token.AccessToken == "" || token.IsExpired() {
			refreshErr = trace.BadParameter("MCP OAuth token refresh did not produce a valid access token")
		} else {
			return bearerAuthHeader(token), nil
		}
	}

	if !s.canReauthorize() {
		return "", trace.Wrap(&mcpOAuthLoginRequiredError{appName: s.appName, reason: refreshErr})
	}

	if err := s.reauthorize(ctx, creds); err != nil {
		// A login the caller abandoned says nothing about the next one.
		if ctx.Err() == nil {
			s.reauthorizeFailed.Store(true)
		}
		if refreshErr != nil {
			err = trace.Wrap(err, "refreshing the MCP OAuth token also failed: %v", refreshErr)
		}
		return "", trace.Wrap(&mcpOAuthLoginRequiredError{appName: s.appName, reason: err})
	}

	creds, err = s.loadCredentials(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if creds.Token.IsExpired() || creds.Token.AccessToken == "" {
		return "", trace.Wrap(&mcpOAuthLoginRequiredError{
			appName: s.appName,
			reason:  trace.BadParameter("automatic login did not produce a valid access token"),
		})
	}
	return bearerAuthHeader(&creds.Token), nil
}

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

func bearerAuthHeader(token *mcpclienttransport.Token) string {
	tokenType := token.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return tokenType + " " + token.AccessToken
}

// mcpOAuthProxyMiddleware adds the stored OAuth token to requests passing
// through `tsh proxy mcp`, so MCP clients talking to the local proxy never
// see a 401 and never try to run their own OAuth flow against the proxy URL.
type mcpOAuthProxyMiddleware struct {
	alpnproxy.DefaultLocalProxyHTTPMiddleware
	appName           string
	getAuthHeader     func(context.Context) (string, error)
	refreshAuthHeader func(context.Context, string) (string, error)
}

// mcpOAuthManagedContextKey marks a request whose client sent no
// Authorization header, so any 401 it gets back is tsh's to handle.
type mcpOAuthManagedContextKey struct{}

func newMCPOAuthProxyMiddleware(source *mcpOAuthHeaderSource) *mcpOAuthProxyMiddleware {
	return &mcpOAuthProxyMiddleware{
		appName:           source.appName,
		getAuthHeader:     source.GetAuthHeader,
		refreshAuthHeader: source.RefreshAuthHeader,
	}
}

// HandleRequest implements [alpnproxy.LocalProxyHTTPMiddleware].
func (m *mcpOAuthProxyMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) (handled bool) {
	// Preserve ownership of credentials supplied by the client.
	if req.Header.Get(authorizationHeader) != "" {
		return false
	}
	header, err := m.getAuthHeader(req.Context())
	if err != nil {
		// A 401 would trigger another OAuth flow against the local proxy URL.
		http.Error(rw, makeMCPReconnectUserMessage(err), http.StatusForbidden)
		return true
	}
	if header != "" {
		req.Header.Set(authorizationHeader, header)
	}
	*req = *req.WithContext(context.WithValue(req.Context(), mcpOAuthManagedContextKey{}, true))
	return false
}

// WrapRoundTripper implements [alpnproxy.LocalProxyHTTPTransportMiddleware].
func (m *mcpOAuthProxyMiddleware) WrapRoundTripper(base http.RoundTripper) http.RoundTripper {
	return &mcpOAuthProxyRetryRoundTripper{
		appName: m.appName,
		base:    base,
		retry:   clientmcp.NewOAuthRetryRoundTripper(base, m.refreshAuthHeader),
	}
}

// mcpOAuthProxyRetryRoundTripper refreshes and retries only requests whose
// authorization tsh manages, and answers any 401 left after that with a 403
// that tells the user to run `tsh mcp login`.
type mcpOAuthProxyRetryRoundTripper struct {
	appName string
	base    http.RoundTripper
	retry   http.RoundTripper
}

func (t *mcpOAuthProxyRetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Context().Value(mcpOAuthManagedContextKey{}) != true {
		return t.base.RoundTrip(req)
	}
	resp, err := t.retry.RoundTrip(req)
	var loginRequired *mcpOAuthLoginRequiredError
	if errors.As(err, &loginRequired) {
		return mcpOAuthProxyRefreshErrorResponse(req, err), nil
	}
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		if resp.Body != nil {
			resp.Body.Close()
		}
		return mcpOAuthProxyRefreshErrorResponse(req, &mcpOAuthLoginRequiredError{
			appName: t.appName,
			reason:  trace.AccessDenied("the MCP server rejected the request with HTTP 401"),
		}), nil
	}
	return resp, err
}

func mcpOAuthProxyRefreshErrorResponse(req *http.Request, err error) *http.Response {
	message := makeMCPReconnectUserMessage(err) + "\n"
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", http.StatusForbidden, http.StatusText(http.StatusForbidden)),
		StatusCode:    http.StatusForbidden,
		Header:        http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		Body:          io.NopCloser(strings.NewReader(message)),
		ContentLength: int64(len(message)),
		Request:       req,
	}
}
