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
	"iter"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/services"
)

type webTokenIndex string

const webTokenNameIndex webTokenIndex = "name"

func newWebTokenCollection(upstream services.WebToken, w types.WatchKind) (*collection[types.WebToken, webTokenIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter WebTokenInterface")
	}

	return &collection[types.WebToken, webTokenIndex]{
		store: newStore(
			types.KindWebToken,
			types.WebToken.Clone,
			map[webTokenIndex]func(types.WebToken) string{
				webTokenNameIndex: types.WebToken.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebToken, error) {
			// TODO(lokraszewski): DELETE IN v21.0.0, replace with regular collect.
			tokens, err := clientutils.CollectWithFallback(ctx, upstream.ListWebTokens, upstream.GetWebTokens)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return tokens, nil
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

// webTokenCollection provides read access to the cached web tokens. Its
// exported methods are promoted onto every topology cache that embeds it;
// the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type webTokenCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.WebToken
	col      *collection[types.WebToken, webTokenIndex]
}

// GetWebToken gets a web token.
func (c webTokenCollection) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetWebToken")
	defer span.End()

	getter := genericGetter[types.WebToken, webTokenIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      webTokenNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebToken, error) {
			token, err := c.upstream.GetWebToken(ctx, req)
			return token, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.Token)
	return out, trace.Wrap(err)
}

func (c webTokenCollection) GetWebTokens(ctx context.Context) ([]types.WebToken, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetWebTokens")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		users, err := c.upstream.GetWebTokens(ctx)
		return users, trace.Wrap(err)
	}

	tokens := make([]types.WebToken, 0, rg.store.len())
	for token := range rg.store.resources(webTokenNameIndex, "", "") {
		tokens = append(tokens, token.Clone())
	}

	return tokens, nil
}

// ListWebTokens returns a page of web tokens
func (c webTokenCollection) ListWebTokens(ctx context.Context, limit int, start string) ([]types.WebToken, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListWebTokens")
	defer span.End()

	lister := genericLister[types.WebToken, webTokenIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        webTokenNameIndex,
		upstreamList: c.upstream.ListWebTokens,
		nextToken:    types.WebToken.GetName,
	}
	out, next, err := lister.list(ctx, limit, start)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return out, next, nil
}

// RangeWebTokens returns web tokens within the range [start, end).
func (c webTokenCollection) RangeWebTokens(ctx context.Context, start, end string) iter.Seq2[types.WebToken, error] {
	lister := genericLister[types.WebToken, webTokenIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        webTokenNameIndex,
		upstreamList: c.upstream.ListWebTokens,
		nextToken:    types.WebToken.GetName,
		// TODO(lokraszewski): DELETE IN v21.0.0
		fallbackGetter: c.upstream.GetWebTokens,
	}

	return func(yield func(types.WebToken, error) bool) {
		ctx, span := c.tracer.Start(ctx, "cache/RangeWebTokens")
		defer span.End()

		for token, err := range lister.RangeWithFallback(ctx, start, end) {
			if !yield(token, err) {
				return
			}

			if err != nil {
				return
			}
		}
	}
}

// GetWebToken gets a web token.
func (c *Cache) GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error) {
	return webTokenCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.WebToken,
		col:      c.collections.webTokens,
	}.GetWebToken(ctx, req)
}

func (c *Cache) GetWebTokens(ctx context.Context) ([]types.WebToken, error) {
	return webTokenCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.WebToken,
		col:      c.collections.webTokens,
	}.GetWebTokens(ctx)
}

// ListWebTokens returns a page of web tokens
func (c *Cache) ListWebTokens(ctx context.Context, limit int, start string) ([]types.WebToken, string, error) {
	return webTokenCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.WebToken,
		col:      c.collections.webTokens,
	}.ListWebTokens(ctx, limit, start)
}

// RangeWebTokens returns web tokens within the range [start, end).
func (c *Cache) RangeWebTokens(ctx context.Context, start, end string) iter.Seq2[types.WebToken, error] {
	return webTokenCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.WebToken,
		col:      c.collections.webTokens,
	}.RangeWebTokens(ctx, start, end)
}
