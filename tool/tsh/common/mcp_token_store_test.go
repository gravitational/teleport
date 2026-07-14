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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/require"
)

func newTestCreds(accessToken string, expiresAt time.Time) *mcpOAuthCredentials {
	return &mcpOAuthCredentials{
		ClientID: "test-client-id",
		Token: mcpclienttransport.Token{
			AccessToken:  accessToken,
			TokenType:    "bearer",
			RefreshToken: "refresh-" + accessToken,
			ExpiresAt:    expiresAt,
		},
	}
}

func TestMCPOAuthCredentialsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cluster", "app.json")
	creds := newTestCreds("token-1", time.Now().Add(time.Hour))

	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	loaded, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, creds.ClientID, loaded.ClientID)
	require.Equal(t, creds.Token.AccessToken, loaded.Token.AccessToken)
	require.Equal(t, creds.Token.RefreshToken, loaded.Token.RefreshToken)

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, "0600", fmt.Sprintf("%04o", fi.Mode().Perm()))
	}
}

func TestLoadMCPOAuthCredentialsNotFound(t *testing.T) {
	_, err := loadMCPOAuthCredentials(filepath.Join(t.TempDir(), "nope.json"))
	require.True(t, trace.IsNotFound(err))
}

func TestFileTokenStore(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "app.json")
	store := &fileTokenStore{path: path, clientID: "test-client-id"}

	// No file yet: must map to mcp-go's sentinel.
	_, err := store.GetToken(ctx)
	require.ErrorIs(t, err, mcpclienttransport.ErrNoToken)

	token := &mcpclienttransport.Token{AccessToken: "tok", RefreshToken: "ref"}
	require.NoError(t, store.SaveToken(ctx, token))

	got, err := store.GetToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "tok", got.AccessToken)

	// SaveToken must preserve the client ID alongside the token.
	creds, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "test-client-id", creds.ClientID)
}

func TestMCPOAuthHeaderSourceFastPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("valid-token", time.Now().Add(time.Hour))))

	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		refresh: func(context.Context, string) (*mcpclienttransport.Token, error) {
			t.Fatal("refresh must not be called for a valid token")
			return nil, nil
		},
	}
	header, err := source.GetAuthHeader(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Bearer valid-token", header)
}

func TestMCPOAuthHeaderSourceNoRefreshToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	creds := newTestCreds("expired-token", time.Now().Add(-time.Hour))
	creds.Token.RefreshToken = ""
	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		refresh: func(context.Context, string) (*mcpclienttransport.Token, error) {
			t.Fatal("refresh must not be called without a refresh token")
			return nil, nil
		},
	}
	_, err := source.GetAuthHeader(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "tsh mcp login app")

	// The client-facing message names the fix command, even with extra
	// wrapping layers like the ones mcp-go and url.Error add in production.
	wrapped := fmt.Errorf("failed to send request: %w", err)
	userMessage := makeMCPReconnectUserMessage(wrapped)
	require.Contains(t, userMessage, "tsh mcp login app")
	require.NotContains(t, userMessage, "ensure your tsh session is valid")
}

// TestMCPOAuthHeaderSourceSingleFlight is the core chunk-5 guarantee: N
// concurrent header requests against one expired token perform exactly one
// refresh, and everyone converges on the refreshed token. Each
// FSTryWriteLockTimeout call opens its own file descriptor, so goroutines
// contend on the flock the same way separate tsh processes do.
func TestMCPOAuthHeaderSourceSingleFlight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("expired-token", time.Now().Add(-time.Hour))))

	var refreshCalls atomic.Int32
	const workers = 8

	newSource := func() *mcpOAuthHeaderSource {
		return &mcpOAuthHeaderSource{
			appName:   "app",
			credsPath: path,
			// Emulates OAuthHandler.RefreshToken: rotates the tokens and
			// persists the result before returning.
			refresh: func(ctx context.Context, refreshToken string) (*mcpclienttransport.Token, error) {
				refreshCalls.Add(1)
				// The stored refresh token rotates on use: refreshing twice
				// with the same one must never happen.
				require.Equal(t, "refresh-expired-token", refreshToken)
				time.Sleep(50 * time.Millisecond) // widen the race window
				token := &mcpclienttransport.Token{
					AccessToken:  "fresh-token",
					TokenType:    "Bearer",
					RefreshToken: "refresh-fresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				store := &fileTokenStore{path: path, clientID: "test-client-id"}
				require.NoError(t, store.SaveToken(ctx, token))
				return token, nil
			},
		}
	}

	var wg sync.WaitGroup
	headers := make([]string, workers)
	errs := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			headers[i], errs[i] = newSource().GetAuthHeader(context.Background())
		}()
	}
	wg.Wait()

	require.Equal(t, int32(1), refreshCalls.Load(), "expected exactly one refresh across all concurrent workers")
	for i := range workers {
		require.NoError(t, errs[i])
		require.Equal(t, "Bearer fresh-token", headers[i])
	}

	// The rotated refresh token is what's on disk afterwards.
	creds, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "refresh-fresh-token", creds.Token.RefreshToken)
}

func TestBearerAuthHeader(t *testing.T) {
	require.Equal(t, "Bearer x", bearerAuthHeader(&mcpclienttransport.Token{AccessToken: "x", TokenType: "bearer"}))
	require.Equal(t, "Bearer x", bearerAuthHeader(&mcpclienttransport.Token{AccessToken: "x", TokenType: ""}))
	require.Equal(t, "MAC x", bearerAuthHeader(&mcpclienttransport.Token{AccessToken: "x", TokenType: "MAC"}))
}

func TestMCPOAuthProxyMiddlewareHandleRequest(t *testing.T) {
	middleware := &mcpOAuthProxyMiddleware{
		getAuthHeader: func(context.Context) (string, error) {
			return "Bearer fresh-token", nil
		},
	}

	t.Run("injects stored token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://localhost:19101/", nil)
		handled := middleware.HandleRequest(httptest.NewRecorder(), req)
		require.False(t, handled)
		require.Equal(t, "Bearer fresh-token", req.Header.Get("Authorization"))
	})

	t.Run("client credentials win", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://localhost:19101/", nil)
		req.Header.Set("Authorization", "Bearer client-supplied")
		handled := middleware.HandleRequest(httptest.NewRecorder(), req)
		require.False(t, handled)
		require.Equal(t, "Bearer client-supplied", req.Header.Get("Authorization"))
	})

	t.Run("login required returns 403 with fix command", func(t *testing.T) {
		failing := &mcpOAuthProxyMiddleware{
			getAuthHeader: func(context.Context) (string, error) {
				return "", trace.Wrap(&mcpOAuthLoginRequiredError{appName: "sentry"})
			},
		}
		req := httptest.NewRequest(http.MethodPost, "http://localhost:19101/", nil)
		recorder := httptest.NewRecorder()
		handled := failing.HandleRequest(recorder, req)
		require.True(t, handled)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), "tsh mcp login sentry")
	})
}
