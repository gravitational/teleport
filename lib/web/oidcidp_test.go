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

package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib"
)

// TestOIDCIdPPublicEndpoints ensures the public endpoints for the AWS OIDC integration are available.
// It also validates that the JWKS_URI points to a correct path.
func TestOIDCIdPPublicEndpoints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Request OpenID Configuration public endpoint.
	publicClt := proxy.newClient(t)
	resp, err := publicClt.Get(ctx, proxy.webURL.String()+"/.well-known/openid-configuration", nil)
	require.NoError(t, err)

	// Deliberately redefining the structs in this test to assert that the JSON
	// representation doesn't unintentionally change.
	type oidcConfiguration struct {
		Issuer                           string   `json:"issuer"`
		JWKSURI                          string   `json:"jwks_uri"`
		Claims                           []string `json:"claims"`
		IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
		ResponseTypesSupported           []string `json:"response_types_supported"`
		ScopesSupported                  []string `json:"scopes_supported"`
		SubjectTypesSupported            []string `json:"subject_types_supported"`
	}

	var gotConfiguration oidcConfiguration
	require.NoError(t, json.Unmarshal(resp.Bytes(), &gotConfiguration))

	expectedConfiguration := oidcConfiguration{
		Issuer:  proxy.webURL.String(),
		JWKSURI: proxy.webURL.String() + "/.well-known/jwks-oidc",
		// OIDC IdPs MUST support RSA256 here.
		IdTokenSigningAlgValuesSupported: []string{"RS256"},
		Claims:                           []string{"iss", "sub", "obo", "aud", "jti", "iat", "exp", "nbf"},
		ResponseTypesSupported:           []string{"id_token"},
		ScopesSupported:                  []string{"openid"},
		SubjectTypesSupported:            []string{"public", "pair-wise"},
	}
	require.Equal(t, expectedConfiguration, gotConfiguration)

	resp, err = publicClt.Get(ctx, gotConfiguration.JWKSURI, nil)
	require.NoError(t, err)

	type jwksKey struct {
		Use     string  `json:"use"`
		KeyID   *string `json:"kid"`
		KeyType string  `json:"kty"`
		Alg     string  `json:"alg"`
	}
	type jwksKeys struct {
		Keys []jwksKey `json:"keys"`
	}

	var gotKeys jwksKeys
	err = json.Unmarshal(resp.Bytes(), &gotKeys)
	require.NoError(t, err)

	// Expect the same key twice, once with a synthesized Key ID, and once with an empty Key ID for compatibility.
	require.Len(t, gotKeys.Keys, 2)
	require.NotEmpty(t, *gotKeys.Keys[0].KeyID)
	require.Equal(t, "", *gotKeys.Keys[1].KeyID)
	expectedKeys := jwksKeys{
		Keys: []jwksKey{
			{
				Use:     "sig",
				KeyType: "RSA",
				Alg:     "RS256",
				KeyID:   gotKeys.Keys[0].KeyID,
			},
			{
				Use:     "sig",
				KeyType: "RSA",
				Alg:     "RS256",
				KeyID:   new(string),
			},
		},
	}
	require.Equal(t, expectedKeys, gotKeys)
}

func TestThumbprint(t *testing.T) {
	ctx := context.Background()

	// Proxy starts with self-signed certificates.
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Request OpenID Configuration public endpoint.
	publicClt := proxy.newClient(t)
	resp, err := publicClt.Get(ctx, proxy.webURL.String()+"/webapi/thumbprint", nil)
	require.NoError(t, err)

	thumbprint := strings.Trim(string(resp.Bytes()), "\"")

	// The Proxy is started using httptest.NewTLSServer, which uses a hard-coded cert
	// located at go/src/net/http/internal/testcert/testcert.go
	// The following value is the sha1 fingerprint of that certificate.
	expectedThumbprint := "15dbd260c7465ecca6de2c0b2181187f66ee0d1a"

	require.Equal(t, expectedThumbprint, thumbprint)
}
