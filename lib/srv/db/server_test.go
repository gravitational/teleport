/*
Copyright 2020 Gravitational, Inc.

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
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
)

// TestDatabaseServerStart validates that started database server updates its
// dynamic labels and heartbeats its presence to the auth server.
func TestDatabaseServerStart(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
		withSelfHostedMongo("mongo"),
		withSelfHostedRedis("redis"))

	tests := []struct {
		database types.Database
	}{
		{database: testCtx.postgres["postgres"].resource},
		{database: testCtx.mysql["mysql"].resource},
		{database: testCtx.mongo["mongo"].resource},
		{database: testCtx.redis["redis"].resource},
	}

	for _, test := range tests {
		labels := testCtx.server.getDynamicLabels(test.database.GetName())
		require.NotNil(t, labels)
		require.Equal(t, "test", labels.Get()["echo"].GetResult())
	}

	// Make sure servers were announced and their labels updated.
	servers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 4)
	for _, server := range servers {
		require.Equal(t, map[string]string{"echo": "test"},
			server.GetDatabase().GetAllLabels())
	}
}

func TestDatabaseServerLimiting(t *testing.T) {
	const (
		user      = "bob"
		role      = "admin"
		dbName    = "postgres"
		dbUser    = user
		connLimit = int64(5) // Arbitrary number
	)

	ctx := context.Background()
	allowDbUsers := []string{types.Wildcard}
	allowDbNames := []string{types.Wildcard}

	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
		withSelfHostedMongo("mongo"),
	)

	connLimiter, err := limiter.NewLimiter(limiter.Config{MaxConnections: connLimit})
	require.NoError(t, err)

	// Set connection limit
	testCtx.server.cfg.Limiter = connLimiter

	go testCtx.startHandlingConnections()
	t.Cleanup(func() {
		err := testCtx.Close()
		require.NoError(t, err)
	})

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, user, role, allowDbUsers, allowDbNames)

	t.Run("postgres", func(t *testing.T) {
		dbConns := make([]*pgconn.PgConn, 0)
		t.Cleanup(func() {
			// Disconnect all clients.
			for _, pgConn := range dbConns {
				err = pgConn.Close(ctx)
				require.NoError(t, err)
			}
		})

		// Connect the maximum allowed number of clients.
		for i := int64(0); i < connLimit; i++ {
			pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
			require.NoError(t, err)

			// Save all connection, so we can close them later.
			dbConns = append(dbConns, pgConn)
		}

		// We keep the previous connections open, so this one should be rejected, because we exhausted the limit.
		_, err = testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeded connection limit")
	})

	t.Run("mysql", func(t *testing.T) {
		dbConns := make([]*client.Conn, 0)
		t.Cleanup(func() {
			// Disconnect all clients.
			for _, dbConn := range dbConns {
				err = dbConn.Close()
				require.NoError(t, err)
			}
		})
		// Connect the maximum allowed number of clients.
		for i := int64(0); i < connLimit; i++ {
			mysqlConn, err := testCtx.mysqlClient(user, "mysql", dbUser)
			require.NoError(t, err)

			// Save all connection, so we can close them later.
			dbConns = append(dbConns, mysqlConn)
		}

		// We keep the previous connections open, so this one should be rejected, because we exhausted the limit.
		_, err = testCtx.mysqlClient(user, "mysql", dbUser)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeded connection limit")
	})

	t.Run("mongodb", func(t *testing.T) {
		dbConns := make([]*mongo.Client, 0)
		t.Cleanup(func() {
			// Disconnect all clients.
			for _, dbConn := range dbConns {
				err = dbConn.Disconnect(ctx)
				require.NoError(t, err)
			}
		})
		// Mongo driver behave different from MySQL and Postgres. In this case we just want to hit the limit
		// by creating some DB connections.
		for i := int64(0); i < 2*connLimit; i++ {
			mongoConn, err := testCtx.mongoClient(ctx, user, "mongo", dbUser)

			if err == nil {
				// Save all connection, so we can close them later.
				dbConns = append(dbConns, mongoConn)

				continue
			}

			require.Contains(t, err.Error(), "exceeded connection limit")
			// When we hit the expected error we can exit.
			return
		}

		require.FailNow(t, "we should exceed the connection limit by now")
	})
}

func TestHeartbeatEvents(t *testing.T) {
	ctx := context.Background()

	dbOne, err := types.NewDatabaseV3(types.Metadata{
		Name: "dbOne",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	dbTwo, err := types.NewDatabaseV3(types.Metadata{
		Name: "dbOne",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// The expected heartbeat count is equal to the sum of:
	// - the number of static Databases
	// - plus 1 because the DatabaseService heartbeats itself to the cluster
	expectedHeartbeatCount := func(dbs types.Databases) int64 {
		return int64(dbs.Len() + 1)
	}

	tests := map[string]struct {
		staticDatabases types.Databases
	}{
		"SingleStaticDatabase": {
			staticDatabases: types.Databases{dbOne},
		},
		"MultipleStaticDatabases": {
			staticDatabases: types.Databases{dbOne, dbTwo},
		},
		"EmptyStaticDatabases": {
			staticDatabases: types.Databases{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var heartbeatEvents int64
			heartbeatRecorder := func(err error) {
				require.NoError(t, err)
				atomic.AddInt64(&heartbeatEvents, 1)
			}

			testCtx := setupTestContext(ctx, t)
			server := testCtx.setupDatabaseServer(ctx, t, agentParams{
				NoStart:     true,
				OnHeartbeat: heartbeatRecorder,
				Databases:   test.staticDatabases,
			})
			require.NoError(t, server.Start(ctx))
			t.Cleanup(func() {
				server.Close()
			})

			require.NotNil(t, server)
			require.Eventually(t, func() bool {
				return atomic.LoadInt64(&heartbeatEvents) == expectedHeartbeatCount(test.staticDatabases)
			}, 2*time.Second, 500*time.Millisecond)
		})
	}
}

func TestShutdown(t *testing.T) {
	tests := []struct {
		name                             string
		hasForkedChild                   bool
		wantDatabaseServersAfterShutdown bool
	}{
		{
			name:                             "regular shutdown",
			hasForkedChild:                   false,
			wantDatabaseServersAfterShutdown: false,
		},
		{
			name:                             "has forked child",
			hasForkedChild:                   true,
			wantDatabaseServersAfterShutdown: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			testCtx := setupTestContext(ctx, t)

			db0, err := makeStaticDatabase("db0", nil)
			require.NoError(t, err)

			server := testCtx.setupDatabaseServer(ctx, t, agentParams{
				Databases: []types.Database{db0},
			})

			// Validate that the server is proxying db0 after start.
			require.Equal(t, server.getProxiedDatabases(), types.Databases{db0})

			// Validate heartbeat is present after start.
			server.ForceHeartbeat()
			dbServers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			require.Len(t, dbServers, 1)
			require.Equal(t, dbServers[0].GetDatabase(), db0)

			// Shutdown should not return error.
			shutdownCtx, cancel := context.WithTimeout(ctx, time.Second*5)
			t.Cleanup(cancel)
			if test.hasForkedChild {
				shutdownCtx = services.ProcessForkedContext(shutdownCtx)
			}

			require.NoError(t, server.Shutdown(shutdownCtx))

			// Validate that the server is not proxying db0 after close.
			require.Empty(t, server.getProxiedDatabases())

			// Validate database servers based on the test.
			dbServersAfterShutdown, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			if test.wantDatabaseServersAfterShutdown {
				require.Equal(t, dbServers, dbServersAfterShutdown)
			} else {
				require.Empty(t, dbServersAfterShutdown)
			}
		})
	}
}

func TestTrackActiveConnections(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"*"}, []string{"*"})

	numActiveConnections := 5
	closeFuncs := []func() error{}

	// Create a few connections, increasing the active connections. Keep track
	// of the closer functions, so we can close them later.
	for i := 0; i < numActiveConnections; i++ {
		expectedActiveConnections := int32(i + 1)
		conn, err := testCtx.postgresClient(ctx, "alice", "postgres", "postgres", "postgres")
		require.NoError(t, err)
		closeFuncs = append(closeFuncs, func() error { return conn.Close(ctx) })
		// We're also adding them to the test cleanup to ensure connections are
		// closed even when the test fails.
		t.Cleanup(func() { conn.Close(ctx) })

		require.Eventually(t, func() bool {
			return expectedActiveConnections == testCtx.server.activeConnections.Load()
		}, time.Second, 100*time.Millisecond, "expected %d active connections, but got %d", expectedActiveConnections, testCtx.server.activeConnections.Load())
	}

	// For each connection we close, the active connections should drop too.
	for i := 0; i < numActiveConnections; i++ {
		expectedActiveConnections := int32(numActiveConnections - (i + 1))
		require.NoError(t, closeFuncs[i]())

		// Decreasing the active connection counter is one of the last things to
		// happen when closing a connection. We might need to give it sometime
		// to properly close the connection.
		require.Eventually(t, func() bool {
			return expectedActiveConnections == testCtx.server.activeConnections.Load()
		}, time.Second, 100*time.Millisecond, "expected %d active connections, but got %d", expectedActiveConnections, testCtx.server.activeConnections.Load())
	}
}

// TestShutdownWithActiveConnections given that a running database server with
// one active connection constantly sending queries, when the server is
// gracefully shut down, it should keep running until the client closes the
// connection.
func TestShutdownWithActiveConnections(t *testing.T) {
	ctx := context.Background()
	server, connErrCh, cancelConn := databaseServerWithActiveConnection(t, ctx)

	shutdownCtx, cancelShutdown := context.WithTimeout(ctx, time.Second*5)
	defer cancelShutdown()
	shutdownErrCh := make(chan error)
	go func() {
		shutdownErrCh <- server.Shutdown(shutdownCtx)
	}()

	// Shutdown doesn't return immediately. We must wait for a short period to
	// ensure it hasn't been completed.
	require.Never(t, func() bool {
		select {
		case <-shutdownErrCh:
			return true
		default:
			return false
		}
	}, time.Second, 100*time.Millisecond, "unexpected server shutdown")

	select {
	case err := <-connErrCh:
		require.Fail(t, "unexpected connection close", err)
	default:
	}

	cancelConn()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(c, <-connErrCh, "unexpected connection error")
	}, time.Second, 100*time.Millisecond)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(c, <-shutdownErrCh, "unexpected server shutdown error")
	}, time.Second, 100*time.Millisecond)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(c, server.Wait(), "unexpected server Wait error")
	}, time.Second, 100*time.Millisecond)
}

// TestShutdownWithActiveConnections given that a running database server with
// one active connection constantly sending queries, when the server is
// killed/stopped, it should drop connections and cleanup resources.
func TestCloseWithActiveConnections(t *testing.T) {
	ctx := context.Background()
	server, connErrCh, _ := databaseServerWithActiveConnection(t, ctx)

	require.NoError(t, server.Close())
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.ErrorIs(t, <-connErrCh, io.ErrUnexpectedEOF)
	}, time.Second, 100*time.Millisecond)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoError(t, server.Wait(), "unexpected error from server Wait")
	}, time.Second, 100*time.Millisecond)
}

// serverWithActiveConnection starts a server with one active connection that
// will actively make queries.
func databaseServerWithActiveConnection(t *testing.T, ctx context.Context) (*Server, chan error, context.CancelFunc) {
	t.Helper()

	databaseName := "postgres"
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres(databaseName))
	go testCtx.startHandlingConnections()
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"*"}, []string{"*"})
	dbTestServer, ok := testCtx.postgres[databaseName]
	require.True(t, ok)

	connCtx, cancelConn := context.WithCancel(ctx)
	t.Cleanup(func() { cancelConn() })
	connErrCh := make(chan error)

	go func() {
		conn, err := testCtx.postgresClient(ctx, "alice", "postgres", "postgres", "postgres")
		if err != nil {
			connErrCh <- err
			return
		}

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		defer conn.Close(ctx)

		for {
			select {
			case <-ticker.C:
				_, err := conn.Exec(ctx, "select 1").ReadAll()
				if err != nil {
					connErrCh <- err
					return
				}
			case <-connCtx.Done():
				connErrCh <- nil
				return
			}
		}
	}()

	require.Eventually(t, func() bool {
		return testCtx.server.activeConnections.Load() == int32(1)
	}, time.Second, 100*time.Millisecond, "expected one active connection, but got none")

	// Ensures the first query has been received.
	require.Eventually(t, func() bool {
		return dbTestServer.db.QueryCount() > uint32(1)
	}, time.Second, 100*time.Millisecond, "database test server hasn't received queries")

	return testCtx.server, connErrCh, cancelConn
}
