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
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"
)

var badParameterCheck = func(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsBadParameter(err), `expected "bad parameter", but got %v`, err)
}

var alreadyExistsCheck = func(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsAlreadyExists(err), `expected "already exists", but got %v`, err)
}

var notFounCheck = func(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsNotFound(err), `expected "not found", but got %v`, err)
}

var baseReq = func() DeployServiceIAMConfigureRequest {
	return DeployServiceIAMConfigureRequest{
		Cluster:         "mycluster",
		IntegrationName: "myintegration",
		Region:          "us-east-1",
		IntegrationRole: "integrationrole",
		TaskRole:        "taskrole",
	}
}

func TestDeployServiceIAMConfigReqDefaults(t *testing.T) {
	for _, tt := range []struct {
		name     string
		req      func() DeployServiceIAMConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected DeployServiceIAMConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseReq,
			errCheck: require.NoError,
			expected: DeployServiceIAMConfigureRequest{
				Cluster:                            "mycluster",
				IntegrationName:                    "myintegration",
				Region:                             "us-east-1",
				IntegrationRole:                    "integrationrole",
				TaskRole:                           "taskrole",
				partitionID:                        "aws",
				IntegrationRoleDeployServicePolicy: "DeployService",
				TaskRoleBoundaryPolicyName:         "taskroleBoundary",
				ResourceCreationTags: awsTags{
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "myintegration",
					"teleport.dev/origin":      "integration_awsoidc",
				},
			},
		},
		{
			name: "missing cluster",
			req: func() DeployServiceIAMConfigureRequest {
				req := baseReq()
				req.Cluster = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration name",
			req: func() DeployServiceIAMConfigureRequest {
				req := baseReq()
				req.IntegrationName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing region",
			req: func() DeployServiceIAMConfigureRequest {
				req := baseReq()
				req.Region = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: func() DeployServiceIAMConfigureRequest {
				req := baseReq()
				req.IntegrationRole = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing task role",
			req: func() DeployServiceIAMConfigureRequest {
				req := baseReq()
				req.TaskRole = ""
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

func TestDeployServiceIAMConfig(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name                 string
		mockAccountID        string
		mockExistingPolicies []string
		mockExistingRoles    []string
		req                  func() DeployServiceIAMConfigureRequest
		errCheck             require.ErrorAssertionFunc
	}{
		{
			name:                 "valid",
			mockAccountID:        "123456789012",
			mockExistingPolicies: []string{},
			mockExistingRoles:    []string{"integrationrole"},
			req:                  baseReq,
			errCheck:             require.NoError,
		},
		{
			name:                 "boundary policy already exists",
			mockAccountID:        "123456789012",
			mockExistingPolicies: []string{"taskroleBoundary"},
			mockExistingRoles:    []string{"integrationrole"},
			req:                  baseReq,
			errCheck:             alreadyExistsCheck,
		},
		{
			name:                 "task role already exists",
			mockAccountID:        "123456789012",
			mockExistingPolicies: []string{},
			mockExistingRoles:    []string{"integrationrole", "taskrole"},
			req:                  baseReq,
			errCheck:             alreadyExistsCheck,
		},
		{
			name:                 "integration role does not exist",
			mockAccountID:        "123456789012",
			mockExistingPolicies: []string{},
			mockExistingRoles:    []string{},
			req:                  baseReq,
			errCheck:             notFounCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockDeployServiceIAMConfigClient{
				accountID:        tt.mockAccountID,
				existingPolicies: tt.mockExistingPolicies,
				existingRoles:    tt.mockExistingRoles,
			}

			err := ConfigureDeployServiceIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

type mockDeployServiceIAMConfigClient struct {
	accountID        string
	existingPolicies []string
	existingRoles    []string
}

// GetCallerIdentity returns information about the caller identity.
func (m *mockDeployServiceIAMConfigClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: &m.accountID,
	}, nil
}

// CreatePolicy creates a new IAM Policy.
func (m *mockDeployServiceIAMConfigClient) CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("Policy %q already exists.", *params.PolicyName)
	if slices.Contains(m.existingPolicies, *params.PolicyName) {
		return nil, &iamTypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}

	m.existingPolicies = append(m.existingPolicies, *params.PolicyName)
	return nil, nil
}

// CreateRole creates a new IAM Role.
func (m *mockDeployServiceIAMConfigClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("Role %q already exists.", *params.RoleName)
	if slices.Contains(m.existingRoles, *params.RoleName) {
		return nil, &iamTypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingRoles = append(m.existingRoles, *params.RoleName)

	return nil, nil
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockDeployServiceIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	noSuchEntityMessage := fmt.Sprintf("Role %q does not exist.", *params.RoleName)
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		return nil, &iamTypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}
