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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	botconfig "github.com/gravitational/teleport/tool/tbot/config"
	"github.com/gravitational/teleport/tool/tbot/identity"
	"github.com/stretchr/testify/require"
)

func DefaultConfig(t *testing.T) *config.FileConfig {
	t.Helper()

	return &config.FileConfig{
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
				EnabledFlag: "true",
			},
			WebAddr: mustGetFreeLocalListenerAddr(t),
			TunAddr: mustGetFreeLocalListenerAddr(t),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: mustGetFreeLocalListenerAddr(t),
			},
		},
	}
}

func mustGetFreeLocalListenerAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().String()
}

// MakeAndRunTestAuthServer creates an auth server useful for testing purposes.
func MakeAndRunTestAuthServer(t *testing.T, fc *config.FileConfig) (auth *service.TeleportProcess) {
	t.Helper()

	var err error
	cfg := service.MakeDefaultConfig()
	require.NoError(t, config.ApplyFileConfig(fc, cfg))

	cfg.CachePolicy.Enabled = false
	cfg.Proxy.DisableWebInterface = true
	auth, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, auth.Start())

	t.Cleanup(func() {
		auth.Close()
	})

	eventCh := make(chan service.Event, 1)
	auth.WaitForEvent(auth.ExitContext(), service.AuthTLSReady, eventCh)
	select {
	case <-eventCh:
	case <-time.After(30 * time.Second):
		// in reality, the auth server should start *much* sooner than this.  we use a very large
		// timeout here because this isn't the kind of problem that this test is meant to catch.
		t.Fatal("auth server didn't start after 30s")
	}
	return auth
}

// MakeBotAuthClient creates a new auth client using a Bot identity.
func MakeBotAuthClient(t *testing.T, fc *config.FileConfig, ident *identity.Identity) auth.ClientI {
	t.Helper()

	cfg := service.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, cfg)
	require.NoError(t, err)

	authConfig := new(authclient.Config)
	authConfig.TLS, err = ident.TLSConfig(cfg.CipherSuites)
	require.NoError(t, err)

	authConfig.AuthServers = cfg.AuthServers
	authConfig.Log = cfg.Log

	client, err := authclient.Connect(context.Background(), authConfig)
	require.NoError(t, err)

	return client
}

// MakeDefaultAuthClient reimplements the bare minimum needed to create a
// default root-level auth client for a Teleport server started by
// MakeAndRunTestAuthServer.
func MakeDefaultAuthClient(t *testing.T, fc *config.FileConfig) auth.ClientI {
	t.Helper()

	cfg := service.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, cfg)
	require.NoError(t, err)

	cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
	require.NoError(t, err)

	identity, err := auth.ReadLocalIdentity(filepath.Join(cfg.DataDir, teleport.ComponentProcess), auth.IdentityID{Role: types.RoleAdmin, HostUUID: cfg.HostUUID})
	require.NoError(t, err)

	authConfig := new(authclient.Config)
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	require.NoError(t, err)

	authConfig.AuthServers = cfg.AuthServers
	authConfig.Log = cfg.Log

	client, err := authclient.Connect(context.Background(), authConfig)
	require.NoError(t, err)

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

	authCfg := service.MakeDefaultConfig()
	err := config.ApplyFileConfig(fc, authCfg)
	require.NoError(t, err)

	cfg := &botconfig.BotConfig{
		AuthServer: authCfg.AuthServers[0].String(),
		Onboarding: &botconfig.OnboardingConfig{
			JoinMethod: botParams.JoinMethod,
			Token:      botParams.TokenID,
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
	require.NoError(t, cfg.CheckAndSetDefaults())

	return cfg
}
