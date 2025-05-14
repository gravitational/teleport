// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type workloadIdentityIndex string

const workloadIdentityNameIndex workloadIdentityIndex = "name"

func newWorkloadIdentityCollection(upstream services.WorkloadIdentities, w types.WatchKind) (*collection[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WorkloadIdentities")
	}

	return &collection[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		store: newStore(
			proto.CloneOf[*workloadidentityv1pb.WorkloadIdentity],
			map[workloadIdentityIndex]func(*workloadidentityv1pb.WorkloadIdentity) string{
				workloadIdentityNameIndex: func(r *workloadidentityv1pb.WorkloadIdentity) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
			var out []*workloadidentityv1pb.WorkloadIdentity
			var nextToken string
			for {
				var page []*workloadidentityv1pb.WorkloadIdentity
				var err error

				const defaultPageSize = 0
				page, nextToken, err = upstream.ListWorkloadIdentities(ctx, defaultPageSize, nextToken)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				out = append(out, page...)
				if nextToken == "" {
					break
				}
			}
			return out, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *workloadidentityv1pb.WorkloadIdentity {
			return &workloadidentityv1pb.WorkloadIdentity{
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

// ListWorkloadIdentities returns a paginated list of WorkloadIdentity resources.
func (c *Cache) ListWorkloadIdentities(ctx context.Context, pageSize int, nextToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWorkloadIdentities")
	defer span.End()

	lister := genericLister[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		cache:        c,
		collection:   c.collections.workloadIdentity,
		index:        workloadIdentityNameIndex,
		upstreamList: c.Config.WorkloadIdentity.ListWorkloadIdentities,
		nextToken: func(t *workloadidentityv1pb.WorkloadIdentity) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, nextToken)
	return out, next, trace.Wrap(err)
}

// GetWorkloadIdentity returns a single WorkloadIdentity by name
func (c *Cache) GetWorkloadIdentity(ctx context.Context, name string) (*workloadidentityv1pb.WorkloadIdentity, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWorkloadIdentity")
	defer span.End()

	getter := genericGetter[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		cache:       c,
		collection:  c.collections.workloadIdentity,
		index:       workloadIdentityNameIndex,
		upstreamGet: c.Config.WorkloadIdentity.GetWorkloadIdentity,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
