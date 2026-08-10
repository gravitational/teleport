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

package gcp

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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

type fakeIDP struct {
	t         *testing.T
	signer    jose.Signer
	publicKey crypto.PublicKey
	server    *httptest.Server
}

func newFakeIDP(t *testing.T) *fakeIDP {
	// GCP uses RSA, prefer to test with it.
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
	response := map[string]interface{}{
		"claims_supported": []string{
			"sub",
			"aud",
			"exp",
			"iat",
			"iss",
			"azp",
			"email",
			"google",
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
	sub string,
	claims IDTokenClaims,
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
	// GCP ID tokens have an "azp" claim that is the same as the "sub" claim.
	// This azp is not included in the "aud" claim. It's a little murky whether
	// this is spec-compliant. We should explicitly reproduce this in our tests
	// since zealous oidc validation implementations may reject it.
	claims.AuthorizedParty = sub
	token, err := jwt.Signed(f.signer).
		Claims(stdClaims).
		Claims(claims).
		CompactSerialize()
	require.NoError(t, err)

	return token
}

func TestIDTokenValidator_Validate(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	clock := clockwork.NewFakeClock()

	sampleCE := IDTokenClaims{
		Google: Google{
			ComputeEngine: ComputeEngine{
				ProjectID:    "12345678",
				Zone:         "z",
				InstanceID:   "87654321",
				InstanceName: "test-instance",
			},
		},
	}
	tests := []struct {
		name        string
		assertError require.ErrorAssertionFunc
		want        IDTokenClaims
		token       string
	}{
		{
			name:        "success",
			assertError: require.NoError,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				sampleCE,
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
			want: sampleCE,
		},
		{
			name:        "success but without compute engine claims",
			assertError: require.NoError,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				IDTokenClaims{
					Email: "tiago-1-sa-test2@project-id.iam.gserviceaccount.com",
				},
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
			want: IDTokenClaims{
				Email: "tiago-1-sa-test2@project-id.iam.gserviceaccount.com",
				Google: Google{
					ComputeEngine: ComputeEngine{
						ProjectID: "project-id",
					},
				},
			},
		},
		{
			name: "default service account: @developer.gserviceaccount.com domain",
			assertError: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err, i...)
				require.Contains(tt, err.Error(), "default compute engine service account")
			},
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				IDTokenClaims{
					Email: "tiago-1-sa-test2@developer.gserviceaccount.com",
				},
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
		{
			name: "invalid service account email: gserviceaccount.com domain",
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err, i...)
				require.Contains(tt, err.Error(), "invalid email claim")
			},
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				IDTokenClaims{
					Email: "tiago-1-sa-test2@project-id.gserviceaccount.coma",
				},
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
		{
			name: "invalid service account email: gserviceaccount.coma domain",
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err, i...)
				require.Contains(tt, err.Error(), "invalid email claim")
			},
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				IDTokenClaims{
					Email: "tiago-1-sa-test2@project-id.iam.gserviceaccount.coma",
				},
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
		{
			name: "invalid service account email: google domain",
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err, i...)
				require.Contains(tt, err.Error(), "invalid email claim")
			},
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				IDTokenClaims{
					Email: "tiago-1-sa-test2@project-id.iam.google.com",
				},
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
		{
			name: "empty service account email",
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err, i...)
				require.Contains(tt, err.Error(), "invalid email claim")
			},
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				IDTokenClaims{
					Email: "",
				},
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
		{
			name:        "expired",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				sampleCE,
				clock.Now().Add(-15*time.Minute),
				clock.Now().Add(-5*time.Minute),
			),
		},
		{
			name:        "future",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"teleport.cluster.local",
				"abcd1234",
				sampleCE,
				clock.Now().Add(10*time.Minute),
				clock.Now().Add(20*time.Minute),
			),
		},
		{
			name:        "invalid audience",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				"incorrect.audience",
				"abcd1234",
				sampleCE,
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
		{
			name:        "invalid issuer",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				"http://the.wrong.issuer",
				"teleport.cluster.local",
				"abcd1234",
				sampleCE,
				clock.Now().Add(-5*time.Minute),
				clock.Now().Add(5*time.Minute),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			v := NewIDTokenValidator(IDTokenValidatorConfig{
				issuerHost: idp.server.Listener.Addr().String(),
				insecure:   true,
			})
			claims, err := v.Validate(ctx, tc.token)
			tc.assertError(t, err)
			if err != nil {
				return
			}
			require.NotNil(t, claims)
			require.Empty(t,
				cmp.Diff(*claims, tc.want, cmpopts.IgnoreTypes(oidc.TokenClaims{})),
			)
		})
	}
}
