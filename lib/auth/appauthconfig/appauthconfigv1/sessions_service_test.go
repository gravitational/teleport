// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package appauthconfigv1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/appauthconfig"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
)

func TestRetrieveJWKS(t *testing.T) {
	_, baseSigs, baseJwks := newJWTSigner(t)
	encodedJwks, err := json.Marshal(baseJwks)
	require.NoError(t, err)

	assertEqualJwks := func(expectedJwks *jose.JSONWebKeySet) require.ValueAssertionFunc {
		return func(tt require.TestingT, i1 any, i2 ...any) {
			require.Empty(t, cmp.Diff(
				expectedJwks,
				i1,
				cmpopts.IgnoreFields(
					jose.JSONWebKey{},
					"Key",
					"Certificates",
					"CertificateThumbprintSHA1",
					"CertificateThumbprintSHA256",
				),
			))
		}
	}

	t.Run("static", func(t *testing.T) {
		t.Run("valid", func(t *testing.T) {
			jwksRes, sigsRes, err := retrieveJWKSAppAuthConfig(
				&appauthconfigv1.AppAuthConfigJWTSpec{
					KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_StaticJwks{
						StaticJwks: string(encodedJwks),
					},
				},
				nil, /* httpClient */
			)
			require.NoError(t, err)
			assertEqualJwks(baseJwks)(t, jwksRes)
			require.ElementsMatch(t, baseSigs, sigsRes)
		})

		t.Run("empty", func(t *testing.T) {
			_, _, err := retrieveJWKSAppAuthConfig(
				&appauthconfigv1.AppAuthConfigJWTSpec{
					KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_StaticJwks{
						StaticJwks: "{}",
					},
				},
				nil, /* httpClient */
			)
			require.Error(t, err)
		})
	})

	t.Run("url", func(t *testing.T) {
		for name, tc := range map[string]struct {
			httpHandler http.HandlerFunc
			assertError require.ErrorAssertionFunc
			assertJwks  require.ValueAssertionFunc
			assertSigs  require.ValueAssertionFunc
		}{
			"success with valid JWKS": {
				httpHandler: func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write(encodedJwks)
				},
				assertError: require.NoError,
				assertJwks:  assertEqualJwks(baseJwks),
				assertSigs: func(tt require.TestingT, i1 any, i2 ...any) {
					require.ElementsMatch(t, baseSigs, i1, i2...)
				},
			},
			"success with empty JWKS": {
				httpHandler: func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("{}"))
				},
				assertError: require.Error,
				assertJwks:  require.Nil,
				assertSigs:  require.Empty,
			},
			"error": {
				httpHandler: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
				assertError: require.Error,
				assertJwks:  require.Nil,
				assertSigs:  require.Empty,
			},
		} {
			t.Run(name, func(t *testing.T) {
				httpSrv := httptest.NewServer(tc.httpHandler)
				t.Cleanup(func() { httpSrv.Close() })

				jwks, sigs, err := retrieveJWKSAppAuthConfig(
					&appauthconfigv1.AppAuthConfigJWTSpec{
						KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
							JwksUrl: httpSrv.URL,
						},
					},
					httpSrv.Client(),
				)
				tc.assertError(t, err)
				tc.assertJwks(t, jwks)
				tc.assertSigs(t, sigs)
			})
		}
	})
}

func TestVerifyJWTToken(t *testing.T) {
	signer, sigs, jwks := newJWTSigner(t)

	for name, tc := range map[string]struct {
		config         *appauthconfigv1.AppAuthConfigJWTSpec
		claims         jwt.Claims
		usernameClaim  any
		assertError    require.ErrorAssertionFunc
		assertUsername require.ValueAssertionFunc
	}{
		"valid token": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:        "https://issuer-url/",
				Audience:      "teleport",
				UsernameClaim: "username",
			},
			claims: jwt.Claims{
				Issuer:   "https://issuer-url/",
				Audience: []string{"teleport"},
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			usernameClaim: struct {
				Username string `json:"username"`
			}{Username: "user@example.com"},
			assertError: require.NoError,
			assertUsername: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "user@example.com", i1, i2)
			},
		},
		"default username claim": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:   "https://issuer-url/",
				Audience: "teleport",
			},
			claims: jwt.Claims{
				Issuer:   "https://issuer-url/",
				Audience: []string{"teleport"},
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			usernameClaim: struct {
				Email string `json:"email"`
			}{Email: "user@example.com"},
			assertError: require.NoError,
			assertUsername: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(tt, "user@example.com", i1, i2)
			},
		},
		"wrong audience": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:   "https://issuer-url/",
				Audience: "teleport",
			},
			claims: jwt.Claims{
				Issuer:   "https://issuer-url/",
				Audience: []string{"random-app"},
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			usernameClaim: struct {
				Email string `json:"email"`
			}{Email: "user@example.com"},
			assertError:    require.Error,
			assertUsername: require.Empty,
		},
		"wrong issuer": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:   "https://issuer-url/",
				Audience: "teleport",
			},
			claims: jwt.Claims{
				Issuer:   "https://random-issuer/",
				Audience: []string{"teleport"},
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			usernameClaim: struct {
				Email string `json:"email"`
			}{Email: "user@example.com"},
			assertError:    require.Error,
			assertUsername: require.Empty,
		},
		"missing username claim": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:   "https://issuer-url/",
				Audience: "teleport",
			},
			claims: jwt.Claims{
				Issuer:   "https://issuer-url/",
				Audience: []string{"teleport"},
				IssuedAt: jwt.NewNumericDate(time.Now()),
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			usernameClaim:  struct{}{},
			assertError:    require.Error,
			assertUsername: require.Empty,
		},
		"missing issued at claim": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:   "https://issuer-url/",
				Audience: "teleport",
			},
			claims: jwt.Claims{
				Issuer:   "https://issuer-url/",
				Audience: []string{"teleport"},
				Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
			usernameClaim: struct {
				Email string `json:"email"`
			}{Email: "user@example.com"},
			assertError:    require.Error,
			assertUsername: require.Empty,
		},
		"missing exp claim": {
			config: &appauthconfigv1.AppAuthConfigJWTSpec{
				Issuer:   "https://issuer-url/",
				Audience: "teleport",
			},
			claims: jwt.Claims{
				Issuer:   "https://issuer-url/",
				Audience: []string{"teleport"},
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
			usernameClaim: struct {
				Email string `json:"email"`
			}{Email: "user@example.com"},
			assertError:    require.Error,
			assertUsername: require.Empty,
		},
	} {
		t.Run(name, func(t *testing.T) {
			jwtToken, err := jwt.Signed(signer).Claims(tc.claims).Claims(tc.usernameClaim).Serialize()
			require.NoError(t, err)

			username, _, err := verifyAppAuthJWTToken(jwtToken, jwks, sigs, tc.config)
			tc.assertError(t, err)
			tc.assertUsername(t, username)
		})
	}

	t.Run("token freshness", func(t *testing.T) {
		config := &appauthconfigv1.AppAuthConfigJWTSpec{
			Issuer:        "https://issuer-url/",
			Audience:      "teleport",
			UsernameClaim: "email",
		}
		baseClaims := jwt.Claims{
			Issuer:   config.Issuer,
			Audience: []string{config.Audience},
		}
		usernameClaim := struct {
			Email string `json:"email"`
		}{Email: "user@example.com"}

		for name, tc := range map[string]struct {
			passedTime  time.Duration
			assertError require.ErrorAssertionFunc
		}{
			"fresh token succeeds": {
				passedTime:  0,
				assertError: require.NoError,
			},
			"outside freshness range": {
				passedTime:  jwtMaxIssuedAtAfter + 1,
				assertError: require.Error,
			},
		} {
			t.Run(name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					claims := baseClaims
					issuedAt := time.Now()
					claims.IssuedAt = jwt.NewNumericDate(issuedAt)
					claims.Expiry = jwt.NewNumericDate(issuedAt.Add(tc.passedTime + time.Hour)) // Always generate non-expired token.

					jwtToken, err := jwt.Signed(signer).Claims(claims).Claims(usernameClaim).Serialize()
					require.NoError(t, err)

					time.Sleep(tc.passedTime)

					_, _, err = verifyAppAuthJWTToken(jwtToken, jwks, sigs, config)
					tc.assertError(t, err)
				})
			})
		}
	})
}

func TestCreateAppSessionWithJWT(t *testing.T) {
	issuer := "https://external-idp/"
	header := "Authorization"
	audience := "teleport"
	usernameClaim := "email"

	signer, _, baseJwks := newJWTSigner(t)
	jwtToken := generateJWTToken(t, signer, issuer, audience)
	encodedJwks, err := json.Marshal(baseJwks)
	require.NoError(t, err)

	user, err := types.NewUser("user")
	require.NoError(t, err)

	config := appauthconfig.NewAppAuthConfigJWT("test-config", []*labelv1.Label{{Name: "*", Values: []string{"*"}}}, &appauthconfigv1.AppAuthConfigJWTSpec{
		Issuer:              issuer,
		AuthorizationHeader: header,
		Audience:            audience,
		UsernameClaim:       usernameClaim,
		KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_StaticJwks{
			StaticJwks: string(encodedJwks),
		},
	})

	assertAuditEventFailure := func(tt require.TestingT, i1 any, i2 ...any) {
		require.IsType(t, &apievents.AppAuthConfigVerify{}, i1, i2...)
		evt, _ := i1.(*apievents.AppAuthConfigVerify)
		require.Equal(t, events.AppAuthConfigVerifyFailureCode, evt.Metadata.Code)
		require.Equal(t, events.AppAuthConfigVerifyFailureEvent, evt.Metadata.Type)
		require.False(t, evt.Status.Success)
		require.NotEmpty(t, evt.Status.Error)
	}

	for name, tc := range map[string]struct {
		authorizer          authz.AuthorizerFunc
		userGetter          *mockUserGetter
		config              *appauthconfigv1.AppAuthConfig
		configErr           error
		createAppSessionErr error
		token               string
		assertError         require.ErrorAssertionFunc
		assertAuditEvent    require.ValueAssertionFunc
	}{
		"create new session": {
			config: config,
			token:  jwtToken,
			authorizer: func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Identity: authz.BuiltinRole{Role: types.RoleProxy},
					Checker:  &fakeAccessChecker{role: types.RoleProxy},
				}, nil

			},
			userGetter:  &mockUserGetter{getStateErr: trace.NotFound(""), user: user},
			assertError: require.NoError,
			assertAuditEvent: func(tt require.TestingT, i1 any, i2 ...any) {
				require.IsType(t, &apievents.AppAuthConfigVerify{}, i1, i2...)
				evt, _ := i1.(*apievents.AppAuthConfigVerify)
				require.Equal(t, events.AppAuthConfigVerifySuccessCode, evt.Metadata.Code)
				require.Equal(t, events.AppAuthConfigVerifySuccessEvent, evt.Metadata.Type)
				require.True(t, evt.Status.Success)
			},
		},
		"user not found": {
			config: config,
			token:  jwtToken,
			authorizer: func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Identity: authz.BuiltinRole{Role: types.RoleProxy},
					Checker:  &fakeAccessChecker{role: types.RoleProxy},
				}, nil

			},
			userGetter:       &mockUserGetter{getStateErr: trace.NotFound(""), getUserErr: trace.NotFound("")},
			assertError:      require.Error,
			assertAuditEvent: assertAuditEventFailure,
		},
		"wrong role requesting": {
			config: config,
			token:  jwtToken,
			authorizer: func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Identity: authz.BuiltinRole{Role: types.RoleAuth},
					Checker:  &fakeAccessChecker{role: types.RoleAuth},
				}, nil
			},
			assertError:      require.Error,
			assertAuditEvent: assertAuditEventFailure,
		},
		"invalid token": {
			config: config,
			token:  "",
			authorizer: func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Identity: authz.BuiltinRole{Role: types.RoleProxy},
					Checker:  &fakeAccessChecker{role: types.RoleProxy},
				}, nil
			},
			assertError:      require.Error,
			assertAuditEvent: assertAuditEventFailure,
		},
		"error while retrieving config": {
			configErr: trace.NotFound("config not found"),
			token:     jwtToken,
			authorizer: func(ctx context.Context) (*authz.Context, error) {
				return &authz.Context{
					Identity: authz.BuiltinRole{Role: types.RoleProxy},
					Checker:  &fakeAccessChecker{role: types.RoleProxy},
				}, nil
			},
			assertError:      require.Error,
			assertAuditEvent: assertAuditEventFailure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			emitter := &eventstest.MockRecorderEmitter{}
			svc, err := NewSessionsService(SessionsServiceConfig{
				Emitter: emitter,
				Reader: &mockAppAuthConfigReader{
					getConfig:    tc.config,
					getConfigErr: tc.configErr,
				},
				SessionsCreator: &mockSessionsCreator{
					session:    createAppSession(t),
					sessionErr: tc.createAppSessionErr,
				},
				Authorizer: tc.authorizer,
				UserGetter: tc.userGetter,
			})
			require.NoError(t, err)

			_, err = svc.CreateAppSessionWithJWT(t.Context(), &appauthconfigv1.CreateAppSessionWithJWTRequest{
				ConfigName: "app-config-example",
				Jwt:        tc.token,
				App: &appauthconfigv1.App{
					AppName:     "mcp-app",
					PublicAddr:  "https://proxy/mcp-app",
					Uri:         "mcp+https://localhost/mcp",
					ClusterName: "example",
				},
			})
			tc.assertError(t, err)
			tc.assertAuditEvent(t, emitter.LastEvent())
		})
	}
}

type mockAppAuthConfigReader struct {
	services.AppAuthConfigReader

	getConfig    *appauthconfigv1.AppAuthConfig
	getConfigErr error
}

func (m *mockAppAuthConfigReader) GetAppAuthConfig(_ context.Context, _ string) (*appauthconfigv1.AppAuthConfig, error) {
	return m.getConfig, m.getConfigErr
}

type mockSessionsCreator struct {
	session    types.WebSession
	sessionErr error
}

func (m *mockSessionsCreator) CreateAppSessionForAppAuth(ctx context.Context, req *CreateAppSessionForAppAuthRequest) (types.WebSession, error) {
	return m.session, m.sessionErr
}

type fakeAccessChecker struct {
	services.AccessChecker
	role types.SystemRole
}

func (f fakeAccessChecker) HasRole(role string) bool {
	return string(f.role) == role
}

type mockUserGetter struct {
	services.UserOrLoginStateGetter

	state       *userloginstate.UserLoginState
	getStateErr error
	user        types.User
	getUserErr  error
}

func (m *mockUserGetter) GetUserLoginState(context.Context, string) (*userloginstate.UserLoginState, error) {
	return m.state, m.getStateErr
}

func (m *mockUserGetter) GetUser(context.Context, string, bool) (types.User, error) {
	return m.user, m.getUserErr
}

func newJWTSigner(t *testing.T) (jose.Signer, []jose.SignatureAlgorithm, *jose.JSONWebKeySet) {
	t.Helper()

	kid := "kid-example"
	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	require.NoError(t, err)

	jwks := &jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Algorithm: string(jose.ES256), KeyID: kid, Key: privateKey.Public()},
	}}
	return signer, []jose.SignatureAlgorithm{jose.ES256}, jwks
}

func generateJWTToken(t *testing.T, signer jose.Signer, issuer, audience string) string {
	t.Helper()

	token, err := jwt.Signed(signer).Claims(jwt.Claims{
		Issuer:   issuer,
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(time.Now()),
		Expiry:   jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).Claims(
		struct {
			Email string `json:"email"`
		}{Email: "user@example.com"},
	).Serialize()
	require.NoError(t, err)
	return token
}

func createAppSession(t *testing.T) types.WebSession {
	t.Helper()
	appSession, err := types.NewWebSession(uuid.New().String(), types.KindAppSession, types.WebSessionSpecV2{
		User: "testuser",
	})
	require.NoError(t, err)
	return appSession
}
