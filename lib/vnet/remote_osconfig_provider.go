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

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// remoteOSConfigProvider implements targetOSConfigProvider when the admin
// service fetches cluster DNS zones and CIDR ranges from the client application
// process over gRPC. It is currently used in the Windows admin service.
type remoteOSConfigProvider struct {
	clt     targetOSConfigGetter
	tunName string
	dnsAddr string
	tunIPv6 string
	tunIPv4 string
}

type targetOSConfigGetter interface {
	GetTargetOSConfiguration(context.Context) (*vnetv1.TargetOSConfiguration, error)
}

func newRemoteOSConfigProvider(clt targetOSConfigGetter, tunName, ipv6Prefix, dnsAddr string) (*remoteOSConfigProvider, error) {
	tunIPv6, err := tunIPv6ForPrefix(ipv6Prefix)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &remoteOSConfigProvider{
		clt:     clt,
		tunName: tunName,
		dnsAddr: dnsAddr,
		tunIPv6: tunIPv6,
	}, nil
}

func (p *remoteOSConfigProvider) targetOSConfig(ctx context.Context) (*osConfig, error) {
	targetOSConfig, err := p.clt.GetTargetOSConfiguration(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting target OS configuration from client application")
	}
	if p.tunIPv4 == "" && len(targetOSConfig.Ipv4CidrRanges) > 0 {
		// Choose an IPv4 address for the TUN interface from the CIDR range of one arbitrary currently
		// logged-in cluster. Only one IPv4 address is needed.
		if err := p.setTunIPv4FromCIDR(targetOSConfig.Ipv4CidrRanges[0]); err != nil {
			return nil, trace.Wrap(err, "setting TUN IPv4 address")
		}
	}
	return &osConfig{
		tunName:    p.tunName,
		tunIPv6:    p.tunIPv6,
		tunIPv4:    p.tunIPv4,
		dnsAddr:    p.dnsAddr,
		dnsZones:   targetOSConfig.GetDnsZones(),
		cidrRanges: targetOSConfig.GetIpv4CidrRanges(),
	}, nil
}

func (p *remoteOSConfigProvider) setTunIPv4FromCIDR(cidrRange string) error {
	if p.tunIPv4 != "" {
		return nil
	}
	ip, err := tunIPv4ForCIDR(cidrRange)
	if err != nil {
		return trace.Wrap(err, "setting TUN IPv4 address for range %s", cidrRange)
	}
	p.tunIPv4 = ip
	return nil
}
