// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package platform

import (
	"context"
	"io"
	"testing"

	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func TestAccessRequesterServer(t *testing.T) {
	t.Run("Tools", func(t *testing.T) {
		clt := setupPlatformMCPClient(t, PlatformServerConfig{
			Client:                       &mockPlatformClient{},
			Username:                     "alice",
			ClusterName:                  "root",
			AccessRequesterServerEnabled: true,
		})

		// Expect only requester tools to be available.
		expectMCPTools(t, clt, []string{
			listRequestableRoles.GetName(),
			listUserAccessRequestsTool.GetName(),
			createAccessRequestTool.GetName(),
		})
	})

	t.Run("ListRequestableRoles", func(t *testing.T) {
		requestableRoles := []string{"read", "other", "everything"}
		clt := setupPlatformMCPClient(t, PlatformServerConfig{
			Client: &mockPlatformClient{
				getAccessCapabilitiesResponse: &types.AccessCapabilities{
					RequestableRoles: requestableRoles,
				},
			},
			Username:                     "alice",
			ClusterName:                  "root",
			AccessRequesterServerEnabled: true,
		})

		res, err := clt.CallTool(t.Context(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: listRequestableRoles.GetName()}})
		require.NoError(t, err)
		require.False(t, res.IsError)

		var l ListUserRequestableRolesResponse
		require.NoError(t, utils.FastUnmarshal([]byte(toolCallResultText(t, res)), &l))
		require.ElementsMatch(t, requestableRoles, l.Roles)
	})

	t.Run("ListUserAccessRequests", func(t *testing.T) {
		// RawListArgs is a simplified structure that contains all values as
		// primitive types acting like a regular MCP client building the tool
		// arguments. We use this to avoid any pre-validation from our Golang
		// definition like.
		type RawListArgs struct {
			State string `json:"state"`
		}

		for name, tc := range map[string]struct {
			platformClient      PlatformAPIClient
			accessRequestState  string
			expectError         bool
			expectedRequestsIDs []string
		}{
			"pending access requests": {
				platformClient: &mockPlatformClient{
					getAccessRequestsResponse: []types.AccessRequest{
						&types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
						&types.AccessRequestV3{Metadata: types.Metadata{Name: "req-2"}},
						&types.AccessRequestV3{Metadata: types.Metadata{Name: "req-3"}},
					},
				},
				accessRequestState:  "PENDING",
				expectedRequestsIDs: []string{"req-1", "req-2", "req-3"},
			},
			"wrong access requests state": {
				platformClient: &mockPlatformClient{
					getAccessRequestsResponse: []types.AccessRequest{
						&types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
					},
				},
				accessRequestState: "RANDOM",
				expectError:        true,
			},
			"empty": {
				platformClient: &mockPlatformClient{
					getAccessRequestsResponse: []types.AccessRequest{},
				},
				accessRequestState:  "PENDING",
				expectedRequestsIDs: []string{},
			},
			"get error": {
				platformClient: &mockPlatformClient{
					getAccessRequestsErr: trace.AccessDenied("unabled to retrieve requests"),
				},
				accessRequestState: "PENDING",
				expectError:        true,
			},
		} {
			t.Run(name, func(t *testing.T) {
				clt := setupPlatformMCPClient(t, PlatformServerConfig{
					Client:                       tc.platformClient,
					Username:                     "alice",
					ClusterName:                  "root",
					AccessRequesterServerEnabled: true,
				})

				res, err := clt.CallTool(t.Context(), mcp.CallToolRequest{Params: mcp.CallToolParams{
					Name:      listUserAccessRequestsTool.GetName(),
					Arguments: RawListArgs{State: tc.accessRequestState},
				}})
				require.NoError(t, err)
				require.Equal(t, tc.expectError, res.IsError)
				if tc.expectError {
					return
				}

				var l ListUserRequestsResponse
				require.NoError(t, utils.FastUnmarshal([]byte(toolCallResultText(t, res)), &l))

				currentIDs := make([]string, len(l.Requests))
				for i, req := range l.Requests {
					uri, err := clientmcp.ParseResourceURI(req.URI)
					require.NoError(t, err, "expected %q to be a MCP resource URI", req.URI)
					currentIDs[i] = uri.GetAccessRequestID()
				}
				require.ElementsMatch(t, tc.expectedRequestsIDs, currentIDs)
			})
		}
	})

	t.Run("CreateAccessRequest", func(t *testing.T) {
		// RawCreateArgs is a simplified structure that contains all values as
		// primitive types acting like a regular MCP client building the tool
		// arguments. We use this to avoid any pre-validation from our Golang
		// definition like types.Duration parsing.
		type RawCreateArgs struct {
			Roles      []string `json:"roles"`
			RequestTTL string   `json:"request_ttl"`
			SessionTTL string   `json:"session_ttl"`
			Reason     string   `json:"reason"`
		}

		for name, tc := range map[string]struct {
			platformClient    PlatformAPIClient
			createArgs        any
			expectError       bool
			expectedRequestID string
		}{
			"success": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{"role-1"},
					RequestTTL: "10m",
					SessionTTL: "10m",
					Reason:     "Requesting access...",
				},
				expectedRequestID: "req-1",
			},
			"malformatted request_ttl": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{"role-1"},
					RequestTTL: "xxx",
					SessionTTL: "10m",
					Reason:     "Requesting access...",
				},
				expectError: true,
			},
			"malformatted session_ttl": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{"role-1"},
					RequestTTL: "10m",
					SessionTTL: "xxx",
					Reason:     "Requesting access...",
				},
				expectError: true,
			},
			"zero request_ttl": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{"role-1"},
					RequestTTL: "0s",
					SessionTTL: "10m",
					Reason:     "Requesting access...",
				},
				expectError: true,
			},
			"zero session_ttl": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{"role-1"},
					RequestTTL: "10m",
					SessionTTL: "0s",
					Reason:     "Requesting access...",
				},
				expectError: true,
			},
			"empty requested roles": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{},
					RequestTTL: "10m",
					SessionTTL: "10m",
					Reason:     "Requesting access...",
				},
				expectError: true,
			},
			"empty args": {
				platformClient: &mockPlatformClient{
					createAccessRequestResponse: &types.AccessRequestV3{Metadata: types.Metadata{Name: "req-1"}},
				},
				createArgs: RawCreateArgs{
					Roles:      []string{""},
					RequestTTL: "",
					SessionTTL: "",
					Reason:     "",
				},
				expectError: true,
			},
			"create error": {
				platformClient: &mockPlatformClient{
					createAccessRequestErr: trace.AccessDenied("failed to create access request"),
				},
				createArgs: RawCreateArgs{
					Roles:      []string{"role-1"},
					RequestTTL: "10m",
					SessionTTL: "10m",
					Reason:     "Requesting access...",
				},
				expectError: true,
			},
		} {
			t.Run(name, func(t *testing.T) {
				clt := setupPlatformMCPClient(t, PlatformServerConfig{
					Client:                       tc.platformClient,
					Username:                     "alice",
					ClusterName:                  "root",
					AccessRequesterServerEnabled: true,
				})

				res, err := clt.CallTool(t.Context(), mcp.CallToolRequest{Params: mcp.CallToolParams{
					Name:      createAccessRequestTool.GetName(),
					Arguments: tc.createArgs,
				}})
				require.NoError(t, err)
				txt := toolCallResultText(t, res)
				require.Equal(t, tc.expectError, res.IsError, txt)
				if tc.expectError {
					return
				}

				var l AccessRequestsCreateResponse
				require.NoError(t, utils.FastUnmarshal([]byte(txt), &l))
				uri, err := clientmcp.ParseResourceURI(l.Request.URI)
				require.NoError(t, err, "expected %q to be a MCP resource URI", l.Request.URI)
				require.Equal(t, tc.expectedRequestID, uri.GetAccessRequestID())
			})
		}
	})
}

func TestAccessRequestsReviewerServer(t *testing.T) {
	t.Run("Tools", func(t *testing.T) {
		clt := setupPlatformMCPClient(t, PlatformServerConfig{
			Client:                             &mockPlatformClient{},
			Username:                           "alice",
			ClusterName:                        "root",
			AccessRequestsReviwerServerEnabled: true,
		})

		// Expect only requester tools to be available.
		expectMCPTools(t, clt, []string{
			listRequestsToBeReviewedTool.GetName(),
			approveAccessRequestTool.GetName(),
			denyAccessRequestTool.GetName(),
		})
	})

	t.Run("ListRequestsToBeReviwed", func(t *testing.T) {
		user := "alice"
		for name, tc := range map[string]struct {
			platformClient      PlatformAPIClient
			expectError         bool
			expectedRequestsIDs []string
		}{
			"reviwable requests": {
				platformClient: &mockPlatformClient{
					getAccessRequestsResponse: []types.AccessRequest{
						// Other user requested and not reviews yet, should be included.
						&types.AccessRequestV3{
							Metadata: types.Metadata{Name: "req-1"},
							Spec:     types.AccessRequestSpecV3{User: "bob"},
						},
						// User is the requester, shouldn't be included.
						&types.AccessRequestV3{
							Metadata: types.Metadata{Name: "req-2"},
							Spec:     types.AccessRequestSpecV3{User: user},
						},
						// Other user requested, and user didn't review yet, should be included.
						&types.AccessRequestV3{
							Metadata: types.Metadata{Name: "req-3"},
							Spec: types.AccessRequestSpecV3{
								User: "bob",
								Reviews: []types.AccessReview{
									{Author: "jeff"},
								},
							},
						},
						// Other user requested, and user already reviewed it, shouldn't be included.
						&types.AccessRequestV3{
							Metadata: types.Metadata{Name: "req-4"},
							Spec: types.AccessRequestSpecV3{
								User: "bob",
								Reviews: []types.AccessReview{
									{Author: user},
								},
							},
						},
					},
				},
				expectedRequestsIDs: []string{"req-1", "req-3"},
			},
			"empty": {
				platformClient: &mockPlatformClient{
					getAccessRequestsResponse: []types.AccessRequest{},
				},
				expectedRequestsIDs: []string{},
			},
			"get error": {
				platformClient: &mockPlatformClient{
					getAccessRequestsErr: trace.AccessDenied("unabled to retrieve requests"),
				},
				expectError: true,
			},
		} {
			t.Run(name, func(t *testing.T) {
				clt := setupPlatformMCPClient(t, PlatformServerConfig{
					Client:                             tc.platformClient,
					Username:                           user,
					ClusterName:                        "root",
					AccessRequestsReviwerServerEnabled: true,
				})

				res, err := clt.CallTool(t.Context(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: listRequestsToBeReviewedTool.GetName()}})
				require.NoError(t, err)
				require.Equal(t, tc.expectError, res.IsError)
				if tc.expectError {
					return
				}

				var l ListRequestsToBeReviewedResponse
				require.NoError(t, utils.FastUnmarshal([]byte(toolCallResultText(t, res)), &l))

				currentIDs := make([]string, len(l.Requests))
				for i, req := range l.Requests {
					uri, err := clientmcp.ParseResourceURI(req.URI)
					require.NoError(t, err, "expected %q to be a MCP resource URI", req.URI)
					currentIDs[i] = uri.GetAccessRequestID()
				}
				require.ElementsMatch(t, tc.expectedRequestsIDs, currentIDs)
			})
		}
	})

	// RawReviewArgs is a simplified structure that contains all values as
	// primitive types acting like a regular MCP client building the tool
	// arguments. We use this to avoid any pre-validation from our Golang
	// definition.
	type RawReviewArgs struct {
		URI    string `json:"access_request_uri"`
		Reason string `json:"reason"`
	}

	t.Run("Review", func(t *testing.T) {
		clusterName := "root"

		for name, tc := range map[string]struct {
			platformClient PlatformAPIClient
			reviewArgs     RawReviewArgs
			expectError    bool
		}{
			"success": {
				platformClient: &mockPlatformClient{
					submitAccessReviewResponse: &types.AccessRequestV3{},
				},
				reviewArgs: RawReviewArgs{
					URI:    clientmcp.NewAccessRequestResourceURI(clusterName, "req-1").String(),
					Reason: "Hello",
				},
			},
			"error": {
				platformClient: &mockPlatformClient{
					submitAccessReviewErr: trace.AccessDenied("failed to submit review"),
				},
				reviewArgs: RawReviewArgs{
					URI:    clientmcp.NewAccessRequestResourceURI(clusterName, "req-1").String(),
					Reason: "Hello",
				},
				expectError: true,
			},
			"malformed URI": {
				platformClient: &mockPlatformClient{
					submitAccessReviewResponse: &types.AccessRequestV3{},
				},
				reviewArgs: RawReviewArgs{
					URI:    "xxx",
					Reason: "Hello",
				},
				expectError: true,
			},
			"wrong resource URI": {
				platformClient: &mockPlatformClient{
					submitAccessReviewResponse: &types.AccessRequestV3{},
				},
				reviewArgs: RawReviewArgs{
					URI:    clientmcp.NewDatabaseResourceURI(clusterName, "db-1").String(),
					Reason: "Hello",
				},
				expectError: true,
			},
		} {
			for _, toolName := range []string{approveAccessRequestTool.GetName(), denyAccessRequestTool.GetName()} {
				t.Run(toolName+"_"+name, func(t *testing.T) {
					clt := setupPlatformMCPClient(t, PlatformServerConfig{
						Client:                             tc.platformClient,
						Username:                           "alice",
						ClusterName:                        clusterName,
						AccessRequestsReviwerServerEnabled: true,
					})

					res, err := clt.CallTool(t.Context(), mcp.CallToolRequest{Params: mcp.CallToolParams{
						Name:      toolName,
						Arguments: tc.reviewArgs,
					}})
					require.NoError(t, err)
					txt := toolCallResultText(t, res)
					require.Equal(t, tc.expectError, res.IsError, txt)
					if tc.expectError {
						return
					}
				})
			}
		}
	})
}

func TestFormatAccessRequestResource(t *testing.T) {
	user := "alice"
	clusterName := "root"
	rootSrv, err := NewPlaformServer(PlatformServerConfig{
		Client:      &mockPlatformClient{},
		Username:    user,
		ClusterName: clusterName,
	})
	require.NoError(t, err)
	srv := NewAccessRequestsServer(rootSrv)

	req, err := types.NewAccessRequest("req-1", user, "role-1", "role-2")
	require.NoError(t, err)
	req.SetState(types.RequestState_APPROVED)

	resource := srv.buildAccessRequestResource(req)
	require.Equal(t, clusterName, resource.ClusterName)
	require.Equal(t, "APPROVED", resource.State)

	uri, err := clientmcp.ParseResourceURI(resource.URI)
	require.NoError(t, err)
	require.True(t, uri.IsAccessRequest())
	require.Equal(t, req.GetName(), uri.GetAccessRequestID())
	require.Equal(t, clusterName, uri.GetClusterName())
}

func TestAccessRequestErrors(t *testing.T) {
	rootSrv, err := NewPlaformServer(PlatformServerConfig{
		Client:      &mockPlatformClient{},
		Username:    "alice",
		ClusterName: "root",
	})
	require.NoError(t, err)

	srv := NewAccessRequestsServer(rootSrv)
	for name, tc := range map[string]struct {
		err             error
		expectedMessage string
	}{
		"expired session error": {
			err:             apiclient.ErrClientCredentialsHaveExpired,
			expectedMessage: clientmcp.ReloginRequiredErrorMessage,
		},
		"API errors": {
			err:             trace.AccessDenied("failed to fetch access requests"),
			expectedMessage: "failed to fetch access requests",
		},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := srv.formatError(tc.err)
			require.NoError(t, err)
			require.True(t, res.IsError)

			require.Len(t, res.Content, 1)
			content := res.Content[0]
			require.IsType(t, mcp.TextContent{}, content)
			require.Contains(t, content.(mcp.TextContent).Text, tc.expectedMessage)
		})
	}
}

// setupPlatformMCPClient starts serving the platform MCP server and initializes
// the MCP client.
func setupPlatformMCPClient(t *testing.T, cfg PlatformServerConfig) *mcpclient.Client {
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	t.Cleanup(func() {
		require.NoError(t, trace.NewAggregate(
			clientIn.Close(), serverOut.Close(),
			serverIn.Close(), clientOut.Close(),
		))
	})

	srv, err := NewPlaformServer(cfg)
	require.NoError(t, err)

	go func() {
		_ = srv.ServeStdio(t.Context(), serverIn, serverOut)
	}()

	clt := mcptest.NewStdioClient(t, clientIn, clientOut)
	_, err = mcptest.InitializeClient(t.Context(), clt)
	require.NoError(t, err)

	return clt
}

func expectMCPTools(t *testing.T, clt *mcpclient.Client, expectedTools []string) {
	toolsResp, err := clt.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)

	tools := make([]string, len(toolsResp.Tools))
	for i, tool := range toolsResp.Tools {
		tools[i] = tool.GetName()
	}

	require.ElementsMatch(t, expectedTools, tools)
}

// toolCallResultText extracts the text result from a tool call, assuming the
// tool returns a single text result.
func toolCallResultText(t *testing.T, res *mcp.CallToolResult) string {
	require.Len(t, res.Content, 1)
	content := res.Content[0]
	require.IsType(t, mcp.TextContent{}, content)
	return content.(mcp.TextContent).Text
}

type mockPlatformClient struct {
	createAccessRequestErr      error
	createAccessRequestResponse types.AccessRequest

	getAccessCapabilitiesResponse *types.AccessCapabilities
	getAccessCapabilitiesErr      error

	getAccessRequestsResponse []types.AccessRequest
	getAccessRequestsErr      error

	submitAccessReviewResponse types.AccessRequest
	submitAccessReviewErr      error
}

// CreateAccessRequestV2 implements PlatformAPIClient.
func (m *mockPlatformClient) CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error) {
	return m.createAccessRequestResponse, m.createAccessRequestErr
}

// GetAccessCapabilities implements PlatformAPIClient.
func (m *mockPlatformClient) GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	return m.getAccessCapabilitiesResponse, m.getAccessCapabilitiesErr
}

// GetAccessRequests implements PlatformAPIClient.
func (m *mockPlatformClient) GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error) {
	return m.getAccessRequestsResponse, m.getAccessRequestsErr
}

// SubmitAccessReview implements PlatformAPIClient.
func (m *mockPlatformClient) SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error) {
	return m.submitAccessReviewResponse, m.submitAccessReviewErr
}

var _ PlatformAPIClient = (*mockPlatformClient)(nil)
