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
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestIdPIAMConfigReqDefaults(t *testing.T) {
	baseIdPIAMConfigReq := func() IdPIAMConfigureRequest {
		return IdPIAMConfigureRequest{
			Cluster:                 "mycluster",
			IntegrationName:         "myintegration",
			IntegrationRole:         "integrationrole",
			ProxyPublicAddress:      "https://proxy.example.com",
			IntegrationPolicyPreset: "",
			AutoConfirm:             true,
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
				IntegrationPolicyPreset: PolicyPresetUnspecified,
				AutoConfirm:             true,
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
		{
			name: "invalid preset type",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReq()
				req.IntegrationPolicyPreset = "invalid_preset"
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
			AutoConfirm:        true,
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
			name:               "role exists with empty trust policy",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles: map[string]mockRole{"integrationrole": {
				tags: []iamtypes.Tag{
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
			name:               "role exists with existing trust policy and without matching tags",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles: map[string]mockRole{"integrationrole": {
				tags: []iamtypes.Tag{
					{Key: aws.String("teleport.dev/origin"), Value: aws.String("should be overwritten")},
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
				gotTags := map[string]string{}
				for _, tag := range role.tags {
					gotTags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}
				wantTags := map[string]string{
					"teleport.dev/origin":      "integration_awsoidc",
					"teleport.dev/cluster":     "mycluster",
					"teleport.dev/integration": "myintegration",
				}
				require.Equal(t, wantTags, gotTags)
			},
		},
		{
			name:               "role exists with matching trust policy",
			mockAccountID:      "123456789012",
			mockExistingIdPUrl: []string{},
			mockExistingRoles: map[string]mockRole{"integrationrole": {
				tags: []iamtypes.Tag{
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
				CallerIdentityGetter: mockSTSClient{accountID: tt.mockAccountID},
				existingRoles:        tt.mockExistingRoles,
				existingIDPUrl:       tt.mockExistingIdPUrl,
			}

			err := ConfigureIdPIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)

			if tt.externalStateCheck != nil {
				tt.externalStateCheck(t, clt)
			}
		})
	}
}

func TestConfigureIdPIAMWithPresetPolicy(t *testing.T) {
	ctx := context.Background()
	tlsServer := httptest.NewTLSServer(nil)
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)
	const mockAccountID string = "123456789012"
	baseIdPIAMConfigReqWithTLServer := func() IdPIAMConfigureRequest {
		return IdPIAMConfigureRequest{
			Cluster:            "mycluster",
			IntegrationName:    "myintegration",
			IntegrationRole:    "integrationrole",
			ProxyPublicAddress: tlsServer.URL,
			AutoConfirm:        true,
		}
	}

	for _, tt := range []struct {
		name               string
		mockExistingRoles  map[string]mockRole
		mockExistingIdPUrl []string
		req                func() IdPIAMConfigureRequest
		errCheck           require.ErrorAssertionFunc
		policyStatement    *awslib.Statement
		externalStateCheck func(*testing.T, mockIdPIAMConfigClient)
	}{
		{
			name: "without policy-preset",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReqWithTLServer()
				req.IntegrationPolicyPreset = ""
				return req
			},
			mockExistingIdPUrl: []string{},
			mockExistingRoles:  map[string]mockRole{},
			errCheck:           require.NoError,
		},
		{
			name: "with PolicyPresetUnspecified",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReqWithTLServer()
				req.IntegrationPolicyPreset = PolicyPresetUnspecified
				return req
			},
			mockExistingIdPUrl: []string{},
			mockExistingRoles:  map[string]mockRole{},
			errCheck:           require.NoError,
		},
		{
			name: "with PolicyPresetAWSIdentityCenter",
			req: func() IdPIAMConfigureRequest {
				req := baseIdPIAMConfigReqWithTLServer()
				req.IntegrationPolicyPreset = PolicyPresetAWSIdentityCenter
				return req
			},
			mockExistingIdPUrl: []string{},
			mockExistingRoles:  map[string]mockRole{},
			policyStatement:    awslib.StatementForAWSIdentityCenterAccess(),
			errCheck:           require.NoError,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clt := mockIdPIAMConfigClient{
				CallerIdentityGetter: mockSTSClient{accountID: mockAccountID},
				existingRoles:        tt.mockExistingRoles,
				existingIDPUrl:       tt.mockExistingIdPUrl,
			}

			err := ConfigureIdPIAM(ctx, &clt, tt.req())
			tt.errCheck(t, err)

			role, ok := clt.existingRoles[(tt.req().IntegrationRole)]
			require.True(t, ok)

			if tt.req().IntegrationPolicyPreset == "" || tt.req().IntegrationPolicyPreset == PolicyPresetUnspecified {
				require.Nil(t, role.presetPolicyDoc)
			} else {
				policyDocument, err := awslib.NewPolicyDocument(
					tt.policyStatement,
				).Marshal()
				require.NoError(t, err)
				require.NotEmpty(t, role.presetPolicyDoc)
				require.Equal(t, &policyDocument, role.presetPolicyDoc)
			}
		})
	}
}

var goldenIdPIAMConfigureRequest IdPIAMConfigureRequest = IdPIAMConfigureRequest{
	Cluster:            "mycluster",
	IntegrationName:    "myintegration",
	IntegrationRole:    "integrationrole",
	ProxyPublicAddress: "https://example.com",
	AutoConfirm:        true,
	fakeThumbprint:     "15dbd260c7465ecca6de2c0b2181187f66ee0d1a",
}

func TestConfigureIdPIAMOutput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := goldenIdPIAMConfigureRequest
	req.stdout = &buf

	clt := mockIdPIAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: "123456789012"},
		existingRoles:        map[string]mockRole{},
		existingIDPUrl:       []string{},
	}

	require.NoError(t, ConfigureIdPIAM(ctx, &clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

func TestConfigureIdPIAMWithPolicyPresetOutput(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	req := goldenIdPIAMConfigureRequest
	req.stdout = &buf
	req.IntegrationPolicyPreset = PolicyPresetAWSIdentityCenter

	clt := mockIdPIAMConfigClient{
		CallerIdentityGetter: mockSTSClient{accountID: "123456789012"},
		existingRoles:        map[string]mockRole{},
		existingIDPUrl:       []string{},
	}

	require.NoError(t, ConfigureIdPIAM(ctx, &clt, req))
	if golden.ShouldSet() {
		golden.Set(t, buf.Bytes())
	}
	require.Equal(t, string(golden.Get(t)), buf.String())
}

type mockRole struct {
	assumeRolePolicyDoc *string
	tags                []iamtypes.Tag
	presetPolicyDoc     *string
}

type mockIdPIAMConfigClient struct {
	CallerIdentityGetter
	existingIDPUrl []string
	existingRoles  map[string]mockRole
}

// CreateRole creates a new IAM Role.
func (m *mockIdPIAMConfigClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("Role %q already exists.", *params.RoleName)
	_, found := m.existingRoles[aws.ToString(params.RoleName)]
	if found {
		return nil, &iamtypes.EntityAlreadyExistsException{
			Message: &alreadyExistsMessage,
		}
	}
	m.existingRoles[*params.RoleName] = mockRole{
		tags:                params.Tags,
		assumeRolePolicyDoc: params.AssumeRolePolicyDocument,
	}

	return &iam.CreateRoleOutput{
		Role: &iamtypes.Role{
			Arn: aws.String("arn:something"),
		},
	}, nil
}

// PutRolePolicy assigns a policy to an existing IAM Role.
func (m *mockIdPIAMConfigClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	doesNotExistMessage := fmt.Sprintf("Role %q does not exist.", *params.RoleName)
	if _, ok := m.existingRoles[aws.ToString(params.RoleName)]; !ok {
		return nil, &iamtypes.NoSuchEntityException{
			Message: &doesNotExistMessage,
		}
	}

	m.existingRoles[*params.RoleName] = mockRole{
		tags:                m.existingRoles[*params.RoleName].tags,
		assumeRolePolicyDoc: m.existingRoles[*params.RoleName].assumeRolePolicyDoc,
		presetPolicyDoc:     params.PolicyDocument,
	}

	return &iam.PutRolePolicyOutput{}, nil
}

// CreateOpenIDConnectProvider creates an IAM OpenID Connect Provider.
func (m *mockIdPIAMConfigClient) CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput, optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error) {
	alreadyExistsMessage := fmt.Sprintf("IdP with URL %q already exists.", *params.Url)
	if slices.Contains(m.existingIDPUrl, *params.Url) {
		return nil, &iamtypes.EntityAlreadyExistsException{
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
		Role: &iamtypes.Role{
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

func (m *mockIdPIAMConfigClient) TagRole(ctx context.Context, params *iam.TagRoleInput, _ ...func(*iam.Options)) (*iam.TagRoleOutput, error) {
	roleName := aws.ToString(params.RoleName)
	role, found := m.existingRoles[roleName]
	if !found {
		return nil, trace.NotFound("role not found")
	}

	tags := tags.AWSTags{}
	for _, existingTag := range role.tags {
		tags[*existingTag.Key] = *existingTag.Value
	}
	for _, newTag := range params.Tags {
		tags[*newTag.Key] = *newTag.Value
	}
	role.tags = tags.ToIAMTags()
	m.existingRoles[roleName] = role
	return &iam.TagRoleOutput{}, nil
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
