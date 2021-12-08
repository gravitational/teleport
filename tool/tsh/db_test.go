/*
Copyright 2015-2017 Gravitational, Inc.

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

package main

import (
	"context"
	"encoding/pem"
	"os"
	"testing"
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

// TestDatabaseLogin verifies "tsh db login" command.
func TestDatabaseLogin(t *testing.T) {
	os.RemoveAll(profile.FullProfilePath(""))
	t.Cleanup(func() {
		os.RemoveAll(profile.FullProfilePath(""))
	})

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	makeTestDatabaseServer(t, authProcess, proxyProcess, service.Database{
		Name:     "postgres",
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	}, service.Database{
		Name:     "mongo",
		Protocol: defaults.ProtocolMongoDB,
		URI:      "localhost:27017",
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run([]string{
		"login", "--insecure", "--debug", "--auth", connector.GetName(), "--proxy", proxyAddr.String(),
	}, cliOption(func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	}))
	require.NoError(t, err)

	// Fetch the active profile.
	profile, err := client.StatusFor("", proxyAddr.Host(), alice.GetName())
	require.NoError(t, err)

	// Log into test Postgres database.
	err = Run([]string{
		"db", "login", "--debug", "postgres",
	})
	require.NoError(t, err)

	// Verify Postgres identity file contains certificate.
	certs, keys, err := decodePEM(profile.DatabaseCertPath("postgres"))
	require.NoError(t, err)
	require.Len(t, certs, 1)
	require.Len(t, keys, 0)

	// Log into test Mongo database.
	err = Run([]string{
		"db", "login", "--debug", "--db-user", "admin", "mongo",
	})
	require.NoError(t, err)

	// Verify Mongo identity file contains both certificate and key.
	certs, keys, err = decodePEM(profile.DatabaseCertPath("mongo"))
	require.NoError(t, err)
	require.Len(t, certs, 1)
	require.Len(t, keys, 1)
}

func makeTestDatabaseServer(t *testing.T, auth *service.TeleportProcess, proxy *service.TeleportProcess, dbs ...service.Database) (db *service.TeleportProcess) {
	// Proxy uses self-signed certificates in tests.
	lib.SetInsecureDevMode(true)

	cfg := service.MakeDefaultConfig()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()

	proxyAddr, err := proxy.ProxyWebAddr()
	require.NoError(t, err)

	cfg.AuthServers = []utils.NetAddr{*proxyAddr}
	cfg.Token = proxy.Config.Token
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = false
	cfg.Databases.Enabled = true
	cfg.Databases.Databases = dbs
	cfg.Log = utils.NewLoggerForTests()

	db, err = service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, db.Start())

	t.Cleanup(func() {
		db.Close()
	})

	// Wait for database agent to start.
	eventCh := make(chan service.Event, 1)
	db.WaitForEvent(db.ExitContext(), service.DatabasesReady, eventCh)
	select {
	case <-eventCh:
	case <-time.After(10 * time.Second):
		t.Fatal("database server didn't start after 10s")
	}

	// Wait for all databases to register to avoid races.
	for _, database := range dbs {
		waitForDatabase(t, auth, database)
	}

	return db
}

func waitForDatabase(t *testing.T, auth *service.TeleportProcess, db service.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-time.After(500 * time.Millisecond):
			all, err := auth.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			for _, a := range all {
				if a.GetName() == db.Name {
					return
				}
			}
		case <-ctx.Done():
			t.Fatal("database not registered after 10s")
		}
	}
}

// decodePEM sorts out specified PEM file into certificates and private keys.
func decodePEM(pemPath string) (certs []pem.Block, keys []pem.Block, err error) {
	bytes, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var block *pem.Block
	for {
		block, bytes = pem.Decode(bytes)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			certs = append(certs, *block)
		case "RSA PRIVATE KEY":
			keys = append(keys, *block)
		}
	}
	return certs, keys, nil
}
