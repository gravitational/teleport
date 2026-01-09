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
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type staticScopedTokensIndex string

const staticScopedTokensNameIndex staticScopedTokensIndex = "name"

func newStaticScopedTokensCollection(upstream services.ClusterConfiguration, w types.WatchKind) (*collection[*joiningv1.StaticScopedTokens, staticScopedTokensIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfig")
	}

	return &collection[*joiningv1.StaticScopedTokens, staticScopedTokensIndex]{
		store: newStore(
			types.KindScopedToken,
			proto.CloneOf[*joiningv1.StaticScopedTokens],
			map[staticScopedTokensIndex]func(*joiningv1.StaticScopedTokens) string{
				staticScopedTokensNameIndex: func(s *joiningv1.StaticScopedTokens) string {
					return s.GetMetadata().GetName()
				},
			},
		),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*joiningv1.StaticScopedTokens, error) {
			tokens, err := upstream.GetStaticScopedTokens(ctx)
			return []*joiningv1.StaticScopedTokens{tokens}, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *joiningv1.StaticScopedTokens {
			return &joiningv1.StaticScopedTokens{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: types.MetaNameStaticScopedTokens,
				},
			}
		},
		watch: w,
	}, nil
}

// GetStaticScopedTokens gets the list of static scoped tokens used to provision nodes.
func (c *Cache) GetStaticScopedTokens(ctx context.Context) (*joiningv1.StaticScopedTokens, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetStaticScopedTokens")
	defer span.End()

	getter := genericGetter[*joiningv1.StaticScopedTokens, staticScopedTokensIndex]{
		cache:      c,
		collection: c.collections.staticScopedTokens,
		index:      staticScopedTokensNameIndex,
		upstreamGet: func(ctx context.Context, ident string) (*joiningv1.StaticScopedTokens, error) {
			tokens, err := c.Config.ClusterConfig.GetStaticScopedTokens(ctx)
			return tokens, trace.Wrap(err)
		},
	}

	out, err := getter.get(ctx, types.MetaNameStaticScopedTokens)
	return out, trace.Wrap(err)
}
