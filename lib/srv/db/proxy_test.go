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
	"bytes"
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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/tlsca"
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

	clientTLSCfg, err := common.MakeTestClientTLSConfig(common.TestClientConfig{
		AuthClient: testCtx.authClient,
		AuthServer: testCtx.authServer,
		Cluster:    testCtx.clusterName,
		Username:   "alice",
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: "pgsvc",
			Protocol:    defaults.ProtocolPostgres,
			Username:    "pguser",
			Database:    "pgdb",
		},
	})
	require.NoError(t, err)

	type proxyTarget struct {
		name            string
		addr            string
		wantSSLResponse []byte
	}

	proxyTargets := []proxyTarget{
		{
			name:            "local proxy",
			addr:            localProxy.GetAddr(),
			wantSSLResponse: []byte("N"),
		},
		{
			name:            "proxy multiplexer",
			addr:            testCtx.mux.DB().Addr().String(),
			wantSSLResponse: []byte("S"),
		},
	}

	// We have to send messages manually since pgconn does not support GSS encryption.
	type task struct {
		sendMsg pgproto3.FrontendMessage
		wantErr error
	}

	// build a basic startup message we can use
	startupMsg := pgproto3.StartupMessage{
		ProtocolVersion: pgproto3.ProtocolVersionNumber,
		Parameters:      make(map[string]string),
	}
	startupMsg.Parameters["user"] = "pguser"
	startupMsg.Parameters["database"] = "pgdb"

	tests := []struct {
		name  string
		tasks []task
	}{
		{
			name: "handles GSSEncRequest followed by SSLRequest then StartupMessage",
			tasks: []task{
				{
					sendMsg: &pgproto3.GSSEncRequest{},
				},
				{
					sendMsg: &pgproto3.SSLRequest{},
				},
				{
					sendMsg: &startupMsg,
				},
				{
					sendMsg: &pgproto3.Terminate{},
					wantErr: io.EOF,
				},
			},
		},
		{
			name: "handles SSLRequest followed by GSSEncRequest then StartupMessage and Terminate",
			tasks: []task{
				{
					sendMsg: &pgproto3.SSLRequest{},
				},
				{
					sendMsg: &pgproto3.GSSEncRequest{},
				},
				{
					sendMsg: &startupMsg,
				},
				{
					sendMsg: &pgproto3.Terminate{},
					wantErr: io.EOF,
				},
			},
		},
		{
			name: "closes connection on repeated GSSEncRequest",
			tasks: []task{
				{
					sendMsg: &pgproto3.GSSEncRequest{},
				},
				{
					sendMsg: &pgproto3.GSSEncRequest{},
					wantErr: io.EOF,
				},
			},
		},
		{
			name: "closes connection on repeated SSLRequest",
			tasks: []task{
				{
					sendMsg: &pgproto3.SSLRequest{},
				},
				{
					sendMsg: &pgproto3.SSLRequest{},
					wantErr: io.EOF,
				},
			},
		},
		{
			name: "handles cancel request and closes connection",
			tasks: []task{
				{
					sendMsg: &pgproto3.CancelRequest{},
					wantErr: io.EOF,
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
				conn, err := net.Dial("tcp", proxy.addr)
				require.NoError(t, err)
				defer conn.Close()
				for _, task := range tt.tasks {
					payload, err := task.sendMsg.Encode(nil)
					require.NoError(t, err, "FrontendMessage.Encode failed")

					nWritten, err := conn.Write(payload)
					require.NoError(t, err)
					require.Len(t, payload, nWritten, "failed to fully write payload")

					var checkResponse responseChecker
					var needsTLSUpgrade bool
					switch task.sendMsg.(type) {
					case *pgproto3.SSLRequest:
						checkResponse = checkNextMessage(conn, proxy.wantSSLResponse)
						needsTLSUpgrade = bytes.Equal(proxy.wantSSLResponse, []byte("S"))
					case *pgproto3.GSSEncRequest:
						checkResponse = checkNextMessage(conn, []byte("N"))
					case *pgproto3.StartupMessage:
						checkResponse = checkReceiveReadyMessage
					case *pgproto3.Terminate:
						// try to read one byte to check for expected EOF
						checkResponse = checkNextMessage(conn, []byte("x"))
					case *pgproto3.CancelRequest:
						// try to read one byte to check for expected EOF
						checkResponse = checkNextMessage(conn, []byte("x"))
					default:
						require.FailNow(t, "unexpected encoder used in test case")
					}
					checkResponse(t, conn, task.wantErr)
					if task.wantErr != nil {
						continue
					}
					if needsTLSUpgrade {
						tlsConn := tls.Client(conn, clientTLSCfg)
						require.NoError(t, tlsConn.Handshake())
						conn = tlsConn
					}
				}
			})
		}
	}
}

type responseChecker func(t *testing.T, conn net.Conn, wantErr error)

// checkNextMessage is a helper that gets one message from the backend and checks it.
func checkNextMessage(conn net.Conn, wantMsg []byte) responseChecker {
	return func(t *testing.T, conn net.Conn, wantErr error) {
		buf := make([]byte, 512)
		nRead, err := io.ReadAtLeast(conn, buf, len(wantMsg))
		if wantErr != nil {
			require.Error(t, err)
			require.ErrorIs(t, err, wantErr)
			return
		}
		require.NoError(t, err)
		require.Equal(t, wantMsg, buf[:nRead])
	}
}

// checkReceiveReadyMessage checks that a pgproto3.ReadyForQuery message is eventually received.
func checkReceiveReadyMessage(t *testing.T, conn net.Conn, wantErr error) {
	frontend := pgproto3.NewFrontend(pgproto3.NewChunkReader(conn), conn)
	for {
		msg, err := frontend.Receive()
		if wantErr != nil {
			require.Error(t, err)
			require.ErrorIs(t, err, wantErr)
			return
		}
		require.NoError(t, err)
		switch msg.(type) {
		case *pgproto3.ReadyForQuery:
			return
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
	require.Eventually(t, func() bool {
		err := mysql.Ping()
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "failed to disconnect client conn")
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
	require.Eventually(t, func() bool {
		err := mysql.Ping()
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "failed to disconnect client conn")
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
	require.Eventually(t, func() bool {
		err := mysql.Ping()
		return err != nil
	}, 5*time.Second, 100*time.Millisecond, "failed to disconnect client conn")
}

func setConfigClientIdleTimoutAndDisconnectExpiredCert(ctx context.Context, t *testing.T, auth *auth.Server, timeout time.Duration) {
	authPref, err := auth.GetAuthPreference(ctx)
	require.NoError(t, err)
	authPref.SetDisconnectExpiredCert(true)
	_, err = auth.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	netConfig, err := auth.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err)
	netConfig.SetClientIdleTimeout(timeout)
	_, err = auth.UpsertClusterNetworkingConfig(ctx, netConfig)
	require.NoError(t, err)
}

func TestExtractMySQLVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedMySQL("mysql", withMySQLServerVersion("8.0.25")))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{"root"}, []string{types.Wildcard})

	version, err := mysql.FetchMySQLVersion(ctx, testCtx.server.proxiedDatabases["mysql"])
	require.NoError(t, err)
	require.Equal(t, "8.0.25", version)
}
