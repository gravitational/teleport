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

// NewKubernetesClient returns an Kubernetes client.
func NewKubernetesClient(c *client.Client) KubernetesClient {
	return KubernetesClient{client: c}
}

// KubernetesClient manages Kubernetes resources.
type KubernetesClient struct {
	client *client.Client
}

// Get reads an Kubernetes by name.
func (r KubernetesClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*types.KubernetesClusterV3, error) {
	cluster, err := r.client.GetKubernetesCluster(ctx, id.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterv3, ok := cluster.(*types.KubernetesClusterV3)
	if !ok {
		return nil, trace.BadParameter("unexpected Kubernetes cluster type: %T", cluster)
	}

	return clusterv3, nil
}

// Create creates an Kubernetes.
func (r KubernetesClient) Create(ctx context.Context, cluster *types.KubernetesClusterV3) error {
	if err := r.client.CreateKubernetesCluster(ctx, cluster); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates an Kubernetes.
func (r KubernetesClient) Upsert(ctx context.Context, cluster *types.KubernetesClusterV3) error {
	if err := r.client.UpdateKubernetesCluster(ctx, cluster); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes an Kubernetes by name.
func (r KubernetesClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
	return trace.Wrap(r.client.DeleteKubernetesCluster(ctx, id.Name))
}
