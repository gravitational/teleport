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

func TestAWSAppAccessConfigReqDefaults(t *testing.T) {
	baseReq := func() AWSAppAccessConfigureRequest {
		return AWSAppAccessConfigureRequest{
			IntegrationRole: "integrationrole",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name     string
		req      func() AWSAppAccessConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected AWSAppAccessConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseReq,
			errCheck: require.NoError,
			expected: AWSAppAccessConfigureRequest{
				AccountID:                         "123456789012",
				IntegrationRole:                   "integrationrole",
				IntegrationRoleAWSAppAccessPolicy: "AWSAppAccess",
				AutoConfirm:                       true,
			},
		},
		{
			name: "missing integration role",
			req: func() AWSAppAccessConfigureRequest {
				req := baseReq()
				req.IntegrationRole = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing account id is ok",
			req: func() AWSAppAccessConfigureRequest {
				req := baseReq()
				req.AccountID = ""
				return req
			},
			expected: AWSAppAccessConfigureRequest{
				IntegrationRole:                   "integrationrole",
				IntegrationRoleAWSAppAccessPolicy: "AWSAppAccess",
				AutoConfirm:                       true,
			},
			errCheck: require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req()
			err := req.CheckAndSetDefaults()
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected, req)
		})
	}
}

func TestAWSAppAccessConfig(t *testing.T) {
	ctx := context.Background()
	baseReq := func() AWSAppAccessConfigureRequest {
		return AWSAppAccessConfigureRequest{
			IntegrationRole: "integrationrole",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name              string
		mockAccountID     string
		mockExistingRoles []string
		req               func() AWSAppAccessConfigureRequest
		errCheck          require.ErrorAssertionFunc
	}{
		{
			name:              "valid",
			req:               baseReq,
			mockAccountID:     "123456789012",
			mockExistingRoles: []string{"integrationrole"},
			errCheck:          require.NoError,
		},
		{
			name:              "integration role does not exist",
			mockAccountID:     "123456789012",
			mockExistingRoles: []string{},
			req:               baseReq,
			errCheck:          notFoundCheck,
		},
		{
			name:              "account does not match expected account",
			req:               baseReq,
			mockAccountID:     "222222222222",
			mockExistingRoles: []string{"integrationrole"},
			errCheck:          badParameterCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			awsClient := &mockAWSAppAccessConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureAWSAppAccess(ctx, awsClient, tt.req())
			tt.errCheck(t, err)
		})
	}
}

func TestAWSAppAccessConfigOutput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := AWSAppAccessConfigureRequest{
		IntegrationRole: "integrationrole",
		AccountID:       "123456789012",
		AutoConfirm:     true,
		stdout:          &buf,
	}
	awsClient := &mockAWSAppAccessConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: req.AccountID},
		existingRoles:        []string{req.IntegrationRole},
	}

	require.NoError(t, ConfigureAWSAppAccess(ctx, awsClient, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockAWSAppAccessConfigClient struct {
	CallerIdentityGetter
	existingRoles []string
}

func (m *mockAWSAppAccessConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
