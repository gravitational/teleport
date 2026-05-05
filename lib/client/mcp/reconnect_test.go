/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"testing/synctest"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func TestProxyStdioConn_autoReconnect(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		app := newAppFromURI(t, "some-mcp", "mcp+stdio://")

		var serverStdioSource atomic.Value
		prepServerWithVersion := func(version string) context.CancelFunc {
			testServer := mcptest.NewServerWithVersion(version)
			testServerSource, testServerDest := mustMakeConnPair(t)
			serverStdioSource.Store(testServerSource)
			listenCtx, cancel := context.WithCancel(ctx)

			// Note that "Listen" creates a background go-routine to handle
			// notifications that keeps running until ctx is canceled. Thus,
			// cancel must be called before synctest.Wait.
			go mcpserver.NewStdioServer(testServer).Listen(listenCtx, testServerDest, testServerDest)
			return cancel
		}
		cancelServer := prepServerWithVersion("1.0.0")

		clientStdioSource, clientStdioDest := mustMakeConnPair(t)
		stdioClient := mcptest.NewStdioClientFromConn(t, clientStdioSource)
		proxyError := make(chan error, 1)

		// Start proxy.
		go func() {
			proxyError <- ProxyStdioConn(ctx, ProxyStdioConnConfig{
				ClientStdio: clientStdioDest,
				GetApp: func(ctx context.Context) (types.Application, error) {
					return app, nil
				},
				DialServer: func(ctx context.Context) (net.Conn, error) {
					return serverStdioSource.Load().(net.Conn), nil
				},
				AutoReconnect: true,
			})
		}()

		// Initialize.
		mcptest.MustInitializeClient(t, stdioClient)

		// Call tool success.
		mcptest.MustCallServerTool(t, stdioClient)

		// Let's kill the server, CallTool should fail.
		serverStdioSource.Load().(io.ReadWriteCloser).Close()
		cancelServer()
		synctest.Wait()
		_, err := mcptest.CallServerTool(ctx, stdioClient)
		require.ErrorContains(t, err, "on closed pipe")

		// Let it try again with a successful reconnect.
		cancelServer = prepServerWithVersion("1.0.0")
		mcptest.MustCallServerTool(t, stdioClient)

		// Let's kill the server again, and prepare a different version.
		serverStdioSource.Load().(io.ReadWriteCloser).Close()
		cancelServer()
		synctest.Wait()
		cancelServer = prepServerWithVersion("2.0.0")
		_, err = mcptest.CallServerTool(ctx, stdioClient)
		require.ErrorContains(t, err, "server info has changed")

		// Cleanup.
		clientStdioSource.Close()
		cancelServer()
		synctest.Wait()
		require.NoError(t, <-proxyError)
	})
}

func TestProxyStdioConn_http(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		app := newAppFromURI(t, "some-mcp", "mcp+http://127.0.0.1:8888/mcp")

		// Remote MCP server.
		mcpServer := mcpserver.NewStreamableHTTPServer(mcptest.NewServer())
		listener := listenerutils.NewInMemoryListener()
		var receivedSessionEnd atomic.Bool
		httpServer := http.Server{
			Handler: http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodDelete {
					receivedSessionEnd.Store(true)
				}
				mcpServer.ServeHTTP(rw, req)
			}),
		}
		t.Cleanup(func() { httpServer.Close() })
		go httpServer.Serve(listener)

		// Start proxy.
		clientStdioSource, clientStdioDest := mustMakeConnPair(t)
		proxyError := make(chan error, 1)
		go func() {
			proxyError <- ProxyStdioConn(ctx, ProxyStdioConnConfig{
				ClientStdio: clientStdioDest,
				GetApp: func(ctx context.Context) (types.Application, error) {
					return app, nil
				},
				DialServer: func(ctx context.Context) (net.Conn, error) {
					return listener.DialContext(ctx, "tcp", "")
				},
				AutoReconnect: true,
			})
		}()

		// Local stdio client.
		stdioClient := mcptest.NewStdioClientFromConn(t, clientStdioSource)
		mcptest.MustInitializeClient(t, stdioClient)
		mcptest.MustCallServerTool(t, stdioClient)

		// Shut down.
		stdioClient.Close()
		synctest.Wait()
		require.NoError(t, <-proxyError)

		// Make sure the transport has sent out "session end" message before
		// ProxyStdioConn returns.
		assert.True(t, receivedSessionEnd.Load())
	})
}

func TestProxyStdioConn_autoReconnectDisabled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		app := newAppFromURI(t, "some-mcp", "mcp+stdio://")

		var mcpServerConnCount atomic.Uint32
		var mcpServerConn atomic.Value
		listenCtx, cancelListen := context.WithCancel(ctx)
		listener := listenerutils.NewInMemoryListener()
		t.Cleanup(func() { listener.Close() })

		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				mcpServerConnCount.Add(1)
				mcpServerConn.Store(conn)

				// Note that "Listen" creates a background go-routine to handle
				// notifications that keeps running until ctx is canceled. Thus,
				// cancelListen must be called before synctest.Wait.
				go mcpserver.NewStdioServer(mcptest.NewServer()).Listen(listenCtx, conn, conn)
			}
		}()

		// Start proxy.
		clientStdioSource, clientStdioDest := mustMakeConnPair(t)
		proxyError := make(chan error, 1)
		go func() {
			proxyError <- ProxyStdioConn(ctx, ProxyStdioConnConfig{
				ClientStdio: clientStdioDest,
				GetApp: func(ctx context.Context) (types.Application, error) {
					return app, nil
				},
				DialServer: func(ctx context.Context) (net.Conn, error) {
					return listener.DialContext(ctx, "tcp", "")
				},
				AutoReconnect: false,
			})
		}()

		// Local stdio client.
		stdioClient := mcptest.NewStdioClientFromConn(t, clientStdioSource)
		mcptest.MustInitializeClient(t, stdioClient)
		mcptest.MustCallServerTool(t, stdioClient)

		// Let's kill the server conn.
		connCloser, ok := mcpServerConn.Load().(io.Closer)
		require.True(t, ok)
		require.NoError(t, connCloser.Close())

		// Wait for ProxyStdioConn.
		cancelListen()
		synctest.Wait()
		require.NoError(t, <-proxyError)

		// New request should fail and no retry is performed.
		_, err := mcptest.CallServerTool(ctx, stdioClient)
		require.ErrorContains(t, err, "transport closed")
		require.Equal(t, uint32(1), mcpServerConnCount.Load())
	})
}

func mustMakeConnPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	source, dest := net.Pipe()
	t.Cleanup(func() {
		source.Close()
		dest.Close()
	})
	return source, dest
}

func newAppFromURI(t *testing.T, name, uri string) types.Application {
	t.Helper()
	spec := types.AppSpecV3{
		URI: uri,
	}
	if types.GetMCPServerTransportType(uri) == types.MCPTransportStdio {
		spec.MCP = &types.MCP{
			Command:       "test",
			RunAsHostUser: "test",
		}
	}
	app, err := types.NewAppV3(types.Metadata{Name: name}, spec)
	require.NoError(t, err)
	return app
}
