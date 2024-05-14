/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package testhelpers

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	botconfig "github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

type DefaultBotConfigOpts struct {
	// Makes the bot connect via the Auth Server instead of the Proxy server.
	UseAuthServer bool

	// Makes the bot accept an Insecure auth or proxy server
	Insecure bool

	ServiceConfigs botconfig.ServiceConfigs
}

const AgentJoinToken = "i-am-a-join-token"

// DefaultConfig returns a FileConfig to be used in tests, with random listen
// addresses that are tied to the listeners returned in the FileDescriptor
// slice, which should be passed as exported file descriptors to NewTeleport;
// this is to ensure that we keep the listening socket open, to prevent other
// processes from using the same port before we're done with it.
func DefaultConfig(t *testing.T) (*config.FileConfig, []*servicecfg.FileDescriptor) {
	var fds []*servicecfg.FileDescriptor

	fc := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: testenv.NewTCPListener(t, service.ListenerProxySSH, &fds),
			},
			WebAddr:    testenv.NewTCPListener(t, service.ListenerProxyWeb, &fds),
			TunAddr:    testenv.NewTCPListener(t, service.ListenerProxyTunnel, &fds),
			KubeAddr:   testenv.NewTCPListener(t, service.ListenerProxyKube, &fds),
			PublicAddr: []string{"localhost"}, // ListenerProxyWeb port will be appended
		},
		Auth: config.Auth{
			ClusterName: "localhost",
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: testenv.NewTCPListener(t, service.ListenerAuth, &fds),
			},
			StaticTokens: config.StaticTokens{
				config.StaticToken("db:" + AgentJoinToken),
			},
		},
	}

	return fc, fds
}

// MakeAndRunTestAuthServer creates an auth server useful for testing purposes.
func MakeAndRunTestAuthServer(t *testing.T, log *slog.Logger, fc *config.FileConfig, fds []*servicecfg.FileDescriptor) (auth *service.TeleportProcess) {
	t.Helper()

	var err error
	cfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))
	cfg.FileDescriptors = fds
	cfg.Logger = log
	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true

	// Disable session recording to avoid flakiness caused by TempDir cleanup.
	cfg.Auth.SessionRecordingConfig.SetMode(types.RecordOff)
	// Disable audit log as we don't rely on this in our tests and it can cause
	// flakiness due to TempDir cleanup.
	cfg.Auth.NoAudit = true

	auth, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, auth.Start())

	t.Cleanup(func() {
		cfg.Logger.InfoContext(context.Background(), "Cleaning up Auth Server.")
		auth.Close()
	})

	_, err = auth.WaitForEventTimeout(30*time.Second, service.AuthTLSReady)
	// in reality, the auth server should start *much* sooner than this.  we use a very large
	// timeout here because this isn't the kind of problem that this test is meant to catch.
	require.NoError(t, err, "auth server didn't start after 30s")

	return auth
}

// MakeDefaultAuthClient reimplements the bare minimum needed to create a
// default root-level auth client for a Teleport server started by
// MakeAndRunTestAuthServer.
func MakeDefaultAuthClient(t *testing.T, fc *config.FileConfig) *auth.Client {
	t.Helper()

	cfg := servicecfg.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, cfg)
	require.NoError(t, err)

	cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
	require.NoError(t, err)

	identity, err := auth.ReadLocalIdentity(filepath.Join(cfg.DataDir, teleport.ComponentProcess), auth.IdentityID{Role: types.RoleAdmin, HostUUID: cfg.HostUUID})
	require.NoError(t, err)

	authConfig := new(authclient.Config)
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	require.NoError(t, err)

	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = utils.NewLogger()

	client, err := authclient.Connect(context.Background(), authConfig)
	require.NoError(t, err)

	// Wait for the server to become available.
	require.Eventually(t, func() bool {
		ping, err := client.Ping(context.Background())
		if err != nil {
			t.Logf("auth server is not yet available")
			return false
		}

		// Make sure the returned proxy address is sane, it should at least be
		// parseable.
		_, _, err = utils.SplitHostPort(ping.ProxyPublicAddr)
		if err != nil {
			t.Logf("proxy public address is not yet valid")
			return false
		}

		return true
	}, time.Second*10, time.Millisecond*250)

	return client
}

// MakeBot creates a server-side bot and returns joining parameters.
func MakeBot(t *testing.T, client *auth.Client, name string, roles ...string) (*botconfig.OnboardingConfig, *machineidv1pb.Bot) {
	ctx := context.TODO()
	t.Helper()

	b, err := client.BotServiceClient().CreateBot(ctx, &machineidv1pb.CreateBotRequest{
		Bot: &machineidv1pb.Bot{
			Metadata: &headerv1.Metadata{
				Name: name,
			},
			Spec: &machineidv1pb.BotSpec{
				Roles: roles,
			},
		},
	})
	require.NoError(t, err)

	tokenName, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	require.NoError(t, err)
	tok, err := types.NewProvisionTokenFromSpec(
		tokenName,
		time.Now().Add(10*time.Minute),
		types.ProvisionTokenSpecV2{
			Roles:   []types.SystemRole{types.RoleBot},
			BotName: b.Metadata.Name,
		})
	require.NoError(t, err)
	err = client.CreateToken(ctx, tok)
	require.NoError(t, err)

	return &botconfig.OnboardingConfig{
		TokenValue: tok.GetName(),
		JoinMethod: types.JoinMethodToken,
	}, b
}

// DefaultBotConfig creates a usable bot config from joining parameters.
// By default it:
// - Has the outputs provided to it via the parameter `outputs`
// - Runs in oneshot mode
// - Uses a memory storage destination
// - Does not verify Proxy WebAPI certificates
func DefaultBotConfig(
	t *testing.T,
	fc *config.FileConfig,
	onboarding *botconfig.OnboardingConfig,
	outputs []botconfig.Output,
	opts DefaultBotConfigOpts,
) *botconfig.BotConfig {
	t.Helper()

	authCfg := servicecfg.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, authCfg)
	require.NoError(t, err)

	var authServer = authCfg.Proxy.WebAddr.String()
	if opts.UseAuthServer {
		authServer = authCfg.AuthServerAddresses()[0].String()
	}

	cfg := &botconfig.BotConfig{
		AuthServer: authServer,
		Onboarding: *onboarding,
		Storage: &botconfig.StorageConfig{
			Destination: &botconfig.DestinationMemory{},
		},
		Oneshot: true,
		Outputs: outputs,
		// Set Insecure so the bot will trust the Proxy's webapi default signed
		// certs.
		Insecure: opts.Insecure,
		Services: opts.ServiceConfigs,
	}

	require.NoError(t, cfg.CheckAndSetDefaults())

	return cfg
}
