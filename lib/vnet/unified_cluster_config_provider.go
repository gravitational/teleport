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

package vnet

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

// UnifiedClusterConfig is a unified VNet configuration for all clusters
// the user is logged in to.
type UnifiedClusterConfig struct {
	// ClusterNames contains the name of each root or leaf cluster the user is
	// logged in to. SSH hosts are reachable via VNet SSH at subdomains of
	// these cluster names.
	ClusterNames []string
	// ProxyPublicAddrs contains the proxy public address of root and leaf
	// cluster the user is logged in to. These are always valid DNS suffixes for
	// TCP apps.
	ProxyPublicAddrs []string
	// CustomDNSZones is the unified set of custom DNS zones configured in all clusters.
	CustomDNSZones []string
	// IPv4CidrRanges is the unified set of IPv4 CIDR ranges configured in all
	// clusters, VNet will try to route all of these to the TUN interface.
	IPv4CidrRanges []string
}

// AppDNSZones returns the DNS suffixes valid for TCP apps.
func (c *UnifiedClusterConfig) AppDNSZones() []string {
	return utils.Deduplicate(slices.Concat(c.CustomDNSZones, c.ProxyPublicAddrs))
}

// AllDNSZones return all DNS suffixes VNet handles.
func (c *UnifiedClusterConfig) AllDNSZones() []string {
	return utils.Deduplicate(slices.Concat(c.CustomDNSZones, c.ProxyPublicAddrs, c.ClusterNames))
}

// UnifiedClusterConfigProvider fetches the [UnifiedClusterConfig].
type UnifiedClusterConfigProvider struct {
	cfg *UnifiedClusterConfigProviderConfig
}

// UnifiedClusterConfigProviderConfig holds configuration parameters for
// [UnifiedClusterConfigProvider].
type UnifiedClusterConfigProviderConfig struct {
	clientApplication  ClientApplication
	clusterConfigCache *ClusterConfigCache
	leafClusterCache   *leafClusterCache
}

// NewUnifiedClusterConfigProvider returns a new [UnifiedClusterConfigProvider].
func NewUnifiedClusterConfigProvider(cfg *UnifiedClusterConfigProviderConfig) *UnifiedClusterConfigProvider {
	return &UnifiedClusterConfigProvider{
		cfg: cfg,
	}
}

// GetUnifiedClusterConfig returns the unified VNet configuration of all
// clusters the user is logged in to.
func (p *UnifiedClusterConfigProvider) GetUnifiedClusterConfig(ctx context.Context) (*UnifiedClusterConfig, error) {
	profiles, err := p.cfg.clientApplication.ListProfiles()
	if err != nil {
		return nil, trace.Wrap(err, "listing profiles")
	}
	var unifiedClusterConfig UnifiedClusterConfig
	for _, profileName := range profiles {
		if err := p.fetchForProfile(ctx, profileName, &unifiedClusterConfig); err != nil {
			log.WarnContext(ctx,
				"Failed to fetch VNet configuration, profile may be expired, not configuring VNet for this profile",
				"profile", profileName, "error", err)
		}
	}
	unifiedClusterConfig.ClusterNames = utils.Deduplicate(unifiedClusterConfig.ClusterNames)
	unifiedClusterConfig.ProxyPublicAddrs = utils.Deduplicate(unifiedClusterConfig.ProxyPublicAddrs)
	unifiedClusterConfig.CustomDNSZones = utils.Deduplicate(unifiedClusterConfig.CustomDNSZones)
	unifiedClusterConfig.IPv4CidrRanges = utils.Deduplicate(unifiedClusterConfig.IPv4CidrRanges)
	return &unifiedClusterConfig, nil
}

func (p *UnifiedClusterConfigProvider) fetchForProfile(
	ctx context.Context,
	profileName string,
	unifiedClusterConfig *UnifiedClusterConfig,
) error {
	rootClusterClient, err := p.cfg.clientApplication.GetCachedClient(ctx, profileName, "" /*leafClusterName*/)
	if err != nil {
		return trace.Wrap(err, "getting root cluster client from cache")
	}
	rootClusterConfig, err := p.cfg.clusterConfigCache.GetClusterConfig(ctx, rootClusterClient)
	if err != nil {
		return trace.Wrap(err, "fetching root cluster VNet config")
	}
	unifiedClusterConfig.ClusterNames = append(unifiedClusterConfig.ClusterNames, rootClusterClient.ClusterName())
	unifiedClusterConfig.ProxyPublicAddrs = append(unifiedClusterConfig.ProxyPublicAddrs, rootClusterConfig.ProxyPublicAddr)
	unifiedClusterConfig.CustomDNSZones = append(unifiedClusterConfig.CustomDNSZones, rootClusterConfig.CustomDNSZones...)
	unifiedClusterConfig.IPv4CidrRanges = append(unifiedClusterConfig.IPv4CidrRanges, rootClusterConfig.IPv4CIDRRange)

	leafClusterNames, err := p.cfg.leafClusterCache.getLeafClusters(ctx, rootClusterClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to list leaf clusters, profile may be expired, not configuring VNet for leaf clusters in this profile",
			"profile", profileName, "error", err)
		return nil
	}
	for _, leafClusterName := range leafClusterNames {
		if err := p.fetchForLeafCluster(ctx, profileName, leafClusterName, unifiedClusterConfig); err != nil {
			log.WarnContext(ctx,
				"Failed to fetch VNet configuration for leaf cluster, VNet will not be configured for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
		}
	}
	return nil
}

func (p *UnifiedClusterConfigProvider) fetchForLeafCluster(
	ctx context.Context,
	profileName string,
	leafClusterName string,
	unifiedClusterConfig *UnifiedClusterConfig,
) error {
	leafClusterClient, err := p.cfg.clientApplication.GetCachedClient(ctx, profileName, leafClusterName)
	if err != nil {
		return trace.Wrap(err, "getting leaf cluster client from cache")
	}
	leafClusterConfig, err := p.cfg.clusterConfigCache.GetClusterConfig(ctx, leafClusterClient)
	if err != nil {
		return trace.Wrap(err, "fetching leaf cluster VNet config from cache")
	}
	unifiedClusterConfig.ClusterNames = append(unifiedClusterConfig.ClusterNames, leafClusterName)
	unifiedClusterConfig.ProxyPublicAddrs = append(unifiedClusterConfig.ProxyPublicAddrs, leafClusterConfig.ProxyPublicAddr)
	unifiedClusterConfig.CustomDNSZones = append(unifiedClusterConfig.CustomDNSZones, leafClusterConfig.CustomDNSZones...)
	unifiedClusterConfig.IPv4CidrRanges = append(unifiedClusterConfig.IPv4CidrRanges, leafClusterConfig.IPv4CIDRRange)
	return nil
}
