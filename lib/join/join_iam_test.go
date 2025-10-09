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

package join_test

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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/join/iam"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/join/iamjoin"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/utils"
)

func responseFromAWSIdentity(id iamjoin.AWSIdentity) string {
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

type iamJoinTestCase struct {
	desc                     string
	authServer               *authtest.Server
	tokenName                string
	requestTokenName         string
	tokenSpec                types.ProvisionTokenSpecV2
	stsClient                utils.HTTPDoClient
	challengeResponseOptions []challengeResponseOption
	challengeResponseErr     error
	assertError              require.ErrorAssertionFunc
}

func TestJoinIAM(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	regularServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir: t.TempDir(),
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, regularServer.Shutdown(ctx)) })

	fipsServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:  t.TempDir(),
			FIPS: true,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, regularServer.Shutdown(ctx)) })

	isAccessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected Access Denied error, actual error: %v", err)
	}
	isBadParameter := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsBadParameter(err), "expected Bad Parameter error, actual error: %v", err)
	}

	testCases := []iamJoinTestCase{
		{
			desc:             "basic passing case",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard arn 1",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-test",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard arn 2",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-123",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "arn assumed role",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard arn assumed role",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wildcard 2 arn assumed role",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-001-test-role/my-session-name",
				}),
			},
			assertError: require.NoError,
		},
		{
			desc:             "wrong wildcard arn assumed role",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-002-test-role/my-session-name",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong wildcard 2 arn assumed role",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "123456789012",
					Arn:     "arn:aws:sts::123456789012:assumed-role/my-super-002-test-role/my-session-name2",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong token",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "challenge response error",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseErr: trace.BadParameter("test error"),
			assertError:          isBadParameter,
		},
		{
			desc:             "wrong arn",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::role/admins-1234",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "wrong challenge",
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
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
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "5678",
					Arn:     "arn:aws::1111",
				}),
			},
			assertError: isAccessDenied,
		},
		{
			desc:             "sts api error",
			authServer:       regularServer,
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
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
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
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
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
			authServer:       regularServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
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
			authServer:       fipsServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withHost("sts-fips.us-east-1.amazonaws.com"),
			},
			assertError: require.NoError,
		},
		{
			desc:             "non-fips client fail",
			authServer:       fipsServer,
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
				respBody: responseFromAWSIdentity(iamjoin.AWSIdentity{
					Account: "1234",
					Arn:     "arn:aws::1111",
				}),
			},
			challengeResponseOptions: []challengeResponseOption{
				withHost("sts.us-east-1.amazonaws.com"),
			},
			assertError: isAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			testIAMJoin(t, &tc)
		})
	}
}

func testIAMJoin(t *testing.T, tc *iamJoinTestCase) {
	ctx := t.Context()
	// Set mock client.
	tc.authServer.Auth().SetHTTPClientForAWSSTS(tc.stsClient)

	// Add token to auth server.
	token, err := types.NewProvisionTokenFromSpec(
		tc.tokenName,
		time.Now().Add(time.Minute),
		tc.tokenSpec)
	require.NoError(t, err)
	require.NoError(t, tc.authServer.Auth().UpsertToken(ctx, token))
	t.Cleanup(func() {
		assert.NoError(t, tc.authServer.Auth().DeleteToken(ctx, token.GetName()))
	})

	// Make an unauthenticated auth client that will be used for the join.
	nopClient, err := tc.authServer.NewClient(authtest.TestNop())
	require.NoError(t, err)
	defer nopClient.Close()

	createSignedSTSIdentityRequest := func(ctx context.Context, challenge string, opts ...iam.STSIdentityRequestOption) ([]byte, error) {
		if tc.challengeResponseErr != nil {
			return nil, trace.Wrap(tc.challengeResponseErr)
		}
		templateInput := defaultIdentityRequestTemplateInput(challenge)
		for _, opt := range tc.challengeResponseOptions {
			opt(&templateInput)
		}
		var identityRequest bytes.Buffer
		if err := identityRequestTemplate.Execute(&identityRequest, templateInput); err != nil {
			return nil, trace.Wrap(err)
		}
		return identityRequest.Bytes(), nil
	}

	// Test joining via the legacy join service.
	//
	// TODO(nklaassen): DELETE IN 20 when removing the legacy join service.
	t.Run("legacy", func(t *testing.T) {
		_, err := joinclient.LegacyJoin(ctx, joinclient.JoinParams{
			Token:      tc.requestTokenName,
			JoinMethod: types.JoinMethodIAM,
			ID: state.IdentityID{
				Role:     types.RoleInstance,
				HostUUID: "test-uuid",
				NodeName: "test-node",
			},
			CreateSignedSTSIdentityRequestFunc: createSignedSTSIdentityRequest,
			AuthClient:                         nopClient,
		})
		tc.assertError(t, err)
	})

	// Tests joining via the new join service with auth-assigned host UUIDs.
	t.Run("new", func(t *testing.T) {
		_, err := joinclient.Join(ctx, joinclient.JoinParams{
			Token: tc.requestTokenName,
			ID: state.IdentityID{
				Role:     types.RoleInstance,
				NodeName: "test-node",
			},
			CreateSignedSTSIdentityRequestFunc: createSignedSTSIdentityRequest,
			AuthClient:                         nopClient,
		})
		tc.assertError(t, err)

		// If the challenge-response is expected to fail, assert that a join
		// failure event was emitted with an error message about the client
		// giving up on the join attempt.
		if tc.challengeResponseErr == nil {
			return
		}
		assert.EventuallyWithT(t, func(t *assert.CollectT) {
			evt, err := lastEvent(ctx, tc.authServer.Auth(), tc.authServer.Auth().GetClock(), "instance.join")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(
				&apievents.InstanceJoin{
					Metadata: apievents.Metadata{
						Type: "instance.join",
						Code: events.InstanceJoinFailureCode,
					},
					Status: apievents.Status{
						Success: false,
						Error: fmt.Sprintf(
							"receiving challenge solution\n\tclient gave up on join attempt: challenge solution failed: creating signed sts:GetCallerIdentity request %s",
							tc.challengeResponseErr,
						),
					},
					ConnectionMetadata: apievents.ConnectionMetadata{
						RemoteAddr: "127.0.0.1",
					},
					Role:      "Instance",
					Method:    "iam",
					NodeName:  "test-node",
					TokenName: "test-token",
				},
				evt,
				protocmp.Transform(),
				cmpopts.IgnoreMapEntries(func(key string, val any) bool {
					return key == "Time" || key == "ID" || key == "TokenExpires"
				}),
			))
		}, 5*time.Second, 5*time.Millisecond)
	})
}
