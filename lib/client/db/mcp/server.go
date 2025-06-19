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

package mcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
)

// listDatabasesTool is the MCP tool that list all databases being served
// (from all protocols).
var listDatabasesTool = mcp.NewTool(listDatabasesToolName,
	mcp.WithDescription("List database resources available to be used with Teleport tools."),
)

// RootServer database access root MCP server. It includes common MCP tools and
// resources across different databases and serves as a root server where
// database-specific MCP servers register their tools.
type RootServer struct {
	*mcpserver.MCPServer

	mu                 sync.RWMutex
	logger             *slog.Logger
	availableDatabases map[string]*Database
}

// NewRootServer initializes a new root MCP server.
func NewRootServer(logger *slog.Logger) *RootServer {
	server := &RootServer{
		MCPServer:          mcpserver.NewMCPServer(serverName, teleport.Version),
		logger:             logger,
		availableDatabases: make(map[string]*Database),
	}
	server.AddTool(listDatabasesTool, server.ListDatabases)

	return server
}

// ListDatabases tool function used to list all available/served databases.
func (s *RootServer) ListDatabases(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.availableDatabases) == 0 {
		return mcp.NewToolResultError(EmptyDatabasesListErrorMessage), nil
	}

	var res []mcp.Content
	for _, db := range s.availableDatabases {
		contents, err := encodeDatabaseResource(db)
		if err != nil {
			s.logger.ErrorContext(ctx, "error while list databases", "error", err)
			return mcp.NewToolResultError(FormatErrorMessage(nil, err).Error()), nil
		}
		res = append(res, mcp.EmbeddedResource{Type: "resource", Resource: contents})
	}

	return &mcp.CallToolResult{
		Content: res,
	}, nil
}

// GetDatabaseResource resource handler for databases.
func (s *RootServer) GetDatabaseResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	db, ok := s.availableDatabases[request.Params.URI]
	if !ok {
		return nil, trace.NotFound("Database is %q not available as MCP resource", request.Params.URI)
	}

	encodedDb, err := encodeDatabaseResource(db)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []mcp.ResourceContents{encodedDb}, nil
}

// RegisterDatabase register a database on the root server. This make it
// available as a MCP resource.
//
// TODO(gabrielcorado): support dynamically registering/deregistering databases
// after the server starts.
func (s *RootServer) RegisterDatabase(db *Database) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uri := db.ResourceURI().String()
	s.availableDatabases[uri] = db
	s.AddResource(mcp.NewResource(uri, fmt.Sprintf("%s Database", db.DB.GetName()), mcp.WithMIMEType(databaseResourceMIMEType)), s.GetDatabaseResource)
}

// ServeStdio starts serving the root MCP using STDIO transport.
func (s *RootServer) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	return trace.Wrap(mcpserver.NewStdioServer(s.MCPServer).Listen(ctx, in, out))
}

func buildDatabaseResource(db *Database) DatabaseResource {
	return DatabaseResource{
		Metadata:    db.DB.GetMetadata(),
		URI:         db.ResourceURI().String(),
		Protocol:    db.DB.GetProtocol(),
		ClusterName: db.ClusterName,
	}
}

func encodeDatabaseResource(db *Database) (mcp.ResourceContents, error) {
	resource := buildDatabaseResource(db)
	out, err := yaml.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return mcp.TextResourceContents{
		URI:      resource.URI,
		MIMEType: databaseResourceMIMEType,
		Text:     string(out),
	}, nil
}

const (
	// serverName is the database MCP server name.
	serverName = "teleport_databases"
	// listDatabasesTool is the list databases tool name.
	listDatabasesToolName = ToolPrefix + "list_databases"
	// databaseResourceMIMEType is the MIME type of the database resources.
	databaseResourceMIMEType = "application/yaml"
)
