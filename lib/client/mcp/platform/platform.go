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
	"log/slog"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"

	"github.com/gravitational/trace"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// PlatformAPIClient defines the API client used by platform MCP servers.
type PlatformAPIClient interface {
	// GetAccessRequests retrieves a list of all access requests matching the provided filter.
	GetAccessRequests(ctx context.Context, filter types.AccessRequestFilter) ([]types.AccessRequest, error)
	// CreateAccessRequestV2 registers a new access request with the auth server.
	CreateAccessRequestV2(ctx context.Context, req types.AccessRequest) (types.AccessRequest, error)
	// SubmitAccessReview applies a review to a request and returns the post-application state.
	SubmitAccessReview(ctx context.Context, params types.AccessReviewSubmission) (types.AccessRequest, error)
	// GetAccessCapabilities requests the access capabilities of a user.
	GetAccessCapabilities(ctx context.Context, req types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error)
}

// PlatformServerConfig is the general platform MCPs configuration.
type PlatformServerConfig struct {
	// Logger is the slog logger.
	Logger *slog.Logger
	// Client is the Teleport API client.
	Client PlatformAPIClient
	// Username is the current user username.
	Username string
	// ClusterName is the current cluster name.
	ClusterName string
	// AccessRequesterServerEnabled defines if the access requester is enabled.
	AccessRequesterServerEnabled bool
	// AccessRequestsReviwerServerEnabled defines if the access requests reviwer
	// is enabled.
	AccessRequestsReviwerServerEnabled bool
}

// CheckAndSetDefaults checks and set defaults.
func (c *PlatformServerConfig) CheckAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.Client == nil {
		return trace.BadParameter("API client is required")
	}
	if c.Username == "" {
		return trace.BadParameter("username is required")
	}
	if c.ClusterName == "" {
		return trace.BadParameter("cluster name is required")
	}
	return nil
}

// Server defines a platform MCP server.
type Server interface {
	// Close closes the MCP server, cleaning up resources.
	Close() error
}

// PlatformServer is the MCP server for platform resources/tools.
//
// This server serves as a root/base server where other servers register their
// tools and resources.
type PlatformServer struct {
	*mcpserver.MCPServer
	config  PlatformServerConfig
	servers []Server
}

// NewPlaformServer initializes the platform MCP server.
func NewPlaformServer(config PlatformServerConfig) (*PlatformServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	srv := &PlatformServer{
		// TODO apply naming convetion here
		MCPServer: mcpserver.NewMCPServer(clientmcp.ServerName("platform"), teleport.Version),
		config:    config,
	}

	if config.AccessRequesterServerEnabled || config.AccessRequestsReviwerServerEnabled {
		srv.servers = append(srv.servers, NewAccessRequestsServer(srv))
	}

	return srv, nil
}

// ServeStdio starts serving the MCP server using STDIO transport.
func (p *PlatformServer) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	return trace.Wrap(mcpserver.NewStdioServer(p.MCPServer).Listen(ctx, in, out))
}

// Close closes all running MCP servers.
func (p *PlatformServer) Close() error {
	var errs []error
	for _, srv := range p.servers {
		errs = append(errs, srv.Close())
	}
	return trace.NewAggregate(errs...)
}
