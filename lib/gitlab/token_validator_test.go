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

package gitlab

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/cryptopatch"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type fakeIDP struct {
	t          *testing.T
	signer     jose.Signer
	privateKey *rsa.PrivateKey
	server     *httptest.Server
}

func newFakeIDP(t *testing.T) *fakeIDP {
	privateKey, err := cryptopatch.GenerateRSAKey(rand.Reader, 2048)
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
		"/oauth/discovery/keys",
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
	// mimic https://gitlab.com/.well-known/openid-configuration
	response := map[string]interface{}{
		"claims_supported": []string{
			"sub",
			"aud",
			"exp",
			"iat",
			"iss",
			"jti",
			"nbf",
		},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                f.issuer(),
		"jwks_uri":                              f.issuer() + "/oauth/discovery/keys",
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
	// mimic https://gitlab.com/oauth/discovery/keys
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
	userLogin,
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
		"user_login": userLogin,
	}
	token, err := jwt.Signed(f.signer).
		Claims(stdClaims).
		Claims(customClaims).
		CompactSerialize()
	require.NoError(t, err)

	return token
}

type mockClusterNameGetter string

func (m mockClusterNameGetter) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{
		ClusterID:   uuid.NewString(),
		ClusterName: string(m),
	})
}

func TestIDTokenValidator_Validate(t *testing.T) {
	t.Parallel()
	idp := newFakeIDP(t)
	teleportClusterName := "teleport.example.com"

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
				idp.issuer(),
				teleportClusterName,
				"unpetitchien",
				"project_path:mygroup/my-project:ref_type:branch:ref:main",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
			want: &IDTokenClaims{
				UserLogin: "unpetitchien",
				Sub:       "project_path:mygroup/my-project:ref_type:branch:ref:main",
			},
		},
		{
			name:        "expired",
			assertError: require.Error,
			token: idp.issueToken(
				t,
				idp.issuer(),
				teleportClusterName,
				"octocat",
				"project_path:mygroup/my-project:ref_type:branch:ref:main",
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
				teleportClusterName,
				"octocat",
				"project_path:mygroup/my-project:ref_type:branch:ref:main",
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
				"wrong-teleport.example.com",
				"octocat",
				"project_path:mygroup/my-project:ref_type:branch:ref:main",
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
				teleportClusterName,
				"octocat",
				"project_path:mygroup/my-project:ref_type:branch:ref:main",
				time.Now().Add(-5*time.Minute),
				time.Now().Add(5*time.Minute),
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			v, err := NewIDTokenValidator(IDTokenValidatorConfig{
				Clock:             clockwork.NewRealClock(),
				insecure:          true,
				ClusterNameGetter: mockClusterNameGetter(teleportClusterName),
			})
			require.NoError(t, err)

			claims, err := v.Validate(
				ctx,
				idp.server.Listener.Addr().String(),
				tt.token,
			)
			tt.assertError(t, err)
			require.Equal(t, tt.want, claims)
		})
	}
}
