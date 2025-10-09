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

// FindMatchingProxyDNS checks if a given request host or app fqdn matches any of the specified proxy DNS names.
// It compares the hostnames without considering the port numbers.
// If a match is found, the method returns the original proxy DNS name (including its port if present).
// If no match is found, it returns the first proxy DNS name from the list.
//
// Parameters:
//   - requestHostnameOrFQDN: A string representing the host in the request, which may include a port.
//   - proxyDNSNames: A slice of strings representing possible DNS names for a proxy, each of which may include a port.
//
// Returns:
//   - A string representing the matching proxy DNS name with its port, or the first proxy DNS name if no matches are found.
func FindMatchingProxyDNS(requestHostnameOrFQDN string, proxyDNSNames []string) string {
	if requestHostnameOrFQDN == "" || len(proxyDNSNames) == 0 {
		return ""
	}

	// Remove port from request host if present.
	normalizedRequestHost := strings.Split(requestHostnameOrFQDN, ":")[0]
	hostParts := strings.Split(normalizedRequestHost, ".")

	// Iterate over each possible suffix of requestHostOrFQDN parts
	for start := 0; start < len(hostParts); start++ {
		possibleHost := strings.Join(hostParts[start:], ".")
		for _, proxyDNSName := range proxyDNSNames {
			// Normalize proxy DNS name by removing port if present
			normalizedProxyDNSName := strings.Split(proxyDNSName, ":")[0]
			if possibleHost == normalizedProxyDNSName {
				return proxyDNSName
			}
		}
	}

	// If no match found, return the first proxyDNSName as fallback
	return proxyDNSNames[0]
}
