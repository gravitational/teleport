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

// InferProxyPublicAddr infers the proxy public address from a fully
// qualified domain name (FQDN) by checking against a list of proxy
// DNS names.
//
// Parameters:
//
//	fqdn - A string representing the fully qualified domain name to be
//	       checked.
//	proxyDNSNames - A slice of strings containing the list of known
//	                proxy DNS names.
//
// Returns:
//
//	A string representing the best match for the proxy public address.
//	If no match is found or if either argument is invalid (empty fqdn or
//	empty proxyDNSNames), it returns an empty string. If no part of
//	the FQDN matches any of the proxy DNS names, it defaults to
//	returning the first proxy DNS name.
func InferProxyPublicAddr(fqdn string, proxyDNSNames []string) string {
	if len(proxyDNSNames) == 0 || fqdn == "" {
		return ""
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

	return proxyDNSNames[0]
}
