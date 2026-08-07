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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/prompt"
)

type mcpOAuthRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mcpOAuthRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWrapMCPClientRegistrationError(t *testing.T) {
	t.Parallel()

	require.NoError(t, wrapMCPClientRegistrationError(nil, "databricks-sql"))

	// The exact error returned by mcp-go when the authorization server
	// metadata has no registration_endpoint.
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

// TestWrapMCPClientRegistrationErrorRejected covers providers that advertise a
// registration endpoint but only allow an approved set of MCP clients to
// register, so the POST is refused rather than failing to be processed.
func TestWrapMCPClientRegistrationErrorRejected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		rejected bool
	}{
		{
			name: "OAuth error body",
			// How mcp-go reports a registration response carrying an OAuth
			// error body, whatever the status code.
			err: fmt.Errorf("registration request failed: %w", mcpclienttransport.OAuthError{
				ErrorCode:        "invalid_client_metadata",
				ErrorDescription: "client_name is not allowed",
			}),
			rejected: true,
		},
		{
			name:     "403 without an OAuth error body",
			err:      errors.New(`registration request failed with status 403: Forbidden`),
			rejected: true,
		},
		{
			name:     "401 without an OAuth error body",
			err:      errors.New(`registration request failed with status 401: Unauthorized`),
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
			// The underlying error is always preserved.
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

func TestResolveMCPOAuthClientName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		flag            string
		preferredClient string
		want            string
	}{
		{
			name: "defaults to tsh",
			want: defaultMCPOAuthClientName,
		},
		{
			name:            "tsh config preferred client",
			preferredClient: "Acme Gateway",
			want:            "Acme Gateway",
		},
		{
			name:            "flag wins over tsh config",
			flag:            "Acme Gateway v2",
			preferredClient: "Acme Gateway",
			want:            "Acme Gateway v2",
		},
		{
			name:            "blank flag falls back to tsh config",
			flag:            "   ",
			preferredClient: "Acme Gateway",
			want:            "Acme Gateway",
		},
		{
			name:            "blank tsh config falls back to tsh",
			preferredClient: "   ",
			want:            defaultMCPOAuthClientName,
		},
		{
			name: "surrounding whitespace is trimmed",
			flag: "  Acme Gateway  ",
			want: "Acme Gateway",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.want, resolveMCPOAuthClientName(test.flag, test.preferredClient))
		})
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
		case req.URL.Host == "mcp.example.com" && req.URL.Path == "/.well-known/oauth-protected-resource/v2/mcp":
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
	// Same wiring as newMCPOAuthHTTPClient: metadata requests for the MCP
	// server origin are rewritten to the tunnel's localhost address.
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
		"https://mcp.example.com/.well-known/oauth-protected-resource/v2/mcp",
		"https://auth.example.com/.well-known/oauth-authorization-server",
	}, requests)

	parsedAuthorizationURL, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	require.Equal(t, "https://auth.example.com/authorize", parsedAuthorizationURL.Scheme+"://"+parsedAuthorizationURL.Host+parsedAuthorizationURL.Path)
	require.Equal(t, publicResource, parsedAuthorizationURL.Query().Get("resource"))
	require.NotContains(t, parsedAuthorizationURL.Query().Get("resource"), "localhost")
}

func TestMCPLoginOAuthClientCredentials(t *testing.T) {
	newCommand := func() *mcpLoginCommand {
		return &mcpLoginCommand{
			cf: &CLIConf{
				Context:        t.Context(),
				overrideStderr: &bytes.Buffer{},
			},
		}
	}

	t.Run("dynamic registration", func(t *testing.T) {
		clientID, clientSecret, err := newCommand().getOAuthClientCredentials()
		require.NoError(t, err)
		require.Empty(t, clientID)
		require.Empty(t, clientSecret)
	})

	t.Run("public pre-registered client", func(t *testing.T) {
		cmd := newCommand()
		cmd.clientID = "client-id"

		clientID, clientSecret, err := cmd.getOAuthClientCredentials()
		require.NoError(t, err)
		require.Equal(t, "client-id", clientID)
		require.Empty(t, clientSecret)
	})

	t.Run("prompt for confidential client secret", func(t *testing.T) {
		oldStdin := prompt.Stdin()
		t.Cleanup(func() {
			prompt.SetStdin(oldStdin)
		})
		prompt.SetStdin(prompt.NewFakeReader().AddString("client-secret"))

		cmd := newCommand()
		cmd.clientID = "client-id"
		cmd.promptSecret = true

		clientID, clientSecret, err := cmd.getOAuthClientCredentials()
		require.NoError(t, err)
		require.Equal(t, "client-id", clientID)
		require.Equal(t, "client-secret", clientSecret)
	})

	t.Run("secret requires client ID", func(t *testing.T) {
		cmd := newCommand()
		cmd.promptSecret = true

		_, _, err := cmd.getOAuthClientCredentials()
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "requires --client-id")
	})

	t.Run("empty client secret", func(t *testing.T) {
		oldStdin := prompt.Stdin()
		t.Cleanup(func() {
			prompt.SetStdin(oldStdin)
		})
		prompt.SetStdin(prompt.NewFakeReader().AddString(""))

		cmd := newCommand()
		cmd.clientID = "client-id"
		cmd.promptSecret = true

		_, _, err := cmd.getOAuthClientCredentials()
		require.True(t, trace.IsBadParameter(err))
		require.ErrorContains(t, err, "client secret is empty")
	})
}
