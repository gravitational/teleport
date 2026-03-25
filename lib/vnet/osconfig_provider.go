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
	"net"
	"slices"

	"github.com/gravitational/trace"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// osConfigProvider fetches a target OS configuration based on cluster
// configuration fetched via the client application process available over gRPC.
type osConfigProvider struct {
	cfg        osConfigProviderConfig
	dnsAddrs   []string
	tunIPv6    string
	tunIPv4    string
	tunIPv4Net *net.IPNet
}

// osConfigProviderConfig holds configuration parameters for an osConfigProvider.
type osConfigProviderConfig struct {
	clt           targetOSConfigGetter
	tunName       string
	ipv6Prefix    string
	dnsIPv6       string
	addDNSAddress func(net.IP) error
}

type targetOSConfigGetter interface {
	GetTargetOSConfiguration(context.Context) (*vnetv1.TargetOSConfiguration, error)
}

func newOSConfigProvider(cfg osConfigProviderConfig) (*osConfigProvider, error) {
	tunIPv6, err := tunIPv6ForPrefix(cfg.ipv6Prefix)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &osConfigProvider{
		cfg:      cfg,
		dnsAddrs: []string{cfg.dnsIPv6},
		tunIPv6:  tunIPv6,
	}, nil
}

func (p *osConfigProvider) targetOSConfig(ctx context.Context) (*osConfig, error) {
	targetOSConfig, err := p.cfg.clt.GetTargetOSConfiguration(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting target OS configuration from client application")
	}
	if p.tunIPv4 == "" && len(targetOSConfig.Ipv4CidrRanges) > 0 {
		// Choose an IPv4 address for the TUN interface and the IPv4 DNS server
		// from the CIDR range of one arbitrary currently logged-in cluster.
		// We currently only assign one V4 address to the interface and only
		// advertise DNS on one V4 address.
		if err := p.setV4IPsFromFirstCIDR(targetOSConfig.Ipv4CidrRanges[0]); err != nil {
			return nil, trace.Wrap(err, "setting TUN IPv4 address")
		}
	}
	return &osConfig{
		tunName:    p.cfg.tunName,
		tunIPv6:    p.tunIPv6,
		tunIPv4:    p.tunIPv4,
		tunIPv4Net: p.tunIPv4Net,
		dnsAddrs:   p.dnsAddrs,
		dnsZones:   targetOSConfig.GetDnsZones(),
		cidrRanges: targetOSConfig.GetIpv4CidrRanges(),
	}, nil
}

func (p *osConfigProvider) setV4IPsFromFirstCIDR(cidrRange string) error {
	if p.tunIPv4 != "" {
		// Only set these once.
		return nil
	}
	tunIPv4, tunIPv4Net, dnsIPv4, err := ipsForCIDR(cidrRange)
	if err != nil {
		return trace.Wrap(err, "setting TUN IPv4 address for range %s", cidrRange)
	}
	if err := p.cfg.addDNSAddress(dnsIPv4); err != nil {
		return trace.Wrap(err, "adding IPv4 DNS server at %s", dnsIPv4.String())
	}
	p.tunIPv4 = tunIPv4.String()
	p.tunIPv4Net = tunIPv4Net
	p.dnsAddrs = append(p.dnsAddrs, dnsIPv4.String())
	return nil
}

// ipsForCIDR returns the V4 IPs to assign to the interface and use for DNS in
// cidrRange.
func ipsForCIDR(cidrRange string) (tunIP net.IP, tunIPNet *net.IPNet, dnsIP net.IP, err error) {
	_, tunIPNet, err = net.ParseCIDR(cidrRange)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err, "parsing CIDR %q", cidrRange)
	}
	// tunIPNet.IP is the network address, ending in 0s, like 100.64.0.0
	// Add 1 to assign the TUN address, like 100.64.0.1
	tunIP = slices.Clone(tunIPNet.IP)
	tunIP[len(tunIP)-1]++

	// Add 1 again to assign the DNS address, like 100.64.0.2
	dnsIP = slices.Clone(tunIP)
	dnsIP[len(dnsIP)-1]++

	return tunIP, tunIPNet, dnsIP, nil
}
