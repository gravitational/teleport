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

			t.Run("tools list", func(t *testing.T) {
				mockEmitter.Reset()

				// First make a request so the handler can track the method by ID.
				clientReq := requestBuilder.makeToolsListRequest()
				_, authErr := handler.processClientRequest(ctx, clientReq)
				require.NoError(t, authErr)

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
