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

package systemdresolved

import (
	"context"
	"net"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"
)

// CheckAvailability returns an error if systemd-resolved is unavailable on D-Bus.
func CheckAvailability(ctx context.Context, conn *dbus.Conn) error {
	var hasOwner bool
	err := conn.Object(DBusService, dbus.ObjectPath(DBusObjectPath)).
		CallWithContext(ctx, DBusNameHasOwner, 0, Service).
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
		Service,
	)
}

// Object returns the systemd-resolved D-Bus object.
func Object(conn *dbus.Conn) dbus.BusObject {
	return conn.Object(Service, dbus.ObjectPath(ObjectPath))
}

// LoadConfiguredDNSServers returns DNS servers currently configured in systemd-resolved.
func LoadConfiguredDNSServers(ctx context.Context, conn *dbus.Conn) ([]DNS, error) {
	return loadDNSProperty(ctx, conn)
}

// loadDNSProperty loads the systemd-resolved DNS property via D-Bus.
func loadDNSProperty(ctx context.Context, conn *dbus.Conn) ([]DNS, error) {
	call := Object(conn).CallWithContext(ctx, DBusPropertiesGet, 0, Manager, DNSProperty)
	if call.Err != nil {
		return nil, trace.Wrap(call.Err, "getting systemd-resolved property %s", DNSProperty)
	}

	var variant dbus.Variant
	if err := call.Store(&variant); err != nil {
		return nil, trace.Wrap(err, "decoding systemd-resolved property %s", DNSProperty)
	}

	var dns []DNS
	if err := dbus.Store([]any{variant.Value()}, &dns); err != nil {
		return nil, trace.Wrap(err, "decoding systemd-resolved property %s", DNSProperty)
	}
	return dns, nil
}

// SetLinkDomains configures per-link DNS search domains.
func SetLinkDomains(ctx context.Context, conn *dbus.Conn, ifaceIndex int32, domains []Domain) error {
	call := Object(conn).CallWithContext(ctx, SetDomainsMethod, 0, ifaceIndex, domains)
	if call.Err != nil {
		return trace.Wrap(call.Err, "setting systemd-resolved link domains")
	}
	return nil
}

// SetLinkDefaultRoute configures whether this link is the default DNS route.
func SetLinkDefaultRoute(ctx context.Context, conn *dbus.Conn, ifaceIndex int32, enabled bool) error {
	call := Object(conn).CallWithContext(ctx, SetDefaultRouteMethod, 0, ifaceIndex, enabled)
	if call.Err != nil {
		return trace.Wrap(call.Err, "setting systemd-resolved link default route")
	}
	return nil
}

// SetLinkDNS configures per-link DNS servers.
func SetLinkDNS(ctx context.Context, conn *dbus.Conn, ifaceIndex int32, addresses []DNSAddress) error {
	call := Object(conn).CallWithContext(ctx, SetLinkDNSMethod, 0, ifaceIndex, addresses)
	if call.Err != nil {
		return trace.Wrap(call.Err, "setting systemd-resolved link DNS")
	}
	return nil
}

// DNSAddressForIP converts an IP adress into a systemd-resolved DNSAddress.
func DNSAddressForIP(raw string) (DNSAddress, error) {
	ip := net.ParseIP(raw)
	if ip == nil {
		return DNSAddress{}, trace.Errorf("invalid IP address")
	}
	if ip4 := ip.To4(); ip4 != nil {
		return DNSAddress{
			Family:  syscall.AF_INET,
			Address: []byte(ip4),
		}, nil
	}
	if ip16 := ip.To16(); ip16 != nil {
		return DNSAddress{
			Family:  syscall.AF_INET6,
			Address: []byte(ip16),
		}, nil
	}
	return DNSAddress{}, trace.Errorf("unsupported IP address")
}
