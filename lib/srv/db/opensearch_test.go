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
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/opensearch"
)

func TestAccessOpenSearch(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases:         []types.Database{withOpenSearch("OpenSearch")(t, ctx, testCtx)},
		AWSConfigProvider: &mocks.AWSConfigProvider{},
	})
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbUsers []string
		dbUser       string
		payload      string
		err          bool
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			payload:      `{"count":31874,"_shards":{"total":6,"successful":6,"skipped":0,"failed":0}}`,
			dbUser:       "opensearch",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "opensearch",
			payload:      `{"error":{"reason":"access to db denied. User does not have permissions. Confirm database user and name.","type":"access_denied_exception"},"status":401}`,
			err:          true,
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "opensearch",
			payload:      `{"error":{"reason":"access to db denied. User does not have permissions. Confirm database user and name.","type":"access_denied_exception"},"status":401}`,
			err:          true,
		},
		{
			desc:         "access allowed to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			payload:      `{"count":31874,"_shards":{"total":6,"successful":6,"skipped":0,"failed":0}}`,
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "baduser",
			payload:      `{"error":{"reason":"access to db denied. User does not have permissions. Confirm database user and name.","type":"access_denied_exception"},"status":401}`,
			err:          true,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{})

			// Try to connect to the database as this user.
			dbConn, proxy, err := testCtx.openSearchClient(ctx, test.user, "OpenSearch", test.dbUser)

			t.Cleanup(func() {
				_ = proxy.Close()
			})

			require.NoError(t, err)

			// Execute a query.
			result, err := dbConn.Count()
			require.NoError(t, err)
			t.Logf("result: %v", result)

			payload, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			require.Equal(t, test.payload, string(payload))

			if test.err {
				require.True(t, result.IsError())
				require.Equal(t, 401, result.StatusCode)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError())
			require.False(t, result.HasWarnings())
			require.Equal(t, `[200 OK] `, result.String())

			require.NoError(t, result.Body.Close())
		})
	}
}

func TestAuditOpenSearch(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	testCtx.server = testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases:         []types.Database{withOpenSearch("OpenSearch")(t, ctx, testCtx)},
		AWSConfigProvider: &mocks.AWSConfigProvider{},
	})
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		// Access denied should trigger an unsuccessful session start event.
		dbConn, proxy, err := testCtx.openSearchClient(ctx, "alice", "OpenSearch", "notadmin")
		require.NoError(t, err)

		resp, err := dbConn.Ping()

		require.NoError(t, err)
		require.True(t, resp.IsError())

		waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
		proxy.Close()
	})

	dbConn, proxy, err := testCtx.openSearchClient(ctx, "alice", "OpenSearch", "admin")
	require.NoError(t, err)

	t.Cleanup(func() {
		if proxy != nil {
			proxy.Close()
		}
	})

	t.Run("session starts event", func(t *testing.T) {
		// Connect should trigger successful session start event.
		resp, err := dbConn.Ping()
		require.NoError(t, err)
		require.False(t, resp.IsError())
		_ = waitForEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		_ = waitForEvent(t, testCtx, libevents.OpenSearchRequestCode)
	})

	t.Run("command sends", func(t *testing.T) {
		// should trigger Query event.
		result, err := dbConn.Count()
		require.NoError(t, err)
		require.Equal(t, `[200 OK] {"count":31874,"_shards":{"total":6,"successful":6,"skipped":0,"failed":0}}`, result.String())

		ev := waitForEvent(t, testCtx, libevents.OpenSearchRequestCode)
		require.Equal(t, "/_count", ev.(*events.OpenSearchRequest).Path)
		require.True(t, true)
	})
}

func withOpenSearch(name string, opts ...opensearch.TestServerOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		OpenSearchServer, err := opensearch.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.NoClientCert, // we are not using mTLS
		}, opts...)
		require.NoError(t, err)
		go OpenSearchServer.Serve()
		t.Cleanup(func() { OpenSearchServer.Close() })

		require.Len(t, testCtx.databaseCA.GetActiveKeys().TLS, 1)
		ca := string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert)

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolOpenSearch,
			URI:      net.JoinHostPort("localhost", OpenSearchServer.Port()),
			AWS: types.AWS{
				Region:    "us-west-1",
				AccountID: "123456789012",
			},
			TLS: types.DatabaseTLS{
				CACert: ca,
			},
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.opensearch[name] = testOpenSearch{
			db:       OpenSearchServer,
			resource: database,
		}
		return database
	}
}
