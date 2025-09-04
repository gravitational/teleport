// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package plugin

import (
	"context"
	"log/slog"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
)

func TestAWSICUserFilters(t *testing.T) {
	testCases := []struct {
		name            string
		labelValues     []string
		originValues    []string
		expectedError   require.ErrorAssertionFunc
		expectedFilters []*types.AWSICUserSyncFilter
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:          "single",
			labelValues:   []string{"a=alpha,b=bravo,c=charlie"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
			},
		},
		{
			name: "multiple label filters",
			labelValues: []string{
				"a=alpha,b=bravo,c=charlie",
				"a=aardvark,b=a buzzing thing,c=big blue wobbly thing",
			},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
				{Labels: map[string]string{"a": "aardvark", "b": "a buzzing thing", "c": "big blue wobbly thing"}},
			},
		},
		{
			name:          "origin only",
			originValues:  []string{types.OriginOkta, types.OriginEntraID},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{types.OriginLabel: types.OriginEntraID}},
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
		},
		{
			name: "complex",
			labelValues: []string{
				"a=alpha,b=bravo,c=charlie",
				"a=aardvark,b=a buzzing thing,c=big blue wobbly thing",
			},
			originValues:  []string{types.OriginOkta, types.OriginEntraID},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICUserSyncFilter{
				{Labels: map[string]string{"a": "alpha", "b": "bravo", "c": "charlie"}},
				{Labels: map[string]string{"a": "aardvark", "b": "a buzzing thing", "c": "big blue wobbly thing"}},
				{Labels: map[string]string{types.OriginLabel: types.OriginEntraID}},
				{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
			},
		},
		{
			name:          "malformed label spec is an error",
			labelValues:   []string{"a=alpha,potato,c=charlie"},
			expectedError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICInstallArgs{
				userLabels:  test.labelValues,
				userOrigins: test.originValues,
			}

			actualFilters, err := cliArgs.parseUserFilters()
			test.expectedError(t, err)
			require.ElementsMatch(t, test.expectedFilters, actualFilters)
		})
	}
}

func TestAWSICGroupFilters(t *testing.T) {
	testCases := []struct {
		name            string
		nameValues      []string
		expectedError   require.ErrorAssertionFunc
		expectedFilters []*types.AWSICResourceFilter
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:          "multiple",
			nameValues:    []string{"alpha", "bravo", "charlie"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
			},
		},
		{
			name:          "malformed regex is an error",
			nameValues:    []string{"alpha", "^[)$", "charlie"},
			expectedError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICInstallArgs{
				groupNameFilters: test.nameValues,
			}

			actualFilters, err := cliArgs.parseGroupFilters()
			test.expectedError(t, err)
			require.ElementsMatch(t, test.expectedFilters, actualFilters)
		})
	}
}

func TestAWSICAccountFilters(t *testing.T) {
	testCases := []struct {
		name            string
		nameValues      []string
		idValues        []string
		expectedError   require.ErrorAssertionFunc
		expectedFilters []*types.AWSICResourceFilter
	}{
		{
			name:          "empty",
			expectedError: require.NoError,
		},
		{
			name:          "names only",
			nameValues:    []string{"alpha", "bravo", "charlie"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
			},
		},
		{
			name:          "ids only",
			idValues:      []string{"0123456789", "9876543210"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_Id{Id: "0123456789"}},
				{Include: &types.AWSICResourceFilter_Id{Id: "9876543210"}},
			},
		},
		{
			name:          "complex",
			nameValues:    []string{"alpha", "bravo", "charlie"},
			idValues:      []string{"0123456789", "9876543210"},
			expectedError: require.NoError,
			expectedFilters: []*types.AWSICResourceFilter{
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "alpha"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "bravo"}},
				{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: "charlie"}},
				{Include: &types.AWSICResourceFilter_Id{Id: "0123456789"}},
				{Include: &types.AWSICResourceFilter_Id{Id: "9876543210"}},
			},
		},
		{
			name:          "malformed regex is an error",
			nameValues:    []string{"alpha", "^[)$", "charlie"},
			expectedError: require.Error,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICInstallArgs{
				accountNameFilters: test.nameValues,
				accountIDFilters:   test.idValues,
			}

			actualFilters, err := cliArgs.parseAccountFilters()
			test.expectedError(t, err)
			require.ElementsMatch(t, test.expectedFilters, actualFilters)
		})
	}
}

func TestSCIMBaseURLValidation(t *testing.T) {
	ctx := context.Background()

	requireURL := func(expectedURL string) require.ValueAssertionFunc {
		return func(subtestT require.TestingT, value any, _ ...any) {
			actualURL, ok := value.(*url.URL)
			require.True(subtestT, ok, "Expected value to be an *URL, got %T instead", value)
			require.Equal(subtestT, expectedURL, actualURL.String())
		}
	}

	testCases := []struct {
		name        string
		suppliedURL string
		forceURL    bool
		expectError require.ErrorAssertionFunc
		expectValue require.ValueAssertionFunc
	}{
		{
			name:        "valid url",
			suppliedURL: "https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2",
			expectError: require.NoError,
			expectValue: requireURL("https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2"),
		},
		{
			name:        "fragments are stripped",
			suppliedURL: "https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2#spurious-fragment",
			expectError: require.NoError,
			expectValue: requireURL("https://scim.us-east-1.amazonaws.com/f3v9c6bc2ca-b104-4571-b669-f2eba522efe8/scim/v2"),
		},
		{
			name:        "invalid AWS SCIM Base URLs are an error",
			suppliedURL: "https://scim.example.com/v2",
			expectError: require.Error,
		},
		{
			name:        "invalid AWS SCIM Base URL can be forced",
			suppliedURL: "https://scim.example.com/v2",
			forceURL:    true,
			expectValue: requireURL("https://scim.example.com/v2"),
			expectError: require.NoError,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := awsICInstallArgs{
				scimURL:      mustParseURL(test.suppliedURL),
				forceSCIMURL: test.forceURL,
			}

			err := cliArgs.validateSCIMBaseURL(ctx, slog.Default().With("test", t.Name()))
			test.expectError(t, err)
			if test.expectValue != nil {
				test.expectValue(t, cliArgs.scimURL)
			}
		})
	}
}

func TestUseSystemCredentialsInput(t *testing.T) {
	testCases := []struct {
		name                string
		useSystemCredential bool
		assumeRoleARN       string
		expectError         require.ErrorAssertionFunc
	}{
		{
			name:                "valid system credential config",
			useSystemCredential: true,
			assumeRoleARN:       "arn:aws:iam::026000000023:role/assume1",
			expectError:         require.NoError,
		},
		{
			name:                "no useSystemCredential",
			useSystemCredential: false,
			assumeRoleARN:       "",
			expectError:         require.Error,
		},
		{
			name:                "useSystemCredential without assumeRoleARN",
			useSystemCredential: true,
			assumeRoleARN:       "",
			expectError:         require.Error,
		},
		{
			name:                "useSystemCredential with invalid assumeRoleARN",
			useSystemCredential: true,
			assumeRoleARN:       "example-credential",
			expectError:         require.Error,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cliArgs := awsICInstallArgs{
				useSystemCredentials: tc.useSystemCredential,
				assumeRoleARN:        tc.assumeRoleARN,
			}

			err := cliArgs.validateSystemCredentialInput()
			tc.expectError(t, err)
		})
	}
}

type mockSCIMClient struct {
	scimsdk.Client
	pingCalled   bool
	pingResponse error
}

func (mock *mockSCIMClient) Ping(ctx context.Context) error {
	mock.pingCalled = true
	return mock.pingResponse
}

func TestRotateAWSICSCIMToken(t *testing.T) {
	const (
		scimURL = "https://scim.example.com"
	)
	validAWSICPlugin := func() *types.PluginV1 {
		return &types.PluginV1{
			Kind:    types.KindPlugin,
			SubKind: types.PluginSubkindAccess,
			Metadata: types.Metadata{
				Name:   types.PluginTypeAWSIdentityCenter,
				Labels: map[string]string{types.HostedPluginLabel: "true"},
			},
			Spec: types.PluginSpecV1{
				Settings: &types.PluginSpecV1_AwsIc{
					AwsIc: &types.PluginAWSICSettings{
						ProvisioningSpec: &types.AWSICProvisioningSpec{
							BaseUrl: scimURL,
						},
					},
				},
			},
			Credentials: &types.PluginCredentialsV1{
				Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
					StaticCredentialsRef: &types.PluginStaticCredentialsRef{
						Labels: map[string]string{
							"plugin-id": "some-aws-ic-integration",
						},
					},
				},
			},
		}
	}

	testCases := []struct {
		name                string
		cliArgs             awsICRotateCredsArgs
		pluginValueProvider func() *types.PluginV1
		pluginFetchError    error
		expectValidation    bool
		validationError     error
		expectUpdate        bool
		updateError         error
		assertError         require.ErrorAssertionFunc
	}{
		{
			name: "default",
			cliArgs: awsICRotateCredsArgs{
				pluginName:    types.PluginTypeAWSIdentityCenter,
				validateToken: true,
				payload:       "some-token",
			},
			pluginValueProvider: validAWSICPlugin,
			expectValidation:    true,
			expectUpdate:        true,
			assertError:         require.NoError,
		},
		{
			name: "no such plugin",
			cliArgs: awsICRotateCredsArgs{
				pluginName:    types.PluginTypeAWSIdentityCenter,
				validateToken: true,
				payload:       "some-token",
			},
			pluginValueProvider: func() *types.PluginV1 { return nil },
			pluginFetchError:    trace.NotFound("no such plugin"),
			assertError:         require.Error,
		},
		{
			name: "wrong plugin type",
			cliArgs: awsICRotateCredsArgs{
				pluginName:    types.PluginTypeAWSIdentityCenter,
				validateToken: true,
				payload:       "some-token",
			},
			pluginValueProvider: func() *types.PluginV1 {
				return &types.PluginV1{
					Kind:    types.KindPlugin,
					SubKind: types.PluginSubkindAccess,
					Metadata: types.Metadata{
						Name:   "okta",
						Labels: map[string]string{types.HostedPluginLabel: "true"},
					},
					Spec: types.PluginSpecV1{
						Settings: &types.PluginSpecV1_Okta{
							Okta: &types.PluginOktaSettings{},
						},
					},
				}
			},
			assertError: require.Error,
		},
		{
			name: "no such credential",
			cliArgs: awsICRotateCredsArgs{
				pluginName:    types.PluginTypeAWSIdentityCenter,
				validateToken: true,
				payload:       "some-token",
			},
			pluginValueProvider: validAWSICPlugin,
			expectValidation:    true,
			expectUpdate:        true,
			updateError:         trace.NotFound("no such credential"),
			assertError:         require.Error,
		},
		{
			name: "validation failure",
			cliArgs: awsICRotateCredsArgs{
				pluginName:    types.PluginTypeAWSIdentityCenter,
				validateToken: true,
				payload:       "some-token",
			},
			expectValidation:    true,
			validationError:     trace.AccessDenied("Validation failed"),
			pluginValueProvider: validAWSICPlugin,
			expectUpdate:        false,
			assertError:         require.Error,
		},
		{
			name: "bypass validation",
			cliArgs: awsICRotateCredsArgs{
				pluginName:    types.PluginTypeAWSIdentityCenter,
				validateToken: false,
				payload:       "some-token",
			},
			expectValidation:    false,
			validationError:     trace.AccessDenied("Validation failed"),
			pluginValueProvider: validAWSICPlugin,
			expectUpdate:        true,
			assertError:         require.NoError,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cliArgs := PluginsCommand{
				rotateCreds: pluginRotateCredsArgs{
					awsic: test.cliArgs,
				},
			}

			pluginsClient := &mockPluginsClient{}
			pluginsClient.
				On("GetPlugin", anyContext, mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) {
					req, ok := args.Get(1).(*pluginsv1.GetPluginRequest)
					require.True(t, ok, "expecting a *pluginsv1.GetPluginRequest, got %T", args.Get(1))
					require.Equal(t, test.cliArgs.pluginName, req.Name)
					require.True(t, req.WithSecrets)
				}).
				Return(test.pluginValueProvider(), test.pluginFetchError)

			if test.expectUpdate {
				pluginsClient.
					On("UpdatePluginStaticCredentials", anyContext, mock.Anything, mock.Anything).
					Return(func(ctx context.Context, in *pluginsv1.UpdatePluginStaticCredentialsRequest, _ ...grpc.CallOption) (*pluginsv1.UpdatePluginStaticCredentialsResponse, error) {
						q := in.GetQuery()
						require.NotNil(t, q, "Update request must specify target labels")
						require.NotEmpty(t, q.Labels, "Update request must specify non-empty labels")

						return &pluginsv1.UpdatePluginStaticCredentialsResponse{
							Credential: &types.PluginStaticCredentialsV1{Spec: in.GetCredential()},
						}, test.updateError
					})
			}

			scimClient := mockSCIMClient{
				Client:       scimsdk.NewSCIMClientMock(),
				pingResponse: test.validationError,
			}

			args := installPluginArgs{
				plugins: pluginsClient,
				scimProvider: func(_, _, token string) (scimsdk.Client, error) {
					require.Equal(t, test.cliArgs.payload, token)
					return &scimClient, nil
				},
			}

			err := cliArgs.RotateAWSICCreds(context.Background(), args)
			test.assertError(t, err)

			pluginsClient.AssertExpectations(t)
			require.Equal(t, test.expectValidation, scimClient.pingCalled,
				"SCIM validation Ping expected: %t, Ping called %t", test.expectValidation, scimClient.pingCalled)
		})
	}
}
