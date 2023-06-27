/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testhelpers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	botconfig "github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// DefaultConfig returns a FileConfig to be used in tests, with random listen
// addresses that are tied to the listeners returned in the FileDescriptor
// slice, which should be passed as exported file descriptors to NewTeleport;
// this is to ensure that we keep the listening socket open, to prevent other
// processes from using the same port before we're done with it.
func DefaultConfig(t *testing.T) (*config.FileConfig, []servicecfg.FileDescriptor) {
	var fds []servicecfg.FileDescriptor

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
			PublicAddr: []string{"localhost"}, // ListenerProxyWeb port will be appended
		},
		Auth: config.Auth{
			ClusterName: "localhost",
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: testenv.NewTCPListener(t, service.ListenerAuth, &fds),
			},
		},
	}

	return fc, fds
}

// MakeAndRunTestAuthServer creates an auth server useful for testing purposes.
func MakeAndRunTestAuthServer(t *testing.T, log utils.Logger, fc *config.FileConfig, fds []servicecfg.FileDescriptor) (auth *service.TeleportProcess) {
	t.Helper()

	var err error
	cfg := servicecfg.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))
	cfg.FileDescriptors = fds
	cfg.Log = log

	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true
	auth, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, auth.Start())

	t.Cleanup(func() {
		cfg.Log.Info("Cleaning up Auth Server.")
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
func MakeDefaultAuthClient(t *testing.T, log utils.Logger, fc *config.FileConfig) auth.ClientI {
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
	authConfig.Log = log

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
func MakeBot(t *testing.T, client auth.ClientI, name string, roles ...string) *proto.CreateBotResponse {
	t.Helper()

	bot, err := client.CreateBot(context.Background(), &proto.CreateBotRequest{
		Name:  name,
		Roles: roles,
	})

	require.NoError(t, err)
	return bot
}

// DefaultBotConfig creates a usable bot config from joining parameters.
// By default it:
// - Has the outputs provided to it via the parameter `outputs`
// - Runs in oneshot mode
// - Uses a memory storage destination
// - Does not verify Proxy WebAPI certificates
func DefaultBotConfig(
	t *testing.T, fc *config.FileConfig, botParams *proto.CreateBotResponse, outputs []botconfig.Output,
) *botconfig.BotConfig {
	t.Helper()

	authCfg := servicecfg.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, authCfg)
	require.NoError(t, err)

	cfg := &botconfig.BotConfig{
		AuthServer: authCfg.AuthServerAddresses()[0].String(),
		Onboarding: botconfig.OnboardingConfig{
			JoinMethod: botParams.JoinMethod,
		},
		Storage: &botconfig.StorageConfig{
			Destination: &botconfig.DestinationMemory{},
		},
		Oneshot: true,
		Outputs: outputs,
		// Set Insecure so the bot will trust the Proxy's webapi default signed
		// certs.
		Insecure: true,
	}

	cfg.Onboarding.SetToken(botParams.TokenID)

	require.NoError(t, cfg.CheckAndSetDefaults())

	return cfg
}
