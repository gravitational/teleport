// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package vnet

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
)

type getClusterConfigFunc = func(ctx context.Context, profileName, leafClusterName string) (*vnet.VnetConfig, error)

type cacheEntry struct {
	vnetConfig *vnet.VnetConfig
	expires    time.Time
}

func (e *cacheEntry) stale(clock clockwork.Clock) bool {
	return clock.Now().After(e.expires)
}

// clusterConfigCache is a read-through cache for cluster VnetConfigs. Cached entries go stale after 5
// minutes, after which they will be re-fetched on the next read.
//
// If a read from the cluster fails but there is a stale cache entry present, this prefers to return the stale
// cached entry. This is desirable in cases where the profile for a cluster expires during VNet operation,
// it's better to use the stale custom DNS zones than to remove all DNS configuration for that cluster.
type clusterConfigCache struct {
	get         getClusterConfigFunc
	cache       map[string]cacheEntry
	mu          sync.RWMutex
	flightGroup singleflight.Group
	clock       clockwork.Clock
}

func newClusterConfigCache(get getClusterConfigFunc, clock clockwork.Clock) *clusterConfigCache {
	return &clusterConfigCache{
		get:   get,
		cache: make(map[string]cacheEntry),
		clock: clock,
	}
}

func (c *clusterConfigCache) getVnetConfig(ctx context.Context, profileName, leafClusterName string) (*vnet.VnetConfig, error) {
	k := clusterCacheKey(profileName, leafClusterName)

	// Use a singleflight.Group to avoid concurrent requests for the same cluster VnetConfig.
	vnetConfig, err, _ := c.flightGroup.Do(k, func() (any, error) {
		// Check the cache inside flightGroup.Do to avoid the chance of immediate repeat calls to the cluster.
		c.mu.RLock()
		existingCacheEntry, existingCacheEntryExists := c.cache[k]
		c.mu.RUnlock()
		if existingCacheEntryExists && !existingCacheEntry.stale(c.clock) {
			return existingCacheEntry.vnetConfig, nil
		}

		vnetConfig, err := c.get(ctx, profileName, leafClusterName)
		if trace.IsNotFound(err) || trace.IsNotImplemented(err) {
			// Default to the empty config on NotFound or NotImplemented.
			vnetConfig = &vnet.VnetConfig{}
			err = nil
		}
		if err != nil {
			// It's better to return a stale cached VnetConfig than an error. The profile probably expired and
			// we want to keep functioning until a relogin. We don't expect the VnetConfig to change very
			// often.
			if existingCacheEntryExists {
				return existingCacheEntry.vnetConfig, nil
			}
			return nil, trace.Wrap(err)
		}

		c.mu.Lock()
		c.cache[k] = cacheEntry{
			vnetConfig: vnetConfig,
			expires:    c.clock.Now().Add(5 * time.Minute),
		}
		c.mu.Unlock()

		return vnetConfig, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return vnetConfig.(*vnet.VnetConfig), nil
}

func clusterCacheKey(profileName, leafClusterName string) string {
	if leafClusterName != "" {
		return profileName + "/" + leafClusterName
	}
	return profileName
}
