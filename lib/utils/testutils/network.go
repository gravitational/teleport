/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package testutils

import (
	"net"

	"github.com/gravitational/trace"
)

// NonLocalhostLocalInterfaceIP returns the IP address of a non-loopback and non-link-local unicast local interface.
func NonLocalhostLocalInterfaceIP() (string, error) {
	localInterfaces, err := net.Interfaces()
	if err != nil {
		return "", trace.Wrap(err)
	}

	for _, iface := range localInterfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", trace.Wrap(err)
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.To4() == nil {
				continue
			}

			return ip.String(), nil
		}
	}

	return "", trace.NotFound("non-loopback interfaces found")
}
