/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package implementation

import (
	"context"

	"github.com/gravitational/teleport/lib/nodetracker"
	"github.com/gravitational/teleport/lib/nodetracker/api"

	"github.com/gravitational/trace"

	"google.golang.org/grpc"
)

// Client is a node tracker client implementation of api.Client
// This might be moved to teleport/e or somewhere else private
type Client struct {
	client api.NodeTrackerServiceClient
}

// NewClient initializes a node tracker service grpc client
func NewClient(listenAddress string) error {
	connection, err := grpc.Dial(listenAddress, grpc.WithInsecure())
	if err != nil {
		return err
	}

	c := Client{
		client: api.NewNodeTrackerServiceClient(connection),
	}

	nodetracker.SetClient(&c)
	return nil
}

// AddNode calls the add node api on the remote node tracker server
func (c *Client) AddNode(
	ctx context.Context,
	nodeID string,
	proxyID string,
	clusterName string,
	addr string,
) error {
	_, err := c.client.AddNode(
		ctx,
		&api.AddNodeRequest{
			NodeID:      nodeID,
			ProxyID:     proxyID,
			ClusterName: clusterName,
			Addr:        addr,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// RemoveNode calls the remove node api on the remote node tracker server
func (c *Client) RemoveNode(ctx context.Context, nodeID string) error {
	_, err := c.client.RemoveNode(
		ctx,
		&api.RemoveNodeRequest{
			NodeID: nodeID,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetProxies calls get proxy api on the remote node tracker server
func (c *Client) GetProxies(ctx context.Context, nodeID string) ([]api.ProxyDetails, error) {
	response, err := c.client.GetProxies(
		ctx,
		&api.GetProxiesRequest{
			NodeID: nodeID,
		},
	)
	if err != nil {
		return nil, trace.NotFound(err.Error())
	}

	return response.ProxyDetails, nil
}
