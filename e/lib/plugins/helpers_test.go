package plugins

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// createSlackPlugin creates a basic Slack plugin for testing purposes
func createSlackPlugin(t *testing.T, name string) types.Plugin {
	p := types.NewPluginV1(
		types.Metadata{
			Name: name,
			Labels: map[string]string{
				HostedPluginLabel: "true",
			},
		},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: "#access-requests",
				},
			},
		},
		&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
				Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
					AccessToken:  "foo",
					RefreshToken: "bar",
					Expires:      time.Now().UTC().Add(12 * time.Hour),
				},
			},
		},
	)
	require.NoError(t, p.CheckAndSetDefaults())
	return p
}
