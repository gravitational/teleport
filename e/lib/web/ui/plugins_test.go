package ui

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
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
					OktaAppName:          "Teleport platform.teleport.sh",
					SCIMBearerToken:      "some-great-big-random-string",
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
					"oktaAppName":          "Teleport platform.teleport.sh",
					"scimBearerToken":      "some-great-big-random-string",
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
