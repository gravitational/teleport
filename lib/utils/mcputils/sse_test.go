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
	"strings"
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
		Method:  MethodInitialize,
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

// TestScanEvents tests scanning SSE events.
// Copied from official go-sdk:
// https://github.com/modelcontextprotocol/go-sdk/blob/a225d4dc7ded92f5492651a1bc60499b3be27044/mcp/event_test.go#L16C1-L103C2
func TestScanEvents(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []event
		wantErr string
	}{
		{
			name:  "simple event",
			input: "event: message\nid: 1\ndata: hello\n\n",
			want: []event{
				{name: "message", id: "1", data: []byte("hello")},
			},
		},
		{
			name:  "multiple data lines",
			input: "data: line 1\ndata: line 2\n\n",
			want: []event{
				{data: []byte("line 1\nline 2")},
			},
		},
		{
			name:  "multiple events",
			input: "data: first\n\nevent: second\ndata: second\n\n",
			want: []event{
				{data: []byte("first")},
				{name: "second", data: []byte("second")},
			},
		},
		{
			name:  "no trailing newline",
			input: "data: hello",
			want: []event{
				{data: []byte("hello")},
			},
		},
		{
			name:    "malformed line",
			input:   "invalid line\n\n",
			wantErr: "malformed line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			var got []event
			var err error
			for e, err2 := range scanEvents(r) {
				if err2 != nil {
					err = err2
					break
				}
				got = append(got, e)
			}

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("scanEvents() got nil error, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("scanEvents() error = %q, want containing %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("scanEvents() returned unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("scanEvents() got %d events, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if g, w := got[i].name, tt.want[i].name; g != w {
					t.Errorf("event %d: name = %q, want %q", i, g, w)
				}
				if g, w := got[i].id, tt.want[i].id; g != w {
					t.Errorf("event %d: id = %q, want %q", i, g, w)
				}
				if g, w := string(got[i].data), string(tt.want[i].data); g != w {
					t.Errorf("event %d: data = %q, want %q", i, g, w)
				}
			}
		})
	}
}
