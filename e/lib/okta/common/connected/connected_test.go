package connected

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestIsOktaConnected(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	ap := newTestAccessPoint(t, clock)
	connected, err := New(Config{
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

// testAccessPoint is a test access point for the Okta service.
type testAccessPoint struct {
	services.Plugins

	serviceCounts map[types.SystemRole]uint64
	mu            sync.Mutex
}

// GetInventoryConnectedServiceCount returns the counts of a particular connected service seen in the inventory.
func (t *testAccessPoint) GetInventoryConnectedServiceCount(service types.SystemRole) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.serviceCounts[service]
}

func newTestAccessPoint(t *testing.T, clock clockwork.Clock) *testAccessPoint {
	t.Helper()

	backend, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)

	plugins := local.NewPluginsService(backend)
	require.NoError(t, err)

	client := &testAccessPoint{
		Plugins: plugins,
	}
	client.serviceCounts = map[types.SystemRole]uint64{
		types.RoleOkta: 1,
	}

	return client
}
