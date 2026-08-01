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
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type staticTokensIndex string

const staticTokensNameIndex staticTokensIndex = "name"

func newStaticTokensCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.StaticTokens, staticTokensIndex], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfig")
	}

	return &collection[types.StaticTokens, staticTokensIndex]{
		store: newStore(
			types.KindStaticTokens,
			types.StaticTokens.Clone,
			map[staticTokensIndex]func(types.StaticTokens) string{
				staticTokensNameIndex: types.StaticTokens.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.StaticTokens, error) {
			token, err := c.GetStaticTokens(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []types.StaticTokens{token}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.StaticTokens {
			return &types.StaticTokensV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: types.MetaNameStaticTokens,
				},
			}
		},
		watch: w,
	}, nil
}

// staticTokensCollection provides read access to the cached static tokens.
// Its exported methods are promoted onto every topology cache that embeds
// it; the read is implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type staticTokensCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.ClusterConfiguration
	col      *collection[types.StaticTokens, staticTokensIndex]
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c staticTokensCollection) GetStaticTokens(ctx context.Context) (types.StaticTokens, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetStaticTokens")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		st, err := rg.store.get(staticTokensNameIndex, types.MetaNameStaticTokens)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return st.Clone(), nil
	}

	st, err := c.upstream.GetStaticTokens(ctx)
	return st, trace.Wrap(err)
}

// GetStaticTokens gets the list of static tokens used to provision nodes.
func (c *Cache) GetStaticTokens(ctx context.Context) (types.StaticTokens, error) {
	return staticTokensCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.ClusterConfig,
		col:      c.collections.staticTokens,
	}.GetStaticTokens(ctx)
}

type provisionTokenIndex string

const provisionTokenStoreNameIndex provisionTokenIndex = "name"

func newProvisionTokensCollection(p services.Provisioner, w types.WatchKind) (*collection[types.ProvisionToken, provisionTokenIndex], error) {
	if p == nil {
		return nil, trace.BadParameter("missing parameter Provisioner")
	}

	return &collection[types.ProvisionToken, provisionTokenIndex]{
		store: newStore(
			types.KindToken,
			types.ProvisionToken.Clone,
			map[provisionTokenIndex]func(types.ProvisionToken) string{
				provisionTokenStoreNameIndex: types.ProvisionToken.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.ProvisionToken, error) {
			tokens, err := stream.Collect(
				clientutils.Resources(
					ctx,
					// ListProvisionTokens take too many arguments for [clientutils.Resources]
					// so we wrap it to get the usual paginated signature.
					func(ctx context.Context, pageSize int, pageKey string) ([]types.ProvisionToken, string, error) {
						return p.ListProvisionTokens(ctx, pageSize, pageKey, nil, "")
					},
				),
			)
			return tokens, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.ProvisionToken {
			return &types.ProvisionTokenV2{
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

// provisionTokensCollection provides read access to cached provision tokens.
// Its exported methods are promoted onto every topology cache that embeds
// it; the reads are implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type provisionTokensCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.Provisioner
	col      *collection[types.ProvisionToken, provisionTokenIndex]
}

// GetTokens returns all active (non-expired) provisioning tokens
// Deprecated: use [ListProvisionTokens] istead.
// TODO(hugoShaka): DELETE IN 19.0.0
func (c provisionTokensCollection) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetTokens")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		tokens, err := c.upstream.GetTokens(ctx)
		return tokens, trace.Wrap(err)
	}

	tokens := make([]types.ProvisionToken, 0, rg.store.len())
	for t := range rg.store.resources(provisionTokenStoreNameIndex, "", "") {
		tokens = append(tokens, t.Clone())
	}

	return tokens, nil
}

// GetTokens returns all active (non-expired) provisioning tokens
// Deprecated: use [ListProvisionTokens] istead.
// TODO(hugoShaka): DELETE IN 19.0.0
func (c *Cache) GetTokens(ctx context.Context) ([]types.ProvisionToken, error) {
	return provisionTokensCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Provisioner,
		col:      c.collections.provisionTokens,
	}.GetTokens(ctx)
}

// GetToken finds and returns token by ID
func (c provisionTokensCollection) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetToken")
	defer span.End()

	rg, err := acquireGuard(c.engine, c.col)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		token, err := c.upstream.GetToken(ctx, name)
		return token, trace.Wrap(err)
	}

	t, err := rg.store.get(provisionTokenStoreNameIndex, name)
	if err != nil {
		// release read lock early
		rg.Release()

		// fallback is sane because method is never used
		// in construction of derivative caches.
		if trace.IsNotFound(err) {
			if token, err := c.upstream.GetToken(ctx, name); err == nil {
				return token, nil
			}
		}
		return nil, trace.Wrap(err)
	}

	return t.Clone(), nil
}

// GetToken finds and returns token by ID
func (c *Cache) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	return provisionTokensCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Provisioner,
		col:      c.collections.provisionTokens,
	}.GetToken(ctx, name)
}

// ListProvisionTokens returns a paginated list of provision tokens. Items can
// be filtered by role and bot name. Tokens with ANY of the provided roles are
// returned. If a bot name is provided, only tokens having a role of Bot are
// returned.
func (c provisionTokensCollection) ListProvisionTokens(ctx context.Context, pageSize int, pageToken string, anyRoles types.SystemRoles, botName string) ([]types.ProvisionToken, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListProvisionTokens")
	defer span.End()

	lister := genericLister[types.ProvisionToken, provisionTokenIndex]{
		engine:          c.engine,
		collection:      c.col,
		index:           provisionTokenStoreNameIndex,
		isDesc:          false,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.ProvisionToken, string, error) {
			return c.upstream.ListProvisionTokens(ctx, pageSize, pageToken, anyRoles, botName)
		},
		filter: func(t types.ProvisionToken) bool {
			return local.MatchToken(t, anyRoles, botName)
		},
		nextToken: func(t types.ProvisionToken) string {
			return t.GetName()
		},
	}
	out, next, err := lister.list(ctx,
		pageSize,
		pageToken,
	)
	return out, next, trace.Wrap(err)
}

// ListProvisionTokens returns a paginated list of provision tokens. Items can
// be filtered by role and bot name. Tokens with ANY of the provided roles are
// returned. If a bot name is provided, only tokens having a role of Bot are
// returned.
func (c *Cache) ListProvisionTokens(ctx context.Context, pageSize int, pageToken string, anyRoles types.SystemRoles, botName string) ([]types.ProvisionToken, string, error) {
	return provisionTokensCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.Provisioner,
		col:      c.collections.provisionTokens,
	}.ListProvisionTokens(ctx, pageSize, pageToken, anyRoles, botName)
}
