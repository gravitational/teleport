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
package services

import (
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/stretchr/testify/require"
)

// TestUnmarshal tests unmarshal of OIDC connector
func TestUnmarshal(t *testing.T) {
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

	require.Equal(t, oc.GetName(), "google")
	require.Equal(t, oc.GetIssuerURL(), "https://accounts.google.com")
	require.Equal(t, oc.GetClientID(), "id-from-google.apps.googleusercontent.com")
	require.Equal(t, oc.GetRedirectURL(), "https://localhost:3080/v1/webapi/oidc/callback")
	require.Equal(t, oc.GetDisplay(), "whatever")
	require.Equal(t, oc.GetPrompt(), teleport.OIDCPromptSelectAccount)
}

// TestUnmarshalEmptyPrompt makes sure that empty prompt value
// that is set does not default to select_account
func TestUnmarshalEmptyPrompt(t *testing.T) {
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

	require.Equal(t, oc.GetName(), "google")
	require.Equal(t, oc.GetIssuerURL(), "https://accounts.google.com")
	require.Equal(t, oc.GetClientID(), "id-from-google.apps.googleusercontent.com")
	require.Equal(t, oc.GetRedirectURL(), "https://localhost:3080/v1/webapi/oidc/callback")
	require.Equal(t, oc.GetDisplay(), "whatever")
	require.Equal(t, oc.GetPrompt(), "")
}

// TestUnmarshalPromptValue makes sure that prompt value is set properly
func TestUnmarshalPromptValue(t *testing.T) {
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

	require.Equal(t, oc.GetName(), "google")
	require.Equal(t, oc.GetIssuerURL(), "https://accounts.google.com")
	require.Equal(t, oc.GetClientID(), "id-from-google.apps.googleusercontent.com")
	require.Equal(t, oc.GetRedirectURL(), "https://localhost:3080/v1/webapi/oidc/callback")
	require.Equal(t, oc.GetDisplay(), "whatever")
	require.Equal(t, oc.GetPrompt(), "consent login")
}

// TestUnmarshalInvalid unmarshals and fails validation of the connector
func TestUnmarshalInvalid(t *testing.T) {
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
