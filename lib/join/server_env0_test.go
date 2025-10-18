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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/join/env0"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

type mockEnv0Validator struct {
	claims  *env0.IDTokenClaims
	idToken string
}

func (m *mockEnv0Validator) ValidateToken(ctx context.Context, token []byte) (*env0.IDTokenClaims, error) {
	if string(token) == m.idToken {
		return m.claims, nil
	}

	return nil, trace.AccessDenied("invalid token")
}

type env0JoinTestCase struct {
	desc             string
	authServer       *authtest.Server
	tokenName        string
	requestTokenName string
	tokenSpec        types.ProvisionTokenSpecV2
	oidcToken        string
	validator        *mockEnv0Validator
	assertError      require.ErrorAssertionFunc
}

func TestJoinEnv0(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	regularServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, regularServer.Shutdown(ctx)) })

	fipsServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:  t.TempDir(),
			FIPS: true,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, regularServer.Shutdown(ctx)) })

	// Define some fake names and IDs the fake "provider" will issue.
	const (
		defaultSubject         = "subject"
		defaultOrganizationID  = "organization-id"
		defaultProjectID       = "project-id"
		defaultProjectName     = "project-name"
		defaultTemplateID      = "template-id"
		defaultTemplateName    = "template-name"
		defaultEnvironmentID   = "environment-id "
		defaultEnvironmentName = "environment-name"
		defaultWorkspaceName   = "workspace-name"
		defaultDeploymentType  = "deployment-type"
		defaultDeployerEmail   = "deployer-email"
		defaultCustomTag       = "custom-tag"
	)
	validator := func(expectToken string, mutator ...func(*env0.IDTokenClaims)) *mockEnv0Validator {
		claims := &env0.IDTokenClaims{
			TokenClaims: oidc.TokenClaims{
				Subject: defaultSubject,
			},
			OrganizationID:  defaultOrganizationID,
			ProjectID:       defaultProjectID,
			ProjectName:     defaultProjectName,
			TemplateID:      defaultTemplateID,
			TemplateName:    defaultTemplateName,
			EnvironmentID:   defaultEnvironmentID,
			EnvironmentName: defaultEnvironmentName,
			WorkspaceName:   defaultWorkspaceName,
			DeploymentType:  defaultDeploymentType,
			DeployerEmail:   defaultDeployerEmail,
			Env0Tag:         defaultCustomTag,
			DeploymentLogID: "foo",
		}
		for _, m := range mutator {
			m(claims)
		}

		return &mockEnv0Validator{
			idToken: expectToken,
			claims:  claims,
		}
	}

	isAccessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	// isBadParameter := func(t require.TestingT, err error, _ ...any) {
	// 	require.True(t, trace.IsBadParameter(err), "expected Bad Parameter error, actual error: %v", err)
	// }

	testCases := []env0JoinTestCase{
		{
			desc:             "basic passing case",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
						},
					},
				},
			},
			assertError: require.NoError,
		},
		{
			desc:             "passes with all rules configured",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							// Include a rule that won't match for good measure.
							OrganizationID: "nope",
							ProjectName:    "also nope",
						},
						{
							OrganizationID:  defaultOrganizationID,
							ProjectID:       defaultProjectID,
							ProjectName:     defaultProjectName,
							TemplateID:      defaultTemplateID,
							TemplateName:    defaultTemplateName,
							EnvironmentID:   defaultEnvironmentID,
							EnvironmentName: defaultEnvironmentName,
							WorkspaceName:   defaultWorkspaceName,
							DeploymentType:  defaultDeploymentType,
							DeployerEmail:   defaultDeployerEmail,
							Env0Tag:         defaultCustomTag,
						},
					},
				},
			},
			assertError: require.NoError,
		},
		{
			desc:             "basic fips passing case",
			authServer:       fipsServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
						},
					},
				},
			},
			assertError: require.NoError,
		},
		{
			desc:             "requested wrong token",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "wrong-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "oidc token fails validation",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "invalid-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong organization",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: "other-org-id",
							ProjectID:      defaultProjectID,
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong project id",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      "other-project-id",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong project name",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							ProjectName:    "other-project-name",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong template id",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							TemplateID:     "other-template-id",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong template name",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							TemplateName:   "other-template-name",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong environment id",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							EnvironmentID:  "other-environment-id",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong environment name",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID:  defaultOrganizationID,
							ProjectID:       defaultProjectID,
							EnvironmentName: "other-environment-name",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong workspace name",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							WorkspaceName:  "other-workspace-name",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong deployment type",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							DeploymentType: "other-deployment-type",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong deployer email",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							DeployerEmail:  "other-deployer-email",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong custom tag",
			authServer:       regularServer,
			tokenName:        "test-token",
			requestTokenName: "test-token",
			oidcToken:        "correct-token",
			validator:        validator("correct-token"),
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles:      []types.SystemRole{types.RoleNode},
				JoinMethod: types.JoinMethodEnv0,
				Env0: &types.ProvisionTokenSpecV2Env0{
					Allow: []*types.ProvisionTokenSpecV2Env0_Rule{
						{
							OrganizationID: defaultOrganizationID,
							ProjectID:      defaultProjectID,
							Env0Tag:        "other-custom-tag",
						},
					},
				},
			},
			assertError: isAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			testEnv0Join(t, &tc)
		})
	}
}

func testEnv0Join(t *testing.T, tc *env0JoinTestCase) {
	ctx := t.Context()
	// Set mock client.
	tc.authServer.Auth().SetEnv0IDTokenValidator(tc.validator)

	// Add token to auth server.
	token, err := types.NewProvisionTokenFromSpec(
		tc.tokenName,
		time.Now().Add(time.Minute),
		tc.tokenSpec)
	require.NoError(t, err)
	require.NoError(t, tc.authServer.Auth().UpsertToken(ctx, token))
	t.Cleanup(func() {
		assert.NoError(t, tc.authServer.Auth().DeleteToken(ctx, token.GetName()))
	})

	// Make an unauthenticated auth client that will be used for the join.
	nopClient, err := tc.authServer.NewClient(authtest.TestNop())
	require.NoError(t, err)
	defer nopClient.Close()

	// Tests joining via the new join service with auth-assigned host UUIDs.
	_, err = joinclient.Join(ctx, joinclient.JoinParams{
		Token: tc.requestTokenName,
		ID: state.IdentityID{
			Role:     types.RoleInstance,
			NodeName: "test-node",
		},
		IDToken:    tc.oidcToken,
		AuthClient: nopClient,
	})
	tc.assertError(t, err)
}
