/*
Copyright 2015-2022 Gravitational, Inc.

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

package web

import (
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/modules"
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
		mockGetProxyServers: func() ([]types.Server, error) {
			var s types.ServerV2
			s.SetPublicAddrs([]string{"test-host:12345678"})

			return []types.Server{&s}, nil
		},
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
							types.InternalResourceIDLabel: utils.Strings{internalResourceID},
						},
					},
				}, nil
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
				require.Contains(t, script, "test-host")
				require.Contains(t, script, "12345678")
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
	} {
		t.Run(test.desc, func(t *testing.T) {
			script, err := getJoinScript(context.Background(), test.settings, m)
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
		mockGetProxyServers: func() ([]types.Server, error) {
			var s types.ServerV2
			s.SetPublicAddrs([]string{"test-host:12345678"})

			return []types.Server{&s}, nil
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

	// Test invalid app data.
	script, err := getJoinScript(context.Background(), badAppName, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	script, err = getJoinScript(context.Background(), badAppURI, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	// Test various 'good' cases.
	expectedOutputs := []string{
		testTokenID,
		"test-host",
		"12345678",
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
			script, err = getJoinScript(context.Background(), tc.settings, m)
			if tc.shouldError {
				require.NotNil(t, err)
				require.Equal(t, script, "")
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

	m := &mockedNodeAPIGetter{
		mockGetProxyServers: func() ([]types.Server, error) {
			var s types.ServerV2
			s.SetPublicAddrs([]string{"test-host:12345678"})

			return []types.Server{&s}, nil
		},
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
						types.InternalResourceIDLabel: utils.Strings{internalResourceID},
					},
					SuggestedAgentMatcherLabels: types.Labels{
						"env":     utils.Strings{"prod"},
						"product": utils.Strings{"*"},
						"os":      utils.Strings{"mac", "linux"},
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
				require.Contains(t, script, "test-host")
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
				require.Contains(t, script, "test-host")
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
			script, err := getJoinScript(context.Background(), test.settings, m)
			test.errAssert(t, err)
			if err != nil {
				require.Empty(t, script)
			}

			if test.extraAssertions != nil {
				t.Log(script)
				test.extraAssertions(script)
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
		mockGetProxyServers: func() ([]types.Server, error) {
			return []types.Server{
				&types.ServerV2{
					Spec: types.ServerSpecV2{PublicAddrs: []string{"test-host:12345678"}},
				},
			}, nil
		},
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
		getGravitationalTeleportLinkRegex := regexp.MustCompile(`https://get\.gravitational\.com/\${TELEPORT_PACKAGE_NAME}[-_]v?\${TELEPORT_VERSION}`)

		t.Run("oss", func(t *testing.T) {
			// Using the OSS Version, all the links must contain only teleport as package name.
			script, err := getJoinScript(context.Background(), scriptSettings{token: validToken}, m)
			require.NoError(t, err)

			matches := getGravitationalTeleportLinkRegex.FindAllString(script, -1)
			require.ElementsMatch(t, matches, []string{
				"https://get.gravitational.com/${TELEPORT_PACKAGE_NAME}-v${TELEPORT_VERSION}",
				"https://get.gravitational.com/${TELEPORT_PACKAGE_NAME}_${TELEPORT_VERSION}",
				"https://get.gravitational.com/${TELEPORT_PACKAGE_NAME}-${TELEPORT_VERSION}",
			})
			require.Contains(t, script, "TELEPORT_PACKAGE_NAME='teleport'")
			require.Contains(t, script, "TELEPORT_ARCHIVE_PATH='teleport'")
		})

		t.Run("ent", func(t *testing.T) {
			// Using the Enterprise Version, the package name must be teleport-ent
			modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
			script, err := getJoinScript(context.Background(), scriptSettings{token: validToken}, m)
			require.NoError(t, err)

			matches := getGravitationalTeleportLinkRegex.FindAllString(script, -1)
			require.ElementsMatch(t, matches, []string{
				"https://get.gravitational.com/${TELEPORT_PACKAGE_NAME}-v${TELEPORT_VERSION}",
				"https://get.gravitational.com/${TELEPORT_PACKAGE_NAME}_${TELEPORT_VERSION}",
				"https://get.gravitational.com/${TELEPORT_PACKAGE_NAME}-${TELEPORT_VERSION}",
			})
			require.Contains(t, script, "TELEPORT_PACKAGE_NAME='teleport-ent'")
			require.Contains(t, script, "TELEPORT_ARCHIVE_PATH='teleport-ent'")
		})
	})

	t.Run("using repo", func(t *testing.T) {
		t.Run("cloud and automatic upgrades", func(t *testing.T) {
			script, err := getJoinScript(context.Background(), scriptSettings{token: validToken, stableCloudChannelRepo: true}, m)
			require.NoError(t, err)
			// Uses the stable/cloud channel.
			require.Contains(t, script, "REPO_CHANNEL='stable/cloud'")
		})
		t.Run("cloud but automatic upgrades disabled", func(t *testing.T) {
			// Using the Enterprise Version, the package name must be teleport-ent
			modules.SetTestModules(t, &modules.TestModules{
				TestFeatures: modules.Features{
					Cloud:             true,
					AutomaticUpgrades: false,
				},
			})
			script, err := getJoinScript(context.Background(), scriptSettings{token: validToken, stableCloudChannelRepo: false}, m)
			require.NoError(t, err)
			// Setting an empty string, means it will use `stable/v<majorVersion>` later on.
			require.Contains(t, script, "REPO_CHANNEL=''")
		})
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
	mockGetProxyServers  func() ([]types.Server, error)
	mockGetClusterCACert func(ctx context.Context) (*proto.GetClusterCACertResponse, error)
	mockGetToken         func(ctx context.Context, token string) (types.ProvisionToken, error)
}

func (m *mockedNodeAPIGetter) GetProxies() ([]types.Server, error) {
	if m.mockGetProxyServers != nil {
		return m.mockGetProxyServers()
	}

	return nil, trace.NotImplemented("mockGetProxyServers not implemented")
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
