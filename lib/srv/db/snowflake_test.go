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
	"database/sql"
	"net"
	"net/http"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/snowflake"
	"github.com/stretchr/testify/require"
)

func init() {
	// Override Snowflake engine that is used normally with the test one
	// with custom HTTP client.
	common.RegisterEngine(newTestSnowflakeEngine, defaults.ProtocolSnowflake)
}

func newTestSnowflakeEngine(ec common.EngineConfig) common.Engine {
	return &snowflake.Engine{
		EngineConfig: ec,
		HttpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

func TestAccessSnowflake(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSnowflake("snowflake"))
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
			dbName:       "snowflake",
			dbUser:       "snowflake",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "snowflake",
			dbUser:       "snowflake",
			err:          "HTTP: 401",
		},
		{
			desc:         "no access to databases",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "snowflake",
			dbUser:       "snowflake",
			err:          "HTTP: 401",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{},
			dbName:       "snowflake",
			dbUser:       "snowflake",
			err:          "HTTP: 401",
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "metrics",
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "snowflake",
			dbUser:       "snowflake",
			err:          "HTTP: 401",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, test.allowDbNames)

			// Try to connect to the database as this user.
			dbConn, proxy, err := testCtx.snowflakeClient(ctx, test.user, "snowflake", test.dbUser, test.dbName)

			t.Cleanup(func() {
				proxy.Close()
			})

			require.NoError(t, err)

			// Execute a query.

			result, err := dbConn.QueryContext(ctx, "select 42")
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}
			require.NoError(t, err)
			defer result.Close()

			for result.Next() {
				var res int
				err = result.Scan(&res)
				require.NoError(t, err)
				require.Equal(t, 42, res)
			}

			// Disconnect.
			err = dbConn.Close()
			require.NoError(t, err)
		})
	}
}

func TestAuditSnowflake(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSnowflake("snowflake"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		// Access denied should trigger an unsuccessful session start event.
		dbConn, proxy, err := testCtx.snowflakeClient(ctx, "alice", "snowflake", "notadmin", "")
		require.NoError(t, err)
		err = dbConn.PingContext(ctx)
		require.Error(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
		proxy.Close()
	})

	var dbConn *sql.DB
	var proxy *alpnproxy.LocalProxy
	t.Cleanup(func() {
		if proxy != nil {
			proxy.Close()
		}
	})

	t.Run("session starts event", func(t *testing.T) {
		// Connect should trigger successful session start event.
		var err error

		dbConn, proxy, err = testCtx.snowflakeClient(ctx, "alice", "snowflake", "admin", "")
		require.NoError(t, err)
		err = dbConn.PingContext(ctx)
		require.NoError(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartCode)
	})

	t.Run("command sends", func(t *testing.T) {
		// SET should trigger Query event.
		result, err := dbConn.QueryContext(ctx, "select 42")
		require.NoError(t, err)
		defer result.Close()

		for result.Next() {
			var res int
			err = result.Scan(&res)
			require.NoError(t, err)
			require.Equal(t, 42, res)
		}

		waitForEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
	})

	t.Run("session ends event", func(t *testing.T) {
		t.Skip() //TODO(jakule): Driver for some reason doesn't terminate the session.
		// Closing connection should trigger session end event.
		err := dbConn.Close()
		require.NoError(t, err)
		waitForEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

func withSnowflake(name string) withDatabaseOption {
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
		postgresServer, err := snowflake.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
		})
		require.NoError(t, err)
		go postgresServer.Serve()
		t.Cleanup(func() { postgresServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolSnowflake,
			URI:           net.JoinHostPort("localhost", postgresServer.Port()),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.snowflake[name] = testSnowflake{
			db:       postgresServer,
			resource: database,
		}
		return database
	}
}
