// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/client/proto"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

func TestPluginsInstallOkta(t *testing.T) {
	testCases := []struct {
		name                     string
		cmd                      PluginsCommand
		expectSAMLConnectorQuery string
		expectPing               bool
		expectRequest            *pluginsv1.CreatePluginRequest
		expectError              require.ErrorAssertionFunc
	}{
		{
			name: "AccessList sync requires at least one default owner",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta",
					okta: oktaArgs{
						accessListSync: true,
					},
				},
			},
			expectError: requireBadParameter,
		},
		{
			name: "SCIM sync requires at least one default owner",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta",
					okta: oktaArgs{
						samlConnector: "fake-saml-connector",
						scimToken:     "i am a scim token",
						appID:         "okta app ID goes here",
					},
				},
			},
			expectError: requireBadParameter,
		},
		{
			name: "SCIM sync requires appID",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta",
					okta: oktaArgs{
						samlConnector: "fake-saml-connector",
						scimToken:     "i am a scim token",
						defaultOwners: []string{"admin"},
						scimEnabled:   true,
						userSync:      true,
						apiToken:      "api-token-goes-here",
					},
				},
			},
			expectSAMLConnectorQuery: "fake-saml-connector",
			expectError:              requireBadParameter,
		},
		{
			name: "Bare bones install succeeds",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta-barebones-test",
					okta: oktaArgs{
						org:           mustParseURL("https://example.okta.com"),
						samlConnector: "okta-integration",
						apiToken:      "api-token-goes-here",
						appGroupSync:  true,
					},
				},
			},
			expectSAMLConnectorQuery: "okta-integration",
			expectRequest: &pluginsv1.CreatePluginRequest{
				Plugin: &types.PluginV1{
					SubKind: types.PluginSubkindAccess,
					Metadata: types.Metadata{
						Labels: map[string]string{
							types.HostedPluginLabel: "true",
						},
						Name: "okta-barebones-test",
					},
					Spec: types.PluginSpecV1{
						Settings: &types.PluginSpecV1_Okta{
							Okta: &types.PluginOktaSettings{
								OrgUrl: "https://example.okta.com",
								SyncSettings: &types.PluginOktaSyncSettings{
									SsoConnectorId: "okta-integration",
								},
							},
						},
					},
				},
				StaticCredentialsList: []*types.PluginStaticCredentialsV1{
					{
						ResourceHeader: types.ResourceHeader{
							Metadata: types.Metadata{
								Name: "okta-barebones-test",
								Labels: map[string]string{
									types.OktaCredPurposeLabel: types.OktaCredPurposeAuth,
								},
							},
						},
						Spec: &types.PluginStaticCredentialsSpecV1{
							Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
								APIToken: "api-token-goes-here",
							},
						},
					},
				},
				CredentialLabels: map[string]string{
					types.OktaOrgURLLabel: "https://example.okta.com",
				},
			},
			expectError: require.NoError,
		},
		{
			name: "Sync service enabled",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta-sync-service-test",
					okta: oktaArgs{
						org:            mustParseURL("https://example.okta.com"),
						apiToken:       "api-token-goes-here",
						samlConnector:  "saml-connector-name",
						userSync:       true,
						accessListSync: true,
						appGroupSync:   true,
						defaultOwners:  []string{"admin"},
						groupFilters:   []string{"group-alpha", "group-beta"},
						appFilters:     []string{"app-gamma", "app-delta", "app-epsilon"},
					},
				},
			},
			expectSAMLConnectorQuery: "saml-connector-name",
			expectRequest: &pluginsv1.CreatePluginRequest{
				Plugin: &types.PluginV1{
					SubKind: types.PluginSubkindAccess,
					Metadata: types.Metadata{
						Labels: map[string]string{
							types.HostedPluginLabel: "true",
						},
						Name: "okta-sync-service-test",
					},
					Spec: types.PluginSpecV1{
						Settings: &types.PluginSpecV1_Okta{
							Okta: &types.PluginOktaSettings{
								OrgUrl: "https://example.okta.com",
								SyncSettings: &types.PluginOktaSyncSettings{
									SyncUsers:       true,
									SyncAccessLists: true,
									SsoConnectorId:  "saml-connector-name",
									DefaultOwners:   []string{"admin"},
									GroupFilters:    []string{"group-alpha", "group-beta"},
									AppFilters:      []string{"app-gamma", "app-delta", "app-epsilon"},
								},
							},
						},
					},
				},
				StaticCredentialsList: []*types.PluginStaticCredentialsV1{
					{
						ResourceHeader: types.ResourceHeader{
							Metadata: types.Metadata{
								Name: "okta-sync-service-test",
								Labels: map[string]string{
									types.OktaCredPurposeLabel: types.OktaCredPurposeAuth,
								},
							},
						},
						Spec: &types.PluginStaticCredentialsSpecV1{
							Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
								APIToken: "api-token-goes-here",
							},
						},
					},
				},
				CredentialLabels: map[string]string{
					types.OktaOrgURLLabel: "https://example.okta.com",
				},
			},
			expectError: require.NoError,
		},
		{
			name: "SCIM service enabled",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta-scim-test",
					okta: oktaArgs{
						org:            mustParseURL("https://example.okta.com"),
						apiToken:       "api-token-goes-here",
						appID:          "okta-app-id",
						samlConnector:  "teleport-saml-connector-id",
						scimToken:      "i am a scim token",
						userSync:       true,
						accessListSync: true,
						appGroupSync:   true,
						defaultOwners:  []string{"admin"},
						groupFilters:   []string{"group-alpha", "group-beta"},
						appFilters:     []string{"app-gamma", "app-delta", "app-epsilon"},
					},
				},
			},
			expectSAMLConnectorQuery: "teleport-saml-connector-id",
			expectPing:               true,
			expectRequest: &pluginsv1.CreatePluginRequest{
				Plugin: &types.PluginV1{
					SubKind: types.PluginSubkindAccess,
					Metadata: types.Metadata{
						Labels: map[string]string{
							types.HostedPluginLabel: "true",
						},
						Name: "okta-scim-test",
					},
					Spec: types.PluginSpecV1{
						Settings: &types.PluginSpecV1_Okta{
							Okta: &types.PluginOktaSettings{
								OrgUrl: "https://example.okta.com",
								SyncSettings: &types.PluginOktaSyncSettings{
									AppId:           "okta-app-id",
									SsoConnectorId:  "teleport-saml-connector-id",
									SyncUsers:       true,
									SyncAccessLists: true,
									DefaultOwners:   []string{"admin"},
									GroupFilters:    []string{"group-alpha", "group-beta"},
									AppFilters:      []string{"app-gamma", "app-delta", "app-epsilon"},
								},
							},
						},
					},
				},
				StaticCredentialsList: []*types.PluginStaticCredentialsV1{
					{
						ResourceHeader: types.ResourceHeader{
							Metadata: types.Metadata{
								Name: "okta-scim-test",
								Labels: map[string]string{
									types.OktaCredPurposeLabel: types.OktaCredPurposeAuth,
								},
							},
						},
						Spec: &types.PluginStaticCredentialsSpecV1{
							Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
								APIToken: "api-token-goes-here",
							},
						},
					},
					{
						ResourceHeader: types.ResourceHeader{
							Metadata: types.Metadata{
								Name: "okta-scim-test-scim-token",
								Labels: map[string]string{
									types.OktaCredPurposeLabel: types.OktaCredPurposeSCIMToken,
								},
							},
						},
						Spec: &types.PluginStaticCredentialsSpecV1{
							Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
								APIToken: "scim-token-goes-here",
							},
						},
					},
				},
				CredentialLabels: map[string]string{
					types.OktaOrgURLLabel: "https://example.okta.com",
				},
			},
			expectError: require.NoError,
		},
		{
			name: "app group sync sync disabled should send okta-auth-scim-only creds",
			cmd: PluginsCommand{
				install: pluginInstallArgs{
					name: "okta-barebones-test",
					okta: oktaArgs{
						org:           mustParseURL("https://example.okta.com"),
						samlConnector: "okta-integration",
						apiToken:      "api-token-goes-here",
						appGroupSync:  false,
						scimToken:     "OktaCredPurposeSCIMToken",
					},
				},
			},
			expectSAMLConnectorQuery: "okta-integration",
			expectPing:               true,
			expectRequest: &pluginsv1.CreatePluginRequest{
				Plugin: &types.PluginV1{
					SubKind: types.PluginSubkindAccess,
					Metadata: types.Metadata{
						Labels: map[string]string{
							types.HostedPluginLabel: "true",
						},
						Name: "okta-barebones-test",
					},
					Spec: types.PluginSpecV1{
						Settings: &types.PluginSpecV1_Okta{
							Okta: &types.PluginOktaSettings{
								OrgUrl: "https://example.okta.com",
								SyncSettings: &types.PluginOktaSyncSettings{
									SsoConnectorId: "okta-integration",
								},
							},
						},
					},
				},
				StaticCredentialsList: []*types.PluginStaticCredentialsV1{
					{
						ResourceHeader: types.ResourceHeader{
							Metadata: types.Metadata{
								Name: "okta-barebones-test",
								Labels: map[string]string{
									types.OktaCredPurposeLabel: types.CredPurposeOKTAAPITokenWithSCIMOnlyIntegration,
								},
							},
						},
						Spec: &types.PluginStaticCredentialsSpecV1{
							Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
								APIToken: "api-token-goes-here",
							},
						},
					},
				},
				CredentialLabels: map[string]string{
					types.OktaOrgURLLabel: "https://example.okta.com",
				},
			},
			expectError: require.NoError,
		},
	}

	cmpOptions := []cmp.Option{
		// Ignore extraneous fields for protobuf bookkeeping
		cmpopts.IgnoreUnexported(pluginsv1.CreatePluginRequest{}),

		// Ignore any SCIM-token credentials because the bcrypt hash of the token
		// will change on every run.
		// TODO: Find a way to only exclude the token hash from the comparison,
		//       rather than the whole credential
		cmpopts.IgnoreSliceElements(func(c *types.PluginStaticCredentialsV1) bool {
			l, _ := c.GetLabel(types.OktaCredPurposeLabel)
			return l == types.OktaCredPurposeSCIMToken
		}),
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var args installPluginArgs

			if testCase.expectRequest != nil {
				pluginsClient := &mockPluginsClient{}
				t.Cleanup(func() { pluginsClient.AssertExpectations(t) })

				pluginsClient.
					On("CreatePlugin", anyContext, mock.Anything, mock.Anything).
					Run(func(args mock.Arguments) {
						require.IsType(t, (*pluginsv1.CreatePluginRequest)(nil), args.Get(1))
						request := args.Get(1).(*pluginsv1.CreatePluginRequest)
						require.Empty(t, cmp.Diff(testCase.expectRequest, request, cmpOptions...))
					}).
					Return(&emptypb.Empty{}, nil)

				args.plugins = pluginsClient
			}

			authClient := &mockAuthClient{}
			if testCase.expectSAMLConnectorQuery != "" {
				t.Cleanup(func() { authClient.AssertExpectations(t) })

				authClient.
					On("GetSAMLConnector", anyContext, testCase.expectSAMLConnectorQuery, false).
					Return(&types.SAMLConnectorV2{}, nil)

				args.authClient = authClient
			}

			if testCase.expectPing {
				authClient.
					On("Ping", anyContext).
					Return(proto.PingResponse{
						ProxyPublicAddr: "example.com",
					}, nil)
			}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			testCase.cmd.config = &servicecfg.Config{
				Logger: slog.Default().With("test", t.Name()),
			}

			err := testCase.cmd.InstallOkta(ctx, args)
			testCase.expectError(t, err)
		})
	}
}

func requireBadParameter(t require.TestingT, err error, _ ...any) {
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "Expecting bad parameter, got %T: \"%v\"", err, err)
}

func mustParseURL(text string) *url.URL {
	url, err := url.Parse(text)
	if err != nil {
		panic(err)
	}
	return url
}

type mockPluginsClient struct {
	mock.Mock
}

func (m *mockPluginsClient) CreatePlugin(ctx context.Context, in *pluginsv1.CreatePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*emptypb.Empty), result.Error(1)
}

type mockAuthClient struct {
	mock.Mock
}

func (m *mockAuthClient) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	result := m.Called(ctx, id, withSecrets)
	return result.Get(0).(types.SAMLConnector), result.Error(1)
}

func (m *mockAuthClient) Ping(ctx context.Context) (proto.PingResponse, error) {
	result := m.Called(ctx)
	return result.Get(0).(proto.PingResponse), result.Error(1)
}

// anyContext is an argument matcher for testify mocks that matches any context.
var anyContext any = mock.MatchedBy(func(context.Context) bool { return true })
