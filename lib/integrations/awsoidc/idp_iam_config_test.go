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

	return &iam.CreateRoleOutput{
		Role: &iamTypes.Role{
			Arn: aws.String("arn:something"),
		},
	}, nil
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

func TestNewIdPIAMConfigureClient(t *testing.T) {
	t.Run("no aws_region env var, returns an error", func(t *testing.T) {
		_, err := NewIdPIAMConfigureClient(context.Background())
		require.ErrorContains(t, err, "please set the AWS_REGION environment variable")
	})

	t.Run("aws_region env var was set, success", func(t *testing.T) {
		t.Setenv("AWS_REGION", "some-region")
		idpClient, err := NewIdPIAMConfigureClient(context.Background())
		require.NoError(t, err)
		require.NotNil(t, idpClient)
	})
}
