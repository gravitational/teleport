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
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	initParams := mcp.InitializeParams{
		ProtocolVersion: "2025-06-18",
		ClientInfo: &mcp.Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
	}
	require.NoError(t, WriteRequest(t.Context(), writer, mustMakeIntID(t, 1), MethodInitialize, initParams))

	// Receive response.
	initResp, err := ReadOneResponse(t.Context(), reader)
	require.NoError(t, err)
	require.Equal(t, mustMakeIntID(t, 1), initResp.ID)
	var initResult mcp.InitializeResult
	require.NoError(t, json.Unmarshal(initResp.Result, &initResult))
	require.Equal(t, "test-server", initResult.ServerInfo.Name)
}

func TestEventMarshal(t *testing.T) {
	e := event{
		name: sseEventMessage,
		data: []byte("hello"),
	}
	require.Equal(t, "event: message\ndata: hello\n\n", string(e.marshal()))
}
