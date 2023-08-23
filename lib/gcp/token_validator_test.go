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

package gcp

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
}

func newFakeIDP(t *testing.T) *fakeIDP {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		signer:     signer,
		privateKey: privateKey,
		t:          t,
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
				Clock:      clock,
				issuerHost: idp.server.Listener.Addr().String(),
				insecure:   true,
			})
			claims, err := v.Validate(ctx, tc.token)
			tc.assertError(t, err)
			if err != nil {
				return
			}
			require.NotNil(t, claims)
			require.EqualValues(t, tc.want, *claims)
		})
	}
}
