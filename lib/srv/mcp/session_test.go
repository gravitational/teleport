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
	"slices"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
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

func BenchmarkAccessToTool(b *testing.B) {
	ctx := b.Context()

	tools := []string{
		"get_file_contents",
		"get_file_blame",
		"github_push_files",
		"list_directory",
		"get_repository",
		"list_repositories",
		"create_issue",
		"update_issue",
		"get_issue",
		"list_issues",
		"add_issue_comment",
		"get_issue_comments",
		"assign_copilot_to_issue",
		"get_copilot_job_status",
		"create_pull_request",
		"list_pull_requests",
		"get_pull_request",
		"pull_request_read",
		"merge_pull_request",
		"get_pull_request_files",
		"get_pull_request_status",
		"update_pull_request_branch",
		"get_pull_request_comments",
		"get_pull_request_reviews",
		"create_pull_request_review",
		"create_pull_request_with_copilot",
		"search_code",
		"search_repositories",
		"search_issues",
		"search_pull_requests",
		"search_users",
		"search_orgs",
		"actions_list",
		"actions_get",
		"get_job_logs",
		"actions_run_trigger",
		"get_workflow",
		"get_workflow_run",
		"get_workflow_run_usage",
		"get_workflow_run_logs_url",
		"download_workflow_run_artifact",
		"get_workflow_job",
		"projects_list",
		"projects_get",
		"projects_write",
		"code_security",
		"secret_protection",
		"dependabot",
		"discussions",
		"labels",
		"gists",
		"notifications",
	}

	complexMCPAccessRole, err := types.NewRole("complex_mcp_access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: map[string]apiutils.Strings{
				"env": {"prod"},
			},
			MCP: &types.MCPPermissions{
				Tools: []string{
					`^(get|read|list|search).*$`,
					`^.*_read$`,

					// 1. Matches file retrieval and metadata tools: get_file_contents, get_file_blame, get_repository, list_repositories
					`^get_(file_(contents|blame)|repositor(y|ies))$`,

					// 2. Deep workflow inspector matching: get_workflow, get_workflow_job, get_workflow_run, get_workflow_run_usage, get_workflow_run_logs_url
					`^get_workflow(_job|_run(_(usage|logs_url))?)?$`,

					// 3. Matches basic issue lifecycle tools: create_issue, update_issue, get_issue, list_issues, add_issue_comment, get_issue_comments
					`^(create|update|get|list|add)_issue(_comments?)?$`,

					// 4. Captures PR read states: pull_request_read, get_pull_request_files, get_pull_request_status, get_pull_request_comments, get_pull_request_reviews
					`^(pull_request_read|[a-z]+_pull_request_(files|status|comments|reviews))$`,

					// 5. Explicitly matches all 6 standalone global search utilities (code, repositories, issues, etc.)
					`^search_(code|repositories|issues|pull_requests|users|orgs)$`,

					// 6. Focuses specifically on automation log extraction: actions_list, actions_get, actions_run_trigger, get_job_logs
					`^(actions_(list|get|run_trigger)|get_job_logs)$`,

					// 7. Isolates administrative Project Kanban actions: projects_list, projects_get, projects_write
					`^projects_(list|get|write)$`,

					// 8. Filters down to security compliance features: code_security, secret_protection, dependabot
					`^(code_security|secret_protection|dependabot)$`,

					// 9. Captures PR generation and merge events: create_pull_request, merge_pull_request, create_pull_request_with_copilot, create_pull_request_review
					`^(create|merge)_pull_request(_(with_copilot|review))?$,`,

					// 10. Catches single-word metadata tools: labels, gists, notifications, discussions
					`^(labels|gists|notifications|discussions)$`,

					// 11. Matches status checks ending specifically in "_status": get_copilot_job_status, get_pull_request_status
					`^get_(copilot_job|pull_request)_status$`,

					// 12. Generically identifies any AI-assisted capability containing the "copilot" token
					`^[a-z]+_copilot_[a-z_]+$`,

					// 13. Targets artifact or flat file retrieval: list_directory, download_workflow_run_artifact
					`^(list_directory|download_workflow_run_artifact)$`,

					// 14. Groups high-privilege structural state changes: github_push_files, projects_write, merge_pull_request
					`^(github_push_files|projects_write|merge_pull_request)$`,

					// 15. Length-bounded match for tools containing exactly 3 snake_case words (e.g., assign_copilot_to_issue)
					`^[a-z]{3,7}_[a-z]{4,12}_[a-z]{4,12}$`,

					// 16. Flags any execution tracker capturing operational logs or triggers containing "run"
					`^[a-z_]+_run(_[a-z_]+)*$`,

					// 17. Isolates collection/list targets processing multiple entries: list_issues, search_pull_requests, etc.
					`^[a-z]+_(issue|pull_request)s?$`,

					// 18. Matches highly specialized, deeply nested getters containing 3 or 4 snake_case segments
					`^get_([a-z]+_){2,3}[a-z]+$`,

					// 19. Matches simplistic, shallow tool naming conventions containing exactly one underscore (e.g., code_security)
					`^[^_]+_[^_]+$`,

					// 20. Enforces rigid structural validation matching standardized API action prefixes
					`^(get|list|create|update|merge|add|download|search)_[a-z_]+$`,
				},
			},
		},
		Deny: types.RoleConditions{

			AppLabels: map[string]apiutils.Strings{
				"env": {"prod"},
			},
			MCP: &types.MCPPermissions{
				Tools: []string{
					// 1. Fails because GitHub MCP tools are strictly snake_case; this targets PascalCase/CamelCase.
					`^(Get|List|Create|Update)[A-Z][a-z]+$`,

					// 2. Fails because there are no digits or hyphens used anywhere in the official tool names.
					`^.*-[0-9]+$`,

					// 3. Fails because core Git plumbing/porcelain commands are not exposed as direct tools.
					`^git_(clone|commit|checkout|stash|rebase|cherry_pick)$`,

					// 4. Fails because the MCP server does not implement any destructive "delete_" endpoints.
					`^delete_(branch|tag|repository|issue_comment)$`,

					// 5. Fails because Webhook primitives are completely absent from the current MCP implementation.
					`^webhook_(create|delete|ping|trigger)_(event|delivery)$`,

					// 6. Fails because GitHub Releases are not supported by any tool in this server.
					`^release_(draft|publish|latest|assets)$`,

					// 7. Fails because project Milestones are currently a missing capability.
					`^milestone_(list|create|close|update)_all$`,

					// 8. Fails because organization Team management endpoints are not exposed (only general 'orgs').
					`^org_team_(members|repos|invitations)$`,

					// 9. Fails because GitHub Actions self-hosted Runner infrastructure is out of scope for the server.
					`^runner_(register|unregister|status|labels)$`,

					// 10. Fails because Deployment Environments are not manageable via this toolset.
					`^environment_(deploy|secrets|variables)$`,

					// 11. Fails because repository Forking operations are not natively implemented.
					`^fork_(repository|sync|status)$`,

					// 12. Fails because GitHub Wikis are completely unsupported by the server's file operations.
					`^wiki_(page_edit|history|create)$`,

					// 13. Fails because enterprise or user Billing APIs are intentionally omitted for security.
					`^billing_(actions|packages|storage|copilot)$`,

					// 14. Fails because specific user-profile social sub-resources are not mapped to getter tools.
					`^get_user_(followers|following|starred|gpg_keys)$`,

					// 15. Fails because api version suffixes (e.g., "_v2") are not appended to the tool names.
					`^[a-z]+_[a-z]+_v[0-9]+$`,

					// 16. Fails because the server handles actions logs and triggers, but not deployment pipelines.
					`^deploy(ment)?_(status|created|triggered|failed)$`,

					// 17. Fails because toggling repository states via user interactions (star/watch) isn't built this way.
					`^(star|watch)_repository_toggle$`,

					// 18. Fails because while Copilot integration exists, administrative seat/billing management does not.
					`^copilot_(settings|billing|seat_management)$`,

					// 19. Fails because the longest single-word tool is "notifications" (13 letters). This demands 14+ letters without underscores.
					`^[^_]{14,}$`,

					// 20. Fails because Branch Protection rule management is not exposed as a configurable toolset.
					`^(create|delete|update)_branch_protection_rule$`,
				},
			},
		},
	})
	require.NoError(b, err)

	testCtx := setupTestContext(b, withRole(complexMCPAccessRole))

	mockEmitter := &eventstest.MockRecorderEmitter{}
	auditor, err := newSessionAuditor(sessionAuditorConfig{
		emitter:    mockEmitter,
		hostID:     "test-host-id",
		sessionCtx: testCtx.SessionCtx,
		preparer:   &libevents.NoOpPreparer{},
	})
	require.NoError(b, err)
	handler, err := newSessionHandler(sessionHandlerConfig{
		SessionCtx:     testCtx.SessionCtx,
		sessionAuth:    &sessionAuth{},
		sessionAuditor: auditor,
		accessPoint:    fakeAccessPoint{},
		parentCtx:      ctx,
	})
	require.NoError(b, err)

	requestBuilder := &requestBuilder{}

	requests := make([]*mcputils.JSONRPCRequest, len(tools))
	for i, tool := range tools {
		requests[i] = requestBuilder.makeToolsCallRequest(tool)
	}

	b.ResetTimer()
	for range b.N {
		for _, req := range requests {
			_ = handler.processClientRequest(ctx, req)
		}
	}
}
