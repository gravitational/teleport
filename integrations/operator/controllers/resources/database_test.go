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

package resources

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
)

func startPostgresTestServer(t *testing.T, authServer *auth.Server) *postgres.TestServer {
	postgresTestServer, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: authServer,
	})
	require.NoError(t, err)

	go func() {
		t.Logf("Postgres Fake server running at %s port", postgresTestServer.Port())
		assert.NoError(t, postgresTestServer.Serve())
	}()
	t.Cleanup(func() {
		postgresTestServer.Close()
	})

	return postgresTestServer
}

func TestDiagnoseConnectionForPostgresDatabases(t *testing.T) {
	modules.SetInsecureTestMode(true)

	ctx := context.Background()

	// Start Teleport Auth and Proxy services
	authProcess, proxyProcess, provisionToken := helpers.MakeTestServers(t)
	authServer := authProcess.GetAuthServer()
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Start Fake Postgres Database
	postgresTestServer := startPostgresTestServer(t, authServer)

	// Start Teleport Database Service
	databaseResourceName := "mypsqldb"
	databaseDBName := "dbname"
	databaseDBUser := "dbuser"
	helpers.MakeTestDatabaseServer(t, *proxyAddr, provisionToken, nil /* resource matchers */, servicecfg.Database{
		Name:     databaseResourceName,
		Protocol: defaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresTestServer.Port()),
	})
	// Wait for the Database Server to be registered
	waitForDatabases(t, func(ctx context.Context, name string) ([]types.DatabaseServer, error) {
		return authServer.GetDatabaseServers(ctx, name)
	}, databaseResourceName)

	roleWithFullAccess, err := types.NewRole("fullaccess", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces:     []string{"default"},
			DatabaseLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			Rules: []types.Rule{
				types.NewRule(types.KindConnectionDiagnostic, services.RW()),
			},
			DatabaseUsers: []string{databaseDBUser},
			DatabaseNames: []string{databaseDBName},
		},
	})
	require.NoError(t, err)
	roleWithFullAccess, err = authServer.UpsertRole(ctx, roleWithFullAccess)
	require.NoError(t, err)
}

func waitForDatabases(t *testing.T, GetDatabaseServers func(ctx context.Context, name string) ([]types.DatabaseServer, error), dbNames ...string) {
	ctx := context.Background()

	require.Eventually(t, func() bool {
		all, err := GetDatabaseServers(ctx, "default")
		assert.NoError(t, err)

		if len(dbNames) > len(all) {
			return false
		}

		registered := 0
		for _, db := range dbNames {
			for _, a := range all {
				if a.GetName() == db {
					registered++
					break
				}
			}
		}
		return registered == len(dbNames)
	}, 30*time.Second, 100*time.Millisecond)
}
