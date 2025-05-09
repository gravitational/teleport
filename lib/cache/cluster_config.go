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

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type clusterNameIndex string

const clusterNameDefaultIndex clusterNameIndex = "name"

func newClusterNameCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.ClusterName, clusterNameIndex], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.ClusterName, clusterNameIndex]{
		store: newStore(
			types.ClusterName.Clone,
			map[clusterNameIndex]func(types.ClusterName) string{
				clusterNameDefaultIndex: types.ClusterName.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.ClusterName, error) {
			name, err := c.GetClusterName(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []types.ClusterName{name}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.ClusterName {
			return &types.ClusterNameV2{
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

// GetClusterName gets the name of the cluster from the backend.
func (c *Cache) GetClusterName(ctx context.Context) (types.ClusterName, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetClusterName")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		name, err := rg.store.get(clusterNameDefaultIndex, types.MetaNameClusterName)
		return name.Clone(), trace.Wrap(err)
	}

	cachedName, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"name"}, func(ctx context.Context) (types.ClusterName, error) {
		cfg, err := c.Config.ClusterConfig.GetClusterName(ctx)
		return cfg, err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cachedName.Clone(), nil
}

type clusterAuditConfigIndex string

const clusterAuditConfigNameIndex clusterAuditConfigIndex = "name"

func newClusterAuditConfigCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.ClusterAuditConfig, clusterAuditConfigIndex], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.ClusterAuditConfig, clusterAuditConfigIndex]{
		store: newStore(
			types.ClusterAuditConfig.Clone,
			map[clusterAuditConfigIndex]func(types.ClusterAuditConfig) string{
				clusterAuditConfigNameIndex: types.ClusterAuditConfig.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.ClusterAuditConfig, error) {
			cfg, err := c.GetClusterAuditConfig(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []types.ClusterAuditConfig{cfg}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.ClusterAuditConfig {
			return &types.ClusterAuditConfigV2{
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

type clusterConfigCacheKey struct {
	kind string
}

// GetClusterAuditConfig gets ClusterAuditConfig from the backend.
func (c *Cache) GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetClusterAuditConfig")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.auditConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		cfg, err := rg.store.get(clusterAuditConfigNameIndex, types.MetaNameClusterAuditConfig)
		return cfg.Clone(), trace.Wrap(err)
	}

	cachedCfg, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"audit"}, func(ctx context.Context) (types.ClusterAuditConfig, error) {
		cfg, err := c.Config.ClusterConfig.GetClusterAuditConfig(ctx)
		return cfg, err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cachedCfg.Clone(), nil
}

type clusterNetworkingConfigIndex string

const clusterNetworkingConfigNameIndex clusterNetworkingConfigIndex = "name"

func newClusterNetworkingConfigCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.ClusterNetworkingConfig, clusterNetworkingConfigIndex], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.ClusterNetworkingConfig, clusterNetworkingConfigIndex]{
		store: newStore(
			types.ClusterNetworkingConfig.Clone,
			map[clusterNetworkingConfigIndex]func(types.ClusterNetworkingConfig) string{
				clusterNetworkingConfigNameIndex: types.ClusterNetworkingConfig.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.ClusterNetworkingConfig, error) {
			cfg, err := c.GetClusterNetworkingConfig(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []types.ClusterNetworkingConfig{cfg}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.ClusterNetworkingConfig {
			return &types.ClusterNetworkingConfigV2{
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

// GetClusterNetworkingConfig gets ClusterNetworkingConfig from the backend.
func (c *Cache) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetClusterNetworkingConfig")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.networkingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		cfg, err := rg.store.get(clusterNetworkingConfigNameIndex, types.MetaNameClusterNetworkingConfig)
		return cfg.Clone(), trace.Wrap(err)
	}

	cachedCfg, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"networking"}, func(ctx context.Context) (types.ClusterNetworkingConfig, error) {
		cfg, err := c.Config.ClusterConfig.GetClusterNetworkingConfig(ctx)
		return cfg, err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cachedCfg.Clone(), nil
}

type authPreferenceIndex string

const authPreferenceNameIndex authPreferenceIndex = "name"

func newAuthPreferenceCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.AuthPreference, authPreferenceIndex], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.AuthPreference, authPreferenceIndex]{
		store: newStore(
			types.AuthPreference.Clone,
			map[authPreferenceIndex]func(types.AuthPreference) string{
				authPreferenceNameIndex: types.AuthPreference.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.AuthPreference, error) {
			pref, err := c.GetAuthPreference(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []types.AuthPreference{pref}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.AuthPreference {
			return &types.AuthPreferenceV2{
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

// GetAuthPreference gets the cluster authentication config.
func (c *Cache) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAuthPreference")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.authPreference)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		cfg, err := rg.store.get(authPreferenceNameIndex, types.MetaNameClusterAuthPreference)
		return cfg.Clone(), trace.Wrap(err)
	}

	cfg, err := c.Config.ClusterConfig.GetAuthPreference(ctx)
	return cfg, trace.Wrap(err)
}

type sessionRecordingConfigIndex string

const sessionRecordingConfigNameIndex sessionRecordingConfigIndex = "name"

func newSessionRecordingConfigCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.SessionRecordingConfig, sessionRecordingConfigIndex], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.SessionRecordingConfig, sessionRecordingConfigIndex]{
		store: newStore(
			types.SessionRecordingConfig.Clone,
			map[sessionRecordingConfigIndex]func(types.SessionRecordingConfig) string{
				sessionRecordingConfigNameIndex: types.SessionRecordingConfig.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.SessionRecordingConfig, error) {
			cfg, err := c.GetSessionRecordingConfig(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []types.SessionRecordingConfig{cfg}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.SessionRecordingConfig {
			return &types.SessionRecordingConfigV2{
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

// GetSessionRecordingConfig gets session recording configuration.
func (c *Cache) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSessionRecordingConfig")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.sessionRecordingConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		cfg, err := rg.store.get(sessionRecordingConfigNameIndex, types.MetaNameSessionRecordingConfig)
		return cfg.Clone(), trace.Wrap(err)
	}

	cfg, err := c.Config.ClusterConfig.GetSessionRecordingConfig(ctx)
	return cfg, trace.Wrap(err)
}

type accessGraphSettingsIndex string

const accessGraphSettingsNameIndex accessGraphSettingsIndex = "name"

func newAccessGraphSettingsCollection(upstream services.ClusterConfiguration, w types.WatchKind) (*collection[*clusterconfigv1.AccessGraphSettings, accessGraphSettingsIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[*clusterconfigv1.AccessGraphSettings, accessGraphSettingsIndex]{
		store: newStore(
			proto.CloneOf[*clusterconfigv1.AccessGraphSettings],
			map[accessGraphSettingsIndex]func(*clusterconfigv1.AccessGraphSettings) string{
				accessGraphSettingsNameIndex: func(r *clusterconfigv1.AccessGraphSettings) string {
					return r.GetMetadata().GetName()
				},
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*clusterconfigv1.AccessGraphSettings, error) {
			set, err := upstream.GetAccessGraphSettings(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []*clusterconfigv1.AccessGraphSettings{set}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *clusterconfigv1.AccessGraphSettings {
			return &clusterconfigv1.AccessGraphSettings{
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

// GetAccessGraphSettings gets AccessGraphSettings from the backend.
func (c *Cache) GetAccessGraphSettings(ctx context.Context) (*clusterconfigv1.AccessGraphSettings, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetAccessGraphSettings")
	defer span.End()

	getter := genericGetter[*clusterconfigv1.AccessGraphSettings, accessGraphSettingsIndex]{
		cache:      c,
		collection: c.collections.accessGraphSettings,
		index:      accessGraphSettingsNameIndex,
		upstreamGet: func(ctx context.Context, s string) (*clusterconfigv1.AccessGraphSettings, error) {
			cachedCfg, err := utils.FnCacheGet(ctx, c.fnCache, clusterConfigCacheKey{"access_graph_settings"}, func(ctx context.Context) (*clusterconfigv1.AccessGraphSettings, error) {
				cfg, err := c.Config.ClusterConfig.GetAccessGraphSettings(ctx)
				return cfg, err
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return apiutils.CloneProtoMsg(cachedCfg), nil
		},
	}
	out, err := getter.get(ctx, types.MetaNameAccessGraphSettings)
	return out, trace.Wrap(err)
}
