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

package join_test

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	apiscopes "github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/join/gitlab"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/join/jointest"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/tlsca"
)

type mockGitLabTokenValidator struct {
	tokens           map[string]gitlab.IDTokenClaims
	lastCalledDomain string
	lastCalledJWKS   []byte
}

func (m *mockGitLabTokenValidator) Validate(
	_ context.Context, domain string, token string,
) (*gitlab.IDTokenClaims, error) {
	m.lastCalledDomain = domain

	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func (m *mockGitLabTokenValidator) ValidateTokenWithJWKS(
	_ context.Context, jwks []byte, token string,
) (*gitlab.IDTokenClaims, error) {
	m.lastCalledJWKS = jwks

	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func TestJoinGitlab(t *testing.T) {
	validIDToken := "test.fake.jwt"
	idTokenValidator := &mockGitLabTokenValidator{
		tokens: map[string]gitlab.IDTokenClaims{
			validIDToken: {
				Sub:                  "project_path:octo-org/octo-repo:ref_type:branch:ref:main",
				ProjectPath:          "octo-org/octo-group/octo-repo",
				NamespacePath:        "octo-org",
				PipelineSource:       "web",
				Environment:          "prod",
				UserLogin:            "octocat",
				Ref:                  "main",
				RefType:              "branch",
				UserID:               "13",
				UserEmail:            "octocat@example.com",
				RefProtected:         "true",
				EnvironmentProtected: "false",
				CIConfigSHA:          "11aabbcc",
				CIConfigRefURI:       "gitlab.example.com/my-group/my-project//.gitlab-ci.yml@refs/heads/main",
				DeploymentTier:       "production",
				ProjectVisibility:    "internal",
			},
		},
	}

	ctx := t.Context()

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:            t.TempDir(),
			ScopesFeatures: scopes.Features{Enabled: true},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(t.Context())) })
	auth := authServer.Auth()

	authServer.Auth().SetGitlabIDTokenValidator(idTokenValidator)

	// helper for creating RegisterUsingTokenRequest
	sshPrivateKey, sshPublicKey, err := testauthority.GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := authtest.PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)
	newRequest := func(idToken string) *types.RegisterUsingTokenRequest {
		return &types.RegisterUsingTokenRequest{
			HostID:       "host-id",
			Role:         types.RoleNode,
			IDToken:      idToken,
			PublicTLSKey: tlsPublicKey,
			PublicSSHKey: sshPublicKey,
		}
	}

	allowRule := func(modifier func(*types.ProvisionTokenSpecV2GitLab_Rule)) *types.ProvisionTokenSpecV2GitLab_Rule {
		rule := &types.ProvisionTokenSpecV2GitLab_Rule{
			Sub:            "project_path:octo-org/octo-repo:ref_type:branch:ref:main",
			ProjectPath:    "octo-org/octo-group/octo-repo",
			NamespacePath:  "octo-org",
			PipelineSource: "web",
			Environment:    "prod",
			Ref:            "main",
			RefType:        "branch",
			UserID:         "13",
			UserLogin:      "octocat",
			UserEmail:      "octocat@example.com",
			RefProtected: &types.BoolOption{
				Value: true,
			},
			EnvironmentProtected: &types.BoolOption{
				Value: false,
			},
			CIConfigSHA:       "11aabbcc",
			CIConfigRefURI:    "gitlab.example.com/my-group/my-project//.gitlab-ci.yml@refs/heads/main",
			DeploymentTier:    "production",
			ProjectVisibility: "internal",
		}
		if modifier != nil {
			modifier(rule)
		}
		return rule
	}

	allowRulesNotMatched := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "id token claims did not match any allow rules")
		require.True(t, trace.IsAccessDenied(err))
	})
	tests := []struct {
		name            string
		request         *types.RegisterUsingTokenRequest
		tokenSpecGitlab *types.ProvisionTokenSpecV2GitLab
		assertError     require.ErrorAssertionFunc
	}{
		{
			name: "success",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(nil),
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "domain-override",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Domain: "gitlab.example.com",
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(nil),
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "multiple-allow-rules",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.Sub = "not matching"
					}),
					allowRule(nil),
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "incorrect-sub",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.Sub = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "globby-project-path-match",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.ProjectPath = "octo-org/octo-group/*"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "globby-project-path-mismatch",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.ProjectPath = "octo-org/different-octo-group/*"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-project-path",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.ProjectPath = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-namespace-path",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.NamespacePath = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-pipeline-source",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.PipelineSource = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-environment",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.Environment = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-ref",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.Ref = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-ref-type",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.RefType = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-user_login",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.UserLogin = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-user_id",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.UserID = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-user_email",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.UserEmail = "not matching"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-ref_protected",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.RefProtected = &types.BoolOption{
							Value: false,
						}
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "ref_protected-ignored-if-nil",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.RefProtected = nil
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "incorrect-environment_protected",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.EnvironmentProtected = &types.BoolOption{
							Value: true,
						}
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-ci_config_sha",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.CIConfigSHA = "not match"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-ci_config_ref_uri",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.CIConfigRefURI = "not match"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-deployment_tier",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.DeploymentTier = "not match"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect-project_visibility",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
						rule.ProjectVisibility = "not match"
					}),
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "success-with-jwks",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(nil),
				},
				StaticJWKS: "xyzzy",
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "failure-with-jwks",
			tokenSpecGitlab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					allowRule(nil),
				},
				StaticJWKS: "xyzzy",
			},
			request:     newRequest("invalidjwt"),
			assertError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenSpec := types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodGitLab,
				GitLab:     tt.tokenSpecGitlab,
			}
			token, err := types.NewProvisionTokenFromSpec(
				tt.name, time.Now().Add(time.Minute), tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.CreateToken(ctx, token))
			tt.request.Token = tt.name

			nopClient, err := authServer.NewClient(authtest.TestNop())
			require.NoError(t, err)

			t.Run("legacy", func(t *testing.T) {
				_, err = auth.RegisterUsingToken(ctx, tt.request)
				tt.assertError(t, err)

				if tt.tokenSpecGitlab.Domain != "" {
					require.Equal(
						t,
						tt.tokenSpecGitlab.Domain,
						idTokenValidator.lastCalledDomain,
					)
				}
				if tt.tokenSpecGitlab.StaticJWKS != "" {
					require.Equal(
						t,
						[]byte(tt.tokenSpecGitlab.StaticJWKS),
						idTokenValidator.lastCalledJWKS,
					)
				} else {
					require.Nil(t, idTokenValidator.lastCalledJWKS)
				}
			})

			t.Run("legacy joinclient", func(t *testing.T) {
				_, err := joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      tt.request.Token,
					JoinMethod: types.JoinMethodGitLab,
					ID: state.IdentityID{
						Role:     tt.request.Role,
						NodeName: "testnode",
						HostUUID: tt.request.HostID,
					},
					IDToken:    tt.request.IDToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
				if err != nil {
					return
				}
			})

			t.Run("new joinclient", func(t *testing.T) {
				_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token:      tt.request.Token,
					JoinMethod: types.JoinMethodGitLab,
					ID: state.IdentityID{
						Role:     types.RoleInstance, // RoleNode is not allowed
						NodeName: "testnode",
					},
					IDToken:    tt.request.IDToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
				if err != nil {
					return
				}
			})

			t.Run("scoped join", func(t *testing.T) {
				ptv2, ok := token.(*types.ProvisionTokenV2)
				require.True(t, ok, "expected provision token to be types.ProvisionTokenSpecV2")
				scoped, err := jointest.ScopedTokenFromProvisionTokenSpec(ptv2.Spec, joiningv1.ScopedToken_builder{
					Scope: "/test",
					Metadata: headerv1.Metadata_builder{
						Name: token.GetName(),
					}.Build(),
					Spec: joiningv1.ScopedTokenSpec_builder{
						AssignedScope: "/test/one",
						UsageMode:     string(joining.TokenUsageModeUnlimited),
					}.Build(),
				}.Build())
				require.NoError(t, err)
				_, err = auth.CreateScopedToken(ctx, joiningv1.CreateScopedTokenRequest_builder{Token: scoped}.Build())
				require.NoError(t, err)

				_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
					Token:      apiscopes.QualifiedName{Scope: scoped.GetScope(), Name: scoped.GetMetadata().GetName()}.String(),
					JoinMethod: types.JoinMethodGitLab,
					ID: state.IdentityID{
						Role:     types.RoleInstance, // RoleNode is not allowed
						NodeName: "testnode",
					},
					IDToken:    tt.request.IDToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
				if err != nil {
					return
				}
			})
		})
	}
}

func TestJoinGitlabCIBot(t *testing.T) {
	validIDToken := "test.fake.jwt"
	idTokenValidator := &mockGitLabTokenValidator{
		tokens: map[string]gitlab.IDTokenClaims{
			validIDToken: {
				Sub:                  "project_path:octo-org/octo-repo:ref_type:branch:ref:main",
				ProjectPath:          "octo-org/octo-group/octo-repo",
				NamespacePath:        "octo-org",
				PipelineSource:       "web",
				Environment:          "prod",
				UserLogin:            "octocat",
				Ref:                  "main",
				RefType:              "branch",
				UserID:               "13",
				UserEmail:            "octocat@example.com",
				RefProtected:         "true",
				EnvironmentProtected: "false",
				CIConfigSHA:          "11aabbcc",
				CIConfigRefURI:       "gitlab.example.com/my-group/my-project//.gitlab-ci.yml@refs/heads/main",
				DeploymentTier:       "production",
				ProjectVisibility:    "internal",
			},
		},
	}

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:            t.TempDir(),
			ScopesFeatures: scopes.Features{Enabled: true},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(t.Context())) })

	authServer.Auth().SetGitlabIDTokenValidator(idTokenValidator)

	nopClient, err := authServer.NewClient(authtest.TestNop())
	require.NoError(t, err)

	t.Run("bot joins with valid scoped token", func(t *testing.T) {
		// Create the spec for a valid scoped token for bot joining.
		tokenSpec := types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodGitLab,
			Roles:      []types.SystemRole{types.RoleBot},
			GitLab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					{
						Sub:         "project_path:octo-org/octo-repo:ref_type:branch:ref:main",
						ProjectPath: "octo-org/octo-group/octo-repo",
					},
				},
			},
		}

		// Convert the token spec into a scoped token.
		scopedToken, err := jointest.ScopedTokenFromProvisionTokenSpec(tokenSpec, joiningv1.ScopedToken_builder{
			Scope: "/test",
			Metadata: headerv1.Metadata_builder{
				Name: "gitlabci-bot-token",
			}.Build(),
			Spec: joiningv1.ScopedTokenSpec_builder{
				UsageMode: joining.TokenUsageModeBot,
				Bot:       CreateScopedBot(t, authServer.Auth(), "gitlabci-bot"),
			}.Build(),
		}.Build())
		require.NoError(t, err)

		// Create a scoped token resource.
		_, err = authServer.Auth().CreateScopedToken(t.Context(), joiningv1.CreateScopedTokenRequest_builder{
			Token: scopedToken,
		}.Build())
		require.NoError(t, err)

		// Join the bot by referring to the scoped token by scope and name.
		result, err := joinclient.Join(t.Context(), joinclient.JoinParams{
			Token:      apiscopes.QualifiedName{Scope: scopedToken.GetScope(), Name: scopedToken.GetMetadata().GetName()}.String(),
			JoinMethod: types.JoinMethodGitLab,
			ID: state.IdentityID{
				Role: types.RoleBot,
			},
			IDToken:    validIDToken,
			AuthClient: nopClient,
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Certs)

		// Parse the TLS certificate and verify bot identity.
		cert, err := tlsca.ParseCertificatePEM(result.Certs.TLS)
		require.NoError(t, err)
		identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
		require.NoError(t, err)

		require.Equal(t, "gitlabci-bot", identity.BotName)
		require.NotEmpty(t, identity.BotInstanceID)
		require.NotNil(t, identity.ScopePin)
		require.Equal(t, "/test", identity.ScopePin.GetScope())
		require.Equal(t, "/test", identity.BotScope)
		require.True(t, identity.BotInternal)

		// Bot results should not contain immutable labels (host-only).
		require.Nil(t, result.ImmutableLabels)
	})

	t.Run("bot is not allowed to join when claims do not match allow rules", func(t *testing.T) {
		// Create a token spec that will not match the allow rules.
		tokenSpec := types.ProvisionTokenSpecV2{
			JoinMethod: types.JoinMethodGitLab,
			Roles:      []types.SystemRole{types.RoleBot},
			GitLab: &types.ProvisionTokenSpecV2GitLab{
				Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
					{
						Sub:         "other-org",
						ProjectPath: "other-org/other-repo",
					},
				},
			},
		}

		// Convert the token spec into a scoped token.
		nonMatchingToken, err := jointest.ScopedTokenFromProvisionTokenSpec(tokenSpec, joiningv1.ScopedToken_builder{
			Scope: "/test",
			Metadata: headerv1.Metadata_builder{
				Name: "gitlabci-bot-token-no-match",
			}.Build(),
			Spec: joiningv1.ScopedTokenSpec_builder{
				UsageMode: joining.TokenUsageModeBot,
				Bot:       CreateScopedBot(t, authServer.Auth(), "gitlabci-bot-no-match"),
			}.Build(),
		}.Build())
		require.NoError(t, err)

		// Create a scoped token resource.
		_, err = authServer.Auth().CreateScopedToken(t.Context(), joiningv1.CreateScopedTokenRequest_builder{
			Token: nonMatchingToken,
		}.Build())
		require.NoError(t, err)

		// Join the bot by referring to the scoped token by scope and name.
		_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
			Token:      apiscopes.QualifiedName{Scope: nonMatchingToken.GetScope(), Name: nonMatchingToken.GetMetadata().GetName()}.String(),
			JoinMethod: types.JoinMethodGitLab,
			ID: state.IdentityID{
				Role: types.RoleBot,
			},
			IDToken:    validIDToken,
			AuthClient: nopClient,
		})

		require.ErrorContains(t, err, "id token claims did not match any allow rules")
		require.True(t, trace.IsAccessDenied(err))
	})
}
