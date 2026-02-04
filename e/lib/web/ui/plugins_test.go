package ui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	entraidui "github.com/gravitational/teleport/e/lib/web/ui/entraid"
	"github.com/gravitational/teleport/lib/plugins/filter"
)

func TestPluginSerialization(t *testing.T) {
	testCases := []struct {
		name     string
		plugin   *Plugin
		expected map[string]any
	}{
		{
			name: "simple",
			plugin: &Plugin{
				Name:       "Nonesuch",
				Type:       types.PluginTypeJira,
				Details:    "Lorem ipsum dolor sit amet",
				StatusCode: types.PluginStatusCode_UNKNOWN,
			},
			expected: map[string]any{
				"name":       "Nonesuch",
				"details":    "Lorem ipsum dolor sit amet",
				"type":       "jira",
				"statusCode": float64(0),
			},
		}, {
			name: "okta with spec",
			plugin: &Plugin{
				Name:       "Okta",
				Type:       types.PluginTypeOkta,
				Details:    "Everything starts with Identity",
				StatusCode: types.PluginStatusCode_RUNNING,
				Spec: &OktaPluginSpec{
					TeleportSSOConnector: "okta-integration",
					OktaAppID:            "0oae1fwmde3GL1HC55d7",
					OktaAppName:          "dev-78836936_teleportauth_4",
					OktaAppLabel:         "Teleport platform.teleport.sh",
					SCIMBearerToken:      "some-great-big-random-string",
					DefaultOwners:        []string{"admin"},
				},
			},
			expected: map[string]any{
				"name":       "Okta",
				"details":    "Everything starts with Identity",
				"type":       "okta",
				"statusCode": float64(1),
				"spec": map[string]any{
					"teleportSsoConnector": "okta-integration",
					"oktaAppId":            "0oae1fwmde3GL1HC55d7",
					"oktaAppName":          "dev-78836936_teleportauth_4",
					"oktaAppLabel":         "Teleport platform.teleport.sh",
					"scimBearerToken":      "some-great-big-random-string",
					"defaultOwners":        []any{"admin"},
				},
			},
		}, {
			name: "okta with spec error",
			plugin: &Plugin{
				Name:       "Okta",
				Type:       types.PluginTypeOkta,
				Details:    "Everything starts with Identity",
				StatusCode: types.PluginStatusCode_RUNNING,
				Spec: &OktaPluginSpec{
					Error: "SSO Connector already exists.",
				},
			},
			expected: map[string]any{
				"name":       "Okta",
				"details":    "Everything starts with Identity",
				"type":       "okta",
				"statusCode": float64(1),
				"spec": map[string]any{
					"error": "SSO Connector already exists.",
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			text, err := json.Marshal(test.plugin)
			require.NoError(t, err)

			var actual map[string]any
			require.NoError(t, json.Unmarshal(text, &actual))
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestOktaPluginSpecJSON(t *testing.T) {
	src := Plugin{
		Name:       "Okta",
		Type:       types.PluginTypeOkta,
		Details:    "Everything starts with Identity",
		StatusCode: types.PluginStatusCode_RUNNING,
		Spec: &OktaPluginSpec{
			TeleportSSOConnector: "okta-integration",
			OktaAppID:            "0oae1fwmde3GL1HC55d7",
			OktaAppName:          "Teleport platform.teleport.sh",
			OktaAppLabel:         "dev-78836936_teleportauth_4",
			SCIMBearerToken:      "some-great-big-random-string",
		},
	}

	bytes, err := json.Marshal(&src)
	require.NoError(t, err)

	var dst Plugin
	require.NoError(t, json.Unmarshal(bytes, &dst))

	require.Equal(t, src, dst)
}

func TestEntraPluginProtoToUI(t *testing.T) {
	const timeStr = "2025-12-11T20:15:25.501482Z"
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		plugin   *types.PluginV1
		expected *Plugin
	}{
		{
			name: "converts all supported fields",
			plugin: &types.PluginV1{
				Kind:    "plugin",
				Version: "v1",
				Metadata: types.Metadata{
					Labels: map[string]string{
						"teleport.dev/hosted-plugin": "true",
					},
					Name:      "entra-id-default",
					Namespace: "default",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_EntraId{
						EntraId: &types.PluginEntraIDSettings{
							SyncSettings: &types.PluginEntraIDSyncSettings{
								CredentialsSource:      types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS,
								DefaultOwners:          []string{"admin"},
								AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID,
								EntraAppId:             "ea66927a-fdb0-4e1a-b869-bf0dd494a27e",
								SsoConnectorId:         "entra-id",
								TenantId:               "e0fe9234-0d2e-45bb-b06e-0bd635e11487",
								GroupFilters: []*types.PluginSyncFilter{
									{Include: &types.PluginSyncFilter_Id{Id: "2"}},
									{Include: &types.PluginSyncFilter_NameRegex{NameRegex: "a*"}},
									{Include: &types.PluginSyncFilter_Id{Id: "4"}},
									{Exclude: &types.PluginSyncFilter_ExcludeId{ExcludeId: "6"}},
									{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "b*"}},
									{Exclude: &types.PluginSyncFilter_ExcludeNameRegex{ExcludeNameRegex: "admin*"}},
								},
							},
							AccessGraphSettings: &types.PluginEntraIDAccessGraphSettings{
								AppSsoSettingsCache: []*types.PluginEntraIDAppSSOSettings{
									{
										AppId:          "ea66927a-fdb0-4e1a-b869-bf0dd494a27e",
										FederatedSsoV2: []byte(`abc-random`),
									},
								},
							},
						},
					},
				},
				Status: types.PluginStatusV1{
					Code:         types.PluginStatusCode_OTHER_ERROR,
					ErrorMessage: "failed to get azure authentication token",
					LastRawError: "raw error",
					LastSyncTime: parsedTime,
					Details: &types.PluginStatusV1_EntraId{
						EntraId: &types.PluginEntraIDStatusV1{
							ImportedUsers:  12,
							ImportedGroups: 10,
						},
					},
				},
			},
			expected: &Plugin{
				Name:       "entra-id-default",
				Type:       "entra-id",
				Details:    "Users and groups will be synchronized from the Entra ID directory",
				StatusCode: 2,
				Spec: &entraidui.EntraPluginSpec{
					DefaultOwners:          []string{"admin"},
					AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID.String(),
					SSOConnectorID:         "entra-id",
					CredentialsSource:      "ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS",
					TenantID:               "e0fe9234-0d2e-45bb-b06e-0bd635e11487",
					EntraAppID:             "ea66927a-fdb0-4e1a-b869-bf0dd494a27e",
					GroupFilters: filter.Inputs{
						ID:               []string{"2", "4"},
						NameRegex:        []string{"a*"},
						ExcludeID:        []string{"6"},
						ExcludeNameRegex: []string{"b*", "admin*"},
					},
					AccessGraphEnabled: true,
				},
				Status: &PluginStatusV1{
					Code:         2,
					LastSyncTime: parsedTime,
					ErrorMessage: "failed to get azure authentication token",
					LastRawError: "raw error",
					Details: &PluginDetails{
						EntraID: &types.PluginEntraIDStatusV1{
							ImportedUsers:  12,
							ImportedGroups: 10,
						},
					},
				},
			},
		},
		{
			name: "access graph disabled",
			plugin: &types.PluginV1{
				Kind:    "plugin",
				Version: "v1",
				Metadata: types.Metadata{
					Labels: map[string]string{
						"teleport.dev/hosted-plugin": "true",
					},
					Name:      "entra-id-default",
					Namespace: "default",
				},
				Spec: types.PluginSpecV1{
					Settings: &types.PluginSpecV1_EntraId{
						EntraId: &types.PluginEntraIDSettings{
							SyncSettings: &types.PluginEntraIDSyncSettings{
								CredentialsSource:      types.EntraIDCredentialsSource_ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS,
								DefaultOwners:          []string{"admin"},
								AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN,
								EntraAppId:             "ea66927a-fdb0-4e1a-b869-bf0dd494a27e",
								SsoConnectorId:         "entra-id",
								TenantId:               "e0fe9234-0d2e-45bb-b06e-0bd635e11487",
								GroupFilters: []*types.PluginSyncFilter{
									{Include: &types.PluginSyncFilter_Id{Id: "2"}},
								},
							},
							AccessGraphSettings: &types.PluginEntraIDAccessGraphSettings{},
						},
					},
				},
				Status: types.PluginStatusV1{
					Code:         types.PluginStatusCode_OTHER_ERROR,
					ErrorMessage: "failed to get azure authentication token",
					LastSyncTime: parsedTime,
				},
			},
			expected: &Plugin{
				Name:       "entra-id-default",
				Type:       "entra-id",
				Details:    "Users and groups will be synchronized from the Entra ID directory",
				StatusCode: 2,
				Spec: &entraidui.EntraPluginSpec{
					DefaultOwners:          []string{"admin"},
					AccessListOwnersSource: types.EntraIDAccessListOwnersSource_ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN.String(),
					SSOConnectorID:         "entra-id",
					CredentialsSource:      "ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS",
					TenantID:               "e0fe9234-0d2e-45bb-b06e-0bd635e11487",
					EntraAppID:             "ea66927a-fdb0-4e1a-b869-bf0dd494a27e",
					GroupFilters: filter.Inputs{
						ID: []string{"2"},
					},
					AccessGraphEnabled: false,
				},
				Status: &PluginStatusV1{
					Code:         2,
					LastSyncTime: parsedTime,
					ErrorMessage: "failed to get azure authentication token",
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			uiPlugin, err := NewPlugin(test.plugin)
			require.NoError(t, err)
			require.Equal(t, test.expected, uiPlugin)
		})
	}
}
