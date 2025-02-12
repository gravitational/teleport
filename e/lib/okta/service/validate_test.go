package oktaservice

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
)

func Test_validateUpdateIntegrationRequest(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		req         *oktapb.UpdateIntegrationRequest
		plugin      *types.PluginV1
		expectedErr string
	}{
		{
			name: "require plugin credentials or request credentials when sync is enabled",
			req: &oktapb.UpdateIntegrationRequest{
				EnableAccessListSync: true,
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							// No credentials info
							CredentialsInfo: &types.PluginOktaCredentialsInfo{},
						},
					},
				},
			},
			expectedErr: "update integration request enables sync but does not provide API credentials, and the plugin has no Okta credentials configured",
		},
		{
			name: "enabling sync with provided API credentials is ok",
			req: &oktapb.UpdateIntegrationRequest{
				EnableAccessListSync: true,
				ApiCredentials: &oktapb.OktaAPICredentials{
					Auth: &oktapb.OktaAPICredentials_OauthId{
						OauthId: "test_client_id",
					},
				},
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							// No credentials info
							CredentialsInfo: &types.PluginOktaCredentialsInfo{},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "enabling sync for plugin with credentials is ok",
			req: &oktapb.UpdateIntegrationRequest{
				EnableAccessListSync: true,
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							CredentialsInfo: &types.PluginOktaCredentialsInfo{
								HasOauthCredentials: true,
							},
						},
					},
				},
			},
			expectedErr: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUpdateIntegrationRequest(tc.req, tc.plugin)
			if tc.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "trace.IsBadParameter(%+v)", err)
				require.ErrorContains(t, err, tc.expectedErr)
			}
		})
	}
}
