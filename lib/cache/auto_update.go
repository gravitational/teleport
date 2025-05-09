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

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type autoUpdateConfigIndex string

const autoUpdateConfigNameIndex autoUpdateConfigIndex = "name"

func newAutoUpdateConfigCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateConfig, autoUpdateConfigIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateServiceGetter")
	}

	return &collection[*autoupdatev1.AutoUpdateConfig, autoUpdateConfigIndex]{
		store: newStore(map[autoUpdateConfigIndex]func(*autoupdatev1.AutoUpdateConfig) string{
			autoUpdateConfigNameIndex: func(r *autoupdatev1.AutoUpdateConfig) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*autoupdatev1.AutoUpdateConfig, error) {
			cfg, err := upstream.GetAutoUpdateConfig(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []*autoupdatev1.AutoUpdateConfig{cfg}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *autoupdatev1.AutoUpdateConfig {
			return &autoupdatev1.AutoUpdateConfig{
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

type autoUpdateCacheKey struct {
	kind string
}

// GetAutoUpdateConfig gets the AutoUpdateConfig from the backend.
func (c *Cache) GetAutoUpdateConfig(ctx context.Context) (*autoupdatev1.AutoUpdateConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAutoUpdateConfig")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateConfig, autoUpdateConfigIndex]{
		cache:      c,
		collection: c.collections.autoUpdateConfig,
		index:      autoUpdateConfigNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*autoupdatev1.AutoUpdateConfig, error) {
			cachedConfig, err := utils.FnCacheGet(ctx, c.fnCache, autoUpdateCacheKey{"config"}, func(ctx context.Context) (*autoupdatev1.AutoUpdateConfig, error) {
				cfg, err := c.Config.AutoUpdateService.GetAutoUpdateConfig(ctx)
				return cfg, trace.Wrap(err)
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return apiutils.CloneProtoMsg(cachedConfig), nil
		},
		clone: apiutils.CloneProtoMsg[*autoupdatev1.AutoUpdateConfig],
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateConfig)
	return out, trace.Wrap(err)
}

type autoUpdateVersionIndex string

const autoUpdateVersionNameIndex autoUpdateVersionIndex = "name"

func newAutoUpdateVersionCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateServiceGetter")
	}

	return &collection[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex]{
		store: newStore(map[autoUpdateVersionIndex]func(*autoupdatev1.AutoUpdateVersion) string{
			autoUpdateVersionNameIndex: func(r *autoupdatev1.AutoUpdateVersion) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*autoupdatev1.AutoUpdateVersion, error) {
			version, err := upstream.GetAutoUpdateVersion(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []*autoupdatev1.AutoUpdateVersion{version}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *autoupdatev1.AutoUpdateVersion {
			return &autoupdatev1.AutoUpdateVersion{
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

// GetAutoUpdateVersion gets the AutoUpdateVersion from the backend.
func (c *Cache) GetAutoUpdateVersion(ctx context.Context) (*autoupdatev1.AutoUpdateVersion, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAutoUpdateVersion")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex]{
		cache:      c,
		collection: c.collections.autoUpdateVerion,
		index:      autoUpdateVersionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*autoupdatev1.AutoUpdateVersion, error) {
			cachedVersion, err := utils.FnCacheGet(ctx, c.fnCache, autoUpdateCacheKey{"version"}, func(ctx context.Context) (*autoupdatev1.AutoUpdateVersion, error) {
				version, err := c.Config.AutoUpdateService.GetAutoUpdateVersion(ctx)
				return version, trace.Wrap(err)
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return apiutils.CloneProtoMsg(cachedVersion), nil
		},
		clone: apiutils.CloneProtoMsg[*autoupdatev1.AutoUpdateVersion],
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateVersion)
	return out, trace.Wrap(err)
}

type autoUpdateAgentRolloutIndex string

const autoUpdateAgentRolloutNameIndex autoUpdateAgentRolloutIndex = "name"

func newAutoUpdateRolloutCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateServiceGetter")
	}

	return &collection[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex]{
		store: newStore(map[autoUpdateAgentRolloutIndex]func(*autoupdatev1.AutoUpdateAgentRollout) string{
			autoUpdateAgentRolloutNameIndex: func(r *autoupdatev1.AutoUpdateAgentRollout) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*autoupdatev1.AutoUpdateAgentRollout, error) {
			rollout, err := upstream.GetAutoUpdateAgentRollout(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []*autoupdatev1.AutoUpdateAgentRollout{rollout}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *autoupdatev1.AutoUpdateAgentRollout {
			return &autoupdatev1.AutoUpdateAgentRollout{
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

// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout from the backend.
func (c *Cache) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdatev1.AutoUpdateAgentRollout, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAutoUpdateAgentRollout")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex]{
		cache:      c,
		collection: c.collections.autoUpdateRollout,
		index:      autoUpdateAgentRolloutNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*autoupdatev1.AutoUpdateAgentRollout, error) {
			cachedRollout, err := utils.FnCacheGet(ctx, c.fnCache, autoUpdateCacheKey{"rollout"}, func(ctx context.Context) (*autoupdatev1.AutoUpdateAgentRollout, error) {
				rollout, err := c.Config.AutoUpdateService.GetAutoUpdateAgentRollout(ctx)
				return rollout, trace.Wrap(err)
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return apiutils.CloneProtoMsg(cachedRollout), nil
		},
		clone: apiutils.CloneProtoMsg[*autoupdatev1.AutoUpdateAgentRollout],
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateAgentRollout)
	return out, trace.Wrap(err)
}
