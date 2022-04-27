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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/stretchr/testify/require"
)

// TestSessiontTracker verifies session tracker functionality for database sessions.
func TestSessionTracker(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		desc         string
		withDatabase withDatabaseOption
		openFunc     func(t *testing.T, testCtx *testContext) (closeFunc func(t *testing.T))
	}{
		{
			desc:         "postgres",
			withDatabase: withSelfHostedPostgres("postgres"),
			openFunc: func(t *testing.T, testCtx *testContext) (closeFunc func(t *testing.T)) {
				psql, err := testCtx.postgresClient(ctx, "alice", "postgres", "admin", "admin")
				require.NoError(t, err)
				return func(t *testing.T) {
					require.NoError(t, psql.Close(ctx))
				}
			},
		}, {
			desc:         "mysql",
			withDatabase: withSelfHostedMySQL("mysql"),
			openFunc: func(t *testing.T, testCtx *testContext) (closeFunc func(t *testing.T)) {
				mysql, err := testCtx.mysqlClient("alice", "mysql", "admin")
				require.NoError(t, err)
				return func(t *testing.T) {
					require.NoError(t, mysql.Close())
				}
			},
		}, {
			desc:         "mongo",
			withDatabase: withSelfHostedMongo("mongo"),
			openFunc: func(t *testing.T, testCtx *testContext) (closeFunc func(t *testing.T)) {
				mongoClient, err := testCtx.mongoClient(ctx, "alice", "mongo", "admin")
				require.NoError(t, err)
				return func(t *testing.T) {
					require.NoError(t, mongoClient.Disconnect(ctx))
				}
			},
		}, {
			desc:         "redis",
			withDatabase: withSelfHostedRedis("redis"),
			openFunc: func(t *testing.T, testCtx *testContext) (closeFunc func(t *testing.T)) {
				redisClient, err := testCtx.redisClient(ctx, "alice", "redis", "admin")
				require.NoError(t, err)
				return func(t *testing.T) {
					require.NoError(t, redisClient.Close())
				}
			},
		}, {
			desc:         "sqlserver",
			withDatabase: withSQLServer("sqlserver"),
			openFunc: func(t *testing.T, testCtx *testContext) (closeFunc func(t *testing.T)) {
				conn, proxy, err := testCtx.sqlServerClient(ctx, "alice", "sqlserver", "admin", "master")
				require.NoError(t, err)
				return func(t *testing.T) {
					require.NoError(t, conn.Close())
					require.NoError(t, proxy.Close())
				}
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			testCtx := setupTestContext(ctx, t, tc.withDatabase)
			go testCtx.startHandlingConnections()
			testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{"admin"})

			// Session tracker should be created for new connection
			closeFunc := tc.openFunc(t, testCtx)

			var tracker types.SessionTracker
			trackerCreated := func() bool {
				trackers, err := testCtx.authClient.GetActiveSessionTrackers(ctx)
				require.NoError(t, err)
				// Note: mongo test creates 3 sessions (unrelated bug?), we just test the first one.
				if len(trackers) > 0 {
					tracker = trackers[0]
					return true
				}
				return false
			}
			require.Eventually(t, trackerCreated, time.Second*15, time.Second)
			require.Equal(t, types.SessionState_SessionStateRunning, tracker.GetState())

			// The session tracker expiration should be extended while the session is active
			testCtx.clock.Advance(defaults.SessionTrackerExpirationUpdateInterval)
			trackerUpdated := func() bool {
				updatedTracker, err := testCtx.authClient.GetSessionTracker(ctx, tracker.GetSessionID())
				require.NoError(t, err)
				return updatedTracker.Expiry().Equal(tracker.Expiry().Add(defaults.SessionTrackerExpirationUpdateInterval))
			}
			require.Eventually(t, trackerUpdated, time.Second*15, time.Second)

			// Closing connection should trigger session tracker state to be terminated.
			closeFunc(t)

			trackerTerminated := func() bool {
				tracker, err := testCtx.authClient.GetSessionTracker(ctx, tracker.GetSessionID())
				require.NoError(t, err)
				return tracker.GetState() == types.SessionState_SessionStateTerminated
			}
			require.Eventually(t, trackerTerminated, time.Second*15, time.Second)
		})
	}
}
