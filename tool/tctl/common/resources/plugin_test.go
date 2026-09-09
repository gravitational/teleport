/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package resources

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apicommon "github.com/gravitational/teleport/api/types/common"
)

func TestPluginResourceWrapper(t *testing.T) {
	tests := []struct {
		name   string
		plugin types.PluginV1
	}{
		{
			name: "okta",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "okta",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							OrgUrl: "https://oktaorg.okta.com",
							SyncSettings: &types.PluginOktaSyncSettings{
								SyncUsers:       true,
								SsoConnectorId:  "connectorID",
								SyncAccessLists: true,
							},
						},
					},
				},
				Credentials: &types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
						StaticCredentialsRef: &types.PluginStaticCredentialsRef{Labels: map[string]string{"label": "value"}},
					},
				},
			},
		},
		{
			name: "slack",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "okta",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_SlackAccessPlugin{
						SlackAccessPlugin: &types.PluginSlackAccessSettings{
							FallbackChannel: "#channel",
						},
					},
				},
				Credentials: &types.PluginCredentialsV1{
					Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
						Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
							AccessToken:  "token",
							RefreshToken: "refresh_token",
						},
					},
				},
			},
		},
		{
			name: "identity center",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: apicommon.OriginAWSIdentityCenter,
					Labels: map[string]string{
						"teleport.dev/hosted-plugin": "true",
					},
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_AwsIc{
						AwsIc: &types.PluginAWSICSettings{
							Credentials: &types.AWSICCredentials{
								Source: &types.AWSICCredentials_System{
									System: &types.AWSICCredentialSourceSystem{},
								},
							},
							Region: "ap-south-2",
							Arn:    "some:arn",
							ProvisioningSpec: &types.AWSICProvisioningSpec{
								BaseUrl: "https://scim.example.com/v2",
							},
							AccessListDefaultOwners: []string{"root"},
							UserSyncFilters: []*types.AWSICUserSyncFilter{
								{Labels: map[string]string{types.OriginLabel: types.OriginOkta}},
								{Labels: map[string]string{types.OriginLabel: types.OriginEntraID}},
							},
							GroupSyncFilters: []*types.AWSICResourceFilter{
								{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `^Group #\\d+$`}},
								{Include: &types.AWSICResourceFilter_Id{Id: "42"}},
							},
							AwsAccountsFilters: []*types.AWSICResourceFilter{
								{Include: &types.AWSICResourceFilter_Id{Id: "314159"}},
								{Include: &types.AWSICResourceFilter_NameRegex{NameRegex: `^Account #\\d+$`}},
							},
						},
					},
				},
				Status: types.PluginStatusV1{
					Code: types.PluginStatusCode_RUNNING,
					Details: &types.PluginStatusV1_AwsIc{
						AwsIc: &types.PluginAWSICStatusV1{
							GroupImportStatus: &types.AWSICGroupImportStatus{
								StatusCode: types.AWSICGroupImportStatusCode_DONE,
							},
						},
					},
				},
			},
		},
		{
			name: "entra_id",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "entra_id",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_EntraId{
						EntraId: &types.PluginEntraIDSettings{},
					},
				},
				Status: types.PluginStatusV1{
					Details: &types.PluginStatusV1_EntraId{
						EntraId: &types.PluginEntraIDStatusV1{},
					},
				},
			},
		},
		{
			name: "gitlab",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "gitlab",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Gitlab{
						Gitlab: &types.PluginGitlabSettings{},
					},
				},
				Status: types.PluginStatusV1{
					Details: &types.PluginStatusV1_Gitlab{
						Gitlab: &types.PluginGitlabStatusV1{},
					},
				},
			},
		},
		{
			name: "net_iq",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "net_iq",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_NetIq{
						NetIq: &types.PluginNetIQSettings{},
					},
				},
				Status: types.PluginStatusV1{
					Details: &types.PluginStatusV1_NetIq{
						NetIq: &types.PluginNetIQStatusV1{},
					},
				},
			},
		},
		{
			name: "scim",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "scim",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Scim{
						Scim: &types.PluginSCIMSettings{},
					},
				},
			},
		},
		{
			name: "github",
			plugin: types.PluginV1{
				Kind:    types.KindPlugin,
				Version: types.V1,
				Metadata: types.Metadata{
					Name: "github",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Github{
						Github: &types.PluginGithubSettings{
							OrganizationName: "acme",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buff, err := json.Marshal(&tc.plugin)
			require.NoError(t, err)
			var failItem types.PluginV1
			err = json.Unmarshal(buff, &failItem)
			require.Error(t, err)
			var item pluginResourceWrapper
			err = json.Unmarshal(buff, &item)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tc.plugin, item.PluginV1))
		})
	}
}
