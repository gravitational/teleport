/*
Copyright 2021 Gravitational, Inc.

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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type mockSTSClient struct {
	nodeIdentity awsIdentity
}

func (m mockSTSClient) Do(req *http.Request) (*http.Response, error) {
	responseBody := fmt.Sprintf(`{
		"GetCallerIdentityResponse": {
			"GetCallerIdentityResult": {
				"Account": "%s",
				"Arn": "%s"
			}}}`, m.nodeIdentity.Account, m.nodeIdentity.Arn)

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}, nil
}

type errorSTSClient struct{}

func (errorSTSClient) Do(req *http.Request) (*http.Response, error) {
	responseBody := "Access Denied"
	return &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}, nil
}

const identityRequestTemplate = `POST / HTTP/1.1
Host: sts.amazonaws.com
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;x-teleport-challenge, Signature=1111111111111111111111111111111111111111111111111111111111111111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=
X-Teleport-Challenge: %s

Action=GetCallerIdentity&Version=2011-06-15`

const wrongHostTemplate = `POST / HTTP/1.1
Host: sts.example.com
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token;x-teleport-challenge, Signature=1111111111111111111111111111111111111111111111111111111111111111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=
X-Teleport-Challenge: %s

Action=GetCallerIdentity&Version=2011-06-15`

const unsignedChallengeTemplate = `POST / HTTP/1.1
Host: sts.amazonaws.com
User-Agent: aws-sdk-go/1.37.17 (go1.17.1; darwin; amd64)
Content-Length: 43
Accept: application/json
Authorization: AWS4-HMAC-SHA256 Credential=AAAAAAAAAAAAAAAAAAAA/20211102/us-east-1/sts/aws4_request, SignedHeaders=accept;content-length;content-type;host;x-amz-date;x-amz-security-token, Signature=1111111111111111111111111111111111111111111111111111111111111111
Content-Type: application/x-www-form-urlencoded; charset=utf-8
X-Amz-Date: 20211102T204300Z
X-Amz-Security-Token: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=
X-Teleport-Challenge: %s

Action=GetCallerIdentity&Version=2011-06-15`

func TestIAMJoin(t *testing.T) {
	a := newAuthServer(t)

	isAccessDenied := func(t require.TestingT, err error, _ ...interface{}) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}

	testCases := []struct {
		desc              string
		tokenSpec         types.ProvisionTokenSpecV2
		stsClient         stsClient
		givenChallenge    string
		responseChallenge string
		requestTemplate   string
		assertError       require.ErrorAssertionFunc
	}{
		{
			desc: "basic passing case",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       require.NoError,
		},
		{
			desc: "wildcard arn 1",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-test",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       require.NoError,
		},
		{
			desc: "wildcard arn 2",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-123",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       require.NoError,
		},
		{
			desc: "wrong arn 1",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-1234",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       isAccessDenied,
		},
		{
			desc: "wrong challenge",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "wrong-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       isAccessDenied,
		},
		{
			desc: "wrong account",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "5678",
					Arn:     "arn:aws::1111",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       isAccessDenied,
		},
		{
			desc: "sts api error",
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
			stsClient:         errorSTSClient{},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   identityRequestTemplate,
			assertError:       isAccessDenied,
		},
		{
			desc: "wrong sts host",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   wrongHostTemplate,
			assertError:       isAccessDenied,
		},
		{
			desc: "unsigned challenge header",
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
			stsClient: mockSTSClient{
				nodeIdentity: awsIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				},
			},
			givenChallenge:    "test-challenge",
			responseChallenge: "test-challenge",
			requestTemplate:   unsignedChallengeTemplate,
			assertError:       isAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()

			// add token to auth server
			token, err := types.NewProvisionTokenFromSpec("test-token",
				time.Now().Add(time.Minute),
				tc.tokenSpec)
			require.NoError(t, err)
			require.NoError(t, a.UpsertToken(ctx, token))
			t.Cleanup(func() { require.NoError(t, a.DeleteToken(ctx, token.GetName())) })

			identityRequest := []byte(fmt.Sprintf(tc.requestTemplate, tc.responseChallenge))

			req := &types.RegisterUsingTokenRequest{
				Token:              "test-token",
				HostID:             "test-node",
				Role:               types.RoleNode,
				STSIdentityRequest: identityRequest,
			}

			err = a.checkIAMRequest(ctx, tc.stsClient, tc.givenChallenge, req)
			tc.assertError(t, err)
		})
	}
}
