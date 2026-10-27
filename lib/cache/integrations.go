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

	integrationpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

type integrationIndex string

const integrationNameIndex integrationIndex = "name"

func newIntegrationCollection(upstream services.Integrations, w types.WatchKind) (*collection[types.Integration, integrationIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Integrations")
	}

	return &collection[types.Integration, integrationIndex]{
		store: newStore(
			types.KindIntegration,
			types.Integration.Clone,
			map[integrationIndex]func(types.Integration) string{
				integrationNameIndex: services.GetCursorForIntegration,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Integration, error) {
			out, err := stream.Collect(upstream.RangeIntegrations(ctx, integrationpb.ListIntegrationsRequest_builder{
				ScopeFilter: w.ScopeFilter.ToProto(),
			}.Build()))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Integration {
			return &types.IntegrationV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// ListIntegrations returns a paginated list of unscoped Integration resources.
//
// This predates scope filtering and only returns unscoped integrations. Prefer
// ListIntegrationsV2, which takes a scope filter.
func (c *Cache) ListIntegrations(ctx context.Context, pageSize int, pageToken string) ([]types.Integration, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListIntegrations")
	defer span.End()

	unscoped := scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build()
	out, next, err := c.integrationLister(unscoped).list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListIntegrationsV2 returns a page of Integration resources respecting scope filters.
func (c *Cache) ListIntegrationsV2(ctx context.Context, req *integrationpb.ListIntegrationsRequest) ([]types.Integration, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListIntegrationsV2")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return nil, "", trace.Wrap(err)
	}

	out, next, err := c.integrationLister(scopeFilter).list(ctx, int(req.GetLimit()), req.GetNextKey())
	return out, next, trace.Wrap(err)
}

// RangeIntegrations returns Integration resources within the range [start, end) respecting scope filters.
func (c *Cache) RangeIntegrations(ctx context.Context, req *integrationpb.ListIntegrationsRequest) iter.Seq2[types.Integration, error] {
	ctx, span := c.Tracer.Start(ctx, "cache/RangeIntegrations")
	defer span.End()

	scopeFilter := req.GetScopeFilter()
	if err := scopes.ValidateFilter(scopeFilter); err != nil {
		return stream.Fail[types.Integration](trace.Wrap(err))
	}

	return c.integrationLister(scopeFilter).Range(ctx, req.GetNextKey(), "")
}

func (c *Cache) integrationLister(scopeFilter *scopesv1.Filter) genericLister[types.Integration, integrationIndex] {
	return genericLister[types.Integration, integrationIndex]{
		cache:      c,
		collection: c.collections.integrations,
		index:      integrationNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]types.Integration, string, error) {
			return c.Config.Integrations.ListIntegrationsV2(ctx, integrationpb.ListIntegrationsRequest_builder{
				Limit:       int32(pageSize),
				NextKey:     pageToken,
				ScopeFilter: scopeFilter,
			}.Build())
		},
		nextToken: services.GetCursorForIntegration,
		filter: func(ig types.Integration) bool {
			return scopes.MatchScope(scopeFilter, ig.GetScope())
		},
	}
}

// GetIntegration returns the specified unscoped Integration resource.
//
// This predates scoping and only resolves unscoped integrations. Prefer GetIntegrationV2.
func (c *Cache) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	return c.GetIntegrationV2(ctx, integrationpb.GetIntegrationRequest_builder{Name: name}.Build())
}

// GetIntegrationV2 returns the specified scoped or unscoped Integration resource.
func (c *Cache) GetIntegrationV2(ctx context.Context, req *integrationpb.GetIntegrationRequest) (types.Integration, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetIntegrationV2")
	defer span.End()

	getter := genericGetter[types.Integration, integrationIndex]{
		cache:      c,
		collection: c.collections.integrations,
		index:      integrationNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (types.Integration, error) {
			return c.Config.Integrations.GetIntegrationV2(ctx, req)
		},
	}
	out, err := getter.get(ctx, scopes.MakeResourceCursor(req.GetScope(), req.GetName()))
	return out, trace.Wrap(err)
}
