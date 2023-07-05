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

package tbot

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	libconfig "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/tlsca"
	libutils "github.com/gravitational/teleport/lib/utils"
)

// TestOnboardViaToken ensures a bot can join using token auth.
func TestOnboardViaToken(t *testing.T) {
	t.Parallel()

	log := libutils.NewLoggerForTests()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Make and join a new bot instance.
	const roleName = "dummy-role"
	role, err := types.NewRole(roleName, types.RoleSpecV6{})
	require.NoError(t, err)
	require.NoError(t, rootClient.UpsertRole(context.Background(), role))

	botParams := testhelpers.MakeBot(t, rootClient, "test", roleName)
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)
	b := New(botConfig, log, nil)
	ident, err := b.getIdentityFromToken()
	require.NoError(t, err)

	tlsIdent, err := tlsca.FromSubject(ident.X509Cert.Subject, ident.X509Cert.NotAfter)
	require.NoError(t, err)

	require.True(t, tlsIdent.Renewable)
	require.False(t, tlsIdent.DisallowReissue)
	require.Equal(t, uint64(1), tlsIdent.Generation)
	require.ElementsMatch(t, []string{botParams.RoleName}, tlsIdent.Groups)

	// Make sure the bot identity actually works.
	botClient := testhelpers.MakeBotAuthClient(t, fc, ident)
	_, err = botClient.GetClusterName()
	require.NoError(t, err)
}

func TestDatabaseRequest(t *testing.T) {
	t.Parallel()

	log := libutils.NewLoggerForTests()
	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	fc.Databases.Databases = []*libconfig.Database{
		{
			Name:     "foo",
			Protocol: "mysql",
			URI:      "foo.example.com:1234",
			StaticLabels: map[string]string{
				"env": "dev",
			},
		},
	}
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Wait for the database to become available. Sometimes this takes a bit
	// of time in CI.
	for i := 0; i < 10; i++ {
		_, err := getDatabase(context.Background(), rootClient, "foo")
		if err == nil {
			break
		} else if !trace.IsNotFound(err) {
			require.NoError(t, err)
		}

		if i >= 9 {
			t.Fatalf("database never became available")
		}

		t.Logf("Database not yet available, waiting...")
		time.Sleep(time.Second * 1)
	}

	// Note: we don't actually need a role granting us database access to
	// request it. Actual access is validated via RBAC at connection time.
	// We do need an actual database and permission to list them, however.

	// Create a role to grant access to the database.
	const roleName = "db-role"
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			DatabaseLabels: types.Labels{
				"*": utils.Strings{"*"},
			},
			DatabaseNames: []string{"bar"},
			DatabaseUsers: []string{"baz"},
			Rules: []types.Rule{
				types.NewRule("db_server", []string{"read", "list"}),
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, rootClient.UpsertRole(context.Background(), role))

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, rootClient, "test", roleName)
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)

	dest := botConfig.Destinations[0]
	dest.Database = &config.Database{
		Service:  "foo",
		Database: "bar",
		Username: "baz",
	}

	// Onboard the bot.
	b := New(botConfig, log, nil)
	ident, err := b.getIdentityFromToken()
	require.NoError(t, err)

	b._client = testhelpers.MakeBotAuthClient(t, fc, ident)
	b._ident = ident

	impersonatedIdent, _, err := b.generateImpersonatedIdentity(
		context.Background(), ident.X509Cert.NotAfter, dest, []string{roleName},
	)
	require.NoError(t, err)

	tlsIdent, err := tlsca.FromSubject(impersonatedIdent.X509Cert.Subject, impersonatedIdent.X509Cert.NotAfter)
	require.NoError(t, err)

	route := tlsIdent.RouteToDatabase

	require.Equal(t, "foo", route.ServiceName)
	require.Equal(t, "bar", route.Database)
	require.Equal(t, "baz", route.Username)
	require.Equal(t, "mysql", route.Protocol)
}

func TestAppRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const appName = "foo"
	log := libutils.NewLoggerForTests()
	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	fc.Apps.Service = libconfig.Service{
		EnabledFlag: "true",
	}
	fc.Apps.Apps = []*libconfig.App{
		{
			Name:       appName,
			PublicAddr: "foo.example.com",
			URI:        "http://foo.example.com:1234",
			StaticLabels: map[string]string{
				"env": "dev",
			},
		},
	}
	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Wait for the app to become available. Sometimes this takes a bit
	// of time in CI.
	require.Eventually(t, func() bool {
		_, err := getApp(ctx, rootClient, appName)
		return err == nil
	}, 10*time.Second, 100*time.Millisecond)

	// Create a role to grant access to the app.
	const roleName = "app-role"
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"env": utils.Strings{"dev"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, rootClient.UpsertRole(ctx, role))

	// Make and join a new bot instance.
	botParams := testhelpers.MakeBot(t, rootClient, "test", roleName)
	botConfig := testhelpers.MakeMemoryBotConfig(t, fc, botParams)

	dest := botConfig.Destinations[0]
	dest.App = &config.App{
		App: appName,
	}

	// Onboard the bot.
	b := New(botConfig, log, nil)
	ident, err := b.getIdentityFromToken()
	require.NoError(t, err)

	b._client = testhelpers.MakeBotAuthClient(t, fc, ident)
	b._ident = ident

	impersonatedIdent, _, err := b.generateImpersonatedIdentity(
		ctx, ident.X509Cert.NotAfter, dest, []string{roleName},
	)
	require.NoError(t, err)

	tlsIdent, err := tlsca.FromSubject(impersonatedIdent.X509Cert.Subject, impersonatedIdent.X509Cert.NotAfter)
	require.NoError(t, err)

	route := tlsIdent.RouteToApp
	require.Equal(t, appName, route.Name)
	require.Equal(t, "foo.example.com", route.PublicAddr)
	require.NotEmpty(t, route.SessionID)
}
