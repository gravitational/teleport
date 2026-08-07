/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package genericoidc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/utils/cert"
)

// fakeIDP provides a minimal fake OIDC provider for use in tests
type fakeIDP struct {
	t          *testing.T
	clock      *clockwork.FakeClock
	privateKey crypto.Signer
	signer     jose.Signer
	publicKey  crypto.PublicKey
	server     *httptest.Server
	mux        *http.ServeMux
	keyID      string

	audience string
}

func newFakeIDP(t *testing.T, clock *clockwork.FakeClock, audience string) *fakeIDP {
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA2048)
	require.NoError(t, err)

	const keyID = "test-key-id"
	signer, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key: jose.JSONWebKey{
				Key:       privateKey,
				KeyID:     keyID,
				Algorithm: string(jose.RS256),
			},
		},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)

	f := &fakeIDP{
		clock:      clock,
		signer:     signer,
		privateKey: privateKey,
		publicKey:  privateKey.Public(),
		t:          t,
		audience:   audience,
		keyID:      keyID,
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
	providerMux.HandleFunc(
		"/.well-known/jwks-alt",
		f.handleJWKSEndpoint,
	)
	f.mux = providerMux

	srv := httptest.NewServer(providerMux)
	t.Cleanup(srv.Close)
	f.server = srv
	return f
}

func (f *fakeIDP) issuer() string {
	return f.server.URL
}

func (f *fakeIDP) handleOpenIDConfig(w http.ResponseWriter, r *http.Request) {
	// Lie about our issuer since the request might be coming via the HTTPS
	// endpoint.
	var issuer string
	if r.TLS != nil {
		issuer = "https://" + r.Host
	} else {
		issuer = "http://" + r.Host
	}

	jwksURI := issuer + "/.well-known/jwks"

	response := map[string]any{
		"claims_supported": []string{
			"sub",
			"iss",
		},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                issuer,
		"jwks_uri":                              jwksURI,
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
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:   f.publicKey,
				KeyID: f.keyID,
			},
		},
	}
	responseBytes, err := json.Marshal(jwks)
	require.NoError(f.t, err)
	_, err = w.Write(responseBytes)
	require.NoError(f.t, err)
}

func (f *fakeIDP) issueTokenWithIssuerAndSigner(
	t *testing.T,
	signer jose.Signer,
	issuer, audience, sub string,
	ttl time.Duration,
	claimsMutators ...func(*IDTokenClaims),
) string {
	claims := IDTokenClaims{
		TokenClaims: oidc.TokenClaims{
			Issuer:          issuer,
			Subject:         sub,
			Audience:        oidc.Audience{audience},
			IssuedAt:        oidc.FromTime(f.clock.Now()),
			NotBefore:       oidc.FromTime(f.clock.Now()),
			Expiration:      oidc.FromTime(f.clock.Now().Add(ttl)),
			AuthorizedParty: "1234567890",
		},

		// Custom MarshalJSON on oidc.IDTokenClaims should merge these into the
		// final doc.
		Claims: map[string]any{
			"email":          "123456789012-compute@developer.gserviceaccount.com",
			"email_verified": true,
			"google": map[string]any{
				"compute_engine": map[string]any{
					"instance_creation_timestamp": 1666452409.0,
					"instance_id":                 "12345678901234567",
					"instance_name":               "hello-world",
					"project_id":                  "example-123456",
					"project_number":              123456123456.0,
					"zone":                        "us-central1-a",
				},
			},
			"custom": map[string]any{
				"float": 123.456,
				"list":  []string{"a", "b", "c"},
			},
		},
	}

	for _, mut := range claimsMutators {
		if mut != nil {
			mut(&claims)
		}
	}

	token, err := jwt.Signed(signer).
		Claims(&claims).
		Serialize()
	require.NoError(t, err)

	return token
}

func (f *fakeIDP) issueTokenWithIssuer(
	t *testing.T,
	issuer, audience, sub string,
	ttl time.Duration,
	claimsMutators ...func(*IDTokenClaims),
) string {
	return f.issueTokenWithIssuerAndSigner(t, f.signer, issuer, audience, sub, ttl, claimsMutators...)
}

func (f *fakeIDP) issueToken(
	t *testing.T,
	audience, sub string,
	ttl time.Duration,
) string {
	return f.issueTokenWithIssuer(t, f.issuer(), audience, sub, ttl)
}

func (f *fakeIDP) staticJWKSConfig(t *testing.T) string {
	// Essentially the same as the handler above, but without HTTP. Not quite
	// worth reusing it due to err handling requirements.
	t.Helper()

	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{{Key: f.publicKey, KeyID: f.keyID}},
	}

	bytes, err := json.Marshal(jwks)
	require.NoError(t, err)

	return string(bytes)
}

type fakeHTTPSIDP struct {
	inner *fakeIDP

	httpsListener net.Listener
	currentCert   atomic.Pointer[tls.Certificate]
}

func newFakeHTTPSIDP(t *testing.T, idp *fakeIDP) *fakeHTTPSIDP {
	// Mildly cursed testing fixture courtesy of Claude; we need a way to swap
	// the cert without creating a new listener.

	outer := &fakeHTTPSIDP{
		inner: idp,
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	outer.httpsListener = l

	srv := &http.Server{
		Handler: idp.mux,
		TLSConfig: &tls.Config{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				if c := outer.currentCert.Load(); c != nil {
					return c, nil
				}
				return nil, errors.New("fakeIDP: no cert set")
			},
		},
	}

	// Empty certFile and keyFile are okay due to GetCertificate.
	go srv.ServeTLS(l, "", "")

	t.Cleanup(func() { _ = srv.Close() })

	return outer
}

func (f *fakeHTTPSIDP) issuer() string {
	return "https://" + f.httpsListener.Addr().String()
}

func (f *fakeHTTPSIDP) issueToken(
	t *testing.T,
	audience, sub string,
	ttl time.Duration,
) string {
	return f.inner.issueTokenWithIssuer(t, f.issuer(), audience, sub, ttl)
}

func (f *fakeHTTPSIDP) rotateCA(t *testing.T) (caPEM string) {
	t.Helper()

	creds, err := cert.GenerateSelfSignedCert(nil, []string{"127.0.0.1"})
	require.NoError(t, err)

	tlsCert, err := tls.X509KeyPair(creds.Cert, creds.PrivateKey)
	require.NoError(t, err)

	f.currentCert.Store(&tlsCert)
	return string(creds.Cert)
}

func TestGenericOIDC(t *testing.T) {
	t.Parallel()

	const audience = "example.teleport.sh"

	expires := time.Now().Add(time.Hour)
	baseToken := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name:    "test",
			Expires: &expires,
		},
		Spec: types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodGenericOIDC,
			BotName:    "example",
			Roles:      []types.SystemRole{types.RoleBot},
			GenericOIDC: &types.ProvisionTokenSpecV2GenericOIDC{
				Audience: audience,
			},
		},
	}

	tests := []struct {
		name         string
		mutateToken  func(token *types.ProvisionTokenV2)
		expectError  require.ErrorAssertionFunc
		expectClaims func(t *testing.T, claims *IDTokenClaims)
	}{
		{
			name: "simple success",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`)
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
							notEqCondition("email_verified", "false"),
							inCondition("azp", "1234567800", "1234567000", "1234567890"),
							notInCondition("google.compute_engine.zone", "us-central1-b", "us-central1-c", "us-central1-d"),
						),
					},
				}
			},
			expectError: require.NoError,
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.NotNil(t, claims)
				require.Equal(t, "123456789012-compute@developer.gserviceaccount.com", claims.Claims["email"])
			},
		},
		{
			name: "single mismatched must_match_fields fails",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": {
							"project_id": "foo",
							"project_number": 123456123456
						}
					}
				}`)
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "incorrect value in claim: google.compute_engine.project_id must be \"foo\"")
			},
		},
		{
			name: "single mismatched allow_any fails",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`)
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "foo"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "claims matched no allow_any rules")
			},
		},
		{
			name: "only must_match_fields allows correctly",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`)
			},
			expectError: require.NoError,
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.NotNil(t, claims)
			},
		},
		{
			name: "only must_match_fields denies correctly",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": {
							"project_number": 11111
						}
					}
				}`)
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				// real error is logged
				require.ErrorContains(t, err, "incorrect value in claim: google.compute_engine.project_number must be 11111")
			},
		},
		{
			name: "only allow_any rules allow correctly",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
						),
					},
				}
			},
			expectError: require.NoError,
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.NotNil(t, claims)
			},
		},
		{
			name: "only allow_any rules deny correctly",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "foo"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				// real error is logged
				require.ErrorContains(t, err, "claims matched no allow_any rules")
			},
		},
		{
			name: "error when no rules are configured",
			expectError: func(t require.TestingT, err error, i ...any) {
				// real error is logged
				require.ErrorContains(t, err, "generic OIDC token has no rules configured")
			},
		},
		{
			name: "field rules must always be evaluated",
			mutateToken: func(token *types.ProvisionTokenV2) {
				// Negative rule doesn't count for
				// validateFieldRulesContainsAnyRule() but should still be
				// honored if set.
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": null
					}
				}`)
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "must be null or unset but had a value")
			},
		},
		{
			name: "allow_any expression allows correctly",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Expression: "claims.google.compute_engine.instance_name == \"hello-world\"",
					},
				}
			},
			expectError: require.NoError,
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.NotNil(t, claims)
			},
		},
		{
			name: "allow_any expression denies correctly",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Expression: "claims.google.compute_engine.instance_name == \"asdf\"",
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "claims matched no allow_any rules")
			},
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.Nil(t, claims)
			},
		},
		{
			name: "multiple allow_any expressions allow eventually",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Expression: "claims.google.compute_engine.instance_name == \"asdf\"",
					},
					{
						Expression: "claims.google.compute_engine.instance_name == \"hello-world\"",
					},
				}
			},
			expectError: require.NoError,
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.NotNil(t, claims)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			idp := newFakeIDP(t, clock, audience)

			validator, err := newIDTokenValidatorWithClock(clock)
			require.NoError(t, err)

			token, ok := baseToken.Clone().(*types.ProvisionTokenV2)
			require.True(t, ok)

			token.Spec.GenericOIDC.Issuer = idp.issuer()
			if tt.mutateToken != nil {
				tt.mutateToken(token)
			}

			claims, err := validator.ValidateToken(
				t.Context(),
				token,
				[]byte(idp.issueToken(t, audience, "example", time.Hour)),
			)
			tt.expectError(t, err)

			if tt.expectClaims != nil {
				tt.expectClaims(t, claims)
			} else {
				require.Nil(t, claims)
			}
		})
	}
}

func TestGenericOIDCStaticJWKS(t *testing.T) {
	t.Parallel()

	const audience = "example.teleport.sh"
	const issuer = "https://not-a-real-url"

	expires := time.Now().Add(time.Hour)
	baseToken := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name:    "test",
			Expires: &expires,
		},
		Spec: types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodGenericOIDC,
			BotName:    "example",
			Roles:      []types.SystemRole{types.RoleBot},
			GenericOIDC: &types.ProvisionTokenSpecV2GenericOIDC{
				Audience: audience,

				// hard-code issuer to an invalid value to make sure we aren't
				// hitting the discovery endpoint of an otherwise-working OIDC
				// issuer
				Issuer: issuer,
			},
		},
	}

	tests := []struct {
		name         string
		mutateToken  func(token *types.ProvisionTokenV2)
		expectError  require.ErrorAssertionFunc
		expectClaims func(t *testing.T, claims *IDTokenClaims)
		mutateClaims func(*IDTokenClaims)
	}{
		{
			name: "success with all rule types",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.MustMatchFields = specStruct(t, `{
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`)
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
							notEqCondition("email_verified", "false"),
							inCondition("azp", "1234567800", "1234567000", "1234567890"),
							notInCondition("google.compute_engine.zone", "us-central1-b", "us-central1-c", "us-central1-d"),
						),
					},
				}
			},
			expectError: require.NoError,
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.NotNil(t, claims)
				require.Equal(t, "123456789012-compute@developer.gserviceaccount.com", claims.Claims["email"])
			},
		},
		{
			name: "denies with a failing rule",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "incorrect"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "claims matched no allow_any rules")
			},
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.Nil(t, claims)
			},
		},
		{
			name: "denies with no rules configured",
			expectError: func(t require.TestingT, err error, i ...any) {
				// real error is logged
				require.ErrorContains(t, err, "generic OIDC token has no rules configured")
			},
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.Nil(t, claims)
			},
		},
		{
			name: "denies with invalid jwks",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "incorrect"),
						),
					},
				}
				token.Spec.GenericOIDC.StaticJWKS = "{asdfasdfasdf"
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "parsing provided jwks")
			},
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.Nil(t, claims)
			},
		},
		{
			name: "denies with incorrect signature",
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "incorrect"),
						),
					},
				}

				other := createPubKey(t, cryptosuites.RSA2048)
				b, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
					{
						Key: other,
						// set KeyID to make it as far down the validation path
						// as possible (log should indicate "error in
						// cryptographic primitive")
						KeyID: "test-key-id",
					},
				}})
				require.NoError(t, err)
				token.Spec.GenericOIDC.StaticJWKS = string(b)

			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "validating jwt signature")
			},
			expectClaims: func(t *testing.T, claims *IDTokenClaims) {
				require.Nil(t, claims)
			},
		},
		{
			name: "rejects token without iat",
			mutateClaims: func(c *IDTokenClaims) {
				c.IssuedAt = 0
			},
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "token must have an `iat` claim")
			},
		},
		{
			name: "rejects token without exp",
			mutateClaims: func(c *IDTokenClaims) {
				c.Expiration = 0
			},
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "token must have an `exp` claim")
			},
		},
		{
			name: "rejects token without sub",
			mutateClaims: func(c *IDTokenClaims) {
				c.Subject = ""
			},
			mutateToken: func(token *types.ProvisionTokenV2) {
				token.Spec.GenericOIDC.AllowAny = []*types.ProvisionTokenSpecV2GenericOIDC_Rule{
					{
						Conditions: conditions(
							eqCondition("google.compute_engine.instance_name", "hello-world"),
						),
					},
				}
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "token must have a `sub` claim")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			idp := newFakeIDP(t, clock, audience)

			validator, err := newIDTokenValidatorWithClock(clock)
			require.NoError(t, err)

			token, ok := baseToken.Clone().(*types.ProvisionTokenV2)
			require.True(t, ok)

			token.Spec.GenericOIDC.StaticJWKS = idp.staticJWKSConfig(t)
			if tt.mutateToken != nil {
				tt.mutateToken(token)
			}

			claims, err := validator.ValidateToken(
				t.Context(),
				token,
				[]byte(idp.issueTokenWithIssuer(t, issuer, audience, "example", time.Hour, tt.mutateClaims)),
			)
			tt.expectError(t, err)

			if tt.expectClaims != nil {
				tt.expectClaims(t, claims)
			} else {
				require.Nil(t, claims)
			}
		})
	}
}

func TestGenericOIDCWithCustomCA(t *testing.T) {
	t.Parallel()

	const audience = "example.teleport.sh"

	clock := clockwork.NewFakeClock()
	idp := newFakeIDP(t, clock, audience)
	httpsIDP := newFakeHTTPSIDP(t, idp)

	expires := time.Now().Add(time.Hour)
	token := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name:    "test",
			Expires: &expires,
		},
		Spec: types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodGenericOIDC,
			BotName:    "example",
			Roles:      []types.SystemRole{types.RoleBot},
			GenericOIDC: &types.ProvisionTokenSpecV2GenericOIDC{
				Audience: audience,
				Issuer:   httpsIDP.issuer(),
				TLSCA:    httpsIDP.rotateCA(t),
				MustMatchFields: specStruct(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com"
				}`),
			},
		},
	}

	validator, err := newIDTokenValidatorWithClock(clock)
	require.NoError(t, err)

	// First, make a request that should fail: rotate (discarding the CA) and
	// try using the old one. We need to do this first since otherwise a working
	// validator would be cached.
	_ = httpsIDP.rotateCA(t)
	claims, err := validator.ValidateToken(
		t.Context(),
		token,
		[]byte(httpsIDP.issueToken(t, audience, "example", time.Hour)),
	)
	require.ErrorContains(t, err, "x509: certificate signed by unknown authority")
	require.Nil(t, claims)

	// Now make a real validation request. It should fetch using the custom CA.
	token.Spec.GenericOIDC.TLSCA = httpsIDP.rotateCA(t)
	claims, err = validator.ValidateToken(
		t.Context(),
		token,
		[]byte(httpsIDP.issueToken(t, audience, "example", time.Hour)),
	)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// Try again, but with the new CA. If the old client is used, then it would
	// try using the old cached CA and fail.
	token.Spec.GenericOIDC.TLSCA = httpsIDP.rotateCA(t)

	claims, err = validator.ValidateToken(
		t.Context(),
		token,
		[]byte(httpsIDP.issueToken(t, audience, "example", time.Hour)),
	)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// One more try to prove the original caching claim: rotate the CA, but
	// don't update the token. It'll use a cached validator and succeed, since
	// it'll never make a request in the first place.
	_ = httpsIDP.rotateCA(t)

	claims, err = validator.ValidateToken(
		t.Context(),
		token,
		[]byte(httpsIDP.issueToken(t, audience, "example", time.Hour)),
	)
	require.NoError(t, err)
	require.NotNil(t, claims)
}

// TestKeyForToken ensures unique caching keys are generated when the CA changes
// but iss/aud remain the same.
func TestKeyForToken(t *testing.T) {
	spec := func(ca string) *types.ProvisionTokenSpecV2GenericOIDC {
		return &types.ProvisionTokenSpecV2GenericOIDC{Issuer: "iss", Audience: "aud", TLSCA: ca}
	}

	none := keyForToken(spec(""))
	caA := keyForToken(spec("ca-A"))
	caA2 := keyForToken(spec("ca-A"))
	caB := keyForToken(spec("ca-B"))

	require.Equal(t, caA, caA2, "same CA must reuse the cached instance")
	require.NotEqual(t, caA, caB, "different CA must build a new instance")
	require.NotEqual(t, none, caA, "absent vs present CA must differ")
	require.Empty(t, none.tlsCAHash)
	require.NotEmpty(t, caA.tlsCAHash)
}

func TestValidateStaticJWKS(t *testing.T) {
	clock := clockwork.NewFakeClock()

	idp := newFakeIDP(t, clock, "aud")
	v, err := newIDTokenValidatorWithClock(clock)
	require.NoError(t, err)

	const issuer = "https://static.example.test"

	// validToken is a valid token jose accepts, with a `kid`
	validToken := []byte(idp.issueTokenWithIssuer(t, issuer, "aud", "sub", time.Hour))

	// noKidToken is missing the `kid` field
	noKidSigner, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: idp.privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	require.NoError(t, err)
	noKidToken := []byte(idp.issueTokenWithIssuerAndSigner(
		t, noKidSigner, issuer, "aud", "sub", time.Hour,
	))

	tests := []struct {
		name         string
		token        []byte
		mutateSpec   func(spec *types.ProvisionTokenSpecV2GenericOIDC)
		nowOffset    time.Duration
		expectError  require.ErrorAssertionFunc
		expectClaims bool
	}{
		{
			name:         "success with sane values",
			token:        validToken,
			expectError:  require.NoError,
			expectClaims: true,
		},
		{
			name:      "fails when expired",
			token:     validToken,
			nowOffset: 2 * time.Hour,
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "validating standard claims")
			},
		},
		{
			name:  "fails with mismatched audience",
			token: validToken,
			mutateSpec: func(spec *types.ProvisionTokenSpecV2GenericOIDC) {
				spec.Audience = "invalid"
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "validating standard claims")
			},
		},
		{
			name:  "fails when token is missing kid",
			token: noKidToken,
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "JWK with matching kid not found")
			},
		},
		{
			name:  "fails with unusable key",
			token: validToken,
			mutateSpec: func(spec *types.ProvisionTokenSpecV2GenericOIDC) {
				b, err := json.Marshal(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
					Key:       []byte("fake-symmetric-key"),
					KeyID:     "test-key-id",
					Algorithm: string(jose.HS256),
					Use:       "sig",
				}}})
				require.NoError(t, err)

				spec.StaticJWKS = string(b)
			},
			expectError: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "contains no keys with a usable signature algorithm")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &types.ProvisionTokenSpecV2GenericOIDC{
				Issuer:     issuer,
				Audience:   "aud",
				StaticJWKS: idp.staticJWKSConfig(t),
			}
			if tt.mutateSpec != nil {
				tt.mutateSpec(spec)
			}

			claims, err := v.validateStaticJWKS(t.Context(), spec, tt.token, clock.Now().Add(tt.nowOffset))
			tt.expectError(t, err)

			if tt.expectClaims {
				require.NotNil(t, claims)
			} else {
				require.Nil(t, claims)
			}
		})
	}
}

func createPubKey(t *testing.T, alg cryptosuites.Algorithm) crypto.PublicKey {
	t.Helper()
	priv, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
	require.NoError(t, err)
	return priv.Public()
}

func createECPubKey(t *testing.T, curve elliptic.Curve) crypto.PublicKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	return priv.Public()
}

func TestAllowedAlgorithmsFromJWKS(t *testing.T) {
	rsaPub := createPubKey(t, cryptosuites.RSA2048)
	ecP256 := createPubKey(t, cryptosuites.ECDSAP256)
	ecP384 := createECPubKey(t, elliptic.P384())
	ecP521 := createECPubKey(t, elliptic.P521())
	edPub := createPubKey(t, cryptosuites.Ed25519)

	key := func(key crypto.PublicKey, alg jose.SignatureAlgorithm) jose.JSONWebKey {
		return jose.JSONWebKey{
			Key:       key,
			Algorithm: string(alg),
		}
	}

	keys := func(keys ...jose.JSONWebKey) []jose.JSONWebKey {
		return keys
	}

	algs := func(algs ...jose.SignatureAlgorithm) []jose.SignatureAlgorithm {
		// stupid helper to save typing
		return algs
	}

	tests := []struct {
		name     string
		keys     []jose.JSONWebKey
		wantAlgs []jose.SignatureAlgorithm
		wantKeys []jose.JSONWebKey
	}{
		{
			name: "acceptable algorithms are passed through",
			keys: keys(
				key(rsaPub, jose.RS256),
				key(rsaPub, jose.RS384),
				key(rsaPub, jose.RS512),
				key(ecP256, jose.ES256),
				key(ecP384, jose.ES384),
				key(ecP521, jose.ES512),
				key(edPub, jose.EdDSA),
			),
			wantAlgs: algs(
				jose.RS256, jose.RS384, jose.RS512,
				jose.ES256, jose.ES384, jose.ES512,
				jose.EdDSA,
			),
			wantKeys: keys(
				key(rsaPub, jose.RS256),
				key(rsaPub, jose.RS384),
				key(rsaPub, jose.RS512),
				key(ecP256, jose.ES256),
				key(ecP384, jose.ES384),
				key(ecP521, jose.ES512),
				key(edPub, jose.EdDSA),
			),
		},
		{
			name: "symmetric requests are dropped",
			keys: keys(
				key(rsaPub, jose.HS256),
				key(rsaPub, jose.HS384),
				key(rsaPub, jose.HS512),
			),
			wantAlgs: algs(), // empty
			wantKeys: keys(),
		},
		{
			name: "only allowed keys pass",
			keys: keys(
				// all allowed key types...
				key(rsaPub, jose.RS256),
				key(rsaPub, jose.RS384),
				key(rsaPub, jose.RS512),
				key(ecP256, jose.ES256),
				key(ecP384, jose.ES384),
				key(ecP521, jose.ES512),
				key(edPub, jose.EdDSA),

				// various banned key types
				key(rsaPub, jose.HS256),
				key(rsaPub, jose.HS384),
				key(rsaPub, jose.HS512),
			),
			wantAlgs: algs(
				jose.RS256, jose.RS384, jose.RS512,
				jose.ES256, jose.ES384, jose.ES512,
				jose.EdDSA,
			),
			wantKeys: keys(
				key(rsaPub, jose.RS256),
				key(rsaPub, jose.RS384),
				key(rsaPub, jose.RS512),
				key(ecP256, jose.ES256),
				key(ecP384, jose.ES384),
				key(ecP521, jose.ES512),
				key(edPub, jose.EdDSA),
			),
		},
		{
			name:     "rsa without alg defaults to RS256",
			keys:     keys(key(rsaPub, "")),
			wantAlgs: algs(jose.RS256),
			wantKeys: keys(key(rsaPub, "")), // alg passes through unmodified
		},
		{
			name:     "ecdsa P-256 without alg",
			keys:     keys(key(ecP256, "")),
			wantAlgs: algs(jose.ES256),
			wantKeys: keys(key(ecP256, "")),
		},
		{
			name:     "ecdsa P-384 without alg",
			keys:     keys(key(ecP384, "")),
			wantAlgs: algs(jose.ES384),
			wantKeys: keys(key(ecP384, "")),
		},
		{
			name:     "ecdsa P-521 without alg maps to ES512",
			keys:     keys(key(ecP521, "")),
			wantAlgs: algs(jose.ES512),
			wantKeys: keys(key(ecP521, "")),
		},
		{
			name:     "ed25519 without alg",
			keys:     keys(key(edPub, "")),
			wantAlgs: algs(jose.EdDSA),
			wantKeys: keys(key(edPub, "")),
		},
		{
			name:     "duplicates are removed",
			keys:     keys(key(rsaPub, jose.RS256), key(rsaPub, jose.RS256)),
			wantAlgs: algs(jose.RS256),

			// keys are not deduped, only algs
			wantKeys: keys(key(rsaPub, jose.RS256), key(rsaPub, jose.RS256)),
		},
		{
			name: "none is ignored",
			keys: keys(
				key([]byte{}, jose.SignatureAlgorithm("none")),
			),
			wantAlgs: []jose.SignatureAlgorithm{},
		},
		{
			name:     "encryption-use key is skipped",
			keys:     keys(jose.JSONWebKey{Key: rsaPub, Use: "enc"}),
			wantAlgs: []jose.SignatureAlgorithm{},
		},
		{
			name: "empty",
			// weirdly, nothing to do
		},
		{
			name: "nil",
			keys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			algs, keys := filterAllowedAlgorithmsFromJWKS(jose.JSONWebKeySet{Keys: tt.keys})
			require.ElementsMatch(t, tt.wantAlgs, algs)
			require.ElementsMatch(t, tt.wantKeys, keys.Keys)
		})
	}
}
