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
	"os"
	"path"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/mcptest"
)

func Test_handleAuthErrStdio(t *testing.T) {
	ctx := t.Context()
	s, err := NewServer(ServerConfig{
		Emitter:       &events.DiscardEmitter{},
		ParentContext: ctx,
		HostID:        "my-host-id",
		AccessPoint:   fakeAccessPoint{},
		CipherSuites:  utils.DefaultCipherSuites(),
		AuthClient:    mockAuthClient{},
	})
	require.NoError(t, err)

	testCtx := setupTestContext(t, withAdminRole(t))

	originalAuthErr := trace.AccessDenied("test access denied")
	handlerDoneCh := make(chan struct{}, 1)
	go func() {
		handlerErr := s.HandleUnauthorizedConnection(ctx, testCtx.SessionCtx.ClientConn, testCtx.SessionCtx.App, originalAuthErr)
		handlerDoneCh <- struct{}{}
		require.ErrorIs(t, handlerErr, originalAuthErr)
	}()

	stdioClient := mcptest.NewStdioClientFromConn(t, testCtx.clientSourceConn)
	_, err = mcptest.InitializeClient(ctx, stdioClient)
	require.ErrorContains(t, err, originalAuthErr.Error())

	select {
	case <-time.After(time.Second * 10):
		require.Fail(t, "timed out waiting for handler")
	case <-handlerDoneCh:
	}
}

func Test_handleStdio(t *testing.T) {
	ctx := t.Context()
	testCtx := setupTestContext(t, withAdminRole(t))
	emitter := eventstest.MockRecorderEmitter{}
	s, err := NewServer(ServerConfig{
		Emitter:       &emitter,
		ParentContext: ctx,
		HostID:        "my-host-id",
		AccessPoint:   fakeAccessPoint{},
		CipherSuites:  utils.DefaultCipherSuites(),
		AuthClient:    mockAuthClient{},
	})
	require.NoError(t, err)

	handlerDoneCh := make(chan struct{}, 1)
	defer close(handlerDoneCh)
	go func() {
		// Use the demo server.
		handlerErr := s.handleStdio(ctx, testCtx.SessionCtx, makeDemoServerRunner)
		handlerDoneCh <- struct{}{}
		require.NoError(t, handlerErr)
	}()

	// Use a real client. Verify session start and end events.
	stdioClient := mcptest.NewStdioClientFromConn(t, testCtx.clientSourceConn)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		event := emitter.LastEvent()
		_, ok := event.(*apievents.MCPSessionStart)
		require.True(t, ok)
	}, time.Second*5, time.Millisecond*100, "expect session start")

	// Some basic tests on the demo server.
	resp, err := mcptest.InitializeClient(ctx, stdioClient)
	require.NoError(t, err)
	require.Equal(t, "teleport-demo", resp.ServerInfo.Name)

	listToolsResult, err := stdioClient.ListTools(ctx, mcp.ListToolsRequest{})
	require.NoError(t, err)
	checkToolsListResult(t, listToolsResult, []string{
		"teleport_user_info",
		"teleport_session_info",
		"teleport_demo_info",
	})

	callToolResult, err := stdioClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "teleport_user_info",
		},
	})
	require.NoError(t, err)
	require.Len(t, callToolResult.Content, 1)

	// Now close the client.
	stdioClient.Close()
	select {
	case <-time.After(time.Second * 5):
		require.Fail(t, "timed out waiting for handler")
	case <-handlerDoneCh:
	}
	event := emitter.LastEvent()
	_, ok := event.(*apievents.MCPSessionEnd)
	require.True(t, ok)
}

// TestHandleSession_execMCPServer tests real server handler for stdio-based MCP
// server but requires docker installed locally. It will run the "everything"
// MCP server and "alpine".
//
// TELEPORT_MCP_TEST_DOCKER_WITH_HOST_USER needs to be set with a username to
// run as for the docker command. To test running with ambient credential vs a
// different user, run the test once with your current user as host user then
// run one more time with sudo.
func TestHandleSession_execMCPServer(t *testing.T) {
	runAsHostUser := os.Getenv("TELEPORT_MCP_TEST_DOCKER_WITH_HOST_USER")
	if runAsHostUser == "" {
		t.Skip("This test requires docker and set TELEPORT_MCP_TEST_DOCKER_WITH_HOST_USER=<host_user_name> in your environment")
	}

	dockerClient := newDockerClient(t)
	s, err := NewServer(ServerConfig{
		Emitter:       &events.DiscardEmitter{},
		ParentContext: t.Context(),
		HostID:        "my-host-id",
		AccessPoint:   fakeAccessPoint{},
		CipherSuites:  utils.DefaultCipherSuites(),
		AuthClient:    mockAuthClient{},
	})
	require.NoError(t, err)

	containerShouldBeRemoved := func(t *testing.T, _ *testContext, containerName string) {
		t.Helper()
		require.Empty(t, findDockerContainerID(t.Context(), dockerClient, containerName))
	}

	connectAfterHandlerStart := func(t *testing.T, testCtx *testContext, containerName string) {
		t.Helper()

		stdioClient := mcptest.NewStdioClientFromConn(t, testCtx.clientSourceConn)
		defer stdioClient.Close()

		reqCtx, reqCancel := context.WithTimeout(t.Context(), time.Second*5)
		defer reqCancel()
		resp, err := mcptest.InitializeClient(reqCtx, stdioClient)
		require.NoError(t, err)
		require.Equal(t, "example-servers/everything", resp.ServerInfo.Name)

		// Check container is running.
		require.NotEmpty(t, findDockerContainerID(t.Context(), dockerClient, containerName))
	}

	tests := []struct {
		name               string
		cmd                string
		checkHandlerError  require.ErrorAssertionFunc
		dockerRunArgs      []string
		cancelHandlerCtx   bool
		afterHandlerStart  func(t *testing.T, testCtx *testContext, containerName string)
		waitForHandlerExit time.Duration
		afterHandlerStop   func(t *testing.T, testCtx *testContext, containerName string)
	}{
		{
			// Verify initialize response from real everything server. Then
			// close the transport to trigger shutdown
			name:               "everything success",
			cmd:                "docker",
			dockerRunArgs:      []string{"mcp/everything"},
			checkHandlerError:  require.NoError,
			afterHandlerStart:  connectAfterHandlerStart,
			waitForHandlerExit: time.Second * 5,
			afterHandlerStop:   containerShouldBeRemoved,
		},
		{
			// Make sure handler exits when handler context is canceled.
			name:               "cancel handler context",
			cmd:                "docker",
			dockerRunArgs:      []string{"mcp/everything"},
			checkHandlerError:  require.NoError,
			cancelHandlerCtx:   true,
			waitForHandlerExit: time.Second * 5,
			afterHandlerStart:  connectAfterHandlerStart,
			afterHandlerStop:   containerShouldBeRemoved,
		},
		{
			// Make sure handler is not blocked when command fails to start.
			name:               "fail to start",
			cmd:                "fail-to-start",
			checkHandlerError:  require.Error,
			waitForHandlerExit: time.Second * 5,
		},
		{
			// Make sure handler is not blocked when command starts then fails
			// right away.
			name:               "error exit",
			cmd:                "docker",
			dockerRunArgs:      []string{"--some-unknown-flag"},
			checkHandlerError:  require.Error,
			waitForHandlerExit: time.Second * 5,
			afterHandlerStop:   containerShouldBeRemoved,
		},
		{
			// Make sure SIGKILL is sent when the MCP server traps SIGINT. This
			// test will last more than 10 seconds to trigger the WaitDelay.
			//
			// Unfortunately, SIGKILL won't remove the docker container. So
			// do not check containerShouldBeRemoved and just let the test do
			// the cleanup. In the future, Teleport should either implement
			// proper docker runtime (like how this test is using docker API) or
			// introduce a proper shut down sequence (like systemctl stop).
			name: "wait for SIGKILL",
			cmd:  "docker",
			dockerRunArgs: []string{
				"alpine", "sh", "-c",
				`trap "" INT; while :; do sleep 1; done`,
			},
			checkHandlerError: require.Error,
			afterHandlerStart: func(t *testing.T, testCtx *testContext, _ string) {
				// Trigger shutdown.
				testCtx.clientSourceConn.Close()
				t.Log("waiting 10 seconds for SIGKILL")
			},
			waitForHandlerExit: time.Second * 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containerName := "teleport-test-mcp-" + path.Base(t.Name())
			t.Cleanup(func() {
				forceRemoveContainer(t, dockerClient, containerName)
			})

			args := tt.dockerRunArgs
			if tt.cmd == "docker" {
				args = append([]string{"run", "--name", containerName, "-i", "--rm"}, tt.dockerRunArgs...)
			}

			app, err := types.NewAppV3(types.Metadata{
				Name: t.Name(),
			}, types.AppSpecV3{
				MCP: &types.MCP{
					Command:       tt.cmd,
					Args:          args,
					RunAsHostUser: runAsHostUser,
				},
			})
			require.NoError(t, err)

			testCtx := setupTestContext(t, withAdminRole(t), withApp(app))
			handlerCtx, handlerCtxCancel := context.WithCancel(t.Context())
			defer handlerCtxCancel()
			handlerDoneCh := make(chan struct{}, 1)
			defer close(handlerDoneCh)
			go func() {
				handlerErr := s.HandleSession(handlerCtx, testCtx.SessionCtx)
				handlerDoneCh <- struct{}{}
				tt.checkHandlerError(t, handlerErr)
			}()

			if tt.afterHandlerStart != nil {
				tt.afterHandlerStart(t, &testCtx, containerName)
			}
			if tt.cancelHandlerCtx {
				handlerCtxCancel()
			}

			select {
			case <-time.After(tt.waitForHandlerExit):
				require.Fail(t, "timed out waiting for handler")
			case <-handlerDoneCh:
			}

			if tt.afterHandlerStop != nil {
				tt.afterHandlerStop(t, &testCtx, containerName)
			}
		})
	}
}
