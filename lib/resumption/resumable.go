// Teleport
// Copyright (C) 2024  Gravitational, Inc.
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
	"log/slog"
	"math"
	"net"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func newResumableConn(localAddr, remoteAddr net.Addr) *Conn {
	r := &Conn{
		managedConn: managedConn{
			localAddr:  localAddr,
			remoteAddr: remoteAddr,
		},
	}
	r.cond.L = &r.mu
	return r
}

// Conn is a [net.Conn] whose underlying transport can be closed and reopened,
// to maintain the illusion of a perfect unbroken stream of bytes even if
// network conditions would otherwise terminate a normal connection.
type Conn struct {
	managedConn

	// requestDetachLocked is non-nil if and only if there is an underlying
	// connection attached; calling it should eventually result in the
	// connection becoming detached, signaled by the field becoming nil. It must
	// only be called while mu is held.
	requestDetachLocked func()
}

var _ net.Conn = (*Conn)(nil)

const handshakeTimeout = 5 * time.Second

// errorTag is the acknowledgement value used to signal a connection close
// or a failed handshake.
const errorTag = math.MaxUint64

// maxFrameSize is the maximum amount of data that can be transmitted at once;
// picked for sanity's sake, and to allow acks to be sent relatively frequently.
const maxFrameSize = 128 * 1024

// runResumeV1Unlocking runs the symmetric resumption v1 protocol for r, using
// nc as the underlying transport. The previous attached transport, if any, will
// be detached immediately. firstConn signifies that the connection has not been
// used, and the initial handshake will be assumed to be 0 for both sides. The
// connection lock is assumed to be held when entering the function, since the
// correct behavior of firstConn requires no possible external interference
// before the attach point is reached; the lock will be not held when the
// function returns.
func runResumeV1Unlocking(r *Conn, nc net.Conn, firstConn bool) error {
	defer nc.Close()

	if !firstConn {
		t0 := time.Now()
		for !r.remoteClosed && r.requestDetachLocked != nil {
			r.requestDetachLocked()
			r.cond.Wait()
		}
		if dt := time.Since(t0); dt > time.Second {
			slog.WarnContext(context.TODO(), "slow resumable connection detach took over one second", "elapsed", logutils.StringerAttr(dt))
		}

		if r.remoteClosed {
			r.mu.Unlock()

			return trace.Wrap(net.ErrClosed, "resuming a connection already closed by the peer")
		}
	} else if r.requestDetachLocked != nil || r.remoteClosed || r.localClosed || r.receiveBuffer.end > 0 || r.sendBuffer.start > 0 {
		r.mu.Unlock()
		panic("firstConn for resume V1 is not actually unused")
	}

	// stopRequested is used by the read and write goroutines to know if a stop
	// was requested in loops that don't perform I/O on nc - any request to
	// detach will also close the connection, so exiting on I/O errors will
	// naturally follow stop requests.
	stopRequested := new(bool)
	requestStopLocked := func() {
		if *stopRequested {
			return
		}
		*stopRequested = true
		r.cond.Broadcast()
	}
	r.requestDetachLocked = func() {
		nc.Close()
		requestStopLocked()
	}
	r.cond.Broadcast()

	defer func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.requestDetachLocked = nil
		r.cond.Broadcast()
	}()

	localPosition := r.receiveBuffer.end
	r.mu.Unlock()

	ncReader, ok := nc.(byteReaderReader)
	if !ok {
		ncReader = bufio.NewReader(nc)
	}

	var peerPosition uint64
	if !firstConn {
		p, err := resumeV1Handshake(r, nc, ncReader, localPosition)
		if err != nil {
			return trace.Wrap(err, "handshake")
		}
		peerPosition = p
	}

	var eg errgroup.Group

	eg.Go(func() error {
		defer func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			requestStopLocked()
		}()
		// the read loop exits on I/O errors (which will kill the write loop
		// too) but also upon receiving an error tag from the remote, signaling
		// that the peer has already been done with the connection for a while
		// now, so anything we're going to write is going to be useless anyway
		defer nc.Close()
		return trace.Wrap(runResumeV1Read(r, ncReader, stopRequested), "read loop")
	})

	eg.Go(func() error {
		defer func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			requestStopLocked()
		}()
		// we shouldn't close the connection when exiting from the write loop,
		// because the read loop might have data still worth parsing (if we
		// exited because of I/O errors)
		return trace.Wrap(runResumeV1Write(r, nc, stopRequested, localPosition, peerPosition), "write loop")
	})

	return trace.Wrap(eg.Wait())
}

func resumeV1Handshake(r *Conn, nc net.Conn, ncReader byteReaderReader, localPosition uint64) (peerPosition uint64, err error) {
	handshakeWatchdog := time.AfterFunc(handshakeTimeout, func() { nc.Close() })
	defer handshakeWatchdog.Stop()

	var eg errgroup.Group
	eg.Go(func() error {
		_, err := nc.Write(binary.AppendUvarint(nil, localPosition))
		return trace.Wrap(err, "writing local receive position")
	})
	eg.Go(func() error {
		var err error
		peerPosition, err = binary.ReadUvarint(ncReader)
		return trace.Wrap(err, "reading peer receive position")
	})
	if err := eg.Wait(); err != nil {
		return 0, trace.Wrap(err)
	}

	r.mu.Lock()
	if minPos, maxPos := r.sendBuffer.start, r.sendBuffer.end; peerPosition < minPos || maxPos < peerPosition {
		// incompatible resume position, mark as remotely closed since we can't
		// ever continue from this; this also includes receiving an errorTag
		// (since that's too big of a position to reach legitimately)
		r.remoteClosed = true
		r.cond.Broadcast()
		r.mu.Unlock()

		_, _ = nc.Write(binary.AppendUvarint(nil, errorTag))
		return 0, trace.BadParameter("got incompatible resume position (%v, expected %v to %v)", peerPosition, minPos, maxPos)
	}

	if r.sendBuffer.start != peerPosition {
		r.sendBuffer.advance(peerPosition - r.sendBuffer.start)
		r.cond.Broadcast()
	}
	r.mu.Unlock()

	return peerPosition, nil
}

func runResumeV1Read(r *Conn, nc byteReaderReader, stopRequested *bool) error {
	for {
		// a frame consists of a variable length integer acknowledging received
		// data, then a variable length integer containing the length of the
		// immediately-following data buffer

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

				// if the other side has sent us the error tag for remote close
				// then it has been done for a while; there's no need to be
				// graceful and send our own data anymore, so we just kill the
				// connection outright
				return trace.Wrap(net.ErrClosed, "peer signaled connection close")
			}

			if maxAck := r.sendBuffer.len(); ack > maxAck {
				r.mu.Unlock()
				return trace.BadParameter("got ack bigger than current send buffer (%v, expected up to %v)", ack, maxAck)
			}

			r.sendBuffer.advance(ack)
			r.cond.Broadcast()
			r.mu.Unlock()
		}

		remainingSize, err := binary.ReadUvarint(nc)
		if err != nil {
			return trace.Wrap(err, "reading size")
		}

		if remainingSize > maxFrameSize {
			return trace.BadParameter("got data size bigger than limit (%v, expected up to %v)", remainingSize, maxFrameSize)
		}

		r.mu.Lock()

		// we don't necessarily have enough space in the receiveBuffer to read
		// all the data, so we just loop until we've exhausted the data and we
		// can start again with the next ack
		for remainingSize > 0 {
			// we are responsible for setting r.remoteClosed and we return
			// immediately after setting it, so we don't need to check for that
			for r.receiveBuffer.len() >= receiveBufferSize && !r.localClosed && !(*stopRequested) {
				r.cond.Wait()
			}

			if *stopRequested {
				r.mu.Unlock()
				return trace.Wrap(net.ErrClosed, "disconnection requested")
			}

			if r.localClosed {
				// if the Conn is locally closed the application will never read
				// from the buffer, but we still need to go through the data
				// from the peer to acknowledge it while we send the last of our
				// own data; so we just discard it, instead
				r.mu.Unlock()

				n, err := io.Copy(io.Discard, io.LimitReader(nc, int64(remainingSize)))

				r.mu.Lock()
				// bump the position of our receive buffer so we will
				// acknowledge the received data
				r.receiveBuffer.end += uint64(n)
				r.receiveBuffer.start = r.receiveBuffer.end
				r.cond.Broadcast()
				if err != nil {
					r.mu.Unlock()
					return trace.Wrap(err, "discarding data")
				}
				break
			}

			next := min(receiveBufferSize-r.receiveBuffer.len(), remainingSize)
			r.receiveBuffer.reserve(next)
			// if there's space (and we just reserved space), the first of the
			// two free slices is going to be non-empty (see [buffer.free]), so
			// we can just ignore the second one, and let the next iterations of
			// the remainingSize loop take care of completing the read of this
			// frame, if necessary
			tail, _ := r.receiveBuffer.free()
			if len64(tail) > remainingSize {
				tail = tail[:remainingSize]
			}
			r.mu.Unlock()

			n, err := io.ReadFull(nc, tail)

			r.mu.Lock()
			// the number returned by I/O functions is always meaningful even if there's an error
			if n > 0 {
				// this will not actually copy any data, since tail was
				// subsliced from the buffer that we're about to copy into, and
				// copy() will be a noop in that case
				r.receiveBuffer.append(tail[:n])
				remainingSize -= uint64(n)
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

func runResumeV1Write(r *Conn, nc io.Writer, stopRequested *bool, localPosition, peerPosition uint64) error {
	// headerBuf and dataBuf are allocated only once because the compiler can't
	// prove that I/O functions won't let the slices escape; by allocating these
	// once and reusing them, we avoid allocating buffers on every loop
	var (
		headerBuf [2 * binary.MaxVarintLen64]byte
		dataBuf   [maxFrameSize]byte
	)

	for {
		var frameAck uint64
		var frameData []byte

		r.mu.Lock()
		// here we wait until we have some data to acknowledge, some data to
		// send, or we should exit
		for {
			// localPosition is the position we have acknowledged so far, so we
			// need to acknowledge until the end of receiveBuffer
			frameAck = r.receiveBuffer.end - localPosition

			frameData = nil
			if r.sendBuffer.end > peerPosition {
				skip := peerPosition - r.sendBuffer.start
				// we need to send as much data as possible starting from
				// peerPosition, but it's convenient to only act on just one of
				// the potential two contiguous slices of data in the buffer;
				// it's ok to send less than all the possible data in a single
				// frame, after all
				d1, d2 := r.sendBuffer.buffered()
				if len64(d1) <= skip {
					frameData = d2[skip-len64(d1):]
				} else {
					frameData = d1[skip:]
				}
			}

			// TODO(espadolini): check if we'll benefit from only acknowledging
			// above a certain amount of bytes, both in terms of bandwidth
			// (likely very minor) and in terms of reducing two-byte syscalls
			// (we shouldn't do the same with data, however)
			if frameAck > 0 || len(frameData) > 0 {
				break
			}

			if *stopRequested {
				r.mu.Unlock()
				return trace.Wrap(net.ErrClosed, "disconnection requested")
			}

			if r.remoteClosed {
				r.mu.Unlock()
				return trace.Wrap(net.ErrClosed, "connection closed by peer")
			}

			if r.localClosed {
				r.mu.Unlock()

				// if we got here we have no data to send, and since the
				// connection was locally closed, we will never have data to
				// send in the future, so we can signal the peer that we're done
				_, _ = nc.Write(binary.AppendUvarint(nil, errorTag))
				return trace.Wrap(net.ErrClosed, "connection closed")
			}

			r.cond.Wait()
		}
		// we can't Write() the slice from the sendBuffer here because otherwise
		// there's no protection against the remote side acknowledging data that
		// we're in the middle of sending, which might result in memory getting
		// overwritten as the application writes more data in the buffer
		//
		// TODO(espadolini): remove this copy, perhaps reserving the outbound
		// data in flight in the buffer
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

		localPosition += frameAck
		peerPosition += len64(frameData)
	}
}

type byteReaderReader interface {
	io.Reader
	io.ByteReader
}
