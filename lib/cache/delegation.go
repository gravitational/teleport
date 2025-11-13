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

	"github.com/gravitational/teleport/api/defaults"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type delegationProfileIndex string

const delegationProfileNameIndex delegationProfileIndex = "name"

func newDelegationProfileCollection(upstream services.DelegationProfiles, w types.WatchKind) (*collection[*delegationv1.DelegationProfile, delegationProfileIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter DelegationProfiles")
	}

	return &collection[*delegationv1.DelegationProfile, delegationProfileIndex]{
		store: newStore(
			string(types.KindDelegationProfile),
			proto.CloneOf[*delegationv1.DelegationProfile],
			map[delegationProfileIndex]func(*delegationv1.DelegationProfile) string{
				delegationProfileNameIndex: func(p *delegationv1.DelegationProfile) string {
					return p.GetMetadata().GetName()
				},
			},
		),
		fetcher: func(ctx context.Context, _ bool) ([]*delegationv1.DelegationProfile, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListDelegationProfiles))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *delegationv1.DelegationProfile {
			return &delegationv1.DelegationProfile{
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

// GetDelegationProfile gets a DelegationProfile by name.
func (c *Cache) GetDelegationProfile(
	ctx context.Context, name string,
) (*delegationv1.DelegationProfile, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetDelegationProfile")
	defer span.End()

	getter := genericGetter[*delegationv1.DelegationProfile, delegationProfileIndex]{
		cache:      c,
		collection: c.collections.delegationProfiles,
		index:      delegationProfileNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*delegationv1.DelegationProfile, error) {
			return c.Config.DelegationProfiles.GetDelegationProfile(ctx, name)
		},
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListDelegationProfiles lists all DelegationProfile resources using Google
// style pagination.
func (c *Cache) ListDelegationProfiles(
	ctx context.Context, pageSize int, lastToken string,
) ([]*delegationv1.DelegationProfile, string, error) {
	lister := genericLister[*delegationv1.DelegationProfile, delegationProfileIndex]{
		cache:           c,
		collection:      c.collections.delegationProfiles,
		index:           delegationProfileNameIndex,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList: func(ctx context.Context, limit int, start string) ([]*delegationv1.DelegationProfile, string, error) {
			return c.Config.DelegationProfiles.ListDelegationProfiles(ctx, limit, start)
		},
		nextToken: func(prof *delegationv1.DelegationProfile) string {
			return prof.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx,
		pageSize,
		lastToken,
	)
	return out, next, trace.Wrap(err)
}
