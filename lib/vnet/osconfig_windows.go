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
	"strconv"

	"github.com/gravitational/trace"
)

// platformOSConfigState holds state about which addresses and routes have
// already been configured in the OS. Experimentally, IP routing seems to be
// flaky/broken on Windows when the same routes are repeatedly configured, as we
// currently do on MacOS. Avoid this by only configuring each IP or route once.
//
// TODO(nklaassen): it would probably be better to read the current routing
// table from the OS, compute a diff, and reconcile the routes that we need.
// This works for now but if something else overwrites our deletes our routes,
// we'll never reset them.
type platformOSConfigState struct {
	configuredV4Address bool
	configuredV6Address bool
	configuredRanges    []string

	ifaceIndex string
}

func (p *platformOSConfigState) getIfaceIndex() (string, error) {
	if p.ifaceIndex != "" {
		return p.ifaceIndex, nil
	}
	iface, err := net.InterfaceByName(tunInterfaceName)
	if err != nil {
		return "", trace.Wrap(err, "looking up TUN interface by name %s", tunInterfaceName)
	}
	p.ifaceIndex = strconv.Itoa(iface.Index)
	return p.ifaceIndex, nil
}

// platformConfigureOS configures the host OS according to cfg. It is safe to
// call repeatedly, and it is meant to be called with an empty osConfig to
// deconfigure anything necessary before exiting.
func platformConfigureOS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
	// There is no need to remove IP addresses or routes, they will automatically be cleaned up when the
	// process exits and the TUN is deleted.

	if cfg.tunIPv4 != "" {
		if !state.configuredV4Address {
			log.InfoContext(ctx, "Setting IPv4 address for the TUN device",
				"device", cfg.tunName, "address", cfg.tunIPv4)
			if err := runCommand(ctx,
				"netsh", "interface", "ip", "set", "address", cfg.tunName, "static", cfg.tunIPv4,
			); err != nil {
				return trace.Wrap(err)
			}
			state.configuredV4Address = true
		}
		for _, cidrRange := range cfg.cidrRanges {
			if slices.Contains(state.configuredRanges, cidrRange) {
				continue
			}
			log.InfoContext(ctx, "Routing CIDR range to the TUN IP",
				"device", cfg.tunName, "range", cidrRange)
			ifaceIndex, err := state.getIfaceIndex()
			if err != nil {
				return trace.Wrap(err, "getting index for TUN interface")
			}
			addr, mask, err := addrMaskForCIDR(cidrRange)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := runCommand(ctx,
				"route", "add", addr, "mask", mask, cfg.tunIPv4, "if", ifaceIndex,
			); err != nil {
				return trace.Wrap(err)
			}
			state.configuredRanges = append(state.configuredRanges, cidrRange)
		}
	}

	if cfg.tunIPv6 != "" && !state.configuredV6Address {
		// It looks like we don't need to explicitly set a route for the IPv6
		// ULA prefix, assigning the address to the interface is enough.
		log.InfoContext(ctx, "Setting IPv6 address for the TUN device.",
			"device", cfg.tunName, "address", cfg.tunIPv6)
		if err := runCommand(ctx,
			"netsh", "interface", "ipv6", "set", "address", cfg.tunName, cfg.tunIPv6,
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredV6Address = true
	}

	// TODO(nklaassen): configure DNS on Windows.

	return nil
}

// addrMaskForCIDR returns the base address and the bitmask for a given CIDR
// range. The "route add" command wants the mask in dotted decimal format, e.g.
// for 100.64.0.0/10 the mask should be 255.192.0.0
func addrMaskForCIDR(cidr string) (string, string, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", trace.Wrap(err, "parsing CIDR range %s", cidr)
	}
	return ipNet.IP.String(), net.IP(ipNet.Mask).String(), nil
}
