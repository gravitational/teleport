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

package githubactions

import (
	"context"
	"crypto"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

type fakeIDP struct {
	t             *testing.T
	signer        jose.Signer
	publicKey     crypto.PublicKey
	server        *httptest.Server
	entepriseSlug string
	ghesMode      bool
}

func newFakeIDP(t *testing.T, ghesMode bool, enterpriseSlug string) *fakeIDP {
	// Github uses RSA2048, prefer to test with it.
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		signer:        signer,
		ghesMode:      ghesMode,
		publicKey:     privateKey.Public(),
		t:             t,
		entepriseSlug: enterpriseSlug,
	}

	providerMux := http.NewServeMux()
	providerMux.HandleFunc(
		f.pathPostfix()+"/.well-known/openid-configuration",
		f.handleOpenIDConfig,
	)
	providerMux.HandleFunc(
		f.pathPostfix()+"/.well-known/jwks",
		f.handleJWKSEndpoint,
	)

	srv := httptest.NewServer(providerMux)
	t.Cleanup(srv.Close)
	f.server = srv
	return f
}

func (f *fakeIDP) pathPostfix() string {
	if f.ghesMode {
		// GHES instances serve the token related content on a prefix of the
		// instance hostname.
		return "/_services/token"
	}
	if f.entepriseSlug != "" {
		return "/" + f.entepriseSlug
	}
	return ""
}

func (f *fakeIDP) issuer() string {
	return f.server.URL + f.pathPostfix()
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
	idp := newFakeIDP(t, false, "")
	ghesIdp := newFakeIDP(t, true, "")
	enterpriseSlugIDP := newFakeIDP(t, false, "slug")

	tests := []struct {
		name           string
		assertError    require.ErrorAssertionFunc
		want           *IDTokenClaims
		token          string
		ghesHost       string
		defaultIDPHost string
		enterpriseSlug string
	}{
		{
			name:           "success",
			assertError:    require.NoError,
			defaultIDPHost: idp.server.Listener.Addr().String(),
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
			// This is intentionally the plain IDP as the GHES Host should
			// override it.
			defaultIDPHost: idp.server.Listener.Addr().String(),
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
			name:           "success with slug",
			assertError:    require.NoError,
			defaultIDPHost: enterpriseSlugIDP.server.Listener.Addr().String(),
			token: enterpriseSlugIDP.issueToken(
				t,
				enterpriseSlugIDP.issuer(),
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
			enterpriseSlug: "slug",
			want: &IDTokenClaims{
				Actor: "octocat",
				Sub:   "repo:octo-org/octo-repo:environment:prod",
			},
		},
		{
			name:           "fails if slugged jwt is used with non-slug idp",
			assertError:    require.Error,
			defaultIDPHost: idp.server.Listener.Addr().String(),
			token: enterpriseSlugIDP.issueToken(
				t,
				enterpriseSlugIDP.issuer(),
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
		{
			name:           "fails if non-slugged jwt is used with idp",
			assertError:    require.Error,
			defaultIDPHost: enterpriseSlugIDP.server.Listener.Addr().String(),
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
			enterpriseSlug: "slug",
		},
		{
			name:           "expired",
			assertError:    require.Error,
			defaultIDPHost: idp.server.Listener.Addr().String(),
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
			name:           "future",
			assertError:    require.Error,
			defaultIDPHost: idp.server.Listener.Addr().String(),
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(10*time.Minute), time.Now().Add(20*time.Minute)),
		},
		{
			name:           "invalid audience",
			assertError:    require.Error,
			defaultIDPHost: idp.server.Listener.Addr().String(),
			token: idp.issueToken(
				t,
				idp.issuer(),
				"incorrect.audience",
				"octocat",
				"repo:octo-org/octo-repo:environment:prod",
				time.Now().Add(-5*time.Minute), time.Now().Add(5*time.Minute)),
		},
		{
			name:           "invalid issuer",
			assertError:    require.Error,
			defaultIDPHost: idp.server.Listener.Addr().String(),
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
				GitHubIssuerHost: tt.defaultIDPHost,
				insecure:         true,
			})

			claims, err := v.Validate(
				ctx, tt.ghesHost, tt.enterpriseSlug, tt.token,
			)
			tt.assertError(t, err)
			require.Empty(t,
				cmp.Diff(claims, tt.want, cmpopts.IgnoreTypes(oidc.TokenClaims{})),
			)
		})
	}
}

func testSigner(t *testing.T) ([]byte, jose.Signer) {
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).
			WithType("JWT").
			WithHeader("kid", "foo"),
	)
	require.NoError(t, err)

	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{
			Key:       key.Public(),
			Use:       "sig",
			Algorithm: string(jose.ES256),
			KeyID:     "foo",
		},
	}}
	jwksData, err := json.Marshal(jwks)
	require.NoError(t, err)
	return jwksData, signer
}

type claims struct {
	IDTokenClaims
	Subject string `json:"sub"`
}

func TestValidateTokenWithJWKS(t *testing.T) {
	jwks, signer := testSigner(t)
	_, wrongSigner := testSigner(t)

	now := time.Now()
	clusterName := "teleport.cluster.local"

	tests := []struct {
		name   string
		signer jose.Signer
		claims claims

		wantResult *IDTokenClaims
		wantErr    string
	}{
		{
			name:   "valid token",
			signer: signer,
			claims: claims{
				IDTokenClaims: IDTokenClaims{
					Repository: "123",
					TokenClaims: oidc.TokenClaims{
						Audience:   oidc.Audience{clusterName},
						IssuedAt:   oidc.FromTime(now.Add(-1 * time.Minute)),
						NotBefore:  oidc.FromTime(now.Add(-1 * time.Minute)),
						Expiration: oidc.FromTime(now.Add(10 * time.Minute)),
					},
				},
				Subject: "foo",
			},
			wantResult: &IDTokenClaims{
				Sub:        "foo",
				Repository: "123",
			},
		},
		{
			name:   "signed by wrong signer",
			signer: wrongSigner,
			claims: claims{
				IDTokenClaims: IDTokenClaims{
					Repository: "123",
					TokenClaims: oidc.TokenClaims{
						Audience:   oidc.Audience{clusterName},
						IssuedAt:   oidc.FromTime(now.Add(-1 * time.Minute)),
						NotBefore:  oidc.FromTime(now.Add(-1 * time.Minute)),
						Expiration: oidc.FromTime(now.Add(10 * time.Minute)),
					},
				},
				Subject: "foo",
			},
			wantResult: &IDTokenClaims{
				Sub:        "foo",
				Repository: "123",
			},
			wantErr: "validating jwt signature",
		},
		{
			name:   "expired",
			signer: signer,
			claims: claims{
				IDTokenClaims: IDTokenClaims{
					Repository: "123",
					TokenClaims: oidc.TokenClaims{
						Audience:   oidc.Audience{clusterName},
						IssuedAt:   oidc.FromTime(now.Add(-2 * time.Minute)),
						NotBefore:  oidc.FromTime(now.Add(-2 * time.Minute)),
						Expiration: oidc.FromTime(now.Add(-1 * time.Minute)),
					},
				},
				Subject: "foo",
			},
			wantErr: "token is expired",
		},
		{
			name:   "not yet valid",
			signer: signer,
			claims: claims{
				IDTokenClaims: IDTokenClaims{
					Repository: "123",
					TokenClaims: oidc.TokenClaims{
						Audience:   oidc.Audience{clusterName},
						IssuedAt:   oidc.FromTime(now.Add(2 * time.Minute)),
						NotBefore:  oidc.FromTime(now.Add(2 * time.Minute)),
						Expiration: oidc.FromTime(now.Add(4 * time.Minute)),
					},
				},
				Subject: "foo",
			},
			wantErr: "token not valid yet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := jwt.Signed(tt.signer).
				Claims(tt.claims).
				CompactSerialize()
			require.NoError(t, err)

			result, err := ValidateTokenWithJWKS(now, jwks, token)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Empty(t,
				cmp.Diff(result, tt.wantResult, cmpopts.IgnoreTypes(oidc.TokenClaims{})),
			)
		})
	}
}
