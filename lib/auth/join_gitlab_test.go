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

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/gitlab"
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

func TestAuth_RegisterUsingToken_GitLab(t *testing.T) {
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
	var withTokenValidator ServerOption = func(server *Server) error {
		server.gitlabIDTokenValidator = idTokenValidator
		return nil
	}
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), withTokenValidator)
	require.NoError(t, err)
	auth := p.a

	// helper for creating RegisterUsingTokenRequest
	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
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
		name        string
		request     *types.RegisterUsingTokenRequest
		tokenSpec   types.ProvisionTokenSpecV2
		assertError require.ErrorAssertionFunc
	}{
		{
			name: "success",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "domain override",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Domain: "gitlab.example.com",
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "multiple allow rules",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.Sub = "not matching"
						}),
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "incorrect sub",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.Sub = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "globby project path match",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.ProjectPath = "octo-org/octo-group/*"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "globby project path mismatch",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.ProjectPath = "octo-org/different-octo-group/*"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect project path",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.ProjectPath = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect namespace path",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.NamespacePath = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect pipeline source",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.PipelineSource = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect environment",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.Environment = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect ref",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.Ref = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect ref type",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.RefType = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect user_login",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.UserLogin = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect user_id",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.UserID = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect user_email",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.UserEmail = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect ref_protected",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.RefProtected = &types.BoolOption{
								Value: false,
							}
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "ref_protected ignored if nil",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.RefProtected = nil
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "incorrect environment_protected",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.EnvironmentProtected = &types.BoolOption{
								Value: true,
							}
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect ci_config_sha",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.CIConfigSHA = "not match"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect ci_config_ref_uri",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.CIConfigRefURI = "not match"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect deployment_tier",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.DeploymentTier = "not match"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect project_visibility",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitLab_Rule) {
							rule.ProjectVisibility = "not match"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "success with JWKS",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(nil),
					},
					StaticJWKS: "xyzzy",
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "failure with JWKS",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitLab,
				Roles:      []types.SystemRole{types.RoleNode},
				GitLab: &types.ProvisionTokenSpecV2GitLab{
					Allow: []*types.ProvisionTokenSpecV2GitLab_Rule{
						allowRule(nil),
					},
					StaticJWKS: "xyzzy",
				},
			},
			request:     newRequest("invalidjwt"),
			assertError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpec(
				tt.name, time.Now().Add(time.Minute), tt.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.CreateToken(ctx, token))
			tt.request.Token = tt.name

			_, err = auth.RegisterUsingToken(ctx, tt.request)
			tt.assertError(t, err)

			if tt.tokenSpec.GitLab.Domain != "" {
				require.Equal(
					t,
					tt.tokenSpec.GitLab.Domain,
					idTokenValidator.lastCalledDomain,
				)
			}
			if tt.tokenSpec.GitLab.StaticJWKS != "" {
				require.Equal(
					t,
					[]byte(tt.tokenSpec.GitLab.StaticJWKS),
					idTokenValidator.lastCalledJWKS,
				)
			} else {
				require.Nil(t, idTokenValidator.lastCalledJWKS)
			}
		})
	}
}

func Test_joinRuleGlobMatch(t *testing.T) {
	tests := []struct {
		name string

		rule  string
		claim string

		want bool
	}{
		{
			name:  "no rule",
			rule:  "",
			claim: "foo",
			want:  true,
		},
		{
			name:  "non-globby rule matches",
			rule:  "foo",
			claim: "foo",
			want:  true,
		},
		{
			name:  "globby rule matches",
			rule:  "?est-*-foo",
			claim: "test-string-foo",
			want:  true,
		},
		{
			name:  "globby rule mismatch",
			rule:  "?est-*-foo",
			claim: "ttest-bar-ffoo",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := joinRuleGlobMatch(tt.rule, tt.claim)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
