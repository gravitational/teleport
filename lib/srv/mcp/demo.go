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
	_ "embed"
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
	DemoServerName = "teleport-mcp-demo"

	// demoInfoCardResourceURI is the ui:// resource URI for the MCP Apps
	demoInfoCardResourceURI = "ui://teleport-demo/info-card"

	// demoTeleportInfoResourceURI is the ui:// resource URI for the animated
	// Teleport company info card.
	demoTeleportInfoResourceURI = "ui://teleport-demo/teleport-info"
)

//go:embed demo_info_card.html
var demoInfoCardHTML string

//go:embed demo_teleport_info.html
var demoTeleportInfoHTML string

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
		Labels:      map[string]string{types.TeleportInternalResourceType: types.DemoResource},
		Description: "A demo MCP server that shows current user and session information",
	}, types.AppSpecV3{
		URI: fmt.Sprintf("%s://%s", types.SchemeMCPStdio, DemoServerName),
	})
	return app, trace.Wrap(err)
}

func isDemoServerApp(app types.Application) bool {
	labelValue, labelFound := app.GetLabel(types.TeleportInternalResourceType)
	return labelFound && labelValue == types.DemoResource && app.GetName() == DemoServerName
}

func makeDemoServerRunner(ctx context.Context, session *sessionHandler) (stdioServerRunner, error) {
	return makeInMemoryServerRunner(newDemoServer(ctx, session), session.logger)
}

func newDemoServer(_ context.Context, session *sessionHandler) *mcpserver.MCPServer {
	demoServer := mcpserver.NewMCPServer(
		"teleport-demo",
		teleport.Version,
	)

	demoServer.AddResource(
		mcp.NewResource(demoInfoCardResourceURI, "Teleport Info Card",
			mcp.WithMIMEType("text/html;profile=mcp-app"),
		),
		func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      demoInfoCardResourceURI,
					MIMEType: "text/html;profile=mcp-app",
					Text:     demoInfoCardHTML,
				},
			}, nil
		},
	)
	demoServer.AddResource(
		mcp.NewResource(demoTeleportInfoResourceURI, "Teleport Company Info",
			mcp.WithMIMEType("text/html;profile=mcp-app"),
		),
		func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      demoTeleportInfoResourceURI,
					MIMEType: "text/html;profile=mcp-app",
					Text:     demoTeleportInfoHTML,
				},
			}, nil
		},
	)

	// uiMeta links a tool to the info-card UI resource. Clients that support
	// the io.modelcontextprotocol/ui extension pre-fetch the resource and
	// render it as an interactive iframe when the tool is called.
	uiMeta := &mcp.Meta{
		AdditionalFields: map[string]any{
			"ui": map[string]any{
				"resourceUri": demoInfoCardResourceURI,
			},
		},
	}
	teleportInfoUiMeta := &mcp.Meta{
		AdditionalFields: map[string]any{
			"ui": map[string]any{
				"resourceUri": demoTeleportInfoResourceURI,
			},
		},
	}

	userInfoTool := mcp.NewTool(
		"teleport_user_info",
		mcp.WithDescription("Shows basic information about your Teleport user."),
	)
	userInfoTool.Meta = uiMeta

	sessionInfoTool := mcp.NewTool(
		"teleport_session_info",
		mcp.WithDescription("Shows information about this MCP session."),
	)
	sessionInfoTool.Meta = uiMeta

	teleportInfoTool := mcp.NewTool(
		"teleport_info",
		mcp.WithDescription("Shows information about Teleport, the Infrastructure Access Platform."),
	)
	teleportInfoTool.Meta = teleportInfoUiMeta

	tools := []mcpserver.ServerTool{
		{
			Tool:    userInfoTool,
			Handler: makeUserInfoToolHandler(session),
		},
		{
			Tool:    sessionInfoTool,
			Handler: makeSessionInfoToolHandler(session),
		},
		{
			Tool: mcp.NewTool(
				"teleport_demo_info",
				mcp.WithDescription("Shows information about this Teleport Demo MCP server."),
			),
			Handler: makeDemoInfoToolHandler(),
		},
		{
			Tool:    teleportInfoTool,
			Handler: makeTeleportInfoToolHandler(),
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

func makeSessionInfoToolHandler(session *sessionHandler) mcpserver.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := json.Marshal(map[string]any{
			"teleport_cluster":         session.Identity.RouteToApp.ClusterName,
			"teleport_app_name":        session.App.GetName(),
			"teleport_app_description": session.App.GetDescription(),
			"mcp_transport_type":       types.GetMCPServerTransportType(session.App.GetURI()),
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
- 'teleport_user_info': Shows basic information about your Teleport user.
- 'teleport_session_info': Shows information about this MCP session.
- 'teleport_demo_info' (this tool): Shows information about this Teleport Demo MCP server.

You can restrict what tools a user can access by listing allowed MCP tools in
the role spec 'role.allow.mcp.tools'.

To learn more about enrolling MCP servers and additional reference materials, please visit:
https://goteleport.com/docs/enroll-resources/mcp-access
`
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(text), nil
	}
}

func makeTeleportInfoToolHandler() mcpserver.ToolHandlerFunc {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data, err := json.Marshal(map[string]any{
			"tagline": "The Infrastructure Access Platform",
			"website": "goteleport.com",
			"features": []string{
				"Zero Trust Access",
				"Role-Based Access Control",
				"Audit Logging",
				"Database Access",
				"Kubernetes Access",
				"MCP Server Access",
			},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return mcp.NewToolResultText(string(data)), nil
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
