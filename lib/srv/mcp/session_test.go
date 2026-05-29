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
	"slices"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

type captureMessageWriter struct {
	mu   sync.Mutex
	msgs []mcp.JSONRPCMessage
}

func (c *captureMessageWriter) WriteMessage(_ context.Context, msg mcp.JSONRPCMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, msg)
	return nil
}

func (c *captureMessageWriter) messages() []mcp.JSONRPCMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.msgs)
}

func Test_sessionHandler(t *testing.T) {
	tests := []struct {
		name         string
		setupOptions []setupTestContextOptionFunc
		allowedTools []string
		deniedTools  []string
	}{
		{
			name:         "admin",
			setupOptions: []setupTestContextOptionFunc{withAdminRole(t)},
			allowedTools: []string{"search_files", "read_file", "write_file"},
		},
		{
			name:         "readonly",
			setupOptions: []setupTestContextOptionFunc{withProdReadOnlyRole(t)},
			allowedTools: []string{"search_files", "read_file"},
			deniedTools:  []string{"write_file"},
		},
		{
			name: "no-access",
			setupOptions: []setupTestContextOptionFunc{
				withAdminRole(t),
				withDenyToolsRole(t),
			},
			allowedTools: nil,
			deniedTools:  []string{"search_files", "read_file", "write_file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			testCtx := setupTestContext(t, tt.setupOptions...)
			mockEmitter := &eventstest.MockRecorderEmitter{}
			requestBuilder := &requestBuilder{}
			auditor, err := newSessionAuditor(sessionAuditorConfig{
				emitter:    mockEmitter,
				hostID:     "test-host-id",
				sessionCtx: testCtx.SessionCtx,
				preparer:   &libevents.NoOpPreparer{},
			})
			require.NoError(t, err)

			handler, err := newSessionHandler(sessionHandlerConfig{
				SessionCtx:     testCtx.SessionCtx,
				sessionAuth:    &sessionAuth{},
				sessionAuditor: auditor,
				accessPoint:    fakeAccessPoint{},
				parentCtx:      ctx,
			})
			require.NoError(t, err)

			t.Run("notification", func(t *testing.T) {
				msg := &mcputils.JSONRPCNotification{
					JSONRPC: mcp.JSONRPC_VERSION,
					Method:  "notifications/initialized",
				}
				capture := &captureMessageWriter{}
				require.NoError(t, handler.onClientNotification(capture)(ctx, msg))
				event := mockEmitter.LastEvent()
				require.NotNil(t, event)
				requestEvent, ok := event.(*apievents.MCPSessionNotification)
				require.True(t, ok)
				require.Equal(t, "notifications/initialized", requestEvent.Message.Method)
				require.Equal(t, []mcp.JSONRPCMessage{msg}, capture.messages())
			})

			for _, allowedTool := range tt.allowedTools {
				t.Run("allow tools call "+allowedTool, func(t *testing.T) {
					clientReq := requestBuilder.makeToolsCallRequest(allowedTool)
					clientCapture := &captureMessageWriter{}
					serverCapture := &captureMessageWriter{}
					require.NoError(t, handler.onClientRequest(clientCapture, serverCapture)(ctx, clientReq))

					event := mockEmitter.LastEvent()
					require.NotNil(t, event)
					requestEvent, ok := event.(*apievents.MCPSessionRequest)
					require.True(t, ok)
					require.True(t, requestEvent.Success)
					require.Equal(t, mcputils.MethodToolsCall, requestEvent.Message.Method)
					checkParamsHaveNameField(t, requestEvent.Message.Params, allowedTool)

					// Server receives the client's request.
					require.Equal(t, []mcp.JSONRPCMessage{clientReq}, serverCapture.messages())
					require.Empty(t, clientCapture.messages())
				})
			}

			for _, deniedTool := range tt.deniedTools {
				t.Run("deny tools call "+deniedTool, func(t *testing.T) {
					clientReq := requestBuilder.makeToolsCallRequest(deniedTool)
					clientCapture := &captureMessageWriter{}
					serverCapture := &captureMessageWriter{}
					require.NoError(t, handler.onClientRequest(clientCapture, serverCapture)(ctx, clientReq))

					event := mockEmitter.LastEvent()
					require.NotNil(t, event)
					requestEvent, ok := event.(*apievents.MCPSessionRequest)
					require.True(t, ok)
					require.False(t, requestEvent.Success)
					require.Equal(t, mcputils.MethodToolsCall, requestEvent.Message.Method)
					checkParamsHaveNameField(t, requestEvent.Message.Params, deniedTool)

					// Server does not receive the client's request. An error is
					// sent to client.
					require.Empty(t, serverCapture.messages())
					clientMessages := clientCapture.messages()
					require.Len(t, clientMessages, 1)
					require.IsType(t, mcp.JSONRPCError{}, clientMessages[0])
				})
			}

			for _, allowedTool := range tt.allowedTools {
				for _, deniedTool := range tt.deniedTools {
					t.Run(fmt.Sprintf("deny %q tool while allowed tool %q is passed with a non-canonical name param", deniedTool, allowedTool), func(t *testing.T) {
						clientReq := requestBuilder.makeToolsCallRequest(deniedTool)
						clientReq.Params["Name"] = allowedTool
						clientReq.Params["NaMe"] = allowedTool
						clientReq.Params["NAME"] = allowedTool
						clientCapture := &captureMessageWriter{}
						serverCapture := &captureMessageWriter{}
						require.NoError(t, handler.onClientRequest(clientCapture, serverCapture)(ctx, clientReq))

						event := mockEmitter.LastEvent()
						require.NotNil(t, event)
						requestEvent, ok := event.(*apievents.MCPSessionRequest)
						require.True(t, ok)
						require.False(t, requestEvent.Success)
						require.Equal(t, mcputils.MethodToolsCall, requestEvent.Message.Method)
						// Verify there is only 1 param and it's "name" and
						// all other non-lower-case name params are removed
						// from the request.
						require.Len(t, requestEvent.Message.Params.Fields, 1)
						require.Equal(t, deniedTool, requestEvent.Message.Params.Fields["name"].GetStringValue(), 1)

						// Server does not receive the client's request. An error is
						// sent to client.
						require.Empty(t, serverCapture.messages())
						clientMessages := clientCapture.messages()
						require.Len(t, clientMessages, 1)
						require.IsType(t, mcp.JSONRPCError{}, clientMessages[0])
					})
				}
			}

			t.Run("tools list", func(t *testing.T) {
				mockEmitter.Reset()

				// First make a request so the handler can track the method by ID.
				clientReq := requestBuilder.makeToolsListRequest()
				respErr := handler.processClientRequest(ctx, clientReq)
				require.Nil(t, respErr)

				// tools/list does not trigger audit event.
				require.Nil(t, mockEmitter.LastEvent())

				// make response from server to contain both allowed and denied
				// tools then check only the allowed tools are present after the
				// filtering.
				allTools := append(tt.allowedTools, tt.deniedTools...)
				serverResponse := makeToolsCallResponse(t, clientReq.ID, allTools...)
				capture := &captureMessageWriter{}
				require.NoError(t, handler.onServerResponse(capture)(ctx, serverResponse))
				clientMessages := capture.messages()
				require.Len(t, clientMessages, 1)
				checkToolsListResponse(t, clientMessages[0], clientReq.ID, tt.allowedTools)
			})
		})
	}
}

func Test_sessionHandler_StreamingReserializesNonCanonicalIDNotification(t *testing.T) {
	ctx := t.Context()
	testCtx := setupTestContext(t, withAdminRole(t), withDenyToolsRole(t))
	mockEmitter := &eventstest.MockRecorderEmitter{}
	auditor, err := newSessionAuditor(sessionAuditorConfig{
		emitter:    mockEmitter,
		hostID:     "test-host-id",
		sessionCtx: testCtx.SessionCtx,
		preparer:   &libevents.NoOpPreparer{},
	})
	require.NoError(t, err)

	handler, err := newSessionHandler(sessionHandlerConfig{
		SessionCtx:     testCtx.SessionCtx,
		sessionAuth:    &sessionAuth{},
		sessionAuditor: auditor,
		accessPoint:    fakeAccessPoint{},
		parentCtx:      ctx,
	})
	require.NoError(t, err)

	readFromClient, writeFromClient := io.Pipe()
	readFromProxy, writeToServer := io.Pipe()
	t.Cleanup(func() {
		readFromClient.Close()
		writeFromClient.Close()
		readFromProxy.Close()
		writeToServer.Close()
	})

	clientCapture := &captureMessageWriter{}
	serverWriter := mcputils.NewStdioMessageWriter(writeToServer)
	reader, err := mcputils.NewMessageReader(mcputils.MessageReaderConfig{
		Transport:      mcputils.NewStdioReader(readFromClient),
		OnParseError:   mcputils.ReplyParseError(clientCapture),
		OnRequest:      handler.onClientRequest(clientCapture, serverWriter),
		OnNotification: handler.onClientNotification(serverWriter),
	})
	require.NoError(t, err)

	const deniedTool = "denied_tool"
	go reader.Run(ctx)
	_, err = fmt.Fprintf(writeFromClient, `{"jsonrpc":"2.0","ID":99,"method":"tools/call","params":{"name":%q}}`+"\n", deniedTool)
	require.NoError(t, err)

	forwardedMessage, err := bufio.NewReader(readFromProxy).ReadString('\n')
	require.NoError(t, err)
	require.JSONEq(t,
		fmt.Sprintf(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":%q}}`, deniedTool),
		forwardedMessage,
	)
	require.Empty(t, clientCapture.messages())

	event := mockEmitter.LastEvent()
	require.NotNil(t, event)
	notificationEvent, ok := event.(*apievents.MCPSessionNotification)
	require.True(t, ok)
	require.Equal(t, mcputils.MethodToolsCall, notificationEvent.Message.Method)
	checkParamsHaveNameField(t, notificationEvent.Message.Params, deniedTool)
}
