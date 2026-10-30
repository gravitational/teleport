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
	"cmp"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/singleflight"

	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	"github.com/gravitational/teleport/lib/client"
)

type ClusterConfig struct {
	// ClusterName is the name reported by the cluster's Ping response.
	ClusterName string
	// RootClusterName is the root cluster's name, even when the profile selects a leaf.
	// After addressing the FIXME comment in [vnetClientApplication.newTeleportClient] this field should be removed.
	RootClusterName string
	// ProxyPublicAddr is the public address of the proxy, it is always a valid DNS zone for apps.
	ProxyPublicAddr string
	// CustomDNSZones is the list of custom DNS zones configured for the cluster.
	CustomDNSZones []string
	// IPv4CIDRRange is the CIDR range that IPv4 addresses should be assigned from for apps in this cluster.
	IPv4CIDRRange string
	// Expires is the time at which this information should be considered stale and refetched. Stale data may
	// be used if a subsequent fetch fails.
	Expires time.Time
}

func (e *ClusterConfig) stale(clock clockwork.Clock) bool {
	return clock.Now().After(e.Expires)
}

func (c *ClusterConfig) appDNSZones() []string {
	return append([]string{c.ProxyPublicAddr}, c.CustomDNSZones...)
}

// ClusterConfigCache is a read-through cache for cluster VnetConfigs. Cached entries go stale after 5
// minutes, after which they will be re-fetched on the next read.
//
// A stale entry is returned when a refresh fails. This keeps VNet's routing
// configuration available while the user's credentials are expired. The one
// exception is a refresh that fails because there are no credentials at all,
// which means the user logged out of the profile. Then the entry is dropped and
// the error is returned, so that VNet stops configuring the cluster.
type ClusterConfigCache struct {
	flightGroup singleflight.Group
	clock       clockwork.Clock
	cache       map[ClusterConfigKey]*ClusterConfig
	mu          sync.RWMutex
}

// ClusterConfigKey identifies a cluster config independently of whether a
// client can currently be acquired for that cluster.
type ClusterConfigKey struct {
	ProfileName     string
	LeafClusterName string
}

func (k ClusterConfigKey) String() string {
	return fmt.Sprintf("%q/%q", k.ProfileName, k.LeafClusterName)
}

func NewClusterConfigCache(clock clockwork.Clock) *ClusterConfigCache {
	return &ClusterConfigCache{
		clock: clock,
		cache: make(map[ClusterConfigKey]*ClusterConfig),
	}
}

func (c *ClusterConfigCache) GetClusterConfig(ctx context.Context, key ClusterConfigKey, app ClientApplication) (*ClusterConfig, error) {
	// Use a singleflight.Group to avoid concurrent requests for the same cluster VnetConfig.
	result, err, _ := c.flightGroup.Do(key.String(), func() (any, error) {
		// Check the cache inside flightGroup.Do to avoid the chance of immediate repeat calls to the cluster.
		c.mu.RLock()
		existingCacheEntry, existingCacheEntryFound := c.cache[key]
		c.mu.RUnlock()
		var clusterConfig *ClusterConfig
		err := app.WithCachedClient(ctx, key.ProfileName, key.LeafClusterName, func(clusterClient ClusterClient) error {
			// Always acquire a client, so logout is detected  before the entry becomes stale.
			if existingCacheEntryFound && !existingCacheEntry.stale(c.clock) {
				clusterConfig = existingCacheEntry
				return nil
			}
			var err error
			clusterConfig, err = c.getClusterConfigUncached(ctx, clusterClient)
			return trace.Wrap(err)
		})
		if err != nil {
			// The user logged out of the profile, remove the config.
			if client.IsNoCredentialsError(err) {
				c.mu.Lock()
				delete(c.cache, key)
				c.mu.Unlock()
				return nil, trace.Wrap(err)
			}
			// It's better to return a stale cached VnetConfig than an error. The profile probably expired and
			// we want to keep functioning until a relogin. We don't expect the VnetConfig to change very
			// often.
			if existingCacheEntryFound {
				return existingCacheEntry, nil
			}
			return nil, trace.Wrap(err)
		}

		c.mu.Lock()
		c.cache[key] = clusterConfig
		c.mu.Unlock()

		return clusterConfig, nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result.(*ClusterConfig), nil
}

func (c *ClusterConfigCache) getClusterConfigUncached(ctx context.Context, clusterClient ClusterClient) (*ClusterConfig, error) {
	pingResp, err := clusterClient.CurrentCluster().Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyPublicAddr := pingResp.ProxyPublicAddr
	if strings.Contains(proxyPublicAddr, ":") {
		proxyPublicAddr, _, err = net.SplitHostPort(pingResp.ProxyPublicAddr)
		if err != nil {
			return nil, trace.Wrap(err, "parsing proxy public addr")
		}
	}

	var customDNSZones []string
	ipv4CIDRRange := typesvnet.DefaultIPv4CIDRRange

	vnetConfig, err := clusterClient.CurrentCluster().GetVnetConfig(ctx)
	if trace.IsNotFound(err) || trace.IsNotImplemented(err) {
		// Use the defaults set above, nothing to do here.
	} else if err != nil {
		return nil, trace.Wrap(err)
	} else {
		for _, zone := range vnetConfig.GetSpec().GetCustomDnsZones() {
			customDNSZones = append(customDNSZones, zone.GetSuffix())
		}
		ipv4CIDRRange = cmp.Or(vnetConfig.GetSpec().GetIpv4CidrRange(), typesvnet.DefaultIPv4CIDRRange)
	}

	return &ClusterConfig{
		ClusterName:     pingResp.ClusterName,
		RootClusterName: clusterClient.RootClusterName(),
		ProxyPublicAddr: proxyPublicAddr,
		CustomDNSZones:  customDNSZones,
		IPv4CIDRRange:   ipv4CIDRRange,
		Expires:         c.clock.Now().Add(5 * time.Minute),
	}, nil
}
