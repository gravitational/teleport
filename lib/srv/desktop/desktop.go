/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// Package desktop implements Desktop Access services, like
// windows_desktop_access.
package desktop

import "github.com/gravitational/teleport/api/constants"

const (
	// SNISuffix is the server name suffix used during SNI to specify the
	// target desktop to connect to. The client (proxy_service) will use SNI
	// like "${UUID}.desktop.teleport.cluster.local" to pass the UUID of the
	// desktop.
	SNISuffix = ".desktop." + constants.APIDomain
	// WildcardServiceDNS is a wildcard DNS address to embed in the service TLS
	// certificate for SNI-based routing. Note: this is different from ALPN SNI
	// routing on the proxy.
	WildcardServiceDNS = "*" + SNISuffix
)
