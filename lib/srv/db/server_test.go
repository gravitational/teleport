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
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
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
	retryutils.RetryStaticFor(5*time.Second, 20*time.Millisecond, func() error {
		servers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
		if err != nil {
			return err
		}
		if len(servers) != 4 {
			return fmt.Errorf("expected 4 servers, got %d", len(servers))
		}
		for _, server := range servers {
			if diff := cmp.Diff(map[string]string{"echo": "test"}, server.GetDatabase().GetAllLabels()); diff != "" {
				return fmt.Errorf("expected echo:test label, diff: %s", diff)
			}
		}
		return nil
	})
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
		dbConns := make([]mysql.TestClientConn, 0)
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

func TestDatabaseServerAutoDisconnect(t *testing.T) {
	const (
		user   = "bob"
		role   = "admin"
		dbName = "postgres"
		dbUser = user
	)

	ctx := context.Background()
	allowDbUsers := []string{types.Wildcard}
	allowDbNames := []string{types.Wildcard}

	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))

	go testCtx.startHandlingConnections()
	t.Cleanup(func() {
		require.NoError(t, testCtx.Close())
	})

	const clientIdleTimeout = time.Second * 30

	// create user/role with client idle timeout
	testCtx.createUserAndRole(ctx, t, user, role, allowDbUsers, allowDbNames, withClientIdleTimeout(clientIdleTimeout))

	// connect
	pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
	require.NoError(t, err)

	// immediate query should work
	_, err = pgConn.Exec(ctx, "select 1").ReadAll()
	require.NoError(t, err)

	// advance clock several times, perform query.
	// the activity should update the idle activity timer.
	for i := 0; i < 10; i++ {
		advanceInSteps(testCtx.clock, clientIdleTimeout/2)
		_, err = pgConn.Exec(ctx, "select 1").ReadAll()
		require.NoErrorf(t, err, "failed on iteration %v", i+1)
	}

	// advance clock by full idle timeout (plus a safety margin, to allow for reads in flight to be finished), expect the client to be disconnected automatically.
	advanceInSteps(testCtx.clock, clientIdleTimeout+time.Second*5)
	waitForEvent(t, testCtx, events.ClientDisconnectCode)

	// expect failure after timeout.
	_, err = pgConn.Exec(ctx, "select 1").ReadAll()
	require.Error(t, err)

	require.NoError(t, pgConn.Close(ctx))
}

// advanceInSteps makes the clockwork.FakeClock behave closer to the real clock, smoothing the transition for large advances in time.
// This works around a class of issues in the production code which expects that clock is smooth, not choppy.
// Most testing code should NOT need to use this function.
//
// In technical terms, it divides the clock advancement into 100 smaller steps, with a short sleep after each one.
func advanceInSteps(clock *clockwork.FakeClock, total time.Duration) {
	step := total / 100
	if step <= 0 {
		step = 1
	}

	end := clock.Now().Add(total)
	for clock.Now().Before(end) {
		clock.Advance(step)
		time.Sleep(time.Millisecond * 1)
	}
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

func TestDatabaseServiceHeartbeatEvents(t *testing.T) {
	t.Run("no label when running on-prem", func(t *testing.T) {
		ctx := context.Background()
		var heartbeatEvents int64
		heartbeatRecorder := func(err error) {
			require.NoError(t, err)
			atomic.AddInt64(&heartbeatEvents, 1)
		}

		testCtx := setupTestContext(ctx, t)
		server := testCtx.setupDatabaseServer(ctx, t, agentParams{
			NoStart:     true,
			OnHeartbeat: heartbeatRecorder,
		})
		require.NoError(t, server.Start(ctx))
		t.Cleanup(func() {
			server.Close()
		})

		var listResp *types.ListResourcesResponse
		var err error
		require.Eventually(t, func() bool {
			listResp, err = testCtx.authServer.ListResources(ctx, proto.ListResourcesRequest{ResourceType: types.KindDatabaseService})
			require.NoError(t, err)

			return atomic.LoadInt64(&heartbeatEvents) == 1 && len(listResp.Resources) == 1
		}, 2*time.Second, 500*time.Millisecond)

		require.NotContains(t, listResp.Resources[0].GetAllLabels(), "teleport.dev/awsoidc-agent")

		dbServices, err := types.ResourcesWithLabels(listResp.Resources).AsDatabaseServices()
		require.NoError(t, err)
		require.Equal(t, "teleport.cluster.local", dbServices[0].GetHostname())
	})
	t.Run("when running as AWS OIDC ECS/Fargate Agent, it has a label indicating that", func(t *testing.T) {
		ctx := context.Background()
		var heartbeatEvents int64
		heartbeatRecorder := func(err error) {
			require.NoError(t, err)
			atomic.AddInt64(&heartbeatEvents, 1)
		}

		t.Setenv(types.InstallMethodAWSOIDCDeployServiceEnvVar, "yes")
		testCtx := setupTestContext(ctx, t)
		server := testCtx.setupDatabaseServer(ctx, t, agentParams{
			NoStart:     true,
			OnHeartbeat: heartbeatRecorder,
		})
		require.NoError(t, server.Start(ctx))
		t.Cleanup(func() {
			server.Close()
		})

		var listResp *types.ListResourcesResponse
		var err error
		require.Eventually(t, func() bool {
			listResp, err = testCtx.authServer.ListResources(ctx, proto.ListResourcesRequest{ResourceType: types.KindDatabaseService})
			require.NoError(t, err)

			return atomic.LoadInt64(&heartbeatEvents) == 1 && len(listResp.Resources) == 1
		}, 2*time.Second, 500*time.Millisecond)

		require.Contains(t, listResp.Resources[0].GetAllLabels(), "teleport.dev/awsoidc-agent")

		dbServices, err := types.ResourcesWithLabels(listResp.Resources).AsDatabaseServices()
		require.NoError(t, err)
		require.Equal(t, "teleport.cluster.local", dbServices[0].GetHostname())
	})
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
			require.Equal(t, types.Databases{db0}, server.getProxiedDatabases())

			// Validate that the heartbeat is eventually emitted and that
			// the configured databases exist in the inventory.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				dbServers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
				if !assert.NoError(t, err) {
					return
				}
				if !assert.Len(t, dbServers, 1) {
					return
				}
				if !assert.Empty(t, cmp.Diff(dbServers[0].GetDatabase(), db0, cmpopts.IgnoreFields(types.Metadata{}, "Revision", "Expires"))) {
					return
				}
			}, 10*time.Second, 100*time.Millisecond)

			require.NoError(t, server.Shutdown(ctx))

			// Send a Goodbye to simulate process shutdown.
			if !test.hasForkedChild {
				require.NoError(t, server.cfg.InventoryHandle.SendGoodbye(ctx))
			}

			require.NoError(t, server.cfg.InventoryHandle.Close())

			// Validate db servers based on the test.
			if test.wantDatabaseServersAfterShutdown {
				dbServersAfterShutdown, err := server.cfg.AuthClient.GetDatabaseServers(ctx, apidefaults.Namespace)
				require.NoError(t, err)
				require.Len(t, dbServersAfterShutdown, 1)
				require.Empty(t, cmp.Diff(dbServersAfterShutdown[0].GetDatabase(), db0, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			} else {
				require.EventuallyWithT(t, func(t *assert.CollectT) {
					dbServersAfterShutdown, err := server.cfg.AuthClient.GetDatabaseServers(ctx, apidefaults.Namespace)
					if !assert.NoError(t, err) {
						return
					}
					if !assert.Empty(t, dbServersAfterShutdown) {
						return
					}
				}, 10*time.Second, 100*time.Millisecond)
			}
		})
	}
}

func TestTrackActiveConnections(t *testing.T) {
	t.Parallel()

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
		}, 2*time.Second, 100*time.Millisecond, "expected %d active connections, but got %d", expectedActiveConnections, testCtx.server.activeConnections.Load())
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
		}, 2*time.Second, 100*time.Millisecond, "expected %d active connections, but got %d", expectedActiveConnections, testCtx.server.activeConnections.Load())
	}
}

// TestShutdownWithActiveConnections given that a running database server with
// one active connection constantly sending queries, when the server is
// gracefully shut down, it should keep running until the client closes the
// connection.
func TestShutdownWithActiveConnections(t *testing.T) {
	t.Parallel()

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
	require.NoError(t, <-connErrCh, "unexpected connection close error")
	require.NoError(t, <-shutdownErrCh, "unexpected server shutdown error")
	require.NoError(t, server.Wait(), "unexpected server Wait error")
}

// TestShutdownWithActiveConnections given that a running database server with
// one active connection constantly sending queries, when the server is
// killed/stopped, it should drop connections and cleanup resources.
func TestCloseWithActiveConnections(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, connErrCh, _ := databaseServerWithActiveConnection(t, ctx)

	require.NoError(t, server.Close())
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		select {
		case err := <-connErrCh:
			assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
		default:
		}
	}, time.Second, 100*time.Millisecond)
	require.NoError(t, server.Wait(), "unexpected server Wait error")
}

// serverWithActiveConnection starts a server with one active connection that
// will actively make queries.
func databaseServerWithActiveConnection(t *testing.T, ctx context.Context) (*Server, chan error, context.CancelFunc) {
	t.Helper()

	databaseName := "postgres"
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres(databaseName))
	go testCtx.startHandlingConnections()
	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"*"}, []string{"*"})

	connCtx, cancelConn := context.WithCancel(ctx)
	t.Cleanup(cancelConn)
	connErrCh := make(chan error)

	conn, err := testCtx.postgresClient(ctx, "alice", "postgres", "postgres", "postgres")
	require.NoError(t, err, "failed to establish connection")

	// Immediately sends a query. This ensures the connection is healthy and
	// active.
	_, err = conn.Exec(ctx, "select 1").ReadAll()
	if err != nil {
		conn.Close(ctx)
		require.Fail(t, "failed to send initial query")
	}

	require.Equal(t, int32(1), testCtx.server.activeConnections.Load(), "expected one active connection, but got none")

	go func() {
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

	return testCtx.server, connErrCh, cancelConn
}
