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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func newStaticTokensCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.StaticTokens, *singletonStore[types.StaticTokens], *staticTokensUpstream], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfig")
	}

	return &collection[types.StaticTokens, *singletonStore[types.StaticTokens], *staticTokensUpstream]{
		store:    &singletonStore[types.StaticTokens]{},
		upstream: &staticTokensUpstream{ClusterConfiguration: c},
		watch:    w,
	}, nil

}

type staticTokensUpstream struct {
	services.ClusterConfiguration
}

func (s staticTokensUpstream) getAll(ctx context.Context, loadSecrets bool) ([]types.StaticTokens, error) {
	token, err := s.ClusterConfiguration.GetStaticTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []types.StaticTokens{token}, nil
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *Cache) GetStaticTokens() (types.StaticTokens, error) {
	ctx, span := c.Tracer.Start(context.TODO(), "cache/GetStaticTokens")
	defer span.End()

	st, err := readCachedResource(
		ctx,
		c,
		c.collections.staticTokens,
		func(_ context.Context, store *singletonStore[types.StaticTokens]) (types.StaticTokens, error) {
			st, err := store.get()
			return st, trace.Wrap(err)
		},
		func(_ context.Context, upstream *staticTokensUpstream) (types.StaticTokens, error) {
			st, err := upstream.GetStaticTokens()
			return st, trace.Wrap(err)
		},
	)
	return st, trace.Wrap(err)
}
