// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resumption

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/constants"
)

type ResumableConn struct {
	managedConn

	// allowRoaming keeps track of whether or not we should allow attaching an
	// underlying connection with a different remote address than the previous
	// one (in the IP address sense, the port doesn't have to match).
	allowRoaming bool

	// attached is non-nil iff there's an underlying connection attached;
	// calling it should eventually result in the connection becoming detached,
	// signaled by the field becoming nil.
	attached func()
}

func (r *ResumableConn) waitForDetachLocked() {
	for r.attached != nil {
		r.attached()
		r.cond.Wait()
	}
}

var _ net.Conn = (*ResumableConn)(nil)

const (
	idleTimeout      = 3 * time.Minute
	graceTimeout     = 5 * time.Second
	handshakeTimeout = 30 * time.Second
)

const (
	errorTag        = ^uint64(0)
	errorTagUvarint = "\xff\xff\xff\xff\xff\xff\xff\xff\xff\x01"
)

const maxFrameSize = 128 * 1024

func RunResumeV1(r *ResumableConn, nc net.Conn, firstConn bool) error {
	defer nc.Close()

	localAddr := nc.LocalAddr()
	remoteAddr := nc.RemoteAddr()

	r.mu.Lock()
	if !firstConn && !r.allowRoaming && !sameAddress(r.remoteAddr, remoteAddr) {
		r.mu.Unlock()

		defer time.AfterFunc(graceTimeout, func() { nc.Close() }).Stop()
		_, _ = nc.Write([]byte(errorTagUvarint))
		return trace.AccessDenied("connection roaming is disabled")
	}

	r.waitForDetachLocked()

	if r.localClosed || r.remoteClosed {
		localClosed := r.localClosed
		r.mu.Unlock()

		defer time.AfterFunc(graceTimeout, func() { nc.Close() }).Stop()
		_, _ = nc.Write([]byte(errorTagUvarint))

		if localClosed {
			return trace.ConnectionProblem(net.ErrClosed, constants.UseOfClosedNetworkConnection)
		}
		return trace.ConnectionProblem(net.ErrClosed, "refusing to attempt to resume a connection already closed by the peer")
	}

	// stopRequested should be checked whenever we're in a loop that doesn't
	// involve I/O on the conn
	var stopRequested atomic.Bool
	requestStop := func() {
		if stopRequested.Swap(true) {
			return
		}
		r.cond.Broadcast()
		nc.Close()
	}

	r.localAddr = localAddr
	r.remoteAddr = remoteAddr

	r.attached = requestStop
	r.cond.Broadcast()
	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.attached = nil
		r.cond.Broadcast()
	}()

	sentReceivePosition := r.receiveBuffer.end
	r.mu.Unlock()

	ncReader, ok := nc.(interface {
		io.Reader
		io.ByteReader
	})
	if !ok {
		ncReader = bufio.NewReader(nc)
	}

	var peerReceivePosition uint64

	handshakeWatchdog := time.AfterFunc(handshakeTimeout, func() { nc.Close() })
	defer handshakeWatchdog.Stop()

	if firstConn {
		if sentReceivePosition != 0 {
			go io.Copy(io.Discard, ncReader)
			_, _ = nc.Write([]byte(errorTagUvarint))
			return trace.BadParameter("handshake for a new connection on a used connection (this is a bug)")
		}
	} else {
		errC := make(chan error, 1)
		go func() {
			_, err := nc.Write(binary.AppendUvarint(nil, sentReceivePosition))
			errC <- err
		}()
		var err error
		peerReceivePosition, err = binary.ReadUvarint(ncReader)
		if err != nil {
			return trace.Wrap(err, "reading peer receive position")
		}
		err = <-errC
		if err != nil {
			return trace.Wrap(err, "writing receive position")
		}
	}

	r.mu.Lock()
	if minPos, maxPos := r.sendBuffer.start, r.sendBuffer.end; peerReceivePosition < minPos || maxPos < peerReceivePosition {
		// incompatible resume position, mark as remotely closed since we can't
		// ever continue from this; this also includes receiving an errorTag
		// (since that's too big of a position to reach legitimately)
		r.remoteClosed = true
		r.cond.Broadcast()
		r.mu.Unlock()

		_, _ = nc.Write([]byte(errorTagUvarint))
		return trace.BadParameter("got incompatible resume position (%v, expected %v to %v)", peerReceivePosition, minPos, maxPos)
	}
	if r.sendBuffer.start != peerReceivePosition {
		r.sendBuffer.advance(peerReceivePosition - r.sendBuffer.start)
		r.cond.Broadcast()
	}
	r.mu.Unlock()

	handshakeWatchdog.Stop()

	eg, ctx := errgroup.WithContext(context.Background())
	context.AfterFunc(ctx, requestStop)

	eg.Go(func() error {
		for {
			ack, err := binary.ReadUvarint(ncReader)
			if err != nil {
				return trace.Wrap(err, "reading ack")
			}

			if ack > 0 {
				r.mu.Lock()
				if ack == errorTag {
					r.remoteClosed = true
					r.cond.Broadcast()
					r.mu.Unlock()

					return trace.ConnectionProblem(net.ErrClosed, "connection closed by peer")
				}

				if maxAck := r.sendBuffer.len(); ack > maxAck {
					r.mu.Unlock()
					return trace.BadParameter("got ack bigger than current send buffer (%v, expected up to %v)", ack, maxAck)
				}

				r.sendBuffer.advance(ack)
				r.cond.Broadcast()
				r.mu.Unlock()
			}

			size, err := binary.ReadUvarint(ncReader)
			if err != nil {
				return trace.Wrap(err, "reading data size")
			}

			if size > maxFrameSize {
				return trace.BadParameter("got data size bigger than limit (%v, expected up to %v)", size, maxFrameSize)
			}

			r.mu.Lock()

			for size > 0 {
				if r.localClosed {
					r.receiveBuffer.advance(r.receiveBuffer.len())
					r.cond.Broadcast()
					r.mu.Unlock()

					n, err := io.Copy(io.Discard, io.LimitReader(ncReader, int64(size)))

					r.mu.Lock()
					r.receiveBuffer.advance(uint64(n))
					if err != nil {
						r.mu.Unlock()
						return trace.Wrap(err, "reading data to discard")
					}
					break
				}

				for r.receiveBuffer.len() >= receiveBufferSize {
					r.cond.Wait()
					if stopRequested.Load() || r.remoteClosed {
						r.mu.Unlock()
						return trace.ConnectionProblem(net.ErrClosed, "connection closed by peer or disconnection requested")
					}
				}

				next := min(receiveBufferSize-r.receiveBuffer.len(), size)
				r.receiveBuffer.reserve(next)
				tail, _ := r.receiveBuffer.free()
				if len64(tail) > size {
					tail = tail[:size]
				}
				r.mu.Unlock()

				n, err := io.ReadFull(ncReader, tail)

				r.mu.Lock()
				if n > 0 {
					r.receiveBuffer.append(tail[:n])
					size -= uint64(n)
					r.cond.Broadcast()
				}

				if err != nil {
					r.mu.Unlock()
					return trace.Wrap(err, "reading data")
				}
			}
			r.mu.Unlock()
		}
	})

	eg.Go(func() error {
		var scratch [2 * binary.MaxVarintLen64]byte
		for {
			var frameAck uint64
			var frameData []byte

			r.mu.Lock()
			for {
				frameAck = r.receiveBuffer.end - sentReceivePosition

				frameData = nil
				if r.sendBuffer.end > peerReceivePosition {
					skip := peerReceivePosition - r.sendBuffer.start
					d1, d2 := r.sendBuffer.buffered()
					if len64(d1) <= skip {
						frameData = d2[skip-len64(d1):]
					} else {
						frameData = d1[skip:]
					}
				}
				if len(frameData) >= maxFrameSize {
					frameData = frameData[:maxFrameSize]
				}
				if frameAck > 0 || len(frameData) > 0 {
					break
				}

				if stopRequested.Load() || r.localClosed || r.remoteClosed {
					localClosed := r.localClosed
					r.mu.Unlock()

					if localClosed {
						_, _ = nc.Write([]byte(errorTagUvarint))
					}
					return trace.ConnectionProblem(net.ErrClosed, "connection closed by peer or disconnection requested")
				}

				r.cond.Wait()
			}
			r.mu.Unlock()

			frameHeader := binary.AppendUvarint(scratch[:0], frameAck)
			frameHeader = binary.AppendUvarint(frameHeader, len64(frameData))
			frameBuffers := net.Buffers{frameHeader, frameData}

			if _, err := frameBuffers.WriteTo(nc); err != nil {
				return trace.Wrap(err, "writing frame")
			}

			r.mu.Lock()
			sentReceivePosition += frameAck
			peerReceivePosition += len64(frameData)
			r.mu.Unlock()
		}
	})

	return trace.Wrap(eg.Wait())
}

// sameAddress returns true if a and b are both [*net.TCPAddr] and their IP
// address is equal.
func sameAddress(a, b net.Addr) bool {
	ta, ok := a.(*net.TCPAddr)
	if !ok || ta == nil {
		return false
	}

	tb, ok := b.(*net.TCPAddr)
	if !ok || tb == nil {
		return false
	}

	return ta.IP.Equal(tb.IP)
}
