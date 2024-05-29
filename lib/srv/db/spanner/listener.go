/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package spanner

import (
	"io"
	"net"
	"sync/atomic"
)

// singleUseListener wraps a single [net.Conn] and returns it from Accept()
// once.
type singleUseListener struct {
	conn     net.Conn
	accepted atomic.Bool
}

func newSingleUseListener(c net.Conn) *singleUseListener {
	return &singleUseListener{
		conn: c,
	}
}

func (l *singleUseListener) Accept() (net.Conn, error) {
	if l.accepted.Swap(true) {
		return nil, io.EOF
	}
	return l.conn, nil
}

func (l *singleUseListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *singleUseListener) Close() error {
	return nil
}
