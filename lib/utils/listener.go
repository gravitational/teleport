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
	"io"
	"net"
	"os"
	"sync/atomic"

	"github.com/gravitational/trace"
)

// GetListenerFile returns file associated with listener
func GetListenerFile(listener net.Listener) (*os.File, error) {
	switch t := listener.(type) {
	case *net.TCPListener:
		f, err := t.File()
		return f, trace.Wrap(err)
	case *net.UnixListener:
		f, err := t.File()
		return f, trace.Wrap(err)
	}
	return nil, trace.BadParameter("unsupported listener: %T", listener)
}

// SingleUseListener wraps a single [net.Conn] and returns it from Accept()
// once.
type SingleUseListener struct {
	conn     net.Conn
	accepted atomic.Bool
}

// NewSingleUseListener creates a new SingleUseListener.
func NewSingleUseListener(c net.Conn) *SingleUseListener {
	return &SingleUseListener{
		conn: c,
	}
}

func (l *SingleUseListener) Accept() (net.Conn, error) {
	if l.accepted.Swap(true) {
		return nil, io.EOF
	}
	return l.conn, nil
}

func (l *SingleUseListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *SingleUseListener) Close() error {
	return nil
}
