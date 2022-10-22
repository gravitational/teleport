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
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgproto3/v2"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
)

// TestProxyProtocolPostgres ensures that clients can successfully connect to a
// Postgres database when Teleport is running behind a proxy that sends a proxy
// line.
func TestProxyProtocolPostgres(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"postgres"}, []string{"postgres"})

	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("v2=%v", v2), func(t *testing.T) {
			// Point our proxy to the Teleport's db listener on the multiplexer.
			proxy, err := multiplexer.NewTestProxy(testCtx.mux.DB().Addr().String(), v2)
			require.NoError(t, err)
			t.Cleanup(func() { proxy.Close() })
			go proxy.Serve()

			// Connect to the proxy instead of directly to Postgres listener and make
			// sure the connection succeeds.
			psql, err := testCtx.postgresClientWithAddr(ctx, proxy.Address(), "alice", "postgres", "postgres", "postgres")
			require.NoError(t, err)
			require.NoError(t, psql.Close(ctx))
		})
	}
}

// TestProxyProtocolPostgresStartup tests that the proxy correctly handles startup messages for PostgreSQL.
// Specifically, this test verifies that:
//   - The proxy handles a GSSEncRequest by responding "N" to indicate that GSS encryption is not supported.
//   - The proxy allows a client to send a SSLRequest after it is told GSS encryption is not supported.
//   - The proxy allows a client to send a GSSEncRequest after it is told SSL encryption is not supported.
//   - The proxy closes the connection if it receives a repeated SSLRequest or GSSEncRequest.
//
// This behavior allows a client to decide what it should do based on the responses from the proxy server.
func TestProxyProtocolPostgresStartup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("pgsvc"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"pguser"}, []string{"pgdb"})
	_, localProxy, err := testCtx.postgresClientLocalProxy(ctx, "alice", "pgsvc", "pguser", "pgdb")
	require.NoError(t, err)

	clientTLSCfg, err := testCtx.tlsServer.ClientTLSConfig(auth.TestUser("alice"))
	require.NoError(t, err)

	// We have to send messages manually since pgconn does not support GSS encryption.
	// Therefore we create some []byte payloads to write into a connection ourselves.
	gssReq := (&pgproto3.GSSEncRequest{}).Encode(nil)
	sslReq := (&pgproto3.SSLRequest{}).Encode(nil)
	startupReq := (&pgproto3.StartupMessage{}).Encode(nil)
	terminateReq := (&pgproto3.Terminate{}).Encode(nil)

	type proxyTarget struct {
		name            string
		addr            string
		wantSSLResponse string
	}

	proxyTargets := []proxyTarget{
		{
			name:            "local proxy",
			addr:            localProxy.GetAddr(),
			wantSSLResponse: "N",
		},
		{
			name:            "proxy multiplexer",
			addr:            testCtx.mux.DB().Addr().String(),
			wantSSLResponse: "S",
		},
	}

	// responseOracle returns the proxyTarget's expected response and whether TLS upgrade is required.
	type responseOracle func(cfg *proxyTarget) (wantResponse string, upgradeToTLS bool)
	sslResponseOracle := func(cfg *proxyTarget) (string, bool) {
		switch cfg.wantSSLResponse {
		case "N":
			return "N", false // SSLRequest not supported; client doesn't need to upgrade conn to TLS to proceed.
		case "S":
			return "S", true // SSLRequest is supported; client needs to upgrade conn to TLS to proceed.
		default:
			panic("unreachable")
		}
	}
	gssResponseOracle := func(_ *proxyTarget) (string, bool) {
		return "N", false // GSSEncRequest not supported; client doesn't need to upgrade conn to TLS to proceed.
	}
	startupResponseOracle := func(_ *proxyTarget) (string, bool) {
		return "", false // just verify the connection is still open, but don't expect a response.
	}

	type task struct {
		payload     []byte
		wantReadErr error
		oracle      responseOracle
	}

	tests := []struct {
		name  string
		tasks []task
	}{
		{
			name: "handles GSSEncRequest followed by SSLRequest then StartupMessage and Terminate",
			tasks: []task{
				{
					payload: gssReq,
					oracle:  gssResponseOracle,
				},
				{
					payload: sslReq,
					oracle:  sslResponseOracle,
				},
				{
					payload: startupReq,
					oracle:  startupResponseOracle,
				},
				{
					payload:     terminateReq,
					wantReadErr: io.EOF,
				},
			},
		},
		{
			name: "handles SSLRequest followed by GSSEncRequest then StartupMessage and Terminate",
			tasks: []task{
				{
					payload: sslReq,
					oracle:  sslResponseOracle,
				},
				{
					payload: gssReq,
					oracle:  gssResponseOracle,
				},
				{
					payload: startupReq,
					oracle:  startupResponseOracle,
				},
				{
					payload:     terminateReq,
					wantReadErr: io.EOF,
				},
			},
		},
		{
			name: "closes connection on repeated GSSEncRequest",
			tasks: []task{
				{
					payload: gssReq,
					oracle:  gssResponseOracle,
				},
				{
					payload:     gssReq,
					wantReadErr: io.EOF,
				},
			},
		},
		{
			name: "closes connection on repeated SSLRequest",
			tasks: []task{
				{
					payload: sslReq,
					oracle:  sslResponseOracle,
				},
				{
					payload:     sslReq,
					wantReadErr: io.EOF,
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		for _, proxy := range proxyTargets {
			proxy := proxy
			testName := fmt.Sprintf("%s %s", proxy.name, tt.name)
			t.Run(testName, func(t *testing.T) {
				t.Parallel()
				rcvBuf := make([]byte, 512)
				conn, err := net.Dial("tcp", proxy.addr)
				require.NoError(t, err)
				defer conn.Close()
				for _, task := range tt.tasks {
					nWritten, err := conn.Write(task.payload)
					require.NoError(t, err)
					require.Equal(t, len(task.payload), nWritten, "failed to fully write payload")
					nRead, err := io.ReadAtLeast(conn, rcvBuf, 1)
					if task.wantReadErr != nil {
						require.Error(t, err)
						require.ErrorIs(t, err, task.wantReadErr)
						continue
					}
					wantResponse, needsTLSUpgrade := task.oracle(&proxy)
					require.Equal(t, wantResponse, string(rcvBuf[:nRead]))
					if needsTLSUpgrade {
						conn = tls.Client(conn, clientTLSCfg)
					}
				}
			})
		}
	}
}

// TestProxyProtocolMySQL ensures that clients can successfully connect to a
// MySQL database when Teleport is running behind a proxy that sends a proxy
// line.
func TestProxyProtocolMySQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})

	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("v2=%v", v2), func(t *testing.T) {
			// Point our proxy to the Teleport's MySQL listener.
			proxy, err := multiplexer.NewTestProxy(testCtx.mysqlListener.Addr().String(), v2)
			require.NoError(t, err)
			t.Cleanup(func() { proxy.Close() })
			go proxy.Serve()

			// Connect to the proxy instead of directly to MySQL listener and make
			// sure the connection succeeds.
			mysql, err := testCtx.mysqlClientWithAddr(proxy.Address(), "alice", "mysql", "root")
			require.NoError(t, err)
			require.NoError(t, mysql.Close())
		})
	}
}

// TestProxyProtocolMongo ensures that clients can successfully connect to a
// Mongo database when Teleport is running behind a proxy that sends a proxy
// line.
func TestProxyProtocolMongo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMongo("mongo"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("v2=%v", v2), func(t *testing.T) {
			// Point our proxy to the Teleport's TLS listener.
			proxy, err := multiplexer.NewTestProxy(testCtx.webListener.Addr().String(), v2)
			require.NoError(t, err)
			t.Cleanup(func() { proxy.Close() })
			go proxy.Serve()

			// Connect to the proxy instead of directly to Teleport listener and make
			// sure the connection succeeds.
			mongo, err := testCtx.mongoClientWithAddr(ctx, proxy.Address(), "alice", "mongo", "admin")
			require.NoError(t, err)
			require.NoError(t, mongo.Disconnect(ctx))
		})
	}
}

func TestProxyProtocolRedis(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedRedis("redis"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"admin"}, []string{types.Wildcard})

	for _, v2 := range []bool{false, true} {
		t.Run(fmt.Sprintf("v2=%v", v2), func(t *testing.T) {
			// Point our proxy to the Teleport's TLS listener.
			proxy, err := multiplexer.NewTestProxy(testCtx.webListener.Addr().String(), v2)
			require.NoError(t, err)
			t.Cleanup(func() { proxy.Close() })
			go proxy.Serve()

			// Connect to the proxy instead of directly to Teleport listener and make
			// sure the connection succeeds.
			redisClient, err := testCtx.redisClientWithAddr(ctx, proxy.Address(), "alice", "redis", "admin")
			require.NoError(t, err)

			// Send ECHO to Redis server and check if we get it back.
			resp := redisClient.Echo(ctx, "hello")
			require.NoError(t, resp.Err())
			require.Equal(t, "hello", resp.Val())

			require.NoError(t, redisClient.Close())
		})
	}
}

// TestProxyClientDisconnectDueToIdleConnection ensures that idle clients will be disconnected.
func TestProxyClientDisconnectDueToIdleConnection(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	err = testCtx.authServer.UpsertLock(ctx, lock)
	require.NoError(t, err)

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

func TestExtractMySQLVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql", mysql.WithServerVersion("8.0.25")))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})

	version, err := mysql.FetchMySQLVersion(ctx, testCtx.server.proxiedDatabases["mysql"])
	require.NoError(t, err)
	require.Equal(t, "8.0.25", version)
}
