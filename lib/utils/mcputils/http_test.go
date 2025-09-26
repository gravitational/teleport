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
	"context"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func TestReplaceHTTPResponse(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Set up a server.
	mcpServer := mcptest.NewServer()
	httpServer := mcpserver.NewTestStreamableHTTPServer(mcpServer)
	t.Cleanup(httpServer.Close)

	// Set up a client with custom transport which calls "ReplaceHTTPResponse".
	httpClientTransport := newTestReplaceHTTPResponseTransport()
	mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP(
		httpServer.URL+"/mcp",
		mcpclienttransport.WithHTTPBasicClient(
			&http.Client{Transport: httpClientTransport},
		),
		mcpclienttransport.WithContinuousListening(),
	)
	require.NoError(t, err)
	client := mcpclient.NewClient(mcpClientTransport)
	require.NoError(t, client.Start(ctx))

	// Initialize client and call a tool.
	_, err = mcptest.InitializeClient(ctx, client)
	require.NoError(t, err)
	mcptest.MustCallServerTool(t, ctx, client)
	assert.Equal(t, uint32(2), httpClientTransport.countMCPResponse.Load())

	// Send notifications from server. Notifications will be sent through SSE.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.Greater(collect, httpClientTransport.getCountMethods()["GET"], 0)
	}, 2*time.Second, 100*time.Millisecond, "client SSE connected")
	mcpServer.SendNotificationToAllClients("notifications/test", nil)
	mcpServer.SendNotificationToAllClients("notifications/test", nil)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.Equal(collect, uint32(2), httpClientTransport.countMCPNotification.Load())
	}, 2*time.Second, 100*time.Millisecond, "expected to receive notification")

	// Close client and count the requests.
	require.NoError(t, client.Close())
	require.Equal(t, map[string]int{
		"GET":    1, // For listening on SSE events.
		"POST":   3, // "initialize", "notifications/initialize", and "tools/call".
		"DELETE": 1, // Close session.
	}, httpClientTransport.getCountMethods())
}

type testReplaceHTTPResponseTransport struct {
	countMethods         map[string]int
	countMethodsMu       sync.Mutex
	countMCPResponse     atomic.Uint32
	countMCPNotification atomic.Uint32
}

func newTestReplaceHTTPResponseTransport() *testReplaceHTTPResponseTransport {
	return &testReplaceHTTPResponseTransport{
		countMethods: make(map[string]int),
	}
}

func (t *testReplaceHTTPResponseTransport) getCountMethods() map[string]int {
	t.countMethodsMu.Lock()
	defer t.countMethodsMu.Unlock()
	return maps.Clone(t.countMethods)
}

func (t *testReplaceHTTPResponseTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.countMethodsMu.Lock()
	t.countMethods[r.Method]++
	t.countMethodsMu.Unlock()

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := ReplaceHTTPResponse(r.Context(), resp, t); err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (t *testReplaceHTTPResponseTransport) ProcessResponse(_ context.Context, response *JSONRPCResponse) mcp.JSONRPCMessage {
	t.countMCPResponse.Add(1)
	return response
}

func (t *testReplaceHTTPResponseTransport) ProcessNotification(_ context.Context, notification *JSONRPCNotification) mcp.JSONRPCMessage {
	t.countMCPNotification.Add(1)
	return notification
}
