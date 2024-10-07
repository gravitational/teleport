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

package awsoidc

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/require"
)

func TestListDatabasesIAMConfigReqDefaults(t *testing.T) {
	for _, tt := range []struct {
		name     string
		req      ConfigureIAMListDatabasesRequest
		errCheck require.ErrorAssertionFunc
		expected ConfigureIAMListDatabasesRequest
	}{
		{
			name: "missing region",
			req: ConfigureIAMListDatabasesRequest{
				IntegrationRole: "integrationrole",
				AccountID:       "123456789012",
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: ConfigureIAMListDatabasesRequest{
				Region:    "us-east-1",
				AccountID: "123456789012",
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing account id is ok",
			req: ConfigureIAMListDatabasesRequest{
				Region:          "us-east-1",
				IntegrationRole: "integrationrole",
			},
			errCheck: require.NoError,
			expected: ConfigureIAMListDatabasesRequest{
				Region:          "us-east-1",
				IntegrationRole: "integrationrole",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.CheckAndSetDefaults()
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected, tt.req)
		})
	}
}

func TestListDatabasesIAMConfig(t *testing.T) {
	ctx := context.Background()
	baseReq := ConfigureIAMListDatabasesRequest{
		Region:          "us-east-1",
		IntegrationRole: "integrationrole",
		AccountID:       "123456789012",
	}

	for _, tt := range []struct {
		name              string
		mockExistingRoles []string
		mockAccountID     string
		req               ConfigureIAMListDatabasesRequest
		errCheck          require.ErrorAssertionFunc
	}{
		{
			name:              "valid",
			req:               baseReq,
			mockExistingRoles: []string{"integrationrole"},
			mockAccountID:     "123456789012",
			errCheck:          require.NoError,
		},
		{
			name:              "account does not match expected account",
			req:               baseReq,
			mockExistingRoles: []string{"integrationrole"},
			mockAccountID:     "222222222222",
			errCheck:          badParameterCheck,
		},
		{
			name:              "integration role does not exist",
			mockExistingRoles: []string{},
			mockAccountID:     "123456789012",
			req:               baseReq,
			errCheck:          notFoundCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockListDatabasesIAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureListDatabasesIAM(ctx, &clt, tt.req)
			tt.errCheck(t, err)
		})
	}
}

type mockListDatabasesIAMConfigClient struct {
	CallerIdentityGetter
	existingRoles []string
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockListDatabasesIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
