/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"crypto"
	"crypto/x509"
	"encoding/json"
	"testing"

	"github.com/gravitational/roundtrip"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestGetSPIFFEBundle(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	authServer := env.server.Auth()
	cn, err := authServer.GetClusterName()
	require.NoError(t, err)
	ca, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: cn.GetClusterName(),
	}, false)
	require.NoError(t, err)

	var wantCACerts []*x509.Certificate
	for _, certPem := range services.GetTLSCerts(ca) {
		cert, err := tlsca.ParseCertificatePEM(certPem)
		require.NoError(t, err)
		wantCACerts = append(wantCACerts, cert)
	}

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	res, err := clt.Get(ctx, clt.Endpoint("webapi", "spiffe", "bundle.json"), nil)
	require.NoError(t, err)

	td, err := spiffeid.TrustDomainFromString(cn.GetClusterName())
	require.NoError(t, err)
	gotBundle, err := spiffebundle.Read(td, res.Reader())
	require.NoError(t, err)

	require.Len(t, gotBundle.X509Authorities(), len(wantCACerts))
	for _, caCert := range wantCACerts {
		require.True(t, gotBundle.HasX509Authority(caCert), "certificate not found in bundle")
	}

	require.Len(t, gotBundle.JWTAuthorities(), len(ca.GetTrustedJWTKeyPairs()))
	for _, jwtKeyPair := range ca.GetTrustedJWTKeyPairs() {
		wantKey, err := keys.ParsePublicKey(jwtKeyPair.PublicKey)
		require.NoError(t, err)
		wantKeyID, err := jwt.KeyID(wantKey)
		require.NoError(t, err)
		gotPubKey, ok := gotBundle.JWTBundle().FindJWTAuthority(wantKeyID)
		require.True(t, ok, "wanted public key not found in bundle")
		require.True(t, gotPubKey.(interface{ Equal(x crypto.PublicKey) bool }).Equal(wantKey), "public keys do not match")
	}
}

// TestSPIFFEJWTPublicEndpoints ensures the public endpoints for the SPIFFE JWT
// OIDC support function correctly.
func TestSPIFFEJWTPublicEndpoints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	// Request OpenID Configuration public endpoint.
	publicClt := proxy.newClient(t)
	resp, err := publicClt.Get(ctx, proxy.webURL.String()+"/workload-identity/.well-known/openid-configuration", nil)
	require.NoError(t, err)

	// Deliberately redefining the structs in this test to assert that the JSON
	// representation doesn't unintentionally change.
	type oidcConfiguration struct {
		Issuer                           string   `json:"issuer"`
		JWKSURI                          string   `json:"jwks_uri"`
		Claims                           []string `json:"claims"`
		IdTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
		ResponseTypesSupported           []string `json:"response_types_supported"`
	}

	var gotConfiguration oidcConfiguration
	require.NoError(t, json.Unmarshal(resp.Bytes(), &gotConfiguration))

	expectedConfiguration := oidcConfiguration{
		Issuer:  proxy.webURL.String() + "/workload-identity",
		JWKSURI: proxy.webURL.String() + "/workload-identity/jwt-jwks.json",
		// OIDC IdPs MUST support RSA256 here.
		IdTokenSigningAlgValuesSupported: []string{"RS256"},
		Claims: []string{
			"iss",
			"sub",
			"jti",
			"aud",
			"exp",
			"iat",
		},
		ResponseTypesSupported: []string{"id_token"},
	}
	require.Equal(t, expectedConfiguration, gotConfiguration)

	resp, err = publicClt.Get(ctx, gotConfiguration.JWKSURI, nil)
	require.NoError(t, err)

	var gotKeys JWKSResponse
	err = json.Unmarshal(resp.Bytes(), &gotKeys)
	require.NoError(t, err)

	require.Len(t, gotKeys.Keys, 1)
	require.NotEmpty(t, gotKeys.Keys[0].KeyID)
}
