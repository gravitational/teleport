// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type kubeServerIndex string

const kubeServerNameIndex kubeServerIndex = "name"

func newKubernetesServerCollection(p services.Presence, w types.WatchKind) (*collection[types.KubeServer, kubeServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.KubeServer, kubeServerIndex]{
		store: newStore(map[kubeServerIndex]func(types.KubeServer) string{
			kubeServerNameIndex: func(u types.KubeServer) string {
				return u.GetHostID() + "/" + u.GetName()
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

	if !rg.ReadCache() {
		servers, err := c.Config.Presence.GetKubernetesServers(ctx)
		return servers, trace.Wrap(err)
	}

	out := make([]types.KubeServer, 0, rg.store.len())
	for k := range rg.store.resources(kubeServerNameIndex, "", "") {
		out = append(out, k.Copy())
	}

	return out, nil
}

type kubeClusterIndex string

const kubeClusterNameIndex = "name"

func newKubernetesClusterCollection(k services.Kubernetes, w types.WatchKind) (*collection[types.KubeCluster, kubeClusterIndex], error) {
	if k == nil {
		return nil, trace.BadParameter("missing parameter Kubernetes")
	}

	return &collection[types.KubeCluster, kubeClusterIndex]{
		store: newStore(map[kubeClusterIndex]func(types.KubeCluster) string{
			kubeClusterNameIndex: func(u types.KubeCluster) string {
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

	if !rg.ReadCache() {
		clusters, err := c.Config.Kubernetes.GetKubernetesClusters(ctx)
		return clusters, trace.Wrap(err)
	}

	out := make([]types.KubeCluster, 0, rg.store.len())
	for k := range rg.store.resources(kubeClusterNameIndex, "", "") {
		out = append(out, k.Copy())
	}

	return out, nil
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

	if !rg.ReadCache() {
		cluster, err := c.Config.Kubernetes.GetKubernetesCluster(ctx, name)
		return cluster, trace.Wrap(err)
	}

	k, err := rg.store.get(kubeClusterNameIndex, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return k.Copy(), nil
}
