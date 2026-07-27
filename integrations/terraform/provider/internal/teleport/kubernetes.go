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
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

// NewKubernetesClient returns a Teleport client to interact with Kubernetes Cluster resources.
func NewKubernetesClient(c *client.Client) KubernetesClient {
	return KubernetesClient{client: c}
}

// KubernetesClient manages Kubernetes Cluster resources in Teleport.
type KubernetesClient struct {
	client *client.Client
}

// Get reads a Kubernetes Cluster by name.
func (r KubernetesClient) Get(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) (*types.KubernetesClusterV3, error) {
	cluster, err := r.client.GetKubeCluster(ctx, presencev1.GetKubeClusterRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterv3, ok := cluster.(*types.KubernetesClusterV3)
	if !ok {
		return nil, trace.BadParameter("unexpected Kubernetes cluster type: %T", cluster)
	}

	return clusterv3, nil
}

// Create creates a Kubernetes Cluster resource in Teleport.
func (r KubernetesClient) Create(ctx context.Context, cluster *types.KubernetesClusterV3) error {
	if err := r.client.CreateKubernetesCluster(ctx, cluster); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Upsert updates a Kubernetes Cluster resource in Teleport.
func (r KubernetesClient) Upsert(ctx context.Context, cluster *types.KubernetesClusterV3) error {
	if err := r.client.UpdateKubernetesCluster(ctx, cluster); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Delete deletes a Kubernetes Cluster resource from Teleport by name.
func (r KubernetesClient) Delete(ctx context.Context, id tfdriver.ScopeQualifiedNameIdentifier) error {
	return trace.Wrap(r.client.DeleteKubeCluster(ctx, presencev1.DeleteKubeClusterRequest_builder{
		Name:  id.Name,
		Scope: id.Scope,
	}.Build()))
}
