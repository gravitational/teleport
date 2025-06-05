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
	"slices"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

type platformOSConfigState struct {
	configuredIPv6       bool
	configuredIPv4       bool
	configuredCidrRanges []string
	configuredNameserver bool
	configuredDNSZones   []string
	broughtUpInterface   bool
}

// platformConfigureOS configures the host OS according to cfg. It is safe to
// call repeatedly, and it is meant to be called with an empty osConfig to
// deconfigure anything necessary before exiting.
func platformConfigureOS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
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
	if cfg.dnsAddr != "" && !state.configuredNameserver {
		log.InfoContext(ctx, "Configuring DNS nameserver", "nameserver", cfg.dnsAddr)
		if err := runCommand(ctx,
			"resolvectl", "dns", cfg.tunName, cfg.dnsAddr,
		); err != nil {
			return trace.Wrap(err)
		}
		state.configuredNameserver = true
	}
	if shouldReconfiguredDNSZones(cfg, state) {
		log.InfoContext(ctx, "Configuring DNS zones", "zones", cfg.dnsZones)
		domains := make([]string, 0, len(cfg.dnsZones))
		for _, dnsZone := range cfg.dnsZones {
			domains = append(domains, "~"+dnsZone)
		}
		args := append([]string{"domain", cfg.tunName}, domains...)
		if err := runCommand(ctx, "resolvectl", args...); err != nil {
			return trace.Wrap(err)
		}
		state.configuredDNSZones = cfg.dnsZones
	}
	if state.configuredIPv6 && state.configuredNameserver && !state.broughtUpInterface {
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
				"ip", "route", "add", cidrRange, "via", cfg.tunIPv4, "dev", cfg.tunName,
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
