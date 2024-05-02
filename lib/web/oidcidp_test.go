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

	jwksURI := struct {
		JWKSURI string `json:"jwks_uri"`
		Issuer  string `json:"issuer"`
	}{}

	err = json.Unmarshal(resp.Bytes(), &jwksURI)
	require.NoError(t, err)

	// Proxy Public addr must match with Issuer
	require.Equal(t, proxy.webURL.String(), jwksURI.Issuer)

	// Follow the `jwks_uri` endpoint and fetch the public keys
	require.NotEmpty(t, jwksURI.JWKSURI)
	resp, err = publicClt.Get(ctx, jwksURI.JWKSURI, nil)
	require.NoError(t, err)

	jwksKeys := struct {
		Keys []struct {
			Use     string  `json:"use"`
			KeyID   *string `json:"kid"`
			KeyType string  `json:"kty"`
			Alg     string  `json:"alg"`
		} `json:"keys"`
	}{}

	err = json.Unmarshal(resp.Bytes(), &jwksKeys)
	require.NoError(t, err)

	require.NotEmpty(t, jwksKeys.Keys)
	require.Len(t, jwksKeys.Keys, 2)

	// Expect the same key twice, once with a synthesized Key ID, and once with an empty Key ID for compatibility.
	key1 := jwksKeys.Keys[0]
	key2 := jwksKeys.Keys[1]
	require.Equal(t, "sig", key1.Use)
	require.Equal(t, "RSA", key1.KeyType)
	require.Equal(t, "RS256", key1.Alg)
	require.Equal(t, key1.Use, key2.Use)
	require.Equal(t, key1.KeyType, key2.KeyType)
	require.Equal(t, key1.Alg, key2.Alg)
	require.NotEmpty(t, *key1.KeyID)
	require.Equal(t, "", *key2.KeyID)
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
