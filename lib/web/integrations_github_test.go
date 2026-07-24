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

package web

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
)

func TestGithubIntegrationCallback(t *testing.T) {
	t.Parallel()

	const user = "callback-test-user"

	secretKey, err := secret.NewKey()
	require.NoError(t, err)
	clientRedirectURL := "http://127.0.0.1:12345/callback?secret_key=" + hex.EncodeToString([]byte(secretKey))

	mockCallbackResponse := &authclient.GithubAuthResponse{
		Username: user,
		Req: authclient.GithubAuthRequest{
			ClientRedirectURL: clientRedirectURL,
		},
	}
	wPack := newWebPack(t, 1, withWebPackProxyOptions(
		withValidateGithubAuthCallback(func(ctx context.Context, q url.Values) (*authclient.GithubAuthResponse, error) {
			return mockCallbackResponse, nil
		}),
	))
	proxy := wPack.proxies[0]
	pack := proxy.authPack(t, user, []types.Role{services.NewPresetEditorRole()})
	callbackEndpoint := pack.clt.Endpoint("webapi", "github", "integration", "callback")

	tests := []struct {
		name              string
		reqBody           map[string]string
		authRequest       *types.GithubAuthRequest
		wantErr           bool
		wantErrContains   string
		wantRedirectErr   bool
		wantRedirectMatch string
		wantSuccess       bool
	}{
		{
			name: "missing code",
			reqBody: map[string]string{
				"code": "some-code",
			},
			wantErr:         true,
			wantErrContains: "missing code or state",
		},
		{
			name: "missing state",
			reqBody: map[string]string{
				"state": "some-state",
			},
			wantErr:         true,
			wantErrContains: "missing code or state",
		},
		{
			name: "invalid state token",
			reqBody: map[string]string{
				"code":  "some-code",
				"state": "nonexistent-state",
			},
			wantErr: true,
		},
		{
			name: "auth request not for authenticated user",
			authRequest: &types.GithubAuthRequest{
				ConnectorID:       "test-connector",
				ClientRedirectURL: "http://127.0.0.1:12345/callback",
			},
			wantRedirectErr:   true,
			wantRedirectMatch: "not for an authenticated user",
		},
		{
			name: "session user does not match",
			authRequest: &types.GithubAuthRequest{
				ConnectorID:       "test-connector",
				AuthenticatedUser: "attacker",
				ClientRedirectURL: "http://127.0.0.1:12345/callback",
				ConnectorSpec: &types.GithubConnectorSpecV3{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "https://proxy.example.com/web/github/integration/callback",
				},
			},
			wantRedirectErr:   true,
			wantRedirectMatch: "does not match the user who initiated the OAuth flow",
		},
		{
			name: "success",
			authRequest: &types.GithubAuthRequest{
				ConnectorID:       "test-connector",
				AuthenticatedUser: user,
				ClientRedirectURL: "http://127.0.0.1:12345/callback",
				ConnectorSpec: &types.GithubConnectorSpecV3{
					ClientID:     "test-client-id",
					ClientSecret: "test-client-secret",
					RedirectURL:  "https://proxy.example.com/web/github/integration/callback",
				},
			},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := tt.reqBody
			if tt.authRequest != nil {
				stateToken := uuid.NewString()
				tt.authRequest.StateToken = stateToken
				tt.authRequest.SetExpiry(time.Now().Add(10 * time.Minute))

				err := wPack.server.Auth().Services.CreateGithubAuthRequest(t.Context(), *tt.authRequest)
				require.NoError(t, err)

				reqBody = map[string]string{
					"code":  "some-code",
					"state": stateToken,
				}
			}

			resp, err := pack.clt.PostJSON(t.Context(), callbackEndpoint, reqBody)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					require.Contains(t, err.Error(), tt.wantErrContains)
				}
				return
			}

			require.NoError(t, err)
			var callbackResp githubIntegrationCallbackResponse
			require.NoError(t, json.Unmarshal(resp.Bytes(), &callbackResp))

			if tt.wantRedirectErr {
				redirectURL, parseErr := url.Parse(callbackResp.RedirectURL)
				require.NoError(t, parseErr)
				errParam := redirectURL.Query().Get("err")
				require.NotEmpty(t, errParam)
				if tt.wantRedirectMatch != "" {
					require.Contains(t, errParam, tt.wantRedirectMatch)
				}
				return
			}

			if tt.wantSuccess {
				require.NotEmpty(t, callbackResp.RedirectURL)
				redirectURL, parseErr := url.Parse(callbackResp.RedirectURL)
				require.NoError(t, parseErr)
				require.Equal(t, "127.0.0.1:12345", redirectURL.Host)
				require.Empty(t, redirectURL.Query().Get("err"))
				return
			}
		})
	}
}
