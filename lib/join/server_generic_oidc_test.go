/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package join_test

import (
	"cmp"
	"crypto"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/join/genericoidc"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

// fakeIDP provides a minimal fake OIDC provider for use in tests
type fakeIDP struct {
	t          *testing.T
	clock      clockwork.Clock
	privateKey crypto.Signer
	signer     jose.Signer
	publicKey  crypto.PublicKey
	server     *httptest.Server
	mux        *http.ServeMux
	keyID      string

	audience string
}

func newGenericOIDCFakeIDP(t *testing.T, clock clockwork.Clock, audience string) *fakeIDP {
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
	issuer := "http://" + r.Host
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
	claimsMutators ...func(*genericoidc.IDTokenClaims),
) string {
	claims := genericoidc.IDTokenClaims{
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
	claimsMutators ...func(*genericoidc.IDTokenClaims),
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

func TestJoinGenericOIDC(t *testing.T) {
	// Namespaced helpers within the function - these aren't useful for
	// constructing scoped tokens and we might as well not pollute the package
	// namespace with these helpers.
	// Additionally, note that we skip various generic_oidc features that are
	// thoroughly tested in lib/join/genericoidc tests, e.g. static_jwks, custom
	// TLS CAs, exhaustive conditions, expr functionality, etc.
	eqCondition := func(attribute, value string) *types.ProvisionTokenSpecV2GenericOIDC_Condition {
		return &types.ProvisionTokenSpecV2GenericOIDC_Condition{
			Attribute: attribute,
			Eq: &types.ProvisionTokenSpecV2GenericOIDC_ConditionEq{
				Value: value,
			},
		}
	}

	conditions := func(conditions ...*types.ProvisionTokenSpecV2GenericOIDC_Condition) *types.ProvisionTokenSpecV2GenericOIDC_Rule {
		return &types.ProvisionTokenSpecV2GenericOIDC_Rule{
			Conditions: conditions,
		}
	}

	expression := func(expr string) *types.ProvisionTokenSpecV2GenericOIDC_Rule {
		return &types.ProvisionTokenSpecV2GenericOIDC_Rule{
			Expression: expr,
		}
	}

	setAllowAny := func(rules ...*types.ProvisionTokenSpecV2GenericOIDC_Rule) func(*types.ProvisionTokenV2) {
		return func(ptv *types.ProvisionTokenV2) {
			ptv.Spec.GenericOIDC.AllowAny = rules
		}
	}

	setMustMatchFields := func(t *testing.T, doc string) func(*types.ProvisionTokenV2) {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(doc), &m))

		s, err := types.NewStructFromGoValues(m)
		require.NoError(t, err)

		return func(v2 *types.ProvisionTokenV2) {
			v2.Spec.GenericOIDC.MustMatchFields = s
		}
	}

	combineMutators := func(muts ...func(*types.ProvisionTokenV2)) func(*types.ProvisionTokenV2) {
		return func(ptv *types.ProvisionTokenV2) {
			for _, m := range muts {
				m(ptv)
			}
		}
	}

	t.Parallel()
	ctx := t.Context()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(ctx)) })

	const audience = "example.teleport.sh"
	idp := newGenericOIDCFakeIDP(t, clockwork.NewRealClock(), audience)

	expires := time.Now().Add(time.Hour)
	baseToken := &types.ProvisionTokenV2{
		Metadata: types.Metadata{
			Name:    "test",
			Expires: &expires,
		},
		Spec: types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodGenericOIDC,
			Roles:      []types.SystemRole{types.RoleApp},
			GenericOIDC: &types.ProvisionTokenSpecV2GenericOIDC{
				Audience:                audience,
				InsecureAllowHTTPIssuer: true,
			},
		},
	}

	tests := []struct {
		name             string
		requestTokenName string
		mutateToken      func(token *types.ProvisionTokenV2)
		expectError      require.ErrorAssertionFunc
		issueToken       func(t *testing.T) string
	}{
		{
			name: "simple success",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com",
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "nope"`),
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			expectError: require.NoError,
		},
		{
			name: "no-allow-any-matches-fail",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com",
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "nope"`),
					expression(`contains(claims.custom.list", "d")`),
					conditions(eqCondition("email", "not an email")),
				),
			),
			expectError: require.Error, // note, error messages have no useful content
		},
		{
			name: "failed-field-match-fails",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com",
					"google": {
						"compute_engine": {
							"project_id": "wrong",
							"project_number": 123456123456
						}
					}
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			expectError: require.Error,
		},
		{
			name: "incorrect-issuer-fails",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com"
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			issueToken: func(t *testing.T) string {
				return idp.issueTokenWithIssuer(t, "http://not-correct.nope", audience, "test", time.Hour)
			},
			expectError: require.Error,
		},
		{
			name: "incorrect-audience-fails",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com"
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			issueToken: func(t *testing.T) string {
				return idp.issueToken(t, "incorrect-audience", "test", time.Hour)
			},
			expectError: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, ok := baseToken.Clone().(*types.ProvisionTokenV2)
			require.True(t, ok)

			token.Spec.GenericOIDC.Issuer = idp.issuer()
			if tt.mutateToken != nil {
				tt.mutateToken(token)
			}

			require.NoError(t, err)
			require.NoError(t, authServer.Auth().UpsertToken(ctx, token))
			t.Cleanup(func() {
				assert.NoError(t, authServer.Auth().DeleteToken(ctx, token.GetName()))
			})

			// Make an unauthenticated auth client that will be used for the join.
			nopClient, err := authServer.NewClient(authtest.TestNop())
			require.NoError(t, err)
			defer nopClient.Close()

			var idToken string
			if tt.issueToken != nil {
				idToken = tt.issueToken(t)
			} else {
				idToken = idp.issueToken(t, audience, "test", time.Hour)
			}

			_, err = joinclient.Join(ctx, joinclient.JoinParams{
				Token: cmp.Or(tt.requestTokenName, token.GetName()),
				ID: state.IdentityID{
					Role:     types.RoleInstance,
					NodeName: "test-node",
				},
				// joinWithMethod only creates a tokenSource when this is unset
				IDToken:    idToken,
				AuthClient: nopClient,
			})
			tt.expectError(t, err)
		})
	}
}

func TestJoinGenericOIDCScoped(t *testing.T) {
	// As above, duplicate the various helpers but translate them for scoped
	// tokens.
	eqCondition := func(attribute, value string) *joiningv1.GenericOIDC_Condition {
		return joiningv1.GenericOIDC_Condition_builder{
			Attribute: attribute,
			Eq:        joiningv1.GenericOIDC_ConditionEq_builder{Value: value}.Build(),
		}.Build()
	}

	conditions := func(conditions ...*joiningv1.GenericOIDC_Condition) *joiningv1.GenericOIDC_Rule {
		return joiningv1.GenericOIDC_Rule_builder{Conditions: conditions}.Build()
	}

	expression := func(expr string) *joiningv1.GenericOIDC_Rule {
		return joiningv1.GenericOIDC_Rule_builder{Expression: expr}.Build()
	}

	// Note, we deliberately omit some tests here that would otherwise be
	// redundant given other tests covering functionality elsewhere, e.g.
	// not_eq/in/not_in.

	setAllowAny := func(rules ...*joiningv1.GenericOIDC_Rule) func(*joiningv1.ScopedToken) {
		return func(token *joiningv1.ScopedToken) {
			token.GetSpec().GetGenericOidc().SetAllowAny(rules)
		}
	}

	setMustMatchFields := func(t *testing.T, doc string) func(*joiningv1.ScopedToken) {
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(doc), &m))

		s, err := structpb.NewStruct(m)
		require.NoError(t, err)

		return func(token *joiningv1.ScopedToken) {
			token.GetSpec().GetGenericOidc().SetMustMatchFields(s)
		}
	}

	combineMutators := func(muts ...func(*joiningv1.ScopedToken)) func(*joiningv1.ScopedToken) {
		return func(token *joiningv1.ScopedToken) {
			for _, m := range muts {
				m(token)
			}
		}
	}

	t.Parallel()
	ctx := t.Context()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:            t.TempDir(),
			ScopesFeatures: scopes.Features{Enabled: true},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(ctx)) })

	const audience = "example.teleport.sh"
	idp := newGenericOIDCFakeIDP(t, clockwork.NewRealClock(), audience)

	expires := time.Now().Add(time.Hour)
	baseToken := joiningv1.ScopedToken_builder{
		Kind:    types.KindScopedToken,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:    "test",
			Expires: timestamppb.New(expires),
		}.Build(),
		Scope: "/example",
		Spec: joiningv1.ScopedTokenSpec_builder{
			AssignedScope: "/example/agents",
			Roles:         []string{types.RoleNode.String()},
			JoinMethod:    string(types.JoinMethodGenericOIDC),
			UsageMode:     string(joining.TokenUsageModeUnlimited),
			GenericOidc: joiningv1.GenericOIDC_builder{
				Audience:                audience,
				InsecureAllowHttpIssuer: true,
			}.Build(),
		}.Build(),
	}.Build()

	tests := []struct {
		name             string
		requestTokenName string
		mutateToken      func(token *joiningv1.ScopedToken)
		expectError      require.ErrorAssertionFunc
		issueToken       func(t *testing.T) string
	}{
		{
			name: "simple success",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com",
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "nope"`),
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			expectError: require.NoError,
		},
		{
			name: "no-allow-any-matches-fail",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com",
					"google": {
						"compute_engine": {
							"project_id": "example-123456",
							"project_number": 123456123456
						}
					}
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "nope"`),
					conditions(eqCondition("email", "not an email")),
				),
			),
			expectError: require.Error, // note, error messages have no useful content
		},
		{
			name: "failed-field-match-fails",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com",
					"google": {
						"compute_engine": {
							"project_id": "wrong",
							"project_number": 123456123456
						}
					}
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			expectError: require.Error,
		},
		{
			name: "incorrect-issuer-fails",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com"
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			issueToken: func(t *testing.T) string {
				return idp.issueTokenWithIssuer(t, "http://not-correct.nope", audience, "test", time.Hour)
			},
			expectError: require.Error,
		},
		{
			name: "incorrect-audience-fails",
			mutateToken: combineMutators(
				setMustMatchFields(t, `{
					"email": "123456789012-compute@developer.gserviceaccount.com"
				}`),
				setAllowAny(
					expression(`claims.google.compute_engine.instance_name == "hello-world"`),
				),
			),
			issueToken: func(t *testing.T) string {
				return idp.issueToken(t, "incorrect-audience", "test", time.Hour)
			},
			expectError: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := proto.CloneOf(baseToken)

			token.GetSpec().GetGenericOidc().SetIssuer(idp.issuer())
			if tt.mutateToken != nil {
				tt.mutateToken(token)
			}

			_, err := authServer.Auth().CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{
				Token: token,
			}.Build())
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := authServer.Auth().DeleteScopedToken(ctx, joiningv1.DeleteScopedTokenRequest_builder{
					Name: token.GetMetadata().GetName(),
				}.Build())
				assert.NoError(t, err)
			})

			// Make an unauthenticated auth client that will be used for the join.
			nopClient, err := authServer.NewClient(authtest.TestNop())
			require.NoError(t, err)
			defer nopClient.Close()

			var idToken string
			if tt.issueToken != nil {
				idToken = tt.issueToken(t)
			} else {
				idToken = idp.issueToken(t, audience, "test", time.Hour)
			}

			_, err = joinclient.Join(ctx, joinclient.JoinParams{
				Token: cmp.Or(tt.requestTokenName, token.GetMetadata().GetName()),
				ID: state.IdentityID{
					Role:     types.RoleInstance,
					NodeName: "test-node",
				},
				// joinWithMethod only creates a tokenSource when this is unset
				IDToken:    idToken,
				AuthClient: nopClient,
			})
			tt.expectError(t, err)
		})
	}
}
