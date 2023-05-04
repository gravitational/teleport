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
	"net"
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
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/utils"
)

// from lib/service/listeners.go
// TODO(espadolini): have the constants exported
const (
	listenerAuth        = "auth"
	listenerProxySSH    = "proxy:ssh"
	listenerProxyWeb    = "proxy:web"
	listenerProxyTunnel = "proxy:tunnel"
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
		Databases: config.Databases{
			Service: config.Service{
				EnabledFlag: "true",
			},
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: newListener(t, listenerProxySSH, &fds),
			},
			WebAddr:    newListener(t, listenerProxyWeb, &fds),
			TunAddr:    newListener(t, listenerProxyTunnel, &fds),
			PublicAddr: []string{"proxy.example.com"},
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: newListener(t, listenerAuth, &fds),
			},
		},
	}

	return fc, fds
}

// newListener creates a new TCP listener on 127.0.0.1:0, adds it to the
// FileDescriptor slice (with the specified type) and returns its actual local
// address as a string (for use in configuration). Takes a pointer to the slice
// so that it's convenient to call in the middle of a FileConfig or Config
// struct literal.
// TODO(espadolini): move this to a more generic place so we can use the same
// approach in other tests that spin up a TeleportProcess
func newListener(t *testing.T, ty string, fds *[]servicecfg.FileDescriptor) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	addr := l.Addr().String()

	// File() returns a dup of the listener's file descriptor as an *os.File, so
	// the original net.Listener still needs to be closed.
	lf, err := l.(*net.TCPListener).File()
	require.NoError(t, err)
	// If the file descriptor slice ends up being passed to a TeleportProcess
	// that successfully starts, listeners will either get "imported" and used
	// or discarded and closed, this is just an extra safety measure that closes
	// the listener at the end of the test anyway (the finalizer would do that
	// anyway, in principle).
	t.Cleanup(func() { lf.Close() })

	*fds = append(*fds, servicecfg.FileDescriptor{
		Type:    ty,
		Address: addr,
		File:    lf,
	})

	return addr
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

// MakeBotAuthClient creates a new auth client using a Bot identity.
func MakeBotAuthClient(t *testing.T, fc *config.FileConfig, ident *identity.Identity) auth.ClientI {
	t.Helper()

	cfg := servicecfg.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, cfg)
	require.NoError(t, err)

	authConfig := new(authclient.Config)
	authConfig.TLS, err = ident.TLSConfig(cfg.CipherSuites)
	require.NoError(t, err)

	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Log

	client, err := authclient.Connect(context.Background(), authConfig)
	require.NoError(t, err)

	return client
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

// MakeMemoryBotConfig creates a usable bot config from joining parameters. It
// only writes artifacts to memory and can be further modified if desired.
func MakeMemoryBotConfig(t *testing.T, fc *config.FileConfig, botParams *proto.CreateBotResponse) *botconfig.BotConfig {
	t.Helper()

	authCfg := servicecfg.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, authCfg)
	require.NoError(t, err)

	cfg := &botconfig.BotConfig{
		AuthServer: authCfg.AuthServerAddresses()[0].String(),
		Onboarding: &botconfig.OnboardingConfig{
			JoinMethod: botParams.JoinMethod,
		},
		Storage: &botconfig.StorageConfig{
			DestinationMixin: botconfig.DestinationMixin{
				Memory: &botconfig.DestinationMemory{},
			},
		},
		Destinations: []*botconfig.DestinationConfig{
			{
				DestinationMixin: botconfig.DestinationMixin{
					Memory: &botconfig.DestinationMemory{},
				},
			},
		},
	}

	cfg.Onboarding.SetToken(botParams.TokenID)

	require.NoError(t, cfg.CheckAndSetDefaults())

	return cfg
}
