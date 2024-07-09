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
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
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
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: ConfigureIAMListDatabasesRequest{
				IntegrationRole: "integrationrole",
			},
			errCheck: badParameterCheck,
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
	}

	for _, tt := range []struct {
		name              string
		mockExistingRoles []string
		req               ConfigureIAMListDatabasesRequest
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
			clt := mockListDatabasesIAMConfigClient{
				existingRoles: tt.mockExistingRoles,
			}

			err := ConfigureListDatabasesIAM(ctx, &clt, tt.req)
			tt.errCheck(t, err)
		})
	}
}

type mockListDatabasesIAMConfigClient struct {
	existingRoles []string
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockListDatabasesIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamTypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
