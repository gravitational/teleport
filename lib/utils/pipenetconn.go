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
	"sync"
	"time"
)

// PipeNetConn implements net.Conn from a provided io.Reader,io.Writer and
// io.Closer
type PipeNetConn struct {
	// Locks writing and closing the connection. If both writer & closer refer
	// to the same underlying object, simultaneous write and close operations
	// introduce a data race (*especially* if that object is a
	// `x/crypto/ssh.channel`), so we will use this mutex to serialize write
	// and close operations.
	mu sync.Mutex

	reader     io.Reader
	writer     io.Writer
	closer     io.Closer
	localAddr  net.Addr
	remoteAddr net.Addr
}

// NewPipeNetConn constructs a new PipeNetConn, providing a net.Conn
// implementation synthesized from the supplied io.Reader, io.Writer &
// io.Closer.
func NewPipeNetConn(reader io.Reader,
	writer io.Writer,
	closer io.Closer,
	fakelocalAddr net.Addr,
	fakeRemoteAddr net.Addr) *PipeNetConn {

	return &PipeNetConn{
		reader:     reader,
		writer:     writer,
		closer:     closer,
		localAddr:  fakelocalAddr,
		remoteAddr: fakeRemoteAddr,
	}
}

func (nc *PipeNetConn) Read(buf []byte) (n int, e error) {
	return nc.reader.Read(buf)
}

func (nc *PipeNetConn) Write(buf []byte) (n int, e error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	return nc.writer.Write(buf)
}

func (nc *PipeNetConn) Close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.closer != nil {
		return nc.closer.Close()
	}
	return nil
}

func (nc *PipeNetConn) LocalAddr() net.Addr {
	return nc.localAddr
}

func (nc *PipeNetConn) RemoteAddr() net.Addr {
	return nc.remoteAddr
}

func (nc *PipeNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (nc *PipeNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (nc *PipeNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}
