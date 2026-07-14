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

package login

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	loginagentv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginagent/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekeyagent"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/hardwarekey"
	"github.com/gravitational/teleport/lib/localagent"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestAgentService(t *testing.T) {
	t.Parallel()

	logger := logtest.NewLogger()

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

	role, err := types.NewRole("empty-role", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = rootClient.CreateRole(t.Context(), role)
	require.NoError(t, err)

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
			AgentServiceBuilder(&AgentConfig{
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

	waitForLocalAgent(t, outputDir, errCh)

	conn, err := localagent.NewClient(outputDir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	loginResp, err := loginagentv1.NewLoginAgentServiceClient(conn).
		Login(t.Context(), &loginagentv1.LoginRequest{})
	require.NoError(t, err)
	require.Equal(t, "bot-test-bot", loginResp.GetUsername())
	require.NotEmpty(t, loginResp.GetSshCert())
	require.NotEmpty(t, loginResp.GetTlsCert())
	require.NotEmpty(t, loginResp.GetPrivateKey())
	require.NotEmpty(t, loginResp.GetHostSigners())

	hwksClient, err := hardwarekey.NewAgentClient(t.Context(), outputDir)
	require.NoError(t, err)
	hwks := hardwarekeyagent.NewService(hwksClient, nil /* fallbackService */)

	privateKey, err := keys.ParsePrivateKey(
		loginResp.GetPrivateKey(),
		keys.WithHardwareKeyService(hwks),
	)
	require.NoError(t, err)

	trustedCerts := make([]authclient.TrustedCerts, len(loginResp.GetHostSigners()))
	for idx, cert := range loginResp.GetHostSigners() {
		trustedCerts[idx] = authclient.TrustedCerts{
			ClusterName:     cert.GetClusterName(),
			AuthorizedKeys:  cert.GetSshAuthorizedKeys(),
			TLSCertificates: cert.GetTlsCaCerts(),
		}
	}

	keyRing := &client.KeyRing{
		SSHPrivateKey: privateKey,
		Cert:          loginResp.GetSshCert(),
		TLSPrivateKey: privateKey,
		TLSCert:       loginResp.GetTlsCert(),
		TrustedCerts:  trustedCerts,
	}

	clusterName := process.Config.Auth.ClusterName.GetClusterName()
	clientCfg := &authclient.Config{
		AuthServers: process.Config.AuthServerAddresses(),
		Log:         logger,
	}
	clientCfg.TLS, err = keyRing.TeleportClientTLSConfig(process.Config.CipherSuites, []string{clusterName})
	require.NoError(t, err)

	authClient, err := authclient.Connect(t.Context(), clientCfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authClient.Close()) })

	user, err := authClient.GetCurrentUser(t.Context())
	require.NoError(t, err)
	require.True(t, user.IsBot())
	require.Equal(t, "bot-test-bot", user.GetName())
	require.Contains(t, user.GetRoles(), role.GetName())

	cancel()
	require.NoError(t, <-errCh)
}

func waitForLocalAgent(t *testing.T, outputDir string, errCh <-chan error) {
	t.Helper()

	checkTicker := time.NewTicker(250 * time.Millisecond)
	t.Cleanup(checkTicker.Stop)

	checkTimeout := time.NewTimer(20 * time.Second)
	t.Cleanup(func() { _ = checkTimeout.Stop() })

	socketPath := filepath.Join(outputDir, localagent.SocketFileName)
	certPath := filepath.Join(outputDir, localagent.CertFileName)

	for {
		select {
		case <-checkTicker.C:
			if _, err := os.Stat(socketPath); err != nil {
				continue
			}
			if _, err := os.Stat(certPath); err == nil {
				return
			}
		case err := <-errCh:
			require.NoError(t, err, "bot exited with an error")
		case <-checkTimeout.C:
			t.Fatal("timeout waiting for login agent to start")
		}
	}
}

func defaultTestServerOpts(log *slog.Logger) testenv.TestServerOptFunc {
	return func(o *testenv.TestServersOpts) error {
		testenv.WithClusterName("root")(o)
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Logger = log
			cfg.Proxy.PublicAddrs = []utils.NetAddr{
				{AddrNetwork: "tcp", Addr: net.JoinHostPort("localhost", strconv.Itoa(cfg.Proxy.WebAddr.Port(0)))},
			}
			cfg.Proxy.TunnelPublicAddrs = []utils.NetAddr{
				cfg.Proxy.ReverseTunnelListenAddr,
			}
		})(o)

		return nil
	}
}

func makeBot(t *testing.T, client *authclient.Client, name string, roles ...string) onboarding.Config {
	t.Helper()

	b, err := client.BotServiceClient().CreateBot(t.Context(), machineidv1pb.CreateBotRequest_builder{
		Bot: machineidv1pb.Bot_builder{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: name,
			}.Build(),
			Spec: machineidv1pb.BotSpec_builder{
				Roles: roles,
			}.Build(),
		}.Build(),
	}.Build())
	require.NoError(t, err)

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	require.NoError(t, err)

	tok, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(10*time.Minute),
		types.ProvisionTokenSpecV2{
			Roles:   []types.SystemRole{types.RoleBot},
			BotName: b.GetMetadata().GetName(),
		})
	require.NoError(t, err)

	err = client.CreateToken(t.Context(), tok)
	require.NoError(t, err)

	return onboarding.Config{
		TokenValue: tok.GetName(),
		JoinMethod: types.JoinMethodToken,
	}
}
