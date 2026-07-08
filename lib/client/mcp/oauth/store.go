/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// expirySkew treats tokens expiring within this window as already stale,
	// so a token can't die between the check and the upstream request.
	expirySkew = 30 * time.Second
	// refreshLockTimeout bounds how long a process waits for another
	// process's in-flight refresh.
	refreshLockTimeout = 30 * time.Second
)

// ErrLoginRequired indicates there is no usable OAuth credential for the app
// and the user must run `tsh mcp login <app>`.
var ErrLoginRequired = errors.New("MCP OAuth login required")

// Credentials is the OAuth state persisted for one MCP app: the token set
// plus everything needed to refresh it without re-running discovery.
type Credentials struct {
	// Token is the OAuth token set (access, refresh, expiry).
	Token *mcpclienttransport.Token `json:"token"`
	// ClientID is the client ID obtained via dynamic client registration.
	ClientID string `json:"client_id"`
	// ClientSecret is set only if the AS issued one during registration.
	ClientSecret string `json:"client_secret,omitempty"`
	// TokenEndpoint is the authorization server's token endpoint, cached at
	// login so refreshes skip metadata discovery.
	TokenEndpoint string `json:"token_endpoint"`
	// Resource is the RFC 8707 resource indicator sent on refresh requests.
	Resource string `json:"resource,omitempty"`
	// UpstreamURL is the MCP app upstream URL these credentials were issued for.
	UpstreamURL string `json:"upstream_url,omitempty"`
	// Issuer is the OAuth authorization server issuer used at login.
	Issuer string `json:"issuer,omitempty"`
}

// Store persists Credentials for one (proxy, cluster, app) on disk and hands
// out valid access tokens, refreshing them under a cross-process file lock.
type Store struct {
	tokenPath           string
	lockPath            string
	httpClient          *http.Client
	expectedUpstreamURL string
}

// NewStore returns a store backed by tokenPath, using lockPath to serialize
// refreshes across processes. httpClient is used for refresh requests and
// must route app-host traffic through the Teleport tunnel (see NewHTTPClient);
// pass nil if the store is only used for Save/Load.
func NewStore(tokenPath, lockPath string, httpClient *http.Client) *Store {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Store{tokenPath: tokenPath, lockPath: lockPath, httpClient: httpClient}
}

// SetExpectedUpstreamURL binds subsequent GetValidAuthHeader calls to the
// current app upstream URL. A mismatch forces re-login instead of sending a
// bearer token minted for a previous app configuration to a new upstream.
func (s *Store) SetExpectedUpstreamURL(upstreamURL string) {
	s.expectedUpstreamURL = upstreamURL
}

// Load reads the credentials. A missing file maps to ErrLoginRequired.
func (s *Store) Load() (*Credentials, error) {
	data, err := os.ReadFile(s.tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, trace.Wrap(ErrLoginRequired, "no stored OAuth credentials at %s", s.tokenPath)
		}
		return nil, trace.ConvertSystemError(err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, trace.Wrap(err, "parsing %s", s.tokenPath)
	}
	if creds.Token == nil {
		return nil, trace.Wrap(ErrLoginRequired, "stored credentials at %s have no token", s.tokenPath)
	}
	return &creds, nil
}

// Save writes the credentials atomically (temp file + rename) with 0600 perms.
func (s *Store) Save(creds *Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}
	dir := filepath.Dir(s.tokenPath)
	if err := ensurePrivateDir(dir); err != nil {
		return trace.Wrap(err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.tokenPath)+".tmp-*")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return trace.ConvertSystemError(err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return trace.ConvertSystemError(err)
	}
	if err := tmp.Close(); err != nil {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(os.Rename(tmp.Name(), s.tokenPath))
}

// SaveLocked writes credentials while holding the same lock used by refreshes.
// It prevents a concurrent connect refresh from overwriting a just-completed
// interactive login with older token state.
func (s *Store) SaveLocked(ctx context.Context, creds *Credentials) error {
	if err := ensurePrivateDir(filepath.Dir(s.tokenPath)); err != nil {
		return trace.Wrap(err)
	}
	unlock, err := utils.FSTryWriteLockTimeout(ctx, s.lockPath, refreshLockTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	defer unlock()
	return trace.Wrap(s.Save(creds))
}

// GetValidAuthHeader returns an Authorization header value backed by a
// currently-valid access token, refreshing it if necessary. Refreshes are
// single-flight across processes: contenders serialize on a file lock and
// re-check the store after acquiring it, so a rotating refresh token is only
// spent once and everyone else reuses the winner's result.
func (s *Store) GetValidAuthHeader(ctx context.Context) (string, error) {
	creds, err := s.Load()
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := s.checkCredentialBinding(creds); err != nil {
		return "", trace.Wrap(err)
	}
	if isFresh(creds.Token) {
		return authHeader(creds.Token), nil
	}
	if creds.Token.RefreshToken == "" {
		return "", trace.Wrap(ErrLoginRequired, "access token expired and no refresh token is available")
	}

	unlock, err := utils.FSTryWriteLockTimeout(ctx, s.lockPath, refreshLockTimeout)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer unlock()

	// Double-check after acquiring the lock: another process may have
	// refreshed while we were waiting.
	creds, err = s.Load()
	if err != nil {
		return "", trace.Wrap(err)
	}
	if err := s.checkCredentialBinding(creds); err != nil {
		return "", trace.Wrap(err)
	}
	if isFresh(creds.Token) {
		return authHeader(creds.Token), nil
	}

	newToken, err := s.refresh(ctx, creds)
	if err != nil {
		return "", trace.Wrap(err)
	}
	creds.Token = newToken
	if err := s.Save(creds); err != nil {
		return "", trace.Wrap(err)
	}
	return authHeader(newToken), nil
}

// refresh exchanges the refresh token at the cached token endpoint. It must
// only be called while holding the refresh lock.
func (s *Store) refresh(ctx context.Context, creds *Credentials) (*mcpclienttransport.Token, error) {
	if err := validateOAuthEndpoint(creds.TokenEndpoint, "token_endpoint"); err != nil {
		return nil, trace.Wrap(err)
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.Token.RefreshToken)
	form.Set("client_id", creds.ClientID)
	if creds.ClientSecret != "" {
		form.Set("client_secret", creds.ClientSecret)
	}
	if creds.Resource != "" {
		// RFC 8707 resource indicator.
		form.Set("resource", creds.Resource)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, creds.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "refreshing OAuth token")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		// 4xx means the grant itself was rejected (expired/revoked/rotated-away
		// refresh token) and only an interactive login can recover.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return nil, trace.Wrap(ErrLoginRequired, "token refresh rejected (HTTP %d): %s", resp.StatusCode, body)
		}
		return nil, trace.Errorf("token refresh failed (HTTP %d): %s", resp.StatusCode, body)
	}

	var token mcpclienttransport.Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, trace.Wrap(err, "decoding token refresh response")
	}
	if token.AccessToken == "" {
		return nil, trace.Wrap(ErrLoginRequired, "token refresh response contained no access token: %s", body)
	}
	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}
	// If the AS didn't rotate the refresh token, keep the old one.
	if token.RefreshToken == "" {
		token.RefreshToken = creds.Token.RefreshToken
	}
	return &token, nil
}

func (s *Store) checkCredentialBinding(creds *Credentials) error {
	if s.expectedUpstreamURL == "" {
		return nil
	}
	if creds.UpstreamURL != s.expectedUpstreamURL {
		return trace.Wrap(ErrLoginRequired, "stored OAuth credentials are for %q, current MCP upstream is %q", creds.UpstreamURL, s.expectedUpstreamURL)
	}
	return nil
}

func ensurePrivateDir(dir string) error {
	return trace.ConvertSystemError(os.MkdirAll(dir, 0o700))
}

func isFresh(t *mcpclienttransport.Token) bool {
	if t.AccessToken == "" {
		return false
	}
	if t.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().Add(expirySkew).Before(t.ExpiresAt)
}

func authHeader(t *mcpclienttransport.Token) string {
	// Per RFC 6749 section 5.1, token_type is case-insensitive; normalize for strict servers.
	tokenType := t.TokenType
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}
	return tokenType + " " + t.AccessToken
}
