// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stream

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaxChunkSize is the maximum number of bytes to send in a single data message.
// According to https://github.com/grpc/grpc.github.io/issues/371 the optimal
// size is between 16KiB to 64KiB.
const MaxChunkSize int = 1024 * 16

// Source is a common interface for grpc client and server streams
// that transport opaque data.
type Source interface {
	Send([]byte) error
	Recv() ([]byte, error)
}

// ReadWriter wraps a grpc source with an [io.ReadWriter] interface.
// All reads are consumed from [Source.Recv] and all writes and sent
// via [Source.Send].
type ReadWriter struct {
	source Source

	wLock  sync.Mutex
	rLock  sync.Mutex
	rBytes []byte
}

// NewReadWriter creates a new ReadWriter that leverages the provided
// source to retrieve data from and write data to.
func NewReadWriter(source Source) (*ReadWriter, error) {
	if source == nil {
		return nil, trace.BadParameter("parameter source required")
	}

	return &ReadWriter{
		source: source,
	}, nil
}

// Read returns data received from the stream source. Any
// data received from the stream that is not consumed will
// be buffered and returned on subsequent reads until there
// is none left. Only then will data be sourced from the stream
// again.
func (c *ReadWriter) Read(b []byte) (n int, err error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()

	if len(c.rBytes) == 0 {
		data, err := c.source.Recv()
		if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
			return 0, io.EOF
		}

		if err != nil {
			return 0, trace.ConnectionProblem(trace.Wrap(err), "failed to receive from source: %v", err)
		}

		if data == nil {
			return 0, trace.BadParameter("received invalid data from source")
		}

		c.rBytes = data
	}

	n = copy(b, c.rBytes)
	c.rBytes = c.rBytes[n:]

	// Stop holding onto buffer immediately
	if len(c.rBytes) == 0 {
		c.rBytes = nil
	}

	return n, nil
}

// Write consumes all data provided and sends it on
// the grpc stream. To prevent exhausting the stream all
// sends on the stream are limited to be at most MaxChunkSize.
// If the data exceeds the MaxChunkSize it will be sent in
// batches.
func (c *ReadWriter) Write(b []byte) (int, error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()

	var sent int
	for len(b) > 0 {
		chunk := b
		if len(chunk) > MaxChunkSize {
			chunk = chunk[:MaxChunkSize]
		}

		if err := c.source.Send(chunk); err != nil {
			return sent, trace.ConnectionProblem(trace.Wrap(err), "failed to send on source: %v", err)
		}

		sent += len(chunk)
		b = b[len(chunk):]
	}

	return sent, nil
}

// Close cleans up resources used by the stream.
func (c *ReadWriter) Close() error {
	if cs, ok := c.source.(io.Closer); ok {
		c.wLock.Lock()
		defer c.wLock.Unlock()
		return trace.Wrap(cs.Close())
	}

	return nil
}

// Conn wraps [ReadWriter] in a [net.Conn] interface.
type Conn struct {
	*ReadWriter

	src net.Addr
	dst net.Addr
}

// NewConn creates a new Conn which transfers data via the provided ReadWriter.
func NewConn(rw *ReadWriter, src net.Addr, dst net.Addr) *Conn {
	return &Conn{
		ReadWriter: rw,
		src:        src,
		dst:        dst,
	}
}

// LocalAddr is the original source address of the client.
func (c *Conn) LocalAddr() net.Addr {
	return c.src
}

// RemoteAddr is the address of the reverse tunnel node.
func (c *Conn) RemoteAddr() net.Addr {
	return c.dst
}

func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
