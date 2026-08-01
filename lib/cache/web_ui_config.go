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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/services"
)

type webUIConfigIndex string

const webUIConfigNameIndex webUIConfigIndex = "name"

func newWebUIConfigCollection(upstream services.ClusterConfiguration, w types.WatchKind) (*collection[types.UIConfig, webUIConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.UIConfig, webUIConfigIndex]{
		store: newStore(
			types.KindUIConfig,
			types.UIConfig.Clone,
			map[webUIConfigIndex]func(types.UIConfig) string{
				webUIConfigNameIndex: types.UIConfig.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.UIConfig, error) {
			uiConfig, err := upstream.GetUIConfig(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []types.UIConfig{uiConfig}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.UIConfig {
			return &types.UIConfigV1{
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

// webUIConfigCollection provides read access to the cached web UI config.
// Its exported methods are promoted onto every topology cache that embeds
// it; the read is implemented exactly once here. It is a stateless value
// assembled inline by each of its consumers so that no shared scaffolding
// couples their lifetimes.
type webUIConfigCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.ClusterConfiguration
	col      *collection[types.UIConfig, webUIConfigIndex]
}

func (c webUIConfigCollection) GetUIConfig(ctx context.Context) (types.UIConfig, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetUIConfig")
	defer span.End()

	getter := genericGetter[types.UIConfig, webUIConfigIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      webUIConfigNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.UIConfig, error) {
			cfg, err := c.upstream.GetUIConfig(ctx)
			return cfg, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, types.MetaNameUIConfig)
	return out, trace.Wrap(err)
}

func (c *Cache) GetUIConfig(ctx context.Context) (types.UIConfig, error) {
	return webUIConfigCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.ClusterConfig,
		col:      c.collections.uiConfigs,
	}.GetUIConfig(ctx)
}
