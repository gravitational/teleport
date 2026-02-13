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
	"net/netip"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"
)

const (
	systemdResolvedService    = "org.freedesktop.resolve1"
	systemdResolvedObjectPath = "/org/freedesktop/resolve1"
	systemdResolvedManager    = "org.freedesktop.resolve1.Manager"

	systemdResolvedDNSProperty = "DNS"
)

type systemdResolvedDNS struct {
	InterfaceIndex int32
	Family         int32
	Address        []byte
}

// platformLoadUpstreamNameservers returns the list of DNS upstreams configured in systemd-resolved.
func platformLoadUpstreamNameservers(ctx context.Context) ([]string, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, trace.NotFound("system D-Bus is unavailable: %v", err)
	}
	defer conn.Close()

	var hasOwner bool
	err = conn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus").
		CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, systemdResolvedService).
		Store(&hasOwner)
	if err != nil {
		return nil, trace.Wrap(err, "checking systemd-resolved D-Bus service owner")
	}
	if !hasOwner {
		return nil, trace.Errorf("systemd-resolved D-Bus service %s is not available", systemdResolvedService)
	}

	obj := conn.Object(systemdResolvedService, dbus.ObjectPath(systemdResolvedObjectPath))
	call := obj.CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, systemdResolvedManager, systemdResolvedDNSProperty)
	if call.Err != nil {
		return nil, trace.Wrap(call.Err, "getting systemd-resolved property %s", systemdResolvedDNSProperty)
	}

	var variant dbus.Variant
	if err := call.Store(&variant); err != nil {
		return nil, trace.Wrap(err, "decoding systemd-resolved property %s", systemdResolvedDNSProperty)
	}

	var dns []systemdResolvedDNS
	if err := dbus.Store([]any{variant.Value()}, &dns); err != nil {
		return nil, trace.Wrap(err, "decoding systemd-resolved property %s", systemdResolvedDNSProperty)
	}

	nameservers := make([]string, 0, len(dns))
	for _, entry := range dns {
		addr, ok := netip.AddrFromSlice(entry.Address)
		if !ok {
			slog.DebugContext(ctx, "Skipping invalid DNS server address", "address_bytes", entry.Address)
			continue
		}
		if addr.IsUnspecified() {
			continue
		}
		nameservers = append(nameservers, WithDNSPort(addr))
	}

	slog.DebugContext(ctx, "Loaded host upstream nameservers", "nameservers", nameservers)
	return nameservers, nil
}
