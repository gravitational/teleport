// Copyright 2024 Gravitational, Inc.
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

package kubeprovision

import (
	"context"

	"github.com/gravitational/trace"

	kubeprovisionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubeprovision/v1"
)

// Client is a client for the Kube Provision API.
type Client struct {
	grpcClient kubeprovisionv1.KubeProvisionServiceClient
}

// NewClient creates a new Kube Provision service client.
func NewClient(grpcClient kubeprovisionv1.KubeProvisionServiceClient) *Client {
	return &Client{
		grpcClient: grpcClient,
	}
}

// ListKubeProvisions returns a list of Kube Provisions.
func (c *Client) ListKubeProvisions(ctx context.Context, pageSize int, nextToken string) ([]*kubeprovisionv1.KubeProvision, string, error) {
	resp, err := c.grpcClient.ListKubeProvisions(ctx, &kubeprovisionv1.ListKubeProvisionsRequest{
		PageSize:  int32(pageSize),
		PageToken: nextToken,
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.KubeProvisions, resp.NextPageToken, nil
}

// CreateKubeProvision creates a new Kube Provision.
func (c *Client) CreateKubeProvision(ctx context.Context, req *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error) {
	rsp, err := c.grpcClient.CreateKubeProvision(ctx, &kubeprovisionv1.CreateKubeProvisionRequest{KubeProvision: req})
	return rsp, trace.Wrap(err)
}

// GetKubeProvision returns a Kube Provision by name.
func (c *Client) GetKubeProvision(ctx context.Context, name string) (*kubeprovisionv1.KubeProvision, error) {
	rsp, err := c.grpcClient.GetKubeProvision(ctx, &kubeprovisionv1.GetKubeProvisionRequest{Name: name})
	return rsp, trace.Wrap(err)
}

// UpdateKubeProvision updates an existing Kube Provision.
func (c *Client) UpdateKubeProvision(ctx context.Context, req *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error) {
	rsp, err := c.grpcClient.UpdateKubeProvision(ctx, &kubeprovisionv1.UpdateKubeProvisionRequest{KubeProvision: req})
	return rsp, trace.Wrap(err)
}

// UpsertKubeProvision upserts a Kube Provision.
func (c *Client) UpsertKubeProvision(ctx context.Context, req *kubeprovisionv1.KubeProvision) (*kubeprovisionv1.KubeProvision, error) {
	rsp, err := c.grpcClient.UpsertKubeProvision(ctx, &kubeprovisionv1.UpsertKubeProvisionRequest{KubeProvision: req})
	return rsp, trace.Wrap(err)
}

// DeleteKubeProvision deletes a Kube Provision.
func (c *Client) DeleteKubeProvision(ctx context.Context, name string) error {
	_, err := c.grpcClient.DeleteKubeProvision(ctx, &kubeprovisionv1.DeleteKubeProvisionRequest{Name: name})
	return trace.Wrap(err)
}

// DeleteAllKubeProvisions is not implemented for the client.
// Deprecated: Can't delete all KubeProvisions over gRPC.
func (c *Client) DeleteAllKubeProvisions(_ context.Context) error {
	return trace.NotImplemented("DeleteAllKubeProvisions is not implemented for gRPC.")
}
