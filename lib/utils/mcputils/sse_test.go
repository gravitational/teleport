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

package mcputils

import (
	"net/http"
	"net/url"
	"testing"

	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func TestConnectSSEServer(t *testing.T) {
	testServerSSEEndpoint := mcptest.MustStartSSETestServer(t)
	testServerURL, err := url.Parse(testServerSSEEndpoint)
	require.NoError(t, err)

	reader, writer, err := ConnectSSEServer(t.Context(), testServerURL, http.DefaultTransport)
	require.NoError(t, err)
	defer reader.Close()

	// Check endpoint info.
	require.Contains(t, writer.GetEndpointURL(), "/message")
	require.NotEmpty(t, writer.GetSessionID())

	// Send initialize.
	initReq := mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(int64(1)),
		Method:  string(mcp.MethodInitialize),
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	}
	require.NoError(t, writer.WriteMessage(t.Context(), initReq))

	// Receive response.
	initResp, err := ReadOneResponse(t.Context(), reader)
	require.NoError(t, err)
	require.Equal(t, initReq.ID.String(), initResp.ID.String())
	initResult, err := initResp.GetInitializeResult()
	require.NoError(t, err)
	require.Equal(t, "test-server", initResult.ServerInfo.Name)
}

func TestEventMarshal(t *testing.T) {
	e := event{
		name: sseEventMessage,
		data: []byte("hello"),
	}
	require.Equal(t, "event: message\ndata: hello\n\n", string(e.marshal()))
}
