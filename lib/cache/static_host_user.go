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
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type staticHostUserIndex string

const staticHostUserNameIndex staticHostUserIndex = "name"

func newStaticHostUserCollection(upstream services.StaticHostUser, w types.WatchKind) (*collection[*userprovisioningv2.StaticHostUser, staticHostUserIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter StaticHostUser")
	}

	return &collection[*userprovisioningv2.StaticHostUser, staticHostUserIndex]{
		store: newStore(
			proto.CloneOf[*userprovisioningv2.StaticHostUser],
			map[staticHostUserIndex]func(*userprovisioningv2.StaticHostUser) string{
				staticHostUserNameIndex: func(shu *userprovisioningv2.StaticHostUser) string {
					return shu.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*userprovisioningv2.StaticHostUser, error) {
			var startKey string
			var allUsers []*userprovisioningv2.StaticHostUser

			for {
				users, nextKey, err := upstream.ListStaticHostUsers(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				allUsers = append(allUsers, users...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}
			return allUsers, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *userprovisioningv2.StaticHostUser {
			return &userprovisioningv2.StaticHostUser{
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

// ListStaticHostUsers lists static host users.
func (c *Cache) ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningv2.StaticHostUser, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListStaticHostUsers")
	defer span.End()

	lister := genericLister[*userprovisioningv2.StaticHostUser, staticHostUserIndex]{
		cache:        c,
		collection:   c.collections.staticHostUsers,
		index:        staticHostUserNameIndex,
		upstreamList: c.Config.StaticHostUsers.ListStaticHostUsers,
		nextToken: func(t *userprovisioningv2.StaticHostUser) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetStaticHostUser returns a static host user by name.
func (c *Cache) GetStaticHostUser(ctx context.Context, name string) (*userprovisioningv2.StaticHostUser, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetStaticHostUser")
	defer span.End()

	getter := genericGetter[*userprovisioningv2.StaticHostUser, staticHostUserIndex]{
		cache:       c,
		collection:  c.collections.staticHostUsers,
		index:       staticHostUserNameIndex,
		upstreamGet: c.Config.StaticHostUsers.GetStaticHostUser,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
