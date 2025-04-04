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

// InferProxyPublicAddrFromRequestHost infers the correct proxy public address from the given request host.
// It splits the request host by "." and constructs candidate addresses by joining segments from each index
// to the end. For example, if requestHost is "client.example.com:8080", the candidates generated will be:
//   - "client.example.com:8080"
//   - "example.com:8080"
//
// The function returns the first candidate that matches an entry in proxyPublicAddrs. If no match is found,
// it returns the first element in proxyPublicAddrs. If requestHost is empty or proxyPublicAddrs is empty,
// an empty string is returned.
func InferProxyPublicAddrFromRequestHost(requestHost string, proxyPublicAddrs []string) string {
	if requestHost == "" || len(proxyPublicAddrs) == 0 {
		return ""
	}

	segments := strings.Split(requestHost, ".")
	// Check by combining segments from left to right
	for i := 0; i < len(segments); i++ {
		candidate := strings.Join(segments[i:], ".")
		for _, addr := range proxyPublicAddrs {
			if candidate == addr {
				return addr
			}
		}
	}

	return proxyPublicAddrs[0]
}
