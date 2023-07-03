// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package db

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/opensearch"
)

func registerTestOpenSearchEngine() {
	common.RegisterEngine(newTestOpenSearchEngine, defaults.ProtocolOpenSearch)
}

func newTestOpenSearchEngine(ec common.EngineConfig) common.Engine {
	staticAWSCredentials := func(client.ConfigProvider, time.Time, string, string, string) *credentials.Credentials {
		return credentials.NewStaticCredentials("AKIDl", "SECRET", "SESSION")
	}

	return &opensearch.Engine{
		EngineConfig: ec,
		// inject mock AWS credentials.
		GetSigningCredsFn: staticAWSCredentials,
	}
}

func TestAccessOpenSearch(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withOpenSearch("OpenSearch"))
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
	testCtx := setupTestContext(ctx, t, withOpenSearch("OpenSearch"))
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
	return func(t *testing.T, ctx context.Context, testCtx *testContext) types.Database {
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
