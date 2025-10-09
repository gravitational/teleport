/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package readonly

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// Upstream represents the upstream data source that the cache will fetch data from.
type Upstream interface {
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
	GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error)
	GetAccessGraphSettings(ctx context.Context) (*clusterconfigpb.AccessGraphSettings, error)
}

// Cache provides simple ttl-based in-memory caching for select resources that are frequently accessed
// on hot paths.  All resources are returned as read-only shared references.
type Cache struct {
	cfg      CacheConfig
	ttlCache *utils.FnCache
}

// CacheConfig holds configuration options for the cache.
type CacheConfig struct {
	// Upstream is the upstream data source that the cache will fetch data from.
	Upstream Upstream
	// TTL is the time-to-live for each cache entry.
	TTL time.Duration
	// Disabled is a flag that can be used to disable ttl-caching. Useful in tests that
	// don't play nicely with stale data.
	Disabled bool
	// ReloadOnErr controls wether or not the underlying ttl cache will hold onto error
	// entries for the full TTL, or reload error entries immediately. As a general rule,
	// this value aught to be true on auth servers and false on agents, though in practice
	// the difference is small unless an unusually long TTL is used.
	ReloadOnErr bool
}

// NewCache sets up a new cache instance with the provided configuration.
func NewCache(cfg CacheConfig) (*Cache, error) {
	if cfg.Upstream == nil {
		return nil, trace.BadParameter("missing upstream data source for readonly cache")
	}
	if cfg.TTL == 0 {
		cfg.TTL = time.Millisecond * 1600
	}

	ttlCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         cfg.TTL,
		ReloadOnErr: cfg.ReloadOnErr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Cache{
		cfg:      cfg,
		ttlCache: ttlCache,
	}, nil
}

type ttlCacheKey struct {
	kind string
}

// GetReadOnlyAuthPreference returns a read-only shared reference to the auth preference resource.
func (c *Cache) GetReadOnlyAuthPreference(ctx context.Context) (AuthPreference, error) {
	if c.cfg.Disabled {
		cfg, err := c.cfg.Upstream.GetAuthPreference(ctx)
		return sealAuthPreference(cfg), trace.Wrap(err)
	}
	cfg, err := utils.FnCacheGet(ctx, c.ttlCache, ttlCacheKey{kind: types.KindClusterAuthPreference}, func(ctx context.Context) (AuthPreference, error) {
		cfg, err := c.cfg.Upstream.GetAuthPreference(ctx)
		return sealAuthPreference(cfg), trace.Wrap(err)
	})
	return cfg, trace.Wrap(err)
}

// GetReadOnlyClusterNetworkingConfig returns a read-only shared reference to the cluster networking config resource.
func (c *Cache) GetReadOnlyClusterNetworkingConfig(ctx context.Context) (ClusterNetworkingConfig, error) {
	if c.cfg.Disabled {
		cfg, err := c.cfg.Upstream.GetClusterNetworkingConfig(ctx)
		return sealClusterNetworkingConfig(cfg), trace.Wrap(err)
	}
	cfg, err := utils.FnCacheGet(ctx, c.ttlCache, ttlCacheKey{kind: types.KindClusterNetworkingConfig}, func(ctx context.Context) (ClusterNetworkingConfig, error) {
		cfg, err := c.cfg.Upstream.GetClusterNetworkingConfig(ctx)
		return sealClusterNetworkingConfig(cfg), trace.Wrap(err)
	})
	return cfg, trace.Wrap(err)
}

// GetReadOnlySessionRecordingConfig returns a read-only shared reference to the session recording config resource.
func (c *Cache) GetReadOnlySessionRecordingConfig(ctx context.Context) (SessionRecordingConfig, error) {
	if c.cfg.Disabled {
		cfg, err := c.cfg.Upstream.GetSessionRecordingConfig(ctx)
		return sealSessionRecordingConfig(cfg), trace.Wrap(err)
	}
	cfg, err := utils.FnCacheGet(ctx, c.ttlCache, ttlCacheKey{kind: types.KindSessionRecordingConfig}, func(ctx context.Context) (SessionRecordingConfig, error) {
		cfg, err := c.cfg.Upstream.GetSessionRecordingConfig(ctx)
		return sealSessionRecordingConfig(cfg), trace.Wrap(err)
	})
	return cfg, trace.Wrap(err)
}

// GetReadOnlyAccessGraphSettings returns a read-only shared reference to the dynamic access graph settings resource.
func (c *Cache) GetReadOnlyAccessGraphSettings(ctx context.Context) (AccessGraphSettings, error) {
	if c.cfg.Disabled {
		cfg, err := c.cfg.Upstream.GetAccessGraphSettings(ctx)
		return sealAccessGraphSettings(cfg), trace.Wrap(err)
	}
	cfg, err := utils.FnCacheGet(ctx, c.ttlCache, ttlCacheKey{kind: types.KindAccessGraphSettings}, func(ctx context.Context) (AccessGraphSettings, error) {
		cfg, err := c.cfg.Upstream.GetAccessGraphSettings(ctx)
		return sealAccessGraphSettings(cfg), trace.Wrap(err)
	})
	return cfg, trace.Wrap(err)
}
