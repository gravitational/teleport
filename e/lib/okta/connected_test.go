package okta

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

func TestIsOktaConnected(t *testing.T) {
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewFakeClock())
	log := utils.NewLoggerForTests()

	t.Run("inventory", func(t *testing.T) {
		// Check inventory behavior
		ap.serviceCounts = map[types.SystemRole]uint64{
			types.RoleAdmin: 1,
			types.RoleAuth:  1,
			types.RoleOkta:  1,
		}
		require.True(t, isOktaServiceConnected(ctx, log, ap, nil))

		ap.serviceCounts = map[types.SystemRole]uint64{
			types.RoleAdmin: 1,
			types.RoleAuth:  1,
		}
		require.False(t, isOktaServiceConnected(ctx, log, ap, nil))
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
		require.False(t, isOktaServiceConnected(ctx, log, ap, ap))

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
		require.NoError(t, ap.CreatePlugin(ctx, oktaPlugin))
		require.True(t, isOktaServiceConnected(ctx, log, ap, ap))
		require.False(t, isOktaServiceConnected(ctx, log, ap, nil))

		require.NoError(t, ap.DeletePlugin(ctx, oktaPlugin.GetName()))
		require.False(t, isOktaServiceConnected(ctx, log, ap, ap))
	})
}
