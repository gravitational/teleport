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
	"net"
	"slices"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/vnet/systemdresolved"
)

type platformOSConfigState struct {
	configuredIPv6       bool
	configuredIPv4       bool
	configuredCidrRanges []string
	configuredNameserver bool
	configuredDNSZones   []string
	broughtUpInterface   bool
	tunName              string
}

// platformConfigureOS configures the host OS according to cfg.
func platformConfigureOS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
	// TODO: use github.com/vishvananda/netlink to set up IPs and routes?
	if cfg.tunIPv6 != "" && !state.configuredIPv6 {
		log.InfoContext(ctx, "Setting IPv6 address for the TUN device.", "device", cfg.tunName, "address", cfg.tunIPv6)
		addrWithPrefix := cfg.tunIPv6 + "/64"
		if err := runCommand(ctx,
			"ip", "addr", "add", addrWithPrefix, "dev", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredIPv6 = true
	}
	if cfg.tunIPv4 != "" && !state.configuredIPv4 {
		log.InfoContext(ctx, "Setting IPv4 address for the TUN device.",
			"device", cfg.tunName, "address", cfg.tunIPv4)
		if err := runCommand(ctx,
			"ip", "addr", "add", cfg.tunIPv4, "dev", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredIPv4 = true
	}
	if err := configureDNS(ctx, cfg, state); err != nil {
		return trace.Wrap(err, "configuring DNS")
	}
	if (state.configuredIPv4 || state.configuredIPv6) && state.configuredNameserver && !state.broughtUpInterface {
		log.InfoContext(ctx, "Bringing up the VNet interface", "device", cfg.tunName)
		if err := runCommand(ctx,
			"ip", "link", "set", cfg.tunName, "up",
		); err != nil {
			return trace.Wrap(err)
		}
		state.broughtUpInterface = true
	}
	if cfg.tunIPv4 != "" && state.configuredIPv4 && state.broughtUpInterface {
		for _, cidrRange := range cfg.cidrRanges {
			if slices.Contains(state.configuredCidrRanges, cidrRange) {
				continue
			}
			log.InfoContext(ctx, "Setting an IPv4 route", "netmask", cidrRange)
			if err := runCommand(ctx,
				"ip", "route", "add", cidrRange, "dev", cfg.tunName,
			); err != nil {
				return trace.Wrap(err)
			}
			state.configuredCidrRanges = append(state.configuredCidrRanges, cidrRange)
		}
	}
	return nil
}

func shouldReconfiguredDNSZones(cfg *osConfig, state *platformOSConfigState) bool {
	return !utils.ContainSameUniqueElements(cfg.dnsZones, state.configuredDNSZones)
}

func configureDNS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
	// systemd-resolved stores DNS settings per network link. For VNet
	// we configure DNS on the TUN link. The TUN is ephemeral, when
	// the admin process exits and the TUN interface is deleted,
	// systemd-resolved deletes the link and its DNS configuration.
	// So we don't need additional DNS cleanup on restart
	if cfg.tunName != "" {
		state.tunName = cfg.tunName
	}
	if len(cfg.dnsAddrs) == 0 && len(cfg.dnsZones) > 0 {
		return trace.BadParameter("empty nameserver with non-empty zones")
	}
	if len(cfg.dnsAddrs) > 0 && cfg.tunName == "" {
		return trace.BadParameter("empty TUN interface name with non-empty nameserver")
	}

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()
	if err := systemdresolved.CheckAvailability(ctx, conn); err != nil {
		return err
	}

	if shouldReconfiguredDNSZones(cfg, state) {
		iface, err := net.InterfaceByName(state.tunName)
		if err != nil {
			return trace.Wrap(err, "looking up interface %s", state.tunName)
		}
		log.InfoContext(ctx, "Configuring DNS zones", "zones", cfg.dnsZones)
		domains := make([]systemdresolved.Domain, 0, len(cfg.dnsZones))
		for _, dnsZone := range cfg.dnsZones {
			domains = append(domains, systemdresolved.Domain{
				Domain:      dnsZone,
				RoutingOnly: true,
			})
		}
		// Equivalent to: resolvectl domain <ifname> ~<zone1> ~<zone2> ...
		if err := systemdresolved.SetLinkDomains(ctx, conn, int32(iface.Index), domains); err != nil {
			return err
		}
		state.configuredDNSZones = cfg.dnsZones
	}

	if len(cfg.dnsAddrs) > 0 && state.tunName != "" && !state.configuredNameserver {
		iface, err := net.InterfaceByName(state.tunName)
		if err != nil {
			return trace.Wrap(err, "looking up interface %s", state.tunName)
		}
		addresses := make([]systemdresolved.DNSAddress, 0, len(cfg.dnsAddrs))
		for _, addr := range cfg.dnsAddrs {
			address, err := systemdresolved.DNSAddressForIP(addr)
			if err != nil {
				return trace.Wrap(err, "parsing DNS nameserver %q", addr)
			}
			addresses = append(addresses, address)
		}
		log.InfoContext(ctx, "Configuring DNS nameserver", "nameservers", cfg.dnsAddrs)
		// Equivalent to: resolvectl default-route <ifname> false
		if err := systemdresolved.SetLinkDefaultRoute(ctx, conn, int32(iface.Index), false); err != nil {
			return err
		}

		// Equivalent to: resolvectl dns <ifname> <addr1> <addr2> ...
		if err := systemdresolved.SetLinkDNS(ctx, conn, int32(iface.Index), addresses); err != nil {
			return err
		}
		state.configuredNameserver = true
	}

	return nil
}
