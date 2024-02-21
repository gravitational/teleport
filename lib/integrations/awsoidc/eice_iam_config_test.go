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
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/stretchr/testify/require"
)

func TestEICEIAMConfigReqDefaults(t *testing.T) {
	baseReq := func() EICEIAMConfigureRequest {
		return EICEIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
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
				Region:                    "us-east-1",
				IntegrationRole:           "integrationrole",
				IntegrationRoleEICEPolicy: "EC2InstanceConnectEndpoint",
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
		}
	}

	for _, tt := range []struct {
		name              string
		mockExistingRoles []string
		req               func() EICEIAMConfigureRequest
		errCheck          require.ErrorAssertionFunc
	}{
		{
			name:              "valid",
			req:               baseReq,
			mockExistingRoles: []string{"integrationrole"},
			errCheck:          require.NoError,
		},
		{
			name:              "integration role does not exist",
			mockExistingRoles: []string{},
			req:               baseReq,
			errCheck:          notFoundCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockEICEIAMConfigClient{
				existingRoles: tt.mockExistingRoles,
			}

			err := ConfigureEICEIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

type mockEICEIAMConfigClient struct {
	existingRoles []string
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockEICEIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamTypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
