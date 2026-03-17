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

package vnet

// VNet's D-Bus daemon claims the org.teleport.vnet1 service name
// and exposes the org.teleport.vnet1.Daemon interface, which has Start
// and Stop methods. For it to work the system must include:
//   - /usr/share/dbus-1/system.d/org.teleport.vnet1.conf to allow the daemon to
//     claim the org.teleport.vnet1 name on the system bus.
//   - /usr/share/dbus-1/system-services/org.teleport.vnet1.service to enable
//     D-Bus activation of the daemon.
//   - /usr/lib/systemd/system/teleport-vnet.service to manage the daemon
//     lifecycle under systemd.
//   - /usr/share/polkit-1/actions/org.teleport.vnet1.policy to define who can
//     perform the org.teleport.vnet1.manage-daemon action (and therefore call
//     Start and Stop methods).
//
// The daemon is managed by systemd. we don’t use systemd directly
// because `systemctl start` takes only unit names and doesn’t let us pass
// per start args. the closest options are template units or environment
// file, but those are clunky so we wrap the admin process in a D-Bus daemon
// and expose Start with explicit parameters.
const (
	vnetDBusServiceName = "org.teleport.vnet1"
	vnetDBusObjectPath  = "/org/teleport/vnet1"
	vnetDBusInterface   = "org.teleport.vnet1.Daemon"
	vnetDBusStartMethod = vnetDBusInterface + ".Start"
	vnetDBusStopMethod  = vnetDBusInterface + ".Stop"
	// vnetPolkitAction must match the action ID defined in the polkit policy file.
	vnetPolkitAction    = "org.teleport.vnet1.manage-daemon"
	vnetSystemdUnitName = "teleport-vnet.service"
)
