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
)

type webTokenIndex string

const webTokenNameIndex webTokenIndex = "name"

func newWebTokenCollection(upstream types.WebTokenInterface, w types.WatchKind) (*collection[types.WebToken, webTokenIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WebTokenInterface")
	}

	return &collection[types.WebToken, webTokenIndex]{
		store: newStore(
			types.WebToken.Clone,
			map[webTokenIndex]func(types.WebToken) string{
				webTokenNameIndex: types.WebToken.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebToken, error) {
			installers, err := upstream.List(ctx)
			return installers, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WebToken {
			return &types.WebTokenV3{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetWebToken gets a web token.
func (c *Cache) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetWebToken")
	defer span.End()

	getter := genericGetter[types.WebToken, webTokenIndex]{
		cache:      c,
		collection: c.collections.webTokens,
		index:      webTokenNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebToken, error) {
			token, err := c.Config.WebToken.Get(ctx, req)
			return token, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.Token)
	return out, trace.Wrap(err)
}

func (c *Cache) GetWebTokens(ctx context.Context) ([]types.WebToken, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInstallers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.webTokens)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		users, err := c.Config.WebToken.List(ctx)
		return users, trace.Wrap(err)
	}

	tokens := make([]types.WebToken, 0, rg.store.len())
	for token := range rg.store.resources(webTokenNameIndex, "", "") {
		tokens = append(tokens, token.Clone())
	}

	return tokens, nil
}
