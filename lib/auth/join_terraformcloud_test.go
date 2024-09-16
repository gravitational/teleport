/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/terraformcloud"
)

type mockTerraformTokenValidator struct {
	tokens map[string]terraformcloud.IDTokenClaims
}

func (m *mockTerraformTokenValidator) Validate(
	_ context.Context, audience, hostname, token string,
) (*terraformcloud.IDTokenClaims, error) {
	if audience != "test.localhost" {
		return nil, fmt.Errorf("bad audience: %s", audience)
	}

	// Hostname override always fails, but that's okay: we're just making sure
	// the value gets plumbed through to here.
	if hostname != "" {
		return nil, fmt.Errorf("bad issuer: %s", hostname)
	}

	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func TestAuth_RegisterUsingToken_Terraform(t *testing.T) {
	validIDToken := "test.fake.jwt"
	idTokenValidator := &mockTerraformTokenValidator{
		tokens: map[string]terraformcloud.IDTokenClaims{
			validIDToken: {
				Sub:              "organization:example-organization:project:example-project:workspace:example-workspace:run_phase:apply",
				OrganizationName: "example-organization",
				OrganizationID:   "example-organization-id",
				ProjectName:      "example-project",
				ProjectID:        "example-project-id",
				WorkspaceName:    "example-workspace",
				WorkspaceID:      "example-workspace-id",
				RunPhase:         "apply",
			},
		},
	}
	var withTokenValidator ServerOption = func(server *Server) error {
		server.terraformIDTokenValidator = idTokenValidator
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

	allowRule := func(modifier func(*types.ProvisionTokenSpecV2TerraformCloud_Rule)) *types.ProvisionTokenSpecV2TerraformCloud_Rule {
		rule := &types.ProvisionTokenSpecV2TerraformCloud_Rule{
			OrganizationName: "example-organization",
			OrganizationID:   "example-organization-id",
			ProjectName:      "example-project",
			ProjectID:        "example-project-id",
			WorkspaceName:    "example-workspace",
			WorkspaceID:      "example-workspace-id",
			RunPhase:         "apply",
		}

		if modifier != nil {
			modifier(rule)
		}

		return rule
	}

	allowRulesNotMatched := require.ErrorAssertionFunc(func(t require.TestingT, err error, i ...interface{}) {
		require.ErrorContains(t, err, "id token claims did not match any allow rules")
		require.True(t, trace.IsAccessDenied(err))
	})
	tests := []struct {
		name          string
		setEnterprise bool
		request       *types.RegisterUsingTokenRequest
		tokenSpec     types.ProvisionTokenSpecV2
		assertError   require.ErrorAssertionFunc
	}{
		{
			name:          "success with all attributes",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "missing enterprise",
			setEnterprise: false,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationName: "example-organization",
							ProjectName:      "example-project",
						},
					},
					Hostname: "terraform.example.com",
				},
			},
			request: newRequest(validIDToken),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "requires Teleport Enterprise")
			},
		},
		{
			name:          "multiple allow rules",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationName: "other-organization",
							ProjectName:      "other-project",
						},
						{
							OrganizationName: "another-organization",
							ProjectName:      "example-project",
						},
						{
							OrganizationName: "example-organization",
							WorkspaceID:      "example-workspace-id",
						},
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "incorrect organization id",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2TerraformCloud_Rule) {
							rule.OrganizationID = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect organization name",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2TerraformCloud_Rule) {
							rule.OrganizationName = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect project name",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2TerraformCloud_Rule) {
							rule.ProjectName = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect project id",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2TerraformCloud_Rule) {
							rule.ProjectID = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect workspace name",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2TerraformCloud_Rule) {
							rule.WorkspaceName = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect workspace id",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2TerraformCloud_Rule) {
							rule.WorkspaceID = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect run_phase",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationID: "example-organization-id",
							WorkspaceID:    "example-workspace-id",
							RunPhase:       "plan",
						},
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "invalid token",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationID: "example-organization-id",
							WorkspaceID:    "example-workspace-id",
						},
					},
				},
			},
			request: newRequest("some other token"),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, errMockInvalidToken)
			},
		},
		{
			name:          "correct explicit audience",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationID: "example-organization-id",
							WorkspaceID:    "example-workspace-id",
						},
					},
					Audience: "test.localhost",
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "incorrect audience",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationID: "example-organization-id",
							WorkspaceID:    "example-workspace-id",
						},
					},
					Audience: "some-other-audience",
				},
			},
			request: newRequest(validIDToken),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bad audience")
			},
		},
		{
			name:          "overridden hostname is honored",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodTerraformCloud,
				Roles:      []types.SystemRole{types.RoleNode},
				TerraformCloud: &types.ProvisionTokenSpecV2TerraformCloud{
					Allow: []*types.ProvisionTokenSpecV2TerraformCloud_Rule{
						{
							OrganizationID: "example-organization-id",
							WorkspaceID:    "example-workspace-id",
						},
					},
					Hostname: "example.com",
				},
			},
			request: newRequest(validIDToken),
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "bad issuer: example.com")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		})
	}
}
