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

package services

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	kubev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"
)

type KubeClusterService interface {
	DeleteKubeCluster(ctx context.Context, req *kubev1.DeleteKubeClusterRequest) error
	GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (types.KubeCluster, error)
	ListKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error)
	RangeKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest, startKey, endKey string) iter.Seq2[types.KubeCluster, error]
}

type KubeClusterReader interface {
	GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (types.KubeCluster, error)
	RangeKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest, startKey, endKey string) iter.Seq2[types.KubeCluster, error]
}

type KubeClusterUpstream interface {
	GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (types.KubeCluster, error)
	ListKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error)
}

type kubeClusterClientAdapter struct {
	grpcClient kubev1.KubeClusterServiceClient
}

func NewKubeClusterClientAdapter(grpcClient kubev1.KubeClusterServiceClient) KubeClusterUpstream {
	return kubeClusterClientAdapter{
		grpcClient: grpcClient,
	}
}

func (c kubeClusterClientAdapter) GetKubeCluster(ctx context.Context, req *kubev1.GetKubeClusterRequest) (types.KubeCluster, error) {
	res, err := c.grpcClient.GetKubeCluster(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res.GetCluster(), nil
}

func (c kubeClusterClientAdapter) ListKubeClusters(ctx context.Context, req *kubev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error) {
	res, err := c.grpcClient.ListKubeClusters(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	clusters := make([]types.KubeCluster, len(res.GetClusters()))
	for i, cluster := range res.GetClusters() {
		clusters[i] = cluster
	}
	return clusters, res.GetNextPageToken(), nil
}

// GetCursorForKubeCluster returns the backend key for a kube cluster with
// consideration for whether or not it is scoped.
func GetCursorForKubeCluster(cluster types.KubeCluster) string {
	return scopes.MakeResourceCursor(cluster.GetScope(), cluster.GetName())
}
