package plugins

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestOktaInstanceFactory(t *testing.T) {
	t.Parallel()

	// GIVEN a running Teleport Cluster...
	clock := clockwork.NewFakeClock()
	cfg := servicecfg.MakeDefaultConfig()
	var err error
	cfg.Clock = clock
	cfg.DataDir = t.TempDir()
	cfg.DiagnosticAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.SetAuthServerAddress(utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"})
	cfg.Auth.Enabled = true
	cfg.Auth.StorageConfig.Params["path"] = t.TempDir()
	cfg.Auth.ListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}
	cfg.Proxy.Enabled = true
	cfg.Proxy.DisableWebInterface = true
	cfg.SSH.Enabled = false
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	process, err := service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, process.Start())

	// WHEN I try to create an Okta plugin instance inside that cluster...
	plugin := types.NewPluginV1(types.Metadata{
		Name: "okta",
	}, types.PluginSpecV1{
		Settings: &types.PluginSpecV1_Okta{
			Okta: &types.PluginOktaSettings{
				OrgUrl: "https://test.url",
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
	},
	)

	factoryCtx, factoryCancel := context.WithCancel(context.Background())
	pluginLifetime, pluginCancel := context.WithCancel(context.Background())

	startFunc, err := oktaInstanceFactory(factoryCtx, plugin, instanceDependencies{
		lifetime:      pluginLifetime,
		log:           logrus.NewEntry(logrus.New()),
		parentProcess: process,
		staticCredentials: []types.PluginStaticCredentials{
			&types.PluginStaticCredentialsV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "cred",
					},
				},
				Spec: &types.PluginStaticCredentialsSpecV1{
					Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
						APIToken: "test",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	// make sure that anything holding a reference to the wrong context is
	// terminated with extreme prejudice
	factoryCancel()

	startErr := make(chan error, 1)
	go func() {
		startErr <- startFunc()
	}()

	readyEvent := services.EventWithComponents(services.OktaReady, "okta", fmt.Sprintf("%d", clock.Now().Unix()))
	closeEvent := services.EventWithComponents(services.OktaStopped, "okta", fmt.Sprintf("%d", clock.Now().Unix()))

	// EXPECT that the plugin process emits a `ready` event
	_, err = process.WaitForEventTimeout(5*time.Second, readyEvent)
	require.NoError(t, err)

	// WHEN I terminate the plugin
	pluginCancel()

	// EXPECT that the plugin process emits a `close` event and eventually
	// terminmates
	_, err = process.WaitForEventTimeout(5*time.Second, closeEvent)
	require.NoError(t, err)

	select {
	case err := <-startErr:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout waiting for start error")
	}
}
