package plugins

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// createSlackPlugin creates a basic Slack plugin for testing purposes
func createSlackPlugin(t *testing.T, name string) types.Plugin {
	t.Helper()

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

// createOktaPlugin creates a basic Okta plugin for testing purposes
func createOktaPlugin(t *testing.T, name string) (types.Plugin, types.PluginStaticCredentials) {
	t.Helper()

	id := uuid.NewString()

	labels := map[string]string{
		"label1":              "value1",
		eteleport.PluginLabel: id,
	}

	p := types.NewPluginV1(
		types.Metadata{
			Name: name,
			Labels: map[string]string{
				HostedPluginLabel: "true",
			},
		},
		types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl:       "https://www.okta.com",
					SyncSettings: &types.PluginOktaSyncSettings{},
				},
			},
		},
		&types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: labels,
				},
			},
		},
	)
	require.NoError(t, p.CheckAndSetDefaults())

	creds, err := types.NewPluginStaticCredentials(
		types.Metadata{
			Name:   name,
			Labels: labels,
		}, types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "test-token",
			},
		},
	)
	require.NoError(t, err)

	return p, creds
}
