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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/native"
	libconfig "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/testhelpers"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	native.PrecomputeTestKeys(m)
	os.Exit(m.Run())
}

// TestBot is a one-shot run of the bot that communicates with a stood up
// in memory auth server. This auth server is configured with resources to
// support the testing of app/db destinations.
// This is effectively as end-to-end as tbot testing gets.
//
// TODO(noah): Make this test more extensible for testing different kinds of
// destination in future.
func TestBot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := utils.NewLoggerForTests()

	// Make a new auth server.
	fc, fds := testhelpers.DefaultConfig(t)
	const appName = "foo"
	fc.Apps = libconfig.Apps{
		Service: libconfig.Service{
			EnabledFlag: "true",
		},
		Apps: []*libconfig.App{
			{
				Name:       appName,
				PublicAddr: "foo.example.com",
				URI:        "http://foo.example.com:1234",
				StaticLabels: map[string]string{
					"env": "dev",
				},
			},
		},
	}
	fc.Databases = libconfig.Databases{
		Service: libconfig.Service{
			EnabledFlag: "true",
		},
		Databases: []*libconfig.Database{
			{
				Name:     "foo",
				Protocol: "mysql",
				URI:      "foo.example.com:1234",
				StaticLabels: map[string]string{
					"env": "dev",
				},
			},
		},
	}

	_ = testhelpers.MakeAndRunTestAuthServer(t, log, fc, fds)
	rootClient := testhelpers.MakeDefaultAuthClient(t, log, fc)

	// Wait for the app/db to become available. Sometimes this takes a bit
	// of time in CI.
	require.Eventually(t, func() bool {
		_, err := getApp(ctx, rootClient, appName)
		if err != nil {
			return false
		}
		_, err = getDatabase(ctx, rootClient, "foo")
		return err == nil
	}, 10*time.Second, 100*time.Millisecond)

	// Make and join a new bot instance.
	const roleName = "destination-role"
	role, err := types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				"env": apiutils.Strings{"dev"},
			},

			// Note: we don't actually need a role granting us database access to
			// request it. Actual access is validated via RBAC at connection time.
			// We do need an actual database and permission to list them, however.
			DatabaseLabels: types.Labels{
				"*": apiutils.Strings{"*"},
			},
			DatabaseNames: []string{"bar"},
			DatabaseUsers: []string{"baz"},
			Rules: []types.Rule{
				types.NewRule("db_server", []string{"read", "list"}),
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, rootClient.UpsertRole(ctx, role))

	botParams := testhelpers.MakeBot(t, rootClient, "test", roleName)
	botConfig := testhelpers.MakeMemoryBotConfig(
		t, fc, botParams, []*config.DestinationConfig{
			// Our first destination is pure identity
			{
				DestinationMixin: config.DestinationMixin{
					Memory: &config.DestinationMemory{},
				},
			},
			// Our second destination tests application access
			{
				DestinationMixin: config.DestinationMixin{
					Memory: &config.DestinationMemory{},
				},
				App: &config.App{
					App: appName,
				},
			},
			// Our third destination tests database access
			{
				DestinationMixin: config.DestinationMixin{
					Memory: &config.DestinationMemory{},
				},
				Database: &config.Database{
					Service:  "foo",
					Database: "bar",
					Username: "baz",
				},
			},
		},
	)
	b := New(botConfig, log)
	require.NoError(t, b.Run(ctx))

	t.Run("validate bot identity", func(t *testing.T) {
		// Some rough checks to ensure the bot identity used follows our
		// expected rules for bot identities.
		botIdent := b.ident()
		tlsIdent, err := tlsca.FromSubject(botIdent.X509Cert.Subject, botIdent.X509Cert.NotAfter)
		require.NoError(t, err)
		require.True(t, tlsIdent.Renewable)
		require.False(t, tlsIdent.DisallowReissue)
		require.Equal(t, uint64(1), tlsIdent.Generation)
		require.ElementsMatch(t, []string{botParams.RoleName}, tlsIdent.Groups)
	})

	t.Run("validate templates", func(t *testing.T) {
		// Check destinations filled as expected
		dest := botConfig.Destinations[0]
		destImpl, err := dest.GetDestination()
		require.NoError(t, err)

		for _, templateName := range config.GetRequiredConfigs() {
			cfg := dest.GetConfigByName(templateName)
			require.NotNilf(t, cfg, "template %q must exist", templateName)

			validateTemplate(t, cfg, destImpl)
		}
	})

	t.Run("validate app destination", func(t *testing.T) {
		dest := botConfig.Destinations[1]
		destImpl, err := dest.GetDestination()
		require.NoError(t, err)

		// Validate that the correct identity fields have been set
		route := tlsIdentFromDest(t, destImpl).RouteToApp
		require.Equal(t, appName, route.Name)
		require.Equal(t, "foo.example.com", route.PublicAddr)
		require.NotEmpty(t, route.SessionID)
	})

	t.Run("validate db destination", func(t *testing.T) {
		dest := botConfig.Destinations[2]
		destImpl, err := dest.GetDestination()
		require.NoError(t, err)

		// Validate that the correct identity fields have been set
		route := tlsIdentFromDest(t, destImpl).RouteToDatabase
		require.Equal(t, "foo", route.ServiceName)
		require.Equal(t, "bar", route.Database)
		require.Equal(t, "baz", route.Username)
		require.Equal(t, "mysql", route.Protocol)
	})
}

func tlsIdentFromDest(t *testing.T, dest bot.Destination) *tlsca.Identity {
	t.Helper()
	keyBytes, err := dest.Read(identity.PrivateKeyKey)
	require.NoError(t, err)
	certBytes, err := dest.Read(identity.TLSCertKey)
	require.NoError(t, err)
	hostCABytes, err := dest.Read(config.DefaultHostCAPath)
	require.NoError(t, err)
	ident := &identity.Identity{}
	err = identity.ReadTLSIdentityFromKeyPair(ident, keyBytes, certBytes, [][]byte{hostCABytes})
	require.NoError(t, err)

	tlsIdent, err := tlsca.FromSubject(
		ident.X509Cert.Subject, ident.X509Cert.NotAfter,
	)
	require.NoError(t, err)
	return tlsIdent
}
