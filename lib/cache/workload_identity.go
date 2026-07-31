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
	"iter"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
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

	nameKey, err := services.WorkloadIdentityKey(services.WorkloadIdentitySortFieldName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spiffeIDKey, err := services.WorkloadIdentityKey(services.WorkloadIdentitySortFieldSPIFFEID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The seed must select the same set as the event stream, which is filtered
	// per-event by services.WatchKindMatchesScope; an unfiltered seed would leave
	// permanently stale out-of-scope entries in the store.
	scopeFilter := w.ScopeFilter.ToProto()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, trace.Wrap(err)
	}
	// An omitted filter means match-all here but identity-based defaults at the
	// authz layer, so require the config to say which is meant.
	if scopeFilter.GetMode() == scopesv1.Mode_MODE_UNSPECIFIED {
		return nil, trace.BadParameter("workload identity cache requires an explicit scope filter mode")
	}

	return &collection[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		store: newStore(
			types.KindWorkloadIdentity,
			proto.CloneOf[*workloadidentityv1pb.WorkloadIdentity],
			map[workloadIdentityIndex]func(*workloadidentityv1pb.WorkloadIdentity) string{
				workloadIdentityNameIndex:     nameKey,
				workloadIdentitySpiffeIDIndex: spiffeIDKey,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*workloadidentityv1pb.WorkloadIdentity, error) {
			out, err := stream.Collect(stream.FilterMap(
				upstream.RangeWorkloadIdentities(ctx, "", "", "", false),
				func(wi *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, bool) {
					return wi, scopes.MatchScope(scopeFilter, wi.GetScope())
				},
			))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *workloadidentityv1pb.WorkloadIdentity {
			// Only unscoped deletes arrive as a ResourceHeader (scoped deletes
			// carry a skeleton WorkloadIdentity with the scope set), so the
			// rebuilt skeleton keys on the bare name.
			return workloadidentityv1pb.WorkloadIdentity_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// RangeWorkloadIdentities returns WorkloadIdentity resources within the range
// [start, end), ordered by the given sort field and direction. Supported sort
// fields are "name" (the default) and "spiffe_id".
func (c *Cache) RangeWorkloadIdentities(
	ctx context.Context,
	start, end string,
	sortField services.WorkloadIdentitySortField,
	sortDesc bool,
) iter.Seq2[*workloadidentityv1pb.WorkloadIdentity, error] {
	index, keyFn, err := workloadIdentitySortIndex(sortField)
	if err != nil {
		return stream.Fail[*workloadidentityv1pb.WorkloadIdentity](trace.Wrap(err))
	}

	lister := genericLister[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		cache:      c,
		collection: c.collections.workloadIdentity,
		index:      index,
		isDesc:     sortDesc,
		upstreamList: func(ctx context.Context, pageSize int, nextToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
			// When the cache is unhealthy, fall back to collecting a page from
			// the upstream backend's range, applying the collection's scope
			// filter so cache health doesn't change the visible set. The
			// backend only supports name-ordered ascending iteration, so a
			// spiffe_id or descending range surfaces an error here.
			scopeFilter := c.collections.workloadIdentity.watch.ScopeFilter.ToProto()
			return generic.CollectPageAndCursor(
				stream.FilterMap(
					c.Config.WorkloadIdentity.RangeWorkloadIdentities(ctx, nextToken, "", sortField, sortDesc),
					func(wi *workloadidentityv1pb.WorkloadIdentity) (*workloadidentityv1pb.WorkloadIdentity, bool) {
						return wi, scopes.MatchScope(scopeFilter, wi.GetScope())
					},
				),
				pageSize,
				keyFn,
			)
		},
		nextToken: keyFn,
	}

	return func(yield func(*workloadidentityv1pb.WorkloadIdentity, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeWorkloadIdentities")
		defer span.End()

		for wi, err := range lister.Range(ctx, start, end) {
			if !yield(wi, err) {
				return
			}

			if err != nil {
				return
			}
		}
	}
}

// GetWorkloadIdentity returns a single WorkloadIdentity by the name and scope in
// the request.
func (c *Cache) GetWorkloadIdentity(ctx context.Context, req *workloadidentityv1pb.GetWorkloadIdentityRequest) (*workloadidentityv1pb.WorkloadIdentity, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWorkloadIdentity")
	defer span.End()

	// The name index is keyed by resource cursor, so look up by the cursor for
	// the requested scope-qualified name. The scope is caller input, so weakly
	// validate it before deriving the cursor.
	if req.GetScope() != "" {
		if err := scopes.WeakValidate(req.GetScope()); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	cursor := scopes.MakeResourceCursor(req.GetScope(), req.GetName())

	getter := genericGetter[*workloadidentityv1pb.WorkloadIdentity, workloadIdentityIndex]{
		cache:      c,
		collection: c.collections.workloadIdentity,
		index:      workloadIdentityNameIndex,
		upstreamGet: func(ctx context.Context, cursor string) (*workloadidentityv1pb.WorkloadIdentity, error) {
			wi, err := c.Config.WorkloadIdentity.GetWorkloadIdentity(ctx, req)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Mirror the healthy path: an identity outside the collection's
			// scope filter is never in the store.
			if !scopes.MatchScope(c.collections.workloadIdentity.watch.ScopeFilter.ToProto(), wi.GetScope()) {
				return nil, trace.NotFound("%q %q does not exist", types.KindWorkloadIdentity, cursor)
			}
			return wi, nil
		},
	}
	out, err := getter.get(ctx, cursor)
	return out, trace.Wrap(err)
}

// workloadIdentitySortIndex maps a sort field to its store index and key
// function.
func workloadIdentitySortIndex(sortField services.WorkloadIdentitySortField) (workloadIdentityIndex, func(*workloadidentityv1pb.WorkloadIdentity) string, error) {
	keyFn, err := services.WorkloadIdentityKey(sortField)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	switch sortField {
	case "", services.WorkloadIdentitySortFieldName:
		return workloadIdentityNameIndex, keyFn, nil
	case services.WorkloadIdentitySortFieldSPIFFEID:
		return workloadIdentitySpiffeIDIndex, keyFn, nil
	default:
		// This branch is technically unreachable as WorkloadIdentityKey has
		// already checked.
		return "", nil, trace.BadParameter("unsupported sort %q", sortField)
	}
}
