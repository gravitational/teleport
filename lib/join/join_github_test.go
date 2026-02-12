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
	"errors"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/join/githubactions"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
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

func checkMockGithubValidatorState(t *testing.T, validator *mockIDTokenValidator, spec types.ProvisionTokenSpecV2) {
	t.Helper()

	require.Equal(
		t,
		spec.GitHub.EnterpriseServerHost,
		validator.lastCalledGHESHost,
	)
	require.Equal(
		t,
		spec.GitHub.EnterpriseSlug,
		validator.lastCalledEnterpriseSlug,
	)
	require.Equal(
		t,
		spec.GitHub.StaticJWKS,
		validator.lastCalledJWKS,
	)
}

func TestJoinGHA(t *testing.T) {
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

	authServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, authServer.Shutdown(t.Context())) })

	authServer.Auth().SetGHAIDTokenValidator(idTokenValidator)
	authServer.Auth().SetGHAIDTokenJWKSValidator(idTokenValidator.ValidateJWKS)

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
			name: "success-with-jwks",
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
			name: "failure-with-jwks",
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
			name: "ghes-override",
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
			name: "enterprise-slug",
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
			name: "ghes-override-requires-enterprise-license",
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
				// Note: testing over the network does not perfectly map errors
				// so we can't use require.ErrorIs(..., services.ErrRequiresEnterprise)
				require.ErrorContains(t, err, "this feature requires Teleport Enterprise")
			}),
		},
		{
			name: "enterprise-slug-requires-enterprise-license",
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
				// Note: testing over the network does not perfectly map errors
				// so we can't use require.ErrorIs(..., services.ErrRequiresEnterprise)
				require.ErrorContains(t, err, "this feature requires Teleport Enterprise")
			}),
		},
		{
			name: "multiple-allow-rules",
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
			name: "incorrect-sub",
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
			name: "incorrect-repository",
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
			name: "incorrect-repository-owner",
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
			name: "incorrect-workflow",
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
			name: "incorrect-environment",
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
			name: "incorrect-actor",
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
			name: "incorrect-ref",
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
			name: "incorrect-ref-type",
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
				modulestest.SetTestModules(
					t,
					modulestest.Modules{TestBuildType: modules.BuildEnterprise},
				)
			}
			token, err := types.NewProvisionTokenFromSpec(
				tt.name, time.Now().Add(time.Minute), tt.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, authServer.Auth().CreateToken(t.Context(), token))
			tt.request.Token = tt.name

			nopClient, err := authServer.NewClient(authtest.TestNop())
			require.NoError(t, err)

			t.Run("legacy joinclient", func(t *testing.T) {
				_, err := joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      tt.request.Token,
					JoinMethod: types.JoinMethodGitHub,
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

				checkMockGithubValidatorState(t, idTokenValidator, tt.tokenSpec)
			})

			t.Run("new joinclient", func(t *testing.T) {
				_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
					Token:      tt.request.Token,
					JoinMethod: types.JoinMethodGitHub,
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

				checkMockGithubValidatorState(t, idTokenValidator, tt.tokenSpec)
			})

			t.Run("legacy", func(t *testing.T) {
				_, err = authServer.Auth().RegisterUsingToken(t.Context(), tt.request)
				tt.assertError(t, err)
				if err != nil {
					return
				}

				checkMockGithubValidatorState(t, idTokenValidator, tt.tokenSpec)
			})
		})
	}
}
