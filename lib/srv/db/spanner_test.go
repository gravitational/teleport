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
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	gspanner "cloud.google.com/go/spanner"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/spanner"
)

func TestAccessSpanner(t *testing.T) {
	ctx := context.Background()
	const dbServiceName = "my-spanner"
	testCtx := setupTestContext(ctx, t, withSpanner(dbServiceName, cloudSpannerAuthToken, func(db *types.DatabaseV3) {
		db.SetStaticLabels(map[string]string{"foo": "bar"})
	}))
	go testCtx.startHandlingConnections()

	dynamicDBLabels := types.Labels{"echo": {"test"}}
	staticDBLabels := types.Labels{"foo": {"bar"}}
	tests := []struct {
		desc          string
		user          string
		role          string
		allowDbNames  []string
		allowDbUsers  []string
		extraRoleOpts []roleOptFn
		dbName        string
		dbUser        string
		err           string
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "googlesql",
			dbUser:       "admin",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "googlesql",
			dbUser:       "admin",
			err:          "access to db denied",
		},
		{
			desc:         "no access to databases",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{types.Wildcard},
			dbName:       "googlesql",
			dbUser:       "admin",
			err:          "access to db denied",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{types.Wildcard},
			allowDbUsers: []string{},
			dbName:       "googlesql",
			dbUser:       "admin",
			err:          "access to db denied",
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"googlesql"},
			allowDbUsers: []string{"alice"},
			dbName:       "googlesql",
			dbUser:       "alice",
		},
		{
			desc:          "access denied to specific user",
			user:          "alice",
			role:          "admin",
			allowDbNames:  []string{"googlesql"},
			allowDbUsers:  []string{"alice"},
			dbName:        "googlesql",
			dbUser:        "alice",
			extraRoleOpts: []roleOptFn{withDeniedDatabaseUsers("alice")},
			err:           "access to db denied",
		},
		{
			desc:          "access denied to specific database",
			user:          "alice",
			role:          "admin",
			allowDbNames:  []string{"googlesql"},
			allowDbUsers:  []string{"alice"},
			dbName:        "googlesql",
			dbUser:        "alice",
			extraRoleOpts: []roleOptFn{withDeniedDatabaseNames("googlesql")},
			err:           "access to db denied",
		},
		{
			desc:         "access allowed to specific user/database by static label",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"googlesql"},
			allowDbUsers: []string{"alice"},
			// The default test role created has wildcard labels allowed.
			// This tests that specific allowed database labels matching the
			// test database's static labels allows access.
			extraRoleOpts: []roleOptFn{withAllowedDBLabels(staticDBLabels)},
			dbName:        "googlesql",
			dbUser:        "alice",
		},
		{
			desc:         "access allowed to specific user/database by dynamic label",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"googlesql"},
			allowDbUsers: []string{"alice"},
			// The default test role created has wildcard labels allowed.
			// This tests that specific allowed database labels matching the
			// test database's dynamic labels allows access, to ensure
			// that RBAC checks against dynamic labels are working.
			extraRoleOpts: []roleOptFn{withAllowedDBLabels(dynamicDBLabels)},
			dbName:        "googlesql",
			dbUser:        "alice",
		},
		{
			desc:         "access denied by dynamic label",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"googlesql"},
			allowDbUsers: []string{"alice"},
			// The default test role created has wildcard labels allowed.
			// This tests that specific denied database labels matching the
			// test database's dynamic labels denies access, to ensure
			// that RBAC checks against dynamic labels are working.
			extraRoleOpts: []roleOptFn{withDeniedDBLabels(dynamicDBLabels)},
			dbName:        "googlesql",
			dbUser:        "alice",
			err:           "access to db denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			testCtx.createUserAndRole(ctx, t, test.user, test.role,
				test.allowDbUsers, test.allowDbNames, test.extraRoleOpts...)

			// Try to connect to the database as this user.
			clt, localProxy, err := testCtx.spannerClient(ctx, test.user, dbServiceName, test.dbUser, test.dbName)
			// authz isn't checked until RPCs are sent, so we can't fail here.
			require.NoError(t, err)
			t.Cleanup(func() {
				// Disconnect.
				clt.Close()
				_ = localProxy.Close()
			})

			// Execute a query.
			row, err := pingSpanner(ctx, clt, 7)
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, row)

			var got int64
			require.NoError(t, row.Column(0, &got))
			require.Equal(t, int64(7), got)
		})
	}
}

func TestAuditSpanner(t *testing.T) {
	ctx := context.Background()
	const dbServiceName = "my-spanner"
	testCtx := setupTestContext(ctx, t, withSpanner(dbServiceName, cloudSpannerAuthToken, func(db *types.DatabaseV3) {
		db.SetStaticLabels(map[string]string{"foo": "bar"})
	}))
	go testCtx.startHandlingConnections()

	const (
		userName    = "alice"
		roleName    = "admin"
		allowedUser = "admin"
		deniedUser  = "notAdmin"
	)
	testCtx.createUserAndRole(ctx, t, userName, roleName, []string{allowedUser}, []string{types.Wildcard})

	t.Run("access denied", func(t *testing.T) {
		// authz isn't checked until RPCs are sent, so we can't fail here.
		clt, localProxy, err := testCtx.spannerClient(ctx, userName, dbServiceName, deniedUser, "googlesql")
		require.NoError(t, err)
		t.Cleanup(func() {
			clt.Close()
			_ = localProxy.Close()
		})

		require.NoError(t, clt.WaitForConnectionState(ctx, connectivity.Ready))
		reconnectingCh := make(chan bool)
		go func() {
			// we should observe the connection leave the "ready" state after
			// it gets an access denied error.
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			reconnectingCh <- clt.ClientConn.WaitForStateChange(ctx, connectivity.Ready)
		}()

		row, err := pingSpanner(ctx, clt, 42)
		require.Error(t, err)
		require.ErrorContains(t, err, "access to db denied")
		require.Nil(t, row)

		ev := requireEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
		dbStart1, ok := ev.(*events.DatabaseSessionStart)
		require.True(t, ok)
		require.Equal(t, "googlesql", dbStart1.DatabaseName)

		require.True(t, <-reconnectingCh, "timed out waiting for the spanner client to reconnect")
		row, err = pingSpanner(ctx, clt, 42)
		require.Error(t, err)
		require.ErrorContains(t, err, "access to db denied")
		require.Nil(t, row)

		ev = requireEvent(t, testCtx, libevents.DatabaseSessionStartFailureCode)
		dbStart2, ok := ev.(*events.DatabaseSessionStart)
		require.True(t, ok)
		require.Equal(t, "googlesql", dbStart2.DatabaseName)

		// session start failure is sticky and causes the connection to shut
		// down - a client should get the same error but Teleport shouldnt emit
		// another start event for the same session, i.e. the client should be
		// forced to reconnect for subsequent RPC attempts
		require.NotEqual(t, dbStart1.SessionID, dbStart2.SessionID)

		// make sure no other events get emitted, including RPC failures, since
		// a session was never started successfully.
		select {
		case evt := <-testCtx.emitter.C():
			require.FailNow(t, "an unexpected audit event was emitted", "got an audit event with code %v", evt.GetCode())
		case <-time.After(1 * time.Second):
		}
	})

	t.Run("successful flow", func(t *testing.T) {
		clt, localProxy, err := testCtx.spannerClient(ctx, userName, dbServiceName, allowedUser, "googlesql")
		require.NoError(t, err)
		t.Cleanup(func() {
			clt.Close()
			_ = localProxy.Close()
		})

		// Successful connection should trigger session start event and RPCs.
		row, err := pingSpanner(ctx, clt, 42)
		require.NoError(t, err)
		require.NotNil(t, row)
		var got int64
		require.NoError(t, row.Column(0, &got))
		require.Equal(t, int64(42), got)

		evt := requireEvent(t, testCtx, libevents.DatabaseSessionStartCode)
		startEvt, ok := evt.(*events.DatabaseSessionStart)
		require.True(t, ok)
		require.Equal(t, "googlesql", startEvt.DatabaseName)

		rpcEvt := requireSpannerRPCEvent(t, testCtx)
		require.Equal(t, "BatchCreateSessions", rpcEvt.Procedure)
		require.Equal(t, "googlesql", rpcEvt.DatabaseName)
		require.Equal(t, startEvt.SessionID, rpcEvt.SessionID)

		rpcEvt = requireSpannerRPCEvent(t, testCtx)
		require.Equal(t, "ExecuteStreamingSql", rpcEvt.Procedure)
		require.Equal(t, "googlesql", rpcEvt.DatabaseName)
		require.Equal(t, startEvt.SessionID, rpcEvt.SessionID)

		// Client disconnects.
		clt.Close()
		_ = requireEvent(t, testCtx, libevents.DatabaseSessionEndCode)
	})
}

func pingSpanner(ctx context.Context, clt *spanner.SpannerTestClient, want int64) (*gspanner.Row, error) {
	query := gspanner.NewStatement(fmt.Sprintf("SELECT %d", want))
	rowIter := clt.Single().Query(ctx, query)
	defer rowIter.Stop()

	return rowIter.Next()
}

func withSpanner(name, authToken string, dbOpts ...databaseOption) withDatabaseOption {
	return func(t testing.TB, ctx context.Context, testCtx *testContext) types.Database {
		spannerServer, err := spanner.NewTestServer(common.TestServerConfig{
			Name:       name,
			AuthClient: testCtx.authClient,
			ClientAuth: tls.NoClientCert, // we are not using mTLS
			AuthToken:  authToken,
		})
		require.NoError(t, err)
		go spannerServer.Serve()
		t.Cleanup(func() { spannerServer.Close() })

		require.Len(t, testCtx.databaseCA.GetActiveKeys().TLS, 1)
		ca := string(testCtx.databaseCA.GetActiveKeys().TLS[0].Cert)

		database, err := types.NewDatabaseV3(types.Metadata{
			Name: name,
		}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolSpanner,
			URI:      net.JoinHostPort("localhost", spannerServer.Port()),
			GCP: types.GCPCloudSQL{
				ProjectID:  "project-id",
				InstanceID: "instance-id",
			},
			TLS: types.DatabaseTLS{
				CACert: ca,
			},
			DynamicLabels: dynamicLabels,
		})
		require.NoError(t, err)
		for _, dbOpt := range dbOpts {
			dbOpt(database)
		}
		testCtx.spanner[name] = testSpannerDB{
			db:       spannerServer,
			resource: database,
		}
		return database
	}
}
