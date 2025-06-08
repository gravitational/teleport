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

package appaccess

import (
	"bytes"
	"context"
	"io"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func testMCP(pack *Pack, t *testing.T) {
	t.Run("DialMCPServer stdio no server found", func(t *testing.T) {
		testMCPDialStdioNoServerFound(t, pack)
	})

	t.Run("DialMCPSererver stdio success", func(t *testing.T) {
		testMCPDialStdio(t, pack)
	})
}

func testMCPDialStdioNoServerFound(t *testing.T, pack *Pack) {
	require.NoError(t, pack.tc.SaveProfile(false))

	_, err := pack.tc.DialMCPServer(context.Background(), "not-found")
	require.Error(t, err)
}

func testMCPDialStdio(t *testing.T, pack *Pack) {
	require.NoError(t, pack.tc.SaveProfile(false))

	serverConn, err := pack.tc.DialMCPServer(context.Background(), "teleport-test-server")
	require.NoError(t, err)

	ctx := context.Background()
	clientTransport := transport.NewIO(serverConn, serverConn, io.NopCloser(bytes.NewReader(nil)))
	stdioClient := mcpclient.NewClient(clientTransport)
	defer stdioClient.Close()
	require.NoError(t, stdioClient.Start(ctx))

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}
	_, err = stdioClient.Initialize(ctx, initReq)
	require.NoError(t, err)

	listTools, err := stdioClient.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, listTools.Tools, 2)
}
