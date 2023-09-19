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
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/lib"
)

var baseIdPIAMConfigReq = func() IdPIAMConfigureRequest {
	return IdPIAMConfigureRequest{
		Cluster:            "mycluster",
		IntegrationName:    "myintegration",
		Region:             "us-east-1",
		IntegrationRole:    "integrationrole",
		ProxyPublicAddress: "https://proxy.example.com",
	}
}

func TestIdPIAMConfigReqDefaults(t *testing.T) {
	for _, tt := range []struct {
		name     string
		req      func() IdPIAMConfigureRequest
		errCheck require.ErrorAssertionFunc
		expected IdPIAMConfigureRequest
	}{
		{
			name:     "set defaults",
			req:      baseIdPIAMConfigReq,
			errCheck: require.NoError,
			expected: IdPIAMConfigureRequest{
				Cluster:            "mycluster",
				IntegrationName:    "myintegration",
				Region:             "us-east-1",
				IntegrationRole:    "integrationrole",
				ProxyPublicAddress: "https://proxy.example.com",
				issuer:             "proxy.example.com",
			},
		},
		{
			name: "missing cluster",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReq()
				req.Cluster = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration name",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReq()
				req.IntegrationName = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing region",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReq()
				req.Region = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing integration role",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReq()
				req.IntegrationRole = ""
				return req
			},
			errCheck: badParameterCheck,
		},
		{
			name: "missing proxy public address",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReq()
				req.ProxyPublicAddress = ""
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

func TestConfigureIdPIAM(t *testing.T) {
	ctx := context.Background()

	tlsServer := httptest.NewTLSServer(nil)
	// TLS Server starts with self-signed certificates.
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	baseIdPIAMConfigReqWithTLServer := func() IdPIAMConfigureRequest {
		base := baseIdPIAMConfigReq()
		base.ProxyPublicAddress = tlsServer.URL
		return base
	}

	for _, tt := range []struct {
		name               string
		mockAccountID      string
		mockExistingRoles  []string
		mockExistingIdPUrl []string
		req                func() IdPIAMConfigureRequest
		errCheck           require.ErrorAssertionFunc
	}{
		{
			name:          "valid",
			mockAccountID: "123456789012",
			req:           baseIdPIAMConfigReqWithTLServer,
			errCheck:      require.NoError,
		},
		{
			name:               "idp url already exists",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{tlsServer.URL},
			req:                baseIdPIAMConfigReqWithTLServer,
			errCheck:           alreadyExistsCheck,
		},
		{
			name:              "integration role already exists",
			mockAccountID:     "123456789012",
			mockExistingRoles: []string{"integrationrole"},
			req:               baseIdPIAMConfigReqWithTLServer,
			errCheck:          alreadyExistsCheck,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockIdPIAMConfigClient{
				accountID:      tt.mockAccountID,
				existingRoles:  tt.mockExistingRoles,
				existingIDPUrl: tt.mockExistingIdPUrl,
			}

			err := ConfigureIdPIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)
		})
	}
}

type mockIdPIAMConfigClient struct {
	accountID      string
	existingRoles  []string
	existingIDPUrl []string
}

// GetCallerIdentity returns information about the caller identity.
func (m *mockIdPIAMConfigClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Account: &m.accountID,
	}, nil
}

// CreateRole creates a new IAM Role.
func (m *mockIdPIAMConfigClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("Role %q already exists.", *params.RoleName)
	if slices.Contains(m.existingRoles, *params.RoleName) {
		return nil, &iamTypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingRoles = append(m.existingRoles, *params.RoleName)

	return nil, nil
}

// CreateOpenIDConnectProvider creates an IAM OpenID Connect Provider.
func (m *mockIdPIAMConfigClient) CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput, optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("IdP with URL %q already exists.", *params.Url)
	if slices.Contains(m.existingIDPUrl, *params.Url) {
		return nil, &iamTypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingIDPUrl = append(m.existingRoles, *params.Url)

	return &iam.CreateOpenIDConnectProviderOutput{
		OpenIDConnectProviderArn: aws.String("arn:something"),
	}, nil
}
