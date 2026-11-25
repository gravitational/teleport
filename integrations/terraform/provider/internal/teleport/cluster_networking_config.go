// Copyright 2026 Gravitational, Inc.
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

package teleport

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewClusterNetworkingConfigClient returns a Teleport client to interact with the cluster networking configuration.
func NewClusterNetworkingConfigClient(c *client.Client) ClusterNetworkingConfigClient {
	return ClusterNetworkingConfigClient{client: c}
}

// ClusterNetworkingConfigClient manages the Teleport cluster networking configuration.
type ClusterNetworkingConfigClient struct {
	client *client.Client
}

// Get reads the Teleport cluster networking configuration.
func (r ClusterNetworkingConfigClient) Get(ctx context.Context, _ tfdriver.SingletonIdentifier) (*types.ClusterNetworkingConfigV2, error) {
	cfg, err := r.client.ClusterConfigClient().GetClusterNetworkingConfig(ctx, &clusterconfigv1.GetClusterNetworkingConfigRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cfg, nil
}

// Create creates the Teleport cluster networking configuration.
func (r ClusterNetworkingConfigClient) Create(ctx context.Context, cfg *types.ClusterNetworkingConfigV2) error {
	if _, err := r.client.UpsertClusterNetworkingConfig(ctx, cfg); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates the Teleport cluster networking configuration.
func (r ClusterNetworkingConfigClient) Upsert(ctx context.Context, cfg *types.ClusterNetworkingConfigV2) error {
	if _, err := r.client.UpsertClusterNetworkingConfig(ctx, cfg); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete resets the Teleport cluster networking configuration.
func (r ClusterNetworkingConfigClient) Delete(ctx context.Context, _ tfdriver.SingletonIdentifier) error {
	return trace.Wrap(r.client.ResetClusterNetworkingConfig(ctx))
}
