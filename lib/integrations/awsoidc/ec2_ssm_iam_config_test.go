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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestEC2SSMIAMConfigReqDefaults(t *testing.T) {
	baseReq := func() EC2SSMIAMConfigureRequest {
		return EC2SSMIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
			SSMDocumentName: "MyDoc",
			ProxyPublicURL:  "https://proxy.example.com",
			ClusterName:     "my-cluster",
			IntegrationName: "my-integration",
			AccountID:       "123456789012",
			AutoConfirm:     true,
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
				ClusterName:                 "my-cluster",
				IntegrationName:             "my-integration",
				AccountID:                   "123456789012",
				AutoConfirm:                 true,
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
			name: "missing integration name",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.IntegrationName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing cluster name",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.ClusterName = ""
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
		{
			name: "missing account id is ok",
			req: func() EC2SSMIAMConfigureRequest {
				req := baseReq()
				req.AccountID = ""
				return req
			},
			errCheck: require.NoError,
			expected: EC2SSMIAMConfigureRequest{
				Region:                      "us-east-1",
				IntegrationRole:             "integrationrole",
				IntegrationRoleEC2SSMPolicy: "EC2DiscoverWithSSM",
				SSMDocumentName:             "MyDoc",
				ProxyPublicURL:              "https://proxy.example.com",
				ClusterName:                 "my-cluster",
				IntegrationName:             "my-integration",
				AutoConfirm:                 true,
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

func TestEC2SSMIAMConfig(t *testing.T) {
	ctx := context.Background()
	baseReq := func() EC2SSMIAMConfigureRequest {
		return EC2SSMIAMConfigureRequest{
			Region:          "us-east-1",
			IntegrationRole: "integrationrole",
			SSMDocumentName: "MyDoc",
			ProxyPublicURL:  "https://proxy.example.com",
			ClusterName:     "my-cluster",
			IntegrationName: "my-integration",
			AccountID:       "123456789012",
			AutoConfirm:     true,
		}
	}

	for _, tt := range []struct {
		name                string
		mockAccountID       string
		mockExistingRoles   []string
		mockExistingSSMDocs []string
		req                 func() EC2SSMIAMConfigureRequest
		errCheck            require.ErrorAssertionFunc
	}{
		{
			name:                "valid",
			req:                 baseReq,
			mockAccountID:       "123456789012",
			mockExistingRoles:   []string{"integrationrole"},
			mockExistingSSMDocs: []string{},
			errCheck:            require.NoError,
		},
		{
			name:                "integration role does not exist",
			mockAccountID:       "123456789012",
			mockExistingRoles:   []string{},
			mockExistingSSMDocs: []string{},
			req:                 baseReq,
			errCheck:            notFoundCheck,
		},
		{
			name:                "ssm document already exists",
			mockAccountID:       "123456789012",
			mockExistingRoles:   []string{},
			mockExistingSSMDocs: []string{"MyDoc"},
			req:                 baseReq,
			errCheck:            require.Error,
		},
		{
			name:                "account does not match expected account",
			req:                 baseReq,
			mockAccountID:       "222222222222",
			mockExistingRoles:   []string{"integrationrole"},
			mockExistingSSMDocs: []string{},
			errCheck:            badParameterCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockEC2SSMIAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
			}

			err := ConfigureEC2SSM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
			if err == nil {
				require.Contains(t, clt.existingDocs, tt.req().SSMDocumentName)
				require.ElementsMatch(t, []ssmtypes.Tag{
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("my-cluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("my-integration")},
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
				}, clt.existingDocs[tt.req().SSMDocumentName])
			}
		})
	}
}

func TestEC2SSMIAMConfigOutput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := EC2SSMIAMConfigureRequest{
		Region:                               "us-east-1",
		IntegrationRole:                      "integrationrole",
		SSMDocumentName:                      "MyDoc",
		ProxyPublicURL:                       "https://proxy.example.com",
		ClusterName:                          "my-cluster",
		IntegrationName:                      "my-integration",
		AccountID:                            "123456789012",
		AutoConfirm:                          true,
		stdout:                               &buf,
		insecureSkipInstallPathRandomization: true,
	}

	clt := mockEC2SSMIAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: req.AccountID},
		existingRoles:        []string{req.IntegrationRole},
	}

	require.NoError(t, ConfigureEC2SSM(ctx, &clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockEC2SSMIAMConfigClient struct {
	CallerIdentityGetter
	existingRoles []string
	existingDocs  map[string][]ssmtypes.Tag
}

// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
func (m *mockEC2SSMIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	if !slices.Contains(m.existingRoles, *params.RoleName) {
		noSuchEntityMessage := fmt.Sprintf("role %q does not exist.", *params.RoleName)
		return nil, &iamtypes.NoSuchEntityException{
			Message: &noSuchEntityMessage,
		}
	}
	return nil, nil
}

// CreateDocument creates an SSM document.
func (m *mockEC2SSMIAMConfigClient) CreateDocument(ctx context.Context, params *ssm.CreateDocumentInput, optFns ...func(*ssm.Options)) (*ssm.CreateDocumentOutput, error) {
	if m.existingDocs == nil {
		m.existingDocs = make(map[string][]ssmtypes.Tag)
	}
	if _, ok := m.existingDocs[aws.ToString(params.Name)]; ok {
		return nil, &ssmtypes.DocumentAlreadyExists{}
	}
	m.existingDocs[aws.ToString(params.Name)] = params.Tags
	return nil, nil
}
