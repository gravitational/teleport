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
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/bitbucket"
	"github.com/gravitational/teleport/lib/modules"
)

const fakeBitbucketIDPURL = "https://api.bitbucket.org/2.0/workspaces/example/pipelines-config/identity/oidc"

type mockBitbucketTokenValidator struct {
	tokens map[string]bitbucket.IDTokenClaims
}

func (m *mockBitbucketTokenValidator) Validate(
	_ context.Context, issuerURL, audience, token string,
) (*bitbucket.IDTokenClaims, error) {
	if issuerURL != fakeBitbucketIDPURL {
		return nil, fmt.Errorf("bad issuer: %s", issuerURL)
	}

	claims, ok := m.tokens[token]
	if !ok {
		return nil, errMockInvalidToken
	}

	return &claims, nil
}

func TestAuth_RegisterUsingToken_Bitbucket(t *testing.T) {
	const (
		validIDToken          = "test.fake.jwt"
		fakeAudience          = "ari:cloud:bitbucket::workspace/e2b9cba7-4ecf-452c-bb1c-87add4084c99"
		fakeWorkspaceUUID     = "{e2b9cba7-4ecf-452c-bb1c-87add4084c99}"
		fakeRepositoryUUID    = "{8864cd24-449d-4441-8862-cf4fe8ad6caa}"
		fakeStepUUID          = "{d5ff2daf-063f-4c13-82fc-cb51af94c2cb}"
		fakePipelineUUID      = "{805b0ca6-1086-4d23-b526-de1d38cf36d7}"
		fakeDeploymentEnvUUID = "{c7ba7cd0-75e2-4616-8834-6d401da3ba0c}"
		fakeBranchName        = "main"
	)

	idTokenValidator := &mockBitbucketTokenValidator{
		tokens: map[string]bitbucket.IDTokenClaims{
			validIDToken: {
				BranchName:                fakeBranchName,
				WorkspaceUUID:             fakeWorkspaceUUID,
				RepositoryUUID:            fakeRepositoryUUID,
				StepUUID:                  fakeStepUUID,
				PipelineUUID:              fakePipelineUUID,
				DeploymentEnvironmentUUID: fakeDeploymentEnvUUID,
				Sub:                       fmt.Sprintf("%s:%s", fakeRepositoryUUID, fakeStepUUID),
			},
		},
	}

	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir(), func(server *Server) error {
		server.bitbucketIDTokenValidator = idTokenValidator
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

	allowRule := func(modifier func(*types.ProvisionTokenSpecV2Bitbucket_Rule)) *types.ProvisionTokenSpecV2Bitbucket_Rule {
		rule := &types.ProvisionTokenSpecV2Bitbucket_Rule{
			WorkspaceUUID:             fakeWorkspaceUUID,
			RepositoryUUID:            fakeRepositoryUUID,
			DeploymentEnvironmentUUID: fakeDeploymentEnvUUID,
			BranchName:                fakeBranchName,
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
			name:          "success",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "multiple allow rules",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Bitbucket_Rule) {
							rule.WorkspaceUUID = "{foo}"
							rule.RepositoryUUID = "{bar}"
						}),
						allowRule(nil),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: require.NoError,
		},
		{
			name:          "incorrect workspace uuid",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Bitbucket_Rule) {
							rule.WorkspaceUUID = "foo"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect repository uuid",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Bitbucket_Rule) {
							rule.RepositoryUUID = "bar"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect branch name",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Bitbucket_Rule) {
							rule.BranchName = "baz"
						}),
					},
				},
			},
			request:     newRequest(validIDToken),
			assertError: allowRulesNotMatched,
		},
		{
			name:          "incorrect deployment environment",
			setEnterprise: true,
			tokenSpec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
						allowRule(func(rule *types.ProvisionTokenSpecV2Bitbucket_Rule) {
							rule.DeploymentEnvironmentUUID = "{foo}"
						}),
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
				JoinMethod: types.JoinMethodBitbucket,
				Roles:      []types.SystemRole{types.RoleNode},
				Bitbucket: &types.ProvisionTokenSpecV2Bitbucket{
					IdentityProviderURL: fakeBitbucketIDPURL,
					Audience:            fakeAudience,
					Allow: []*types.ProvisionTokenSpecV2Bitbucket_Rule{
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
