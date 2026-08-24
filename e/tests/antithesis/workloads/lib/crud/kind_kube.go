package crud

import (
	"context"

	"github.com/gravitational/trace"

	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// KubernetesClusterOpsForClient returns type-erased CRUD operations for Kubernetes cluster resources.
func KubernetesClusterOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.Kubernetes)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindKubernetesCluster)
	}
	return KubernetesClusterOps(c), nil
}

type kubernetesClusterOps struct {
	clt services.Kubernetes
}

type kubernetesClusterKindOps struct {
	*kubernetesClusterOps
	*resourceOpsLegacy[*kubernetesClusterOps, types.KubeCluster]
}

// KubernetesClusterOps returns CRUD operations for Kubernetes cluster resources.
func KubernetesClusterOps(client services.Kubernetes) KindResourceOps {
	ops := &kubernetesClusterOps{clt: client}
	return &kubernetesClusterKindOps{
		kubernetesClusterOps: ops,
		resourceOpsLegacy:    &resourceOpsLegacy[*kubernetesClusterOps, types.KubeCluster]{ops: ops},
	}
}

func (o *kubernetesClusterOps) Kind() string {
	return types.KindKubernetesCluster
}

func (o *kubernetesClusterOps) NewResource(name string) (types.Resource153, error) {
	cluster, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(cluster), nil
}

func (o *kubernetesClusterOps) Clone(resource types.KubeCluster) types.KubeCluster {
	return resource.Copy()
}

func (o *kubernetesClusterOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	cluster, err := unwrapLegacy[types.KubeCluster](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.CreateKubernetesCluster(ctx, cluster))
}

func (o *kubernetesClusterOps) Get(ctx context.Context, name string) (types.KubeCluster, error) {
	return o.clt.GetKubeCluster(ctx, &presencev1.GetKubeClusterRequest{
		Name: name,
	})
}

func (o *kubernetesClusterOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.KubeCluster, string, error) {
	clusters, next, err := o.clt.ListKubeClusters(ctx, &presencev1.ListKubeClustersRequest{
		PageSize:  int32(pageSize),
		PageToken: pageToken,
	})
	return clusters, next, trace.Wrap(err)
}

func (o *kubernetesClusterOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	cluster, err := unwrapLegacy[types.KubeCluster](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.UpdateKubernetesCluster(ctx, cluster))
}

func (o *kubernetesClusterOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteKubeCluster(ctx, &presencev1.DeleteKubeClusterRequest{
		Name: name,
	}))
}

func (o *kubernetesClusterOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllKubernetesClusters(ctx))
}

func (o *kubernetesClusterOps) SupportsDeleteAll() bool {
	return true
}

func (o *kubernetesClusterOps) Equal(a, b types.Resource153) bool {
	return EqualLegacyResource[types.KubeCluster](a, b)
}
