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

package socketpair

import (
	"net"
)

// Dialer emulates a [net.Dial] by passing fds across a socektpair. Closing a
// [Dialer] will close the associated listener as well.
type Dialer struct {
	c *net.UnixConn
}

// Close closes the underlying unix conn.
func (d *Dialer) Close() error {
	return d.c.Close()
}
