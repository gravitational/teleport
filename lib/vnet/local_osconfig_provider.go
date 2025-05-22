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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// LocalOSConfigProvider fetches target OS configuration parameters.
// Its methods get exposed by [clientApplicationService] so that
// [remoteOSConfigProvider] can be implemented by calling these methods from the
// VNet admin process.
type LocalOSConfigProvider struct {
	cfg *LocalOSConfigProviderConfig
}

// LocalOSConfigProviderConfig holds configuration parameters for
// LocalOSConfigProvider.
type LocalOSConfigProviderConfig struct {
	clientApplication  ClientApplication
	clusterConfigCache *ClusterConfigCache
	leafClusterCache   *leafClusterCache
}

// NewLocalOSConfigProvider returns a new LocalOSConfigProvider.
func NewLocalOSConfigProvider(cfg *LocalOSConfigProviderConfig) *LocalOSConfigProvider {
	return &LocalOSConfigProvider{
		cfg: cfg,
	}
}

// GetTargetOSConfiguration returns the configuration values that should be
// configured in the OS, including DNS zones that should be handled by the VNet
// DNS nameserver and the IPv4 CIDR ranges that should be routed to the VNet TUN
// interface. This is not all of the OS configuration values, only the ones that
// must be communicated from the client application to the admin process.
func (p *LocalOSConfigProvider) GetTargetOSConfiguration(ctx context.Context) (*vnetv1.TargetOSConfiguration, error) {
	profiles, err := p.cfg.clientApplication.ListProfiles()
	if err != nil {
		return nil, trace.Wrap(err, "listing profiles")
	}
	var targetOSConfig vnetv1.TargetOSConfiguration
	for _, profileName := range profiles {
		profileTargetConfig := p.targetOSConfigurationForProfile(ctx, profileName)
		targetOSConfig.DnsZones = append(targetOSConfig.DnsZones, profileTargetConfig.DnsZones...)
		targetOSConfig.Ipv4CidrRanges = append(targetOSConfig.Ipv4CidrRanges, profileTargetConfig.Ipv4CidrRanges...)
	}
	targetOSConfig.DnsZones = utils.Deduplicate(targetOSConfig.DnsZones)
	targetOSConfig.Ipv4CidrRanges = utils.Deduplicate(targetOSConfig.Ipv4CidrRanges)
	return &targetOSConfig, nil
}

// targetOSConfigurationForProfile does not return errors, it is better to
// configure VNet for any working profiles and log errors for failures.
func (p *LocalOSConfigProvider) targetOSConfigurationForProfile(ctx context.Context, profileName string) *vnetv1.TargetOSConfiguration {
	targetOSConfig := &vnetv1.TargetOSConfiguration{}
	rootClusterClient, err := p.cfg.clientApplication.GetCachedClient(ctx, profileName, "" /*leafClusterName*/)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to get root cluster client from cache, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)
		return targetOSConfig
	}
	rootClusterConfig, err := p.cfg.clusterConfigCache.GetClusterConfig(ctx, rootClusterClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to load VNet configuration, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)
		return targetOSConfig
	}
	targetOSConfig.DnsZones = rootClusterConfig.DNSZones
	targetOSConfig.Ipv4CidrRanges = []string{rootClusterConfig.IPv4CIDRRange}

	leafClusterNames, err := p.cfg.leafClusterCache.getLeafClusters(ctx, rootClusterClient)
	if err != nil {
		log.WarnContext(ctx,
			"Failed to list leaf clusters, profile may be expired, not configuring VNet for leaf clusters of this cluster",
			"profile", profileName, "error", err)
		return targetOSConfig
	}
	for _, leafClusterName := range leafClusterNames {
		leafClusterClient, err := p.cfg.clientApplication.GetCachedClient(ctx, profileName, leafClusterName)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to create leaf cluster client, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			return targetOSConfig
		}
		leafClusterConfig, err := p.cfg.clusterConfigCache.GetClusterConfig(ctx, leafClusterClient)
		if err != nil {
			log.WarnContext(ctx,
				"Failed to load VNet configuration, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			return targetOSConfig
		}
		targetOSConfig.DnsZones = append(targetOSConfig.DnsZones, leafClusterConfig.DNSZones...)
		targetOSConfig.Ipv4CidrRanges = append(targetOSConfig.Ipv4CidrRanges, leafClusterConfig.IPv4CIDRRange)
	}
	return targetOSConfig
}
