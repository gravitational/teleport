// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package gitserver

import (
	"context"

	"github.com/gravitational/trace"

	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
)

// ReadOnlyClient defines getter functions for Git servers.
type ReadOnlyClient interface {
	// ListGitServers returns a paginated list of Git servers.
	ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error)
	// GetGitServer returns a Git server by name.
	GetGitServer(ctx context.Context, name string) (types.Server, error)
}

// Client is an Git servers client.
type Client struct {
	grpcClient gitserverv1.GitServerServiceClient
}

// NewClient creates a new Git servers client.
func NewClient(grpcClient gitserverv1.GitServerServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetGitServer returns Git servers by name.
func (c *Client) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	server, err := c.grpcClient.GetGitServer(ctx, &gitserverv1.GetGitServerRequest{Name: name})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return server, nil
}

// ListGitServers returns all Git servers matching filter.
func (c *Client) ListGitServers(ctx context.Context, pageSize int, pageToken string) ([]types.Server, string, error) {
	resp, err := c.grpcClient.ListGitServers(ctx, &gitserverv1.ListGitServersRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	servers := make([]types.Server, 0, len(resp.Servers))
	for _, server := range resp.Servers {
		servers = append(servers, server)
	}
	return servers, resp.NextPageToken, nil
}

func toServerV2(server types.Server) (*types.ServerV2, error) {
	serverV2, ok := server.(*types.ServerV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected server type: %T", serverV2)
	}
	return serverV2, nil
}

// CreateGitServer creates a Git server resource.
func (c *Client) CreateGitServer(ctx context.Context, item types.Server) (types.Server, error) {
	serverV2, err := toServerV2(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := c.grpcClient.CreateGitServer(ctx, &gitserverv1.CreateGitServerRequest{
		Server: serverV2,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpdateGitServer updates a Git server resource.
func (c *Client) UpdateGitServer(ctx context.Context, item types.Server) (types.Server, error) {
	serverV2, err := toServerV2(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := c.grpcClient.UpdateGitServer(ctx, &gitserverv1.UpdateGitServerRequest{
		Server: serverV2,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpsertGitServer updates a Git server resource, creating it if it doesn't exist.
func (c *Client) UpsertGitServer(ctx context.Context, item types.Server) (types.Server, error) {
	serverV2, err := toServerV2(item)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := c.grpcClient.UpsertGitServer(ctx, &gitserverv1.UpsertGitServerRequest{
		Server: serverV2,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// DeleteGitServer removes the specified Git server resource.
func (c *Client) DeleteGitServer(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteGitServer(ctx, &gitserverv1.DeleteGitServerRequest{Name: name})
	return trace.Wrap(err)
}

// DeleteAllGitServers removes all Git server resources.
func (c *Client) DeleteAllGitServers(ctx context.Context) error {
	return trace.NotImplemented("DeleteAllGitServers servers not implemented")
}

// CreateGitHubAuthRequest starts GitHub OAuth flow for authenticated user.
func (c *Client) CreateGitHubAuthRequest(ctx context.Context, req *types.GithubAuthRequest, org string) (*types.GithubAuthRequest, error) {
	resp, err := c.grpcClient.CreateGitHubAuthRequest(ctx, &gitserverv1.CreateGitHubAuthRequestRequest{
		Request:      req,
		Organization: org,
	})
	return resp, trace.Wrap(err)
}
