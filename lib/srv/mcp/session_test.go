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
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

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
				sessionAuditor: auditor,
				accessPoint:    fakeAccessPoint{},
				parentCtx:      ctx,
			})
			require.NoError(t, err)

			t.Run("notification", func(t *testing.T) {
				handler.processClientNotification(ctx, &mcputils.JSONRPCNotification{
					JSONRPC: mcp.JSONRPC_VERSION,
					Method:  "notifications/initialized",
				})
				event := mockEmitter.LastEvent()
				require.NotNil(t, event)
				requestEvent, ok := event.(*apievents.MCPSessionNotification)
				require.True(t, ok)
				require.Equal(t, "notifications/initialized", requestEvent.Message.Method)
			})

			for _, allowedTool := range tt.allowedTools {
				t.Run("allow tools call "+allowedTool, func(t *testing.T) {
					clientReq := requestBuilder.makeToolsCallRequest(allowedTool)
					msg, dir := handler.processClientRequest(ctx, clientReq)
					require.Equal(t, replyToServer, dir)
					require.Equal(t, clientReq, msg)

					event := mockEmitter.LastEvent()
					require.NotNil(t, event)
					requestEvent, ok := event.(*apievents.MCPSessionRequest)
					require.True(t, ok)
					require.True(t, requestEvent.Success)
					require.Equal(t, string(mcp.MethodToolsCall), requestEvent.Message.Method)
					checkParamsHaveNameField(t, requestEvent.Message.Params, allowedTool)
				})
			}

			for _, deniedTool := range tt.deniedTools {
				t.Run("deny tools call "+deniedTool, func(t *testing.T) {
					clientReq := requestBuilder.makeToolsCallRequest(deniedTool)
					msg, dir := handler.processClientRequest(ctx, clientReq)
					require.Equal(t, replyToClient, dir)
					errMsg, ok := msg.(mcp.JSONRPCError)
					require.True(t, ok)
					require.Equal(t, clientReq.ID, errMsg.ID)

					event := mockEmitter.LastEvent()
					require.NotNil(t, event)
					requestEvent, ok := event.(*apievents.MCPSessionRequest)
					require.True(t, ok)
					require.False(t, requestEvent.Success)
					require.Equal(t, string(mcp.MethodToolsCall), requestEvent.Message.Method)
					checkParamsHaveNameField(t, requestEvent.Message.Params, deniedTool)
				})
			}

			t.Run("tools list", func(t *testing.T) {
				mockEmitter.Reset()

				// First make a request so the handler can track the method by ID.
				clientReq := requestBuilder.makeToolsListRequest()
				_, dir := handler.processClientRequest(ctx, clientReq)
				require.Equal(t, replyToServer, dir)

				// tools/list does not trigger audit event.
				require.Nil(t, mockEmitter.LastEvent())

				// make response from server to contain both allowed and denied
				// tools then check only the allowed tools are present after the
				// filtering.
				allTools := append(tt.allowedTools, tt.deniedTools...)
				serverResponse := makeToolsCallResponse(t, clientReq.ID, allTools...)
				processedResponse := handler.processServerResponse(ctx, serverResponse)
				checkToolsListResponse(t, processedResponse, clientReq.ID, tt.allowedTools)
			})
		})
	}
}
