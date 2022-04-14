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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"

	"github.com/stretchr/testify/require"
)

// Verify that an OIDC connector with no mappings produces no roles.
func TestOIDCRoleMappingEmpty(t *testing.T) {
	// create a connector
	oidcConnector, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
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
	require.Len(t, roles, 0)
}

// TestOIDCRoleMapping verifies basic mapping from OIDC claims to roles.
func TestOIDCRoleMapping(t *testing.T) {
	// create a connector
	oidcConnector, err := types.NewOIDCConnector("example", types.OIDCConnectorSpecV3{
		IssuerURL:    "https://www.exmaple.com",
		ClientID:     "example-client-id",
		ClientSecret: "example-client-secret",
		RedirectURLs: []string{"https://localhost:3080/v1/webapi/oidc/callback"},
		Display:      "sign in with example.com",
		Scope:        []string{"foo", "bar"},
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "roles",
				Value: "teleport-user",
				Roles: []string{"user"},
			},
		},
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

// TestOIDCUnmarshal tests unmarshal of OIDC connector
func TestOIDCUnmarshal(t *testing.T) {
	input := `
      {
        "kind": "oidc",
        "version": "v2",
        "metadata": {
          "name": "google"
        },
        "spec": {
          "issuer_url": "https://accounts.google.com",
          "client_id": "id-from-google.apps.googleusercontent.com",
          "client_secret": "secret-key-from-google",
          "redirect_url": "https://localhost:3080/v1/webapi/oidc/callback",
          "display": "whatever",
          "scope": ["roles"],
		  "claims_to_roles": [{
            "claim": "roles",
            "value": "teleport-user",
            "roles": ["dictator"]
          }],
          "prompt": "consent login"
        }
      }
	`

	oc, err := UnmarshalOIDCConnector([]byte(input))
	require.NoError(t, err)

	require.Equal(t, "google", oc.GetName())
	require.Equal(t, "https://accounts.google.com", oc.GetIssuerURL())
	require.Equal(t, "id-from-google.apps.googleusercontent.com", oc.GetClientID())
	require.Equal(t, []string{"https://localhost:3080/v1/webapi/oidc/callback"}, oc.GetRedirectURLs())
	require.Equal(t, "whatever", oc.GetDisplay())
	require.Equal(t, "consent login", oc.GetPrompt())
}

// TestOIDCUnmarshalOmitPrompt makes sure that that setting
// prompt value to none will omit the prompt value.
func TestOIDCUnmarshalOmitPrompt(t *testing.T) {
	input := `
      {
        "kind": "oidc",
        "version": "v2",
        "metadata": {
          "name": "google"
        },
        "spec": {
          "issuer_url": "https://accounts.google.com",
          "client_id": "id-from-google.apps.googleusercontent.com",
          "client_secret": "secret-key-from-google",
          "redirect_url": "https://localhost:3080/v1/webapi/oidc/callback",
          "display": "whatever",
          "scope": ["roles"],
          "prompt": "none",
          "claims_to_roles": [
             {
                "claim": "email",
                "value": "*",
                "roles": [
                   "access"
                ]
             }
          ]
        }
      }
	`

	oc, err := UnmarshalOIDCConnector([]byte(input))
	require.NoError(t, err)

	require.Equal(t, "google", oc.GetName())
	require.Equal(t, "https://accounts.google.com", oc.GetIssuerURL())
	require.Equal(t, "id-from-google.apps.googleusercontent.com", oc.GetClientID())
	require.Equal(t, []string{"https://localhost:3080/v1/webapi/oidc/callback"}, oc.GetRedirectURLs())
	require.Equal(t, "whatever", oc.GetDisplay())
	require.Equal(t, "", oc.GetPrompt())
}

// TestOIDCUnmarshalOmitPrompt makes sure that an
// empty prompt value will default to select account.
func TestOIDCUnmarshalPromptDefault(t *testing.T) {
	input := `
      {
        "kind": "oidc",
        "version": "v2",
        "metadata": {
          "name": "google"
        },
        "spec": {
          "issuer_url": "https://accounts.google.com",
          "client_id": "id-from-google.apps.googleusercontent.com",
          "client_secret": "secret-key-from-google",
          "redirect_url": "https://localhost:3080/v1/webapi/oidc/callback",
          "display": "whatever",
          "scope": ["roles"],
          "claims_to_roles": [
             {
                "claim": "email",
                "value": "*",
                "roles": [
                   "access"
                ]
             }
          ]
        }
      }
	`

	oc, err := UnmarshalOIDCConnector([]byte(input))
	require.NoError(t, err)

	require.Equal(t, "google", oc.GetName())
	require.Equal(t, "https://accounts.google.com", oc.GetIssuerURL())
	require.Equal(t, "id-from-google.apps.googleusercontent.com", oc.GetClientID())
	require.Equal(t, []string{"https://localhost:3080/v1/webapi/oidc/callback"}, oc.GetRedirectURLs())
	require.Equal(t, "whatever", oc.GetDisplay())
	require.Equal(t, teleport.OIDCPromptSelectAccount, oc.GetPrompt())
}

// TestOIDCUnmarshalInvalid unmarshals and fails validation of the connector
func TestOIDCUnmarshalInvalid(t *testing.T) {
	// Test missing roles in claims_to_roles
	input := `
      {
        "kind": "oidc",
        "version": "v2",
        "metadata": {
          "name": "google"
        },
        "spec": {
          "issuer_url": "https://accounts.google.com",
          "client_id": "id-from-google.apps.googleusercontent.com",
          "client_secret": "secret-key-from-google",
          "redirect_url": "https://localhost:3080/v1/webapi/oidc/callback",
          "display": "whatever",
          "scope": ["roles"],
          "claims_to_roles": [{
            "claim": "roles",
            "value": "teleport-user"
          }]
        }
      }
	`

	_, err := UnmarshalOIDCConnector([]byte(input))
	require.Error(t, err)
}

func TestOIDCGetRedirectURL(t *testing.T) {
	conn, err := types.NewOIDCConnector("oidc", types.OIDCConnectorSpecV3{
		ClientID: "clientID",
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
