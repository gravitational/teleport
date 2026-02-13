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

const (
	DBusService       = "org.freedesktop.DBus"
	DBusObjectPath    = "/org/freedesktop/DBus"
	DBusNameHasOwner  = "org.freedesktop.DBus.NameHasOwner"
	DBusPropertiesGet = "org.freedesktop.DBus.Properties.Get"

	Service    = "org.freedesktop.resolve1"
	ObjectPath = "/org/freedesktop/resolve1"
	Manager    = "org.freedesktop.resolve1.Manager"

	SetLinkDNSMethod      = Manager + ".SetLinkDNS"
	SetDomainsMethod      = Manager + ".SetLinkDomains"
	SetDefaultRouteMethod = Manager + ".SetLinkDefaultRoute"

	DNSProperty = "DNS"
)
