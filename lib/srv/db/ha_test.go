/*
Copyright 2021 Gravitational, Inc.

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

package db

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"

	"github.com/stretchr/testify/require"
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
	offlineResource, err := types.NewDatabaseServerV3(types.Metadata{
		Name: offlineHostID,
	}, types.DatabaseServerSpecV3{
		HostID:    offlineHostID,
		Hostname:  constants.APIDomain,
		Databases: []*types.DatabaseV3{offlineDB},
	})
	require.NoError(t, err)
	testCtx.setupDatabaseServer(ctx, t, offlineResource)
	testCtx.fakeRemoteSite.OfflineTunnels = map[string]struct{}{
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
	onlineResource, err := types.NewDatabaseServerV3(types.Metadata{
		Name: onlineHostID,
	}, types.DatabaseServerSpecV3{
		HostID:    onlineHostID,
		Hostname:  constants.APIDomain,
		Databases: []*types.DatabaseV3{onlineDB},
	})
	require.NoError(t, err)
	onlineServer := testCtx.setupDatabaseServer(ctx, t, onlineResource)
	go func() {
		for conn := range testCtx.proxyConn {
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
