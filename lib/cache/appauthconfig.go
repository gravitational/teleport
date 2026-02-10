/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package cache

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/defaults"
	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type appAuthConfigIndex string

const appAuthConfigNameIndex appAuthConfigIndex = "name"

func newAppAuthConfigCollection(upstream services.AppAuthConfigReader, w types.WatchKind) (*collection[*appauthconfigv1.AppAuthConfig, appAuthConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AppAuthConfigReader")
	}

	return &collection[*appauthconfigv1.AppAuthConfig, appAuthConfigIndex]{
		store: newStore(
			types.KindAppAuthConfig,
			proto.CloneOf[*appauthconfigv1.AppAuthConfig],
			map[appAuthConfigIndex]func(*appauthconfigv1.AppAuthConfig) string{
				appAuthConfigNameIndex: func(r *appauthconfigv1.AppAuthConfig) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*appauthconfigv1.AppAuthConfig, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListAppAuthConfigs))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *appauthconfigv1.AppAuthConfig {
			return &appauthconfigv1.AppAuthConfig{
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

// ListAppAuthConfigs lists app auth configs with pagination.
func (c *Cache) ListAppAuthConfigs(ctx context.Context, pageSize int, nextToken string) ([]*appauthconfigv1.AppAuthConfig, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListAppAuthConfigs")
	defer span.End()

	lister := genericLister[*appauthconfigv1.AppAuthConfig, appAuthConfigIndex]{
		cache:           c,
		collection:      c.collections.appAuthConfig,
		index:           appAuthConfigNameIndex,
		defaultPageSize: defaults.DefaultChunkSize,
		upstreamList:    c.Config.AppAuthConfig.ListAppAuthConfigs,
		nextToken: func(t *appauthconfigv1.AppAuthConfig) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx,
		pageSize,
		nextToken,
	)
	return out, next, trace.Wrap(err)
}

// GetAppAuthConfig fetches an app auth config by name.
func (c *Cache) GetAppAuthConfig(ctx context.Context, name string) (*appauthconfigv1.AppAuthConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAppAuthConfig")
	defer span.End()

	getter := genericGetter[*appauthconfigv1.AppAuthConfig, appAuthConfigIndex]{
		cache:       c,
		collection:  c.collections.appAuthConfig,
		index:       appAuthConfigNameIndex,
		upstreamGet: c.Config.AppAuthConfig.GetAppAuthConfig,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
