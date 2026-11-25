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
		Name:        id.Name,
		Scope:       id.Scope,
		WithSecrets: true,
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
