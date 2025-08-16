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
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	libmcp "github.com/gravitational/teleport/lib/srv/mcp"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func testMCP(pack *Pack, t *testing.T) {
	t.Run("DialMCPServer stdio no server found", func(t *testing.T) {
		testMCPDialStdioNoServerFound(t, pack)
	})

	t.Run("DialMCPServer stdio success", func(t *testing.T) {
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

	serverConn, err := pack.tc.DialMCPServer(context.Background(), libmcp.DemoServerName)
	require.NoError(t, err)

	ctx := context.Background()
	stdioClient := mcptest.NewStdioClientFromConn(t, serverConn)

	_, err = mcptest.InitializeClient(ctx, stdioClient)
	require.NoError(t, err)

	listTools, err := stdioClient.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, listTools.Tools, 3)
}
