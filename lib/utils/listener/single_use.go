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

package listener

import (
	"io"
	"net"
	"sync/atomic"
)

// SingleUseListener wraps a single [net.Conn] and returns it from Accept()
// once.
type SingleUseListener struct {
	conn     net.Conn
	accepted atomic.Bool
}

// NewSingleUseListener creates a new SingleUseListener.
//
// SingleUseListener does not assume ownership of the provided net.Conn. The
// caller must close the provided net.Conn after use.
func NewSingleUseListener(c net.Conn) *SingleUseListener {
	return &SingleUseListener{
		conn: c,
	}
}

// Accept returns the provided net.Conn on first call and returns io.EOF on
// subsequent calls.
func (l *SingleUseListener) Accept() (net.Conn, error) {
	if l.accepted.Swap(true) {
		return nil, io.EOF
	}
	return l.conn, nil
}

// Addr returns the provided net.Conn's local address.
func (l *SingleUseListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// Close is a no-op. The caller is responsible for closing the provided
// net.Conn after use.
func (l *SingleUseListener) Close() error {
	return nil
}
