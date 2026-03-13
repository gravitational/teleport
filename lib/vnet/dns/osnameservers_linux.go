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

package dns

import (
	"context"
	"log/slog"
	"net"
	"net/netip"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/vnet/systemdresolved"
)

// platformLoadUpstreamNameservers returns the list of DNS upstreams configured in systemd-resolved.
func platformLoadUpstreamNameservers(ctx context.Context, slog *slog.Logger) ([]string, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()
	if err := systemdresolved.CheckAvailability(ctx, conn); err != nil {
		return nil, err
	}

	dns, err := systemdresolved.LoadConfiguredDNSServers(ctx, conn)
	if err != nil {
		return nil, err
	}

	nameservers := make([]string, 0, len(dns))
	for _, entry := range dns {
		addr, ok := netip.AddrFromSlice(entry.Address)
		if !ok {
			slog.DebugContext(ctx, "Skipping invalid DNS server address", "address_bytes", entry.Address)
			continue
		}
		if addr.IsUnspecified() || addr.IsLoopback() {
			continue
		}
		// Link-local IPv6 addresses require a scope to be routable.
		if addr.Is6() && addr.IsLinkLocalUnicast() {
			if entry.InterfaceIndex == 0 {
				slog.DebugContext(ctx, "Skipping link-local DNS server without interface index", "address", addr.String())
				continue
			}
			iface, err := net.InterfaceByIndex(int(entry.InterfaceIndex))
			if err != nil {
				slog.DebugContext(ctx, "Skipping link-local DNS server with unknown interface", "address", addr.String(), "interface_index", entry.InterfaceIndex, "error", err)
				continue
			}
			addr = addr.WithZone(iface.Name)
		}
		nameservers = append(nameservers, AddrWithDNSPort(addr))
	}

	slog.DebugContext(ctx, "Loaded host upstream nameservers", "nameservers", nameservers)
	return nameservers, nil
}
