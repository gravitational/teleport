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
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/srv/ingress"
)

type fakeIngressReporter struct {
	connected            atomic.Int32
	authenticatedService atomic.Pointer[string]
}

func (f *fakeIngressReporter) ConnectionAccepted(string, net.Conn) {
	f.connected.Add(1)
}
func (f *fakeIngressReporter) ConnectionClosed(string, net.Conn) {
	f.connected.Add(-1)
}
func (f *fakeIngressReporter) ConnectionAuthenticated(service string, _ net.Conn) {
	f.authenticatedService.Store(&service)
}
func (f *fakeIngressReporter) AuthenticatedConnectionClosed(string, net.Conn) {
}

func (f *fakeIngressReporter) getConnected() int {
	return int(f.connected.Load())
}
func (f *fakeIngressReporter) getLastAuthenticatedService() string {
	loaded := f.authenticatedService.Load()
	if loaded == nil {
		return ""
	}
	return *loaded
}
func (f *fakeIngressReporter) waitForNumConnected(t *testing.T, expect int) {
	t.Helper()
	require.EventuallyWithT(
		t,
		func(collect *assert.CollectT) {
			assert.Equal(collect, expect, f.getConnected())
		},
		time.Second*2,
		time.Millisecond*100,
		"timed out waiting for %d connections",
		expect,
	)
}

func TestProxyConnectionLimitingAndIngressReporting(t *testing.T) {
	const (
		postgresDbName      = "postgres"
		dbUser              = "teleport-admin"
		connLimitByConnIP   = 4 // Arbitrary number
		connLimitByIdentity = 3 // Arbitrary but is smaller than IP limit to avoid hitting that first
	)

	connLimit, err := limiter.NewLimiter(limiter.Config{MaxConnections: connLimitByConnIP})
	require.NoError(t, err)

	reporter := &fakeIngressReporter{}

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
		withSelfHostedRedis("redis"),
		withProxyLimiter(connLimit),
		withIngressReporter(reporter),
	)

	go testCtx.startHandlingConnections()

	createUserWithPerIdentityLimit := func(username string, maxConnections int64) {
		testCtx.createUserAndRole(ctx, t,
			username,                 // username
			username,                 // role name
			[]string{types.Wildcard}, // db users
			[]string{types.Wildcard}, // db names
			func(role types.Role) {
				options := role.GetOptions()
				options.MaxDatabaseConnections = maxConnections
				role.SetOptions(options)
			},
		)
	}

	createUserWithPerIdentityLimit("by-ip", 0 /* no limit */)
	createUserWithPerIdentityLimit("by-identity", connLimitByIdentity)

	testDatabaseTypes := []struct {
		name                       string
		connect                    func(user string) (func(context.Context) error, error)
		expectAuthenticatedService string
	}{
		{
			name: "postgres",
			connect: func(user string) (func(context.Context) error, error) {
				pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, postgresDbName)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return pgConn.Close, nil
			},
			expectAuthenticatedService: ingress.Postgres,
		},
		{
			name: "mysql",
			connect: func(user string) (func(context.Context) error, error) {
				mysqlClient, err := testCtx.mysqlClient(user, "mysql", dbUser)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return func(_ context.Context) error {
					return mysqlClient.Close()
				}, nil
			},
			expectAuthenticatedService: ingress.MySQL,
		},
		{
			name: "redis",
			connect: func(user string) (func(context.Context) error, error) {
				redisClient, err := testCtx.redisClient(ctx, user, "redis", dbUser)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return func(_ context.Context) error {
					return redisClient.Close()
				}, nil
			},
			expectAuthenticatedService: ingress.DatabaseTLS,
		},
	}

	testLimitTypes := []struct {
		username            string
		limit               int
		expectErrorContains string
	}{
		{
			username:            "by-ip",
			limit:               connLimitByConnIP,
			expectErrorContains: "exceeded connection limit",
		},
		{
			username:            "by-identity",
			limit:               connLimitByIdentity,
			expectErrorContains: "too many concurrent database connections",
		},
	}

	for _, limitType := range testLimitTypes {
		t.Run(limitType.username, func(t *testing.T) {
			for _, databaseType := range testDatabaseTypes {
				checkErrorMessage := func(t *testing.T, err error) {
					switch databaseType.name {
					// Connections are closed before interpreting the database
					// protocol so client only sees EOF.
					case "redis":
						require.Contains(t, err.Error(), "EOF")
					default:
						require.Contains(t, err.Error(), limitType.expectErrorContains)
					}
				}

				t.Run(databaseType.name, func(t *testing.T) {
					// Keep close functions to all connections. Call and release all active connection at the end of test.
					connsClosers := make([]func(context.Context) error, 0)
					t.Cleanup(func() {
						for _, connClose := range connsClosers {
							err := connClose(ctx)
							require.NoError(t, err)
						}
						reporter.waitForNumConnected(t, 0)
					})

					require.Zero(t, reporter.getConnected())

					t.Run("limit can be hit", func(t *testing.T) {
						for i := 0; i < limitType.limit; i++ {
							// Try to connect to the database.
							dbConn, err := databaseType.connect(limitType.username)
							require.NoError(t, err)

							connsClosers = append(connsClosers, dbConn)
						}

						// Check ingress counter.
						require.Equal(t, limitType.limit, reporter.getConnected())
						require.Equal(t, databaseType.expectAuthenticatedService, reporter.getLastAuthenticatedService())

						// This connection should go over the limit.
						_, err = databaseType.connect(limitType.username)
						require.Error(t, err)
						checkErrorMessage(t, err)
					})

					// When a connection is released a new can be established
					t.Run("reconnect one", func(t *testing.T) {
						// Get one open connection.
						require.NotEmpty(t, connsClosers)
						oneConn := connsClosers[len(connsClosers)-1]
						connsClosers = connsClosers[:len(connsClosers)-1]

						// Close it, this should decrease the connection limit.
						err = oneConn(ctx)
						require.NoError(t, err)

						// Wait until ingress counter changed.
						reporter.waitForNumConnected(t, limitType.limit-1)

						// Create a new connection. We do not expect an error here as we have just closed one.
						dbConn, err := databaseType.connect(limitType.username)
						require.NoError(t, err)
						connsClosers = append(connsClosers, dbConn)

						// Here the limit should be reached again.
						_, err = databaseType.connect(limitType.username)
						require.Error(t, err)
						checkErrorMessage(t, err)
					})
				})
			}
		})
	}
}

func TestProxyRateLimiting(t *testing.T) {
	const (
		user            = "bob"
		role            = "admin"
		postgresDbName  = "postgres"
		dbUser          = user
		connLimitNumber = 20 // Should be enough to hit the connection limit.
	)

	connLimit, err := limiter.NewLimiter(limiter.Config{
		// Set rates low, so we can easily hit them.
		Rates: []limiter.Rate{
			{
				Period:  10 * time.Second,
				Average: 3,
				Burst:   3,
			},
		}})
	require.NoError(t, err)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
		withSelfHostedMongo("mongodb"),
		withSelfHostedRedis("redis"),
		withProxyLimiter(connLimit),
	)

	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, user, role, []string{types.Wildcard}, []string{types.Wildcard})

	tests := []struct {
		name    string
		connect func() (func(context.Context) error, error)
	}{
		{
			"postgres",
			func() (func(context.Context) error, error) {
				pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, postgresDbName)
				return pgConn.Close, err
			},
		},
		{
			"mysql",
			func() (func(context.Context) error, error) {
				mysqlClient, err := testCtx.mysqlClient(user, "mysql", dbUser)
				return func(_ context.Context) error {
					return mysqlClient.Close()
				}, err
			},
		},
		{
			"mongodb",
			func() (func(context.Context) error, error) {
				mongoClient, err := testCtx.mongoClient(ctx, user, "mongodb", dbUser)
				return mongoClient.Disconnect, err
			},
		},
		{
			"redis",
			func() (func(context.Context) error, error) {
				redisClient, err := testCtx.redisClient(ctx, user, "redis", dbUser)
				return func(_ context.Context) error {
					return redisClient.Close()
				}, err
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			// Keep close functions to all connections. Call and release all active connection at the end of test.
			connsClosers := make([]func(context.Context) error, 0)
			t.Cleanup(func() {
				for _, connClose := range connsClosers {
					err := connClose(ctx)
					require.NoError(t, err)
				}
			})

			for i := 0; i < connLimitNumber; i++ {
				// Try to connect to the database.
				pgConn, err := tt.connect()
				if err == nil {
					connsClosers = append(connsClosers, pgConn)

					continue
				}

				require.Error(t, err)

				switch tt.name {
				case "mongodb", "redis":
					//TODO(jakule) currently TLS proxy (which is used by mongodb and redis) doesn't know
					// how to propagate errors, so this check is disabled.
				default:
					require.Contains(t, err.Error(), "rate limit exceeded")
				}

				return
			}

			require.FailNow(t, "we should hit the limit by now")
		})
	}
}

func TestProxyPerUserMaxConnections(t *testing.T) {
	const (
		user           = "bob"
		role           = "admin"
		postgresDbName = "postgres"
		dbUser         = user
		maxConnections = 3 // Arbitrary number
	)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
	)

	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t,
		user, role,
		[]string{types.Wildcard}, []string{types.Wildcard},
		func(role types.Role) {
			options := role.GetOptions()
			options.MaxDatabaseConnections = maxConnections
			role.SetOptions(options)
		},
	)
	tests := []struct {
		name    string
		connect func() (func(context.Context) error, error)
	}{
		{
			"postgres",
			func() (func(context.Context) error, error) {
				pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, postgresDbName)
				return pgConn.Close, err
			},
		},
		{
			"mysql",
			func() (func(context.Context) error, error) {
				mysqlClient, err := testCtx.mysqlClient(user, "mysql", dbUser)
				return func(_ context.Context) error {
					return mysqlClient.Close()
				}, err
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			// Keep close functions to all connections. Call and release all active connection at the end of test.
			connsClosers := make([]func(context.Context) error, 0)
			t.Cleanup(func() {
				for _, connClose := range connsClosers {
					err := connClose(ctx)
					require.NoError(t, err)
				}
			})

			t.Run("limit can be hit", func(t *testing.T) {
				for i := 0; i < maxConnections; i++ {
					// Try to connect to the database.
					dbConn, err := tt.connect()
					require.NoError(t, err)

					connsClosers = append(connsClosers, dbConn)
				}

				// This connection should go over the limit.
				_, err := tt.connect()
				require.Error(t, err)
				require.Contains(t, err.Error(), "too many concurrent database connections")
			})

			// When a connection is released a new can be established
			t.Run("reconnect one", func(t *testing.T) {
				// Get one open connection.
				require.NotEmpty(t, connsClosers)
				oneConn := connsClosers[len(connsClosers)-1]
				connsClosers = connsClosers[:len(connsClosers)-1]

				// Close it, this should decrease the connection limit.
				err := oneConn(ctx)
				require.NoError(t, err)

				// Create a new connection. We do not expect an error here as we have just closed one.
				dbConn, err := tt.connect()
				require.NoError(t, err)
				connsClosers = append(connsClosers, dbConn)

				// Here the limit should be reached again.
				_, err = tt.connect()
				require.Error(t, err)
				require.Contains(t, err.Error(), "too many concurrent database connections")
			})
		})
	}
}

func TestProxyMySQLVersion(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedMySQL("mysql", withMySQLServerVersion("8.0.12")),
	)

	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, "bob", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	t.Run("correct version when using proxy", func(t *testing.T) {
		mysqlClient, proxy, err := testCtx.mysqlClientLocalProxy(ctx, "bob", "mysql", "bob")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, mysqlClient.Close())
			require.NoError(t, proxy.Close())
		})
		require.Equal(t, "8.0.12", mysqlClient.GetServerVersion())
	})
}
