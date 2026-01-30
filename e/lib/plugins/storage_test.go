package plugins

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestPluginStorage(t *testing.T) {
	const pluginName = "foo"

	ctx := context.Background()
	mem, err := memory.New(memory.Config{
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	backendService := local.NewPluginsService(mem)
	pluginStore := newPluginStore(backendService, pluginName, logtest.NewLogger())
	initialPlugin := createSlackPlugin(t, pluginName).(*types.PluginV1)
	require.NoError(t, backendService.CreatePlugin(ctx, initialPlugin))

	gotCreds, err := pluginStore.GetCredentials(ctx)
	require.NoError(t, err)
	require.Equal(t, initialPlugin.Credentials.GetOauth2AccessToken().AccessToken, gotCreds.AccessToken)
	require.Equal(t, initialPlugin.Credentials.GetOauth2AccessToken().RefreshToken, gotCreds.RefreshToken)
	require.Equal(t, initialPlugin.Credentials.GetOauth2AccessToken().Expires, gotCreds.ExpiresAt)

	newCreds := &storage.Credentials{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresAt:    time.Now().UTC().Add(12 * time.Hour),
	}
	err = pluginStore.PutCredentials(ctx, newCreds)
	require.NoError(t, err)
	gotCreds, err = pluginStore.GetCredentials(ctx)
	require.NoError(t, err)
	require.Equal(t, newCreds, gotCreds)

	// Other fields of the plugin resource should remain untouched
	gotPlugin, err := backendService.GetPlugin(ctx, pluginName, true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(initialPlugin, gotPlugin,
		cmpopts.IgnoreFields(types.PluginV1{}, "Credentials"),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
	))
}
