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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	testMCPOAuthResourceURI = "mcp+https://mcp.example.com/mcp"
	testMCPOAuthIssuer      = "https://auth.example.com"
)

// Keep dependency upgrades from silently changing persisted credential JSON.
type persistedMCPToken = struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

var _ mcpclienttransport.Token = persistedMCPToken{}

type mcpOAuthDialerClient struct {
	client.MCPServerDialerClient
	app types.Application
}

func (c *mcpOAuthDialerClient) ListApps(context.Context, *proto.ListResourcesRequest) ([]types.Application, error) {
	return []types.Application{c.app}, nil
}

func newTestMCPOAuthApp(t *testing.T) types.Application {
	app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: testMCPOAuthResourceURI})
	require.NoError(t, err)
	return app
}

func newTestMCPOAuthDialer(t *testing.T) *client.MCPServerDialer {
	return client.NewMCPServerDialer(&mcpOAuthDialerClient{app: newTestMCPOAuthApp(t)}, scopes.QualifiedName{Name: "app"})
}

func newTestCreds(accessToken string, expiresAt time.Time) *mcpOAuthCredentials {
	return &mcpOAuthCredentials{
		ResourceURI:  testMCPOAuthResourceURI,
		Issuer:       testMCPOAuthIssuer,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:12345/callback",
		Scopes:       []string{"mcp:tools", "mcp:resources"},
		Token: mcpclienttransport.Token{
			AccessToken:  accessToken,
			TokenType:    "bearer",
			RefreshToken: "refresh-" + accessToken,
			ExpiresAt:    expiresAt,
		},
	}
}

func TestMCPOAuthTokenPath(t *testing.T) {
	home := t.TempDir()
	path := func(username string, app scopes.QualifiedName) string {
		path, err := mcpOAuthTokenPath(home, "proxy.example.com", username, "root", app)
		require.NoError(t, err)
		return path
	}

	require.NotEqual(t,
		path("alice", scopes.QualifiedName{Name: "app"}),
		path("bob", scopes.QualifiedName{Name: "app"}),
	)
	require.NotEqual(t,
		path("alice", scopes.QualifiedName{Name: "app", Scope: "/prod"}),
		path("alice", scopes.QualifiedName{Name: "app", Scope: "/dev"}),
	)
	_, err := mcpOAuthTokenPath(home, "proxy.example.com", "alice", "root", scopes.QualifiedName{Name: "../../profile"})
	require.Error(t, err)
}

func TestMCPOAuthCredentialsRoundTrip(t *testing.T) {
	path, err := mcpOAuthTokenPath(t.TempDir(), "proxy.example.com", "alice", "root", scopes.QualifiedName{Name: "app"})
	require.NoError(t, err)
	creds := newTestCreds("token-1", time.Now().Add(time.Hour))

	require.NoError(t, saveMCPOAuthCredentials(path, creds))
	if runtime.GOOS != "windows" {
		require.NoError(t, os.Chmod(path, 0o644))
		require.NoError(t, saveMCPOAuthCredentials(path, creds))
	}

	loaded, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, creds.ResourceURI, loaded.ResourceURI)
	require.Equal(t, creds.Issuer, loaded.Issuer)
	require.Equal(t, creds.ClientID, loaded.ClientID)
	require.Equal(t, creds.ClientSecret, loaded.ClientSecret)
	require.Equal(t, creds.RedirectURI, loaded.RedirectURI)
	require.Equal(t, creds.Scopes, loaded.Scopes)
	require.Equal(t, creds.Token.AccessToken, loaded.Token.AccessToken)
	require.Equal(t, creds.Token.RefreshToken, loaded.Token.RefreshToken)

	if runtime.GOOS != "windows" {
		fi, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, "0600", fmt.Sprintf("%04o", fi.Mode().Perm()))
	}
}

func TestFileTokenStore(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	lockPath := filepath.Join(dir, "locks", "mcp_oauth.lock")
	store := &fileTokenStore{
		path:             path,
		mutationLockPath: lockPath,
		resourceURI:      testMCPOAuthResourceURI,
		issuer:           testMCPOAuthIssuer,
	}

	_, err := store.GetToken(ctx)
	require.ErrorIs(t, err, mcpclienttransport.ErrNoToken)

	creds := newTestCreds("old-token", time.Now().Add(-time.Hour))
	creds.Scopes = []string{"mcp:tools"}
	require.NoError(t, saveMCPOAuthCredentials(path, creds))
	token := &mcpclienttransport.Token{AccessToken: "tok", RefreshToken: "ref"}
	require.NoError(t, store.SaveToken(ctx, token))
	require.FileExists(t, lockPath)

	got, err := store.GetToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "tok", got.AccessToken)

	creds, err = loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "test-client-id", creds.ClientID)
	require.Equal(t, "test-client-secret", creds.ClientSecret)
	require.Equal(t, "http://localhost:12345/callback", creds.RedirectURI)
	require.Equal(t, []string{"mcp:tools"}, creds.Scopes)

	creds.Issuer = "https://replacement.example.com"
	require.NoError(t, saveMCPOAuthCredentials(path, creds))
	_, err = store.GetToken(ctx)
	require.ErrorContains(t, err, "bound to a different issuer")
}

func TestFileTokenStoreDoesNotRestoreLoggedOutCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	lockPath := filepath.Join(dir, "mcp_oauth.lock")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("old-token", time.Now().Add(-time.Hour))))

	unlock, err := utils.FSTryWriteLockTimeout(t.Context(), lockPath, time.Second)
	require.NoError(t, err)
	locked := true
	t.Cleanup(func() {
		if locked {
			_ = unlock()
		}
	})

	saved := make(chan error, 1)
	go func() {
		saved <- (&fileTokenStore{
			path:             path,
			mutationLockPath: lockPath,
			resourceURI:      testMCPOAuthResourceURI,
			issuer:           testMCPOAuthIssuer,
		}).SaveToken(t.Context(), &mcpclienttransport.Token{AccessToken: "new-token"})
	}()

	select {
	case err := <-saved:
		require.FailNow(t, "save ignored credential mutation lock", "%v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, os.Remove(path))
	require.NoError(t, unlock())
	locked = false
	require.Error(t, <-saved)
	require.NoFileExists(t, path)
}

func TestMCPOAuthHeaderSourceMissingCredentials(t *testing.T) {
	dir := t.TempDir()
	source := newMCPOAuthHeaderSource(
		newTestMCPOAuthDialer(t),
		filepath.Join(dir, "missing.json"),
		filepath.Join(dir, "mcp_oauth.lock"),
		"app",
		func(context.Context, *mcpOAuthCredentials) error {
			t.Fatal("automatic login must not run without stored credentials")
			return nil
		},
	)

	// No token to send; the server's 401 decides what happens next.
	header, err := source.GetAuthHeader(t.Context())
	require.NoError(t, err)
	require.Empty(t, header)

	// A 401 with nothing stored is a login-required error, not a browser.
	_, err = source.RefreshAuthHeader(t.Context(), "")
	require.ErrorContains(t, err, "tsh mcp login app")
}

func TestNewMCPOAuthHeaderSourceDefersDiscovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("valid-token", time.Now().Add(time.Hour))))

	source := newMCPOAuthHeaderSource(newTestMCPOAuthDialer(t), path, filepath.Join(filepath.Dir(path), "mcp_oauth.lock"), "app", nil)
	header, err := source.GetAuthHeader(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Bearer valid-token", header)
}

func TestMCPOAuthHeaderSourceFastPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("valid-token", time.Now().Add(time.Hour))))

	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		dialer:    newTestMCPOAuthDialer(t),
		refresh: func(context.Context, *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
			t.Fatal("refresh must not be called for a valid token")
			return nil, nil
		},
	}
	header, err := source.GetAuthHeader(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Bearer valid-token", header)
}

// requireSingleFlightHeader calls fn from eight goroutines at once and checks
// that every one of them gets header back.
func requireSingleFlightHeader(t *testing.T, header string, fn func() (string, error)) {
	t.Helper()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := fn()
			assert.NoError(t, err)
			assert.Equal(t, header, got)
		})
	}
	wg.Wait()
}

func TestMCPOAuthHeaderSourceRejectedTokenSingleFlight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	rejectedHeader := "Bearer rejected-token"
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("rejected-token", time.Now().Add(time.Hour))))

	var refreshCalls atomic.Int32
	newSource := func() *mcpOAuthHeaderSource {
		return &mcpOAuthHeaderSource{
			appName:   "app",
			credsPath: path,
			dialer:    newTestMCPOAuthDialer(t),
			refresh: func(ctx context.Context, creds *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
				refreshCalls.Add(1)
				require.Equal(t, "refresh-rejected-token", creds.Token.RefreshToken)
				token := &mcpclienttransport.Token{
					AccessToken:  "fresh-token",
					TokenType:    "Bearer",
					RefreshToken: "refresh-fresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				store := &fileTokenStore{
					path:             path,
					mutationLockPath: filepath.Join(filepath.Dir(path), "mcp_oauth.lock"),
					resourceURI:      testMCPOAuthResourceURI,
					issuer:           testMCPOAuthIssuer,
				}
				require.NoError(t, store.SaveToken(ctx, token))
				return token, nil
			},
		}
	}

	requireSingleFlightHeader(t, "Bearer fresh-token", func() (string, error) {
		return newSource().RefreshAuthHeader(t.Context(), rejectedHeader)
	})
	require.Equal(t, int32(1), refreshCalls.Load())

	creds, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:12345/callback", creds.RedirectURI)
	require.Equal(t, []string{"mcp:tools", "mcp:resources"}, creds.Scopes)
}

func TestMCPOAuthHeaderSourceNoRefreshToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	creds := newTestCreds("expired-token", time.Now().Add(-time.Hour))
	creds.Token.RefreshToken = ""
	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		dialer:    newTestMCPOAuthDialer(t),
		refresh: func(context.Context, *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
			t.Fatal("refresh must not be called without a refresh token")
			return nil, nil
		},
	}
	_, err := source.GetAuthHeader(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "tsh mcp login app")

	wrapped := fmt.Errorf("failed to send request: %w", err)
	userMessage := makeMCPReconnectUserMessage(wrapped)
	require.Contains(t, userMessage, "tsh mcp login app")
	require.NotContains(t, userMessage, "ensure your tsh session is valid")
}

func TestMCPOAuthHeaderSourceIgnoresCredentialsForAnotherResource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	creds := newTestCreds("valid-token", time.Now().Add(time.Hour))
	creds.ResourceURI = "mcp+https://replacement.example.com/mcp"
	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		dialer:    newTestMCPOAuthDialer(t),
		refresh: func(context.Context, *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
			t.Fatal("refresh must not receive credentials for another resource")
			return nil, nil
		},
	}
	header, err := source.GetAuthHeader(t.Context())
	require.NoError(t, err)
	require.Empty(t, header)

	_, err = source.RefreshAuthHeader(t.Context(), "")
	require.ErrorContains(t, err, "bound to a different resource")
	require.ErrorContains(t, err, "tsh mcp login app")
}

func TestMCPOAuthHeaderSourceAutomaticReauthorizationSingleFlight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	creds := newTestCreds("expired-token", time.Now().Add(-time.Hour))
	creds.Token.RefreshToken = ""
	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	var reauthorizeCalls atomic.Int32
	newSource := func() *mcpOAuthHeaderSource {
		return &mcpOAuthHeaderSource{
			appName:   "app",
			credsPath: path,
			dialer:    newTestMCPOAuthDialer(t),
			refresh: func(context.Context, *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
				t.Fatal("refresh must not be called without a refresh token")
				return nil, nil
			},
			reauthorize: func(ctx context.Context, expired *mcpOAuthCredentials) error {
				reauthorizeCalls.Add(1)
				require.Equal(t, "test-client-id", expired.ClientID)
				require.Equal(t, "http://localhost:12345/callback", expired.RedirectURI)
				fresh := newTestCreds("fresh-token", time.Now().Add(time.Hour))
				return saveMCPOAuthCredentials(path, fresh)
			},
		}
	}

	requireSingleFlightHeader(t, "Bearer fresh-token", func() (string, error) {
		return newSource().GetAuthHeader(t.Context())
	})
	require.Equal(t, int32(1), reauthorizeCalls.Load())
}

func TestMCPOAuthHeaderSourceAutomaticReauthorizationFailsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	creds := newTestCreds("expired-token", time.Now().Add(-time.Hour))
	creds.Token.RefreshToken = ""
	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	var reauthorizeCalls atomic.Int32
	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		dialer:    newTestMCPOAuthDialer(t),
		reauthorize: func(context.Context, *mcpOAuthCredentials) error {
			reauthorizeCalls.Add(1)
			return trace.AccessDenied("user closed the browser")
		},
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, err := source.GetAuthHeader(t.Context())
			assert.ErrorContains(t, err, "tsh mcp login app")
		})
	}
	wg.Wait()
	require.Equal(t, int32(1), reauthorizeCalls.Load())
}

func TestMCPOAuthHeaderSourceCancelledReauthorizationDoesNotLatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	creds := newTestCreds("expired-token", time.Now().Add(-time.Hour))
	creds.Token.RefreshToken = ""
	require.NoError(t, saveMCPOAuthCredentials(path, creds))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var reauthorizeCalls atomic.Int32
	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		dialer:    newTestMCPOAuthDialer(t),
		reauthorize: func(ctx context.Context, _ *mcpOAuthCredentials) error {
			reauthorizeCalls.Add(1)
			// The client gave up mid-flow.
			cancel()
			return ctx.Err()
		},
	}

	_, err := source.GetAuthHeader(ctx)
	require.Error(t, err)
	_, err = source.GetAuthHeader(t.Context())
	require.Error(t, err)
	require.Equal(t, int32(2), reauthorizeCalls.Load(), "a canceled login must not stop the next one")
}

func TestMCPOAuthHeaderSourceAutomaticReauthorizationAfterRefreshFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("expired-token", time.Now().Add(-time.Hour))))

	var refreshCalls, reauthorizeCalls atomic.Int32
	source := &mcpOAuthHeaderSource{
		appName:   "app",
		credsPath: path,
		dialer:    newTestMCPOAuthDialer(t),
		refresh: func(context.Context, *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
			refreshCalls.Add(1)
			return nil, trace.AccessDenied("refresh token expired")
		},
		reauthorize: func(ctx context.Context, expired *mcpOAuthCredentials) error {
			reauthorizeCalls.Add(1)
			require.Equal(t, "test-client-id", expired.ClientID)
			return saveMCPOAuthCredentials(path, newTestCreds("fresh-token", time.Now().Add(time.Hour)))
		},
	}

	header, err := source.GetAuthHeader(t.Context())
	require.NoError(t, err)
	require.Equal(t, "Bearer fresh-token", header)
	require.Equal(t, int32(1), refreshCalls.Load())
	require.Equal(t, int32(1), reauthorizeCalls.Load())
}

func TestMCPOAuthHeaderSourceSingleFlight(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.json")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("expired-token", time.Now().Add(-time.Hour))))

	var refreshCalls atomic.Int32
	newSource := func() *mcpOAuthHeaderSource {
		return &mcpOAuthHeaderSource{
			appName:   "app",
			credsPath: path,
			dialer:    newTestMCPOAuthDialer(t),
			refresh: func(ctx context.Context, creds *mcpOAuthCredentials) (*mcpclienttransport.Token, error) {
				refreshCalls.Add(1)
				require.Equal(t, "refresh-expired-token", creds.Token.RefreshToken)
				token := &mcpclienttransport.Token{
					AccessToken:  "fresh-token",
					TokenType:    "Bearer",
					RefreshToken: "refresh-fresh-token",
					ExpiresAt:    time.Now().Add(time.Hour),
				}
				store := &fileTokenStore{
					path:             path,
					mutationLockPath: filepath.Join(filepath.Dir(path), "mcp_oauth.lock"),
					resourceURI:      testMCPOAuthResourceURI,
					issuer:           testMCPOAuthIssuer,
				}
				require.NoError(t, store.SaveToken(ctx, token))
				return token, nil
			},
		}
	}

	requireSingleFlightHeader(t, "Bearer fresh-token", func() (string, error) {
		return newSource().GetAuthHeader(t.Context())
	})
	require.Equal(t, int32(1), refreshCalls.Load())

	creds, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "refresh-fresh-token", creds.Token.RefreshToken)
	require.Equal(t, "test-client-secret", creds.ClientSecret)
	require.Equal(t, "http://localhost:12345/callback", creds.RedirectURI)
	require.Equal(t, []string{"mcp:tools", "mcp:resources"}, creds.Scopes)
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

	t.Run("no stored token sends the request as is", func(t *testing.T) {
		empty := &mcpOAuthProxyMiddleware{
			getAuthHeader: func(context.Context) (string, error) { return "", nil },
		}
		req := httptest.NewRequest(http.MethodPost, "http://localhost:19101/", nil)
		require.False(t, empty.HandleRequest(httptest.NewRecorder(), req))
		require.Empty(t, req.Header.Get("Authorization"))
		require.Equal(t, true, req.Context().Value(mcpOAuthManagedContextKey{}))
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

type mcpTokenStoreRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mcpTokenStoreRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestMCPOAuthProxyMiddlewareInvalidTokenRetry(t *testing.T) {
	t.Run("refreshes injected token and retries", func(t *testing.T) {
		var headers, bodies []string
		middleware := &mcpOAuthProxyMiddleware{
			getAuthHeader: func(context.Context) (string, error) {
				return "Bearer rejected-token", nil
			},
			refreshAuthHeader: func(_ context.Context, rejected string) (string, error) {
				require.Equal(t, "Bearer rejected-token", rejected)
				return "Bearer fresh-token", nil
			},
		}
		base := mcpTokenStoreRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			headers = append(headers, req.Header.Get("Authorization"))
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			bodies = append(bodies, string(body))
			if len(headers) == 1 {
				return mcpInvalidTokenTestResponse(req), nil
			}
			return mcpTestHTTPResponse(req, http.StatusOK), nil
		})
		req := httptest.NewRequest(http.MethodPost, "http://localhost:19101/mcp", strings.NewReader(`{"method":"tools/list"}`))
		require.Nil(t, req.GetBody)
		require.False(t, middleware.HandleRequest(httptest.NewRecorder(), req))

		resp, err := middleware.WrapRoundTripper(base).RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, []string{"Bearer rejected-token", "Bearer fresh-token"}, headers)
		require.Equal(t, []string{`{"method":"tools/list"}`, `{"method":"tools/list"}`}, bodies)
	})

	t.Run("401 after refresh becomes login required", func(t *testing.T) {
		middleware := &mcpOAuthProxyMiddleware{
			appName: "sentry",
			getAuthHeader: func(context.Context) (string, error) {
				return "Bearer rejected-token", nil
			},
			refreshAuthHeader: func(context.Context, string) (string, error) {
				return "Bearer fresh-token", nil
			},
		}
		base := mcpTokenStoreRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return mcpInvalidTokenTestResponse(req), nil
		})
		req, err := http.NewRequest(http.MethodGet, "http://localhost:19101/mcp", nil)
		require.NoError(t, err)
		require.False(t, middleware.HandleRequest(httptest.NewRecorder(), req))

		resp, err := middleware.WrapRoundTripper(base).RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "tsh mcp login sentry")
	})

	t.Run("does not refresh client supplied token", func(t *testing.T) {
		var requests int
		middleware := &mcpOAuthProxyMiddleware{
			getAuthHeader: func(context.Context) (string, error) {
				t.Fatal("stored token must not replace a client-supplied token")
				return "", nil
			},
			refreshAuthHeader: func(context.Context, string) (string, error) {
				t.Fatal("client-supplied token must not be refreshed by tsh")
				return "", nil
			},
		}
		base := mcpTokenStoreRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return mcpInvalidTokenTestResponse(req), nil
		})
		req, err := http.NewRequest(http.MethodGet, "http://localhost:19101/mcp", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer client-token")
		require.False(t, middleware.HandleRequest(httptest.NewRecorder(), req))

		resp, err := middleware.WrapRoundTripper(base).RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		require.Equal(t, 1, requests)
	})

	t.Run("refresh failure returns actionable forbidden response", func(t *testing.T) {
		middleware := &mcpOAuthProxyMiddleware{
			getAuthHeader: func(context.Context) (string, error) {
				return "Bearer rejected-token", nil
			},
			refreshAuthHeader: func(context.Context, string) (string, error) {
				return "", trace.Wrap(&mcpOAuthLoginRequiredError{appName: "sentry"})
			},
		}
		base := mcpTokenStoreRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return mcpInvalidTokenTestResponse(req), nil
		})
		req, err := http.NewRequest(http.MethodGet, "http://localhost:19101/mcp", nil)
		require.NoError(t, err)
		require.False(t, middleware.HandleRequest(httptest.NewRecorder(), req))

		resp, err := middleware.WrapRoundTripper(base).RoundTrip(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "tsh mcp login sentry")
	})
}

func mcpInvalidTokenTestResponse(req *http.Request) *http.Response {
	resp := mcpTestHTTPResponse(req, http.StatusUnauthorized)
	resp.Header.Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	return resp
}

func mcpTestHTTPResponse(req *http.Request, status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(http.StatusText(status))),
		Request:    req,
	}
}
