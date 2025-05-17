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

package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/azuredevops"
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

func TestAuth_RegisterUsingToken_AzureDevops(t *testing.T) {
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

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), func(server *Server) error {
		server.azureDevopsIDTokenValidator = idTokenValidator
		return nil
	})
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

	allowRulesNotMatched := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...interface{}) {
		require.ErrorContains(t, err, "id token claims failed to match any allow rules")
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
				JoinMethod: types.JoinMethodAzureDevops,
				Roles:      []types.SystemRole{types.RoleNode},
				AzureDevops: &types.ProvisionTokenSpecV2AzureDevops{
					OrganizationID: validOrgID,
					Allow: []*types.ProvisionTokenSpecV2AzureDevops_Rule{
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request:     newRequest(validIDToken),
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
			request: newRequest("some other token"),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, errMockInvalidToken)
			},
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
		})
	}
}
