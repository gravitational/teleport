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
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/gocql/gocql"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	cassandra "github.com/gravitational/teleport/lib/srv/db/cassandra/testing"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func TestAccessCassandra(t *testing.T) {
	t.Parallel()

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
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, test.allowDbNames)

			// Try to connect to the database as this user.
			dbConn, err := testCtx.cassandraClient(ctx, test.user, "cassandra", test.dbUser)
			if test.err != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.err)
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

func TestAccessCassandraHandshake(t *testing.T) {
	t.Parallel()

	tests := []struct {
		protocolVersion primitive.ProtocolVersion
	}{
		{
			protocolVersion: primitive.ProtocolVersion5,
		},
		{
			protocolVersion: primitive.ProtocolVersion4,
		},
		{
			protocolVersion: primitive.ProtocolVersion3,
		},
	}

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withCassandra("cassandra"))
	go testCtx.startHandlingConnections()
	const teleportUser = "alice"
	const streamID = 101

	for _, tc := range tests {
		t.Run(tc.protocolVersion.String(), func(t *testing.T) {
			t.Run("successful", func(t *testing.T) {
				testCtx.createUserAndRole(ctx, t, teleportUser, "admin", []string{"cassandra"}, []string{types.Wildcard})
				cqlRawClient, err := testCtx.cassandraRawClient(ctx, teleportUser, "cassandra", "cassandra")
				require.NoError(t, err)
				err = cqlRawClient.InitiateHandshake(tc.protocolVersion, streamID)
				require.NoError(t, err)
				fr := frame.NewFrame(tc.protocolVersion, 102, &message.Query{
					Query: "select * from system.local where key='local'"},
				)
				_, err = cqlRawClient.SendAndReceive(fr)
				require.NoError(t, err)
				cqlRawClient.Close()
			})

			t.Run("auth error", func(t *testing.T) {
				testCtx.createUserAndRole(ctx, t, teleportUser, "admin", []string{"cassandra"}, []string{types.Wildcard})
				opts := []cassandra.ClientOptions{cassandra.WithCassandraUsername("unknown_user")}
				cqlRawClient, err := testCtx.cassandraRawClient(ctx, teleportUser, "cassandra", "unknown_user", opts...)
				require.NoError(t, err)
				err = cqlRawClient.InitiateHandshake(tc.protocolVersion, streamID)
				require.Error(t, err)
				require.Contains(t, err.Error(), "access to db denied")
				cqlRawClient.Close()
			})

			t.Run("user  doesn't exist", func(t *testing.T) {
				testCtx.createUserAndRole(ctx, t, teleportUser, "admin", []string{"unknown_user"}, []string{types.Wildcard})
				opts := []cassandra.ClientOptions{cassandra.WithCassandraUsername("unknown_user")}
				cqlRawClient, err := testCtx.cassandraRawClient(ctx, teleportUser, "cassandra", "unknown_user", opts...)
				require.NoError(t, err)
				err = cqlRawClient.InitiateHandshake(tc.protocolVersion, streamID)
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid credentials")
				cqlRawClient.Close()
			})
		})
	}
}

func TestAuditCassandra(t *testing.T) {
	t.Parallel()
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

func TestBatchCassandra(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withCassandra("cassandra"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"cassandra"}, []string{types.Wildcard})

	dbConn, err := testCtx.cassandraClient(ctx, "alice", "cassandra", "cassandra")
	require.NoError(t, err)
	t.Cleanup(dbConn.Close)

	batch := dbConn.NewBatch(gocql.LoggedBatch)
	batch.Query("INSERT INTO batch_table (id) VALUES 1")
	batch.Query("INSERT INTO batch_table (id) VALUES 2")

	err = dbConn.ExecuteBatch(batch)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		event := waitForEvent(t, testCtx, libevents.CassandraBatchEventCode)
		query, ok := event.(*events.CassandraBatch)
		if !ok {
			return false
		}
		if len(query.Children) != 2 {
			return false
		}
		require.Equal(t, "INSERT INTO batch_table (id) VALUES 1", query.Children[0].Query)
		require.Equal(t, "INSERT INTO batch_table (id) VALUES 2", query.Children[1].Query)
		return true

	}, 3*time.Second, time.Millisecond*100)
}

func TestEventCassandra(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withCassandra("cassandra"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"cassandra"}, []string{types.Wildcard})

	dbConn, err := testCtx.cassandraClient(ctx, "alice", "cassandra", "cassandra")
	require.NoError(t, err)
	t.Cleanup(dbConn.Close)

	waitForEvent(t, testCtx, libevents.CassandraRegisterEventCode)
}

func withCassandra(name string, opts ...cassandra.TestServerOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
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
