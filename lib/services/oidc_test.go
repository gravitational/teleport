/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

// TestOIDCUnmarshal tests UnmarshalOIDCConnector
func TestOIDCUnmarshal(t *testing.T) {
	for _, tc := range []struct {
		desc       string
		input      string
		expectErr  bool
		expectSpec types.OIDCConnectorSpecV3
	}{
		{
			desc: "basic connector",
			input: `{
				"version": "v3",
				"kind": "oidc",
				"metadata": {
					"name": "google"
				},
				"spec": {
					"client_id": "id-from-google.apps.googleusercontent.com",
					"client_secret": "secret-key-from-google",
					"display": "whatever",
					"scope": ["roles"],
					"prompt": "consent login",
					"claims_to_roles": [
						{
							"claim": "roles",
							"value": "teleport-user",
							"roles": ["dictator"]
						}
					],
					"redirect_url": "https://localhost:3080/v1/webapi/oidc/callback"
				}
			}`,
			expectSpec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClientSecret:  "secret-key-from-google",
				Display:       "whatever",
				Scope:         []string{"roles"},
				Prompt:        "consent login",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs:  []string{"https://localhost:3080/v1/webapi/oidc/callback"},
			},
		}, {
			desc: "multiple redirect urls",
			input: `{
				"version": "v3",
				"kind": "oidc",
				"metadata": {
					"name": "google"
				},
				"spec": {
					"client_id": "id-from-google.apps.googleusercontent.com",
					"client_secret": "secret-key-from-google",
					"claims_to_roles": [
						{
							"claim": "roles",
							"value": "teleport-user",
							"roles": ["dictator"]
						}
					],
					"redirect_url": [
						"https://localhost:3080/v1/webapi/oidc/callback",
						"https://proxy.example.com/v1/webapi/oidc/callback",
						"https://other.proxy.example.com/v1/webapi/oidc/callback"
					]
				}
			}`,
			expectSpec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClientSecret:  "secret-key-from-google",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs: []string{
					"https://localhost:3080/v1/webapi/oidc/callback",
					"https://proxy.example.com/v1/webapi/oidc/callback",
					"https://other.proxy.example.com/v1/webapi/oidc/callback",
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			connector, err := UnmarshalOIDCConnector([]byte(tc.input))
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			expectedConnector, err := types.NewOIDCConnector("google", tc.expectSpec)
			require.NoError(t, err)
			require.Equal(t, expectedConnector, connector)
		})
	}
}

func TestOIDCCheckAndSetDefaults(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		spec   types.OIDCConnectorSpecV3
		expect func(*testing.T, types.OIDCConnector, error)
	}{
		{
			desc: "basic spec and defaults",
			spec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClientSecret:  "some-client-secret",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs:  []string{"https://localhost:3080/v1/webapi/oidc/callback"},
			},
			expect: func(t *testing.T, c types.OIDCConnector, err error) {
				require.NoError(t, err)
				require.Equal(t, types.V3, c.GetVersion())
				require.Equal(t, types.KindOIDCConnector, c.GetKind())
				require.Equal(t, "google", c.GetName())
				require.Equal(t, "id-from-google.apps.googleusercontent.com", c.GetClientID())
				require.Equal(t, []string{"https://localhost:3080/v1/webapi/oidc/callback"}, c.GetRedirectURLs())
				require.Equal(t, constants.OIDCPromptSelectAccount, c.GetPrompt())
			},
		}, {
			desc: "omit prompt",
			spec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClientSecret:  "some-client-secret",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs: []string{
					"https://localhost:3080/v1/webapi/oidc/callback",
					"https://proxy.example.com/v1/webapi/oidc/callback",
					"https://other.proxy.example.com/v1/webapi/oidc/callback",
				},
				Prompt: "none",
			},
			expect: func(t *testing.T, c types.OIDCConnector, err error) {
				require.NoError(t, err)
				require.Equal(t, "", c.GetPrompt())
			},
		}, {
			desc: "invalid claims to roles",
			spec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClientSecret:  "some-client-secret",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user"}},
				RedirectURLs: []string{
					"https://localhost:3080/v1/webapi/oidc/callback",
					"https://proxy.example.com/v1/webapi/oidc/callback",
					"https://other.proxy.example.com/v1/webapi/oidc/callback",
				},
				Prompt: "none",
			},
			expect: func(t *testing.T, c types.OIDCConnector, err error) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			connector, err := types.NewOIDCConnector("google", tc.spec)
			tc.expect(t, connector, err)
		})
	}
}

func TestOIDCGetRedirectURL(t *testing.T) {
	conn, err := types.NewOIDCConnector("oidc", types.OIDCConnectorSpecV3{
		ClientID:      "id-from-google.apps.googleusercontent.com",
		ClientSecret:  "some-client-secret",
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs: []string{
			"https://proxy.example.com/v1/webapi/oidc/callback",
			"https://other.example.com/v1/webapi/oidc/callback",
			"https://other.example.com:443/v1/webapi/oidc/callback",
			"https://other.example.com:3080/v1/webapi/oidc/callback",
			"https://eu.proxy.example.com/v1/webapi/oidc/callback",
			"https://us.proxy.example.com:443/v1/webapi/oidc/callback",
		},
	})
	require.NoError(t, err)

	expectedMapping := map[string]string{
		"proxy.example.com":         "https://proxy.example.com/v1/webapi/oidc/callback",
		"proxy.example.com:443":     "https://proxy.example.com/v1/webapi/oidc/callback",
		"other.example.com":         "https://other.example.com/v1/webapi/oidc/callback",
		"other.example.com:80":      "https://other.example.com/v1/webapi/oidc/callback",
		"other.example.com:443":     "https://other.example.com:443/v1/webapi/oidc/callback",
		"other.example.com:3080":    "https://other.example.com:3080/v1/webapi/oidc/callback",
		"eu.proxy.example.com":      "https://eu.proxy.example.com/v1/webapi/oidc/callback",
		"eu.proxy.example.com:443":  "https://eu.proxy.example.com/v1/webapi/oidc/callback",
		"eu.proxy.example.com:3080": "https://eu.proxy.example.com/v1/webapi/oidc/callback",
		"us.proxy.example.com":      "https://us.proxy.example.com:443/v1/webapi/oidc/callback",
		"notfound.example.com":      "https://proxy.example.com/v1/webapi/oidc/callback",
	}

	for proxyAddr, redirectURL := range expectedMapping {
		t.Run(proxyAddr, func(t *testing.T) {
			url, err := GetRedirectURL(conn, proxyAddr)
			require.NoError(t, err)
			require.Equal(t, redirectURL, url)
		})
	}
}
