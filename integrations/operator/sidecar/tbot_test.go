/*
Copyright 2023 Gravitational, Inc.

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

package sidecar

import (
	"context"
	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestTbotJoinTwice(t *testing.T) {
	// Test setup: start a Teleport server and get an admin client
	ctx := context.Background()
	teleportServer, operatorName, roleName := defaultTeleportServiceConfig(t)
	require.NoError(t, teleportServer.Start())
	identityFilePath := helpers.MustCreateUserIdentityFile(t, teleportServer, operatorName, time.Hour)
	creds := client.LoadIdentityFile(identityFilePath)
	authClientConfig := new(authclient.Config)
	var err error
	authClientConfig.TLS, err = creds.TLSConfig()
	require.NoError(t, err)
	authClientConfig.AuthServers = teleportServer.Process.Config.AuthServerAddresses()
	log := logrus.StandardLogger()
	authClientConfig.Log = log
	authClient, err := authclient.Connect(ctx, authClientConfig)

	t.Cleanup(func() {
		err := teleportServer.StopAll()
		require.NoError(t, err)
	})

	t.Cleanup(func() {
		err := authClient.Close()
		require.NoError(t, err)
	})

	_, err = authClient.Ping(ctx)
	require.NoError(t, err)

	// Test setup: create a bot

	options := Options{
		Addr: teleportServer.Auth,
		Name: operatorName,
		Role: roleName,
	}

	bot := &Bot{
		running:    false,
		rootClient: authClient,
		opts:       options,
	}
	bot.initializeConfig(ctx)

	// Test part 1: we create the bot
	botCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	// To avoid dealing with concurrency and contexts, we configure the bot to run in one shot
	bot.cfg.Oneshot = true
	err = bot.Start(botCtx)
	assert.NoError(t, err)

	// We check that the bot built a valid client, we get it and ping the auth
	botClient, err := bot.GetClient(botCtx)
	require.NoError(t, err)
	_, err = botClient.Ping(botCtx)
	require.NoError(t, err)
	require.NoError(t, botClient.Close())

	// Test part 2: we reuse the bot we created to
	// This simulates a natural certificate renewal and should increase the cert generation counter

	realBot := tbot.New(bot.cfg, log)
	require.NoError(t, realBot.Run(ctx))

	// Test part 3: Simulate an operator restart by creating a new bot and joining again
	bot = &Bot{
		running:    false,
		rootClient: authClient,
		opts:       options,
	}

	bot.initializeConfig(ctx)

	bot.cfg.Oneshot = true
	err = bot.Start(botCtx)
	assert.NoError(t, err)

	// We check that the bot built a valid client, we get it and ping the auth
	botClient, err = bot.GetClient(botCtx)
	require.NoError(t, err)
	_, err = botClient.Ping(botCtx)
	require.NoError(t, err)
	require.NoError(t, botClient.Close())
}

func defaultTeleportServiceConfig(t *testing.T) (*helpers.TeleInstance, string, string) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			OIDC: true,
			SAML: true,
		},
	})

	teleportServer := helpers.NewInstance(t, helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Log:         logrus.StandardLogger(),
	})

	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.Version = "v2"

	roleName := "operator-role"
	role, err := sidecarRole(roleName)
	require.NoError(t, err)

	operatorName := "operator"
	_ = teleportServer.AddUserWithRole(operatorName, role)

	err = teleportServer.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	return teleportServer, operatorName, roleName
}
