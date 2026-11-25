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
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewTrustedClusterClient returns a trusted cluster client.
func NewTrustedClusterClient(c *client.Client) TrustedClusterClient {
	return TrustedClusterClient{client: c}
}

// TrustedClusterClient manages trusted cluster resources.
type TrustedClusterClient struct {
	client *client.Client
}

// Get reads a trusted cluster by name.
func (r TrustedClusterClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.TrustedClusterV2, error) {
	trustedCluster, err := r.client.GetTrustedCluster(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedClusterv2, ok := trustedCluster.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.BadParameter("unexpected trusted cluster type: %T", trustedCluster)
	}

	return trustedClusterv2, nil
}

// Create creates a trusted cluster.
func (r TrustedClusterClient) Create(ctx context.Context, trustedCluster *types.TrustedClusterV2) error {
	if _, err := r.client.UpsertTrustedClusterV2(ctx, trustedCluster); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a trusted cluster.
func (r TrustedClusterClient) Upsert(ctx context.Context, trustedCluster *types.TrustedClusterV2) error {
	if _, err := r.client.UpsertTrustedClusterV2(ctx, trustedCluster); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a trusted cluster by name.
func (r TrustedClusterClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteTrustedCluster(ctx, id.Name))
}
