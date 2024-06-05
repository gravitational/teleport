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
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"
)

func TestEC2SSMIAMConfigReqDefaults(t *testing.T) {
	baseReq := func() EC2SSMIAMConfigureRequest {
		return EC2SSMIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
			SSMDocumentName: "MyDoc",
			ProxyPublicURL:  "https://proxy.example.com",
		}
	}

	for _, tt := range []struct {
		name     string
		req      func() EC2SSMIAMConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected EC2SSMIAMConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseReq,
			errCheck: require.NoError,
			expected: EC2SSMIAMConfigureRequest{
				Region:                      "us-east-1",
				IntegrationRole:             "integrationrole",
				IntegrationRoleEC2SSMPolicy: "EC2DiscoverWithSSM",
				SSMDocumentName:             "MyDoc",
				ProxyPublicURL:              "https://proxy.example.com",
			},
		},
		{
			name: "missing region",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.Region = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.IntegrationRole = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing ssm document",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.SSMDocumentName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing proxy url",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.ProxyPublicURL = ""
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

func TestEC2SSMIAMConfig(t *testing.T) {
	ctx := context.Background()
	baseReq := func() EC2SSMIAMConfigureRequest {
		return EC2SSMIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
			SSMDocumentName: "MyDoc",
			ProxyPublicURL:  "https://proxy.example.com",
		}
	}

	for _, tt := range []struct {
		name                string
		mockExistingRoles   []string
		mockExistingSSMDocs []string
		req                 func() EC2SSMIAMConfigureRequest
		errCheck            require.ErrorAssertionFunc
	}{
		{
			name:                "valid",
			req:                 baseReq,
			mockExistingRoles:   []string{"integrationrole"},
			mockExistingSSMDocs: []string{},
			errCheck:            require.NoError,
		},
		{
			name:                "integration role does not exist",
			mockExistingRoles:   []string{},
			mockExistingSSMDocs: []string{},
			req:                 baseReq,
			errCheck:            notFoundCheck,
		},
		{
			name:                "ssm document already exists",
			mockExistingRoles:   []string{},
			mockExistingSSMDocs: []string{"MyDoc"},
			req:                 baseReq,
			errCheck:            require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockEC2SSMIAMConfigClient{
				existingRoles: tt.mockExistingRoles,
			}

			err := ConfigureEC2SSM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

type mockEC2SSMIAMConfigClient struct {
	existingRoles []string
	existingDocs  []string
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockEC2SSMIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamTypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}

// CreateDocument creates an SSM document.
func (m *mockEC2SSMIAMConfigClient) CreateDocument(ctx context.Context, params *ssm.CreateDocumentInput, optFns ...func(*ssm.Options)) (*ssm.CreateDocumentOutput, error) {
	if slices.Contains(m.existingDocs, aws.ToString(params.Name)) {
		return nil, &ssmtypes.DocumentAlreadyExists{}
	}
	return nil, nil
}
