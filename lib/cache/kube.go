package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func newKubernetesServerCollection(p services.Presence, w types.WatchKind) (*collection[types.KubeServer], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.KubeServer]{
		store: newStore(map[string]func(types.KubeServer) string{
			"name": func(u types.KubeServer) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.KubeServer, error) {
			return p.GetKubernetesServers(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.KubeServer {
			return &types.KubernetesServerV3{
				Kind:    types.KindKubeServer,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
				Spec: types.KubernetesServerSpecV3{
					HostID: hdr.Metadata.Description,
				},
			}
		},
		watch: w,
	}, nil
}

// GetKubernetesServers is a part of auth.Cache implementation
func (c *Cache) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesServers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.kubeServers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		out := make([]types.KubeServer, 0, rg.store.len())
		for k := range rg.store.resources("name", "", "") {
			out = append(out, k.Copy())
		}

		return out, nil
	}

	servers, err := c.Config.Presence.GetKubernetesServers(ctx)
	return servers, trace.Wrap(err)
}

func newKubernetesClusterCollection(k services.Kubernetes, w types.WatchKind) (*collection[types.KubeCluster], error) {
	if k == nil {
		return nil, trace.BadParameter("missing parameter Kubernetes")
	}

	return &collection[types.KubeCluster]{
		store: newStore(map[string]func(types.KubeCluster) string{
			"name": func(u types.KubeCluster) string {
				return u.GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.KubeCluster, error) {
			return k.GetKubernetesClusters(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.KubeCluster {
			return &types.KubernetesClusterV3{
				Kind:    types.KindKubernetesCluster,
				Version: types.V3,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (c *Cache) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesClusters")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.kubeClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		out := make([]types.KubeCluster, 0, rg.store.len())
		for k := range rg.store.resources("name", "", "") {
			out = append(out, k.Copy())
		}

		return out, nil
	}

	clusters, err := c.Config.Kubernetes.GetKubernetesClusters(ctx)
	return clusters, trace.Wrap(err)
}

// GetKubernetesCluster returns the specified kubernetes cluster resource.
func (c *Cache) GetKubernetesCluster(ctx context.Context, name string) (types.KubeCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesCluster")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.kubeClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		k, err := rg.store.get("name", name)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return k.Copy(), nil
	}

	cluster, err := c.Config.Kubernetes.GetKubernetesCluster(ctx, name)
	return cluster, trace.Wrap(err)
}
