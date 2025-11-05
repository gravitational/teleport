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
	"encoding/json"
	"net"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

type setupTestContextOptions struct {
	roleSet    services.RoleSet
	app        types.Application
	clientConn net.Conn
}

type setupTestContextOptionFunc func(*setupTestContextOptions)

func withApp(app types.Application) setupTestContextOptionFunc {
	return func(o *setupTestContextOptions) {
		o.app = app
	}
}

func withRole(role types.Role) setupTestContextOptionFunc {
	return func(opts *setupTestContextOptions) {
		opts.roleSet = append(opts.roleSet, role)
	}
}

func withClientConn(conn net.Conn) setupTestContextOptionFunc {
	return func(opts *setupTestContextOptions) {
		opts.clientConn = conn
	}
}

// withAdminRole assigns to ai_user a role that allows all MCP servers and their
// tools.
func withAdminRole(t *testing.T) setupTestContextOptionFunc {
	t.Helper()
	role := services.NewPresetMCPUserRole()
	require.NoError(t, services.CheckAndSetDefaults(role))
	return withRole(role)
}

// withProdReadOnlyRole assigns to the ai_user a role that allows MCP servers
// with label env=prod and allows read-only tools.
func withProdReadOnlyRole(t *testing.T) setupTestContextOptionFunc {
	t.Helper()
	role, err := types.NewRole("prod-read-only", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: map[string]apiutils.Strings{
				"env": {"prod"},
			},
			MCP: &types.MCPPermissions{
				Tools: []string{"^(get|read|list|search).*$"},
			},
		},
	})
	require.NoError(t, err)
	return withRole(role)
}

// withDenyToolsRole assigns to the ai_user a role that denies all MCP tools.
func withDenyToolsRole(t *testing.T) setupTestContextOptionFunc {
	t.Helper()
	role, err := types.NewRole("deny-access", types.RoleSpecV6{
		Deny: types.RoleConditions{
			MCP: &types.MCPPermissions{
				Tools: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)
	return withRole(role)
}

func makeDualPipeNetConn(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	clientSourceConn, clientDestConn, err := utils.DualPipeNetConn(
		utils.MustParseAddr("127.0.0.1:1111"),
		utils.MustParseAddr("127.0.0.1:2222"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		clientSourceConn.Close()
		clientDestConn.Close()
	})
	return clientSourceConn, clientDestConn
}

type testContext struct {
	*SessionCtx

	// clientSourceConn connects to SessionCtx.ClientConn.
	clientSourceConn net.Conn
}

func setupTestContext(t *testing.T, applyOpts ...setupTestContextOptionFunc) testContext {
	t.Helper()

	var opts setupTestContextOptions
	for _, applyOpt := range applyOpts {
		applyOpt(&opts)
	}

	// Fake connection if not passed in.
	var clientSourceConn, clientDestConn net.Conn
	if opts.clientConn != nil {
		clientDestConn = opts.clientConn
	} else {
		clientSourceConn, clientDestConn = makeDualPipeNetConn(t)
	}

	// App.
	if opts.app == nil {
		app, err := types.NewAppV3(types.Metadata{
			Name: "my-mcp-server",
			Labels: map[string]string{
				"env": "prod",
			},
		}, types.AppSpecV3{
			MCP: &types.MCP{
				Command:       "npx",
				Args:          []string{"my-mcp-server"},
				RunAsHostUser: "my-user",
			},
		})
		require.NoError(t, err)
		opts.app = app
	}

	// SessionCtx.
	sessionCtx := &SessionCtx{
		ClientConn: clientDestConn,
		App:        opts.app,
		AuthCtx:    makeTestAuthContext(t, opts.roleSet, opts.app),
	}
	require.NoError(t, sessionCtx.checkAndSetDefaults())

	return testContext{
		clientSourceConn: clientSourceConn,
		SessionCtx:       sessionCtx,
	}
}

func makeTestAuthContext(t *testing.T, roleSet services.RoleSet, app types.Application) *authz.Context {
	t.Helper()

	user, err := types.NewUser("ai")
	require.NoError(t, err)
	user.SetRoles(slices.Collect(types.ResourceNames(roleSet)))

	identity := authz.LocalUser{
		Username: user.GetName(),
		Identity: tlsca.Identity{
			Username:   user.GetName(),
			Groups:     user.GetRoles(),
			Principals: user.GetLogins(),
		},
	}
	if app != nil {
		identity.Identity.RouteToApp.Name = app.GetName()
		identity.Identity.RouteToApp.SessionID = "session-id-for+" + app.GetName()
	}

	accessInfo, err := services.AccessInfoFromLocalTLSIdentity(identity.Identity)
	require.NoError(t, err)
	checker := services.NewAccessCheckerWithRoleSet(accessInfo, "my-cluster", roleSet)
	return &authz.Context{
		User:     user,
		Identity: identity,
		Checker:  checker,
	}
}

type requestBuilder struct {
	idCounter int64
}

func (c *requestBuilder) makeRequestID() mcp.RequestId {
	return mcp.NewRequestId(atomic.AddInt64(&c.idCounter, 1))
}

func (c *requestBuilder) makeToolsCallRequest(toolName string) *mcputils.JSONRPCRequest {
	return &mcputils.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      c.makeRequestID(),
		Method:  mcp.MethodToolsCall,
		Params: mcputils.JSONRPCParams{
			"name": toolName,
		},
	}
}

func (c *requestBuilder) makeToolsListRequest() *mcputils.JSONRPCRequest {
	return &mcputils.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      c.makeRequestID(),
		Method:  mcp.MethodToolsList,
	}
}

func makeToolsCallResponse(t *testing.T, requestID mcp.RequestId, toolNames ...string) *mcputils.JSONRPCResponse {
	t.Helper()
	result := mcp.ListToolsResult{}
	for _, toolName := range toolNames {
		result.Tools = append(result.Tools, mcp.NewTool(toolName, mcp.WithDescription("description")))
	}
	resultJSON, err := json.Marshal(&result)
	require.NoError(t, err)
	return &mcputils.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      requestID,
		Result:  resultJSON,
	}
}

type fakeAccessPoint struct {
}

func (f fakeAccessPoint) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}
func (f fakeAccessPoint) GetClusterName(context.Context) (types.ClusterName, error) {
	clusterName, err := types.NewClusterName(types.ClusterNameSpecV2{
		ClusterName: "my-cluster",
		ClusterID:   "my_cluster_id",
	})
	return clusterName, trace.Wrap(err)
}

func checkParamsHaveNameField(t *testing.T, params *apievents.Struct, wantName string) {
	t.Helper()
	require.NotNil(t, params)
	require.NotNil(t, params.Fields)
	value, ok := params.Fields["name"]
	require.True(t, ok)
	require.Equal(t, wantName, value.GetStringValue())
}

func checkToolsListResponse(t *testing.T, response mcp.JSONRPCMessage, wantID mcp.RequestId, wantTools []string) {
	t.Helper()
	// assume we don't know the internal type of response.
	data, err := json.Marshal(response)
	require.NoError(t, err)

	var mcpResponse mcputils.JSONRPCResponse
	require.NoError(t, json.Unmarshal(data, &mcpResponse))
	require.Equal(t, wantID.String(), mcpResponse.ID.String())

	var result mcp.ListToolsResult
	require.NoError(t, json.Unmarshal(mcpResponse.Result, &result))
	checkToolsListResult(t, &result, wantTools)
}

func checkToolsListResult(t *testing.T, result *mcp.ListToolsResult, wantTools []string) {
	t.Helper()
	require.NotNil(t, result)
	var actualNames []string
	for _, tool := range result.Tools {
		actualNames = append(actualNames, tool.Name)
	}
	require.ElementsMatch(t, wantTools, actualNames)
}

func newDockerClient(t *testing.T) *docker.Client {
	t.Helper()
	dockerClient, err := docker.NewClientWithOpts(
		docker.FromEnv,
		docker.WithAPIVersionNegotiation(),
		docker.WithTimeout(10*time.Second),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		dockerClient.Close()
	})
	return dockerClient
}

func findDockerContainer(ctx context.Context, dockerClient *docker.Client, containerName string) container.Summary {
	containers, err := dockerClient.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return container.Summary{}
	}
	for _, container := range containers {
		if slices.Contains(container.Names, "/"+containerName) {
			return container
		}
	}
	return container.Summary{}
}

func findDockerContainerID(ctx context.Context, dockerClient *docker.Client, containerName string) string {
	return findDockerContainer(ctx, dockerClient, containerName).ID
}

func forceRemoveContainer(t *testing.T, dockerClient *docker.Client, containerName string) {
	if containerID := findDockerContainerID(context.Background(), dockerClient, containerName); containerID != "" {
		if err := dockerClient.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true}); err != nil {
			t.Log("Failed to remove container", err)
		}
	}
}

type mockAuthClient struct {
}

func (m mockAuthClient) GenerateAppToken(_ context.Context, req types.GenerateAppTokenRequest) (string, error) {
	return "app-token-for-" + req.Username, nil
}
