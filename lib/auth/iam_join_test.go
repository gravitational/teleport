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

func TestIAMJoin(t *testing.T) {
	a := newAuthServer(t)

	isNil := func(err error) bool {
		if err != nil {
			log.WithError(err).Error("unexpected error")
		}
		return err == nil
	}

	testCases := []struct {
		desc              string
		tokenSpec         types.ProvisionTokenSpecV2
		givenChallenge    string
		responseChallenge string
		stsClient         stsClient
		expectError       func(error) bool
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
			expectError:       isNil,
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
			expectError:       isNil,
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
			expectError:       isNil,
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
			expectError:       trace.IsAccessDenied,
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
			expectError:       trace.IsAccessDenied,
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
			expectError:       trace.IsAccessDenied,
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
			expectError:       trace.IsAccessDenied,
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

			signedRequest, err := createSignedSTSIdentityRequest(tc.responseChallenge)
			require.NoError(t, err)
			req := &types.RegisterUsingTokenRequest{
				Token:              "test-token",
				HostID:             "test-node",
				Role:               types.RoleNode,
				STSIdentityRequest: signedRequest,
			}

			err = a.checkIAMRequest(ctx, tc.stsClient, tc.givenChallenge, req)
			require.True(t, tc.expectError(err))
		})
	}
}
