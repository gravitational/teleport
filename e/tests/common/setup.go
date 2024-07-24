package common

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

// SUT is a system under test.
type SUT struct {
	Teleport *helpers.TeleInstance
	Clock    clockwork.Clock
}

func InitSUT(t *testing.T, opts ...option) *SUT {
	options := &sutOptions{}
	for _, opt := range opts {
		opt(options)
	}

	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	})

	clock := clockwork.NewRealClock()
	cfg := newInstanceConfig(t)
	cfg.Clock = clock
	teleport := helpers.NewInstance(t, cfg)

	teleport.ProcessProvider = &entProcessProvider{}

	serviceConfig := newTeleportConfig()

	serviceConfig.Auth.BootstrapResources = options.resources

	serviceConfig.Auth.HostedPlugins.Enabled = true
	serviceConfig.Clock = clock
	err := teleport.CreateEx(t, nil, serviceConfig)
	require.NoError(t, err)

	err = teleport.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, teleport.StopAll())
	})

	auth := teleport.Process.GetAuthServer()
	_, err = auth.GetAccessLists(context.Background())
	require.NoError(t, err)

	return &SUT{
		Teleport: teleport,
		Clock:    clock,
	}
}

// GetClusterClientForUser returns a client for the given user.
func (s *SUT) GetClusterClientForUser(t *testing.T, user string) *client.ClusterClient {
	t.Helper()
	var tc *client.ClusterClient
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		aliceClient, err := s.Teleport.NewClient(helpers.ClientConfig{
			TeleportUser: user,
			Cluster:      helpers.Site,
			Host:         helpers.Host,
		})
		require.NoError(t, err)

		tc, err = aliceClient.ConnectToCluster(context.Background())
		require.NoError(t, err)
		_, err = tc.AuthClient.Ping(context.Background())
		if err != nil {
			tc.Close()
		}
		assert.NoError(collect, err)

	}, time.Second*5, time.Millisecond*100)

	_, err := tc.AuthClient.Ping(context.Background())
	require.NoError(t, err)

	return tc
}

func newInstanceConfig(t *testing.T) helpers.InstanceConfig {
	// Create the CA authority that will be used in Auth.
	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)
	const (
		host   = helpers.Host
		site   = helpers.Site
		hostID = helpers.HostID
	)
	return helpers.InstanceConfig{
		ClusterName: site,
		HostID:      host,
		NodeName:    host,
		Priv:        priv,
		Pub:         pub,
		Log:         utils.NewLoggerForTests(),
	}
}

func newTeleportConfig() *servicecfg.Config {
	serviceConfig := servicecfg.MakeDefaultConfig()
	// Replace the default auth and proxy listeners with the ones so we can
	// run multiple tests in parallel.
	serviceConfig.Console = nil
	serviceConfig.Proxy.DisableWebInterface = true
	serviceConfig.PollingPeriod = 500 * time.Millisecond
	serviceConfig.Testing.ClientTimeout = time.Second
	serviceConfig.Testing.ShutdownTimeout = 2 * serviceConfig.Testing.ClientTimeout
	return serviceConfig
}
