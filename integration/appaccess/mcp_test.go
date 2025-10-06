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
	"crypto/tls"
	"fmt"
	"net/http"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
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

	t.Run("DialMCPServer stdio to sse success", func(t *testing.T) {
		testMCPDialStdioToSSE(t, pack, "test-sse")
	})

	t.Run("proxy streamable HTTP requests with TLS cert", func(t *testing.T) {
		testMCPProxyStreamableHTTP(t, pack, "test-http")
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

	ctx := t.Context()
	stdioClient := mcptest.NewStdioClientFromConn(t, serverConn)

	_, err = mcptest.InitializeClient(ctx, stdioClient)
	require.NoError(t, err)

	listTools, err := stdioClient.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.Len(t, listTools.Tools, 3)
}

func testMCPDialStdioToSSE(t *testing.T, pack *Pack, appName string) {
	require.NoError(t, pack.tc.SaveProfile(false))

	serverConn, err := pack.tc.DialMCPServer(context.Background(), appName)
	require.NoError(t, err)

	ctx := t.Context()
	stdioClient := mcptest.NewStdioClientFromConn(t, serverConn)

	_, err = mcptest.InitializeClient(ctx, stdioClient)
	require.NoError(t, err)

	mcptest.MustCallServerTool(t, ctx, stdioClient)
}

func testMCPProxyStreamableHTTP(t *testing.T, pack *Pack, appName string) {
	require.NoError(t, pack.tc.SaveProfile(false))

	// Find the MCP server.
	filter := pack.tc.ResourceFilter(types.KindAppServer)
	filter.PredicateExpression = fmt.Sprintf(`name == "%s"`, appName)
	apps, err := pack.tc.ListApps(t.Context(), filter)
	require.NoError(t, err)
	require.Len(t, apps, 1)

	// Issue a TLS cert with app route.
	keyRing, err := pack.tc.IssueUserCertsWithMFA(t.Context(), client.ReissueParams{
		RouteToCluster: pack.rootCluster.Secrets.SiteName,
		RouteToApp: proto.RouteToApp{
			ClusterName: pack.rootCluster.Secrets.SiteName,
			Name:        apps[0].GetName(),
			PublicAddr:  apps[0].GetPublicAddr(),
		},
	})
	require.NoError(t, err)
	appCert, err := keyRing.AppTLSCert(appName)
	require.NoError(t, err)

	// Create an MCP client with app cert.
	ctx := t.Context()
	mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP(
		"https://"+pack.rootCluster.Web,
		mcpclienttransport.WithHTTPBasicClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:       []tls.Certificate{appCert},
					InsecureSkipVerify: true,
				},
			},
		}),
	)
	require.NoError(t, err)
	client := mcpclient.NewClient(mcpClientTransport)
	require.NoError(t, client.Start(ctx))
	defer client.Close()

	// Initialize client and call a tool.
	_, err = mcptest.InitializeClient(ctx, client)
	require.NoError(t, err)
	mcptest.MustCallServerTool(t, ctx, client)
}
