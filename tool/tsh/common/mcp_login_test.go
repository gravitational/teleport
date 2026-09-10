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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.dny.dev/ssrf"
	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

type mcpOAuthRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mcpOAuthRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSaveMCPOAuthLoginCredentialsSerializesWithRefresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	mutationLockPath := filepath.Join(dir, "mcp_oauth.lock")
	require.NoError(t, saveMCPOAuthCredentials(path, newTestCreds("old-token", time.Now().Add(-time.Hour))))
	unlock, err := utils.FSTryWriteLockTimeout(t.Context(), path+".lock", time.Second)
	require.NoError(t, err)

	loggedIn := newTestCreds("login-token", time.Now().Add(time.Hour))
	loggedIn.ClientID = "new-client"
	saved := make(chan error, 1)
	go func() {
		saved <- saveMCPOAuthLoginCredentials(t.Context(), path, mutationLockPath, false, loggedIn)
	}()
	select {
	case err := <-saved:
		require.FailNow(t, "login write did not wait for refresh", "%v", err)
	case <-time.After(100 * time.Millisecond):
	}

	store := &fileTokenStore{
		path:             path,
		mutationLockPath: mutationLockPath,
		resourceURI:      testMCPOAuthResourceURI,
		issuer:           testMCPOAuthIssuer,
	}
	require.NoError(t, store.SaveToken(t.Context(), &mcpclienttransport.Token{AccessToken: "refreshed-old-token"}))
	require.NoError(t, unlock())
	require.NoError(t, <-saved)

	creds, err := loadMCPOAuthCredentials(path)
	require.NoError(t, err)
	require.Equal(t, "new-client", creds.ClientID)
	require.Equal(t, "login-token", creds.Token.AccessToken)
}

func TestCheckMCPOAuthCallbackIssuer(t *testing.T) {
	t.Parallel()

	const issuer = "https://auth.example.com"
	require.NoError(t, checkMCPOAuthCallbackIssuer(url.Values{}, issuer))
	require.NoError(t, checkMCPOAuthCallbackIssuer(url.Values{"iss": {issuer}}, issuer))
	err := checkMCPOAuthCallbackIssuer(url.Values{"iss": {issuer + "/"}}, issuer)
	require.True(t, trace.IsAccessDenied(err), "%v", err)
	err = checkMCPOAuthCallbackIssuer(url.Values{"iss": {"https://attacker.example.com"}}, issuer)
	require.True(t, trace.IsAccessDenied(err), "%v", err)
}

func TestSaveAutomaticMCPOAuthLoginCredentialsRequiresExistingCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.json")
	err := saveMCPOAuthLoginCredentials(t.Context(), path, filepath.Join(dir, "mcp_oauth.lock"), true, newTestCreds("token", time.Now().Add(time.Hour)))
	require.Error(t, err)
	require.NoFileExists(t, path)
}

func TestWrapMCPClientRegistrationError(t *testing.T) {
	t.Parallel()

	dcrErr := errors.New("server does not support dynamic client registration")
	err := wrapMCPClientRegistrationError(dcrErr, "databricks-sql")
	require.ErrorContains(t, err, "does not support dynamic client registration")
	require.ErrorContains(t, err, "tsh mcp login databricks-sql --client-id")
	require.ErrorContains(t, err, "--callback-port")

	otherErr := errors.New("failed to get server metadata: boom")
	err = wrapMCPClientRegistrationError(otherErr, "databricks-sql")
	require.ErrorContains(t, err, "boom")
	require.NotContains(t, err.Error(), "--client-id")
}

func TestWrapMCPClientRegistrationErrorRejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		rejected bool
	}{
		{
			name: "OAuth error body",
			err: fmt.Errorf("registration request failed: %w", mcpclienttransport.OAuthError{
				ErrorCode:        "unapproved_software_statement",
				ErrorDescription: "client is not approved",
			}),
			rejected: true,
		},
		{
			name:     "403 without an OAuth error body",
			err:      trace.AccessDenied("registration request failed with status 403: Forbidden"),
			rejected: true,
		},
		{
			name:     "401 without an OAuth error body",
			err:      trace.AccessDenied("registration request failed with status 401: Unauthorized"),
			rejected: true,
		},
		{
			name:     "server error is not a rejection",
			err:      errors.New(`registration request failed with status 500: Internal Server Error`),
			rejected: false,
		},
		{
			name:     "transport failure is not a rejection",
			err:      errors.New("failed to send registration request: connection refused"),
			rejected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := wrapMCPClientRegistrationError(test.err, "figma")
			require.ErrorIs(t, err, test.err)
			if !test.rejected {
				require.NotContains(t, err.Error(), "--client-id")
				return
			}
			require.ErrorContains(t, err, "rejected the client registration")
			require.ErrorContains(t, err, "tsh mcp login figma --client-id")
			require.ErrorContains(t, err, "--callback-port")
		})
	}
}

func TestWrapMCPClientRegistrationErrorInvalidMetadata(t *testing.T) {
	t.Parallel()

	registrationErr := fmt.Errorf("registration request failed: %w", mcpclienttransport.OAuthError{
		ErrorCode:        "invalid_client_metadata",
		ErrorDescription: "scope must not be empty",
	})
	err := wrapMCPClientRegistrationError(registrationErr, "stripe")
	require.ErrorIs(t, err, registrationErr)
	require.ErrorContains(t, err, "client metadata sent by tsh is invalid")
	require.NotContains(t, err.Error(), "approved set of MCP clients")
}

func TestRegisterMCPOAuthClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		scopes             []string
		registrationStatus int
		registrationBody   string
		wantScope          string
		wantScopeField     bool
		wantErrorCode      string
	}{
		{
			name:               "empty scopes omitted",
			registrationStatus: http.StatusCreated,
			registrationBody:   `{"client_id":"registered-client","client_secret":"registered-secret"}`,
		},
		{
			name:               "scopes included",
			scopes:             []string{"mcp:tools", "mcp:resources"},
			registrationStatus: http.StatusCreated,
			registrationBody:   `{"client_id":"registered-client","client_secret":"registered-secret"}`,
			wantScope:          "mcp:tools mcp:resources",
			wantScopeField:     true,
		},
		{
			name:               "OAuth error response",
			registrationStatus: http.StatusBadRequest,
			registrationBody:   `{"error":"invalid_client_metadata","error_description":"scope must not be empty"}`,
			wantErrorCode:      "invalid_client_metadata",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var registrationRequest map[string]any
			httpClient := &http.Client{Transport: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				var statusCode int
				var body string
				switch req.URL.String() {
				case "https://mcp.example.com/.well-known/oauth-protected-resource/mcp":
					statusCode = http.StatusOK
					body = `{"resource":"https://mcp.example.com/mcp","authorization_servers":["https://auth.example.com"]}`
				case "https://auth.example.com/.well-known/oauth-authorization-server":
					statusCode = http.StatusOK
					body = `{"issuer":"https://auth.example.com","authorization_endpoint":"https://auth.example.com/authorize","token_endpoint":"https://auth.example.com/token","registration_endpoint":"https://auth.example.com/register"}`
				case "https://auth.example.com/register":
					require.Equal(t, http.MethodPost, req.Method)
					require.NoError(t, json.NewDecoder(req.Body).Decode(&registrationRequest))
					statusCode = test.registrationStatus
					body = test.registrationBody
				default:
					require.FailNow(t, "unexpected OAuth request", req.URL.String())
				}
				return &http.Response{
					StatusCode: statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})}

			discoveryHandler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{HTTPClient: httpClient})
			discoveryHandler.SetBaseURL("https://mcp.example.com/mcp")
			metadata, err := getMCPOAuthServerMetadata(t.Context(), discoveryHandler)
			require.NoError(t, err)
			clientID, clientSecret, err := registerMCPOAuthClient(
				t.Context(), metadata, httpClient, "http://127.0.0.1:12345/callback", test.scopes,
			)

			require.Equal(t, defaultMCPOAuthClientName, registrationRequest["client_name"])
			require.Equal(t, mcpOAuthClientURI, registrationRequest["client_uri"])
			if test.wantScopeField {
				require.Equal(t, test.wantScope, registrationRequest["scope"])
			} else {
				require.NotContains(t, registrationRequest, "scope")
			}
			if test.wantErrorCode != "" {
				require.Error(t, err)
				oauthErr, ok := errors.AsType[mcpclienttransport.OAuthError](err)
				require.True(t, ok)
				require.Equal(t, test.wantErrorCode, oauthErr.ErrorCode)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "registered-client", clientID)
			require.Equal(t, "registered-secret", clientSecret)
		})
	}
}

func TestCheckMCPOAuthServerMetadataUnchanged(t *testing.T) {
	metadata := func(issuer, authorizationEndpoint, tokenEndpoint string) *mcpclienttransport.AuthServerMetadata {
		return &mcpclienttransport.AuthServerMetadata{
			Issuer:                issuer,
			AuthorizationEndpoint: authorizationEndpoint,
			TokenEndpoint:         tokenEndpoint,
		}
	}
	registered := metadata("issuer", "authorize", "token")
	require.NoError(t, checkMCPOAuthServerMetadataUnchanged(registered, metadata("issuer", "authorize", "token")))

	for _, current := range []*mcpclienttransport.AuthServerMetadata{
		metadata("other", "authorize", "token"),
		metadata("issuer", "other", "token"),
		metadata("issuer", "authorize", "other"),
	} {
		require.ErrorContains(t, checkMCPOAuthServerMetadataUnchanged(registered, current), "changed during client registration")
	}
}

func TestMCPOAuthDiscoveryBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appURI  string
		want    string
		wantErr string
	}{
		{
			name:   "root endpoint",
			appURI: "mcp+https://mcp.example.com",
			want:   "https://mcp.example.com",
		},
		{
			name:   "standard MCP path",
			appURI: "mcp+https://mcp.example.com/mcp",
			want:   "https://mcp.example.com/mcp",
		},
		{
			name:   "nested provider path",
			appURI: "mcp+https://mcp.example.com/v2/mcp?tenant=ignored#fragment",
			want:   "https://mcp.example.com/v2/mcp",
		},
		{
			name:   "escaped path",
			appURI: "mcp+https://mcp.example.com/tenant%2Fone/mcp",
			want:   "https://mcp.example.com/tenant%2Fone/mcp",
		},
		{
			name:    "non MCP scheme",
			appURI:  "https://mcp.example.com/mcp",
			wantErr: "does not use HTTP transport",
		},
		{
			name:    "missing host",
			appURI:  "mcp+https:///mcp",
			wantErr: "missing a host",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := mcpOAuthDiscoveryBaseURL(test.appURI)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestGetMCPOAuthServerMetadataRejectsUnsafeEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata string
		wantErr  string
	}{
		{
			name:     "insecure token endpoint",
			metadata: `{"issuer":"https://auth.example.com","authorization_endpoint":"https://auth.example.com/authorize","token_endpoint":"http://auth.example.com/token"}`,
			wantErr:  "token endpoint must be an absolute HTTPS URL",
		},
		{
			name:     "private authorization endpoint",
			metadata: `{"issuer":"https://auth.example.com","authorization_endpoint":"https://127.0.0.1/authorize","token_endpoint":"https://auth.example.com/token"}`,
			wantErr:  "authorization endpoint uses a prohibited network address",
		},
		{
			name:     "insecure registration endpoint",
			metadata: `{"issuer":"https://auth.example.com","authorization_endpoint":"https://auth.example.com/authorize","token_endpoint":"https://auth.example.com/token","registration_endpoint":"http://auth.example.com/register"}`,
			wantErr:  "registration endpoint must be an absolute HTTPS URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			httpClient := &http.Client{Transport: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				body := `{"authorization_servers":["https://auth.example.com"]}`
				if req.URL.Host == "auth.example.com" {
					body = test.metadata
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})}
			handler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{HTTPClient: httpClient})
			handler.SetBaseURL("https://mcp.example.com/mcp")

			_, err := getMCPOAuthServerMetadata(t.Context(), handler)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestGetMCPOAuthServerMetadataRejectsAdvertisedIssuerMismatch(t *testing.T) {
	t.Parallel()

	const resource = "https://mcp.example.com/mcp"
	var paths []string
	transport := mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		statusCode := http.StatusNotFound
		body := http.StatusText(statusCode)
		if req.URL.Path == "/.well-known/oauth-protected-resource/mcp" {
			statusCode = http.StatusOK
			body = `{"resource":"` + resource + `","authorization_servers":["https://auth.example.com/tenant"]}`
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	mcpServerOrigin, err := url.Parse(resource)
	require.NoError(t, err)
	httpClient := &http.Client{Transport: &hostRoutingTransport{
		tunneled:        transport,
		direct:          transport,
		mcpServerOrigin: mcpServerOrigin,
	}}
	handler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{HTTPClient: httpClient})
	handler.SetBaseURL(resource)

	// The SDK synthesizes root endpoints after both discovery requests fail.
	_, err = getMCPOAuthServerMetadata(t.Context(), handler)
	require.ErrorContains(t, err, `protected resource metadata advertised "https://auth.example.com/tenant"`)
	require.Equal(t, []string{
		"/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server/tenant",
		"/.well-known/openid-configuration/tenant",
	}, paths)
}

func TestMCPOAuthHTTPClientBlocksPrivateRedirect(t *testing.T) {
	t.Parallel()

	httpClient, err := newMCPOAuthHTTPClient(&client.MCPServerDialer{}, "mcp+https://mcp.example.com")
	require.NoError(t, err)
	routing := httpClient.Transport.(*hostRoutingTransport)
	routing.tunneled = mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://127.0.0.1:443/"}},
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})
	routing.direct = mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "127.0.0.1", req.URL.Hostname())
		return nil, mcpOAuthNetworkGuard.Safe("tcp4", "127.0.0.1:443", nil)
	})

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://mcp.example.com/.well-known/oauth-protected-resource", nil)
	require.NoError(t, err)
	resp, err := httpClient.Do(req)
	if resp != nil {
		require.NoError(t, resp.Body.Close())
	}
	require.ErrorIs(t, err, ssrf.ErrProhibitedIP)
}

func TestListenMCPOAuthCallbackReusesStoredRedirectURI(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	callbackPort := probe.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", callbackPort)
	require.NoError(t, probe.Close())

	listener, gotRedirectURI, err := listenMCPOAuthCallback(0, redirectURI)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	require.Equal(t, redirectURI, gotRedirectURI)
	require.Equal(t, callbackPort, listener.Addr().(*net.TCPAddr).Port)
}

func TestListenMCPOAuthCallbackRejectsUnsafeStoredRedirectURI(t *testing.T) {
	tests := []string{
		"https://localhost:12345/callback",
		"http://example.com:12345/callback",
		"http://localhost:12345/not-callback",
		"http://localhost/callback",
	}
	for _, redirectURI := range tests {
		t.Run(redirectURI, func(t *testing.T) {
			listener, _, err := listenMCPOAuthCallback(0, redirectURI)
			require.Error(t, err)
			require.Nil(t, listener)
		})
	}
}

func TestMCPOAuthAutomaticReauthorizationHonorsBrowserNone(t *testing.T) {
	require.Nil(t, newMCPOAuthReauthorizeFunc(nil, "app", "credentials.json", "mcp_oauth.lock", teleport.BrowserNone, io.Discard))
}

func TestMCPOAuthPathAwareDiscoveryUsesPublicResource(t *testing.T) {
	t.Parallel()

	const (
		appURI         = "mcp+https://mcp.example.com/v2/mcp"
		publicResource = "https://mcp.example.com/v2/mcp"
	)

	var requests []string
	mockTransport := mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.URL.String())
		var body string
		switch {
		case req.URL.Host == "localhost" && req.URL.Path == "/.well-known/oauth-protected-resource/v2/mcp":
			body = `{"resource":"` + publicResource + `","authorization_servers":["https://auth.example.com"]}`
		case req.URL.Host == "auth.example.com" && req.URL.Path == "/.well-known/oauth-authorization-server":
			body = `{"issuer":"https://auth.example.com","authorization_endpoint":"https://auth.example.com/authorize","token_endpoint":"https://auth.example.com/token"}`
		default:
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})

	baseURL, err := mcpOAuthDiscoveryBaseURL(appURI)
	require.NoError(t, err)
	parsedBaseURL, err := url.Parse(baseURL)
	require.NoError(t, err)
	httpClient := &http.Client{Transport: &hostRoutingTransport{
		tunneled:        mockTransport,
		direct:          mockTransport,
		mcpServerOrigin: parsedBaseURL,
	}}
	handler := mcpclienttransport.NewOAuthHandler(mcpclienttransport.OAuthConfig{
		ClientID:    "pre-registered-client",
		RedirectURI: "http://localhost:12345/callback",
		PKCEEnabled: true,
		HTTPClient:  httpClient,
	})
	handler.SetBaseURL(baseURL)

	authorizationURL, err := handler.GetAuthorizationURL(t.Context(), "state", "challenge")
	require.NoError(t, err)
	require.Equal(t, []string{
		"http://localhost/.well-known/oauth-protected-resource/v2/mcp",
		"https://auth.example.com/.well-known/oauth-authorization-server",
	}, requests)

	parsedAuthorizationURL, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	require.Equal(t, "https://auth.example.com/authorize", parsedAuthorizationURL.Scheme+"://"+parsedAuthorizationURL.Host+parsedAuthorizationURL.Path)
	require.Equal(t, publicResource, parsedAuthorizationURL.Query().Get("resource"))
	require.NotContains(t, parsedAuthorizationURL.Query().Get("resource"), "localhost")
}

func TestHostRoutingTransportBoundsOAuthMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "within limit", size: 1024},
		{name: "over limit", size: teleport.MaxHTTPResponseSize + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			body := `{"issuer":"https://auth.example.com","padding":"` + strings.Repeat("x", test.size) + `"}`
			mcpServerOrigin, err := url.Parse("https://mcp.example.com/mcp")
			require.NoError(t, err)
			transport := &hostRoutingTransport{
				direct: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
				}),
				mcpServerOrigin: mcpServerOrigin,
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://auth.example.com/.well-known/oauth-authorization-server", nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)
			if test.wantErr {
				require.ErrorContains(t, err, "read limit")
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, body, string(got))
		})
	}
}

func TestHostRoutingTransportValidatesRedirectedProtectedResourceMetadata(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		resource string
		wantErr  string
	}{
		{name: "matching resource", resource: "https://mcp.example.com/mcp"},
		{name: "mismatched resource", resource: "https://other.example.com/mcp", wantErr: `expected "https://mcp.example.com/mcp"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mockTransport := mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if _, ok := oauthMetadataRootPath(req.URL.Path); ok {
					return &http.Response{
						StatusCode: http.StatusFound,
						Header:     http.Header{"Location": []string{"https://mcp.example.com/metadata.json"}},
						Body:       http.NoBody,
						Request:    req,
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"resource":"` + test.resource + `"}`)),
					Request:    req,
				}, nil
			})
			mcpServerOrigin, err := url.Parse("https://mcp.example.com/mcp")
			require.NoError(t, err)
			client := &http.Client{Transport: &hostRoutingTransport{
				tunneled:        mockTransport,
				direct:          mockTransport,
				mcpServerOrigin: mcpServerOrigin,
			}}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://mcp.example.com/.well-known/oauth-protected-resource/mcp", nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestMCPOAuthHTTPClientHonorsHTTPSProxy(t *testing.T) {
	proxyWasHit := false
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxyWasHit = true
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxyServer.Close)
	t.Setenv("HTTPS_PROXY", proxyServer.URL)
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	httpClient, err := newMCPOAuthHTTPClient(&client.MCPServerDialer{}, "mcp+https://mcp.example.com/mcp")
	require.NoError(t, err)
	httpClient.Timeout = 2 * time.Second

	resp, err := httpClient.Get("https://8.8.8.8/token")
	if resp != nil {
		require.NoError(t, resp.Body.Close())
	}
	require.Error(t, err)
	require.True(t, proxyWasHit)
}

func TestHostRoutingTransportRoutesRequests(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		proxy   string
		target  string
		wantVia string
		wantErr string
	}{
		{name: "MCP resource uses tunnel", target: "https://mcp.example.com/mcp", wantVia: "tunneled"},
		{name: "protected resource metadata uses tunnel", target: "https://mcp.example.com/.well-known/oauth-protected-resource/mcp", wantVia: "tunneled"},
		{name: "same-origin authorization metadata goes direct", target: "https://mcp.example.com/.well-known/oauth-authorization-server", wantVia: "direct"},
		{name: "same-origin OpenID metadata goes direct", target: "https://mcp.example.com/.well-known/openid-configuration", wantVia: "direct"},
		{name: "no proxy goes direct", target: "https://8.8.8.8/token", wantVia: "direct"},
		{name: "proxy is honored", proxy: "http://proxy.example.com:3128", target: "https://8.8.8.8/token", wantVia: "proxied"},
		{name: "proxied private target is rejected", proxy: "http://proxy.example.com:3128", target: "https://10.0.0.1/token", wantErr: "prohibited network address"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var via string
			record := func(name string) mcpOAuthRoundTripperFunc {
				return func(req *http.Request) (*http.Response, error) {
					via = name
					return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
				}
			}
			mcpServerOrigin, err := url.Parse("https://mcp.example.com/mcp")
			require.NoError(t, err)
			transport := &hostRoutingTransport{
				tunneled: record("tunneled"),
				direct:   record("direct"),
				proxied:  record("proxied"),
				proxy: func(*http.Request) (*url.URL, error) {
					if test.proxy == "" {
						return nil, nil
					}
					return url.Parse(test.proxy)
				},
				mcpServerOrigin: mcpServerOrigin,
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, test.target, nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Empty(t, via)
				return
			}
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, test.wantVia, via)
		})
	}
}

func TestMCPOAuthHTTPClientRefusesPOSTRedirects(t *testing.T) {
	t.Parallel()

	httpClient, err := newMCPOAuthHTTPClient(&client.MCPServerDialer{}, "mcp+https://mcp.example.com/mcp")
	require.NoError(t, err)
	var hosts []string
	routing := httpClient.Transport.(*hostRoutingTransport)
	routing.proxy = nil
	routing.direct = mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		hosts = append(hosts, req.URL.Host)
		return &http.Response{
			StatusCode: http.StatusTemporaryRedirect,
			Header:     http.Header{"Location": []string{"https://evil.example.com/token"}},
			Body:       http.NoBody,
			Request:    req,
		}, nil
	})

	resp, err := httpClient.Post("https://auth.example.com/token", "application/x-www-form-urlencoded", strings.NewReader("code=secret"))
	if resp != nil {
		require.NoError(t, resp.Body.Close())
	}
	require.ErrorContains(t, err, "refusing to follow a redirect")
	require.Equal(t, []string{"auth.example.com"}, hosts)
}

func TestHostRoutingTransportBoundsResponses(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"https://mcp.example.com/mcp", "https://auth.example.com/token"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			oversized := mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", teleport.MaxHTTPResponseSize+1))),
					Request:    req,
				}, nil
			})
			mcpServerOrigin, err := url.Parse("https://mcp.example.com/mcp")
			require.NoError(t, err)
			transport := &hostRoutingTransport{
				tunneled:        oversized,
				direct:          oversized,
				mcpServerOrigin: mcpServerOrigin,
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, target, nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			_, err = io.ReadAll(resp.Body)
			require.ErrorIs(t, err, utils.ErrLimitReached)
		})
	}
}

func TestHostRoutingTransportValidatesIssuer(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		metadataURL string
		issuer      string
		wantErr     string
	}{
		{name: "root issuer", metadataURL: "https://auth.example.com/.well-known/oauth-authorization-server", issuer: "https://auth.example.com"},
		{name: "root issuer with trailing slash", metadataURL: "https://auth.example.com/.well-known/openid-configuration/", issuer: "https://auth.example.com/"},
		{name: "trailing slash mismatch", metadataURL: "https://auth.example.com/.well-known/openid-configuration", issuer: "https://auth.example.com/", wantErr: `identifies issuer "https://auth.example.com/", expected "https://auth.example.com"`},
		{name: "path-aware issuer", metadataURL: "https://auth.example.com/.well-known/oauth-authorization-server/tenant", issuer: "https://auth.example.com/tenant"},
		{name: "impersonated issuer", metadataURL: "https://attacker.example.com/.well-known/oauth-authorization-server", issuer: "https://auth.example.com", wantErr: `identifies issuer "https://auth.example.com", expected "https://attacker.example.com"`},
		{name: "wrong tenant", metadataURL: "https://auth.example.com/.well-known/oauth-authorization-server/tenant", issuer: "https://auth.example.com/other", wantErr: "identifies issuer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mcpServerOrigin, err := url.Parse("https://mcp.example.com/mcp")
			require.NoError(t, err)
			transport := &hostRoutingTransport{
				direct: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"issuer":"` + test.issuer + `"}`)),
						Request:    req,
					}, nil
				}),
				mcpServerOrigin: mcpServerOrigin,
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, test.metadataURL, nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestHostRoutingTransportOAuthMetadataRootFallback(t *testing.T) {
	t.Parallel()

	const (
		pathAwarePath = "/.well-known/oauth-protected-resource/mcp"
		rootPath      = "/.well-known/oauth-protected-resource"
		resourceURI   = "https://mcp.example.com/mcp"
	)
	tests := []struct {
		name            string
		pathAwareStatus int
		resource        string
		wantPaths       []string
		wantErr         string
	}{
		{
			name:            "unauthorized",
			pathAwareStatus: http.StatusUnauthorized,
			resource:        resourceURI,
			wantPaths:       []string{pathAwarePath, rootPath},
		},
		{
			name:            "forbidden",
			pathAwareStatus: http.StatusForbidden,
			resource:        resourceURI,
			wantPaths:       []string{pathAwarePath, rootPath},
		},
		{
			name:            "mismatched resource",
			pathAwareStatus: http.StatusOK,
			resource:        "https://other.example.com/mcp",
			wantPaths:       []string{pathAwarePath},
			wantErr:         "expected \"https://mcp.example.com/mcp\"",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var paths []string
			mockTransport := mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				statusCode := test.pathAwareStatus
				if req.URL.Path == rootPath {
					statusCode = http.StatusOK
				}
				body := http.StatusText(statusCode)
				if statusCode == http.StatusOK {
					body = `{"resource":"` + test.resource + `"}`
				}
				return &http.Response{
					StatusCode: statusCode,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    req,
				}, nil
			})
			mcpServerOrigin, err := url.Parse(resourceURI)
			require.NoError(t, err)
			transport := &hostRoutingTransport{
				tunneled:        mockTransport,
				direct:          mockTransport,
				mcpServerOrigin: mcpServerOrigin,
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://mcp.example.com"+pathAwarePath, nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, resp)
				require.Equal(t, test.wantPaths, paths)
				return
			}
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, test.wantPaths, paths)
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestFetchMCPOAuthChallengeScopes(t *testing.T) {
	t.Parallel()

	resourceURL, err := url.Parse("https://mcp.example.com/v2/mcp")
	require.NoError(t, err)
	httpClient := &http.Client{Transport: &hostRoutingTransport{
		mcpServerOrigin: resourceURL,
		tunneled: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodPost, req.Method)
			require.Equal(t, "http://localhost/v2/mcp", req.URL.String())
			require.Equal(t, "application/json, text/event-stream", req.Header.Get("Accept"))
			var request struct {
				Method string `json:"method"`
			}
			require.NoError(t, json.NewDecoder(req.Body).Decode(&request))
			require.Equal(t, "initialize", request.Method)
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header: http.Header{
					"Www-Authenticate": []string{`Bearer realm="mcp", scope="mcp:tools,read mcp:resources"`},
				},
				Body:    http.NoBody,
				Request: req,
			}, nil
		}),
		direct: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.FailNow(t, "scope probe bypassed the Teleport tunnel", req.URL.String())
			return nil, nil
		}),
	}}

	status, scopes, err := fetchMCPOAuthChallengeScopes(t.Context(), httpClient, resourceURL.String())
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, status)
	require.Equal(t, []string{"mcp:tools,read", "mcp:resources"}, scopes)
}

func TestFetchMCPOAuthChallengeScopesNoChallenge(t *testing.T) {
	t.Parallel()

	resourceURL, err := url.Parse("https://mcp.example.com/mcp")
	require.NoError(t, err)
	httpClient := &http.Client{Transport: &hostRoutingTransport{
		mcpServerOrigin: resourceURL,
		tunneled: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
		}),
	}}

	status, scopes, err := fetchMCPOAuthChallengeScopes(t.Context(), httpClient, resourceURL.String())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, scopes)
}

func TestFetchAdvertisedMCPOAuthScopes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		wantScopes []string
		wantErr    bool
	}{
		{
			name:       "scopes advertised",
			statusCode: http.StatusOK,
			body:       `{"resource":"https://mcp.example.com/v2/mcp","scopes_supported":["mcp:tools","mcp:resources"]}`,
			wantScopes: []string{"mcp:tools", "mcp:resources"},
		},
		{
			name:       "no scopes advertised",
			statusCode: http.StatusOK,
			body:       `{"resource":"https://mcp.example.com/v2/mcp"}`,
		},
		{
			name:       "metadata unavailable",
			statusCode: http.StatusNotFound,
			body:       "not found",
			wantErr:    true,
		},
		{
			name:       "malformed metadata",
			statusCode: http.StatusOK,
			body:       "not json",
			wantErr:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			httpClient := &http.Client{Transport: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "https://mcp.example.com/.well-known/oauth-protected-resource/v2/mcp", req.URL.String())
				return &http.Response{
					StatusCode: test.statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(test.body)),
					Request:    req,
				}, nil
			})}

			scopes, err := fetchAdvertisedMCPOAuthScopes(t.Context(), httpClient, "https://mcp.example.com/v2/mcp")
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.wantScopes, scopes)
		})
	}
}

func TestMCPOAuthCallbackHandler(t *testing.T) {
	callbackCh := make(chan url.Values, 1)
	handler := mcpOAuthCallbackHandler("expected", callbackCh)
	get := func(target string) int {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code
	}

	// A request without our state must not take the slot from the real response.
	require.Equal(t, http.StatusBadRequest, get("/callback?state=wrong&error=access_denied"))
	require.Empty(t, callbackCh)

	require.Equal(t, http.StatusOK, get("/callback?state=expected&code=first"))
	// A duplicate is dropped without blocking the handler.
	require.Equal(t, http.StatusOK, get("/callback?state=expected&code=second"))
	require.Equal(t, "first", (<-callbackCh).Get("code"))
	require.Empty(t, callbackCh)
}
