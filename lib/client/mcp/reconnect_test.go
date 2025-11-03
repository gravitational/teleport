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
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func TestProxyStdioConn_autoReconnect(t *testing.T) {
	ctx := t.Context()
	app := newAppFromURI(t, "some-mcp", "mcp+stdio://")

	var serverStdioSource atomic.Value
	prepServerWithVersion := func(version string) {
		testServerV1 := mcptest.NewServerWithVersion(version)
		testServerSource, testServerDest := mustMakeConnPair(t)
		serverStdioSource.Store(testServerSource)
		go func() {
			mcpserver.NewStdioServer(testServerV1).Listen(t.Context(), testServerDest, testServerDest)
		}()
	}
	prepServerWithVersion("1.0.0")

	clientStdioSource, clientStdioDest := mustMakeConnPair(t)
	stdioClient := mcptest.NewStdioClientFromConn(t, clientStdioSource)
	proxyError := make(chan error, 1)
	serverConnClosed := make(chan struct{}, 1)

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
			onServerConnClosed: func() {
				serverConnClosed <- struct{}{}
			},
		})
	}()

	// Initialize.
	mcptest.MustInitializeClient(t, stdioClient)

	// Call tool success.
	mcptest.MustCallServerTool(t, stdioClient)

	// Let's kill the server, CallTool should fail.
	serverStdioSource.Load().(io.ReadWriteCloser).Close()
	select {
	case <-serverConnClosed:
	case <-time.After(time.Second * 5):
		t.Fatal("timed out waiting for server connection to close")
	}
	_, err := mcptest.CallServerTool(ctx, stdioClient)
	require.ErrorContains(t, err, "on closed pipe")

	// Let it try again with a successful reconnect.
	prepServerWithVersion("1.0.0")
	mcptest.MustCallServerTool(t, stdioClient)

	// Let's kill the server again, and prepare a different version.
	serverStdioSource.Load().(io.ReadWriteCloser).Close()
	select {
	case <-serverConnClosed:
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for server connection to close")
	}
	prepServerWithVersion("2.0.0")
	_, err = mcptest.CallServerTool(ctx, stdioClient)
	require.ErrorContains(t, err, "server info has changed")

	// Cleanup.
	clientStdioSource.Close()
	select {
	case proxyErr := <-proxyError:
		require.NoError(t, proxyErr)
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for proxy to complete")
	}
}

func TestProxyStdioConn_http(t *testing.T) {
	ctx := t.Context()
	app := newAppFromURI(t, "some-mcp", "mcp+http://127.0.0.1:8888/mcp")

	// Remote MCP server.
	mcpServer := mcptest.NewServer()
	listener := listenerutils.NewInMemoryListener()
	t.Cleanup(func() { listener.Close() })
	go http.Serve(listener, mcpserver.NewStreamableHTTPServer(mcpServer))

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
	select {
	case proxyErr := <-proxyError:
		require.NoError(t, proxyErr)
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for proxy to complete")
	}
}

func TestProxyStdioConn_autoReconnectDisabled(t *testing.T) {
	ctx := t.Context()
	app := newAppFromURI(t, "some-mcp", "mcp+stdio://")

	var mcpServerConnCount atomic.Uint32
	var mcpServerConn atomic.Value
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
			go mcpserver.NewStdioServer(mcptest.NewServer()).Listen(t.Context(), conn, conn)
		}
	}()

	// Start proxy.
	clientStdioSource, clientStdioDest := mustMakeConnPair(t)
	serverConnClosed := make(chan struct{}, 1)
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
			onServerConnClosed: func() {
				serverConnClosed <- struct{}{}
			},
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
	select {
	case <-serverConnClosed:
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for server connection to close")
	}

	// Check proxy has ended.
	select {
	case proxyErr := <-proxyError:
		require.NoError(t, proxyErr)
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for proxy to complete")
	}

	// New request should fail and no retry is performed.
	_, err := mcptest.CallServerTool(t.Context(), stdioClient)
	require.ErrorContains(t, err, "on closed pipe")
	require.Equal(t, uint32(1), mcpServerConnCount.Load())
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
