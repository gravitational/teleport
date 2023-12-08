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
	"testing"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
)

// TestLocalProxyPostgres verifies connecting to a Postgres database
// through the local authenticated ALPN proxy.
func TestLocalProxyPostgres(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	// Create test user/role.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Try to connect to the database as this user.
	conn, proxy, err := testCtx.postgresClientLocalProxy(ctx, "alice", "postgres", "postgres", "postgres")
	require.NoError(t, err)

	// Close connection and local proxy after the test.
	t.Cleanup(func() {
		require.NoError(t, conn.Close(ctx))
		require.NoError(t, proxy.Close())
	})

	// Execute a query.
	results, err := conn.Exec(ctx, "select 1").ReadAll()
	require.NoError(t, err)
	require.Equal(t, []*pgconn.Result{postgres.TestQueryResponse}, results)

	// Execute a "long running" query and cancel it.
	resultReader := conn.Exec(ctx, postgres.TestLongRunningQuery)
	require.NoError(t, conn.CancelRequest(ctx))
	// timeout this test quickly if the cancel request fails to propagate.
	conn.Conn().SetDeadline(time.Now().Add(time.Second * 10))
	results, err = resultReader.ReadAll()
	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, pgerrcode.QueryCanceled, pgErr.Code)
	require.Empty(t, results)
}

// TestLocalProxyMySQL verifies connecting to a MySQL database
// through the local authenticated ALPN proxy.
func TestLocalProxyMySQL(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	// Create test user/role.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Connect to the database as this user.
	conn, proxy, err := testCtx.mysqlClientLocalProxy(ctx, "alice", "mysql", "alice")
	require.NoError(t, err)

	// Close connection and local proxy after the test.
	t.Cleanup(func() {
		require.NoError(t, conn.Close())
		require.NoError(t, proxy.Close())
	})

	// Execute a query.
	_, err = conn.Execute("select 1")
	require.NoError(t, err)
}

// TestLocalProxyMongoDB verifies connecting to a MongoDB database
// through the local authenticated ALPN proxy.
func TestLocalProxyMongoDB(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo"))
	go testCtx.startHandlingConnections()

	// Create test user/role.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Connect to the database as this user.
	client, proxy, err := testCtx.mongoClientLocalProxy(ctx, "alice", "mongo", "admin")
	require.NoError(t, err)

	// Close connection and local proxy after the test.
	t.Cleanup(func() {
		require.NoError(t, client.Disconnect(ctx))
		require.NoError(t, proxy.Close())
	})

	// Execute a query.
	_, err = client.Database("admin").Collection("test").Find(ctx, bson.M{})
	require.NoError(t, err)
}

// TestLocalProxyRedis verifies connecting to a Redis database
// through the local authenticated ALPN proxy.
func TestLocalProxyRedis(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	// Create test user/role.
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	// Connect to the database as this user.
	client, proxy, err := testCtx.redisClientLocalProxy(ctx, "alice", "redis", "admin")
	require.NoError(t, err)

	// Close connection and local proxy after the test.
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		require.NoError(t, proxy.Close())
	})

	// Execute a query.
	result := client.Echo(ctx, "ping")
	require.NoError(t, result.Err())
	require.Equal(t, "ping", result.Val())
}
