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
	"io"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

// Tool defines an interface an MCP tool being registered with Teleport MCP
// server must implement.
type Tool interface {
	// GetTool returns the MCP tool definition.
	GetTool() mcp.Tool
	// GetHandler returns the MCP tool handler.
	GetHandler() server.ToolHandlerFunc
}

type toolMaker func(cfg Config) (Tool, error)

var mu sync.Mutex
var toolsRegistry map[string][]toolMaker = make(map[string][]toolMaker)

// RegisterTool adds an MCP tool to the registry of available tools for the
// server to register.
func RegisterTool(toolset string, t toolMaker) {
	mu.Lock()
	defer mu.Unlock()
	toolsRegistry[toolset] = append(toolsRegistry[toolset], t)
}

func getToolsRegistry() map[string][]toolMaker {
	mu.Lock()
	defer mu.Unlock()
	return toolsRegistry
}

// Config is the Teleport MCP server configuration.
type Config struct {
	// Auth is the cluster's auth client.
	Auth authclient.ClientI
	// WebProxyAddr is the web proxy address. Used to build web UI URLs.
	WebProxyAddr string
	// Log is the server logger.
	Log *slog.Logger
	// Toolsets is the list of toolsets to register. This allows to selectively
	// toggle certain groups of tools to avoid LLM tool context pollution.
	//
	// Empty means all available tools.
	Toolsets []string
}

// CheckAndSetDefaults checks the MCP server config and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Auth == nil {
		return trace.BadParameter("missing auth client")
	}
	if c.WebProxyAddr == "" {
		return trace.BadParameter("missing web proxy address")
	}
	if c.Log == nil {
		return trace.BadParameter("missing logger")
	}
	return nil
}

// Server is the Teleport MCP server.
type Server struct {
	// MCPServer is the underlying MCP server.
	*server.MCPServer
	// cfg is the MCP server configuration.
	cfg Config
}

// MCPServerName is the name of the Teleport MCP server.
const MCPServerName = "teleport"

// NewMCPServer builds an MCP server and registers all tools.
func NewMCPServer(cfg Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	server := &Server{
		MCPServer: server.NewMCPServer(MCPServerName, teleport.Version),
		cfg:       cfg,
	}
	for _, toolMaker := range getToolsToRegister(cfg) {
		tool, err := toolMaker(cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.Log.Info("Registering MCP tool", "tool", tool.GetTool().Name)
		server.AddTool(tool.GetTool(), tool.GetHandler())
	}
	return server, nil
}

// ListenStdio starts the MCP server on stdio.
func (s *Server) ListenStdio(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	return server.NewStdioServer(s.MCPServer).Listen(ctx, stdin, stdout)
}

// getToolsToRegister returns the list of tools to register based on the enabled
// toolsets in the config.
func getToolsToRegister(cfg Config) (tools []toolMaker) {
	registry := getToolsRegistry()
	if len(cfg.Toolsets) == 0 {
		for _, tool := range registry {
			tools = append(tools, tool...)
		}
		return tools
	}
	for _, toolset := range cfg.Toolsets {
		tools = append(tools, registry[toolset]...)
	}
	return tools
}
