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
	"log/slog"
	"net"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/mcp"
)

// NewServerConfig configuration passed to the server constructors.
type NewServerConfig struct {
	Logger     *slog.Logger
	RootServer *RootServer
	Databases  []*Database
}

// NewServerFunc the MCP server constructor function definition.
type NewServerFunc func(context.Context, *NewServerConfig) (Server, error)

// Server represents a MCP server.
type Server interface {
	// Close closes the server.
	Close(context.Context) error
}

// Registry represents the available databases MCP servers per protocol and
// their constructors.
type Registry map[string]NewServerFunc

// IsSupported returns if the database protocol is supported by any MCP server
// available.
func (m Registry) IsSupported(protocol string) bool {
	_, ok := m[protocol]
	return ok
}

// LookupFunc is the function used to resolve database address. Follows the
// net.Resolver.LookupAddr format.
type LookupFunc func(ctx context.Context, host string) (addrs []string, err error)

// DialContextFunc is a function used to dial the database. Follows the
// net.Dialer.DialContext format.
type DialContextFunc func(ctx context.Context, network string, addr string) (net.Conn, error)

// Database the database served by an MCP server.
type Database struct {
	// DB contains all information from the database.
	DB types.Database
	// ClusterName is the cluster name where the database is located.
	ClusterName string
	// Addr is the address the MCP server used to create a new database
	// connection.
	Addr string
	// DatabaseUser is the database username used on the connections.
	DatabaseUser string
	// DatabaseName is the database name used on the connections.
	DatabaseName string
	// ExternalErrorRetriever used to retrieve any external error that might
	// have happened while connecting/communicating with the database.
	ExternalErrorRetriever ExternalErrorRetriever
	// LookupFunc is the lookup function to resolve database address.
	LookupFunc LookupFunc
	// DialContextFunc is the dial function used to connect to the database.
	DialContextFunc DialContextFunc
}

// ResourceURI returns the database MCP resource URI.
func (d Database) ResourceURI() mcp.ResourceURI {
	return mcp.NewDatabaseResourceURI(d.ClusterName, d.DB.GetName())
}

// DatabaseResource MCP resource representation of a Teleport database.
type DatabaseResource struct {
	types.Metadata
	// URI is the MCP URI resource.
	URI string `json:"uri"`
	// Protocol is the database protocol.
	Protocol string `json:"protocol"`
	// ClusterName is the cluster the database is.
	ClusterName string `json:"cluster_name"`
}

// ToolName generates a database access tool name.
func ToolName(protocol, name string) string {
	return ToolPrefix + protocol + "_" + name
}

// ToolPrefix is the default tool prefix for every MCP tool.
const ToolPrefix = "teleport_"
