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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/redis"
)

// TestAuditPostgres verifies proper audit events are emitted for Postgres
// connections.
func TestAuditPostgres(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"postgres"}, []string{"postgres"})

	// Access denied should trigger an unsuccessful session start event.
	_, err := testCtx.postgresClient(ctx, "alice", "postgres", "notpostgres", "notpostgres")
	require.Error(t, err)
	requireEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)

	// Connect should trigger successful session start event.
	psql, err := testCtx.postgresClient(ctx, "alice", "postgres", "postgres", "postgres")
	require.NoError(t, err)
	requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)

	// Simple query should trigger the query event.
	_, err = psql.Exec(ctx, "select 1").ReadAll()
	require.NoError(t, err)
	requireQueryEvent(t, testCtx, libevents.DatabaseSessionQueryCode, "select 1")

	// Execute unnamed prepared statement.
	resultUnnamed := psql.ExecParams(ctx, "select now()", nil, nil, nil, nil).Read()
	require.NoError(t, resultUnnamed.Err)
	requireEvent(t, testCtx, libevents.PostgresParseCode)
	requireEvent(t, testCtx, libevents.PostgresBindCode)
	requireEvent(t, testCtx, libevents.PostgresExecuteCode)

	// Execute named prepared statement.
	_, err = psql.Prepare(ctx, "test-stmt", "select 1", nil)
	require.NoError(t, err)
	resultNamed := psql.ExecPrepared(ctx, "test-stmt", nil, nil, nil)
	require.NoError(t, resultNamed.Read().Err)
	requireEvent(t, testCtx, libevents.PostgresParseCode)
	requireEvent(t, testCtx, libevents.PostgresBindCode)
	requireEvent(t, testCtx, libevents.PostgresExecuteCode)

	// Closing connection should trigger session end event.
	err = psql.Close(ctx)
	require.NoError(t, err)
	requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
}

// TestAuditMySQL verifies proper audit events are emitted for MySQL
// connections.
func TestAuditMySQL(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})

	// Access denied should trigger an unsuccessful session start event.
	_, err := testCtx.mysqlClient("alice", "mysql", "notroot")
	require.Error(t, err)
	requireEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)

	// Connect should trigger successful session start event.
	mysql, err := testCtx.mysqlClient("alice", "mysql", "root")
	require.NoError(t, err)
	requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)

	// Simple query should trigger the query event.
	_, err = mysql.Execute("select 1")
	require.NoError(t, err)
	requireQueryEvent(t, testCtx, libevents.DatabaseSessionQueryCode, "select 1")

	// Closing connection should trigger session end event.
	err = mysql.Close()
	require.NoError(t, err)
	requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
}

// TestAuditMongo verifies proper audit events are emitted for MongoDB
// connections.
func TestAuditMongo(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{"admin"})

	// Access denied should trigger an unsuccessful session start event.
	_, err := testCtx.mongoClient(ctx, "alice", "mongo", "notadmin")
	require.Error(t, err)
	waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)

	// Connect should trigger successful session start event.
	mongoClient, err := testCtx.mongoClient(ctx, "alice", "mongo", "admin")
	require.NoError(t, err)
	waitForEvent(t, testCtx, libevents.DatabaseSessionStartCode)

	// Find command in a database we don't have access to.
	_, err = mongoClient.Database("notadmin").Collection("test").Find(ctx, bson.M{})
	require.Error(t, err)
	waitForEvent(t, testCtx, libevents.DatabaseSessionQueryFailedCode)

	// Find command should trigger the query event.
	_, err = mongoClient.Database("admin").Collection("test").Find(ctx, bson.M{})
	require.NoError(t, err)
	waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)

	// Closing connection should trigger session end event.
	err = mongoClient.Disconnect(ctx)
	require.NoError(t, err)
	waitForEvent(t, testCtx, libevents.DatabaseSessionEndCode)
}

func TestAuditRedis(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		// Access denied should trigger an unsuccessful session start event.
		_, err := testCtx.redisClient(ctx, "alice", "redis", "notadmin")
		require.Error(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
	})

	var redisClient *redis.Client

	t.Run("session starts event", func(t *testing.T) {
		// Connect should trigger successful session start event.
		var err error
		redisClient, err = testCtx.redisClient(ctx, "alice", "redis", "admin")
		require.NoError(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartCode)
	})

	t.Run("command sends", func(t *testing.T) {
		// SET should trigger Query event.
		err := redisClient.Set(ctx, "foo", "bar", 0).Err()
		require.NoError(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
	})

	t.Run("session ends event", func(t *testing.T) {
		// Closing connection should trigger session end event.
		err := redisClient.Close()
		require.NoError(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

// TestAuditSQLServer verifies proper audit events are emitted for SQLServer
// connections.
func TestAuditSQLServer(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSQLServer("sqlserver"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "admin", "admin", []string{"admin"}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		_, _, err := testCtx.sqlServerClient(ctx, "admin", "sqlserver", "invalid", "se")
		require.Error(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
	})

	t.Run("successful flow", func(t *testing.T) {
		conn, proxy, err := testCtx.sqlServerClient(ctx, "admin", "sqlserver", "admin", "se")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, proxy.Close())
		})

		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)

		err = conn.Ping(context.Background())
		require.NoError(t, err)
		requireEvent(t, testCtx, libevents.DatabaseSessionQueryCode)

		require.NoError(t, conn.Close())
		requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

// TestAuditClickHouseHTTP verifies proper audit events are emitted for Clickhouse HTTP connections.
func TestAuditClickHouseHTTP(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withClickhouseHTTP(defaults.ProtocolClickHouseHTTP))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "admin", "admin", []string{"admin"}, []string{types.Wildcard})

	_, _, err := testCtx.clickHouseHTTPClient(ctx, "admin", defaults.ProtocolClickHouseHTTP, "invalid", "")
	require.Error(t, err)
	waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)

	t.Run("successful flow", func(t *testing.T) {
		conn, proxy, err := testCtx.clickHouseHTTPClient(ctx, "admin", defaults.ProtocolClickHouseHTTP, "admin", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, proxy.Close())
		})

		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		// Select timezone.
		event := waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
		assertDatabaseQueryFromAuditEvent(t, event, "SELECT timezone()")

		event = waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
		assertDatabaseQueryFromAuditEvent(t, event, "SELECT 1")

		require.NoError(t, conn.Close())
		requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})

	t.Run("successful flow native http client", func(t *testing.T) {
		proxy, _, err := testCtx.startLocalProxy(ctx, "admin", defaults.ProtocolClickHouseHTTP, "admin", "")
		require.NoError(t, err)
		defer proxy.Close()

		r := bytes.NewBufferString("SELECT 1")
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s", proxy.GetAddr()), r)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		event := waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
		assertDatabaseQueryFromAuditEvent(t, event, "SELECT 1")
		requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)

		req, err = http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s?query=SELECT", proxy.GetAddr()), bytes.NewBufferString("1"))
		require.NoError(t, err)
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())

		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		event = waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
		assertDatabaseQueryFromAuditEvent(t, event, "SELECT 1")
	})
}

func assertDatabaseQueryFromAuditEvent(t *testing.T, event events.AuditEvent, wantQuery string) {
	query, ok := event.(*events.DatabaseSessionQuery)
	require.True(t, ok)
	require.Equal(t, wantQuery, query.DatabaseQuery)
}

func requireEvent(t *testing.T, testCtx *testContext, code string) {
	event := waitForAnyEvent(t, testCtx)
	require.Equal(t, code, event.GetCode())
}

func requireQueryEvent(t *testing.T, testCtx *testContext, code, query string) {
	event := waitForAnyEvent(t, testCtx)
	require.Equal(t, code, event.GetCode())
	require.Equal(t, query, event.(*events.DatabaseSessionQuery).DatabaseQuery)
}

func waitForAnyEvent(t *testing.T, testCtx *testContext) events.AuditEvent {
	select {
	case event := <-testCtx.emitter.C():
		return event
	case <-time.After(time.Second):
		t.Fatalf("didn't receive any event after 1 second")
	}
	return nil
}

// waitForEvent waits for particular event code ignoring other events.
func waitForEvent(t *testing.T, testCtx *testContext, code string) events.AuditEvent {
	for {
		select {
		case event := <-testCtx.emitter.C():
			if event.GetCode() != code {
				// ignored events may be helpful in debugging test failures
				bytes, err := json.Marshal(event)
				if err != nil {
					bytes = []byte(err.Error())
				}
				t.Logf("ignoring mismatched event, wanted %v, got type=%v code=%v json=%v", code, event.GetType(), event.GetCode(), string(bytes))
				continue
			}
			return event
		case <-time.After(time.Second):
			t.Fatalf("didn't receive %v event after 1 second", code)
		}
	}
}
