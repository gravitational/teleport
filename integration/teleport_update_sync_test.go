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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestTeleportUpdateErrorCode verifies the teleport-update status is set during the agent initiation.
func TestTeleportUpdateErrorCode(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)
	helpers.SetTestTimeouts(2 * time.Second)

	configPath := filepath.Join(t.TempDir(), "update.yaml")

	t.Setenv(automaticupgrades.EnvUpgrader, types.UpgraderKindTeleportUpdate)
	t.Setenv(automaticupgrades.EnvUpgraderVersion, "1.2.3")
	t.Setenv("TELEPORT_UPDATE_CONFIG_FILE", configPath)

	// Create fake teleport-update configuration file with default values.
	updateConfig := &agent.UpdateConfig{
		Version: "v1",
		Kind:    "update_config",
		Spec: agent.UpdateSpec{
			Enabled: true,
		},
		Status: agent.UpdateStatus{
			LastUpdate: &agent.LastUpdate{
				ErrorCode: agent.ErrorCodeNoSpaceLeft,
			},
		},
	}
	writeTeleportUpdateConfig(t, configPath, updateConfig)

	// Start Teleport Auth service.
	authClock := clockwork.NewFakeClock()
	authProcess, provisionToken := makeTestServer(t, authClock)
	authServer := authProcess.GetAuthServer()
	authAddr, err := authProcess.AuthAddr()
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()

	// Start Teleport SSH agent service.
	node := helpers.MakeAgentServer(t, cfg, *authAddr, provisionToken)
	require.NotNil(t, node)

	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		debugClt := debug.NewClient(authProcess.Config.DataDir)
		rdy, err := debugClt.GetReadiness(ctx)
		require.NoError(t, err)
		require.True(t, rdy.Ready, "Not yet ready: %q", rdy.Status)

		authClock.Advance(2 * time.Minute)
		hello, err := authServer.LookupAgentInInventory(ctx, authServer.ServerID)
		require.NoError(t, err)

		require.NotEmpty(t, hello)
		require.NotNil(t, hello[0].UpdaterInfo)
		require.Equal(t, types.UpdaterStatus_UPDATER_STATUS_OK, hello[0].UpdaterInfo.UpdaterStatus)
		require.Equal(t, uint32(agent.ErrorCodeNoSpaceLeft), hello[0].UpdaterInfo.ErrorCode)
	}, 20*time.Second, time.Second)
}

// TestTeleportUpdateSync verifies the teleport-update status direct updated by demand.
func TestTeleportUpdateSync(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)
	helpers.SetTestTimeouts(2 * time.Second)

	configPath := filepath.Join(t.TempDir(), "update.yaml")

	t.Setenv(automaticupgrades.EnvUpgrader, types.UpgraderKindTeleportUpdate)
	t.Setenv(automaticupgrades.EnvUpgraderVersion, "1.2.3")
	t.Setenv("TELEPORT_UPDATE_CONFIG_FILE", configPath)

	// Create fake teleport-update configuration file with default values.
	updateConfig := &agent.UpdateConfig{
		Version: "v1",
		Kind:    "update_config",
		Spec: agent.UpdateSpec{
			Enabled: true,
		},
	}
	writeTeleportUpdateConfig(t, configPath, updateConfig)

	// Start Teleport Auth service.
	authClock := clockwork.NewFakeClock()
	authProcess, provisionToken := makeTestServer(t, authClock)
	authServer := authProcess.GetAuthServer()
	authAddr, err := authProcess.AuthAddr()
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()

	// Start Teleport SSH agent service.
	node := helpers.MakeAgentServer(t, cfg, *authAddr, provisionToken)
	require.NotNil(t, node)

	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		debugClt := debug.NewClient(authProcess.Config.DataDir)
		rdy, err := debugClt.GetReadiness(ctx)
		require.NoError(t, err)
		require.True(t, rdy.Ready, "Not yet ready: %q", rdy.Status)

		updateConfig.Spec.Group = "test"
		updateConfig.Spec.Pinned = true
		updateConfig.Status.LastUpdate = &agent.LastUpdate{
			ErrorCode: agent.ErrorCodeNoSpaceLeft,
		}
		writeTeleportUpdateConfig(t, configPath, updateConfig)
		authClock.Advance(2 * time.Minute)

		err = debugClt.UpdateMetadata(ctx)
		require.NoError(t, err)

		hello, err := authServer.LookupAgentInInventory(ctx, authServer.ServerID)
		require.NoError(t, err)

		require.NotEmpty(t, hello)
		require.NotNil(t, hello[0].UpdaterInfo)
		require.Equal(t, "test", hello[0].UpdaterInfo.UpdateGroup)
		require.Equal(t, types.UpdaterStatus_UPDATER_STATUS_PINNED, hello[0].UpdaterInfo.UpdaterStatus)
		require.Equal(t, uint32(agent.ErrorCodeNoSpaceLeft), hello[0].UpdaterInfo.ErrorCode)

	}, 20*time.Second, time.Second)
}

func writeTeleportUpdateConfig(t require.TestingT, path string, cfg *agent.UpdateConfig) {
	cfgData, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	err = os.WriteFile(path, cfgData, 0o600)
	require.NoError(t, err)
}

func makeTestServer(t *testing.T, clock clockwork.Clock) (auth *service.TeleportProcess, provisionToken string) {
	provisionToken = uuid.NewString()
	var err error

	cfg := servicecfg.MakeDefaultConfig()
	cfg.Clock = clock
	cfg.DebugService.Enabled = true
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()
	cfg.SetAuthServerAddress(cfg.Auth.ListenAddr)
	cfg.Auth.ListenAddr.Addr = helpers.NewListener(t, service.ListenerAuth, &cfg.FileDescriptors)
	cfg.Auth.Preference.SetSecondFactor(constants.SecondFactorOff)
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   provisionToken,
		}},
	})
	require.NoError(t, err)
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = true
	cfg.Proxy.Enabled = false
	cfg.Logger = logtest.NewLogger()

	auth, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, auth.Start())

	t.Cleanup(func() {
		require.NoError(t, auth.Close())
		require.NoError(t, auth.Wait())
	})

	_, err = auth.WaitForEventTimeout(30*time.Second, service.AuthTLSReady)
	require.NoError(t, err, "auth server didn't start after 30s")

	return auth, provisionToken
}
