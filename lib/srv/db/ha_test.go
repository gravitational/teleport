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

package db

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
)

// TestHA tests scenario with multiple database services proxying the same database.
func TestHA(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	go testCtx.startProxy()

	// Start test Postgres server.
	postgresServer, err := postgres.NewTestServer(common.TestServerConfig{
		Name:       "postgres",
		AuthClient: testCtx.authClient,
	})
	require.NoError(t, err)
	go postgresServer.Serve()
	t.Cleanup(func() { postgresServer.Close() })

	// Offline database server will be tried first and trigger connection error.
	offlineHostID := "host-id-1"
	offlineDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresServer.Port()),
	})
	require.NoError(t, err)
	testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: types.Databases{offlineDB},
		HostID:    offlineHostID,
	})
	testCtx.fakeCluster.OfflineTunnels = map[string]struct{}{
		fmt.Sprintf("%v.%v", offlineHostID, testCtx.clusterName): {},
	}

	// Online database server will serve the connection.
	onlineHostID := "host-id-2"
	onlineDB, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      net.JoinHostPort("localhost", postgresServer.Port()),
	})
	require.NoError(t, err)
	onlineServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: types.Databases{onlineDB},
		HostID:    onlineHostID,
	})
	go func() {
		for conn := range testCtx.fakeCluster.ProxyConn() {
			go onlineServer.HandleConnection(conn)
		}
	}()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"postgres"}, []string{"postgres"})

	// Make sure we can connect successfully.
	psql, err := testCtx.postgresClient(ctx, "alice", "postgres", "postgres", "postgres")
	require.NoError(t, err)

	err = psql.Close(ctx)
	require.NoError(t, err)
}
