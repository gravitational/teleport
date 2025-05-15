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
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubewaitingcontainerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
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
		store: newStore(
			types.KubeServer.Copy,
			map[kubeServerIndex]func(types.KubeServer) string{
				kubeServerNameIndex: func(u types.KubeServer) string {
					return u.GetHostID() + "/" + u.GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.KubeServer, error) {
			return p.GetKubernetesServers(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.KubeServer {
			return &types.KubernetesServerV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
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
		store: newStore(
			types.KubeCluster.Copy,
			map[kubeClusterIndex]func(types.KubeCluster) string{
				kubeClusterNameIndex: types.KubeCluster.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.KubeCluster, error) {
			return k.GetKubernetesClusters(ctx)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.KubeCluster {
			return &types.KubernetesClusterV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
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

type kubeWaitingContainerIndex string

const kubeWaitingContainerNameIndex kubeWaitingContainerIndex = "name"

func newKubernetesWaitingContainerCollection(upstream services.KubeWaitingContainer, w types.WatchKind) (*collection[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter KubeWaitingContainers")
	}

	return &collection[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]{
		store: newStore(
			proto.CloneOf[*kubewaitingcontainerv1.KubernetesWaitingContainer],
			map[kubeWaitingContainerIndex]func(*kubewaitingcontainerv1.KubernetesWaitingContainer) string{
				kubeWaitingContainerNameIndex: func(u *kubewaitingcontainerv1.KubernetesWaitingContainer) string {
					return u.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
			var startKey string
			var allConts []*kubewaitingcontainerv1.KubernetesWaitingContainer
			for {
				conts, nextKey, err := upstream.ListKubernetesWaitingContainers(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				allConts = append(allConts, conts...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}
			return allConts, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *kubewaitingcontainerv1.KubernetesWaitingContainer {
			return &kubewaitingcontainerv1.KubernetesWaitingContainer{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// ListKubernetesWaitingContainers lists Kubernetes ephemeral
// containers that are waiting to be created until moderated
// session conditions are met.
func (c *Cache) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerv1.KubernetesWaitingContainer, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListKubernetesWaitingContainers")
	defer span.End()

	lister := genericLister[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]{
		cache:        c,
		collection:   c.collections.kubeWaitingContainers,
		index:        kubeWaitingContainerNameIndex,
		upstreamList: c.Config.KubeWaitingContainers.ListKubernetesWaitingContainers,
		nextToken: func(t *kubewaitingcontainerv1.KubernetesWaitingContainer) string {
			spec := t.GetSpec()
			return spec.GetUsername() + "/" + spec.GetCluster() + "/" + spec.GetNamespace() + "/" + spec.GetPodName() + "/" + t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (c *Cache) GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerv1.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetKubernetesWaitingContainer")
	defer span.End()

	getter := genericGetter[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]{
		cache:      c,
		collection: c.collections.kubeWaitingContainers,
		index:      kubeWaitingContainerNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
			container, err := c.Config.KubeWaitingContainers.GetKubernetesWaitingContainer(ctx, req)
			return container, trace.Wrap(err)
		},
	}

	name := req.GetUsername() + "/" + req.GetCluster() + "/" + req.GetNamespace() + "/" + req.GetPodName() + "/" + req.GetContainerName()
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
