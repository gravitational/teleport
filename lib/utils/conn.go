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
	"sync/atomic"

	"github.com/gravitational/trace"
)

// NewWaitConn returns new connection wrapper that
// provides the ability to wait for the connection
// to be closed.
func NewWaitConn(conn net.Conn) *WaitConn {
	wc := &WaitConn{
		Conn:   conn,
		closed: make(chan struct{}),
	}
	wc.close = sync.OnceValue(func() error {
		err := wc.Conn.Close()
		close(wc.closed)
		return trace.Wrap(err)
	})
	return wc
}

// WaitConn wraps a connection and provides the ability to wait for the
// connection to be closed.
type WaitConn struct {
	net.Conn

	close  func() error
	closed chan struct{}
}

// Close closes the connection.
func (c *WaitConn) Close() error {
	return c.close()
}

func (c *WaitConn) Done() <-chan struct{} { return c.closed }
func (c *WaitConn) Wait()                 { <-c.Done() }

// TrackingConn is a net.Conn that keeps track of how much data was transmitted
// (TX) and received (RX) over the net.Conn. A maximum of about 18446
// petabytes can be kept track of for TX and RX before it rolls over.
// See https://golang.org/ref/spec#Numeric_types for more details.
type TrackingConn struct {
	net.Conn
	r *trackingReader
	w *trackingWriter
}

// NewTrackingConn returns a net.Conn that can keep track of how much data was
// transmitted over it.
func NewTrackingConn(conn net.Conn) *TrackingConn {
	return &TrackingConn{
		Conn: conn,
		r:    &trackingReader{r: conn},
		w:    &trackingWriter{w: conn},
	}
}

// Stat returns the transmitted (TX) and received (RX) bytes over the net.Conn.
func (s *TrackingConn) Stat() (written, read uint64) {
	return s.w.Count(), s.r.Count()
}

func (s *TrackingConn) Read(b []byte) (n int, err error) {
	return s.r.Read(b)
}

func (s *TrackingConn) Write(b []byte) (n int, err error) {
	return s.w.Write(b)
}

// trackingReader is an io.Reader that counts the total number of bytes read.
// It's thread-safe if the underlying io.Reader is thread-safe.
type trackingReader struct {
	r     io.Reader
	count uint64
}

// Count returns the total number of bytes read so far.
func (r *trackingReader) Count() uint64 {
	return atomic.LoadUint64(&r.count)
}

func (r *trackingReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	atomic.AddUint64(&r.count, uint64(n))

	// This has to use the original error type or else utilities using the connection
	// (like io.Copy, which is used by the oxy forwarder) may incorrectly categorize
	// the error produced by this and terminate the connection unnecessarily.
	return n, err
}

// trackingWriter is an io.Writer that counts the total number of bytes
// written.
// It's thread-safe if the underlying io.Writer is thread-safe.
type trackingWriter struct {
	count uint64 // intentionally placed first to ensure 64-bit alignment
	w     io.Writer
}

// Count returns the total number of bytes written so far.
func (w *trackingWriter) Count() uint64 {
	return atomic.LoadUint64(&w.count)
}

func (w *trackingWriter) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	atomic.AddUint64(&w.count, uint64(n))
	return n, trace.Wrap(err)
}

// ConnWithAddr is a [net.Conn] wrapper that allows the local and remote address
// to be overridden.
type ConnWithAddr struct {
	net.Conn
	localAddrOverride  net.Addr
	remoteAddrOverride net.Addr
}

// LocalAddr implements [net.Conn].
func (c *ConnWithAddr) LocalAddr() net.Addr {
	if c.localAddrOverride != nil {
		return c.localAddrOverride
	}

	return c.Conn.LocalAddr()
}

// RemoteAddr implements [net.Conn].
func (c *ConnWithAddr) RemoteAddr() net.Addr {
	if c.remoteAddrOverride != nil {
		return c.remoteAddrOverride
	}

	return c.Conn.RemoteAddr()
}

// NetConn returns the underlying [net.Conn].
func (c *ConnWithAddr) NetConn() net.Conn {
	return c.Conn
}

// NewConnWithSrcAddr wraps provided connection and overrides client remote address.
func NewConnWithSrcAddr(conn net.Conn, clientSrcAddr net.Addr) *ConnWithAddr {
	return &ConnWithAddr{
		Conn: conn,

		remoteAddrOverride: clientSrcAddr,
	}
}

// NewConnWithAddr wraps a [net.Conn] optionally overriding the local and remote
// addresses with the provided ones, if non-nil.
func NewConnWithAddr(conn net.Conn, localAddr, remoteAddr net.Addr) *ConnWithAddr {
	return &ConnWithAddr{
		Conn: conn,

		localAddrOverride:  localAddr,
		remoteAddrOverride: remoteAddr,
	}
}
