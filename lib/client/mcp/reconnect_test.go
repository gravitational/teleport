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
	"sync/atomic"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/mcptest"
	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestProxyStdioConnWithAutoReconnect(t *testing.T) {
	ctx := t.Context()

	var serverStdioSource atomic.Value
	prepServerWithVersion := func(version string) {
		testServerV1 := mcptest.NewServerWithVersion(version)
		testServerSource, testServerDest := mustMakeSocketPair(t)
		serverStdioSource.Store(testServerSource)
		go func() {
			mcpserver.NewStdioServer(testServerV1).Listen(t.Context(), testServerDest, testServerDest)
		}()
	}
	prepServerWithVersion("1.0.0")

	clientStdioSource, clientStdioDest := mustMakeSocketPair(t)
	stdioClient := mcptest.NewStdioClientFromConn(t, clientStdioSource)
	proxyError := make(chan error, 1)
	serverConnClosed := make(chan struct{}, 1)

	// Start proxy.
	go func() {
		proxyError <- ProxyStdioConnWithAutoReconnect(ctx, ProxyStdioConnWithAutoReconnectConfig{
			ClientStdio: clientStdioDest,
			MakeReconnectUserMessage: func(err error) string {
				return err.Error()
			},
			DialServer: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return serverStdioSource.Load().(io.ReadWriteCloser), nil
			},
			onServerConnClosed: func() {
				serverConnClosed <- struct{}{}
			},
		})
	}()

	// Initialize.
	_, err := mcptest.InitializeClient(ctx, stdioClient)
	require.NoError(t, err)

	// Call tool success.
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = "hello-server"
	_, err = stdioClient.CallTool(ctx, callToolRequest)
	require.NoError(t, err)

	// Let's kill the server, CallTool should fail.
	serverStdioSource.Load().(io.ReadWriteCloser).Close()
	select {
	case <-serverConnClosed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server connection to close")
	}
	_, err = stdioClient.CallTool(ctx, callToolRequest)
	require.ErrorContains(t, err, "use of closed network connection")

	// Let it try again with a successful reconnect.
	prepServerWithVersion("1.0.0")
	_, err = stdioClient.CallTool(ctx, callToolRequest)
	require.NoError(t, err)

	// Let's kill the server again, and prepare a different version.
	serverStdioSource.Load().(io.ReadWriteCloser).Close()
	select {
	case <-serverConnClosed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for server connection to close")
	}
	prepServerWithVersion("2.0.0")
	_, err = stdioClient.CallTool(ctx, callToolRequest)
	require.ErrorContains(t, err, "server info has changed")

	// Cleanup.
	clientStdioSource.Close()
	select {
	case proxyErr := <-proxyError:
		require.NoError(t, proxyErr)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for proxy connection")
	}
}

func mustMakeSocketPair(t *testing.T) (io.ReadWriteCloser, io.ReadWriteCloser) {
	t.Helper()
	source, dest, err := uds.NewSocketpair(uds.SocketTypeStream)
	require.NoError(t, err)
	t.Cleanup(func() {
		source.Close()
		dest.Close()
	})
	return source, dest
}
