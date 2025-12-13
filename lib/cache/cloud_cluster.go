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

	cloudclusterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type cloudClusterIndex string

const cloudClusterNameIndex cloudClusterIndex = "name"

func newCloudClusterCollection(upstream services.CloudClusterService, w types.WatchKind) (*collection[*cloudclusterv1.CloudCluster, cloudClusterIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter CloudClusters")
	}

	return &collection[*cloudclusterv1.CloudCluster, cloudClusterIndex]{
		store: newStore(
			types.KindCloudCluster,
			proto.CloneOf[*cloudclusterv1.CloudCluster],
			map[cloudClusterIndex]func(*cloudclusterv1.CloudCluster) string{
				cloudClusterNameIndex: func(r *cloudclusterv1.CloudCluster) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*cloudclusterv1.CloudCluster, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, i int, nextToken string) ([]*cloudclusterv1.CloudCluster, string, error) {
				return upstream.ListCloudClusters(ctx, i, nextToken)
			}))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *cloudclusterv1.CloudCluster {
			return &cloudclusterv1.CloudCluster{
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

// ListCloudClusters returns a list of CloudCluster resources.
func (c *Cache) ListCloudClusters(ctx context.Context, pageSize int, pageToken string) ([]*cloudclusterv1.CloudCluster, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListCloudClusters")
	defer span.End()

	lister := genericLister[*cloudclusterv1.CloudCluster, cloudClusterIndex]{
		cache:      c,
		collection: c.collections.cloudClusters,
		index:      cloudClusterNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*cloudclusterv1.CloudCluster, string, error) {
			out, next, err := c.Config.CloudClusterService.ListCloudClusters(ctx, pageSize, pageToken)
			return out, next, trace.Wrap(err)
		},
		nextToken: func(t *cloudclusterv1.CloudCluster) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetCloudCluster returns the specified CloudCluster resource.
func (c *Cache) GetCloudCluster(ctx context.Context, name string) (*cloudclusterv1.CloudCluster, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCloudCluster")
	defer span.End()

	getter := genericGetter[*cloudclusterv1.CloudCluster, cloudClusterIndex]{
		cache:       c,
		collection:  c.collections.cloudClusters,
		index:       cloudClusterNameIndex,
		upstreamGet: c.Config.CloudClusterService.GetCloudCluster,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
