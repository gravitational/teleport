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

package cache

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type principalStateIndex struct{}

var principalStateNameIndex principalStateIndex

func principalStateNameIndexKey(downstream services.DownstreamID, id services.ProvisioningStateID) string {
	return string(downstream) + "/" + string(id)
}

func principalStateNameIndexKeyForPrincipalState(ps *provisioningv1.PrincipalState) string {
	return principalStateNameIndexKey(services.DownstreamID(ps.GetSpec().GetDownstreamId()), services.ProvisioningStateID(ps.GetMetadata().GetName()))
}

func newPrincipalStateCollection(upstream services.ProvisioningStates, w types.WatchKind) (*collection[*provisioningv1.PrincipalState, principalStateIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter ProvisioningStates")
	}

	return &collection[*provisioningv1.PrincipalState, principalStateIndex]{
		store: newStore(
			types.KindProvisioningPrincipalState,
			proto.CloneOf[*provisioningv1.PrincipalState],
			map[principalStateIndex]func(*provisioningv1.PrincipalState) string{
				principalStateNameIndex: principalStateNameIndexKeyForPrincipalState,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*provisioningv1.PrincipalState, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListProvisioningStatesForAllDownstreams))
			return out, trace.Wrap(err)
		},
		watch: w,
	}, nil
}

func (c *Cache) GetProvisioningState(ctx context.Context, downstream services.DownstreamID, id services.ProvisioningStateID) (*provisioningv1.PrincipalState, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetProvisioningState")
	defer span.End()

	getter := genericGetter[*provisioningv1.PrincipalState, principalStateIndex]{
		cache:      c,
		collection: c.collections.provisioningStates,
		index:      principalStateNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*provisioningv1.PrincipalState, error) {
			out, err := c.Config.ProvisioningStates.GetProvisioningState(ctx, downstream, id)
			return out, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, principalStateNameIndexKey(downstream, id))
	return out, trace.Wrap(err)
}

func (c *Cache) ListProvisioningStatesForAllDownstreams(ctx context.Context, pageSize int, pageToken string) ([]*provisioningv1.PrincipalState, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListProvisioningStatesForAllDownstreams")
	defer span.End()

	out, next, err := c.ListProvisioningStatesForAllDownstreams2(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

func (c *Cache) ListProvisioningStatesForAllDownstreams2(ctx context.Context, pageSize int, pageToken string) ([]*provisioningv1.PrincipalState, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListProvisioningStatesForAllDownstreams2")
	defer span.End()

	lister := genericLister[*provisioningv1.PrincipalState, principalStateIndex]{
		cache:        c,
		collection:   c.collections.provisioningStates,
		index:        principalStateNameIndex,
		upstreamList: c.Config.ProvisioningStates.ListProvisioningStatesForAllDownstreams,
		nextToken:    principalStateNameIndexKeyForPrincipalState,
	}

	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}
