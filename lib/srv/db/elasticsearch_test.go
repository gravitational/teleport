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
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/elasticsearch"
)

func registerTestElasticsearchEngine() {
	// Override Elasticsearch engine that is used normally with the test one
	// with custom HTTP client.
	common.RegisterEngine(newTestElasticsearchEngine, defaults.ProtocolElasticsearch)
}

func newTestElasticsearchEngine(ec common.EngineConfig) common.Engine {
	return &elasticsearch.Engine{
		EngineConfig: ec,
	}
}

func TestAccessElasticsearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withElasticsearch("Elasticsearch"))
	go testCtx.startHandlingConnections()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbUsers []string
		dbUser       string
		err          bool
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "Elasticsearch",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "Elasticsearch",
			err:          true,
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "Elasticsearch",
			err:          true,
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "",
			err:          true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{})

			// Try to connect to the database as this user.
			dbConn, proxy, err := testCtx.elasticsearchClient(ctx, test.user, "Elasticsearch", test.dbUser)

			t.Cleanup(func() {
				proxy.Close()
			})

			require.NoError(t, err)

			// Execute a query.
			result, err := dbConn.Query(`{ "query": "SELECT 42" }`)
			require.NoError(t, err)

			if test.err {
				t.Logf("result: %v", result)
				assert.NoError(t, result.Body.Close())
				require.Equal(t, http.StatusUnauthorized, result.StatusCode)
				return
			}

			out, err := io.ReadAll(result.Body)
			assert.NoError(t, err)
			assert.NoError(t, result.Body.Close())
			require.Equal(t, http.StatusOK, result.StatusCode)
			require.Equal(t, `{"columns":[{"name":"42","type":"integer"}],"rows":[[42]]}`, string(out))
		})
	}
}

func TestAuditElasticsearch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withElasticsearch("Elasticsearch"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		// Access denied should trigger an unsuccessful session start event.
		dbConn, proxy, err := testCtx.elasticsearchClient(ctx, "alice", "Elasticsearch", "notadmin")
		require.NoError(t, err)

		resp, err := dbConn.Ping()
		require.NoError(t, err)
		assert.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		waitForEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
		proxy.Close()
	})

	dbConn, proxy, err := testCtx.elasticsearchClient(ctx, "alice", "Elasticsearch", "admin")
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
		assert.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusOK, resp.StatusCode)
		waitForEvent(t, testCtx, libevents.DatabaseSessionStartCode)
	})

	t.Run("command sends", func(t *testing.T) {
		// should trigger Query event.
		result, err := dbConn.Query(`{ "query": "SELECT 42" }`)
		require.NoError(t, err)
		out, err := io.ReadAll(result.Body)
		assert.NoError(t, err)
		assert.NoError(t, result.Body.Close())
		assert.Equal(t, `{"columns":[{"name":"42","type":"integer"}],"rows":[[42]]}`, string(out))

		_ = waitForEvent(t, testCtx, libevents.ElasticsearchRequestCode)

		// actual query
		ev := waitForEvent(t, testCtx, libevents.ElasticsearchRequestCode)
		require.Equal(t, "/_sql", ev.(*events.ElasticsearchRequest).Path)
		require.Equal(t, []byte(`{ "query": "SELECT 42" }`), ev.(*events.ElasticsearchRequest).Body)
	})
}

func withElasticsearch(name string, opts ...elasticsearch.TestServerOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		ElasticsearchServer, err := elasticsearch.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}, opts...)
		require.NoError(t, err)
		go ElasticsearchServer.Serve()
		t.Cleanup(func() { ElasticsearchServer.Close() })
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolElasticsearch,
			URI:           net.JoinHostPort("localhost", ElasticsearchServer.Port()),
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		testCtx.elasticsearch[name] = testElasticsearch{
			db:       ElasticsearchServer,
			resource: database,
		}
		return database
	}
}
