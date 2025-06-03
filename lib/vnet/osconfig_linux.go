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

	"github.com/gravitational/trace"
)

type platformOSConfigState struct {
	setupIPv6          bool
	configuredDNS      bool
	broughtUpInterface bool
}

// platformConfigureOS configures the host OS according to cfg. It is safe to
// call repeatedly, and it is meant to be called with an empty osConfig to
// deconfigure anything necessary before exiting.
func platformConfigureOS(ctx context.Context, cfg *osConfig, state *platformOSConfigState) error {
	if cfg.tunIPv6 != "" && !state.setupIPv6 {
		log.InfoContext(ctx, "Setting IPv6 address for the TUN device.", "device", cfg.tunName, "address", cfg.tunIPv6)
		addrWithPrefix := cfg.tunIPv6 + "/64"
		if err := runCommand(ctx,
			"ip", "addr", "add", addrWithPrefix, "dev", cfg.tunName,
		); err != nil {
			return trace.Wrap(err)
		}
		state.setupIPv6 = true
	}
	if cfg.dnsAddr != "" && !state.configuredDNS {
		log.InfoContext(ctx, "Configuring DNS")
		if err := runCommand(ctx,
			"resolvectl", "dns", cfg.tunName, cfg.dnsAddr,
		); err != nil {
			return trace.Wrap(err)
		}
		domains := make([]string, 0, len(cfg.dnsZones))
		for _, dnsZone := range cfg.dnsZones {
			domains = append(domains, "~"+dnsZone)
		}
		args := append([]string{"domain", cfg.tunName}, domains...)
		if err := runCommand(ctx, "resolvectl", args...); err != nil {
			return trace.Wrap(err)
		}
		state.configuredDNS = true
	}
	if state.setupIPv6 && state.configuredDNS && !state.broughtUpInterface {
		log.InfoContext(ctx, "Bringing up the VNet interface", "device", cfg.tunName)
		if err := runCommand(ctx,
			"ip", "link", "set", cfg.tunName, "up",
		); err != nil {
			return trace.Wrap(err)
		}
		state.broughtUpInterface = true
	}
	return nil
}
