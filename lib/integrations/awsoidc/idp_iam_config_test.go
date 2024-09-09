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
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
)

func TestIdPIAMConfigReqDefaults(t *testing.T) {
	baseIdPIAMConfigReq := func() IdPIAMConfigureRequest {
		return IdPIAMConfigureRequest{
			Cluster:            "mycluster",
			IntegrationName:    "myintegration",
			IntegrationRole:    "integrationrole",
			ProxyPublicAddress: "https://proxy.example.com",
		}
	}

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
				issuerURL:          "https://proxy.example.com",
				ownershipTags: tags.AWSTags{
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "myintegration",
					"teleport.dev/origin":      "integration_awsoidc",
				},
			},
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

func policyDocWithStatementsJSON(statement ...string) *string {
	statements := strings.Join(statement, ",")
	ret := fmt.Sprintf(`{
        "Version": "2012-10-17",
        "Statement": [
            %s
        ]
    }`, statements)
	return &ret
}

func assumeRoleStatementJSON(issuer string) string {
	return fmt.Sprintf(`{
    "Effect": "Allow",
    "Action": "sts:AssumeRoleWithWebIdentity",
    "Principal": {
        "Federated": "arn:aws:iam::123456789012:oidc-provider/%s"
    },
    "Condition": {
        "StringEquals": {
            "%s:aud": "discover.teleport"
        }
    }
}`, issuer, issuer)
}

func TestConfigureIdPIAM(t *testing.T) {
	ctx := context.Background()

	tlsServer := httptest.NewTLSServer(nil)
	tlsServerURL, err := url.Parse(tlsServer.URL)
	require.NoError(t, err)

	tlsServerIssuer := tlsServerURL.Host
	// TLS Server starts with self-signed certificates.

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	baseIdPIAMConfigReqWithTLServer := func() IdPIAMConfigureRequest {
		return IdPIAMConfigureRequest{
			Cluster:            "mycluster",
			IntegrationName:    "myintegration",
			IntegrationRole:    "integrationrole",
			ProxyPublicAddress: tlsServer.URL,
		}
	}

	for _, tt := range []struct {
		name               string
		mockAccountID      string
		mockExistingRoles  map[string]mockRole
		mockExistingIdPUrl []string
		req                func() IdPIAMConfigureRequest
		errCheck           require.ErrorAssertionFunc
		externalStateCheck func(*testing.T, mockIdPIAMConfigClient)
	}{
		{
			name:               "valid",
			mockAccountID:      "123456789012",
			req:                baseIdPIAMConfigReqWithTLServer,
			mockExistingIdPUrl: []string{},
			mockExistingRoles:  map[string]mockRole{},
			errCheck:           require.NoError,
		},
		{
			name:               "idp url already exists",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{tlsServer.URL},
			mockExistingRoles:  map[string]mockRole{},
			req:                baseIdPIAMConfigReqWithTLServer,
			errCheck:           require.NoError,
		},
		{
			name:               "role exists, no ownership tags",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles:  map[string]mockRole{"integrationrole": {}},
			req:                baseIdPIAMConfigReqWithTLServer,
			errCheck:           badParameterCheck,
		},
		{
			name:               "role exists, ownership tags, no assume role",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles: map[string]mockRole{"integrationrole": {
				tags: []iamTypes.Tag{
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myintegration")},
				},
				assumeRolePolicyDoc: aws.String(`{"Version":"2012-10-17", "Statements":[]}`),
			}},
			req:      baseIdPIAMConfigReqWithTLServer,
			errCheck: require.NoError,
			externalStateCheck: func(t *testing.T, mipc mockIdPIAMConfigClient) {
				role := mipc.existingRoles["integrationrole"]
				expectedAssumeRolePolicyDoc := policyDocWithStatementsJSON(
					assumeRoleStatementJSON(tlsServerIssuer),
				)
				require.JSONEq(t, *expectedAssumeRolePolicyDoc, aws.ToString(role.assumeRolePolicyDoc))
			},
		},
		{
			name:               "role exists, ownership tags, with existing assume role",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles: map[string]mockRole{"integrationrole": {
				tags: []iamTypes.Tag{
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myintegration")},
				},
				assumeRolePolicyDoc: policyDocWithStatementsJSON(
					assumeRoleStatementJSON("some-other-issuer"),
				),
			}},
			req:      baseIdPIAMConfigReqWithTLServer,
			errCheck: require.NoError,
			externalStateCheck: func(t *testing.T, mipc mockIdPIAMConfigClient) {
				role := mipc.existingRoles["integrationrole"]
				expectedAssumeRolePolicyDoc := policyDocWithStatementsJSON(
					assumeRoleStatementJSON("some-other-issuer"),
					assumeRoleStatementJSON(tlsServerIssuer),
				)
				require.JSONEq(t, *expectedAssumeRolePolicyDoc, aws.ToString(role.assumeRolePolicyDoc))
			},
		},
		{
			name:               "role exists, ownership tags, assume role already exists",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles: map[string]mockRole{"integrationrole": {
				tags: []iamTypes.Tag{
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
					{Key: aws.String("teleport.dev/cluster"), Value: aws.String("mycluster")},
					{Key: aws.String("teleport.dev/integration"), Value: aws.String("myintegration")},
				},
				assumeRolePolicyDoc: policyDocWithStatementsJSON(
					assumeRoleStatementJSON(tlsServerIssuer),
				),
			}},
			req:      baseIdPIAMConfigReqWithTLServer,
			errCheck: require.NoError,
			externalStateCheck: func(t *testing.T, mipc mockIdPIAMConfigClient) {
				role := mipc.existingRoles["integrationrole"]
				expectedAssumeRolePolicyDoc := policyDocWithStatementsJSON(
					assumeRoleStatementJSON(tlsServerIssuer),
				)
				require.JSONEq(t, *expectedAssumeRolePolicyDoc, aws.ToString(role.assumeRolePolicyDoc))
			},
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

			if tt.externalStateCheck != nil {
				tt.externalStateCheck(t, clt)
			}
		})
	}
}

type mockRole struct {
	assumeRolePolicyDoc *string
	tags                []iamTypes.Tag
}

type mockIdPIAMConfigClient struct {
	accountID      string
	existingIDPUrl []string
	existingRoles  map[string]mockRole
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
	_, found := m.existingRoles[aws.ToString(params.RoleName)]
	if found {
		return nil, &iamTypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingRoles[*params.RoleName] = mockRole{
		tags:                params.Tags,
		assumeRolePolicyDoc: params.AssumeRolePolicyDocument,
	}

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
	m.existingIDPUrl = append(m.existingIDPUrl, *params.Url)

	return &iam.CreateOpenIDConnectProviderOutput{}, nil
}

// GetRole retrieves information about the specified role, including the role's path,
// GUID, ARN, and the role's trust policy that grants permission to assume the
// role.
func (m *mockIdPIAMConfigClient) GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	role, found := m.existingRoles[aws.ToString(params.RoleName)]
	if !found {
		return nil, trace.NotFound("role not found")
	}
	return &iam.GetRoleOutput{
		Role: &iamTypes.Role{
			Tags:                     role.tags,
			AssumeRolePolicyDocument: role.assumeRolePolicyDoc,
		},
	}, nil
}

// UpdateAssumeRolePolicy updates the policy that grants an IAM entity permission to assume a role.
// This is typically referred to as the "role trust policy".
func (m *mockIdPIAMConfigClient) UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error) {
	role, found := m.existingRoles[aws.ToString(params.RoleName)]
	if !found {
		return nil, trace.NotFound("role not found")
	}

	role.assumeRolePolicyDoc = params.PolicyDocument
	m.existingRoles[aws.ToString(params.RoleName)] = role

	return &iam.UpdateAssumeRolePolicyOutput{}, nil
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
