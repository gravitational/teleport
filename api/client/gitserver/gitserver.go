// Copyright 2024 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitserver

import (
	"context"

	gitserverv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type Client struct {
	grpcClient gitserverv1.GitServerServiceClient
}

func NewClient(grpcClient gitserverv1.GitServerServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// GetGitServer is used to retrieve a Git server object.
func (c *Client) GetGitServer(ctx context.Context, name string) (types.Server, error) {
	resp, err := c.grpcClient.GetGitServer(ctx, &gitserverv1.GetGitServerRequest{
		Name: name,
	})
	return resp, trace.Wrap(err)
}

// UpsertGitServer is used to create or replace a Git server object.
func (c *Client) UpsertGitServer(ctx context.Context, server types.Server) (types.Server, error) {
	serverV2, ok := server.(*types.ServerV2)
	if !ok {
		return nil, trace.BadParameter("server object is not *types.ServerV2")
	}

	resp, err := c.grpcClient.UpsertGitServer(ctx, &gitserverv1.UpsertGitServerRequest{
		Server: serverV2,
	})
	return resp, trace.Wrap(err)
}

// DeleteGitServer is used to delete a Git server object.
func (c *Client) DeleteGitServer(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteGitServer(ctx, &gitserverv1.DeleteGitServerRequest{
		Name: name,
	})
	return trace.Wrap(err)
}
