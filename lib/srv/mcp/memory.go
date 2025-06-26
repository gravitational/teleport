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
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport/api/types"
)

const (
	// InMemoryServerEnvVar enables an in-memory MCP server for testing
	// purposes. The test app enables a stdio MCP server that has a
	// "teleport-hello-test" tool and a "teleport-echo-test" tool.
	InMemoryServerEnvVar = "TELEPORT_UNSTABLE_MCP_IN_MEMORY_SERVER"

	// InMemoryServerName is the name of the in-memory MCP server.
	InMemoryServerName = "teleport-mcp-test-server"
)

// NewInMemoryServerApp returns the app definition for the in-memory test server.
func NewInMemoryServerApp() (types.Application, error) {
	app, err := types.NewAppV3(types.Metadata{
		Name: InMemoryServerName,
		Labels: map[string]string{
			types.TeleportInternalLabelPrefix + "mcp-in-memory-server": "true",
		},
	}, types.AppSpecV3{
		MCP: &types.MCP{
			Command:       "in-memory-server",
			RunAsHostUser: "in-memory-server",
		},
	})
	return app, trace.Wrap(err)
}

func isInMemoryServerApp(app types.Application) bool {
	value, ok := app.GetLabel(types.TeleportInternalLabelPrefix + "mcp-in-memory-server")
	return ok && value == "true"
}

func (s *Server) handleInMemoryServerSession(ctx context.Context, sessionCtx *SessionCtx) error {
	s.cfg.Log.DebugContext(ctx, "Started in-memory server session")
	defer s.cfg.Log.DebugContext(ctx, "Completed in-memory server session")

	session, err := s.makeSessionHandler(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	session.emitStartEvent(s.cfg.ParentContext)
	defer session.emitEndEvent(s.cfg.ParentContext)

	// TODO(greedy52) audit log notification and requests.
	server := mcpserver.NewMCPServer("hello-test-server", "1.0.0")
	stdioServer := mcpserver.NewStdioServer(server)
	stdioServer.SetErrorLogger(slog.NewLogLogger(s.cfg.Log.Handler(), slog.LevelDebug))

	helloTool := mcp.NewTool("teleport-hello-test",
		mcp.WithDescription("this is simple hello test and it always return \"hello client\""),
	)
	if session.checkAccessToTool(ctx, helloTool.GetName()) == nil {
		server.AddTool(helloTool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent("hello client")},
			}, nil
		})
	}

	echoTool := mcp.NewTool("teleport-echo-test",
		mcp.WithDescription("this is simple echo and it always return the input back"),
		mcp.WithString("input", mcp.Required(), mcp.Description("input for echo")),
	)
	if session.checkAccessToTool(ctx, echoTool.GetName()) == nil {
		server.AddTool(echoTool, func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			input, err := request.RequireString("input")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(input)},
			}, nil
		})
	}
	return stdioServer.Listen(ctx, sessionCtx.ClientConn, sessionCtx.ClientConn)
}
