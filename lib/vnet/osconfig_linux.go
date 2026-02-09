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
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
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

const (
	systemdResolvedService         = "org.freedesktop.resolve1"
	systemdResolvedObjectPath      = "/org/freedesktop/resolve1"
	systemdResolvedManager         = "org.freedesktop.resolve1.Manager"
	systemdResolvedSetLinkDNS      = systemdResolvedManager + ".SetLinkDNS"
	systemdResolvedSetDomains      = systemdResolvedManager + ".SetLinkDomains"
	systemdResolvedSetDefaultRoute = systemdResolvedManager + ".SetLinkDefaultRoute"
)

type systemdResolvedDNSAddress struct {
	Family  int32
	Address []byte
}

type systemdResolvedDomain struct {
	Domain      string
	RoutingOnly bool
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

	if err := checkSystemdResolvedAvailability(ctx); err != nil {
		return err
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()
	obj := conn.Object(systemdResolvedService, dbus.ObjectPath(systemdResolvedObjectPath))

	if shouldReconfiguredDNSZones(cfg, state) {
		iface, err := net.InterfaceByName(state.tunName)
		if err != nil {
			return trace.Wrap(err, "looking up interface %s", state.tunName)
		}
		log.InfoContext(ctx, "Configuring DNS zones", "zones", cfg.dnsZones)
		domains := make([]systemdResolvedDomain, 0, len(cfg.dnsZones))
		for _, dnsZone := range cfg.dnsZones {
			domains = append(domains, systemdResolvedDomain{
				Domain:      dnsZone,
				RoutingOnly: true,
			})
		}
		// Equivalent to: resolvectl domain <ifname> ~<zone1> ~<zone2> ...
		call := obj.CallWithContext(ctx, systemdResolvedSetDomains, 0, int32(iface.Index), domains)
		if call.Err != nil {
			return trace.Wrap(call.Err, "setting systemd-resolved link domains")
		}
		state.configuredDNSZones = cfg.dnsZones
	}

	if len(cfg.dnsAddrs) > 0 && state.tunName != "" && !state.configuredNameserver {
		iface, err := net.InterfaceByName(state.tunName)
		if err != nil {
			return trace.Wrap(err, "looking up interface %s", state.tunName)
		}
		addresses := make([]systemdResolvedDNSAddress, 0, len(cfg.dnsAddrs))
		for _, addr := range cfg.dnsAddrs {
			address, err := systemdResolvedDNSAddressForIP(addr)
			if err != nil {
				return trace.Wrap(err, "parsing DNS nameserver %q", addr)
			}
			addresses = append(addresses, address)
		}
		log.InfoContext(ctx, "Configuring DNS nameserver", "nameservers", cfg.dnsAddrs)
		// Equivalent to: resolvectl default-route <ifname> false
		call := obj.CallWithContext(ctx, systemdResolvedSetDefaultRoute, 0, int32(iface.Index), false)
		if call.Err != nil {
			return trace.Wrap(call.Err, "setting systemd-resolved link default route")
		}

		// Equivalent to: resolvectl dns <ifname> <addr1> <addr2> ...
		call = obj.CallWithContext(ctx, systemdResolvedSetLinkDNS, 0, int32(iface.Index), addresses)
		if call.Err != nil {
			return trace.Wrap(call.Err, "setting systemd-resolved link DNS")
		}
		state.configuredNameserver = true
	}

	return nil
}

func systemdResolvedDNSAddressForIP(raw string) (systemdResolvedDNSAddress, error) {
	ip := net.ParseIP(raw)
	if ip == nil {
		return systemdResolvedDNSAddress{}, trace.BadParameter("invalid IP address")
	}
	if ip4 := ip.To4(); ip4 != nil {
		return systemdResolvedDNSAddress{
			Family:  syscall.AF_INET,
			Address: []byte(ip4),
		}, nil
	}
	if ip16 := ip.To16(); ip16 != nil {
		return systemdResolvedDNSAddress{
			Family:  syscall.AF_INET6,
			Address: []byte(ip16),
		}, nil
	}
	return systemdResolvedDNSAddress{}, trace.BadParameter("unsupported IP address")
}

func checkSystemdResolvedAvailability(ctx context.Context) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return trace.Wrap(err, "system D-Bus is unavailable")
	}
	defer conn.Close()

	var hasOwner bool
	err = conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, systemdResolvedService).
		Store(&hasOwner)
	if err != nil {
		return trace.Wrap(err, "checking systemd-resolved D-Bus service owner")
	}
	if hasOwner {
		return nil
	}

	return trace.Errorf(
		"systemd-resolved is not running (D-Bus service %s has no owner).\n"+
			"you can enable it with:\n"+
			"  sudo systemctl enable --now systemd-resolved\n",
		systemdResolvedService,
	)
}
