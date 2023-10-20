/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsoidc

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type mockGetUpsertToken struct {
	token types.ProvisionToken
}

// GetToken returns a provision token by name.
func (m *mockGetUpsertToken) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	if m.token != nil && name == m.token.GetName() {
		return m.token, nil
	}

	return nil, trace.NotFound("token not found")
}

// UpsertToken creates or updates a provision token.
func (m *mockGetUpsertToken) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	m.token = token

	return nil
}

func TestUpsertIAMJoinToken(t *testing.T) {
	ctx := context.Background()

	t.Run("when token doesnt exist, it is created", func(t *testing.T) {
		m := &mockGetUpsertToken{}

		err := upsertIAMJoinToken(ctx, upsertIAMJoinTokenRequest{
			tokenName:      "t",
			accountID:      "123456789012",
			region:         "us-east-1",
			iamRole:        "myrole",
			deploymentMode: DatabaseServiceDeploymentMode,
		}, m)
		require.NoError(t, err)

		iamToken, err := m.GetToken(ctx, "t")
		require.NoError(t, err)

		require.Equal(t, "t", iamToken.GetName())
		require.Contains(t, iamToken.GetRoles(), types.RoleDatabase)
		require.Len(t, iamToken.GetAllowRules(), 1)
		require.Equal(t, iamToken.GetAllowRules()[0], &types.TokenRule{
			AWSAccount: "123456789012",
			AWSARN:     "arn:aws:sts::123456789012:assumed-role/myrole/*",
		})
	})

	t.Run("when token exist but is missing the required allow rule and system role, it is updated", func(t *testing.T) {
		m := &mockGetUpsertToken{
			token: &types.ProvisionTokenV2{
				Metadata: types.Metadata{
					Name: "t",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodIAM,
					Roles:      []types.SystemRole{},
					Allow:      []*types.TokenRule{},
				},
			},
		}

		err := upsertIAMJoinToken(ctx, upsertIAMJoinTokenRequest{
			tokenName:      "t",
			accountID:      "123456789012",
			region:         "us-east-1",
			iamRole:        "myrole",
			deploymentMode: DatabaseServiceDeploymentMode,
		}, m)
		require.NoError(t, err)

		iamToken, err := m.GetToken(ctx, "t")
		require.NoError(t, err)

		require.Equal(t, "t", iamToken.GetName())
		require.Len(t, iamToken.GetAllowRules(), 1)
		require.Contains(t, iamToken.GetRoles(), types.RoleDatabase)
		require.Equal(t, iamToken.GetAllowRules()[0], &types.TokenRule{
			AWSAccount: "123456789012",
			AWSARN:     "arn:aws:sts::123456789012:assumed-role/myrole/*",
		})
	})

	t.Run("when token exist but has an invalid join method, it returns an error", func(t *testing.T) {
		m := &mockGetUpsertToken{
			token: &types.ProvisionTokenV2{
				Metadata: types.Metadata{
					Name: "t",
				},
				Spec: types.ProvisionTokenSpecV2{
					JoinMethod: types.JoinMethodEC2,
					Roles:      []types.SystemRole{},
					Allow:      []*types.TokenRule{},
				},
			},
		}

		err := upsertIAMJoinToken(ctx, upsertIAMJoinTokenRequest{
			tokenName:      "t",
			accountID:      "123456789012",
			region:         "us-east-1",
			iamRole:        "myrole",
			deploymentMode: DatabaseServiceDeploymentMode,
		}, m)
		require.ErrorContains(t, err, `Token "t" already exists but has the wrong join method "ec2". Please remove it before continuing.`)
	})

	t.Run("when deployment method is invalid, it returns an error", func(t *testing.T) {
		m := &mockGetUpsertToken{}

		err := upsertIAMJoinToken(ctx, upsertIAMJoinTokenRequest{
			tokenName:      "t",
			accountID:      "123456789012",
			region:         "us-east-1",
			iamRole:        "myrole",
			deploymentMode: "invalid-deploy-method",
		}, m)
		require.ErrorContains(t, err, "invalid deployment mode, please use one of the following: [database-service]")
	})
}
