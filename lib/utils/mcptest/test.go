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

package mcptest

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/require"
)

// NewServer creates an MCP test server that provides a "hello-server" tool that
// always returns "hello-client".
func NewServer() *mcpserver.MCPServer {
	return NewServerWithVersion("1.0.0")
}

// NewServerWithVersion creates an MCP test server with the specified version
// and provides a "hello-server" tool that always returns "hello-client".
func NewServerWithVersion(version string) *mcpserver.MCPServer {
	server := mcpserver.NewMCPServer("test-server", version)
	server.AddTool(mcp.Tool{
		Name: "hello-server",
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("hello client")},
		}, nil
	})
	return server
}

// MustStartSSETestServer starts an SSE server returns the SSE endpoint.
func MustStartSSETestServer(t *testing.T) string {
	t.Helper()
	sseServer := mcpserver.NewSSEServer(NewServer())
	httpServer := httptest.NewServer(sseServer)
	t.Cleanup(httpServer.Close)
	return httpServer.URL + "/sse"
}

// NewStdioClient creates a new stdio client and ensures the client is closed
// when testing is done.
func NewStdioClient(t *testing.T, input io.Reader, output io.WriteCloser) *mcpclient.Client {
	t.Helper()
	stdioClientTransport := mcpclienttransport.NewIO(input, output, io.NopCloser(bytes.NewReader(nil)))
	require.NoError(t, stdioClientTransport.Start(t.Context()))
	stdioClient := mcpclient.NewClient(stdioClientTransport)
	t.Cleanup(func() {
		stdioClient.Close()
	})
	require.NoError(t, stdioClient.Start(t.Context()))
	return stdioClient
}

func NewStdioClientFromConn(t *testing.T, conn io.ReadWriteCloser) *mcpclient.Client {
	t.Helper()
	return NewStdioClient(t, conn, conn)
}

// InitializeClient starts initialize sequence and returns the initialize result
// from the server.
func InitializeClient(ctx context.Context, client *mcpclient.Client) (*mcp.InitializeResult, error) {
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}
	resp, err := client.Initialize(ctx, initReq)
	return resp, trace.Wrap(err)
}

// MustCallServerTool calls the "hello-server" tool and verifies the result.
func MustCallServerTool(t *testing.T, ctx context.Context, client *mcpclient.Client) {
	t.Helper()
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = "hello-server"
	callToolResult, err := client.CallTool(ctx, callToolRequest)
	require.NoError(t, err)
	require.NotNil(t, callToolResult)
	require.Equal(t, []mcp.Content{
		mcp.NewTextContent("hello client"),
	}, callToolResult.Content)
}
