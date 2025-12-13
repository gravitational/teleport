// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudcluster

import (
	"context"

	"github.com/gravitational/trace"

	cloudclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
)

// Client is a client for the Cloud Cluster API.
type Client struct {
	grpcClient cloudclusterv1.CloudClusterServiceClient
}

// NewClient creates a new Cloud Cluster client.
func NewClient(grpcClient cloudclusterv1.CloudClusterServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListCloudClusters returns a list of Cloud Clusters.
func (c *Client) ListCloudClusters(ctx context.Context, pageSize int, nextToken string) ([]*cloudclusterv1.CloudCluster, string, error) {
	resp, err := c.grpcClient.ListCloudClusters(ctx, &cloudclusterv1.ListCloudClustersRequest{
		PageSize:  int32(pageSize),
		PageToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.Clusters, resp.NextPageToken, nil
}

// CreateCloudCluster creates a new Cloud Cluster.
func (c *Client) CreateCloudCluster(ctx context.Context, req *cloudclusterv1.CloudCluster) (*cloudclusterv1.CloudCluster, error) {
	resp, err := c.grpcClient.CreateCloudCluster(ctx, &cloudclusterv1.CreateCloudClusterRequest{
		Cluster: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// GetCloudCluster returns a Cloud Cluster by name.
func (c *Client) GetCloudCluster(ctx context.Context, name string) (*cloudclusterv1.CloudCluster, error) {
	resp, err := c.grpcClient.GetCloudCluster(ctx, &cloudclusterv1.GetCloudClusterRequest{
		Name: name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpdateCloudCluster updates an existing Cloud Cluster.
func (c *Client) UpdateCloudCluster(ctx context.Context, req *cloudclusterv1.CloudCluster) (*cloudclusterv1.CloudCluster, error) {
	resp, err := c.grpcClient.UpdateCloudCluster(ctx, &cloudclusterv1.UpdateCloudClusterRequest{
		Cluster: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// UpsertCloudCluster upserts a Cloud Cluster.
func (c *Client) UpsertCloudCluster(ctx context.Context, req *cloudclusterv1.CloudCluster) (*cloudclusterv1.CloudCluster, error) {
	resp, err := c.grpcClient.UpsertCloudCluster(ctx, &cloudclusterv1.UpsertCloudClusterRequest{
		Cluster: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// DeleteCloudCluster deletes a Cloud Cluster.
func (c *Client) DeleteCloudCluster(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteCloudCluster(ctx, &cloudclusterv1.DeleteCloudClusterRequest{
		Name: name,
	})
	return trace.Wrap(err)
}
