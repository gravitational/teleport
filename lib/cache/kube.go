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
	"iter"
	"log/slog"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"rsc.io/ordered"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	kubewaitingcontainerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	presencev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type kubeServerIndex string

const kubeServerNameIndex kubeServerIndex = "name"
const kubeServerClusterNameIndex kubeServerIndex = "cluster_name"

func kubeServerByClusterNameKey(s types.KubeServer) string {
	// Delete events deliver header only resources with a nil Cluster. This
	// returns "" so the secondary index lookup is a no-op. The primary
	// index deletion removes the entry from all indexes.
	cluster := s.GetCluster()
	if cluster == nil {
		return ""
	}
	// The second component is the scope-aware resource cursor, so within each
	// kube cluster name the in-memory ordering matches the backend listing order
	// (unscoped first, then scoped) and index keys are interchangeable with
	// the fallback pagination tokens.
	return string(ordered.Encode(cluster.GetName(), services.GetCursorForKubeServer(s)))
}

func newKubernetesServerCollection(p services.Presence, w types.WatchKind) (*collection[types.KubeServer, kubeServerIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Presence")
	}

	return &collection[types.KubeServer, kubeServerIndex]{
		store: newStore(
			types.KindKubeServer,
			types.KubeServer.Copy,
			map[kubeServerIndex]func(types.KubeServer) string{
				kubeServerNameIndex:        services.GetCursorForKubeServer,
				kubeServerClusterNameIndex: kubeServerByClusterNameKey,
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

// kubeServerCollection provides read access to cached kubernetes servers.
// Its exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type kubeServerCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	logger   *slog.Logger
	upstream services.Presence
	col      *collection[types.KubeServer, kubeServerIndex]
}

// GetKubernetesServers is a part of auth.Cache implementation
func (c kubeServerCollection) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetKubernetesServers")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		servers, err := c.upstream.GetKubernetesServers(ctx)
		return servers, trace.Wrap(err)
	}

	out := make([]types.KubeServer, 0, rg.store.len())
	for k := range rg.store.resources(kubeServerNameIndex, "", "") {
		out = append(out, k.Copy())
	}

	return out, nil
}

// GetKubernetesServers is a part of auth.Cache implementation
func (c *Cache) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	return kubeServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.Presence,
		col:      c.collections.kubeServers,
	}.GetKubernetesServers(ctx)
}

// RangeKubernetesServersWithName returns an iterator over kubernetes servers for a given cluster name.
func (c kubeServerCollection) RangeKubernetesServersWithName(ctx context.Context, clusterName string) iter.Seq2[types.KubeServer, error] {
	if clusterName == "" {
		return stream.Fail[types.KubeServer](trace.BadParameter("missing kubernetes cluster name"))
	}

	return func(yield func(types.KubeServer, error) bool) {
		ctx, span := c.tracer.Start(ctx, "cache/RangeKubernetesServersWithName")
		defer span.End()

		upstreamListFn := func(ctx context.Context, pageSize int, startToken string) ([]types.KubeServer, string, error) {
			var tokenClusterName string
			rest, err := ordered.DecodePrefix([]byte(startToken), &tokenClusterName)
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			// Verify that the token's cluster name matches the requested cluster name.
			// This ensures that if the token is malformed or belongs to a different
			// cluster, we don't return incorrect results.
			if tokenClusterName != clusterName {
				return nil, "", trace.BadParameter("pagination token does not match the requested kubernetes cluster name")
			}

			// The remainder of the token is the scope aware resource
			startKey := ""
			if len(rest) > 0 {
				if err := ordered.Decode(rest, &startKey); err != nil {
					return nil, "", trace.Wrap(err)
				}
			}

			resp, err := c.upstream.ListResources(ctx, clientproto.ListResourcesRequest{
				ResourceType: types.KindKubeServer,
				Namespace:    defaults.Namespace,
				Limit:        int32(pageSize),
				StartKey:     startKey,
			})
			if err != nil {
				return nil, "", trace.Wrap(err)
			}

			var page []types.KubeServer
			for _, r := range resp.Resources {
				server, ok := r.(types.KubeServer)
				if !ok {
					c.logger.WarnContext(ctx, "expected KubeServer but received unexpected type", "resource_type", logutils.TypeAttr(r))
					continue
				}
				if cluster := server.GetCluster(); cluster != nil && cluster.GetName() == clusterName {
					page = append(page, server)
				}
			}

			next := ""
			if resp.NextKey != "" {
				next = string(ordered.Encode(clusterName, resp.NextKey))
			}
			return page, next, nil
		}

		lister := genericLister[types.KubeServer, kubeServerIndex]{
			engine:          c.engine,
			collection:      c.col,
			index:           kubeServerClusterNameIndex,
			nextToken:       kubeServerByClusterNameKey,
			defaultPageSize: defaults.DefaultChunkSize,
			upstreamList:    upstreamListFn,
		}

		start := string(ordered.Encode(clusterName))
		end := string(ordered.Encode(clusterName, ordered.Inf))
		for item, err := range lister.Range(ctx, start, end) {
			if !yield(item, err) {
				return
			}
		}
	}
}

// RangeKubernetesServersWithName returns an iterator over kubernetes servers for a given cluster name.
func (c *Cache) RangeKubernetesServersWithName(ctx context.Context, clusterName string) iter.Seq2[types.KubeServer, error] {
	return kubeServerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		logger:   c.Logger,
		upstream: c.Config.Presence,
		col:      c.collections.kubeServers,
	}.RangeKubernetesServersWithName(ctx, clusterName)
}

type kubeClusterIndex string

const kubeClusterNameIndex = "name"

// KubeClusterUpstream implements fetching and listing over [types.KubeCluster] resources.
type KubeClusterUpstream interface {
	GetKubeCluster(ctx context.Context, req *presencev1.GetKubeClusterRequest) (types.KubeCluster, error)
	ListKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error)
}

func newKubernetesClusterCollection(upstream KubeClusterUpstream, w types.WatchKind) (*collection[types.KubeCluster, kubeClusterIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter KubeClusterUpstream")
	}

	return &collection[types.KubeCluster, kubeClusterIndex]{
		store: newStore(
			types.KindKubernetesCluster,
			types.KubeCluster.Copy,
			map[kubeClusterIndex]func(types.KubeCluster) string{
				kubeClusterNameIndex: services.GetCursorForKubeCluster,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.KubeCluster, error) {
			return stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeCluster, string, error) {
				return upstream.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
					PageSize:    int32(pageSize),
					PageToken:   pageToken,
					ScopeFilter: w.ScopeFilter.ToProto(),
				}.Build())
			}))
		},
		// TODO (eriktate): DELETE IN v20: headerTransform is kept for backwards compatibility, but delete events for
		// kube clusters are expected to be the resource type going forward
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

// kubeClusterCollection provides read access to cached kubernetes clusters.
// Its exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type kubeClusterCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Kubernetes
	col      *collection[types.KubeCluster, kubeClusterIndex]
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (c kubeClusterCollection) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetKubernetesClusters")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		clusters, err := c.upstream.GetKubernetesClusters(ctx)
		return clusters, trace.Wrap(err)
	}

	out := make([]types.KubeCluster, 0, rg.store.len())
	for k := range rg.store.resources(kubeClusterNameIndex, "", "") {
		out = append(out, k.Copy())
	}

	return out, nil
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (c *Cache) GetKubernetesClusters(ctx context.Context) ([]types.KubeCluster, error) {
	return kubeClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Kubernetes,
		col:      c.collections.kubeClusters,
	}.GetKubernetesClusters(ctx)
}

// ListKubeClusters returns a page of registered kubernetes clusters.
func (c kubeClusterCollection) ListKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListKubeClusters")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}
	lister := genericLister[types.KubeCluster, kubeClusterIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      kubeClusterNameIndex,
		upstreamList: func(ctx context.Context, _ int, _ string) ([]types.KubeCluster, string, error) {
			return c.upstream.ListKubeClusters(ctx, req)
		},
		nextToken: services.GetCursorForKubeCluster,
		filter: func(cluster types.KubeCluster) bool {
			return scopes.MatchScope(scopeFilter, cluster.GetScope())
		},
	}
	return lister.list(ctx, int(req.GetPageSize()), req.GetPageToken())
}

// ListKubeClusters returns a page of registered kubernetes clusters.
func (c *Cache) ListKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) ([]types.KubeCluster, string, error) {
	return kubeClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Kubernetes,
		col:      c.collections.kubeClusters,
	}.ListKubeClusters(ctx, req)
}

// RangeKubeClusters returns kubernetes clusters within the range [start, end).
func (c kubeClusterCollection) RangeKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) iter.Seq2[types.KubeCluster, error] {
	ctx, span := c.tracer.Start(ctx, "cache/RangeKubeClusters")
	defer span.End()
	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.KubeCluster](trace.Wrap(err))
	}

	lister := genericLister[types.KubeCluster, kubeClusterIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      kubeClusterNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.KubeCluster, string, error) {
			return c.upstream.ListKubeClusters(ctx, presencev1.ListKubeClustersRequest_builder{
				PageSize:    int32(pageSize),
				PageToken:   pageToken,
				ScopeFilter: scopeFilter,
			}.Build())
		},
		filter: func(cluster types.KubeCluster) bool {
			return scopes.MatchScope(scopeFilter, cluster.GetScope())
		},
		nextToken: services.GetCursorForKubeCluster,
	}

	return lister.Range(ctx, req.GetPageToken(), "")
}

// RangeKubeClusters returns kubernetes clusters within the range [start, end).
func (c *Cache) RangeKubeClusters(ctx context.Context, req *presencev1.ListKubeClustersRequest) iter.Seq2[types.KubeCluster, error] {
	return kubeClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Kubernetes,
		col:      c.collections.kubeClusters,
	}.RangeKubeClusters(ctx, req)
}

// GetKubeCluster returns the specified kubernetes cluster resource.
func (c kubeClusterCollection) GetKubeCluster(ctx context.Context, req *presencev1.GetKubeClusterRequest) (types.KubeCluster, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetKubeCluster")
	defer span.End()

	clusterCursor := scopes.MakeResourceCursor(req.GetScope(), req.GetName())

	getter := genericGetter[types.KubeCluster, kubeClusterIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      kubeClusterNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (types.KubeCluster, error) {
			return c.upstream.GetKubeCluster(ctx, req)
		},
	}

	out, err := getter.get(ctx, clusterCursor)
	if out == nil {
		return nil, trace.Wrap(err)
	}
	return out.Copy(), trace.Wrap(err)
}

// GetKubeCluster returns the specified kubernetes cluster resource.
func (c *Cache) GetKubeCluster(ctx context.Context, req *presencev1.GetKubeClusterRequest) (types.KubeCluster, error) {
	return kubeClusterCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Kubernetes,
		col:      c.collections.kubeClusters,
	}.GetKubeCluster(ctx, req)
}

type kubeWaitingContainerIndex string

const kubeWaitingContainerNameIndex kubeWaitingContainerIndex = "name"

func newKubernetesWaitingContainerCollection(upstream services.KubeWaitingContainer, w types.WatchKind) (*collection[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter KubeWaitingContainers")
	}

	return &collection[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]{
		store: newStore(
			types.KindKubeWaitingContainer,
			proto.CloneOf[*kubewaitingcontainerv1.KubernetesWaitingContainer],
			map[kubeWaitingContainerIndex]func(*kubewaitingcontainerv1.KubernetesWaitingContainer) string{
				kubeWaitingContainerNameIndex: func(u *kubewaitingcontainerv1.KubernetesWaitingContainer) string {
					spec := u.GetSpec()
					return kubernetesWaitingContainerCacheKey(spec)
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListKubernetesWaitingContainers))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

// kubeWaitingContainerCollection provides read access to cached kubernetes
// waiting containers. Its exported methods are promoted onto every topology
// cache that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no shared
// scaffolding couples their lifetimes.
type kubeWaitingContainerCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.KubeWaitingContainer
	col      *collection[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]
}

// ListKubernetesWaitingContainers lists Kubernetes ephemeral
// containers that are waiting to be created until moderated
// session conditions are met.
func (c kubeWaitingContainerCollection) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerv1.KubernetesWaitingContainer, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListKubernetesWaitingContainers")
	defer span.End()

	lister := genericLister[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        kubeWaitingContainerNameIndex,
		upstreamList: c.upstream.ListKubernetesWaitingContainers,
		nextToken: func(t *kubewaitingcontainerv1.KubernetesWaitingContainer) string {
			spec := t.GetSpec()
			return kubernetesWaitingContainerCacheKey(spec)
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListKubernetesWaitingContainers lists Kubernetes ephemeral
// containers that are waiting to be created until moderated
// session conditions are met.
func (c *Cache) ListKubernetesWaitingContainers(ctx context.Context, pageSize int, pageToken string) ([]*kubewaitingcontainerv1.KubernetesWaitingContainer, string, error) {
	return kubeWaitingContainerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.KubeWaitingContainers,
		col:      c.collections.kubeWaitingContainers,
	}.ListKubernetesWaitingContainers(ctx, pageSize, pageToken)
}

// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (c kubeWaitingContainerCollection) GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerv1.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetKubernetesWaitingContainer")
	defer span.End()

	getter := genericGetter[*kubewaitingcontainerv1.KubernetesWaitingContainer, kubeWaitingContainerIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      kubeWaitingContainerNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
			container, err := c.upstream.GetKubernetesWaitingContainer(ctx, req)
			return container, trace.Wrap(err)
		},
	}

	name := kubernetesWaitingContainerCacheKey(req)
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetKubernetesWaitingContainer returns a Kubernetes ephemeral
// container that are waiting to be created until moderated
// session conditions are met.
func (c *Cache) GetKubernetesWaitingContainer(ctx context.Context, req *kubewaitingcontainerv1.GetKubernetesWaitingContainerRequest) (*kubewaitingcontainerv1.KubernetesWaitingContainer, error) {
	return kubeWaitingContainerCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.KubeWaitingContainers,
		col:      c.collections.kubeWaitingContainers,
	}.GetKubernetesWaitingContainer(ctx, req)
}

type kubernetesWaitingContainerCacheKeyFieldGetter interface {
	GetUsername() string
	GetCluster() string
	GetNamespace() string
	GetPodName() string
	GetContainerName() string
}

func kubernetesWaitingContainerCacheKey(c kubernetesWaitingContainerCacheKeyFieldGetter) string {
	return c.GetUsername() + "/" + c.GetCluster() + "/" + c.GetNamespace() + "/" + c.GetPodName() + "/" + c.GetContainerName()
}
