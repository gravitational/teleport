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
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/stretchr/testify/require"
)

func TestProxyConnectionLimiting(t *testing.T) {
	const (
		user            = "bob"
		role            = "admin"
		postgresDbName  = "postgres"
		dbUser          = user
		connLimitNumber = 3 // Arbitrary number
	)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"))
	// TODO(jakule): Mongo seems to create some internal connections. I didn't find a way to predict
	// how many connection will be created and decided to skip it for now. Otherwise, the whole test may be flaky.

	connLimit, err := limiter.NewLimiter(limiter.Config{MaxConnections: connLimitNumber})
	require.NoError(t, err)

	// Set proxy connection limiter.
	testCtx.proxyServer.cfg.Limiter = connLimit

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
				for i := 0; i < connLimitNumber; i++ {
					// Try to connect to the database.
					dbConn, err := tt.connect()
					require.NoError(t, err)

					connsClosers = append(connsClosers, dbConn)
				}

				// This connection should go over the limit.
				_, err = tt.connect()
				require.Error(t, err)
				require.Contains(t, err.Error(), "exceeded connection limit")
			})

			// When a connection is released a new can be established
			t.Run("reconnect one", func(t *testing.T) {
				// Get one open connection.
				require.GreaterOrEqual(t, len(connsClosers), 1)
				oneConn := connsClosers[len(connsClosers)-1]
				connsClosers = connsClosers[:len(connsClosers)-1]

				// Close it, this should decrease the connection limit.
				err = oneConn(ctx)
				require.NoError(t, err)

				// Create a new connection. We do not expect an error here as we have just closed one.
				dbConn, err := tt.connect()
				require.NoError(t, err)
				connsClosers = append(connsClosers, dbConn)

				// Here the limit should be reached again.
				_, err = tt.connect()
				require.Error(t, err)
				require.Contains(t, err.Error(), "exceeded connection limit")
			})
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

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
		withSelfHostedMongo("mongodb"),
		withSelfHostedRedis("redis"),
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

	// Set proxy connection limiter.
	testCtx.proxyServer.cfg.Limiter = connLimit

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

func TestProxyMySQLVersion(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedMySQL("mysql", mysql.WithServerVersion("8.0.12")),
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
