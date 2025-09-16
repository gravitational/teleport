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

func TestEICEIAMConfigReqDefaults(t *testing.T) {
	baseReq := func() EICEIAMConfigureRequest {
		return EICEIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name     string
		req      func() EICEIAMConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected EICEIAMConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseReq,
			errCheck: require.NoError,
			expected: EICEIAMConfigureRequest{
				AccountID:                 "123456789012",
				Region:                    "us-east-1",
				IntegrationRole:           "integrationrole",
				IntegrationRoleEICEPolicy: "EC2InstanceConnectEndpoint",
				AutoConfirm:               true,
			},
		},
		{
			name: "missing region",
			req: func() EICEIAMConfigureRequest {
				req := baseReq()
				req.Region = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: func() EICEIAMConfigureRequest {
				req := baseReq()
				req.IntegrationRole = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing account id is ok",
			req: func() EICEIAMConfigureRequest {
				req := baseReq()
				req.AccountID = ""
				return req
			},
			errCheck: require.NoError,
			expected: EICEIAMConfigureRequest{
				Region:                    "us-east-1",
				IntegrationRole:           "integrationrole",
				IntegrationRoleEICEPolicy: "EC2InstanceConnectEndpoint",
				AutoConfirm:               true,
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

func TestEICEIAMConfig(t *testing.T) {
	ctx := context.Background()
	baseReq := func() EICEIAMConfigureRequest {
		return EICEIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name              string
		mockAccountID     string
		mockExistingRoles []string
		req               func() EICEIAMConfigureRequest
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
			clt := mockEICEIAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureEICEIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

func TestEICEIAMConfigOutput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := EICEIAMConfigureRequest{
		Region:          "us-east-1",
		IntegrationRole: "integrationrole",
		AccountID:       "123456789012",
		AutoConfirm:     true,
		stdout:          &buf,
	}

	clt := mockEICEIAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: req.AccountID},
		existingRoles:        []string{req.IntegrationRole},
	}

	require.NoError(t, ConfigureEICEIAM(ctx, &clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockEICEIAMConfigClient struct {
	CallerIdentityGetter
	existingRoles []string
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockEICEIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
