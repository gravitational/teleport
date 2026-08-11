// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
