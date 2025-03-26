package oktaservice

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
)

func Test_validateCreateIntegrationRequest(t *testing.T) {
	t.Parallel()

	type samlConnectorDesc struct {
		name string
		sso  string
	}

	newSamlConnector := func(t *testing.T, desc samlConnectorDesc) *types.SAMLConnectorV2 {
		t.Helper()
		return &types.SAMLConnectorV2{
			Metadata: types.Metadata{
				Name: desc.name,
			},
			Spec: types.SAMLConnectorSpecV2{
				SSO: desc.sso,
			},
		}
	}

	testCases := []struct {
		name          string
		req           *oktapb.CreateIntegrationRequest
		samlConnector types.SAMLConnector
		expectedErr   string
	}{
		{
			name: "invalid SSO metadata URL",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl: "/path",
			},
			expectedErr: "hostname missing",
		},
		{
			name: "Okta org URL with invalid scheme",
			req: &oktapb.CreateIntegrationRequest{
				OktaOrganizationUrl: "http://example.com",
			},
			expectedErr: "required https scheme",
		},
		{
			name: "it is ok to provide URL with no scheme",
			req: &oktapb.CreateIntegrationRequest{
				OktaOrganizationUrl: "example.com",
			},
			expectedErr: "",
		},
		{
			name: "SSO metadata URL and Okta org URL have different hostnames",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl:      "example.com/sso",
				OktaOrganizationUrl: "https://subdomain.example.com",
			},
			expectedErr: "SSO metadata URL and Okta org URL have different hostnames",
		},
		{
			name: "SSO metadata URL and SAML connector SSO URL have different hostnames",
			req: &oktapb.CreateIntegrationRequest{
				OktaOrganizationUrl: "https://subdomain.example.com",
			},
			samlConnector: newSamlConnector(t, samlConnectorDesc{
				name: "validate-create-integration-request-test-connector",
				sso:  "https://doesn.not.match.example.com",
			}),
			expectedErr: `SAML connector "validate-create-integration-request-test-connector" SSO URL and Okta org URL have different hostnames`,
		},
		{
			name:        "SSO metadata URL and Okta org URL empty",
			req:         &oktapb.CreateIntegrationRequest{},
			expectedErr: "Okta org URL or SSO metadata URL missing",
		},
		{
			name: "API credentials are required when sync is enabled",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl: "example.com/sso", // to bypass URL validation
				EnableUserSync: true,
			},
			expectedErr: "Okta API credentials are required for user sync",
		},
		{
			name: "apps and groups sync can be only enabled if user sync is enabled",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl:       "example.com/sso", // to bypass URL validation
				EnableAccessListSync: true,
				EnableAppGroupSync:   true,
			},
			expectedErr: "App and Group sync can be enabled only when user sync is enabled",
		},
		{
			name: "apps and groups sync must be enabled for access list sync",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl: "example.com/sso", // to bypass URL validation
				ApiCredentials: &oktapb.OktaAPICredentials{
					Auth: &oktapb.OktaAPICredentials_OauthId{
						OauthId: "validation-test-oauth-id",
					},
				},
				EnableUserSync:       true,
				EnableAppGroupSync:   false,
				EnableAccessListSync: true,
			},
			expectedErr: "Access List sync can be enabled only when App and Group sync is enabled",
		},
		{
			name: "bidirectional sync can be set only when App and Group sync is enabled",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl:          "example.com/sso", // to bypass URL validation
				EnableBidirectionalSync: true,
			},
			expectedErr: "bidirectional sync can be set only when App and Group sync is enabled",
		},
		{
			name: "valid case for setting bidirectional sync",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl: "example.com/sso", // to bypass URL validation
				ApiCredentials: &oktapb.OktaAPICredentials{
					Auth: &oktapb.OktaAPICredentials_OauthId{
						OauthId: "validation-test-oauth-id",
					},
				},
				EnableUserSync:          true,
				EnableAccessListSync:    true,
				EnableAppGroupSync:      true,
				EnableBidirectionalSync: true,
			},
			expectedErr: "",
		},
		{
			name: "valid case with matching SSO metadata URL and Okta org URL and connector SSO URL",
			req: &oktapb.CreateIntegrationRequest{
				SsoMetadataUrl:      "the.same.example.com/metadata/saml",
				OktaOrganizationUrl: "https://the.same.example.com",
			},
			samlConnector: newSamlConnector(t, samlConnectorDesc{
				name: "validate-create-integration-request-test-connector",
				sso:  "https://the.same.example.com/saml/sso",
			}),
			expectedErr: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCreateIntegrationRequest(tc.req, tc.samlConnector)
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

func Test_validateUpdateIntegrationRequest(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name        string
		req         *oktapb.UpdateIntegrationRequest
		plugin      *types.PluginV1
		expectedErr string
	}{
		{
			name: "apps and groups sync can be only enabled if user sync is enabled",
			req: &oktapb.UpdateIntegrationRequest{
				EnableAccessListSync: true,
				EnableAppGroupSync:   true,
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
			expectedErr: "App and Group sync can be enabled only when user sync is enabled",
		},
		{
			name: "app and group sync is required for Access List sync",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync:       true,
				EnableAppGroupSync:   false,
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
			expectedErr: "Access List sync can be enabled only when App and Group sync is enabled",
		},
		{
			name: "bidirectional sync can be set only when app group sync and access list sync are enabled",
			req: &oktapb.UpdateIntegrationRequest{
				EnableBidirectionalSync: true,
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{},
					},
				},
			},
			expectedErr: "bidirectional sync can be set only when App and Group sync is enabled",
		},
		{
			name: "valid case for setting bidirectional sync",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync:          true,
				EnableAccessListSync:    true,
				EnableAppGroupSync:      true,
				EnableBidirectionalSync: true,
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
		{
			name: "require plugin credentials or request credentials when sync is enabled",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync: true,
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
			expectedErr: "update integration request enables sync but does not provide Okta API credentials, and the plugin has no Okta API credentials configured",
		},
		{
			name: "require plugin credentials or request credentials when sync is enabled with nil credentials info",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync: true,
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							// nil credentials info
							CredentialsInfo: nil,
						},
					},
				},
			},
			expectedErr: "update integration request enables sync but does not provide Okta API credentials, and the plugin has no Okta API credentials configured",
		},
		{
			name: "enabling sync with provided API credentials is ok",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync:          true,
				EnableAccessListSync:    true,
				EnableAppGroupSync:      true,
				EnableBidirectionalSync: true,
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
				EnableUserSync: true,
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
		{
			name: "for legacy plugins without credentials info set, assume valid credentials when user sync is enabled in the plugin",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync: true,
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							SyncSettings: &types.PluginOktaSyncSettings{
								SyncUsers: true,
							},
							CredentialsInfo: nil,
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "for legacy plugins with credentials info empty, assume valid credentials when user sync is enabled in the plugin",
			req: &oktapb.UpdateIntegrationRequest{
				EnableUserSync:          true,
				EnableAccessListSync:    true,
				EnableAppGroupSync:      true,
				EnableBidirectionalSync: true,
			},
			plugin: &types.PluginV1{
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_Okta{
						Okta: &types.PluginOktaSettings{
							SyncSettings: &types.PluginOktaSyncSettings{
								SyncUsers:            true,
								DisableSyncAppGroups: false,
								SyncAccessLists:      true,
							},
							CredentialsInfo: &types.PluginOktaCredentialsInfo{},
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
