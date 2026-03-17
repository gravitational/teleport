/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/bigquery"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func TestAccessBigQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{withBigQuery("BigQuery")(t, ctx, testCtx)},
	})
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbUsers []string
		dbUser       string
		wantErrMsg   string
	}{
		{
			desc:         "has access to all database users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "bq-reader",
		},
		{
			desc:         "access allowed to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"bq-reader"},
			dbUser:       "bq-reader",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "bq-reader",
			wantErrMsg:   "access to db denied",
		},
		{
			desc:         "access denied to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"bq-reader"},
			dbUser:       "bq-writer",
			wantErrMsg:   "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{})

			clt, lp, err := testCtx.bigqueryClient(ctx, test.user, "BigQuery", test.dbUser)
			t.Cleanup(func() {
				if lp != nil {
					lp.Close()
				}
			})
			require.NoError(t, err)

			resp, err := clt.Query("test-project", "SELECT 1")
			if test.wantErrMsg != "" {
				require.NoError(t, err) // HTTP request succeeds but response has error
				require.Equal(t, 403, resp.StatusCode, "expected 403 for access denied")
				return
			}
			require.NoError(t, err)
			require.Equal(t, 200, resp.StatusCode)
		})
	}
}

func TestAuditBigQuery(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{withBigQuery("BigQuery")(t, ctx, testCtx)},
	})
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"bq-admin"}, []string{types.Wildcard})

	clientCtx, cancel := context.WithCancel(ctx)

	t.Run("access denied", func(t *testing.T) {
		clt, lp, err := testCtx.bigqueryClient(clientCtx, "alice", "BigQuery", "notadmin")
		t.Cleanup(func() {
			if lp != nil {
				lp.Close()
			}
		})
		require.NoError(t, err)

		resp, err := clt.Query("test-project", "SELECT 1")
		require.NoError(t, err)
		require.Equal(t, 403, resp.StatusCode)
		requireEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
	})

	clt, lp, err := testCtx.bigqueryClient(clientCtx, "alice", "BigQuery", "bq-admin")
	t.Cleanup(func() {
		cancel()
		if lp != nil {
			lp.Close()
		}
	})
	require.NoError(t, err)

	t.Run("session starts and emits a query event", func(t *testing.T) {
		resp, err := clt.Query("test-project", "SELECT 1 as test")
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		requireEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
	})

	t.Run("session ends when local proxy closes the connection", func(t *testing.T) {
		resp, err := clt.Query("test-project", "SELECT 2 as another_test")
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		requireEvent(t, testCtx, libevents.DatabaseSessionQueryCode)
		cancel()
		lp.Close()
		requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

func withBigQuery(name string, opts ...bigquery.TestServerOption) withDatabaseOption {
	return func(t testing.TB, _ context.Context, testCtx *testContext) types.Database {
		config := common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
		}
		server, err := bigquery.NewTestServer(config, opts...)
		require.NoError(t, err)
		go server.Serve()
		t.Cleanup(func() { server.Close() })

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolBigQuery,
			URI:           "http://" + net.JoinHostPort("localhost", server.Port()),
			DynamicLabels: dynamicLabels,
			GCP: types.GCPCloudSQL{
				ProjectID: "test-project",
			},
		})
		require.NoError(t, err)

		testCtx.bigquery[name] = testBigQuery{
			db:       server,
			resource: database,
		}
		return database
	}
}
