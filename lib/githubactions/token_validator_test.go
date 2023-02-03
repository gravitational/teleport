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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

type fakeIDP struct {
	t          *testing.T
	signer     jose.Signer
	privateKey *rsa.PrivateKey
	server     *httptest.Server
	ghesMode   bool
}

func newFakeIDP(t *testing.T, ghesMode bool) *fakeIDP {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		signer:     signer,
		ghesMode:   ghesMode,
		privateKey: privateKey,
		t:          t,
	}

	providerMux := http.NewServeMux()
	providerMux.HandleFunc(
		f.pathPrefix()+"/.well-known/openid-configuration",
		f.handleOpenIDConfig,
	)
	providerMux.HandleFunc(
		f.pathPrefix()+"/.well-known/jwks",
		f.handleJWKSEndpoint,
	)

	srv := httptest.NewServer(providerMux)
	t.Cleanup(srv.Close)
	f.server = srv
	return f
}

func (f *fakeIDP) pathPrefix() string {
	if f.ghesMode {
		// GHES instances serve the token related content on a prefix of the
		// instance hostname.
		return "/_services/token"
	}
	return ""
}

func (f *fakeIDP) issuer() string {
	return f.server.URL + f.pathPrefix()
}

func (f *fakeIDP) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
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
		"issuer":                                f.issuer(),
		"jwks_uri":                              f.issuer() + "/.well-known/jwks",
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
	// mimic https://token.actions.githubusercontent.com/.well-known/jwks
	// but with our own keys
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key: &f.privateKey.PublicKey,
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
	actor,
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
		"actor": actor,
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
	idp := newFakeIDP(t, false)
	ghesIdp := newFakeIDP(t, true)

	tests := []struct {
		name        string
		assertError require.ErrorAssertionFunc
		want        *IDTokenClaims
		token       string
		ghesHost    string
	}{
		{
			name:        "success",
			assertError: require.NoError,
			token: idp.issueToken(
				t,
				idp.issuer(),
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
			name:        "success with ghes",
			assertError: require.NoError,
			token: ghesIdp.issueToken(
				t,
				ghesIdp.issuer(),
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
			ghesHost: ghesIdp.server.Listener.Addr().String(),
		},
		{
			name:        "expired",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
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
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(10*time.Minute), time.Now().Add(20*time.Minute)),
		},
		{
			name:        "invalid audience",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"incorrect.audience",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute), time.Now().Add(5*time.Minute)),
		},
		{
			name:        "invalid issuer",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				"https://the.wrong.issuer",
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
				Clock:            clockwork.NewRealClock(),
				GitHubIssuerHost: idp.server.Listener.Addr().String(),
				insecure:         true,
			})

			claims, err := v.Validate(ctx, tt.ghesHost, tt.token)
			tt.assertError(t, err)
			require.Equal(t, tt.want, claims)
		})
	}
}
