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
	"net"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials"
	awsdynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/dynamodb"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/migration"
)

func registerTestDynamoDBEngine() {
	// Override DynamoDB engine that is used normally with the test one
	// with custom HTTP client.
	common.RegisterEngine(newTestDynamoDBEngine, defaults.ProtocolDynamoDB)
}

func newTestDynamoDBEngine(ec common.EngineConfig) common.Engine {
	return &dynamodb.Engine{
		EngineConfig:  ec,
		RoundTrippers: make(map[string]http.RoundTripper),
		// inject mock AWS credentials.
		CredentialsGetter: awsutils.NewStaticCredentialsGetter(
			migration.NewCredentialsAdapter(
				credentials.NewStaticCredentialsProvider("AKIDl", "SECRET", "SESSION"),
			),
		),
	}
}

func TestAccessDynamoDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mockTables := []string{"table-one", "table-two"}
	testCtx := setupTestContext(ctx, t,
		withDynamoDB("DynamoDB"))
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
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{types.Wildcard},
			dbUser:       "alice",
		},
		{
			desc:         "access allowed to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "alice",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "alice",
			wantErrMsg:   "access to db denied",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{},
			dbUser:       "alice",
			wantErrMsg:   "access to db denied",
		},
		{
			desc:         "access denied to specific user",
			user:         "alice",
			role:         "admin",
			allowDbUsers: []string{"alice"},
			dbUser:       "bob",
			wantErrMsg:   "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role, test.allowDbUsers, []string{} /*allow DB names*/)

			// Try to connect to the database as this user.
			clt, lp, err := testCtx.dynamodbClient(ctx, test.user, "DynamoDB", test.dbUser)
			t.Cleanup(func() {
				if lp != nil {
					lp.Close()
				}
			})
			require.NoError(t, err)

			// Execute a dynamodb query.
			out, err := clt.ListTables(ctx, &awsdynamodb.ListTablesInput{})
			if test.wantErrMsg != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.wantErrMsg)
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, mockTables, out.TableNames)
		})
	}
}

func TestAuditDynamoDB(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withDynamoDB("DynamoDB"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	clientCtx, cancel := context.WithCancel(ctx)
	t.Run("access denied", func(t *testing.T) {
		// Try to connect to the database as this user.
		clt, lp, err := testCtx.dynamodbClient(clientCtx, "alice", "DynamoDB", "notadmin")
		t.Cleanup(func() {
			if lp != nil {
				lp.Close()
			}
		})
		require.NoError(t, err)

		// Execute a dynamodb query.
		_, err = clt.ListTables(ctx, &awsdynamodb.ListTablesInput{})
		require.Error(t, err)
		require.ErrorContains(t, err, "access to db denied")
		requireEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
	})

	// HTTP request should trigger successful session start/end events and emit an audit event for the request.
	clt, lp, err := testCtx.dynamodbClient(clientCtx, "alice", "DynamoDB", "admin")
	t.Cleanup(func() {
		cancel()
		if lp != nil {
			lp.Close()
		}
	})
	require.NoError(t, err)

	t.Run("session starts and emits a request event", func(t *testing.T) {
		_, err := clt.ListTables(ctx, &awsdynamodb.ListTablesInput{})
		require.NoError(t, err)
		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		requireEvent(t, testCtx, libevents.DynamoDBRequestCode)
	})

	t.Run("session ends when client closes the connection", func(t *testing.T) {
		clt.HTTPClient.CloseIdleConnections()
		requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})

	t.Run("session ends when local proxy closes the connection", func(t *testing.T) {
		// closing local proxy and canceling the context used to start it should trigger session end event.
		// without this cancel, the session will not end until the smaller of client_idle_timeout or the testCtx closes.
		_, err := clt.ListTables(ctx, &awsdynamodb.ListTablesInput{})
		require.NoError(t, err)
		requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		requireEvent(t, testCtx, libevents.DynamoDBRequestCode)
		cancel()
		lp.Close()
		requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

func withDynamoDB(name string, opts ...dynamodb.TestServerOption) withDatabaseOption {
	return func(t testing.TB, _ context.Context, testCtx *testContext) types.Database {
		config := common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.NoClientCert, // DynamoDB is cloud hosted and does not use mTLS.
		}
		server, err := dynamodb.NewTestServer(config, opts...)
		require.NoError(t, err)
		go server.Serve()
		t.Cleanup(func() { server.Close() })

		require.Len(t, testCtx.databaseCA.GetActiveKeys().TLS, 1)
		ca := string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert)
		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol:      defaults.ProtocolDynamoDB,
			URI:           net.JoinHostPort("localhost", server.Port()),
			DynamicLabels: dynamicLabels,
			AWS: types.AWS{
				Region:    "us-west-1",
				AccountID: "123456789012",
			},
			TLS: types.DatabaseTLS{
				// Set CA, otherwise the engine will attempt to download and use the AWS CA.
				CACert: ca,
			},
		})
		require.NoError(t, err)

		testCtx.dynamodb[name] = testDynamoDB{
			db:       server,
			resource: database,
		}
		return database
	}
}
