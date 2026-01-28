/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/join/azuredevops"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/join/jointest"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

type mockAzureDevopsTokenValidator struct {
	tokens map[string]map[string]azuredevops.IDTokenClaims
}

func (m *mockAzureDevopsTokenValidator) Validate(
	_ context.Context, organizationID string, token string,
) (*azuredevops.IDTokenClaims, error) {
	orgTokens := m.tokens[organizationID]
	if orgTokens == nil {
		return nil, fmt.Errorf("bad organization ID: %s", organizationID)
	}
	claims, ok := orgTokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func TestJoinAzureDevops(t *testing.T) {
	const (
		validIDToken          = "test.fake.jwt"
		validOrgID            = "0000-0000-0000-1337"
		fakeSub               = "p://my-org/my-project/my-pipeline"
		fakeOrgName           = "my-org"
		fakeProjectName       = "my-project"
		fakePipelineName      = "my-pipeline"
		fakeProjectID         = "711ef6f7-0000-0000-0000-4b54d912999"
		fakeDefinitionID      = "1"
		fakeRepositoryURI     = "https://github.com/gravitational/teleport.git"
		fakeRepositoryVersion = "e6b9eb29a288b27a3a82cc19c48b9d94b80aff36"
		fakeRepositoryRef     = "refs/heads/main"
	)

	idTokenValidator := &mockAzureDevopsTokenValidator{
		tokens: map[string]map[string]azuredevops.IDTokenClaims{
			validOrgID: {
				validIDToken: {
					Sub:               fakeSub,
					ProjectName:       fakeProjectName,
					OrganizationName:  fakeOrgName,
					ProjectID:         fakeProjectID,
					DefinitionID:      fakeDefinitionID,
					RepositoryURI:     fakeRepositoryURI,
					RepositoryVersion: fakeRepositoryVersion,
					RepositoryRef:     fakeRepositoryRef,
					PipelineName:      fakePipelineName,
				},
			},
		},
	}

	ctx := t.Context()
	server, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	auth := server.Auth()
	auth.SetAzureDevopsIDTokenValidator(idTokenValidator)

	nopClient, err := server.NewClient(authtest.TestNop())
	require.NoError(t, err)

	allowRule := func(modifier func(devops *types.ProvisionTokenSpecV2AzureDevops_Rule)) *types.ProvisionTokenSpecV2AzureDevops_Rule {
		rule := &types.ProvisionTokenSpecV2AzureDevops_Rule{
			Sub:               fakeSub,
			ProjectName:       fakeProjectName,
			PipelineName:      fakePipelineName,
			ProjectID:         fakeProjectID,
			DefinitionID:      fakeDefinitionID,
			RepositoryURI:     fakeRepositoryURI,
			RepositoryVersion: fakeRepositoryVersion,
			RepositoryRef:     fakeRepositoryRef,
		}
		if modifier != nil {
			modifier(rule)
		}
		return rule
	}

	allowRulesNotMatched := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...any) {
		require.ErrorContains(t, err, "id token claims failed to match any allow rules")
		require.True(t, trace.IsAccessDenied(err))
	})

	tests := []struct {
		name        string
		idToken     string
		tokenSpec   types.ProvisionTokenSpecV2
		assertError require.ErrorAssertionFunc
	}{
		{
			name: "success",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(nil),
					},
				},
			},
			idToken:     validIDToken,
			assertError: require.NoError,
		},
		{
			name: "multiple allow rules",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.Sub = "no-match"
						}),
						allowRule(nil),
					},
				},
			},
			idToken:     validIDToken,
			assertError: require.NoError,
		},
		{
			name: "incorrect sub",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.Sub = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect project_name",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.ProjectName = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect pipeline name",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.PipelineName = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect project id",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.ProjectID = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect definition id",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.DefinitionID = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect repository uri",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.RepositoryURI = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect repository version",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.RepositoryVersion = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "incorrect repository ref",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2AzureDevops_Rule) {
							rule.RepositoryRef = "wrong"
						}),
					},
				},
			},
			idToken:     validIDToken,
			assertError: allowRulesNotMatched,
		},
		{
			name: "invalid token",
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
						allowRule(nil),
					},
				},
			},
			idToken: "some other token",
			assertError: func(t require.TestingT, err error, i ...any) {
				// The error identity is lost at the gRPC boundary, preventing
				// ErrorIs, but we can check the string.
				require.Error(t, err)
				require.ErrorContains(t, err, errMockInvalidToken.Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := types.NewProvisionTokenFromSpec(
				"testtoken", time.Now().Add(time.Minute), tt.tokenSpec,
			)
			require.NoError(t, err)
			require.NoError(t, auth.UpsertToken(ctx, token))

			scopedToken, err := jointest.ScopedTokenFromProvisionToken(token, &joiningv1.ScopedToken{
				Scope: "/test",
				Metadata: &headerv1.Metadata{
					Name: "scoped_" + token.GetName(),
				},
				Spec: &joiningv1.ScopedTokenSpec{
					AssignedScope: "/test/one",
					UsageMode:     string(joining.TokenUsageModeUnlimited),
				},
			})
			require.NoError(t, err)

			_, err = auth.CreateScopedToken(t.Context(), &joiningv1.CreateScopedTokenRequest{
				Token: scopedToken,
			})
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := auth.DeleteScopedToken(ctx, &joiningv1.DeleteScopedTokenRequest{
					Name: scopedToken.GetMetadata().GetName(),
				})
				require.NoError(t, err)
			})

			t.Run("new", func(t *testing.T) {
				_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: token.GetName(),
					ID: state.IdentityID{
						Role:     types.RoleInstance,
						NodeName: "testnode",
					},
					IDToken:    tt.idToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
			})
			t.Run("scoped", func(t *testing.T) {
				_, err = joinclient.Join(t.Context(), joinclient.JoinParams{
					Token: "scoped_" + token.GetName(),
					ID: state.IdentityID{
						Role:     types.RoleInstance,
						NodeName: "testnode",
					},
					IDToken:    tt.idToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
			})
			t.Run("legacy", func(t *testing.T) {
				_, err = joinclient.LegacyJoin(t.Context(), joinclient.JoinParams{
					Token:      token.GetName(),
					JoinMethod: types.JoinMethodAzureDevops,
					ID: state.IdentityID{
						Role:     types.RoleInstance,
						HostUUID: "testuuid",
						NodeName: "testnode",
					},
					IDToken:    tt.idToken,
					AuthClient: nopClient,
				})
				tt.assertError(t, err)
			})
		})
	}
}
