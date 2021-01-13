/*
Copyright 2017-2020 Gravitational, Inc.

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
package types

import (
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"

	"github.com/stretchr/testify/require"
)

// TestOIDCUnmarshal tests unmarshal of OIDC connector
func TestOIDCUnmarshal(t *testing.T) {
	utils.InitLoggerForTests(testing.Verbose())
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
          }]
        }
      }
	`

	oc, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	require.NoError(t, err)

	require.Equal(t, "google", oc.GetName())
	require.Equal(t, "https://accounts.google.com", oc.GetIssuerURL())
	require.Equal(t, "id-from-google.apps.googleusercontent.com", oc.GetClientID())
	require.Equal(t, "https://localhost:3080/v1/webapi/oidc/callback", oc.GetRedirectURL())
	require.Equal(t, "whatever", oc.GetDisplay())
	require.Equal(t, constants.OIDCPromptSelectAccount, oc.GetPrompt())
}

// TestOIDCUnmarshalEmptyPrompt makes sure that empty prompt value
// that is set does not default to select_account
func TestOIDCUnmarshalEmptyPrompt(t *testing.T) {
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
          "prompt": ""
        }
      }
	`

	oc, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	require.NoError(t, err)

	require.Equal(t, "google", oc.GetName())
	require.Equal(t, "https://accounts.google.com", oc.GetIssuerURL())
	require.Equal(t, "id-from-google.apps.googleusercontent.com", oc.GetClientID())
	require.Equal(t, "https://localhost:3080/v1/webapi/oidc/callback", oc.GetRedirectURL())
	require.Equal(t, "whatever", oc.GetDisplay())
	require.Equal(t, "", oc.GetPrompt())
}

// TestUnmarshalOIDCPromptValue makes sure that prompt value is set properly
func TestOIDCUnmarshalPromptValue(t *testing.T) {
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
          "prompt": "consent login"
        }
      }
	`

	oc, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	require.NoError(t, err)

	require.Equal(t, "google", oc.GetName())
	require.Equal(t, "https://accounts.google.com", oc.GetIssuerURL())
	require.Equal(t, "id-from-google.apps.googleusercontent.com", oc.GetClientID())
	require.Equal(t, "https://localhost:3080/v1/webapi/oidc/callback", oc.GetRedirectURL())
	require.Equal(t, "whatever", oc.GetDisplay())
	require.Equal(t, "consent login", oc.GetPrompt())
}

// TestOIDCUnmarshalInvalid unmarshals and fails validation of the connector
func TestOIDCUnmarshalInvalid(t *testing.T) {
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
          }]
        }
      }
	`

	_, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	require.Error(t, err)
}
