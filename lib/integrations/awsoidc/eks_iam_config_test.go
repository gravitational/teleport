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

package awsoidc

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestEKSIAMConfigReqDefaults(t *testing.T) {
	for _, tt := range []struct {
		name     string
		req      EKSIAMConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected EKSIAMConfigureRequest
	}{
		{
			name: "set defaults",
			req: EKSIAMConfigureRequest{
				AccountID:       "123456789012",
				Region:          "us-east-1",
				IntegrationRole: "integrationRole",
				AutoConfirm:     true,
			},
			errCheck: require.NoError,
			expected: EKSIAMConfigureRequest{
				AccountID:                "123456789012",
				Region:                   "us-east-1",
				IntegrationRole:          "integrationRole",
				IntegrationRoleEKSPolicy: "EKSAccess",
				AutoConfirm:              true,
			},
		},
		{
			name: "missing region",
			req: EKSIAMConfigureRequest{
				AccountID:       "123456789012",
				IntegrationRole: "integrationRole",
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: EKSIAMConfigureRequest{
				AccountID: "123456789012",
				Region:    "us-east-1",
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing account id is ok",
			req: EKSIAMConfigureRequest{
				IntegrationRole: "integrationRole",
				Region:          "us-east-1",
			},
			errCheck: require.NoError,
			expected: EKSIAMConfigureRequest{
				Region:                   "us-east-1",
				IntegrationRole:          "integrationRole",
				IntegrationRoleEKSPolicy: "EKSAccess",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req
			err := req.CheckAndSetDefaults()
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected, req)
		})
	}
}

func TestEKSIAMConfig(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name              string
		mockAccountID     string
		mockExistingRoles []string
		req               EKSIAMConfigureRequest
		errCheck          require.ErrorAssertionFunc
	}{
		{
			name: "valid",
			req: EKSIAMConfigureRequest{
				Region:          "us-east-1",
				IntegrationRole: "integrationRole",
				AccountID:       "123456789012",
				AutoConfirm:     true,
			},
			mockAccountID:     "123456789012",
			mockExistingRoles: []string{"integrationRole"},
			errCheck:          require.NoError,
		},
		{
			name:              "integration role does not exist",
			mockAccountID:     "123456789012",
			mockExistingRoles: []string{},
			req: EKSIAMConfigureRequest{
				Region:          "us-east-1",
				IntegrationRole: "integrationRole",
				AccountID:       "123456789012",
				AutoConfirm:     true,
			},
			errCheck: notFoundCheck,
		},
		{
			name: "account does not match expected account",
			req: EKSIAMConfigureRequest{
				Region:          "us-east-1",
				IntegrationRole: "integrationRole",
				AccountID:       "123456789012",
			},
			mockAccountID:     "222222222222",
			mockExistingRoles: []string{"integrationRole"},
			errCheck:          badParameterCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockEKSIAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureEKSIAM(ctx, &clt, tt.req)
			tt.errCheck(t, err)
		})
	}
}

func TestEKSIAMConfigOutput(t *testing.T) {
	var buf bytes.Buffer
	req := EKSIAMConfigureRequest{
		Region:          "us-east-1",
		IntegrationRole: "integrationRole",
		AccountID:       "123456789012",
		AutoConfirm:     true,
		stdout:          &buf,
	}
	clt := &mockEKSIAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: req.AccountID},
		existingRoles:        []string{req.IntegrationRole},
	}

	ctx := context.Background()
	require.NoError(t, ConfigureEKSIAM(ctx, clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockEKSIAMConfigClient struct {
	CallerIdentityGetter
	existingRoles []string
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockEKSIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
