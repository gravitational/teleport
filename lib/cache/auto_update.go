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
	"google.golang.org/protobuf/proto"

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
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
		store: newStore(
			types.KindAutoUpdateConfig,
			proto.CloneOf[*autoupdatev1.AutoUpdateConfig],
			map[autoUpdateConfigIndex]func(*autoupdatev1.AutoUpdateConfig) string{
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
			return autoupdatev1.AutoUpdateConfig_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

type autoUpdateCacheKey struct {
	kind string
}

// autoUpdateConfigCollection provides read access to the cached autoupdate
// config. Its exported methods are promoted onto every topology cache that
// embeds it; the reads are implemented exactly once here. It is a stateless
// value assembled inline by each of its consumers so that no shared
// scaffolding couples their lifetimes.
type autoUpdateConfigCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	fnCache  *utils.FnCache
	upstream services.AutoUpdateServiceGetter
	col      *collection[*autoupdatev1.AutoUpdateConfig, autoUpdateConfigIndex]
}

// GetAutoUpdateConfig gets the AutoUpdateConfig from the backend.
func (c autoUpdateConfigCollection) GetAutoUpdateConfig(ctx context.Context) (*autoupdatev1.AutoUpdateConfig, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAutoUpdateConfig")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateConfig, autoUpdateConfigIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      autoUpdateConfigNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*autoupdatev1.AutoUpdateConfig, error) {
			cachedConfig, err := utils.FnCacheGet(ctx, c.fnCache, autoUpdateCacheKey{"config"}, func(ctx context.Context) (*autoupdatev1.AutoUpdateConfig, error) {
				cfg, err := c.upstream.GetAutoUpdateConfig(ctx)
				return cfg, trace.Wrap(err)
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return apiutils.CloneProtoMsg(cachedConfig), nil
		},
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateConfig)
	return out, trace.Wrap(err)
}

// GetAutoUpdateConfig gets the AutoUpdateConfig from the backend.
func (c *Cache) GetAutoUpdateConfig(ctx context.Context) (*autoupdatev1.AutoUpdateConfig, error) {
	return autoUpdateConfigCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.AutoUpdateService,
		col:      c.collections.autoUpdateConfig,
	}.GetAutoUpdateConfig(ctx)
}

type autoUpdateVersionIndex string

const autoUpdateVersionNameIndex autoUpdateVersionIndex = "name"

func newAutoUpdateVersionCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateServiceGetter")
	}

	return &collection[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex]{
		store: newStore(
			types.KindAutoUpdateVersion,
			proto.CloneOf[*autoupdatev1.AutoUpdateVersion],
			map[autoUpdateVersionIndex]func(*autoupdatev1.AutoUpdateVersion) string{
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
			return autoupdatev1.AutoUpdateVersion_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// autoUpdateVersionCollection provides read access to the cached autoupdate
// version. Its exported methods are promoted onto every topology cache that
// embeds it; the reads are implemented exactly once here. It is a stateless
// value assembled inline by each of its consumers so that no shared
// scaffolding couples their lifetimes.
type autoUpdateVersionCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	fnCache  *utils.FnCache
	upstream services.AutoUpdateServiceGetter
	col      *collection[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex]
}

// GetAutoUpdateVersion gets the AutoUpdateVersion from the backend.
func (c autoUpdateVersionCollection) GetAutoUpdateVersion(ctx context.Context) (*autoupdatev1.AutoUpdateVersion, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAutoUpdateVersion")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateVersion, autoUpdateVersionIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      autoUpdateVersionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*autoupdatev1.AutoUpdateVersion, error) {
			cachedVersion, err := utils.FnCacheGet(ctx, c.fnCache, autoUpdateCacheKey{"version"}, func(ctx context.Context) (*autoupdatev1.AutoUpdateVersion, error) {
				version, err := c.upstream.GetAutoUpdateVersion(ctx)
				return version, trace.Wrap(err)
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return apiutils.CloneProtoMsg(cachedVersion), nil
		},
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateVersion)
	return out, trace.Wrap(err)
}

// GetAutoUpdateVersion gets the AutoUpdateVersion from the backend.
func (c *Cache) GetAutoUpdateVersion(ctx context.Context) (*autoupdatev1.AutoUpdateVersion, error) {
	return autoUpdateVersionCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.AutoUpdateService,
		col:      c.collections.autoUpdateVersion,
	}.GetAutoUpdateVersion(ctx)
}

type autoUpdateAgentRolloutIndex string

const autoUpdateAgentRolloutNameIndex autoUpdateAgentRolloutIndex = "name"

func newAutoUpdateRolloutCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateServiceGetter")
	}

	return &collection[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex]{
		store: newStore(
			types.KindAutoUpdateAgentRollout,
			proto.CloneOf[*autoupdatev1.AutoUpdateAgentRollout],
			map[autoUpdateAgentRolloutIndex]func(*autoupdatev1.AutoUpdateAgentRollout) string{
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
			return autoupdatev1.AutoUpdateAgentRollout_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// autoUpdateRolloutCollection provides read access to the cached autoupdate
// agent rollout. Its exported methods are promoted onto every topology cache
// that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no
// shared scaffolding couples their lifetimes.
type autoUpdateRolloutCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	fnCache  *utils.FnCache
	upstream services.AutoUpdateServiceGetter
	col      *collection[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex]
}

// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout from the backend.
func (c autoUpdateRolloutCollection) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdatev1.AutoUpdateAgentRollout, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAutoUpdateAgentRollout")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateAgentRollout, autoUpdateAgentRolloutIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      autoUpdateAgentRolloutNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*autoupdatev1.AutoUpdateAgentRollout, error) {
			cachedRollout, err := utils.FnCacheGet(ctx, c.fnCache, autoUpdateCacheKey{"rollout"}, func(ctx context.Context) (*autoupdatev1.AutoUpdateAgentRollout, error) {
				rollout, err := c.upstream.GetAutoUpdateAgentRollout(ctx)
				return rollout, trace.Wrap(err)
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return apiutils.CloneProtoMsg(cachedRollout), nil
		},
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateAgentRollout)
	return out, trace.Wrap(err)
}

// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout from the backend.
func (c *Cache) GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdatev1.AutoUpdateAgentRollout, error) {
	return autoUpdateRolloutCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		fnCache:  c.fnCache,
		upstream: c.Config.AutoUpdateService,
		col:      c.collections.autoUpdateRollout,
	}.GetAutoUpdateAgentRollout(ctx)
}

type autoUpdateAgentReportIndex string

const autoUpdateAgentReportNameIndex autoUpdateAgentReportIndex = "name"

func newAutoUpdateAgentReportCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateAgentReports")
	}

	return &collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]{
		store: newStore(
			types.KindSecurityReport,
			proto.CloneOf[*autoupdatev1.AutoUpdateAgentReport],
			map[autoUpdateAgentReportIndex]func(*autoupdatev1.AutoUpdateAgentReport) string{
				autoUpdateAgentReportNameIndex: func(r *autoupdatev1.AutoUpdateAgentReport) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*autoupdatev1.AutoUpdateAgentReport, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListAutoUpdateAgentReports))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *autoupdatev1.AutoUpdateAgentReport {
			return autoupdatev1.AutoUpdateAgentReport_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// autoUpdateAgentReportCollection provides read access to the cached
// autoupdate agent reports. Its exported methods are promoted onto every
// topology cache that embeds it; the reads are implemented exactly once
// here. It is a stateless value assembled inline by each of its consumers so
// that no shared scaffolding couples their lifetimes.
type autoUpdateAgentReportCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.AutoUpdateServiceGetter
	col      *collection[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]
}

// GetAutoUpdateAgentReport gets the AutoUpdateAgentReport from the backend.
func (c autoUpdateAgentReportCollection) GetAutoUpdateAgentReport(ctx context.Context, name string) (*autoupdatev1.AutoUpdateAgentReport, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAutoUpdateAgentReport")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]{
		engine:      c.engine,
		collection:  c.col,
		index:       autoUpdateAgentReportNameIndex,
		upstreamGet: c.upstream.GetAutoUpdateAgentReport,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListAutoUpdateAgentReports lists autoupdate_agent_reports.
func (c autoUpdateAgentReportCollection) ListAutoUpdateAgentReports(ctx context.Context, pageSize int, pageToken string) ([]*autoupdatev1.AutoUpdateAgentReport, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListAutoUpdateAgentReports")
	defer span.End()

	lister := genericLister[*autoupdatev1.AutoUpdateAgentReport, autoUpdateAgentReportIndex]{
		engine:       c.engine,
		collection:   c.col,
		index:        autoUpdateAgentReportNameIndex,
		upstreamList: c.upstream.ListAutoUpdateAgentReports,
		nextToken: func(t *autoupdatev1.AutoUpdateAgentReport) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetAutoUpdateAgentReport gets the AutoUpdateAgentReport from the backend.
func (c *Cache) GetAutoUpdateAgentReport(ctx context.Context, name string) (*autoupdatev1.AutoUpdateAgentReport, error) {
	return autoUpdateAgentReportCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.AutoUpdateService,
		col:      c.collections.autoUpdateAgentReports,
	}.GetAutoUpdateAgentReport(ctx, name)
}

// ListAutoUpdateAgentReports lists autoupdate_agent_reports.
func (c *Cache) ListAutoUpdateAgentReports(ctx context.Context, pageSize int, pageToken string) ([]*autoupdatev1.AutoUpdateAgentReport, string, error) {
	return autoUpdateAgentReportCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.AutoUpdateService,
		col:      c.collections.autoUpdateAgentReports,
	}.ListAutoUpdateAgentReports(ctx, pageSize, pageToken)
}

type autoUpdateBotInstanceReportIndex string

const autoUpdateBotInstanceReportNameIndex autoUpdateBotInstanceReportIndex = "name"

func newAutoUpdateBotInstanceReportCollection(upstream services.AutoUpdateServiceGetter, w types.WatchKind) (*collection[*autoupdatev1.AutoUpdateBotInstanceReport, autoUpdateBotInstanceReportIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter AutoUpdateServiceGetter")
	}

	return &collection[*autoupdatev1.AutoUpdateBotInstanceReport, autoUpdateBotInstanceReportIndex]{
		store: newStore(
			types.KindAutoUpdateBotInstanceReport,
			proto.CloneOf[*autoupdatev1.AutoUpdateBotInstanceReport],
			map[autoUpdateBotInstanceReportIndex]func(*autoupdatev1.AutoUpdateBotInstanceReport) string{
				autoUpdateBotInstanceReportNameIndex: func(r *autoupdatev1.AutoUpdateBotInstanceReport) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*autoupdatev1.AutoUpdateBotInstanceReport, error) {
			report, err := upstream.GetAutoUpdateBotInstanceReport(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return []*autoupdatev1.AutoUpdateBotInstanceReport{report}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *autoupdatev1.AutoUpdateBotInstanceReport {
			return autoupdatev1.AutoUpdateBotInstanceReport_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// autoUpdateBotInstanceReportCollection provides read access to the cached
// autoupdate bot instance report. Its exported methods are promoted onto
// every topology cache that embeds it; the reads are implemented exactly
// once here. It is a stateless value assembled inline by each of its
// consumers so that no shared scaffolding couples their lifetimes.
type autoUpdateBotInstanceReportCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.AutoUpdateServiceGetter
	col      *collection[*autoupdatev1.AutoUpdateBotInstanceReport, autoUpdateBotInstanceReportIndex]
}

func (c autoUpdateBotInstanceReportCollection) GetAutoUpdateBotInstanceReport(ctx context.Context) (*autoupdatev1.AutoUpdateBotInstanceReport, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetAutoUpdateBotInstanceReport")
	defer span.End()

	getter := genericGetter[*autoupdatev1.AutoUpdateBotInstanceReport, autoUpdateBotInstanceReportIndex]{
		engine:     c.engine,
		collection: c.col,
		index:      autoUpdateBotInstanceReportNameIndex,
		upstreamGet: func(ctx context.Context, _ string) (*autoupdatev1.AutoUpdateBotInstanceReport, error) {
			return c.upstream.GetAutoUpdateBotInstanceReport(ctx)
		},
	}
	out, err := getter.get(ctx, types.MetaNameAutoUpdateBotInstanceReport)
	return out, trace.Wrap(err)
}

func (c *Cache) GetAutoUpdateBotInstanceReport(ctx context.Context) (*autoupdatev1.AutoUpdateBotInstanceReport, error) {
	return autoUpdateBotInstanceReportCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.AutoUpdateService,
		col:      c.collections.autoUpdateBotInstanceReports,
	}.GetAutoUpdateBotInstanceReport(ctx)
}
