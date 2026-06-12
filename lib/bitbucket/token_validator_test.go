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

package bitbucket

import (
	"cmp"
	"context"
	"crypto"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

type fakeIDP struct {
	t         *testing.T
	signer    jose.Signer
	publicKey crypto.PublicKey
	server    *httptest.Server
	audience  string
}

func newFakeIDP(t *testing.T, audience string) *fakeIDP {
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		signer:    signer,
		publicKey: privateKey.Public(),
		t:         t,
		audience:  audience,
	}

	providerMux := http.NewServeMux()
	providerMux.HandleFunc(
		"/.well-known/openid-configuration",
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

func (f *fakeIDP) issuer() string {
	return f.server.URL
}

func (f *fakeIDP) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	// mimic https://api.bitbucket.org/2.0/workspaces/$workspace/pipelines-config/identity/oidc/.well-known/openid-configuration
	response := map[string]interface{}{
		"claims_supported": []string{
			"sub",
			"iss",
			"aud",
			"exp",
			"iat",
			"branchName",
			"compliant",
			"deploymentEnvironmentUuid",
			"pipelineUuid",
			"repositoryUuid",
			"stepUuid",
			"workspaceUuid",
		},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                f.issuer(),
		"jwks_uri":                              f.issuer() + "/.well-known/jwks",
		"response_types_supported":              []string{"id_token"},
		"scopes_supported":                      []string{"openid"},
		"subject_types_supported":               []string{"public"},
	}
	responseBytes, err := json.Marshal(response)
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)
}

func (f *fakeIDP) handleJWKSEndpoint(w http.ResponseWriter, r *http.Request) {
	// mimic https://app.terraform.io/.well-known/jwks but with our own keys
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key: f.publicKey,
			},
		},
	}
	responseBytes, err := json.Marshal(jwks)
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)
}

func (f *fakeIDP) issueToken(
	t *testing.T,
	issuer,
	audience,
	workspaceUUID,
	repositoryUUID,
	stepUUID,
	sub string,
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
		"workspaceUuid":  workspaceUUID,
		"repositoryUuid": repositoryUUID,
		"stepUuid":       stepUUID,
	}
	token, err := jwt.Signed(f.signer).
		Claims(stdClaims).
		Claims(customClaims).
		CompactSerialize()
	require.NoError(t, err)

	return token
}

func TestIDTokenValidator_Validate(t *testing.T) {
	t.Parallel()

	// workspace-id is the bitbucket workspace UUID, without braces
	const audience = "ari:cloud:bitbucket::workspace/workspace-uuid"
	idp := newFakeIDP(t, audience)

	tests := []struct {
		name             string
		assertError      require.ErrorAssertionFunc
		want             *IDTokenClaims
		token            string
		audienceOverride string
	}{
		{
			name:        "success",
			assertError: require.NoError,
			token: idp.issueToken(
				t,
				idp.issuer(),
				idp.audience,
				"{workspace}",
				"{repository}",
				"{step}",
				"{repository}:{step}",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
			want: &IDTokenClaims{
				WorkspaceUUID:  "{workspace}",
				RepositoryUUID: "{repository}",
				StepUUID:       "{step}",
				Sub:            "{repository}:{step}",
			},
		},
		{
			name:        "expired",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				idp.audience,
				"{workspace}",
				"{repository}",
				"{step}",
				"{repository}:{step}",
				time.Now().Add(-15*time.Minute),
				time.Now().Add(-5*time.Minute),
			),
		},
		{
			name:        "future",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				idp.audience,
				"{workspace}",
				"{repository}",
				"{step}",
				"{repository}:{step}",
				time.Now().Add(10*time.Minute),
				time.Now().Add(20*time.Minute),
			),
		},
		{
			name:        "invalid audience",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"some.wrong.audience.example.com",
				"{workspace}",
				"{repository}",
				"{step}",
				"{repository}:{step}",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
		{
			name:        "invalid issuer",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				"https://the.wrong.issuer",
				idp.audience,
				"{workspace}",
				"{repository}",
				"{step}",
				"{repository}:{step}",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			issuerAddr := "http://" + idp.server.Listener.Addr().String()

			v := NewIDTokenValidator(clockwork.NewRealClock())

			claims, err := v.Validate(
				ctx,
				issuerAddr,
				cmp.Or(tt.audienceOverride, audience),
				tt.token,
			)
			tt.assertError(t, err)
			require.Equal(t, tt.want, claims)
		})
	}
}
