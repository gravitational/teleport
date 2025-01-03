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

func TestAccessGraphIAMConfigReqDefaults(t *testing.T) {
	baseReq := func() AccessGraphAWSIAMConfigureRequest {
		return AccessGraphAWSIAMConfigureRequest{
			IntegrationRole: "integrationrole",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name     string
		req      func() AccessGraphAWSIAMConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected AccessGraphAWSIAMConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseReq,
			errCheck: require.NoError,
			expected: AccessGraphAWSIAMConfigureRequest{
				IntegrationRole:          "integrationrole",
				IntegrationRoleTAGPolicy: "AccessGraphSyncAccess",
				AccountID:                "123456789012",
				AutoConfirm:              true,
			},
		},
		{
			name: "missing integration role",
			req: func() AccessGraphAWSIAMConfigureRequest {
				req := baseReq()
				req.IntegrationRole = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing account id is ok",
			req: func() AccessGraphAWSIAMConfigureRequest {
				req := baseReq()
				req.AccountID = ""
				return req
			},
			errCheck: require.NoError,
			expected: AccessGraphAWSIAMConfigureRequest{
				IntegrationRole:          "integrationrole",
				IntegrationRoleTAGPolicy: "AccessGraphSyncAccess",
				AutoConfirm:              true,
			},
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

func TestAccessGraphAWSIAMConfig(t *testing.T) {
	ctx := context.Background()
	baseReq := func() AccessGraphAWSIAMConfigureRequest {
		return AccessGraphAWSIAMConfigureRequest{
			IntegrationRole: "integrationrole",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name              string
		mockAccountID     string
		mockExistingRoles []string
		req               func() AccessGraphAWSIAMConfigureRequest
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
			clt := mockAccessGraphAWSAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureAccessGraphSyncIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

func TestAccessGraphAWSIAMConfigOuput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := AccessGraphAWSIAMConfigureRequest{
		IntegrationRole: "integrationrole",
		AccountID:       "123456789012",
		AutoConfirm:     true,
		stdout:          &buf,
	}
	clt := mockAccessGraphAWSAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: req.AccountID},
		existingRoles:        []string{req.IntegrationRole},
	}

	require.NoError(t, ConfigureAccessGraphSyncIAM(ctx, &clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockAccessGraphAWSAMConfigClient struct {
	CallerIdentityGetter
	existingRoles []string
}

func (m *mockAccessGraphAWSAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
