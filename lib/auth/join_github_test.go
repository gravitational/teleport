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
	"errors"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/githubactions"
	"github.com/gravitational/teleport/lib/modules"
)

type mockIDTokenValidator struct {
	tokens                   map[string]githubactions.IDTokenClaims
	lastCalledGHESHost       string
	lastCalledEnterpriseSlug string
	lastCalledJWKS           string
}

var errMockInvalidToken = errors.New("invalid token")

func (m *mockIDTokenValidator) Validate(
	_ context.Context, ghes, enterpriseSlug, token string,
) (*githubactions.IDTokenClaims, error) {
	m.lastCalledGHESHost = ghes
	m.lastCalledEnterpriseSlug = enterpriseSlug
	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func (m *mockIDTokenValidator) reset() {
	m.lastCalledGHESHost = ""
	m.lastCalledEnterpriseSlug = ""
	m.lastCalledJWKS = ""
}

func (m *mockIDTokenValidator) ValidateJWKS(
	_ time.Time, jwks []byte, token string,
) (*githubactions.IDTokenClaims, error) {
	m.lastCalledJWKS = string(jwks)
	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}
	return &claims, nil
}

func TestAuth_RegisterUsingToken_GHA(t *testing.T) {
	validIDToken := "test.fake.jwt"
	idTokenValidator := &mockIDTokenValidator{
		tokens: map[string]githubactions.IDTokenClaims{
			validIDToken: {
				Sub:             "repo:octo-org/octo-repo:environment:prod",
				Repository:      "octo-org/octo-repo",
				RepositoryOwner: "octo-org",
				Workflow:        "example-workflow",
				Environment:     "prod",
				Actor:           "octocat",
				Ref:             "refs/heads/main",
				RefType:         "branch",
			},
		},
	}
	var withTokenValidator ServerOption = func(server *Server) error {
		server.ghaIDTokenValidator = idTokenValidator
		server.ghaIDTokenJWKSValidator = idTokenValidator.ValidateJWKS
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

	allowRule := func(modifier func(*types.ProvisionTokenSpecV2GitHub_Rule)) *types.ProvisionTokenSpecV2GitHub_Rule {
		rule := &types.ProvisionTokenSpecV2GitHub_Rule{
			Sub:             "repo:octo-org/octo-repo:environment:prod",
			Repository:      "octo-org/octo-repo",
			RepositoryOwner: "octo-org",
			Workflow:        "example-workflow",
			Environment:     "prod",
			Actor:           "octocat",
			Ref:             "refs/heads/main",
			RefType:         "branch",
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
		name          string
		request       *types.RegisterUsingTokenRequest
		tokenSpec     types.ProvisionTokenSpecV2
		assertError   require.ErrorAssertionFunc
		setEnterprise bool
	}{
		{
			name: "success",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "success with jwks",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
					StaticJWKS: "my-jwks",
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name: "failure with jwks",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
					StaticJWKS: "my-jwks",
				},
			},
			request:     newRequest("invalid"),
			assertError: require.Error,
		},
		{
			name: "ghes override",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					EnterpriseServerHost: "my.ghes.instance",
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
				},
			},
			request:       newRequest(validIDToken),
			assertError:   require.NoError,
			setEnterprise: true,
		},
		{
			name: "enterprise slug",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					EnterpriseSlug: "slug",
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
				},
			},
			setEnterprise: true,
			request:       newRequest(validIDToken),
			assertError:   require.NoError,
		},
		{
			name: "ghes override requires enterprise license",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					EnterpriseServerHost: "my.ghes.instance",
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
				},
			},
			request: newRequest(validIDToken),
			assertError: require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, ErrRequiresEnterprise)
			}),
		},
		{
			name: "enterprise slug requires enterprise license",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					EnterpriseSlug: "slug",
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(nil),
					},
				},
			},
			request: newRequest(validIDToken),
			assertError: require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, ErrRequiresEnterprise)
			}),
		},
		{
			name: "multiple allow rules",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
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
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.Sub = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect repository",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.Repository = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect repository owner",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.RepositoryOwner = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect workflow",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.Workflow = "not matching"
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
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.Environment = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect actor",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.Actor = "not matching"
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
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
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
				JoinMethod: types.JoinMethodGitHub,
				Roles:      []types.SystemRole{types.RoleNode},
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2GitHub_Rule) {
							rule.RefType = "not matching"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(idTokenValidator.reset)
			if tt.setEnterprise {
				modules.SetTestModules(
					t,
					&modules.TestModules{TestBuildType: modules.BuildEnterprise},
				)
			}
			token, err := types.NewProvisionTokenFromSpec(
				tt.name, time.Now().Add(time.Minute), tt.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.CreateToken(ctx, token))
			tt.request.Token = tt.name

			_, err = auth.RegisterUsingToken(ctx, tt.request)
			tt.assertError(t, err)
			if err != nil {
				return
			}

			require.Equal(
				t,
				tt.tokenSpec.GitHub.EnterpriseServerHost,
				idTokenValidator.lastCalledGHESHost,
			)
			require.Equal(
				t,
				tt.tokenSpec.GitHub.EnterpriseSlug,
				idTokenValidator.lastCalledEnterpriseSlug,
			)
			require.Equal(
				t,
				tt.tokenSpec.GitHub.StaticJWKS,
				idTokenValidator.lastCalledJWKS,
			)
		})
	}
}
