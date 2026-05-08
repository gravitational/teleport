/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package identity

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/hardwarekey"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestKeyAgentService(t *testing.T) {
	t.Parallel()

	logger := logtest.NewLogger()

	// Spin up a Teleport process.
	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		defaultTestServerOpts(logger),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

	// Create an empty role, bots need at least one role to function.
	role, err := types.NewRole("empty-role", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = rootClient.CreateRole(t.Context(), role)
	require.NoError(t, err)

	// Run the bot.
	outputDir := filepath.Join(t.TempDir(), "output")
	b, err := bot.New(bot.Config{
		Connection: connection.Config{
			Address:     proxyAddr.Addr,
			AddressKind: connection.AddressKindProxy,
			Insecure:    true,
		},
		Onboarding: makeBot(t, rootClient, "test-bot", role.GetName()),
		Logger:     logger,
		Services: []bot.ServiceBuilder{
			KeyAgentServiceBuilder(&KeyAgentConfig{
				Destination: &destination.Directory{
					Path: outputDir,
				},
			}),
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	errCh := make(chan error, 1)
	go func() { errCh <- b.Run(ctx) }()

	// Wait for the identity file to be written.
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(outputDir, "identity"))
		return err == nil
	}, 5*time.Second, 100*time.Millisecond, "waiting for identity file to be written")

	// Create hardware key client.
	hwksClient, err := hardwarekey.NewAgentClient(t.Context(), outputDir)
	require.NoError(t, err)
	hwks := hardwarekeyagent.NewService(hwksClient, nil /* fallbackService */)

	// Load the identity file.
	clusterName := process.Config.Auth.ClusterName.GetClusterName()
	keyRing, err := identityfile.KeyRingFromIdentityFile(
		filepath.Join(outputDir, "identity"),
		proxyAddr.Addr,
		clusterName,
		identityfile.WithHardwareKeyService(hwks),
	)
	require.NoError(t, err)

	// Create a client.
	clientCfg := &authclient.Config{
		AuthServers: process.Config.AuthServerAddresses(),
		Log:         logger,
	}
	clientCfg.TLS, err = keyRing.TeleportClientTLSConfig(process.Config.CipherSuites, []string{clusterName})
	require.NoError(t, err)

	client, err := authclient.Connect(t.Context(), clientCfg)
	require.NoError(t, err)

	// Call the auth server to check the end-to-end connection is working.
	user, err := client.GetCurrentUser(t.Context())
	require.NoError(t, err)
	require.True(t, user.IsBot())
	require.Equal(t, "bot-test-bot", user.GetName())
	require.Contains(t, user.GetRoles(), role.GetName())

	cancel()
	require.NoError(t, <-errCh)
}
