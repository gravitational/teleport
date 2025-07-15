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

package auth

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/utils"
)

func responseFromAWSIdentity(id awsIdentity) string {
	return fmt.Sprintf(`{
		"GetCallerIdentityResponse": {
			"GetCallerIdentityResult": {
				"Account": "%s",
				"Arn": "%s"
			}}}`, id.Account, id.Arn)
}

type mockClient struct {
	respStatusCode int
	respBody       string
}

func (c *mockClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: c.respStatusCode,
		Body:       io.NopCloser(strings.NewReader(c.respBody)),
	}, nil
}

var identityRequestTemplate = template.Must(template.New("sts-request").Parse(`POST / HTTP/1.1
Host: {{.Host}}
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;{{.SignedHeader}}, Signature=111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaa
X-Teleport-Challenge: {{.Challenge}}

Action=GetCallerIdentity&Version=2011-06-15`))

type identityRequestTemplateInput struct {
	Host         string
	SignedHeader string
	Challenge    string
}

func defaultIdentityRequestTemplateInput(challenge string) identityRequestTemplateInput {
	return identityRequestTemplateInput{
		Host:         "sts.amazonaws.com",
		SignedHeader: "x-teleport-challenge;",
		Challenge:    challenge,
	}
}

type challengeResponseOption func(*identityRequestTemplateInput)

func withHost(host string) challengeResponseOption {
	return func(templateInput *identityRequestTemplateInput) {
		templateInput.Host = host
	}
}

func withSignedHeader(signedHeader string) challengeResponseOption {
	return func(templateInput *identityRequestTemplateInput) {
		templateInput.SignedHeader = signedHeader
	}
}

func withChallenge(challenge string) challengeResponseOption {
	return func(templateInput *identityRequestTemplateInput) {
		templateInput.Challenge = challenge
	}
}

func TestAuth_RegisterUsingIAMMethod(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	isAccessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	isBadParameter := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsBadParameter(err), "expected Bad Parameter error, actual error: %v", err)
	}

	testCases := []struct {
		desc                     string
		tokenName                string
		requestTokenName         string
		tokenSpec                types.ProvisionTokenSpecV2
		stsClient                utils.HTTPDoClient
		iamRegisterOptions       []iamRegisterOption
		challengeResponseOptions []challengeResponseOption
		challengeResponseErr     error
		assertError              require.ErrorAssertionFunc
	}{
		{
			desc:             "basic passing case",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard arn 1",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::role/admins-*",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-test",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard arn 2",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::role/admins-???",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-123",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "arn assumed role",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSARN:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard arn assumed role",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSARN:     "arn:aws:sts::123456789012:assumed-role/my-*-test-role/my-session-name",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard 2 arn assumed role",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSARN:     "arn:aws:sts::123456789012:assumed-role/my-*-test-role/*",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wrong wildcard arn assumed role",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSARN:     "arn:aws:sts::123456789012:assumed-role/my-*-test-role",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-002-test-role/my-session-name",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong wildcard 2 arn assumed role",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "123456789012",
						AWSARN:     "arn:aws:sts::123456789012:assumed-role/my-*-test-role",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-002-test-role/my-session-name2",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong token",
			tokenName:        "test-token",
			requestTokenName: "wrong-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "challenge response error",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseErr: trace.BadParameter("test error"),
			assertError:          isBadParameter,
		},
		{
			desc:             "wrong arn",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::role/admins-???",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-1234",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong challenge",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withChallenge("wrong-challenge"),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong account",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "5678",
					Arn:     "arn:aws::1111",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "sts api error",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusForbidden,
				respBody:       "access denied",
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong sts host",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withHost("sts.wrong-host.amazonaws.com"),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "regional sts endpoint",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withHost("sts.us-west-2.amazonaws.com"),
			},
			assertError: require.NoError,
		},
		{
			desc:             "unsigned challenge header",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withSignedHeader(""),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "fips pass",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			iamRegisterOptions: []iamRegisterOption{
				withFips(true),
				withAuthVersion(&semver.Version{Major: 12}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withHost("sts-fips.us-east-1.amazonaws.com"),
			},
			assertError: require.NoError,
		},
		{
			desc:             "non-fips client fail v12",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					{
						AWSAccount: "1234",
						AWSARN:     "arn:aws::1111",
					},
				},
				JoinMethod: types.JoinMethodIAM,
			},
			stsClient: &mockClient{
				respStatusCode: http.StatusOK,
				respBody: responseFromAWSIdentity(awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			iamRegisterOptions: []iamRegisterOption{
				withFips(true),
				withAuthVersion(&semver.Version{Major: 12}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withHost("sts.us-east-1.amazonaws.com"),
			},
			assertError: isAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Set mock client.
			a.httpClientForAWSSTS = tc.stsClient

			// add token to auth server
			token, err := types.NewProvisionTokenFromSpec(
				tc.tokenName,
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)
			require.NoError(t, a.UpsertToken(ctx, token))
			defer func() {
				require.NoError(t, a.DeleteToken(ctx, token.GetName()))
			}()

			_, err = a.RegisterUsingIAMMethodWithOpts(context.Background(), func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
				templateInput := defaultIdentityRequestTemplateInput(challenge)
				for _, opt := range tc.challengeResponseOptions {
					opt(&templateInput)
				}
				var identityRequest bytes.Buffer
				require.NoError(t, identityRequestTemplate.Execute(&identityRequest, templateInput))

				req := &proto.RegisterUsingIAMMethodRequest{
					RegisterUsingTokenRequest: &types.RegisterUsingTokenRequest{
						Token:        tc.requestTokenName,
						HostID:       "test-node",
						Role:         types.RoleNode,
						PublicSSHKey: sshPublicKey,
						PublicTLSKey: tlsPublicKey,
					},
					StsIdentityRequest: identityRequest.Bytes(),
				}
				return req, tc.challengeResponseErr
			}, tc.iamRegisterOptions...)
			tc.assertError(t, err)
		})
	}
}
