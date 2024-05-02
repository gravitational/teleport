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

package common

import (
	"context"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestPluginsInstallOkta(t *testing.T) {

	testCases := []struct {
		name                     string
		cmd                      PluginsCommand
		expectSAMLConnectorQuery string
		expectRequest            *pluginsv1.CreatePluginRequest
		expectError              require.ErrorAssertionFunc
	}{
		{
			name: "AccessList sync requires at least one default owner",
			cmd: PluginsCommand{
				install: installArgs{
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
				install: installArgs{
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
				install: installArgs{
					okta: oktaArgs{
						samlConnector: "fake-saml-connector",
						scimToken:     "i am a scim token",
						defaultOwners: []string{"admin"},
					},
				},
			},
			expectError: requireBadParameter,
		},
		{
			name: "SCIM sync requires SAML connector",
			cmd: PluginsCommand{
				install: installArgs{
					okta: oktaArgs{
						scimToken:     "i am a scim token",
						defaultOwners: []string{"admin"},
						appID:         "okta app ID goes here",
					},
				},
			},
			expectError: requireBadParameter,
		},
		{
			name: "Bare bones install succeeds",
			cmd: PluginsCommand{
				install: installArgs{
					name: "okta-barebones-test",
					okta: oktaArgs{
						org:      must(url.Parse("https://example.okta.com")),
						apiToken: "api-token-goes-here",
					},
				},
			},
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
								OrgUrl:       "https://example.okta.com",
								SyncSettings: &types.PluginOktaSyncSettings{},
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
				install: installArgs{
					name: "okta-sync-service-test",
					okta: oktaArgs{
						org:            must(url.Parse("https://example.okta.com")),
						apiToken:       "api-token-goes-here",
						userSync:       true,
						accessListSync: true,
						defaultOwners:  []string{"admin"},
						groupFilters:   []string{"group-alpha", "group-beta"},
						appFilters:     []string{"app-gamma", "app-delta", "app-epsilon"},
					},
				},
			},
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
				install: installArgs{
					name: "okta-scim-test",
					okta: oktaArgs{
						org:            must(url.Parse("https://example.okta.com")),
						apiToken:       "api-token-goes-here",
						appID:          "okta-app-id",
						samlConnector:  "teleport-saml-connector-id",
						scimToken:      "i am a scim token",
						userSync:       true,
						accessListSync: true,
						defaultOwners:  []string{"admin"},
						groupFilters:   []string{"group-alpha", "group-beta"},
						appFilters:     []string{"app-gamma", "app-delta", "app-epsilon"},
					},
				},
			},
			expectSAMLConnectorQuery: "teleport-saml-connector-id",
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

			if testCase.expectSAMLConnectorQuery != "" {
				samlConnectorsClient := &mockSAMLConnectorsClient{}
				t.Cleanup(func() { samlConnectorsClient.AssertExpectations(t) })

				samlConnectorsClient.
					On("GetSAMLConnector", anyContext, testCase.expectSAMLConnectorQuery, false).
					Return(&types.SAMLConnectorV2{}, nil)

				args.samlConnectors = samlConnectorsClient
			}

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			err := testCase.cmd.InstallOkta(ctx, args)
			testCase.expectError(t, err)
		})
	}
}

func requireBadParameter(t require.TestingT, err error, _ ...any) {
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "Expecting bad parameter, got %T: \"%v\"", err, err)
}

// must will apply the Go "must" idiom to any arbitrary function that returns a
// conventional (value, error) pair
func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

type mockPluginsClient struct {
	mock.Mock
}

func (m *mockPluginsClient) CreatePlugin(ctx context.Context, in *pluginsv1.CreatePluginRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	result := m.Called(ctx, in, opts)
	return result.Get(0).(*emptypb.Empty), result.Error(1)
}

type mockSAMLConnectorsClient struct {
	mock.Mock
}

func (m *mockSAMLConnectorsClient) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (types.SAMLConnector, error) {
	result := m.Called(ctx, id, withSecrets)
	return result.Get(0).(types.SAMLConnector), result.Error(1)
}

// anyContext is an argument matcher for testify mocks that matches any context.
var anyContext any = mock.MatchedBy(func(context.Context) bool { return true })
