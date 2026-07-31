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
			want:   "http://localhost",
		},
		{
			name:   "standard MCP path",
			appURI: "mcp+https://mcp.example.com/mcp",
			want:   "http://localhost/mcp",
		},
		{
			name:   "nested provider path",
			appURI: "mcp+https://mcp.example.com/v2/mcp?tenant=ignored#fragment",
			want:   "http://localhost/v2/mcp",
		},
		{
			name:   "escaped path",
			appURI: "mcp+https://mcp.example.com/tenant%2Fone/mcp",
			want:   "http://localhost/tenant%2Fone/mcp",
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
	httpClient := &http.Client{Transport: mcpOAuthRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
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
	})}

	baseURL, err := mcpOAuthDiscoveryBaseURL(appURI)
	require.NoError(t, err)
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
