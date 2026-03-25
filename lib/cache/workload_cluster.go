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

package cache

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type workloadClusterIndex string

const workloadClusterNameIndex workloadClusterIndex = "name"

func newWorkloadClusterCollection(upstream services.WorkloadClusterService, w types.WatchKind) (*collection[*workloadclusterv1.WorkloadCluster, workloadClusterIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WorkloadClusters")
	}

	return &collection[*workloadclusterv1.WorkloadCluster, workloadClusterIndex]{
		store: newStore(
			types.KindWorkloadCluster,
			proto.CloneOf[*workloadclusterv1.WorkloadCluster],
			map[workloadClusterIndex]func(*workloadclusterv1.WorkloadCluster) string{
				workloadClusterNameIndex: func(r *workloadclusterv1.WorkloadCluster) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*workloadclusterv1.WorkloadCluster, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, i int, nextToken string) ([]*workloadclusterv1.WorkloadCluster, string, error) {
				return upstream.ListWorkloadClusters(ctx, i, nextToken)
			}))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *workloadclusterv1.WorkloadCluster {
			return &workloadclusterv1.WorkloadCluster{
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

// ListWorkloadClusters returns a list of WorkloadCluster resources.
func (c *Cache) ListWorkloadClusters(ctx context.Context, pageSize int, pageToken string) ([]*workloadclusterv1.WorkloadCluster, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWorkloadClusters")
	defer span.End()

	lister := genericLister[*workloadclusterv1.WorkloadCluster, workloadClusterIndex]{
		cache:      c,
		collection: c.collections.workloadClusters,
		index:      workloadClusterNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*workloadclusterv1.WorkloadCluster, string, error) {
			out, next, err := c.Config.WorkloadClusterService.ListWorkloadClusters(ctx, pageSize, pageToken)
			return out, next, trace.Wrap(err)
		},
		nextToken: func(t *workloadclusterv1.WorkloadCluster) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetWorkloadCluster returns the specified WorkloadCluster resource.
func (c *Cache) GetWorkloadCluster(ctx context.Context, name string) (*workloadclusterv1.WorkloadCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWorkloadCluster")
	defer span.End()

	getter := genericGetter[*workloadclusterv1.WorkloadCluster, workloadClusterIndex]{
		cache:       c,
		collection:  c.collections.workloadClusters,
		index:       workloadClusterNameIndex,
		upstreamGet: c.Config.WorkloadClusterService.GetWorkloadCluster,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
