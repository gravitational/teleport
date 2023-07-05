/*
Copyright 2023 Gravitational, Inc.
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
	key := jwksKeys.Keys[0]
	require.Equal(t, key.Use, "sig")
	require.Equal(t, key.KeyType, "RSA")
	require.Equal(t, key.Alg, "RS256")
	require.NotNil(t, key.KeyID) // AWS requires this to be present (even if empty string).
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
