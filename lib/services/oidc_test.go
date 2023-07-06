/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"testing"

	"github.com/coreos/go-oidc/jose"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

// TestOIDCRoleMapping verifies basic mapping from OIDC claims to roles.
func TestOIDCRoleMapping(t *testing.T) {
	// create a connector
	oidcConnector, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:     "https://www.exmaple.com",
		ClientID:      "example-client-id",
		ClientSecret:  "example-client-secret",
		Display:       "sign in with example.com",
		Scope:         []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"user"}}},
		RedirectURLs:  []string{"https://localhost:3080/webapi/oidc/callback"},
	})
	require.NoError(t, err)

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	traits := OIDCClaimsToTraits(claims)
	require.Len(t, traits, 4)

	_, roles := TraitsToRoles(oidcConnector.GetTraitMappings(), traits)
	require.Len(t, roles, 1)
	require.Equal(t, "user", roles[0])
}

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
					"redirect_url": "https://localhost:3080/webapi/oidc/callback"
				}
			}`,
			expectSpec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClientSecret:  "secret-key-from-google",
				Display:       "whatever",
				Scope:         []string{"roles"},
				Prompt:        "consent login",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs:  []string{"https://localhost:3080/webapi/oidc/callback"},
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
					"claims_to_roles": [
						{
							"claim": "roles",
							"value": "teleport-user",
							"roles": ["dictator"]
						}
					],
					"redirect_url": [
						"https://localhost:3080/webapi/oidc/callback",
						"https://proxy.example.com/webapi/oidc/callback",
						"https://other.proxy.example.com/webapi/oidc/callback"
					]
				}
			}`,
			expectSpec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs: []string{
					"https://localhost:3080/webapi/oidc/callback",
					"https://proxy.example.com/webapi/oidc/callback",
					"https://other.proxy.example.com/webapi/oidc/callback",
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
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs:  []string{"https://localhost:3080/webapi/oidc/callback"},
			},
			expect: func(t *testing.T, c types.OIDCConnector, err error) {
				require.NoError(t, err)
				require.Equal(t, types.V3, c.GetVersion())
				require.Equal(t, types.KindOIDCConnector, c.GetKind())
				require.Equal(t, "google", c.GetName())
				require.Equal(t, "id-from-google.apps.googleusercontent.com", c.GetClientID())
				require.Equal(t, []string{"https://localhost:3080/webapi/oidc/callback"}, c.GetRedirectURLs())
				require.Equal(t, constants.OIDCPromptSelectAccount, c.GetPrompt())
			},
		}, {
			desc: "omit prompt",
			spec: types.OIDCConnectorSpecV3{
				ClientID:      "id-from-google.apps.googleusercontent.com",
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
				RedirectURLs: []string{
					"https://localhost:3080/webapi/oidc/callback",
					"https://proxy.example.com/webapi/oidc/callback",
					"https://other.proxy.example.com/webapi/oidc/callback",
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
				ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user"}},
				RedirectURLs: []string{
					"https://localhost:3080/webapi/oidc/callback",
					"https://proxy.example.com/webapi/oidc/callback",
					"https://other.proxy.example.com/webapi/oidc/callback",
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
		ClaimsToRoles: []types.ClaimMapping{{Claim: "roles", Value: "teleport-user", Roles: []string{"dictator"}}},
		RedirectURLs: []string{
			"https://proxy.example.com/webapi/oidc/callback",
			"https://other.example.com/webapi/oidc/callback",
			"https://other.example.com:443/webapi/oidc/callback",
			"https://other.example.com:3080/webapi/oidc/callback",
			"https://eu.proxy.example.com/webapi/oidc/callback",
			"https://us.proxy.example.com:443/webapi/oidc/callback",
		},
	})
	require.NoError(t, err)

	expectedMapping := map[string]string{
		"proxy.example.com":         "https://proxy.example.com/webapi/oidc/callback",
		"proxy.example.com:443":     "https://proxy.example.com/webapi/oidc/callback",
		"other.example.com":         "https://other.example.com/webapi/oidc/callback",
		"other.example.com:80":      "https://other.example.com/webapi/oidc/callback",
		"other.example.com:443":     "https://other.example.com:443/webapi/oidc/callback",
		"other.example.com:3080":    "https://other.example.com:3080/webapi/oidc/callback",
		"eu.proxy.example.com":      "https://eu.proxy.example.com/webapi/oidc/callback",
		"eu.proxy.example.com:443":  "https://eu.proxy.example.com/webapi/oidc/callback",
		"eu.proxy.example.com:3080": "https://eu.proxy.example.com/webapi/oidc/callback",
		"us.proxy.example.com":      "https://us.proxy.example.com:443/webapi/oidc/callback",
		"notfound.example.com":      "https://proxy.example.com/webapi/oidc/callback",
	}

	for proxyAddr, redirectURL := range expectedMapping {
		t.Run(proxyAddr, func(t *testing.T) {
			url, err := GetRedirectURL(conn, proxyAddr)
			require.NoError(t, err)
			require.Equal(t, redirectURL, url)
		})
	}
}
