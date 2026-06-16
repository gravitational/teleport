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
	"github.com/jsimonetti/rtnetlink/v2"
	"golang.org/x/sys/unix"
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

func platformConfigureHost(ctx context.Context, device vnet.TUNDevice, cfg *vnet.EmbeddedVNetHostConfig) error {
	deviceName, err := device.Name()
	if err != nil {
		return trace.Wrap(err, "getting device name")
	}
	iface, err := net.InterfaceByName(deviceName)
	if err != nil {
		return trace.Wrap(err, "resolving TUN device interface")
	}

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return trace.Wrap(err, "dialing route netlink")
	}
	defer conn.Close()

	if err := addInterfaceAddress(ctx, conn, iface.Index, cfg.DeviceIPv4); err != nil {
		return trace.Wrap(err, "adding IPv4 address %s to TUN device", cfg.DeviceIPv4)
	}

	if err := addInterfaceAddress(ctx, conn, iface.Index, cfg.DeviceIPv6); err != nil {
		return trace.Wrap(err, "adding IPv6 address %s to TUN device", cfg.DeviceIPv6)
	}

	if err := setInterfaceUp(ctx, conn, iface.Index); err != nil {
		return trace.Wrap(err, "bringing TUN device up")
	}

	for _, cidr := range cfg.CIDRRanges {
		if err := replaceRoute(ctx, conn, iface.Index, cidr); err != nil {
			return trace.Wrap(err, "adding route from %s to TUN device", cidr)
		}
	}
	return nil
}

func addInterfaceAddress(ctx context.Context, conn *rtnetlink.Conn, linkIndex int, address string) error {
	family, addr, prefixLen, err := parseInterfaceAddress(address)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(conn.Address.New(&rtnetlink.AddressMessage{
		Family:       family,
		PrefixLength: prefixLen,
		Index:        uint32(linkIndex),
		Attributes: &rtnetlink.AddressAttributes{
			Address: addr,
			Local:   addr,
		},
	}))
}

func setInterfaceUp(ctx context.Context, conn *rtnetlink.Conn, linkIndex int) error {
	return trace.Wrap(conn.Link.Set(&rtnetlink.LinkMessage{
		Family: unix.AF_UNSPEC,
		Index:  uint32(linkIndex),
		Flags:  unix.IFF_UP,
		Change: unix.IFF_UP,
	}))
}

func replaceRoute(ctx context.Context, conn *rtnetlink.Conn, linkIndex int, cidr string) error {
	family, dst, prefixLen, err := parseRouteDestination(cidr)
	if err != nil {
		return trace.Wrap(err, "parsing CIDR range")
	}
	return trace.Wrap(conn.Route.Replace(&rtnetlink.RouteMessage{
		Family:    family,
		DstLength: prefixLen,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  unix.RTPROT_BOOT,
		Scope:     unix.RT_SCOPE_UNIVERSE,
		Type:      unix.RTN_UNICAST,
		Attributes: rtnetlink.RouteAttributes{
			Dst:      dst,
			OutIface: uint32(linkIndex),
		},
	}))
}

func parseInterfaceAddress(address string) (family uint8, addr net.IP, prefixLen uint8, err error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return 0, nil, 0, trace.BadParameter("invalid IP address %q", address)
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return unix.AF_INET, ipv4, 32, nil
	}
	return unix.AF_INET6, ip.To16(), 128, nil
}

func parseRouteDestination(cidr string) (family uint8, dst net.IP, prefixLen uint8, err error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return 0, nil, 0, trace.Wrap(err)
	}

	ones, bits := ipNet.Mask.Size()
	if ones < 0 {
		return 0, nil, 0, trace.BadParameter("invalid CIDR mask %q", cidr)
	}

	if ipv4 := ipNet.IP.To4(); ipv4 != nil {
		if bits != net.IPv4len*8 {
			return 0, nil, 0, trace.BadParameter("invalid IPv4 CIDR mask size %d for %q", bits, cidr)
		}
		return unix.AF_INET, ipv4, uint8(ones), nil
	}

	ipv6 := ipNet.IP.To16()
	if ipv6 == nil || bits != net.IPv6len*8 {
		return 0, nil, 0, trace.BadParameter("invalid IPv6 CIDR %q", cidr)
	}
	return unix.AF_INET6, ipv6, uint8(ones), nil
}
