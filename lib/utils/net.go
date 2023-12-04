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
