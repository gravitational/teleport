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
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/clientcache"
)

type osConfig struct {
	tunName    string
	tunIPv4    string
	tunIPv6    string
	cidrRanges []string
	dnsAddr    string
	dnsZones   []string
}

type osConfigurator struct {
	clientStore        *client.Store
	clientCache        *clientcache.Cache
	clusterConfigCache *ClusterConfigCache
	tunName            string
	tunIPv6            string
	dnsAddr            string
	homePath           string
	tunIPv4            string
}

func newOSConfigurator(tunName, ipv6Prefix, dnsAddr, homePath string) (*osConfigurator, error) {
	if homePath == "" {
		// This runs as root so we need to be configured with the user's home path.
		return nil, trace.BadParameter("homePath must be passed from unprivileged process")
	}

	// ipv6Prefix always looks like "fdxx:xxxx:xxxx::"
	// Set the IPv6 address for the TUN to "fdxx:xxxx:xxxx::1", the first valid address in the range.
	tunIPv6 := ipv6Prefix + "1"

	configurator := &osConfigurator{
		tunName:     tunName,
		tunIPv6:     tunIPv6,
		dnsAddr:     dnsAddr,
		homePath:    homePath,
		clientStore: client.NewFSClientStore(homePath),
	}
	configurator.clusterConfigCache = NewClusterConfigCache(clockwork.NewRealClock())

	clientCache, err := clientcache.New(clientcache.Config{
		NewClientFunc: configurator.getClient,
		RetryWithReloginFunc: func(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
			// osConfigurator is ran from a root process, so there's no way for it to relogin.
			// Instead, osConfigurator depends on the user performing a relogin from another process.
			return trace.Wrap(fn())
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configurator.clientCache = clientCache

	return configurator, nil
}

func (c *osConfigurator) close() error {
	return trace.Wrap(c.clientCache.Clear())
}

func (c *osConfigurator) updateOSConfiguration(ctx context.Context) error {
	var dnsZones []string
	var cidrRanges []string

	profileNames, err := profile.ListProfileNames(c.homePath)
	if err != nil {
		return trace.Wrap(err, "listing user profiles")
	}
	for _, profileName := range profileNames {
		profileDNSZones, profileCIDRRanges := c.getDNSZonesAndCIDRRangesForProfile(ctx, profileName)
		dnsZones = append(dnsZones, profileDNSZones...)
		cidrRanges = append(cidrRanges, profileCIDRRanges...)
	}

	dnsZones = utils.Deduplicate(dnsZones)
	cidrRanges = utils.Deduplicate(cidrRanges)

	if c.tunIPv4 == "" && len(cidrRanges) > 0 {
		// Choose an IPv4 address for the TUN interface from the CIDR range of one arbitrary currently
		// logged-in cluster. Only one IPv4 address is needed.
		if err := c.setTunIPv4FromCIDR(cidrRanges[0]); err != nil {
			return trace.Wrap(err, "setting TUN IPv4 address")
		}
	}

	err = configureOS(ctx, &osConfig{
		tunName:    c.tunName,
		tunIPv6:    c.tunIPv6,
		tunIPv4:    c.tunIPv4,
		dnsAddr:    c.dnsAddr,
		dnsZones:   dnsZones,
		cidrRanges: cidrRanges,
	})
	return trace.Wrap(err, "configuring OS")
}

// getDNSZonesAndCIDRRangesForProfile returns DNS zones and CIDR ranges for the root cluster and its
// leaf clusters.
//
// It's important for this function to return any data it manages to collect. For example, if it
// manages to grab DNS zones and CIDR ranges of the root cluster but it fails to list leaf clusters,
// it should still return the zones and ranges of the root cluster. Hence the use of named return
// values.
func (c *osConfigurator) getDNSZonesAndCIDRRangesForProfile(ctx context.Context, profileName string) (dnsZones []string, cidrRanges []string) {
	shouldClearCacheForRoot := true
	defer func() {
		if shouldClearCacheForRoot {
			if err := c.clientCache.ClearForRoot(profileName); err != nil {
				slog.ErrorContext(ctx, "Error while clearing client cache", "profile", profileName, "error", err)
			}
		}
	}()

	rootClient, err := c.clientCache.Get(ctx, profileName, "" /*leafClusterName*/)
	if err != nil {
		slog.WarnContext(ctx,
			"Failed to get root cluster client from cache, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)

		return
	}
	clusterConfig, err := c.clusterConfigCache.GetClusterConfig(ctx, rootClient)
	if err != nil {
		slog.WarnContext(ctx,
			"Failed to load VNet configuration, profile may be expired, not configuring VNet for this cluster",
			"profile", profileName, "error", err)

		return
	}

	dnsZones = append(dnsZones, clusterConfig.DNSZones...)
	cidrRanges = append(cidrRanges, clusterConfig.IPv4CIDRRange)

	leafClusters, err := getLeafClusters(ctx, rootClient)
	if err != nil {
		slog.WarnContext(ctx,
			"Failed to list leaf clusters, profile may be expired, not configuring VNet for leaf clusters of this cluster",
			"profile", profileName, "error", err)

		return
	}

	// getLeafClusters was the last call using the root client. Do not clear cache if any call to
	// a leaf cluster fails – it might fail because of a problem with the leaf cluster, not because of
	// an expired cert.
	shouldClearCacheForRoot = false

	for _, leafClusterName := range leafClusters {
		clusterClient, err := c.clientCache.Get(ctx, profileName, leafClusterName)
		if err != nil {
			slog.WarnContext(ctx,
				"Failed to create leaf cluster client, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		clusterConfig, err := c.clusterConfigCache.GetClusterConfig(ctx, clusterClient)
		if err != nil {
			slog.WarnContext(ctx,
				"Failed to load VNet configuration, not configuring VNet for this cluster",
				"profile", profileName, "leaf_cluster", leafClusterName, "error", err)
			continue
		}

		dnsZones = append(dnsZones, clusterConfig.DNSZones...)
		cidrRanges = append(cidrRanges, clusterConfig.IPv4CIDRRange)
	}

	return
}

func (c *osConfigurator) deconfigureOS(ctx context.Context) error {
	// configureOS is meant to be called with an empty config to deconfigure anything necessary.
	return trace.Wrap(configureOS(ctx, &osConfig{}))
}

func (c *osConfigurator) setTunIPv4FromCIDR(cidrRange string) error {
	if c.tunIPv4 != "" {
		return nil
	}

	_, ipnet, err := net.ParseCIDR(cidrRange)
	if err != nil {
		return trace.Wrap(err, "parsing CIDR %q", cidrRange)
	}

	// ipnet.IP is the network address, ending in 0s, like 100.64.0.0
	// Add 1 to assign the TUN address, like 100.64.0.1
	tunAddress := ipnet.IP
	tunAddress[len(tunAddress)-1]++
	c.tunIPv4 = tunAddress.String()
	return nil
}

func (c *osConfigurator) getClient(ctx context.Context, profileName, leafClusterName string) (*client.TeleportClient, error) {
	// This runs in the root process, so obviously we don't have access to the client cache in the user
	// process. This loads cluster profiles and credentials from TELEPORT_HOME.
	clientConfig := &client.Config{
		ClientStore: c.clientStore,
	}
	if err := clientConfig.LoadProfile(c.clientStore, profileName); err != nil {
		return nil, trace.Wrap(err, "loading client profile")
	}
	if leafClusterName != "" {
		clientConfig.SiteName = leafClusterName
	}
	tc, err := client.NewClient(clientConfig)
	return tc, trace.Wrap(err)
}
