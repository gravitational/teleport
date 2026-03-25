/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestReadiness is a smoke test making sure that Teleport eventually becomes ready.
// It comes loaded with most services and fails if the instance did not turn ready
// after 20 seconds.
func TestReadiness(t *testing.T) {
	testDir := t.TempDir()
	prometheus.DefaultRegisterer = metricRegistryBlackHole{}
	log := logtest.NewLogger()

	// Test setup: creating a teleport instance running auth and proxy
	const clusterName = "root.example.com"
	cfg := helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Logger:      log.With("test_component", "test-instance"),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)

	// Manually create the listeners for kube and windows.
	kubeListener := helpers.NewListener(t, service.ListenerKube, &cfg.Fds)
	kubeAddr, err := utils.ParseAddr(kubeListener)
	require.NoError(t, err)
	windowsListener := helpers.NewListener(t, service.ListenerWindowsDesktop, &cfg.Fds)
	windowsAddr, err := utils.ParseAddr(windowsListener)

	require.NoError(t, err)
	rc := helpers.NewInstance(t, cfg)

	matchAll := []services.ResourceMatcher{
		{Labels: types.Labels{"*": []string{"*"}}},
	}

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "data")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.Apps.Enabled = true
	rcConf.Apps.ResourceMatchers = matchAll
	rcConf.Databases.Enabled = true
	rcConf.Databases.ResourceMatchers = matchAll
	rcConf.WindowsDesktop.Enabled = true
	rcConf.WindowsDesktop.ResourceMatchers = matchAll
	rcConf.WindowsDesktop.ListenAddr = *windowsAddr
	rcConf.Kube.Enabled = true
	rcConf.Kube.ResourceMatchers = matchAll
	rcConf.Kube.ListenAddr = kubeAddr
	rcConf.Version = "v3"
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	// We use the debug service unix socket to check health
	rcConf.DebugService.Enabled = true

	// Test setup: starting the Teleport instance
	require.NoError(t, rc.CreateEx(t, nil, rcConf))
	require.NoError(t, rc.Start())

	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		debugClt := debug.NewClient(rcConf.DataDir)
		rdy, err := debugClt.GetReadiness(ctx)
		require.NoError(t, err)
		require.True(t, rdy.Ready, "Not yet ready: %q", rdy.Status)
	}, 20*time.Second, 500*time.Millisecond)
}
