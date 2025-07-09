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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

var badParameterCheck = func(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsBadParameter(err), `expected "bad parameter", but got %v`, err)
}

var notFoundCheck = func(t require.TestingT, err error, msgAndArgs ...interface{}) {
	require.True(t, trace.IsNotFound(err), `expected "not found", but got %v`, err)
}

var baseReq = func() DeployServiceIAMConfigureRequest {
	return DeployServiceIAMConfigureRequest{
		AccountID:       "123456789012",
		Cluster:         "mycluster",
		IntegrationName: "myintegration",
		Region:          "us-east-1",
		IntegrationRole: "integrationrole",
		TaskRole:        "taskrole",
		AutoConfirm:     true,
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
				AccountID:                          "123456789012",
				Cluster:                            "mycluster",
				IntegrationName:                    "myintegration",
				Region:                             "us-east-1",
				IntegrationRole:                    "integrationrole",
				TaskRole:                           "taskrole",
				partitionID:                        "aws",
				IntegrationRoleDeployServicePolicy: "DeployService",
				ResourceCreationTags: tags.AWSTags{
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "myintegration",
					"teleport.dev/origin":      "integration_awsoidc",
				},
				AutoConfirm: true,
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
		{
			name: "missing account id is ok",
			req: func() DeployServiceIAMConfigureRequest {
				req := baseReq()
				req.AccountID = ""
				return req
			},
			errCheck: require.NoError,
			expected: DeployServiceIAMConfigureRequest{
				Cluster:                            "mycluster",
				IntegrationName:                    "myintegration",
				Region:                             "us-east-1",
				IntegrationRole:                    "integrationrole",
				TaskRole:                           "taskrole",
				partitionID:                        "aws",
				IntegrationRoleDeployServicePolicy: "DeployService",
				ResourceCreationTags: tags.AWSTags{
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "myintegration",
					"teleport.dev/origin":      "integration_awsoidc",
				},
				AutoConfirm: true,
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

func TestDeployServiceIAMConfig(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct {
		name              string
		mockAccountID     string
		mockExistingRoles map[string]mockRole
		req               func() DeployServiceIAMConfigureRequest
		errCheck          require.ErrorAssertionFunc
	}{
		{
			name:              "valid",
			mockAccountID:     "123456789012",
			mockExistingRoles: map[string]mockRole{"integrationrole": {}},
			req:               baseReq,
			errCheck:          require.NoError,
		},
		{
			name:          "task role already exists",
			mockAccountID: "123456789012",
			mockExistingRoles: map[string]mockRole{
				"integrationrole": {},
				"taskrole": {
					assumeRolePolicyDoc: aws.String(`{"Version":"2012-10-17", "Statements":[]}`),
				},
			},
			req:      baseReq,
			errCheck: require.NoError,
		},
		{
			name:              "integration role does not exist",
			mockAccountID:     "123456789012",
			mockExistingRoles: map[string]mockRole{},
			req:               baseReq,
			errCheck:          notFoundCheck,
		},
		{
			name:              "account does not match expected account",
			mockAccountID:     "222222222222",
			mockExistingRoles: map[string]mockRole{"integrationrole": {}},
			req:               baseReq,
			errCheck:          badParameterCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockDeployServiceIAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureDeployServiceIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

func TestDeployServiceIAMConfigOutput(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	req := baseReq()
	req.stdout = &buf

	clt := mockDeployServiceIAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: req.AccountID},
		existingRoles:        map[string]mockRole{req.IntegrationRole: {}},
	}

	require.NoError(t, ConfigureDeployServiceIAM(ctx, &clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockDeployServiceIAMConfigClient struct {
	CallerIdentityGetter
	existingRoles map[string]mockRole
}

// CreateRole creates a new IAM Role.
func (m *mockDeployServiceIAMConfigClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	roleName := aws.ToString(params.RoleName)
	alreadyExistsMessage := fmt.Sprintf("Role %q already exists.", roleName)
	if _, found := m.existingRoles[roleName]; found {
		return nil, &iamtypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingRoles[roleName] = mockRole{
		assumeRolePolicyDoc: params.AssumeRolePolicyDocument,
		tags:                params.Tags,
	}

	return &iam.CreateRoleOutput{}, nil
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockDeployServiceIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	roleName := aws.ToString(params.RoleName)
	if _, found := m.existingRoles[roleName]; !found {
		noSuchEntityMessage := fmt.Sprintf("Role %q does not exist.", roleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}

func (m *mockDeployServiceIAMConfigClient) GetRole(ctx context.Context, params *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	roleName := aws.ToString(params.RoleName)
	role, found := m.existingRoles[roleName]
	if !found {
		return nil, trace.NotFound("role %q not found", roleName)
	}
	return &iam.GetRoleOutput{
		Role: &iamtypes.Role{
			AssumeRolePolicyDocument: role.assumeRolePolicyDoc,
		},
	}, nil
}

func (m *mockDeployServiceIAMConfigClient) UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error) {
	roleName := aws.ToString(params.RoleName)
	role, found := m.existingRoles[roleName]
	if !found {
		return nil, trace.NotFound("role not found")
	}

	role.assumeRolePolicyDoc = params.PolicyDocument
	m.existingRoles[roleName] = role

	return &iam.UpdateAssumeRolePolicyOutput{}, nil
}

func (m *mockDeployServiceIAMConfigClient) TagRole(ctx context.Context, params *iam.TagRoleInput, _ ...func(*iam.Options)) (*iam.TagRoleOutput, error) {
	roleName := aws.ToString(params.RoleName)
	role, found := m.existingRoles[roleName]
	if !found {
		return nil, trace.NotFound("role not found")
	}

Outer:
	for _, addTag := range params.Tags {
		for _, roleTag := range role.tags {
			if aws.ToString(roleTag.Key) == aws.ToString(addTag.Key) {
				roleTag.Value = addTag.Value
				continue Outer
			}
		}
		role.tags = append(role.tags, addTag)
	}
	return &iam.TagRoleOutput{}, nil
}

type mockSTSClient struct {
	accountID string
}

// GetCallerIdentity returns information about the caller identity.
func (m mockSTSClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: &m.accountID,
	}, nil
}
