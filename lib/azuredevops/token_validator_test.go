/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package azuredevops

import (
	"context"
	"crypto"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

type fakeIDP struct {
	t         *testing.T
	signer    jose.Signer
	publicKey crypto.PublicKey
	server    *httptest.Server
	kid       string
}

func newFakeIDP(t *testing.T) *fakeIDP {
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	kid := "xyzzy"

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		signer:    signer,
		publicKey: privateKey.Public(),
		t:         t,
		kid:       kid,
	}

	providerMux := http.NewServeMux()
	providerMux.HandleFunc(
		"/{orgid}/.well-known/openid-configuration",
		f.handleOpenIDConfig,
	)
	providerMux.HandleFunc(
		"/.well-known/jwks",
		f.handleJWKSEndpoint,
	)

	srv := httptest.NewServer(providerMux)
	t.Cleanup(srv.Close)
	f.server = srv
	return f
}

func (f *fakeIDP) issuer(orgID string) string {
	return f.server.URL + "/" + orgID
}

func (f *fakeIDP) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"claims_supported": []string{
			"sub",
			"aud",
			"exp",
			"iat",
			"iss",
			"nbf",
		},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                f.issuer(r.PathValue("orgid")),
		"jwks_uri":                              f.server.URL + "/.well-known/jwks",
		"response_types_supported":              []string{"id_token"},
		"scopes_supported":                      []string{"openid"},
		"subject_types_supported":               []string{"public", "pairwise"},
	}
	responseBytes, err := json.Marshal(response)
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)
}

func (f *fakeIDP) handleJWKSEndpoint(w http.ResponseWriter, r *http.Request) {
	responseBytes, err := f.jwks()
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)
}

func (f *fakeIDP) jwks() ([]byte, error) {
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:   f.publicKey,
				KeyID: f.kid,
			},
		},
	}
	responseBytes, err := json.Marshal(jwks)
	if err != nil {
		return nil, err
	}
	return responseBytes, nil
}

func (f *fakeIDP) issueToken(
	t *testing.T,
	issuer,
	audience,
	sub string,
	orgID string,
	issuedAt time.Time,
	expiry time.Time,
) string {
	stdClaims := jwt.Claims{
		Issuer:    issuer,
		Subject:   sub,
		Audience:  jwt.Audience{audience},
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
		Expiry:    jwt.NewNumericDate(expiry),
	}
	customClaims := map[string]interface{}{
		"org_id": orgID,
	}
	token, err := jwt.Signed(f.signer).
		Claims(stdClaims).
		Claims(customClaims).
		Serialize()
	require.NoError(t, err)

	return token
}

func TestIDTokenValidator_Validate(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	goodOrgId := "0000-1111-2222-3333-4444"
	tests := []struct {
		name        string
		assertError require.ErrorAssertionFunc
		want        *IDTokenClaims
		token       string
	}{
		{
			name:        "success",
			assertError: require.NoError,
			token: idp.issueToken(
				t,
				idp.issuer(goodOrgId),
				audience,
				"p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				goodOrgId,
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
			want: &IDTokenClaims{
				Sub:              "p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				OrganizationID:   goodOrgId,
				OrganizationName: "noahstride0304",
				ProjectName:      "testing-azure-devops-join",
				PipelineName:     "strideynet.azure-devops-testing",
			},
		},
		{
			name:        "expired",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(goodOrgId),
				audience,
				"p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				goodOrgId,
				time.Now().Add(-15*time.Minute),
				time.Now().Add(-5*time.Minute),
			),
		},
		{
			name:        "future",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(goodOrgId),
				audience,
				"p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				goodOrgId,
				time.Now().Add(10*time.Minute),
				time.Now().Add(20*time.Minute),
			),
		},
		{
			name:        "invalid issuer",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer("0000-bad-0000"),
				audience,
				"p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				goodOrgId,
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
		{
			name:        "invalid audience",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(goodOrgId),
				"wrong-audience",
				"p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				goodOrgId,
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
		{
			name: "mismatched org id",
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorContains(t, err, "organization ID in token")
			},
			token: idp.issueToken(
				t,
				idp.issuer(goodOrgId),
				audience,
				"p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing",
				"bad-org-id",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			v := NewIDTokenValidator()
			v.insecureDiscovery = true
			v.overrideDiscoveryHost = idp.server.Listener.Addr().String()

			claims, err := v.Validate(
				ctx,
				goodOrgId,
				tt.token,
			)
			tt.assertError(t, err)
			require.Empty(t,
				cmp.Diff(claims, tt.want, cmpopts.IgnoreTypes(oidc.TokenClaims{})),
			)
		})
	}
}
