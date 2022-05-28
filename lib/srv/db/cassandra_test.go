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

package db

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/cassandra"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/stretchr/testify/require"
)

func TestAccessCassandra(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withCassandra("cassandra"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbNames []string
		allowDbUsers []string
		dbName       string
		dbUser       string
		err          string
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "cassandra",
			dbUser:       "cassandra",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "cassandra",
			dbUser:       "cassandra",
			err:          "access to db denied",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{},
			dbName:       "cassandra",
			dbUser:       "cassandra",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"cassandra"},
			dbName:       "metrics",
			dbUser:       "cassandra",
		},
		{
			desc:         "access denied to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "cassandra",
			dbUser:       "cassandra",
			err:          "access to db denied",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, test.allowDbNames)

			// Try to connect to the database as this user.
			dbConn, err := testCtx.cassandraClient(ctx, test.user, "cassandra", test.dbUser)
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}
			require.NoError(t, err)

			// Execute a query.
			var clusterName string
			err = dbConn.Query("select cluster_name from system.local").Scan(&clusterName)
			require.NoError(t, err)
			require.Equal(t, "Test Cluster", clusterName)

			// Disconnect.
			dbConn.Close()
		})
	}
}

func TestAuditCassandra(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withCassandra("cassandra"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"cassandra"}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		// Access denied should trigger an unsuccessful session start event.
		_, err := testCtx.cassandraClient(ctx, "alice", "cassandra", "nonexistent")
		require.Error(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
	})

	var dbConn *cassandra.Session
	t.Run("session starts event", func(t *testing.T) {
		// Connect should trigger successful session start event.
		var err error

		dbConn, err = testCtx.cassandraClient(ctx, "alice", "cassandra", "cassandra")
		require.NoError(t, err)

		waitForEvent(t, testCtx, libevents.DatabaseSessionStartCode)
	})

	t.Run("command sends", func(t *testing.T) {
		// Select should trigger Query event.
		var clusterName string
		err := dbConn.Query("select cluster_name from system.local").Scan(&clusterName)
		require.NoError(t, err)
		require.Equal(t, "Test Cluster", clusterName)

		waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
	})

	t.Run("session ends event", func(t *testing.T) {
		// Closing connection should trigger session end event.
		dbConn.Close()
		waitForEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

func withCassandra(name string, opts ...cassandra.TestServerOption) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		cassandraServer, err := cassandra.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}, opts...)
		require.NoError(t, err)
		go cassandraServer.Serve()
		t.Cleanup(func() { cassandraServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolCassandra,
			URI:           net.JoinHostPort("localhost", cassandraServer.Port()),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.cassandra[name] = testCassandra{
			db:       cassandraServer,
			resource: database,
		}
		return database
	}
}
