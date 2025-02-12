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

package web

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	libui "github.com/gravitational/teleport/lib/ui"
	utils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/ui"
)

func TestGenerateIAMTokenName(t *testing.T) {
	t.Parallel()
	rule1 := types.TokenRule{
		AWSAccount: "100000000000",
		AWSARN:     "arn:aws:iam:1",
	}

	rule1Name := "teleport-ui-iam-2218897454"

	// make sure the hash algorithm don't change accidentally
	hash1, err := generateIAMTokenName([]*types.TokenRule{&rule1})
	require.NoError(t, err)
	require.Equal(t, rule1Name, hash1)

	rule2 := types.TokenRule{
		AWSAccount: "200000000000",
		AWSARN:     "arn:aws:iam:b",
	}

	// make sure the order doesn't matter
	hash1, err = generateIAMTokenName([]*types.TokenRule{&rule1, &rule2})
	require.NoError(t, err)

	hash2, err := generateIAMTokenName([]*types.TokenRule{&rule2, &rule1})
	require.NoError(t, err)

	require.Equal(t, hash1, hash2)

	// generate different hashes for different rules
	hash1, err = generateIAMTokenName([]*types.TokenRule{&rule1})
	require.NoError(t, err)

	hash2, err = generateIAMTokenName([]*types.TokenRule{&rule2})
	require.NoError(t, err)

	require.NotEqual(t, hash1, hash2)
}

type tokenData struct {
	name   string
	roles  types.SystemRoles
	expiry time.Time
}

func TestGetTokens(t *testing.T) {
	t.Parallel()
	username := "test-user@example.com"
	ctx := context.Background()
	expiry := time.Now().UTC().Add(30 * time.Minute)

	staticUIToken := ui.JoinToken{
		ID:       "static-token",
		SafeName: "************",
		Roles:    types.SystemRoles{types.RoleNode},
		Expiry:   time.Unix(0, 0).UTC(),
		IsStatic: true,
		Method:   types.JoinMethodToken,
		Content:  "kind: token\nmetadata:\n  expires: \"1970-01-01T00:00:00Z\"\n  labels:\n    teleport.dev/origin: config-file\n  name: static-token\nspec:\n  join_method: token\n  roles:\n  - Node\nversion: v2\n",
	}

	tt := []struct {
		name             string
		tokenData        []tokenData
		expected         []ui.JoinToken
		noAccess         bool
		includeUserToken bool
	}{
		{
			name:      "no access",
			tokenData: []tokenData{},
			noAccess:  true,
			expected:  []ui.JoinToken{},
		},
		{
			name:      "only static tokens exist",
			tokenData: []tokenData{},
			expected: []ui.JoinToken{
				staticUIToken,
			},
		},
		{
			name:      "static and sign up tokens",
			tokenData: []tokenData{},
			expected: []ui.JoinToken{
				staticUIToken,
			},
			includeUserToken: true,
		},
		{
			name: "all tokens",
			tokenData: []tokenData{
				{
					name: "test-token",
					roles: types.SystemRoles{
						types.RoleNode,
					},
					expiry: expiry,
				},
				{
					name: "test-token-2",
					roles: types.SystemRoles{
						types.RoleNode,
						types.RoleDatabase,
					},
					expiry: expiry,
				},
				{
					name: "test-token-3-and-super-duper-long",
					roles: types.SystemRoles{
						types.RoleNode,
						types.RoleKube,
						types.RoleDatabase,
					},
					expiry: expiry,
				},
			},
			expected: []ui.JoinToken{
				{
					ID:       "test-token",
					SafeName: "**********",
					IsStatic: false,
					Expiry:   expiry,
					Roles: types.SystemRoles{
						types.RoleNode,
					},
					Method: types.JoinMethodToken,
				},
				{
					ID:       "test-token-2",
					SafeName: "************",
					IsStatic: false,
					Expiry:   expiry,
					Roles: types.SystemRoles{
						types.RoleNode,
						types.RoleDatabase,
					},
					Method: types.JoinMethodToken,
				},
				{
					ID:       "test-token-3-and-super-duper-long",
					SafeName: "************************uper-long",
					IsStatic: false,
					Expiry:   expiry,
					Roles: types.SystemRoles{
						types.RoleNode,
						types.RoleKube,
						types.RoleDatabase,
					},
					Method: types.JoinMethodToken,
				},
				staticUIToken,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			env := newWebPack(t, 1)
			proxy := env.proxies[0]
			pack := proxy.authPack(t, username, nil /* roles */)

			if tc.noAccess {
				noAccessRole, err := types.NewRole(services.RoleNameForUser("test-no-access@example.com"), types.RoleSpecV6{})
				require.NoError(t, err)
				noAccessPack := proxy.authPack(t, "test-no-access@example.com", []types.Role{noAccessRole})
				endpoint := noAccessPack.clt.Endpoint("webapi", "tokens")
				_, err = noAccessPack.clt.Get(ctx, endpoint, url.Values{})
				require.Error(t, err)
				return
			}

			if tc.includeUserToken {
				passwordToken, err := env.server.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
					Name: username,
					TTL:  defaults.MaxSignupTokenTTL,
					Type: authclient.UserTokenTypeResetPasswordInvite,
				})
				require.NoError(t, err)
				userToken, err := types.NewProvisionToken(passwordToken.GetName(), types.SystemRoles{types.RoleSignup}, passwordToken.Expiry())
				require.NoError(t, err)

				userUiToken := ui.JoinToken{
					ID:       userToken.GetName(),
					SafeName: userToken.GetSafeName(),
					IsStatic: false,
					Expiry:   userToken.Expiry(),
					Roles:    userToken.GetRoles(),
					Method:   types.JoinMethodToken,
				}
				tc.expected = append(tc.expected, userUiToken)
			}

			for _, td := range tc.tokenData {
				token, err := types.NewProvisionTokenFromSpec(td.name, td.expiry, types.ProvisionTokenSpecV2{
					Roles: td.roles,
				})
				require.NoError(t, err)
				err = env.server.Auth().CreateToken(ctx, token)
				require.NoError(t, err)
			}

			endpoint := pack.clt.Endpoint("webapi", "tokens")
			re, err := pack.clt.Get(ctx, endpoint, url.Values{})
			require.NoError(t, err)

			resp := GetTokensResponse{}
			require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
			require.Len(t, resp.Items, len(tc.expected))
			require.Empty(t, cmp.Diff(resp.Items, tc.expected, cmpopts.IgnoreFields(ui.JoinToken{}, "Content")))
		})
	}
}

func TestDeleteToken(t *testing.T) {
	ctx := context.Background()
	username := "test-user@example.com"
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, nil /* roles */)
	endpoint := pack.clt.Endpoint("webapi", "tokens")
	staticUIToken := ui.JoinToken{
		ID:       "static-token",
		SafeName: "************",
		Roles:    types.SystemRoles{types.RoleNode},
		Expiry:   time.Unix(0, 0).UTC(),
		IsStatic: true,
		Method:   types.JoinMethodToken,
		Content:  "kind: token\nmetadata:\n  expires: \"1970-01-01T00:00:00Z\"\n  labels:\n    teleport.dev/origin: config-file\n  name: static-token\nspec:\n  join_method: token\n  roles:\n  - Node\nversion: v2\n",
	}

	// create join token
	token, err := types.NewProvisionTokenFromSpec("my-token", time.Now().UTC().Add(30*time.Minute), types.ProvisionTokenSpecV2{
		Roles: types.SystemRoles{
			types.RoleNode,
			types.RoleDatabase,
		},
	})
	require.NoError(t, err)
	err = env.server.Auth().CreateToken(ctx, token)
	require.NoError(t, err)

	// create password reset token
	passwordToken, err := env.server.Auth().CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
		Name: username,
		TTL:  defaults.MaxSignupTokenTTL,
		Type: authclient.UserTokenTypeResetPasswordInvite,
	})
	require.NoError(t, err)
	userToken, err := types.NewProvisionToken(passwordToken.GetName(), types.SystemRoles{types.RoleSignup}, passwordToken.Expiry())
	require.NoError(t, err)

	// should have static token + a signup token now
	re, err := pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	resp := GetTokensResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 3 /* static + sign up + join */)

	// delete
	req, err := http.NewRequest("DELETE", endpoint, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", pack.session.Token))
	req.Header.Set(HeaderTokenName, userToken.GetName())
	_, err = pack.clt.RoundTrip(func() (*http.Response, error) {
		return pack.clt.HTTPClient().Do(req)
	})
	require.NoError(t, err)
	req.Header.Set(HeaderTokenName, token.GetName())
	_, err = pack.clt.RoundTrip(func() (*http.Response, error) {
		return pack.clt.HTTPClient().Do(req)
	})
	require.NoError(t, err)

	re, err = pack.clt.Get(ctx, endpoint, url.Values{})
	require.NoError(t, err)
	resp = GetTokensResponse{}
	require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
	require.Len(t, resp.Items, 1 /* only static again */)
	require.Empty(t, cmp.Diff(resp.Items, []ui.JoinToken{staticUIToken}, cmpopts.IgnoreFields(ui.JoinToken{}, "Content")))
}

func TestCreateTokenForDiscovery(t *testing.T) {
	ctx := context.Background()
	username := "test-user@example.com"
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	pack := proxy.authPack(t, username, nil /* roles */)

	match := func(resp nodeJoinToken, userLabels types.Labels) {
		if len(userLabels) > 0 {
			require.Empty(t, cmp.Diff([]libui.Label{{Name: "env"}, {Name: "teleport.internal/resource-id"}}, resp.SuggestedLabels, cmpopts.SortSlices(
				func(a, b libui.Label) bool {
					return a.Name < b.Name
				},
			), cmpopts.IgnoreFields(libui.Label{}, "Value")))
		} else {
			require.Empty(t, cmp.Diff([]libui.Label{{Name: "teleport.internal/resource-id"}}, resp.SuggestedLabels, cmpopts.IgnoreFields(libui.Label{}, "Value")))
		}
		require.NotEmpty(t, resp.ID)
		require.NotEmpty(t, resp.Expiry)
		require.Equal(t, types.JoinMethodToken, resp.Method)
	}

	tt := []struct {
		name string
		req  types.ProvisionTokenSpecV2
	}{
		{
			name: "with suggested labels",
			req: types.ProvisionTokenSpecV2{
				Roles:           []types.SystemRole{types.RoleNode},
				SuggestedLabels: types.Labels{"env": []string{"testing"}},
			},
		},
		{
			name: "without suggested labels",
			req: types.ProvisionTokenSpecV2{
				Roles:           []types.SystemRole{types.RoleNode},
				SuggestedLabels: nil,
			},
		},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("v1 %s", tc.name), func(t *testing.T) {
			endpointV1 := pack.clt.Endpoint("v1", "webapi", "token")
			re, err := pack.clt.PostJSON(ctx, endpointV1, tc.req)
			require.NoError(t, err)

			resp := nodeJoinToken{}
			require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
			match(resp, tc.req.SuggestedLabels)
		})

		t.Run(fmt.Sprintf("v2 %s", tc.name), func(t *testing.T) {
			endpointV2 := pack.clt.Endpoint("v2", "webapi", "token")
			re, err := pack.clt.PostJSON(ctx, endpointV2, tc.req)
			require.NoError(t, err)

			resp := nodeJoinToken{}
			require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
			match(resp, tc.req.SuggestedLabels)
		})
	}
}

func TestGenerateAzureTokenName(t *testing.T) {
	t.Parallel()
	rule1 := types.ProvisionTokenSpecV2Azure_Rule{
		Subscription: "abcd1234",
	}
	rule2 := types.ProvisionTokenSpecV2Azure_Rule{
		Subscription: "efgh5678",
	}

	t.Run("hash algorithm hasn't changed", func(t *testing.T) {
		rule1Name := "teleport-ui-azure-2091772181"
		hash1, err := generateAzureTokenName([]*types.ProvisionTokenSpecV2Azure_Rule{&rule1})
		require.NoError(t, err)
		require.Equal(t, rule1Name, hash1)
	})

	t.Run("order doesn't matter", func(t *testing.T) {
		hash1, err := generateAzureTokenName([]*types.ProvisionTokenSpecV2Azure_Rule{&rule1, &rule2})
		require.NoError(t, err)
		hash2, err := generateAzureTokenName([]*types.ProvisionTokenSpecV2Azure_Rule{&rule2, &rule1})
		require.NoError(t, err)
		require.Equal(t, hash1, hash2)
	})

	t.Run("different hashes for different rules", func(t *testing.T) {
		hash1, err := generateAzureTokenName([]*types.ProvisionTokenSpecV2Azure_Rule{&rule1})
		require.NoError(t, err)
		hash2, err := generateAzureTokenName([]*types.ProvisionTokenSpecV2Azure_Rule{&rule2})
		require.NoError(t, err)
		require.NotEqual(t, hash1, hash2)
	})

}

func TestSortRules(t *testing.T) {
	t.Parallel()
	tt := []struct {
		name     string
		rules    []*types.TokenRule
		expected []*types.TokenRule
	}{
		{
			name: "different account ID, no ARN",
			rules: []*types.TokenRule{
				{AWSAccount: "200000000000"},
				{AWSAccount: "100000000000"},
			},
			expected: []*types.TokenRule{
				{AWSAccount: "100000000000"},
				{AWSAccount: "200000000000"},
			},
		},
		{
			name: "different account ID, no ARN, already ordered",
			rules: []*types.TokenRule{
				{AWSAccount: "100000000000"},
				{AWSAccount: "200000000000"},
			},
			expected: []*types.TokenRule{
				{AWSAccount: "100000000000"},
				{AWSAccount: "200000000000"},
			},
		},
		{
			name: "different account ID, with ARN",
			rules: []*types.TokenRule{
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
			expected: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
		},
		{
			name: "different account ID, with ARN, already ordered",
			rules: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
			expected: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
		},
		{
			name: "same account ID, different ARN, already ordered",
			rules: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
			expected: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
		},
		{
			name: "same account ID, different ARN",
			rules: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:a",
				},
			},
			expected: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
			},
		},
		{
			name: "multiple account ID and ARNs",
			rules: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000001",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:b",
				},

				{
					AWSAccount: "200000000001",
					AWSARN:     "arn:aws:iam:z",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "300000000000",
					AWSARN:     "arn:aws:iam:a",
				},
			},
			expected: []*types.TokenRule{
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "100000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:a",
				},
				{
					AWSAccount: "200000000000",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000001",
					AWSARN:     "arn:aws:iam:b",
				},
				{
					AWSAccount: "200000000001",
					AWSARN:     "arn:aws:iam:z",
				},
				{
					AWSAccount: "300000000000",
					AWSARN:     "arn:aws:iam:a",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			sortRules(tc.rules)
			require.Equal(t, tc.expected, tc.rules)
		})
	}
}

func TestSortAzureRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		rules    []*types.ProvisionTokenSpecV2Azure_Rule
		expected []*types.ProvisionTokenSpecV2Azure_Rule
	}{
		{
			name: "unordered",
			rules: []*types.ProvisionTokenSpecV2Azure_Rule{
				{Subscription: "200000000000"},
				{Subscription: "300000000000"},
				{Subscription: "100000000000"},
			},
			expected: []*types.ProvisionTokenSpecV2Azure_Rule{
				{Subscription: "100000000000"},
				{Subscription: "200000000000"},
				{Subscription: "300000000000"},
			},
		},
		{
			name: "already ordered",
			rules: []*types.ProvisionTokenSpecV2Azure_Rule{
				{Subscription: "100000000000"},
				{Subscription: "200000000000"},
				{Subscription: "300000000000"},
			},
			expected: []*types.ProvisionTokenSpecV2Azure_Rule{
				{Subscription: "100000000000"},
				{Subscription: "200000000000"},
				{Subscription: "300000000000"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sortAzureRules(tc.rules)
			require.Equal(t, tc.expected, tc.rules)
		})
	}
}

func toHex(s string) string { return hex.EncodeToString([]byte(s)) }

func TestGetNodeJoinScript(t *testing.T) {
	validToken := "f18da1c9f6630a51e8daf121e7451daa"
	validIAMToken := "valid-iam-token"
	internalResourceID := "967d38ff-7a61-4f42-bd2d-c61965b44db0"

	m := &mockedNodeAPIGetter{
		mockGetClusterCACert: func(context.Context) (*proto.GetClusterCACertResponse, error) {
			fakeBytes := []byte(fixtures.SigningCertPEM)
			return &proto.GetClusterCACertResponse{TLSCA: fakeBytes}, nil
		},
		mockGetToken: func(_ context.Context, token string) (types.ProvisionToken, error) {
			if token == validToken || token == validIAMToken {
				return &types.ProvisionTokenV2{
					Metadata: types.Metadata{
						Name: token,
					},
					Spec: types.ProvisionTokenSpecV2{
						SuggestedLabels: types.Labels{
							types.InternalResourceIDLabel: apiutils.Strings{internalResourceID},
						},
					},
				}, nil
			}
			return nil, trace.NotFound("token does not exist")
		},
	}

	h := newAutoupdateTestHandler(t, nil, nil, nil)
	hostname, port, err := utils.SplitHostPort(h.PublicProxyAddr())
	require.NoError(t, err)

	for _, test := range []struct {
		desc            string
		settings        scriptSettings
		errAssert       require.ErrorAssertionFunc
		extraAssertions func(script string)
	}{
		{
			desc:      "zero value",
			settings:  scriptSettings{},
			errAssert: require.Error,
		},
		{
			desc:      "short token length",
			settings:  scriptSettings{token: toHex("f18da1c9f6630a51e8daf121e7451d")},
			errAssert: require.Error,
		},
		{
			desc:      "valid length but does not exist",
			settings:  scriptSettings{token: toHex("xxxxxxx9f6630a51e8daf121exxxxxxx")},
			errAssert: require.Error,
		},
		{
			desc:      "valid",
			settings:  scriptSettings{token: validToken},
			errAssert: require.NoError,
			extraAssertions: func(script string) {
				require.Contains(t, script, validToken)
				require.Contains(t, script, hostname)
				require.Contains(t, script, port)
				require.Contains(t, script, "sha256:")
				require.NotContains(t, script, "JOIN_METHOD='iam'")
			},
		},
		{
			desc: "invalid IAM",
			settings: scriptSettings{
				token:      toHex("invalid-iam-token"),
				joinMethod: string(types.JoinMethodIAM),
			},
			errAssert: require.Error,
		},
		{
			desc: "valid iam",
			settings: scriptSettings{
				token:      validIAMToken,
				joinMethod: string(types.JoinMethodIAM),
			},
			errAssert: require.NoError,
			extraAssertions: func(script string) {
				require.Contains(t, script, "JOIN_METHOD='iam'")
			},
		},
		{
			desc:      "internal resourceid label",
			settings:  scriptSettings{token: validToken},
			errAssert: require.NoError,
			extraAssertions: func(script string) {
				require.Contains(t, script, "--labels ")
				require.Contains(t, script, fmt.Sprintf("%s=%s", types.InternalResourceIDLabel, internalResourceID))
			},
		},
		{
			desc:      "app server labels",
			settings:  scriptSettings{token: validToken, appInstallMode: true, appName: "app-name", appURI: "app-uri"},
			errAssert: require.NoError,
			extraAssertions: func(script string) {
				require.Contains(t, script, `APP_NAME='app-name'`)
				require.Contains(t, script, `APP_URI='app-uri'`)
				require.Contains(t, script, `public_addr`)
				require.Contains(t, script, fmt.Sprintf("    labels:\n      %s: %s", types.InternalResourceIDLabel, internalResourceID))
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			script, err := h.getJoinScript(context.Background(), test.settings, m)
			test.errAssert(t, err)
			if err != nil {
				require.Empty(t, script)
			}

			if test.extraAssertions != nil {
				test.extraAssertions(script)
			}
		})
	}
}

type autoupdateAccessPointMock struct {
	authclient.ProxyAccessPoint
	mock.Mock
}

func (a *autoupdateAccessPointMock) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdatev1pb.AutoUpdateAgentRollout, error) {
	args := a.Called(ctx)
	return args.Get(0).(*autoupdatev1pb.AutoUpdateAgentRollout), args.Error(1)
}

func newAutoupdateTestHandler(t *testing.T, testModules *modules.TestModules, channels automaticupgrades.Channels, rollout *autoupdatev1pb.AutoUpdateAgentRollout) *Handler {
	addr := fmt.Sprintf("proxy-%d.example.com:%d", rand.Int(), rand.IntN(65535))

	if channels == nil {
		channels = automaticupgrades.Channels{}
	}
	require.NoError(t, channels.CheckAndSetDefaults())

	ap := &autoupdateAccessPointMock{}
	if rollout == nil {
		ap.On("GetAutoUpdateAgentRollout", mock.Anything).Return(rollout, trace.NotFound("rollout does not exist"))
	} else {
		ap.On("GetAutoUpdateAgentRollout", mock.Anything).Return(rollout, nil)
	}

	if testModules == nil {
		testModules = &modules.TestModules{
			TestBuildType: modules.BuildCommunity,
		}
	}
	modules.SetTestModules(t, testModules)
	h := &Handler{
		clusterFeatures: *testModules.Features().ToProto(),
		cfg: Config{
			AutomaticUpgradesChannels: channels,
			AccessPoint:               ap,
			PublicProxyAddr:           addr,
		},
		logger: utils.NewSlogLoggerForTests(),
	}
	h.PublicProxyAddr()
	return h
}

func TestGetAppJoinScript(t *testing.T) {
	testTokenID := "f18da1c9f6630a51e8daf121e7451daa"
	m := &mockedNodeAPIGetter{
		mockGetToken: func(_ context.Context, token string) (types.ProvisionToken, error) {
			if token == testTokenID {
				return &types.ProvisionTokenV2{
					Metadata: types.Metadata{
						Name: token,
					},
				}, nil
			}
			return nil, trace.NotFound("token does not exist")
		},
		mockGetClusterCACert: func(context.Context) (*proto.GetClusterCACertResponse, error) {
			fakeBytes := []byte(fixtures.SigningCertPEM)
			return &proto.GetClusterCACertResponse{TLSCA: fakeBytes}, nil
		},
	}
	badAppName := scriptSettings{
		token:          testTokenID,
		appInstallMode: true,
		appName:        "",
		appURI:         "127.0.0.1:0",
	}

	badAppURI := scriptSettings{
		token:          testTokenID,
		appInstallMode: true,
		appName:        "test-app",
		appURI:         "",
	}

	h := newAutoupdateTestHandler(t, nil, nil, nil)
	hostname, port, err := utils.SplitHostPort(h.PublicProxyAddr())
	require.NoError(t, err)

	// Test invalid app data.
	script, err := h.getJoinScript(context.Background(), badAppName, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	script, err = h.getJoinScript(context.Background(), badAppURI, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	// Test various 'good' cases.
	expectedOutputs := []string{
		testTokenID,
		hostname,
		port,
		"sha256:",
	}

	tests := []struct {
		desc        string
		settings    scriptSettings
		shouldError bool
		outputs     []string
	}{
		{
			desc: "node only join mode with other values not provided",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: false,
			},
			outputs: expectedOutputs,
		},
		{
			desc: "node only join mode with values set to blank",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: false,
				appName:        "",
				appURI:         "",
			},
			outputs: expectedOutputs,
		},
		{
			desc: "all settings set correctly",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "test-app123",
				appURI:         "http://localhost:12345/landing page__",
			},
			outputs: append(
				expectedOutputs,
				"test-app123",
				"http://localhost:12345",
			),
		},
		{
			desc: "all settings set correctly with a longer app name",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "this-is-a-much-longer-app-name-being-used-for-testing",
				appURI:         "https://1.2.3.4:54321",
			},
			outputs: append(
				expectedOutputs,
				"this-is-a-much-longer-app-name-being-used-for-testing",
				"https://1.2.3.4:54321",
			),
		},
		{
			desc: "app name containing double quotes is rejected",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        `ab"cd`,
				appURI:         "https://1.2.3.4:54321",
			},
			shouldError: true,
		},
		{
			desc: "app URI containing double quotes is rejected",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "abcd",
				appURI:         `https://1.2.3.4:54321/x"y"z`,
			},
			shouldError: true,
		},
		{
			desc: "app name containing a backtick is rejected",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "ab`whoami`cd",
				appURI:         "https://1.2.3.4:54321",
			},
			shouldError: true,
		},
		{
			desc: "app URI containing a backtick is rejected",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "abcd",
				appURI:         "https://1.2.3.4:54321/`whoami`",
			},
			shouldError: true,
		},
		{
			desc: "app name containing a dollar sign is rejected",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "ab$HOME",
				appURI:         "https://1.2.3.4:54321",
			},
			shouldError: true,
		},
		{
			desc: "app URI containing a dollar sign is rejected",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "abcd",
				appURI:         "https://1.2.3.4:54321/$HOME",
			},
			shouldError: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			script, err = h.getJoinScript(context.Background(), tc.settings, m)
			if tc.shouldError {
				require.Error(t, err)
				require.Empty(t, script)
			} else {
				require.NoError(t, err)
				for _, output := range tc.outputs {
					require.Contains(t, script, output)
				}
			}
		})
	}
}

func TestGetDatabaseJoinScript(t *testing.T) {
	validToken := "f18da1c9f6630a51e8daf121e7451daa"
	emptySuggestedAgentMatcherLabelsToken := "f18da1c9f6630a51e8daf121e7451000"
	internalResourceID := "967d38ff-7a61-4f42-bd2d-c61965b44db0"

	h := newAutoupdateTestHandler(t, nil, nil, nil)
	hostname, port, err := utils.SplitHostPort(h.PublicProxyAddr())
	require.NoError(t, err)

	m := &mockedNodeAPIGetter{
		mockGetClusterCACert: func(context.Context) (*proto.GetClusterCACertResponse, error) {
			fakeBytes := []byte(fixtures.SigningCertPEM)
			return &proto.GetClusterCACertResponse{TLSCA: fakeBytes}, nil
		},
		mockGetToken: func(_ context.Context, token string) (types.ProvisionToken, error) {
			provisionToken := &types.ProvisionTokenV2{
				Metadata: types.Metadata{
					Name: token,
				},
				Spec: types.ProvisionTokenSpecV2{
					SuggestedLabels: types.Labels{
						types.InternalResourceIDLabel: apiutils.Strings{internalResourceID},
					},
					SuggestedAgentMatcherLabels: types.Labels{
						"env":     apiutils.Strings{"prod"},
						"product": apiutils.Strings{"*"},
						"os":      apiutils.Strings{"mac", "linux"},
					},
				},
			}
			if token == validToken {
				return provisionToken, nil
			}
			if token == emptySuggestedAgentMatcherLabelsToken {
				provisionToken.Spec.SuggestedAgentMatcherLabels = types.Labels{}
				return provisionToken, nil
			}
			return nil, trace.NotFound("token does not exist")
		},
	}

	for _, test := range []struct {
		desc            string
		settings        scriptSettings
		errAssert       require.ErrorAssertionFunc
		extraAssertions func(script string)
	}{
		{
			desc: "two installation methods",
			settings: scriptSettings{
				token:               validToken,
				databaseInstallMode: true,
				appInstallMode:      true,
			},
			errAssert: require.Error,
		},
		{
			desc: "valid",
			settings: scriptSettings{
				databaseInstallMode: true,
				token:               validToken,
			},
			errAssert: require.NoError,
			extraAssertions: func(script string) {
				require.Contains(t, script, validToken)
				require.Contains(t, script, hostname)
				require.Contains(t, script, port)
				require.Contains(t, script, "sha256:")
				require.Contains(t, script, "--labels ")
				require.Contains(t, script, fmt.Sprintf("%s=%s", types.InternalResourceIDLabel, internalResourceID))
				require.Contains(t, script, `
db_service:
  enabled: "yes"
  resources:
    - labels:
        env: prod
        os:
          - mac
          - linux
        product: '*'
`)
			},
		},
		{
			desc: "empty suggestedAgentMatcherLabels",
			settings: scriptSettings{
				databaseInstallMode: true,
				token:               emptySuggestedAgentMatcherLabelsToken,
			},
			errAssert: require.NoError,
			extraAssertions: func(script string) {
				require.Contains(t, script, emptySuggestedAgentMatcherLabelsToken)
				require.Contains(t, script, hostname)
				require.Contains(t, script, port)
				require.Contains(t, script, "sha256:")
				require.Contains(t, script, "--labels ")
				require.Contains(t, script, fmt.Sprintf("%s=%s", types.InternalResourceIDLabel, internalResourceID))
				require.Contains(t, script, `
db_service:
  enabled: "yes"
  resources:
    - labels:
        {}
`)
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			script, err := h.getJoinScript(context.Background(), test.settings, m)
			test.errAssert(t, err)
			if err != nil {
				require.Empty(t, script)
			}

			if test.extraAssertions != nil {
				test.extraAssertions(script)
			}
		})
	}
}

func TestGetDiscoveryJoinScript(t *testing.T) {
	const validToken = "f18da1c9f6630a51e8daf121e7451daa"
	h := newAutoupdateTestHandler(t, nil, nil, nil)
	hostname, port, err := utils.SplitHostPort(h.PublicProxyAddr())
	require.NoError(t, err)

	m := &mockedNodeAPIGetter{
		mockGetClusterCACert: func(context.Context) (*proto.GetClusterCACertResponse, error) {
			fakeBytes := []byte(fixtures.SigningCertPEM)
			return &proto.GetClusterCACertResponse{TLSCA: fakeBytes}, nil
		},
		mockGetToken: func(_ context.Context, token string) (types.ProvisionToken, error) {
			provisionToken := &types.ProvisionTokenV2{
				Metadata: types.Metadata{
					Name: token,
				},
				Spec: types.ProvisionTokenSpecV2{},
			}
			if token == validToken {
				return provisionToken, nil
			}
			return nil, trace.NotFound("token does not exist")
		},
	}

	for _, test := range []struct {
		desc            string
		settings        scriptSettings
		errAssert       require.ErrorAssertionFunc
		extraAssertions func(t *testing.T, script string)
	}{
		{
			desc: "valid",
			settings: scriptSettings{
				discoveryInstallMode: true,
				discoveryGroup:       "my-group",
				token:                validToken,
			},
			errAssert: require.NoError,
			extraAssertions: func(t *testing.T, script string) {
				require.Contains(t, script, validToken)
				require.Contains(t, script, hostname)
				require.Contains(t, script, port)
				require.Contains(t, script, "sha256:")
				require.Contains(t, script, "--labels ")
				require.Contains(t, script, `
discovery_service:
  enabled: "yes"
  discovery_group: "my-group"`)
			},
		},
		{
			desc: "fails when discovery group is not defined",
			settings: scriptSettings{
				discoveryInstallMode: true,
				token:                validToken,
			},
			errAssert: require.Error,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			script, err := h.getJoinScript(context.Background(), test.settings, m)
			test.errAssert(t, err)
			if err != nil {
				require.Empty(t, script)
			}

			if test.extraAssertions != nil {
				test.extraAssertions(t, script)
			}
		})
	}
}

func TestIsSameRuleSet(t *testing.T) {
	tt := []struct {
		name     string
		r1       []*types.TokenRule
		r2       []*types.TokenRule
		expected bool
	}{
		{
			name:     "empty slice",
			expected: true,
		},
		{
			name: "simple identical rules",
			r1: []*types.TokenRule{
				{
					AWSAccount: "123123123123",
				},
			},
			r2: []*types.TokenRule{
				{
					AWSAccount: "123123123123",
				},
			},
			expected: true,
		},
		{
			name: "different rules",
			r1: []*types.TokenRule{
				{
					AWSAccount: "123123123123",
				},
			},
			r2: []*types.TokenRule{
				{
					AWSAccount: "111111111111",
				},
			},
			expected: false,
		},
		{
			name: "same rules in different order",
			r1: []*types.TokenRule{
				{
					AWSAccount: "123123123123",
				},
				{
					AWSAccount: "222222222222",
				},
				{
					AWSAccount: "111111111111",
					AWSARN:     "arn:*",
				},
			},
			r2: []*types.TokenRule{
				{
					AWSAccount: "222222222222",
				},
				{
					AWSAccount: "111111111111",
					AWSARN:     "arn:*",
				},
				{
					AWSAccount: "123123123123",
				},
			},
			expected: true,
		},
		{
			name: "almost the same rules",
			r1: []*types.TokenRule{
				{
					AWSAccount: "123123123123",
				},
				{
					AWSAccount: "222222222222",
				},
				{
					AWSAccount: "111111111111",
					AWSARN:     "arn:*",
				},
			},
			r2: []*types.TokenRule{
				{
					AWSAccount: "123123123123",
				},
				{
					AWSAccount: "222222222222",
				},
				{
					AWSAccount: "111111111111",
					AWSARN:     "arn:",
				},
			},
			expected: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isSameRuleSet(tc.r1, tc.r2))
		})
	}
}

func TestJoinScript(t *testing.T) {
	validToken := "f18da1c9f6630a51e8daf121e7451daa"

	m := &mockedNodeAPIGetter{
		mockGetClusterCACert: func(context.Context) (*proto.GetClusterCACertResponse, error) {
			fakeBytes := []byte(fixtures.SigningCertPEM)
			return &proto.GetClusterCACertResponse{TLSCA: fakeBytes}, nil
		},
		mockGetToken: func(_ context.Context, token string) (types.ProvisionToken, error) {
			return &types.ProvisionTokenV2{
				Metadata: types.Metadata{
					Name: token,
				},
			}, nil
		},
	}

	t.Run("direct download links", func(t *testing.T) {
		getGravitationalTeleportLinkRegex := regexp.MustCompile(`https://cdn\.teleport\.dev/\${TELEPORT_PACKAGE_NAME}[-_]v?\${TELEPORT_VERSION}`)

		t.Run("oss", func(t *testing.T) {
			h := newAutoupdateTestHandler(t, nil, nil, nil)
			// Using the OSS Version, all the links must contain only teleport as package name.
			script, err := h.getJoinScript(context.Background(), scriptSettings{token: validToken}, m)
			require.NoError(t, err)

			matches := getGravitationalTeleportLinkRegex.FindAllString(script, -1)
			require.ElementsMatch(t, matches, []string{
				"https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}-v${TELEPORT_VERSION}",
				"https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}_${TELEPORT_VERSION}",
				"https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}-${TELEPORT_VERSION}",
			})
			require.Contains(t, script, "TELEPORT_PACKAGE_NAME='teleport'")
			require.Contains(t, script, "TELEPORT_ARCHIVE_PATH='teleport'")
		})

		t.Run("ent", func(t *testing.T) {
			// Using the Enterprise Version, the package name must be teleport-ent
			h := newAutoupdateTestHandler(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise}, nil, nil)
			script, err := h.getJoinScript(context.Background(), scriptSettings{token: validToken}, m)
			require.NoError(t, err)

			matches := getGravitationalTeleportLinkRegex.FindAllString(script, -1)
			require.ElementsMatch(t, matches, []string{
				"https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}-v${TELEPORT_VERSION}",
				"https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}_${TELEPORT_VERSION}",
				"https://cdn.teleport.dev/${TELEPORT_PACKAGE_NAME}-${TELEPORT_VERSION}",
			})
			require.Contains(t, script, "TELEPORT_PACKAGE_NAME='teleport-ent'")
			require.Contains(t, script, "TELEPORT_ARCHIVE_PATH='teleport-ent'")
		})
	})

	t.Run("using repo", func(t *testing.T) {
		t.Run("installUpdater is true", func(t *testing.T) {
			currentStableCloudVersion := "1.2.3"
			h := newAutoupdateTestHandler(t,
				&modules.TestModules{TestFeatures: modules.Features{Cloud: true, AutomaticUpgrades: true}},
				automaticupgrades.Channels{
					automaticupgrades.DefaultChannelName: &automaticupgrades.Channel{StaticVersion: currentStableCloudVersion},
				}, nil)

			script, err := h.getJoinScript(context.Background(), scriptSettings{token: validToken}, m)
			require.NoError(t, err)

			// list of packages must include the updater
			require.Contains(t, script, ""+
				"    PACKAGE_LIST=${TELEPORT_PACKAGE_PIN_VERSION}\n"+
				"    # (warning): This expression is constant. Did you forget the $ on a variable?\n"+
				"    # Disabling the warning above because expression is templated.\n"+
				"    # shellcheck disable=SC2050\n"+
				"    if is_using_systemd && [[ \"true\" == \"true\" ]]; then\n"+
				"        # Teleport Updater requires systemd.\n"+
				"        PACKAGE_LIST+=\" ${TELEPORT_UPDATER_PIN_VERSION}\"\n"+
				"    fi\n",
			)
			// Repo channel is stable/cloud
			require.Contains(t, script, "REPO_CHANNEL='stable/cloud'")
			// TELEPORT_VERSION is the one provided by https://updates.releases.teleport.dev/v1/stable/cloud/version
			require.Contains(t, script, fmt.Sprintf("TELEPORT_VERSION='%s'", currentStableCloudVersion))
		})
		t.Run("installUpdater is false", func(t *testing.T) {
			h := newAutoupdateTestHandler(t, nil, nil, nil)
			script, err := h.getJoinScript(context.Background(), scriptSettings{token: validToken}, m)
			require.NoError(t, err)
			require.Contains(t, script, ""+
				"    PACKAGE_LIST=${TELEPORT_PACKAGE_PIN_VERSION}\n"+
				"    # (warning): This expression is constant. Did you forget the $ on a variable?\n"+
				"    # Disabling the warning above because expression is templated.\n"+
				"    # shellcheck disable=SC2050\n"+
				"    if is_using_systemd && [[ \"false\" == \"true\" ]]; then\n"+
				"        # Teleport Updater requires systemd.\n"+
				"        PACKAGE_LIST+=\" ${TELEPORT_UPDATER_PIN_VERSION}\"\n"+
				"    fi\n",
			)
			// Default based on current version is used instead
			require.Contains(t, script, "REPO_CHANNEL=''")
			// Current version must be used
			require.Contains(t, script, fmt.Sprintf("TELEPORT_VERSION='%s'", teleport.Version))
		})
	})
}

func TestAutomaticUpgrades(t *testing.T) {
	t.Run("cloud and automatic upgrades enabled", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestFeatures: modules.Features{
				Cloud:             true,
				AutomaticUpgrades: true,
			},
		})

		got := automaticUpgrades(*modules.GetModules().Features().ToProto())
		require.True(t, got)
	})
	t.Run("cloud but automatic upgrades disabled", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestFeatures: modules.Features{
				Cloud:             true,
				AutomaticUpgrades: false,
			},
		})

		got := automaticUpgrades(*modules.GetModules().Features().ToProto())
		require.False(t, got)
	})

	t.Run("automatic upgrades enabled but is not cloud", func(t *testing.T) {
		modules.SetTestModules(t, &modules.TestModules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Cloud:             false,
				AutomaticUpgrades: true,
			},
		})

		got := automaticUpgrades(*modules.GetModules().Features().ToProto())
		require.False(t, got)
	})
}

func TestIsSameAzureRuleSet(t *testing.T) {
	tests := []struct {
		name     string
		r1       []*types.ProvisionTokenSpecV2Azure_Rule
		r2       []*types.ProvisionTokenSpecV2Azure_Rule
		expected bool
	}{
		{
			name:     "empty slice",
			expected: true,
		},
		{
			name: "simple identical rules",
			r1: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "123123123123",
				},
			},
			r2: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "123123123123",
				},
			},
			expected: true,
		},
		{
			name: "different rules",
			r1: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "123123123123",
				},
			},
			r2: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "456456456456",
				},
			},
			expected: false,
		},
		{
			name: "same rules in different order",
			r1: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "456456456456",
				},
				{
					Subscription: "123123123123",
				},
			},
			r2: []*types.ProvisionTokenSpecV2Azure_Rule{
				{
					Subscription: "123123123123",
				},
				{
					Subscription: "456456456456",
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, isSameAzureRuleSet(tc.r1, tc.r2))
		})
	}
}

type mockedNodeAPIGetter struct {
	mockGetClusterCACert func(ctx context.Context) (*proto.GetClusterCACertResponse, error)
	mockGetToken         func(ctx context.Context, token string) (types.ProvisionToken, error)
}

func (m *mockedNodeAPIGetter) GetClusterCACert(ctx context.Context) (*proto.GetClusterCACertResponse, error) {
	if m.mockGetClusterCACert != nil {
		return m.mockGetClusterCACert(ctx)
	}

	return nil, trace.NotImplemented("mockGetClusterCACert not implemented")
}

func (m *mockedNodeAPIGetter) GetToken(ctx context.Context, token string) (types.ProvisionToken, error) {
	if m.mockGetToken != nil {
		return m.mockGetToken(ctx, token)
	}
	return nil, trace.NotImplemented("mockGetToken not implemented")
}
