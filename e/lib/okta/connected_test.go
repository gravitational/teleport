package okta

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestIsOktaConnected(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	ap := newTestAccessPoint(t, clock)
	connected, err := NewOktaConnected(OktaConnectedConfig{
		Clock:           clock,
		ConnectedGetter: ap,
		Plugins:         ap,
	})
	require.NoError(t, err)

	t.Run("inventory", func(t *testing.T) {
		// Check inventory behavior
		ap.serviceCounts = map[types.SystemRole]uint64{
			types.RoleAdmin: 1,
			types.RoleAuth:  1,
			types.RoleOkta:  1,
		}
		require.True(t, connected.IsConnected(ctx))

		ap.serviceCounts = map[types.SystemRole]uint64{
			types.RoleAdmin: 1,
			types.RoleAuth:  1,
		}
		clock.Advance(10 * time.Second)

		// Result should be cached
		require.True(t, connected.IsConnected(ctx))
		clock.Advance(time.Minute)

		require.False(t, connected.IsConnected(ctx))
	})

	t.Run("plugins", func(t *testing.T) {
		// Make sure the inventory doesn't have an Okta service.
		ap.serviceCounts = map[types.SystemRole]uint64{
			types.RoleAdmin: 1,
			types.RoleAuth:  1,
		}
		// Check plugins behavior
		slackPlugin := types.NewPluginV1(types.Metadata{
			Name: "slack",
		}, types.PluginSpecV1{
			Settings: &types.PluginSpecV1_SlackAccessPlugin{
				SlackAccessPlugin: &types.PluginSlackAccessSettings{
					FallbackChannel: "some-channel",
				},
			},
		}, &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_Oauth2AccessToken{
				Oauth2AccessToken: &types.PluginOAuth2AccessTokenCredentials{
					AccessToken:  "access",
					RefreshToken: "refresh",
					Expires:      time.Now(),
				},
			},
		})
		require.NoError(t, ap.CreatePlugin(ctx, slackPlugin))
		clock.Advance(time.Minute)
		require.False(t, connected.IsConnected(ctx))

		oktaPlugin := types.NewPluginV1(types.Metadata{
			Name: "okta",
		}, types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl: "some-url",
				},
			},
		}, &types.PluginCredentialsV1{
			Credentials: &types.PluginCredentialsV1_StaticCredentialsRef{
				StaticCredentialsRef: &types.PluginStaticCredentialsRef{
					Labels: map[string]string{
						"label1": "value1",
					},
				},
			},
		})

		// Plugin created, is connected is true.
		require.NoError(t, ap.CreatePlugin(ctx, oktaPlugin))
		clock.Advance(time.Minute)
		require.True(t, connected.IsConnected(ctx))

		// Plugin deleted, but result still cached.
		require.NoError(t, ap.DeletePlugin(ctx, oktaPlugin.GetName()))
		clock.Advance(10 * time.Second)
		require.True(t, connected.IsConnected(ctx))

		// Cache expired.
		clock.Advance(time.Minute)
		require.False(t, connected.IsConnected(ctx))
	})
}
