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
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/defaults"
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
		case r.URL.Path == "/.well-known/oauth-authorization-server":
			w.Write([]byte("{}"))
			w.WriteHeader(http.StatusOK)
		case r.URL.Path != "/mcp":
			// Unhappy scenario.
			w.WriteHeader(http.StatusNotFound)
		case r.Header.Get("Authorization") != "Bearer app-token-for-ai-by-oidc_idp":
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
				Value: "Bearer {{internal.id_token}}",
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
		AuthClient:    &mockAuthClient{},
	})
	require.NoError(t, err)

	// Run MCP handler behind a listener.
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	listener := listenerutils.NewInMemoryListener()
	t.Cleanup(func() { _ = listener.Close() })
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
				testCtx.sessionID = "test-session-id" // use same session id
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
		checkSessionStartAndInitializeEvents(t, emitter.Events(),
			checkSessionStartWithServerInfo("test-server", "1.0.0"),
			checkSessionStartHasExternalSessionID(),
			checkSessionStartWithEgressAuthType(egressAuthTypeAppIDToken),
		)

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
			mcpclienttransport.WithHTTPHeaders(map[string]string{
				"test-header": "test-value",
			}),
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
		require.NotEmpty(t, lastEvent.Headers)
		require.Equal(t, "test-value", http.Header(lastEvent.Headers).Get("test-header"))
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

	t.Run("passthrough well-known", func(t *testing.T) {
		emitter.Reset()
		httpClient := listener.MakeHTTPClient()
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/.well-known/oauth-authorization-server", nil)
		require.NoError(t, err)
		response, err := httpClient.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		// No audit events.
		events := emitter.Events()
		require.Empty(t, events)
	})
}

// Test_Server_HandleSession_request_sanitization verifies the forwarded request is stripped of
// non-canonical fields (e.g. uppercase) which may confuse the upstream server.
func Test_Server_HandleSession_request_sanitization(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const allowedTool = "allowed_tool"
	const allowedToolTextContent = "allowed_tool_executed_on_upstream"
	const deniedTool = "denied_tool"
	const deniedToolTextContent = "DENIED_TOOL_EXECUTED_ON_UPSTREAM"

	role, err := types.NewRole("allowed_tool_access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: map[string]apiutils.Strings{
				types.Wildcard: {types.Wildcard},
			},
			MCP: &types.MCPPermissions{
				Tools: []string{allowedTool},
			},
		},
	})
	require.NoError(t, err)

	upstream := mcpserver.NewMCPServer("test-server", "1.0.0")
	upstream.AddTool(
		mcp.Tool{Name: allowedTool},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(allowedToolTextContent)}}, nil
		},
	)
	upstream.AddTool(
		mcp.Tool{Name: deniedTool},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(deniedToolTextContent)}}, nil
		},
	)

	recorder, mcpClientTransport, proxyURL := newStreamableMCPServer(t, upstream, role)

	// Initialize message has to be sent first. It isn't a part of the test.
	_, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(1),
		Method:  string(mcp.MethodInitialize),
		Params: map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"clientInfo": map[string]any{
				"name":    "test-client-transport",
				"version": "1.0.0",
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, mcpClientTransport.GetSessionId())

	// Verify everything works as expected if the canonical lower-case "name" param is
	// provided and allowed tool is executed.
	recorder.Reset()
	resp, err := mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(2),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"name": allowedTool,
		},
	})
	require.NoError(t, err)
	require.Contains(t, string(resp.Result), allowedToolTextContent)
	// Verify the request was forwarded.
	require.Len(t, recorder.requests, 1)

	// Verify everything works as expected if the canonical lower-case "name" param is
	// provided and denied tool is rejected.
	recorder.Reset()
	resp, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(2),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"name": deniedTool,
		},
	})
	require.NoError(t, err)
	requireDeniedToolResponse(t, testJSON(t, resp))
	// Verify the request was not forwarded.
	require.Empty(t, recorder.requests)

	// Verify that when an empty "name" param is specified the request is rejected as invalid.
	recorder.Reset()
	resp, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(4),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"name": "",
		},
	})
	require.NoError(t, err)
	requireToolNameMissingResponse(t, testJSON(t, resp))
	// Verify the request was not forwarded.
	require.Empty(t, recorder.requests)

	// Verify that when non-canonical capitalized "Name" param is specified and send through
	// streamable transport the request is correctly rejected.
	recorder.Reset()
	resp, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(3),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"Name": allowedTool,
			"name": deniedTool,
			"NAME": allowedTool,
		},
	})
	require.NoError(t, err)
	requireDeniedToolResponse(t, testJSON(t, resp))
	// Verify the request was not forwarded.
	require.Empty(t, recorder.requests)

	// Verify that when non-canonical capitalized "Name" param is specified and send through
	// HTTP transport the request is correctly rejected.
	recorder.Reset()
	proxyRequest := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{%q:%q,%q:%q,%q:%q,%q:%q}}`,
		"Name", allowedTool,
		"NaMe", allowedTool,
		"name", deniedTool,
		"NAME", allowedTool,
	)
	rawResp := testSendRAWRequest(t, http.MethodPost, proxyURL, mcpClientTransport.GetSessionId(), proxyRequest)
	requireDeniedToolResponse(t, rawResp)
	// Verify the request was not forwarded.
	require.Empty(t, recorder.requests)

	// Verify that when non-canonical capitalized "Params" field is specified and send through
	// HTTP transport the request is correctly rejected.
	recorder.Reset()
	proxyRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","Params":{%q:%q}}`,
		"name", deniedTool,
	)
	rawResp = testSendRAWRequest(t, http.MethodPost, proxyURL, mcpClientTransport.GetSessionId(), proxyRequest)
	requireToolNameMissingResponse(t, rawResp)
	// Verify the request was not forwarded.
	require.Empty(t, recorder.requests)

	// Verify that when non-canonical capitalized "Method" field is specified and send through
	// HTTP transport the request is sanitized before forwarding, resulting in a ping message.
	recorder.Reset()
	proxyRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","id":9,"method":"ping","Method":"tools/call","params":{%q:%q}}`,
		"name", deniedTool,
	)
	expectedForwardedRequest := fmt.Sprintf(
		`{"jsonrpc":"2.0","id":9,"method":"ping","params":{%q:%q}}`,
		"name", deniedTool,
	)
	rawResp = testSendRAWRequest(t, http.MethodPost, proxyURL, mcpClientTransport.GetSessionId(), proxyRequest)
	require.JSONEq(t, `{"jsonrpc":"2.0","id":9,"result":{}}`, string(rawResp))
	// Verify the request was forwarded as ping.
	require.Len(t, recorder.requests, 1)
	forwardedRequest := string(testGetRequestPayload(t, recorder.requests[0]))
	require.JSONEq(t, expectedForwardedRequest, forwardedRequest)
	require.Contains(t, proxyRequest, `"Method"`)
	require.NotContains(t, forwardedRequest, `"Method"`)

	// Verify that when non-canonical capitalized "ID" field is specified and send through HTTP
	// transport the request is sanitized before forwarding, resulting in a Notification
	// message.
	recorder.Reset()
	proxyRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","ID":8,"method":"tools/call","params":{%q:%q}}`, "name", deniedTool,
	)
	expectedForwardedRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"tools/call","params":{%q:%q}}`, "name", deniedTool,
	)
	rawResp = testSendRAWRequest(t, http.MethodPost, proxyURL, mcpClientTransport.GetSessionId(), proxyRequest)
	require.Empty(t, rawResp) // MCP Notification response.
	// Verify the forwarded request's confusing fields are pruned.
	require.Len(t, recorder.requests, 1)
	forwardedRequest = string(testGetRequestPayload(t, recorder.requests[0]))
	require.JSONEq(t, expectedForwardedRequest, forwardedRequest)
	require.Contains(t, proxyRequest, `"ID":8`)
	require.NotContains(t, forwardedRequest, `"ID":8`)

	// Verify multiple non-canonical fields are stripped from a message send over HTTP before
	// forwarding.
	recorder.Reset()
	proxyRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","ID":100,"id":9,"Method":"ping","method":"tools/call","Params":{"a":"b"},"params":{%q:%q,%q:%q,%q:%q,%q:%q}}`,
		"Name", deniedTool,
		"NaMe", deniedTool,
		"name", allowedTool,
		"NAME", deniedTool,
	)
	expectedForwardedRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{%q:%q}}`,
		"name", allowedTool,
	)
	rawResp = testSendRAWRequest(t, http.MethodPost, proxyURL, mcpClientTransport.GetSessionId(), proxyRequest)
	require.Contains(t, string(rawResp), allowedToolTextContent)
	// Verify the forwarded request's confusing fields are pruned.
	require.Len(t, recorder.requests, 1)
	forwardedRequest = string(testGetRequestPayload(t, recorder.requests[0]))
	require.JSONEq(t, expectedForwardedRequest, forwardedRequest)
	require.Contains(t, proxyRequest, `"ID":`)
	require.NotContains(t, forwardedRequest, `"ID"`)
	require.Contains(t, proxyRequest, `"Method":`)
	require.NotContains(t, forwardedRequest, `"Method"`)
	require.Contains(t, proxyRequest, `"Params":`)
	require.NotContains(t, forwardedRequest, `"Params"`)
}

func Test_handleAuthErrHTTP(t *testing.T) {
	t.Run("initialize", func(t *testing.T) {
		t.Parallel()
		s, err := NewServer(ServerConfig{
			Emitter:       &libevents.DiscardEmitter{},
			ParentContext: t.Context(),
			HostID:        "my-host-id",
			AccessPoint:   fakeAccessPoint{},
			CipherSuites:  utils.DefaultCipherSuites(),
			AuthClient:    &mockAuthClient{},
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

		mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP("http://" + listener.Addr().String())
		require.NoError(t, err)

		client := mcpclient.NewClient(mcpClientTransport)
		_, err = mcptest.InitializeClient(t.Context(), client)
		// TODO(greedy52) handle errors in a manner that returns access denied
		// meesages to clients instead of ErrLegacySSEServer.
		// require.ErrorContains(t, err, "access denied")
		require.ErrorIs(t, err, mcpclienttransport.ErrLegacySSEServer)
	})

	t.Run("notification", func(t *testing.T) {
		t.Parallel()
		s, err := NewServer(ServerConfig{
			Emitter:       &libevents.DiscardEmitter{},
			ParentContext: t.Context(),
			HostID:        "my-host-id",
			AccessPoint:   fakeAccessPoint{},
			CipherSuites:  utils.DefaultCipherSuites(),
			AuthClient:    &mockAuthClient{},
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

		mcpClientTransport, err := mcpclienttransport.NewStreamableHTTP("http://" + listener.Addr().String())
		require.NoError(t, err)

		resp, err := mcpClientTransport.SendRequest(t.Context(), mcpclienttransport.JSONRPCRequest{
			JSONRPC: mcp.JSONRPC_VERSION,
			Method:  "notifications/test",
		})
		require.NoError(t, err)
		require.NotNil(t, resp.Error)
		require.Equal(t, "access denied", resp.Error.Message)
	})
}

func Test_Server_serveHTTPCon_closes_idle_connections(t *testing.T) {
	t.Parallel()

	acceptedHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	t.Run("Verify ReadHeaderTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			clientConn, serverConn := net.Pipe()

			go new(Server).serveHTTPConn(ctx, serverConn, acceptedHandler)

			// Idling without sending anything.
			readConnErrCh := make(chan error)
			go func() {
				_, err := clientConn.Read(make([]byte, 1))
				readConnErrCh <- err
			}()

			time.Sleep(defaults.ReadHeadersTimeout + 1*time.Nanosecond)

			// Check closed.
			select {
			case err := <-readConnErrCh:
				require.ErrorIs(t, err, io.EOF)
			default:
				t.Error("Expected the connection to be closed by idle timeout")
			}
		})
	})

	t.Run("Verify IdleTimeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			clientConn, serverConn := net.Pipe()

			go new(Server).serveHTTPConn(ctx, serverConn, acceptedHandler)

			// Establish the connection by sending a request.
			req, err := http.NewRequest(http.MethodGet, "https://mcp.test", nil)
			require.NoError(t, err)
			err = req.Write(clientConn)
			require.NoError(t, err)

			resp, err := http.ReadResponse(bufio.NewReader(clientConn), req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusAccepted, resp.StatusCode)

			// Idling now.
			readConnErrCh := make(chan error)
			go func() {
				_, err := clientConn.Read(make([]byte, 1))
				readConnErrCh <- err
			}()

			time.Sleep(apidefaults.DefaultIdleTimeout + 1*time.Nanosecond)

			// Check closed.
			select {
			case err := <-readConnErrCh:
				require.ErrorIs(t, err, io.EOF)
			default:
				t.Error("Expected the connection to be closed by idle timeout")
			}
		})
	})
}
