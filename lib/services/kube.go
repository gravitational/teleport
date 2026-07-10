package services

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	kubev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kube/v1"
	"github.com/gravitational/teleport/api/types"
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
