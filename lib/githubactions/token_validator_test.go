/*
Copyright 2022 Gravitational, Inc.

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

package githubactions

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

func fakeGithubIDP(t *testing.T) (*httptest.Server, jose.Signer) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	providerMux := http.NewServeMux()
	srv := httptest.NewServer(providerMux)
	t.Cleanup(srv.Close)
	providerMux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// mimic https://token.actions.githubusercontent.com/.well-known/openid-configuration
		response := map[string]interface{}{
			"claims_supported": []string{
				"sub",
				"aud",
				"exp",
				"iat",
				"iss",
				"jti",
				"nbf",
				"ref",
				"repository",
				"repository_id",
				"repository_owner",
				"repository_owner_id",
				"run_id",
				"run_number",
				"run_attempt",
				"actor",
				"actor_id",
				"workflow",
				"head_ref",
				"base_ref",
				"event_name",
				"ref_type",
				"environment",
				"job_workflow_ref",
				"repository_visibility",
			},
			"id_token_signing_alg_values_supported": []string{"RS256"},
			"issuer":                                srv.URL,
			"jwks_uri":                              fmt.Sprintf("%s/.well-known/jwks", srv.URL),
			"response_types_supported":              []string{"id_token"},
			"scopes_supported":                      []string{"openid"},
			"subject_types_supported":               []string{"public", "pairwise"},
		}
		responseBytes, err := json.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(responseBytes)
		require.NoError(t, err)
	})
	providerMux.HandleFunc("/.well-known/jwks", func(w http.ResponseWriter, r *http.Request) {
		// mimic https://token.actions.githubusercontent.com/.well-known/jwks
		// but with our own keys
		jwks := jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key: &privateKey.PublicKey,
				},
			},
		}
		responseBytes, err := json.Marshal(jwks)
		require.NoError(t, err)
		_, err = w.Write(responseBytes)
		require.NoError(t, err)

	})

	return srv, signer
}

func makeToken(t *testing.T, jwtSigner jose.Signer, issuer, audience, actor, sub string, issuedAt time.Time, expiry time.Time) string {
	stdClaims := jwt.Claims{
		Issuer:    issuer,
		Subject:   sub,
		Audience:  jwt.Audience{audience},
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		NotBefore: jwt.NewNumericDate(issuedAt),
		Expiry:    jwt.NewNumericDate(expiry),
	}
	customClaims := map[string]interface{}{
		"actor": actor,
	}
	token, err := jwt.Signed(jwtSigner).
		Claims(stdClaims).
		Claims(customClaims).
		CompactSerialize()
	require.NoError(t, err)

	return token
}

func TestIDTokenValidator_Validate(t *testing.T) {
	t.Parallel()
	providerServer, jwtSigner := fakeGithubIDP(t)

	tests := []struct {
		name        string
		assertError require.ErrorAssertionFunc
		want        *IDTokenClaims
		token       string
	}{
		{
			name:        "success",
			assertError: require.NoError,
			token: makeToken(
				t,
				jwtSigner,
				providerServer.URL,
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
			want: &IDTokenClaims{
				Actor: "octocat",
				Sub:   "repo:octo-org/octo-repo:environment:prod",
			},
		},
		{
			name:        "expired",
			assertError: require.Error,
			token: makeToken(
				t,
				jwtSigner,
				providerServer.URL,
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-15*time.Minute),
				time.Now().Add(-5*time.Minute),
			),
		},
		{
			name:        "future",
			assertError: require.Error,
			token: makeToken(
				t,
				jwtSigner,
				providerServer.URL,
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(10*time.Minute), time.Now().Add(20*time.Minute)),
		},
		{
			name:        "invalid audience",
			assertError: require.Error,
			token: makeToken(
				t,
				jwtSigner,
				providerServer.URL,
				"incorrect.audience",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute), time.Now().Add(5*time.Minute)),
		},
		{
			name:        "invalid issuer",
			assertError: require.Error,
			token: makeToken(
				t,
				jwtSigner,
				"https://not.the.issuer",
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute), time.Now().Add(5*time.Minute)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			v := NewIDTokenValidator(IDTokenValidatorConfig{
				Clock:     clockwork.NewRealClock(),
				IssuerURL: providerServer.URL,
			})

			claims, err := v.Validate(ctx, tt.token)
			tt.assertError(t, err)
			require.Equal(t, tt.want, claims)
		})
	}
}
