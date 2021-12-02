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
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/multiplexer"

	"github.com/stretchr/testify/require"
)

// TestProxyProtocolPostgres ensures that clients can successfully connect to a
// Postgres database when Teleport is running behind a proxy that sends a proxy
// line.
func TestProxyProtocolPostgres(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"postgres"}, []string{"postgres"})

	// Point our proxy to the Teleport's db listener on the multiplexer.
	proxy, err := multiplexer.NewTestProxy(testCtx.mux.DB().Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { proxy.Close() })
	go proxy.Serve()

	// Connect to the proxy instead of directly to Postgres listener and make
	// sure the connection succeeds.
	psql, err := testCtx.postgresClientWithAddr(ctx, proxy.Address(), "alice", "postgres", "postgres", "postgres")
	require.NoError(t, err)
	require.NoError(t, psql.Close(ctx))
}

// TestProxyProtocolMySQL ensures that clients can successfully connect to a
// MySQL database when Teleport is running behind a proxy that sends a proxy
// line.
func TestProxyProtocolMySQL(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})

	// Point our proxy to the Teleport's MySQL listener.
	proxy, err := multiplexer.NewTestProxy(testCtx.mysqlListener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { proxy.Close() })
	go proxy.Serve()

	// Connect to the proxy instead of directly to MySQL listener and make
	// sure the connection succeeds.
	mysql, err := testCtx.mysqlClientWithAddr(proxy.Address(), "alice", "mysql", "root")
	require.NoError(t, err)
	require.NoError(t, mysql.Close())
}

// TestProxyProtocolMongo ensures that clients can successfully connect to a
// Mongo database when Teleport is running behind a proxy that sends a proxy
// line.
func TestProxyProtocolMongo(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	// Point our proxy to the Teleport's TLS listener.
	proxy, err := multiplexer.NewTestProxy(testCtx.webListener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { proxy.Close() })
	go proxy.Serve()

	// Connect to the proxy instead of directly to Teleport listener and make
	// sure the connection succeeds.
	mongo, err := testCtx.mongoClientWithAddr(ctx, proxy.Address(), "alice", "mongo", "admin")
	require.NoError(t, err)
	require.NoError(t, mongo.Disconnect(ctx))
}

// TestProxyClientDisconnectDueToIdleConnection ensures that idle clients will be disconnected.
func TestProxyClientDisconnectDueToIdleConnection(t *testing.T) {
	const (
		idleClientTimeout             = time.Minute
		connMonitorDisconnectTimeBuff = time.Second * 5
	)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})
	setConfigClientIdleTimoutAndDisconnectExpiredCert(ctx, t, testCtx.authServer, idleClientTimeout)

	mysql, err := testCtx.mysqlClient("alice", "mysql", "root")
	require.NoError(t, err)

	err = mysql.Ping()
	require.NoError(t, err)

	testCtx.clock.Advance(idleClientTimeout + connMonitorDisconnectTimeBuff)

	waitForEvent(t, testCtx, events.ClientDisconnectCode)
	err = mysql.Ping()
	require.Error(t, err)
}

// TestProxyClientDisconnectDueToCertExpiration ensures that if the DisconnectExpiredCert cluster flag is enabled
// clients will be disconnected after cert expiration.
func TestProxyClientDisconnectDueToCertExpiration(t *testing.T) {
	const (
		ttlClientCert = time.Hour
	)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})
	setConfigClientIdleTimoutAndDisconnectExpiredCert(ctx, t, testCtx.authServer, time.Hour*24)

	mysql, err := testCtx.mysqlClient("alice", "mysql", "root")
	require.NoError(t, err)

	err = mysql.Ping()
	require.NoError(t, err)

	testCtx.clock.Advance(ttlClientCert)

	waitForEvent(t, testCtx, events.ClientDisconnectCode)
	err = mysql.Ping()
	require.Error(t, err)
}

// TestProxyClientDisconnectDueToLockInForce ensures that clients will be
// disconnected when there is a matching lock in force.
func TestProxyClientDisconnectDueToLockInForce(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})

	mysql, err := testCtx.mysqlClient("alice", "mysql", "root")
	require.NoError(t, err)

	err = mysql.Ping()
	require.NoError(t, err)

	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: types.LockTarget{User: "alice"},
	})
	require.NoError(t, err)
	testCtx.authServer.UpsertLock(ctx, lock)

	waitForEvent(t, testCtx, events.ClientDisconnectCode)
	err = mysql.Ping()
	require.Error(t, err)
}

func setConfigClientIdleTimoutAndDisconnectExpiredCert(ctx context.Context, t *testing.T, auth *auth.Server, timeout time.Duration) {
	authPref, err := auth.GetAuthPreference(ctx)
	require.NoError(t, err)
	authPref.SetDisconnectExpiredCert(true)
	err = auth.SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	netConfig, err := auth.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err)
	netConfig.SetClientIdleTimeout(timeout)
	err = auth.SetClusterNetworkingConfig(ctx, netConfig)
	require.NoError(t, err)
}
