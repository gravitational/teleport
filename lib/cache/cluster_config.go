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
	"github.com/gravitational/teleport/lib/utils"
)

const clusterNameIndex = "name"

func newClusterNameCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.ClusterName], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.ClusterName]{
		store: newStore(map[string]func(types.ClusterName) string{
			clusterNameIndex: func(n types.ClusterName) string {
				return n.GetName()
			},
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
		name, err := rg.store.get(clusterNameIndex, types.MetaNameClusterName)
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

const clusterAuditConfigIndex = "name"

func newClusterAuditConfigCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.ClusterAuditConfig], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.ClusterAuditConfig]{
		store: newStore(map[string]func(types.ClusterAuditConfig) string{
			clusterAuditConfigIndex: func(n types.ClusterAuditConfig) string {
				return n.GetName()
			},
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
		cfg, err := rg.store.get(clusterAuditConfigIndex, types.MetaNameClusterAuditConfig)
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

const clusterNetworkingConfigIndex = "name"

func newClusterNetworkingConfigCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.ClusterNetworkingConfig], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.ClusterNetworkingConfig]{
		store: newStore(map[string]func(types.ClusterNetworkingConfig) string{
			clusterNetworkingConfigIndex: func(n types.ClusterNetworkingConfig) string {
				return n.GetName()
			},
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
		cfg, err := rg.store.get(clusterNetworkingConfigIndex, types.MetaNameClusterNetworkingConfig)
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

const authPreferenceIndex = "name"

func newAuthPreferenceCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.AuthPreference], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.AuthPreference]{
		store: newStore(map[string]func(types.AuthPreference) string{
			authPreferenceIndex: func(n types.AuthPreference) string {
				return n.GetName()
			},
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
		cfg, err := rg.store.get(authPreferenceIndex, types.MetaNameClusterAuthPreference)
		return cfg.Clone(), trace.Wrap(err)
	}

	cfg, err := c.Config.ClusterConfig.GetAuthPreference(ctx)
	return cfg, trace.Wrap(err)
}

const sessionRecordingConfigIndex = "name"

func newSessionRecordingConfigCollection(c services.ClusterConfiguration, w types.WatchKind) (*collection[types.SessionRecordingConfig], error) {
	if c == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.SessionRecordingConfig]{
		store: newStore(map[string]func(types.SessionRecordingConfig) string{
			sessionRecordingConfigIndex: func(n types.SessionRecordingConfig) string {
				return n.GetName()
			},
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
		cfg, err := rg.store.get(sessionRecordingConfigIndex, types.MetaNameSessionRecordingConfig)
		return cfg.Clone(), trace.Wrap(err)
	}

	cfg, err := c.Config.ClusterConfig.GetSessionRecordingConfig(ctx)
	return cfg, trace.Wrap(err)
}
