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
	"encoding/base32"

	"github.com/gravitational/trace"
	"golang.org/x/text/cases"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type workloadIdentityIndex string

const (
	workloadIdentityNameIndex     workloadIdentityIndex = "name"
	workloadIdentitySpiffeIDIndex workloadIdentityIndex = "spiffe_id"
)

func newWorkloadIdentityCollection(upstream services.WorkloadIdentities, w types.WatchKind) (*collection[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WorkloadIdentities")
	}

	return &collection[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		store: newStore(
			types.KindWorkloadIdentity,
			proto.CloneOf[*workloadidentityv1pb.WorkloadIdentity],
			map[workloadIdentityIndex]func(*workloadidentityv1pb.WorkloadIdentity) string{
				workloadIdentityNameIndex:     keyForWorkloadIdentityNameIndex,
				workloadIdentitySpiffeIDIndex: keyForWorkloadIdentitySpiffeIDIndex,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
			out, err := stream.Collect(clientutils.Resources(ctx,
				func(ctx context.Context, pageSize int, currentToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
					return upstream.ListWorkloadIdentities(ctx, pageSize, currentToken, nil)
				}))
			return out, trace.Wrap(err)
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
func (c *Cache) ListWorkloadIdentities(
	ctx context.Context,
	pageSize int,
	nextToken string,
	options *services.ListWorkloadIdentitiesRequestOptions,
) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListWorkloadIdentities")
	defer span.End()

	index := workloadIdentityNameIndex
	keyFn := keyForWorkloadIdentityNameIndex
	isDesc := options.GetSortDesc()
	switch options.GetSortField() {
	case "name":
		index = workloadIdentityNameIndex
		keyFn = keyForWorkloadIdentityNameIndex
	case "spiffe_id":
		index = workloadIdentitySpiffeIDIndex
		keyFn = keyForWorkloadIdentitySpiffeIDIndex
	case "":
		// default ordering as defined above
	default:
		return nil, "", trace.BadParameter("unsupported sort %q but expected name or spiffe_id", options.GetSortField())
	}

	lister := genericLister[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		cache:      c,
		collection: c.collections.workloadIdentity,
		index:      index,
		isDesc:     isDesc,
		upstreamList: func(ctx context.Context, pageSize int, nextToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
			return c.Config.WorkloadIdentity.ListWorkloadIdentities(ctx, pageSize, nextToken, options)
		},
		filter: func(b *workloadidentityv1pb.WorkloadIdentity) bool {
			return services.MatchWorkloadIdentity(b, options.GetFilterSearchTerm())
		},
		nextToken: keyFn,
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

func keyForWorkloadIdentityNameIndex(r *workloadidentityv1pb.WorkloadIdentity) string {
	return r.GetMetadata().GetName()
}

func keyForWorkloadIdentitySpiffeIDIndex(r *workloadidentityv1pb.WorkloadIdentity) string {
	name := keyForWorkloadIdentityNameIndex(r)
	// Sort case-insensitively to keep /spiffe-1 and /Spiffe-1 together
	spiffeID := cases.Fold().String(r.GetSpec().GetSpiffe().GetId())
	// Encode the id avoid; "a/b" + "/" + "c" vs. "a" + "/" + "b/c". Base32 hex
	// maintains original ordering.
	spiffeID = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(spiffeID))
	// SPIFFE IDs may not be unique, so append the resource name
	return spiffeID + "/" + name
}
