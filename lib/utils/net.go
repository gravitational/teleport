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

package utils

import (
	"net"
	"slices"
	"strings"

	"github.com/gravitational/trace"
)

// ClientIPFromConn extracts host from provided remote address.
func ClientIPFromConn(conn net.Conn) (string, error) {
	clientRemoteAddr := conn.RemoteAddr()

	clientIP, _, err := net.SplitHostPort(clientRemoteAddr.String())
	if err != nil {
		return "", trace.Wrap(err)
	}

	return clientIP, nil
}

// InferProxyPublicAddr tries to extract a proxyDNSName from a given fqdn and the list of proxy public addrs.
// If a proxyDNS name is not found, the default param will be returned.
func InferProxyPublicAddr(fqdn string, proxyDNSNames []string, defaultPublicAddr string) string {
	if fqdn == "" || defaultPublicAddr == "" {
		return defaultPublicAddr
	}
	// Split the FQDN into its components.
	fqdnParts := strings.Split(fqdn, ".")

	// check each part to find the proxyDNSName.
	for i := 0; i < len(fqdnParts); i++ {
		potentialDNS := strings.Join(fqdnParts[i:], ".")
		if slices.Contains(proxyDNSNames, potentialDNS) {
			return potentialDNS
		}
	}

	return defaultPublicAddr
}
