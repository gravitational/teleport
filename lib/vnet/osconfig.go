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
	"log/slog"
	"net"
	"os"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/client"
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
	// Static config values.
	tunName string
	tunIPv6 string
	dnsAddr string

	// Computed config values.
	tunIPv4 string

	// Other state.
	homePath           string
	clientStore        *client.Store
	clusterConfigCache *clusterConfigCache
}

func newOSConfigurator(tunName, ipv6Prefix, dnsAddr string) (*osConfigurator, error) {
	homePath := os.Getenv(types.HomeEnvVar)
	if homePath == "" {
		// This runs as root so we need to be configured with the user's home path.
		return nil, trace.BadParameter("%s must be set", types.HomeEnvVar)
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
	configurator.clusterConfigCache = newClusterConfigCache(configurator.getVnetConfig, clockwork.NewRealClock())

	return configurator, nil
}

func (c *osConfigurator) updateOSConfiguration(ctx context.Context) error {
	var dnsZones []string
	var cidrRanges []string

	profileNames, err := profile.ListProfileNames(c.homePath)
	if err != nil {
		return trace.Wrap(err, "listing user profiles")
	}
	for _, profileName := range profileNames {
		// TODO(nklaassen): support leaf clusters
		vnetConfig, err := c.clusterConfigCache.getVnetConfig(ctx, profileName, "" /*leafClusterName*/)
		if err != nil {
			slog.WarnContext(ctx,
				"Failed to load VNet configuration, profile may be expired, not configuring VNet for this cluster",
				"profile", profileName, "error", err)
			continue
		}

		// profileName is the web proxy address, add the default DNS zone for it.
		// TODO(nklaassen): add the custom DNS zones as well, after the rest of VNet supports it.
		dnsZones = append(dnsZones, profileName)

		cidrRange := cmp.Or(vnetConfig.GetSpec().GetIpv4CidrRange(), defaultIPv4CIDRRange)
		cidrRanges = append(cidrRanges, cidrRange)
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

func (c *osConfigurator) deconfigureOS() error {
	// configureOS is meant to be called with an empty config to deconfigure anything necessary.
	// Pass context.Background() because we are likely deconfiguring because we received a signal to terminate
	// and all contexts have been canceled.
	return trace.Wrap(configureOS(context.Background(), &osConfig{}))
}

func (c *osConfigurator) setTunIPv4FromCIDR(cidrRange string) error {
	if c.tunIPv4 != "" {
		return nil
	}

	_, net, err := net.ParseCIDR(cidrRange)
	if err != nil {
		return trace.Wrap(err, "parsing CIDR %q", cidrRange)
	}

	// net.IP is the network address, ending in 0s, like 100.64.0.0
	// Add 1 to assign the TUN address, like 100.64.0.1
	tunAddress := net.IP
	tunAddress[len(tunAddress)-1]++
	c.tunIPv4 = tunAddress.String()
	return nil
}

func (c *osConfigurator) getVnetConfig(ctx context.Context, profileName, leafClusterName string) (*vnet.VnetConfig, error) {
	clt, err := c.vnetConfigClient(ctx, profileName, leafClusterName)
	if err != nil {
		return nil, trace.Wrap(err, "getting vnet client for profile %s %s", profileName, leafClusterName)
	}

	vnetConfig, err := clt.GetVnetConfig(ctx, &vnet.GetVnetConfigRequest{})
	return vnetConfig, trace.Wrap(err)
}

func (c *osConfigurator) vnetConfigClient(ctx context.Context, profileName, leafClusterName string) (vnet.VnetConfigServiceClient, error) {
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
	if err != nil {
		return nil, trace.Wrap(err, "creating new teleport client")
	}

	clusterClt, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to cluster")
	}

	vnetConfigClt := clusterClt.CurrentCluster().VnetConfigServiceClient()
	return vnetConfigClt, nil
}
