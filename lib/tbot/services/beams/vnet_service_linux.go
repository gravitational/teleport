//go:build linux

// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package beams

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/tun"

	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

func platformCreateTUN() (vnet.TUNDevice, error) {
	const mtu = 1500
	tun, err := tun.CreateTUN("vnet", mtu)
	if err != nil {
		return nil, trace.Wrap(err, "creating TUN device")
	}
	return tun, nil
}

func platformUpstreamNameserverSource(logger *slog.Logger) (dns.UpstreamNameserverSource, error) {
	src, err := dns.CachingUpstreamNameserverSource(
		dns.ResolvConfUpstreamNameserverSource(logger),
		10*time.Minute, // The /etc/resolv.conf file is essentially static in the Beam environment.
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating upstream nameserver source")
	}
	return src, nil
}

func platformConfigureHost(_ context.Context, device vnet.TUNDevice, cfg *vnet.EmbeddedVNetHostConfig) error {
	deviceName, err := device.Name()
	if err != nil {
		return trace.Wrap(err, "getting device name")
	}
	link, err := netlink.LinkByName(deviceName)
	if err != nil {
		return trace.Wrap(err, "resolving device link")
	}

	// Add IPv4 address for the TUN device.
	linkAddrV4 := &netlink.Addr{
		IPNet: netlink.NewIPNet(net.ParseIP(cfg.DeviceIPv4)),
	}
	if err := netlink.AddrAdd(link, linkAddrV4); err != nil {
		return trace.Wrap(err, "adding IPv4 address %s to TUN device", cfg.DeviceIPv4)
	}

	// Add IPv6 address for the TUN device.
	linkAddrV6 := &netlink.Addr{
		IPNet: netlink.NewIPNet(net.ParseIP(cfg.DeviceIPv6)),
	}
	if err := netlink.AddrAdd(link, linkAddrV6); err != nil {
		return trace.Wrap(err, "adding IPv6 address %s to TUN device", cfg.DeviceIPv6)
	}

	// Bring the TUN device up.
	if err := netlink.LinkSetUp(link); err != nil {
		return trace.Wrap(err, "bringing TUN device up")
	}

	// Update the routing table.
	linkIdx := link.Attrs().Index
	for _, cidr := range cfg.CIDRRanges {
		dst, err := netlink.ParseIPNet(cidr)
		if err != nil {
			return trace.Wrap(err, "parsing CIDR range %s", cidr)
		}
		route := &netlink.Route{
			LinkIndex: linkIdx,
			Dst:       dst,
		}
		if err := netlink.RouteReplace(route); err != nil {
			return trace.Wrap(err, "adding route from %s to TUN device", cidr)
		}
	}
	return nil
}
