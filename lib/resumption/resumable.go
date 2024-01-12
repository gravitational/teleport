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
)

func newResumableConn(localAddr, remoteAddr net.Addr) *ResumableConn {
	r := &ResumableConn{
		managedConn: managedConn{
			localAddr:  localAddr,
			remoteAddr: remoteAddr,
		},
	}
	r.cond.L = &r.mu
	return r
}

type ResumableConn struct {
	managedConn

	// attached is non-nil iff there's an underlying connection attached;
	// calling it should eventually result in the connection becoming detached,
	// signaled by the field becoming nil.
	attached func()
}

var _ net.Conn = (*ResumableConn)(nil)

const handshakeTimeout = 5 * time.Second

const (
	errorTag        = ^uint64(0)
	errorTagUvarint = "\xff\xff\xff\xff\xff\xff\xff\xff\xff\x01"
)

const maxFrameSize = 128 * 1024

// runResumeV1Unlocking runs the symmetric resumption v1 protocol for r, using
// nc as the underlying transport. The previous attached transport, if any, will
// be detached immediately. firstConn signifies that the connection has not been
// used, and the initial handshake will be assumed to be 0 for both sides. The
// connection lock is assumed to be held when entering the function, since the
// correct behavior of firstConn requires no possible external interference
// before the attach point is reached; the lock will be not held when the
// function returns.
func runResumeV1Unlocking(r *ResumableConn, nc net.Conn, firstConn bool) error {
	defer nc.Close()

	if !firstConn {
		for !r.remoteClosed && r.attached != nil {
			r.attached()
			r.cond.Wait()
		}

		if r.remoteClosed {
			r.mu.Unlock()

			return trace.ConnectionProblem(errBrokenPipe, "attempting to resume a connection already closed by the peer")
		}
	} else if r.attached != nil || r.remoteClosed || r.localClosed || r.receiveBuffer.end > 0 || r.sendBuffer.start > 0 {
		r.mu.Unlock()
		panic("firstConn for resume V1 is not actually unused")
	}

	var stopRequested atomic.Bool
	requestStop := func() {
		nc.Close()
		if !stopRequested.Swap(true) {
			r.cond.Broadcast()
		}
	}

	r.attached = requestStop
	r.cond.Broadcast()

	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.attached = nil
		r.cond.Broadcast()
	}()

	sentPosition := r.receiveBuffer.end
	r.mu.Unlock()

	ncReader, ok := nc.(byteReaderReader)
	if !ok {
		ncReader = bufio.NewReader(nc)
	}

	var peerPosition uint64

	handshakeWatchdog := time.AfterFunc(handshakeTimeout, func() { nc.Close() })
	defer handshakeWatchdog.Stop()

	if !firstConn {
		errC := make(chan error, 1)
		go func() {
			_, err := nc.Write(binary.AppendUvarint(nil, sentPosition))
			errC <- err
		}()
		var err error
		peerPosition, err = binary.ReadUvarint(ncReader)
		if err != nil {
			return trace.Wrap(err, "reading peer receive position")
		}
		err = <-errC
		if err != nil {
			return trace.Wrap(err, "writing receive position")
		}
	}

	r.mu.Lock()
	if minPos, maxPos := r.sendBuffer.start, r.sendBuffer.end; peerPosition < minPos || maxPos < peerPosition {
		// incompatible resume position, mark as remotely closed since we can't
		// ever continue from this; this also includes receiving an errorTag
		// (since that's too big of a position to reach legitimately)
		r.remoteClosed = true
		r.cond.Broadcast()
		r.mu.Unlock()

		_, _ = nc.Write([]byte(errorTagUvarint))
		return trace.BadParameter("got incompatible resume position (%v, expected %v to %v)", peerPosition, minPos, maxPos)
	}
	if r.sendBuffer.start != peerPosition {
		r.sendBuffer.advance(peerPosition - r.sendBuffer.start)
		r.cond.Broadcast()
	}
	r.mu.Unlock()

	handshakeWatchdog.Stop()

	eg, ctx := errgroup.WithContext(context.Background())
	defer context.AfterFunc(ctx, func() {
		if !stopRequested.Swap(true) {
			r.cond.Broadcast()
		}
	})()

	eg.Go(func() error {
		return runResumeV1Read(r, ncReader, &stopRequested)
	})

	eg.Go(func() error {
		return runResumeV1Write(r, nc, &stopRequested, sentPosition, peerPosition)
	})

	return trace.Wrap(eg.Wait())
}

func runResumeV1Read(r *ResumableConn, nc byteReaderReader, stopRequested *atomic.Bool) error {
	for {
		ack, err := binary.ReadUvarint(nc)
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

		size, err := binary.ReadUvarint(nc)
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

				n, err := io.Copy(io.Discard, io.LimitReader(nc, int64(size)))

				r.mu.Lock()
				r.receiveBuffer.start += uint64(n)
				r.receiveBuffer.end = r.receiveBuffer.start
				r.cond.Broadcast()
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
					if stopRequested.Load() {
						return trace.ConnectionProblem(net.ErrClosed, "disconnection requested")
					}
					return trace.ConnectionProblem(net.ErrClosed, "connection closed by peer")
				}
			}

			next := min(receiveBufferSize-r.receiveBuffer.len(), size)
			r.receiveBuffer.reserve(next)
			tail, _ := r.receiveBuffer.free()
			if len64(tail) > size {
				tail = tail[:size]
			}
			r.mu.Unlock()

			n, err := io.ReadFull(nc, tail)

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
}

func runResumeV1Write(r *ResumableConn, nc io.Writer, stopRequested *atomic.Bool, sentPosition, peerPosition uint64) error {
	var headerBuf [2 * binary.MaxVarintLen64]byte
	var dataBuf [maxFrameSize]byte

	for {
		var frameAck uint64
		var frameData []byte

		r.mu.Lock()
		for {
			frameAck = r.receiveBuffer.end - sentPosition

			frameData = nil
			if r.sendBuffer.end > peerPosition {
				skip := peerPosition - r.sendBuffer.start
				d1, d2 := r.sendBuffer.buffered()
				if len64(d1) <= skip {
					frameData = d2[skip-len64(d1):]
				} else {
					frameData = d1[skip:]
				}
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
		frameData = dataBuf[:copy(dataBuf[:], frameData)]
		r.mu.Unlock()

		frameHeader := binary.AppendUvarint(headerBuf[:0], frameAck)
		frameHeader = binary.AppendUvarint(frameHeader, len64(frameData))

		if _, err := nc.Write(frameHeader); err != nil {
			return trace.Wrap(err, "writing frame header")
		}
		if _, err := nc.Write(frameData); err != nil {
			return trace.Wrap(err, "writing frame data")
		}

		r.mu.Lock()
		sentPosition += frameAck
		peerPosition += len64(frameData)
		r.mu.Unlock()
	}
}

type byteReaderReader interface {
	io.Reader
	io.ByteReader
}
