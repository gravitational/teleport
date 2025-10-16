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
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/teleport/lib/utils/mcptest"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func Test_handleStreamableHTTP(t *testing.T) {
	t.Parallel()

	remoteMCPServer := mcpserver.NewStreamableHTTPServer(mcptest.NewServer())
	remoteMCPHTTPServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path != "/mcp":
			// Unhappy scenario.
			w.WriteHeader(http.StatusNotFound)
		case r.Header.Get("Authorization") != "Bearer app-token-for-ai":
			// Verify rewrite headers.
			w.WriteHeader(http.StatusUnauthorized)
		default:
			remoteMCPServer.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(remoteMCPHTTPServer.Close)

	app, err := types.NewAppV3(types.Metadata{
		Name: "test-http",
	}, types.AppSpecV3{
		URI: fmt.Sprintf("mcp+%s/mcp", remoteMCPHTTPServer.URL),
		Rewrite: &types.Rewrite{
			Headers: []*types.Header{{
				Name:  "Authorization",
				Value: "Bearer {{internal.jwt}}",
			}},
		},
	})
	require.NoError(t, err)

	emitter := eventstest.MockRecorderEmitter{}
	s, err := NewServer(ServerConfig{
		Emitter:       &emitter,
		ParentContext: t.Context(),
		HostID:        "my-host-id",
		AccessPoint:   fakeAccessPoint{},
		CipherSuites:  utils.DefaultCipherSuites(),
		AuthClient:    mockAuthClient{},
	})
	require.NoError(t, err)

	// Run MCP handler behind a listener.
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	listener := listenerutils.NewInMemoryListener()
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				assert.True(t, utils.IsOKNetworkError(err))
				return
			}
			wg.Go(func() {
				defer conn.Close()
				testCtx := setupTestContext(t, withAdminRole(t), withApp(app), withClientConn(conn))
				assert.NoError(t, s.HandleSession(t.Context(), testCtx.SessionCtx))
			})
		}
	}()

	t.Run("success", func(t *testing.T) {
		ctx := t.Context()
		emitter.Reset()
		mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP(
			"http://memory",
			mcpclienttransport.WithHTTPBasicClient(listener.MakeHTTPClient()),
			mcpclienttransport.WithContinuousListening(),
		)
		require.NoError(t, err)
		client := mcpclient.NewClient(mcpClientTransport)
		require.NoError(t, client.Start(ctx))

		// Initialize client, then call a tool. Note that the order can be
		// undeterministic as the listen request is sent from a go-routine by
		// mcp-go client.
		getEventCode := func(e apievents.AuditEvent) string {
			return e.GetCode()
		}
		mcptest.MustInitializeClient(t, client)
		mcptest.MustCallServerTool(t, client)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			require.ElementsMatch(t, []string{
				libevents.MCPSessionStartCode,
				libevents.MCPSessionRequestCode, // "initialize"
				libevents.MCPSessionNotificationCode,
				libevents.MCPSessionListenSSEStreamCode,
				libevents.MCPSessionRequestCode, // "tools/call"
			}, sliceutils.Map(emitter.Events(), getEventCode))
		}, 2*time.Second, time.Millisecond*100, "waiting for events")

		// Close client and wait for end event.
		require.NoError(t, client.Close())
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			require.Equal(t, libevents.MCPSessionEndEvent, emitter.LastEvent().GetType())
		}, 2*time.Second, time.Millisecond*100, "waiting for end event")
	})

	t.Run("endpoint not found", func(t *testing.T) {
		ctx := t.Context()
		emitter.Reset()
		mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP(
			"http://memory/notfound",
			mcpclienttransport.WithHTTPBasicClient(listener.MakeHTTPClient()),
		)
		require.NoError(t, err)

		// Initialize client should fail.
		client := mcpclient.NewClient(mcpClientTransport)
		_, err = mcptest.InitializeClient(ctx, client)
		require.Error(t, err)

		// Close client and verify failure event.
		events := emitter.Events()
		require.Len(t, events, 1)
		lastEvent, ok := events[0].(*apievents.MCPSessionRequest)
		require.True(t, ok)
		require.Equal(t, libevents.MCPSessionRequestEvent, lastEvent.GetType())
		require.Equal(t, libevents.MCPSessionRequestFailureCode, lastEvent.GetCode())
		require.False(t, lastEvent.Success)
		require.Equal(t, "HTTP 404 Not Found", lastEvent.Error)
	})

	t.Run("unsupported method", func(t *testing.T) {
		emitter.Reset()
		httpClient := listener.MakeHTTPClient()
		request, err := http.NewRequestWithContext(t.Context(), http.MethodOptions, "http://localhost/", nil)
		require.NoError(t, err)
		response, err := httpClient.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusMethodNotAllowed, response.StatusCode)
		require.Equal(t, libevents.MCPSessionInvalidHTTPRequest, emitter.LastEvent().GetType())
	})
}

func Test_handleAuthErrHTTP(t *testing.T) {
	s, err := NewServer(ServerConfig{
		Emitter:       &libevents.DiscardEmitter{},
		ParentContext: t.Context(),
		HostID:        "my-host-id",
		AccessPoint:   fakeAccessPoint{},
		CipherSuites:  utils.DefaultCipherSuites(),
		AuthClient:    mockAuthClient{},
	})

	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			assert.True(t, utils.IsOKNetworkError(err))
			return
		}
		defer conn.Close()
		s.handleAuthErrHTTP(t.Context(), conn, trace.AccessDenied("access denied"))
	}()

	mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP(
		fmt.Sprintf("http://%s", listener.Addr().String()),
	)
	require.NoError(t, err)
	client := mcpclient.NewClient(mcpClientTransport)
	_, err = mcptest.InitializeClient(t.Context(), client)
	require.ErrorContains(t, err, "access denied")
}
