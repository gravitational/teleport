/*
Copyright 2021-2022 Gravitational, Inc.

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

package auth

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
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

const identityRequestTemplate = `POST / HTTP/1.1
Host: sts.amazonaws.com
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;x-teleport-challenge, Signature=111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaa
X-Teleport-Challenge: %s

Action=GetCallerIdentity&Version=2011-06-15`

const wrongHostTemplate = `POST / HTTP/1.1
Host: sts.example.com
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;x-teleport-challenge, Signature=111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaa
X-Teleport-Challenge: %s

Action=GetCallerIdentity&Version=2011-06-15`

const unsignedChallengeTemplate = `POST / HTTP/1.1
Host: sts.amazonaws.com
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token, Signature=111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaa
X-Teleport-Challenge: %s

Action=GetCallerIdentity&Version=2011-06-15`

func TestAuth_RegisterUsingIAMMethod(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

	sshPrivateKey, sshPublicKey, err := a.GenerateKeyPair("")
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	isAccessDenied := func(t require.TestingT, err error, _ ...interface{}) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	isBadParameter := func(t require.TestingT, err error, _ ...interface{}) {
		require.True(t, trace.IsBadParameter(err), "expected Bad Parameter error, actual error: %v", err)
	}

	testCases := []struct {
		desc                      string
		tokenName                 string
		requestTokenName          string
		tokenSpec                 types.ProvisionTokenSpecV2
		stsClient                 stsClient
		challengeResponseOverride string
		requestTemplate           string
		challengeResponseErr      error
		assertError               require.ErrorAssertionFunc
	}{
		{
			desc:             "basic passing case",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     require.NoError,
		},
		{
			desc:             "wildcard arn 1",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     require.NoError,
		},
		{
			desc:             "wildcard arn 2",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     require.NoError,
		},
		{
			desc:             "wrong token",
			tokenName:        "test-token",
			requestTokenName: "wrong-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     isAccessDenied,
		},
		{
			desc:             "challenge response error",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate:      identityRequestTemplate,
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
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     isAccessDenied,
		},
		{
			desc:             "wrong challenge",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			challengeResponseOverride: "wrong-challenge",
			requestTemplate:           identityRequestTemplate,
			assertError:               isAccessDenied,
		},
		{
			desc:             "wrong account",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     isAccessDenied,
		},
		{
			desc:             "sts api error",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: identityRequestTemplate,
			assertError:     isAccessDenied,
		},
		{
			desc:             "wrong sts host",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: wrongHostTemplate,
			assertError:     isAccessDenied,
		},
		{
			desc:             "unsigned challenge header",
			tokenName:        "test-token",
			requestTokenName: "test-token",
			tokenSpec: types.ProvisionTokenSpecV2{
				Roles: []types.SystemRole{types.RoleNode},
				Allow: []*types.TokenRule{
					&types.TokenRule{
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
			requestTemplate: unsignedChallengeTemplate,
			assertError:     isAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// add token to auth server
			token, err := types.NewProvisionTokenFromSpec(
				tc.tokenName,
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)
			require.NoError(t, a.UpsertToken(ctx, token))
			t.Cleanup(func() { require.NoError(t, a.DeleteToken(ctx, token.GetName())) })

			requestContext := context.Background()
			requestContext = context.WithValue(requestContext, ContextClientAddr, &net.IPAddr{})
			requestContext = context.WithValue(requestContext, stsClientKey{}, tc.stsClient)

			_, err = a.RegisterUsingIAMMethod(requestContext, func(challenge string) (*proto.RegisterUsingIAMMethodRequest, error) {
				if tc.challengeResponseOverride != "" {
					challenge = tc.challengeResponseOverride
				}
				identityRequest := []byte(fmt.Sprintf(tc.requestTemplate, challenge))
				req := &proto.RegisterUsingIAMMethodRequest{
					RegisterUsingTokenRequest: &types.RegisterUsingTokenRequest{
						Token:        tc.requestTokenName,
						HostID:       "test-node",
						Role:         types.RoleNode,
						PublicSSHKey: sshPublicKey,
						PublicTLSKey: tlsPublicKey,
					},
					StsIdentityRequest: identityRequest,
				}
				return req, tc.challengeResponseErr
			})
			tc.assertError(t, err)
		})
	}
}
