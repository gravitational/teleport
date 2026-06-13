// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package integration

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/join/joinclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// TestJoinClientVersionCheck exercises the client-side minimum version check in
// joinclient.joinViaProxy against a real proxy.
func TestJoinClientVersionCheck(t *testing.T) {
	testDir := t.TempDir()

	cfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		Logger:      logtest.NewLogger(),
	}
	cfg.Listeners = helpers.StandardListenerSetup(t, &cfg.Fds)
	cluster := helpers.NewInstance(t, cfg)

	clusterCfg := servicecfg.MakeDefaultConfig()
	clusterCfg.DataDir = filepath.Join(testDir, "cluster")
	clusterCfg.Auth.Enabled = true
	clusterCfg.Proxy.Enabled = true
	clusterCfg.Proxy.DisableWebInterface = true
	clusterCfg.SSH.Enabled = false
	clusterCfg.Version = "v3"
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles: []types.SystemRole{types.RoleInstance},
			Token: "token",
		}},
	})
	require.NoError(t, err)
	clusterCfg.Auth.StaticTokens = staticTokens

	require.NoError(t, cluster.CreateEx(t, nil, clusterCfg))
	require.NoError(t, cluster.Start())
	defer cluster.StopAll()

	proxyServer := utils.NetAddr{AddrNetwork: "tcp", Addr: cluster.Web}

	tooOldVersion := semver.Version{Major: teleport.MinClientSemVer().Major - 1}.String()

	t.Run("client too old is rejected", func(t *testing.T) {
		_, err := joinclient.Join(t.Context(), joinclient.JoinParams{
			Token:       "token",
			ID:          state.IdentityID{Role: types.RoleInstance},
			ProxyServer: proxyServer,
			JoinMethod:  types.JoinMethodToken,
			Insecure:    true,
			Testing:     joinclient.JoinTestingParams{TeleportVersion: tooOldVersion},
		})
		require.ErrorIs(t, err, joinclient.ErrClientTooOld)
		// The error must name the client version and that a minimum exists so a
		// user knows what to upgrade.
		require.ErrorContains(t, err, "client v"+tooOldVersion)
		require.ErrorContains(t, err, fmt.Sprintf("minimum v%d", teleport.MinClientSemVer().Major))
	})

	t.Run("too old client joins when version check is skipped", func(t *testing.T) {
		// TeleportVersion is not sent over the wire, so this test is just
		// exercising the client-side version check skipping logic. In this
		// case, the client will be allowed to join even though TeleportVersion
		// is below the proxy's advertised minimum because the version the auth
		// service will check is the api.Version of the client.
		var logs bytes.Buffer
		result, err := joinclient.Join(t.Context(), joinclient.JoinParams{
			Token:            "token",
			ID:               state.IdentityID{Role: types.RoleInstance},
			ProxyServer:      proxyServer,
			JoinMethod:       types.JoinMethodToken,
			Insecure:         true,
			SkipVersionCheck: true,
			Log:              slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})),
			Testing:          joinclient.JoinTestingParams{TeleportVersion: tooOldVersion},
		})
		require.NoError(t, err)
		require.NotNil(t, result)

		// The bypass must be logged, naming the flag and the client version, so
		// an operator can see why a too-old client was allowed to connect.
		output := logs.String()
		require.Contains(t, output, "--skip-version-check")
		require.Contains(t, output, "client v"+tooOldVersion)
	})

	t.Run("current client joins successfully", func(t *testing.T) {
		result, err := joinclient.Join(t.Context(), joinclient.JoinParams{
			Token:       "token",
			ID:          state.IdentityID{Role: types.RoleInstance},
			ProxyServer: proxyServer,
			JoinMethod:  types.JoinMethodToken,
			Insecure:    true,
		})
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}
