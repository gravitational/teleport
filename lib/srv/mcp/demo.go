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
	"fmt"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/mcputils"
)

const (
	// DemoServerName is the name of the "Teleport Demo" MCP server.
	DemoServerName = "teleport-demo"

	// demoServerLabel is a special label used to identity the demo server.
	demoServerLabel = types.TeleportNamespace + "/mcp-demo-server"
)

// NewDemoServerApp returns the app definition for the "Teleport Demo" MCP
// server.
//
// The purpose of the "Teleport Demo" MCP server is to provide a quick demo on
// MCP access without the need for external environment setup on MCP
// servers. This MCP server is in-memory only and uses stdio transport. Access
// to this MCP server is the same as any other MCP server (`tsh`, RBAC, audit
// events, etc.).
func NewDemoServerApp() (types.Application, error) {
	app, err := types.NewAppV3(types.Metadata{
		Name:        DemoServerName,
		Labels:      map[string]string{demoServerLabel: "true"},
		Description: "Teleport MCP access demo server",
	}, types.AppSpecV3{
		MCP: &types.MCP{
			Command:       "teleport",
			RunAsHostUser: "teleport",
		},
	})
	return app, trace.Wrap(err)
}

func isDemoServerApp(app types.Application) bool {
	labelValue, labelFound := app.GetLabel(demoServerLabel)
	return labelFound && labelValue == "true" && app.GetName() == DemoServerName
}

func makeDemoServerRunner(ctx context.Context, session *sessionHandler) (stdioServerRunner, error) {
	return makeInMemoryServerRunner(newDemoServer(ctx, session), session.logger)
}

func newDemoServer(_ context.Context, session *sessionHandler) *mcpserver.MCPServer {
	demoServer := mcpserver.NewMCPServer(
		"teleport-demo",
		teleport.Version,
	)

	tools := []mcpserver.ServerTool{
		{
			Tool: mcp.NewTool(
				"teleport_user_info",
				mcp.WithDescription("Shows connected Teleport user information."),
			),
			Handler: makeUserInfoToolHandler(session),
		},
		{
			Tool: mcp.NewTool(
				"teleport_demo_info",
				mcp.WithDescription("Shows information about this Teleport Demo MCP server"),
			),
			Handler: makeDemoInfoToolHandler(),
		},
		{
			// IMPORTANT: remember to update this mini guide when new
			// capabilities are added.
			Tool: mcp.NewTool(
				"teleport_enroll_mcp_server_guide",
				mcp.WithDescription("A quick guide for enrolling MCP servers with Teleport."),
			),
			Handler: makeEnrollMCPGuideToolHandler(),
		},
	}

	demoServer.AddTools(tools...)
	return demoServer
}

func makeUserInfoToolHandler(session *sessionHandler) mcpserver.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := json.Marshal(map[string]any{
			"name":      session.AuthCtx.User.GetName(),
			"user_kind": session.makeUserMetadata().UserKind.String(),
			"roles":     session.AuthCtx.User.GetRoles(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func makeDemoInfoToolHandler() mcpserver.ToolHandlerFunc {
	text := `Teleport can provide secure connections to your MCP servers while
improving both access control and visibility.

This 'teleport-demo' MCP server is a demonstration that showcases how Teleport
MCP access works.

You can find this 'teleport-demo' server in the Teleport Web UI or by running
'tsh mcp ls'.

To connect to the demo server with stdio transport from your AI tool, use 'tsh
mcp connect teleport-demo'. Or run 'tsh mcp config teleport-demo' for more
configuration details.

If you are an auditor, you can also find this MCP session and corresponding
requests in the audit events.

Available Tools from the demo server:
- 'teleport_user_info': Displays basic information about your Teleport user.
- 'teleport_demo_info' (this tool): Provides an overview of this demo server.
- 'teleport_enroll_mcp_server_guide': A quick guide for enrolling MCP servers with Teleport.
`
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(text), nil
	}
}

func makeEnrollMCPGuideToolHandler() mcpserver.ToolHandlerFunc {
	yamlBlockStart := "```yaml"
	yamlBlockEnd := "```"
	text := fmt.Sprintf(`Teleport can provide secure connections to your MCP
servers while improving both access control and visibility.

First, add MCP server definitions to the YAML config of your Teleport
Application service then restart it. Here is a sample configuration:
%s
app_service:
  enabled: true
  # Enables this demo server.
  mcp_demo_server: true
  apps:
  # This section contains definitions of all applications proxied by this
  # service. It can contain multiple items.
  apps:
  # Name of the application. Used for identification purposes.
  - name: "mcp-everything"
    # Free-form application description.
    description: "Example everything MCP server"
    # Static labels to assign to the app. Used in RBAC.
    labels:
      env: "prod"
    # Contains MCP server-related configurations.
    mcp:
      # Command to launch stdio-based MCP servers. Must be available on the host
      # running this Teleport application service.
      command: "docker"
      # Args to execute with the command.
      args: ["run", "-i", "--rm", "mcp/everything"]
      # Name of the host user account under which the command will be
      # executed. Use a dedicated host user account for best security or use the
      # same user account that runs Teleport. Required for stdio-based MCP
      # servers.
      run_as_host_user: "docker"
%s

You can use Teleport's role-based access control (RBAC) system to set up
granular permissions for authenticating to MCP servers connected to Teleport.
Here's a sample role:
%s
kind: role
metadata:
  name: mcp-developer
spec:
  allow:
    # app_labels: a user with this role will be allowed to connect to
    # MCP servers with labels matching below.
    app_labels:
      "env": "dev"
    # app_labels_expression: optional field which has the same purpose of the
    # matching app_labels fields, but support predicate expressions instead of
    # label matchers.
    app_labels_expression: 'labels["env"] == "staging"'

    # mcp: defines MCP servers related permissions.
    mcp:
      # tools: list of tools allowed for this role.
      #
      # No tools are allowed if not specified.
      # Each entry can be a literal string, a glob pattern, or a regular
      # expression (must start with '^' and end with '$'). A wildcard '*' allows
      # all tools.
      # This value field also supports variable interpolation.
      tools:
      - search-files
      - teleport_*
      - ^(get|list|read|slack).*$
      - "{{internal.mcp_tools}}"
      - "{{external.mcp_tools}}"

  deny:
    mcp:
      # tools: list of tools denied for this role.
      tools:
      - slack_post_message

version: v8
%s

Now once your Teleport users are granted MCP access, run 'tsh mcp ls' to list of
MCP servers and 'tsh mcp config' to configure your AI tool.

For more details, see official documentation at:
https://goteleport.com/docs/enroll-resources/mcp-access
`, yamlBlockStart, yamlBlockEnd, yamlBlockStart, yamlBlockEnd)
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(text), nil
	}
}

type inMemoryServerRunner struct {
	serverStdin    io.ReadCloser
	serverStdout   io.WriteCloser
	writeToServer  io.WriteCloser
	readFromServer io.ReadCloser
	mcpServer      *mcpserver.MCPServer
	log            *slog.Logger
}

func makeInMemoryServerRunner(mcpServer *mcpserver.MCPServer, log *slog.Logger) (stdioServerRunner, error) {
	if mcpServer == nil {
		return nil, trace.BadParameter("mcpServer must not be nil")
	}
	if log == nil {
		log = slog.Default()
	}

	serverStdin, writeToServer := io.Pipe()
	readFromServer, serverStdout := io.Pipe()
	return &inMemoryServerRunner{
		serverStdin:    serverStdin,
		serverStdout:   serverStdout,
		writeToServer:  writeToServer,
		readFromServer: readFromServer,
		mcpServer:      mcpServer,
		log:            log,
	}, nil
}

func (s *inMemoryServerRunner) getStdinPipe() (io.WriteCloser, error) {
	return s.writeToServer, nil
}

func (s *inMemoryServerRunner) getStdoutPipe() (io.ReadCloser, error) {
	return s.readFromServer, nil
}

func (s *inMemoryServerRunner) run(ctx context.Context) error {
	s.log.DebugContext(ctx, "Running in-memory MCP server")
	defer s.log.DebugContext(ctx, "Finished running in-memory MCP server")
	err := mcpserver.NewStdioServer(s.mcpServer).Listen(ctx, s.serverStdin, s.serverStdout)
	if err != nil && !mcputils.IsOKCloseError(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (s *inMemoryServerRunner) close() {
	if err := s.writeToServer.Close(); err != nil {
		s.log.DebugContext(context.Background(), "Failed to close pipe", "error", err)
	}
	if err := s.serverStdout.Close(); err != nil {
		s.log.DebugContext(context.Background(), "Failed to close pipe", "error", err)
	}
}
