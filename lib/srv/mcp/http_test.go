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
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpclienttransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
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

// Test_Server_HandleSession_reject_req_missing_name makes sure requests with missing canonical
// "name" param are rejected.
func Test_Server_HandleSession_reject_req_missing_name(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const allowedTool = "allowed_tool"
	const allowedToolContext = "allowed_tool_executed_on_upstream"
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
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(allowedToolContext)}}, nil
		},
	)
	upstream.AddTool(
		mcp.Tool{Name: deniedTool},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(deniedToolTextContent)}}, nil
		},
	)

	emitter, mcpClientTransport, proxyURL := newStreamableMCPServer(t, upstream, role)

	sessionID := testMCPInit(t, mcpClientTransport)

	// Verify it works as expected if the canonical lower-case "name" param is provided.
	emitter.Reset()
	resp, err := mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(uuid.NewString()),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"name": deniedTool,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Equal(t, mcp.INVALID_PARAMS, resp.Error.Code)
	respJSON := testJSONString(t, resp)
	require.NotContains(t, respJSON, deniedToolTextContent)
	require.Contains(t, respJSON, "User does not have permissions")

	// Verify that when non-canonical capitalized "Name" param is specified the request is
	// rejected.
	emitter.Reset()
	resp, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(uuid.NewString()),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"Name": deniedTool,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Equal(t, mcp.INVALID_REQUEST, resp.Error.Code)
	respJSON = testJSONString(t, resp)
	require.NotContains(t, respJSON, deniedToolTextContent)
	require.Contains(t, respJSON, testJSONString(t, errInvalidRequestMissingName.Error()))

	// Verify that when an empty "name" param is specified the request is rejected.
	emitter.Reset()
	resp, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(uuid.NewString()),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"name": "",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Equal(t, mcp.INVALID_REQUEST, resp.Error.Code)
	respJSON = testJSONString(t, resp)
	require.NotContains(t, respJSON, deniedToolTextContent)
	require.Contains(t, respJSON, testJSONString(t, errInvalidRequestMissingName.Error()))

	// Verify that correct request is properly unauthorized.
	emitter.Reset()
	resp, err = mcpClientTransport.SendRequest(ctx, mcpclienttransport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(uuid.NewString()),
		Method:  string(mcp.MethodToolsCall),
		Params: map[string]any{
			"name": deniedTool,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Equal(t, mcp.INVALID_PARAMS, resp.Error.Code)
	respJSON = testJSONString(t, resp)
	require.NotContains(t, respJSON, deniedToolTextContent)
	require.Contains(t, respJSON, "User does not have permissions.")

	// Verify that when non-canonical non-lower-case "name" params are specified in the request
	// along side the canonical lower-case one, they are pruned from the request before
	// forwarding.
	emitter.Reset()
	respBytes := testSendRAWRequest(t, http.MethodPost, proxyURL, sessionID, fmt.Sprintf(
		`{"jsonrpc":"2.0","id":"raw-1","method":"tools/call","params":{%q:%q,%q:%q,%q:%q,%q:%q}}`,
		"Name", allowedTool,
		"NaMe", allowedTool,
		"name", deniedTool,
		"NAME", allowedTool,
	))
	respJSON = string(respBytes)
	require.Contains(t, respJSON, strconv.Itoa(mcp.INVALID_PARAMS))
	require.NotContains(t, respJSON, allowedToolContext)
	require.NotContains(t, respJSON, deniedToolTextContent)
	require.Contains(t, respJSON, "User does not have permissions.")
	require.Len(t, emitter.Events(), 1)
	event, ok := emitter.LastEvent().(*apievents.MCPSessionRequest)
	require.True(t, ok)
	// Verify there was 1 param sent and it was the lowercase "name".
	require.Len(t, event.Message.Params.Fields, 1)
	require.Equal(t, deniedTool, event.Message.Params.Fields["name"].GetStringValue())

	emitter.Reset()
	respBytes = testSendRAWRequest(t, http.MethodPost, proxyURL, sessionID, fmt.Sprintf(
		`{"jsonrpc":"2.0","id":"raw-2","method":"tools/call","params":{%q:%q,%q:%q,%q:%q,%q:%q}}`,
		"Name", deniedTool,
		"NaMe", deniedTool,
		"name", allowedTool,
		"NAME", deniedTool,
	))
	respJSON = string(respBytes)
	require.NotContains(t, respJSON, deniedToolTextContent)
	require.Contains(t, respJSON, allowedToolContext)
	require.Len(t, emitter.Events(), 1)
	event, ok = emitter.LastEvent().(*apievents.MCPSessionRequest)
	require.True(t, ok)
	// Verify there was 1 param sent and it was the lowercase "name".
	require.Len(t, event.Message.Params.Fields, 1)
	require.Equal(t, allowedTool, event.Message.Params.Fields["name"].GetStringValue())
}

func Test_Server_HandleSession_remove_non_canonical_ID(t *testing.T) {
	t.Parallel()

	const testTool = "test_tool"

	role, err := types.NewRole("all_mcp_tools_access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: map[string]apiutils.Strings{
				types.Wildcard: {types.Wildcard},
			},
			MCP: &types.MCPPermissions{
				Tools: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	upstream := mcpserver.NewMCPServer("test-server", "1.0.0")
	upstream.AddTool(
		mcp.Tool{Name: testTool},
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("test_tool_context")}}, nil
		},
	)

	upstreamRequestRecorder := &testRequestsRecorder{}
	emitter, mcpClientTransport, proxyURL := newStreamableMCPServerWithUpstreamRequestRecorder(t, upstream, role, upstreamRequestRecorder)

	sessionID := testMCPInit(t, mcpClientTransport)

	// Verify that a request with a non-canonical "ID" field is forwarded as a
	// notification body, so a case-insensitive upstream cannot reinterpret it as
	// an authorized tools/call request.
	emitter.Reset()
	upstreamRequestRecorder.Reset()
	requestToSend := fmt.Sprintf(
		`{"jsonrpc":"2.0","ID":100,"method":"tools/call","params":{"name":%q}}`, testTool,
	)
	expectedForwardedRequest := fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":%q}}`, testTool,
	)
	_ = testSendRAWRequest(t, http.MethodPost, proxyURL, sessionID, requestToSend)
	forwarded := upstreamRequestRecorder.Contents()
	require.Len(t, forwarded, 1, printableContents(forwarded))
	require.JSONEq(t, expectedForwardedRequest, string(forwarded[0]))

	// Same for "iD".
	emitter.Reset()
	upstreamRequestRecorder.Reset()
	requestToSend = fmt.Sprintf(
		`{"jsonrpc":"2.0","iD":100,"method":"tools/call","params":{"name":%q}}`, testTool,
	)
	expectedForwardedRequest = fmt.Sprintf(
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":%q}}`, testTool,
	)
	_ = testSendRAWRequest(t, http.MethodPost, proxyURL, sessionID, requestToSend)
	forwarded = upstreamRequestRecorder.Contents()
	require.Len(t, forwarded, 1)
	require.JSONEq(t, expectedForwardedRequest, string(forwarded[0]))

	// But canonical "id" is forwarded.
	emitter.Reset()
	upstreamRequestRecorder.Reset()
	requestToSend = fmt.Sprintf(
		`{"jsonrpc":"2.0","id":100,"method":"tools/call","params":{"name":%q}}`, testTool,
	)
	expectedForwardedRequest = requestToSend
	_ = testSendRAWRequest(t, http.MethodPost, proxyURL, sessionID, requestToSend)
	forwarded = upstreamRequestRecorder.Contents()
	require.Len(t, forwarded, 1)
	require.JSONEq(t, expectedForwardedRequest, string(forwarded[0]))
}

type printableContents [][]byte

func (c printableContents) String() string {
	var b strings.Builder
	for _, bs := range c {
		b.Write(bs)
		b.WriteString(",")
	}
	return b.String()[:max(0, b.Len()-1)]
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
