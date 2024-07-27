/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// TestTimeSyncDifference launches two instances with clock differences in system clock,
// to verify that global notification is created about time drifting.
func TestTimeSyncDifference(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create temporary directories for the auth and agent data directories.
	authDir, agentDir := t.TempDir(), t.TempDir()

	// Write the instance assets to the temporary directories to set up pre-existing
	// state for our teleport instances to use.
	require.NoError(t, basicDirCopy("testdata/auth", authDir))
	require.NoError(t, basicDirCopy("testdata/agent", agentDir))

	authClock := clockwork.NewFakeClockAt(time.Now())
	authCfg := servicecfg.MakeDefaultConfig()
	authCfg.Clock = authClock
	authCfg.Version = defaults.TeleportConfigVersionV3
	authCfg.DataDir = authDir
	authCfg.Auth.Enabled = true
	authCfg.Auth.ListenAddr.Addr = helpers.NewListener(t, service.ListenerAuth, &authCfg.FileDescriptors)
	authCfg.SetAuthServerAddress(authCfg.Auth.ListenAddr)
	// ensure auth server is using the pre-constructed sqlite db
	authCfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(authDir, defaults.BackendDir)}
	var err error
	authCfg.Auth.ClusterName, err = services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "auth-server",
	})
	require.NoError(t, err)
	authCfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)

	authCfg.Proxy.Enabled = true
	authCfg.Proxy.DisableWebInterface = true
	proxyAddr := helpers.NewListener(t, service.ListenerProxyWeb, &authCfg.FileDescriptors)
	authCfg.Proxy.WebAddr.Addr = proxyAddr

	authCfg.SSH.Enabled = true
	authCfg.SSH.Addr.Addr = "localhost:0"
	authCfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	authCfg.Log = utils.NewLoggerForTests()
	authCfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()

	serviceC := make(chan *service.TeleportProcess, 20)
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- service.Run(ctx, *authCfg, func(cfg *servicecfg.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err != nil {
				return nil, err
			}
			serviceC <- svc
			return svc, err
		})
	}()

	authService, err := waitForProcessStart(serviceC)
	require.NoError(t, err)

	// Start the agent service with Node and WindowsDesktop capabilities.
	agentClock := clockwork.NewFakeClockAt(time.Now())
	agentCfg := servicecfg.MakeDefaultConfig()
	agentCfg.Clock = agentClock
	agentCfg.Version = defaults.TeleportConfigVersionV3
	agentCfg.DataDir = agentDir
	agentCfg.ProxyServer = utils.NetAddr{AddrNetwork: "tcp", Addr: proxyAddr}

	agentCfg.Auth.Enabled = false
	agentCfg.Proxy.Enabled = false
	agentCfg.SSH.Enabled = true
	agentCfg.WindowsDesktop.Enabled = true

	agentCfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	agentCfg.MaxRetryPeriod = time.Second
	agentCfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	agentCfg.Logger = utils.NewSlogLoggerForTests()

	agentRunErrCh := make(chan error, 1)
	go func() {
		agentRunErrCh <- service.Run(ctx, *agentCfg, func(cfg *servicecfg.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err != nil {
				return nil, err
			}
			serviceC <- svc
			return svc, err
		})
	}()

	agentService, err := waitForProcessStart(serviceC)
	require.NoError(t, err)

	t.Cleanup(func() {
		authService.Shutdown(context.Background())
		agentService.Shutdown(context.Background())
	})

	// Wait till the service is ready and inventory stream connection is established between services.
	_, err = agentService.WaitForEventTimeout(20*time.Second, service.TeleportReadyEvent)
	require.NoError(t, err, "timeout waiting for Teleport readiness")

	// Must trigger the monitor watch logic to create the global notification.
	agentClock.Advance(15 * time.Minute)
	authClock.Advance(10 * time.Minute)

	err = retryutils.RetryStaticFor(20*time.Second, time.Second, func() error {
		notifications, _, err := authService.GetAuthServer().ListGlobalNotifications(ctx, 100, "")
		if err != nil {
			return trace.Wrap(err)
		}
		var found bool
		for _, notification := range notifications {
			found = found || notification.GetMetadata().GetName() == "cluster-monitor-system-clock-warning"
		}
		if !found {
			return trace.BadParameter("expected notification is not found")
		}
		return nil
	})
	require.NoError(t, err)
}
