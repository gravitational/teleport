// Copyright 2022 Gravitational, Inc
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

package peer

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
)

const (
	// maxChunkSize is the maximum number of bytes to send in a single data message.
	// According to https://github.com/grpc/grpc.github.io/issues/371 the optimal
	// size is between 16KiB to 64KiB.
	maxChunkSize int = 1024 * 16
)

// Stream is a common interface for grpc client and server streams.
type Stream interface {
	Context() context.Context
	Send(*proto.Frame) error
	Recv() (*proto.Frame, error)
}

// streamConn wraps a grpc stream with a net.Conn interface.
type streamConn struct {
	stream Stream

	wLock  sync.Mutex
	rLock  sync.Mutex
	rBytes []byte

	src net.Addr
	dst net.Addr
}

// newStreamConn creates a new streamConn.
func newStreamConn(stream Stream, src net.Addr, dst net.Addr) *streamConn {
	return &streamConn{
		stream: stream,
		src:    src,
		dst:    dst,
	}
}

// Read reads data received over the grpc stream.
func (c *streamConn) Read(b []byte) (n int, err error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()

	if len(c.rBytes) == 0 {
		frame, err := c.stream.Recv()
		if errors.Is(err, io.EOF) {
			return 0, io.EOF
		}
		if err != nil {
			return 0, trace.ConnectionProblem(err, "failed to receive on stream")
		}

		data := frame.GetData()
		if data == nil {
			return 0, trace.BadParameter("received invalid data frame")
		}

		c.rBytes = data.Bytes
	}

	n = copy(b, c.rBytes)
	c.rBytes = c.rBytes[n:]

	// Stop holding onto buffer immediately
	if len(c.rBytes) == 0 {
		c.rBytes = nil
	}

	return n, nil
}

// Write sends data over the grpc stream.
func (c *streamConn) Write(b []byte) (int, error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()

	var sent int
	for len(b) > 0 {
		chunk := b
		if len(chunk) > maxChunkSize {
			chunk = chunk[:maxChunkSize]
		}

		err := c.stream.Send(&proto.Frame{Message: &proto.Frame_Data{Data: &proto.Data{
			Bytes: chunk,
		}}})
		if err != nil {
			return sent, trace.ConnectionProblem(err, "failed to send on stream")
		}

		sent += len(chunk)
		b = b[len(chunk):]
	}

	return sent, nil
}

// Close cleans up resources used by the connection.
func (c *streamConn) Close() error {
	var err error
	if cstream, ok := c.stream.(grpc.ClientStream); ok {
		c.wLock.Lock()
		defer c.wLock.Unlock()
		err = cstream.CloseSend()
	}

	return trace.Wrap(err)
}

// LocalAddr is the original source address of the client.
func (c *streamConn) LocalAddr() net.Addr {
	return c.src
}

// RemoteAddr is the address of the reverse tunnel node.
func (c *streamConn) RemoteAddr() net.Addr {
	return c.dst
}

func (c *streamConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *streamConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// pipeConn copies between two net.Conns and closes them when done.
func pipeConn(ctx context.Context, src net.Conn, dst net.Conn) (int64, int64, error) {
	var (
		sent, received int64
		wg             sync.WaitGroup
		o              sync.Once
	)

	errC := make(chan error, 1)
	cleanup := func(err error) {
		o.Do(func() {
			srcErr := src.Close()
			dstErr := dst.Close()
			errC <- trace.NewAggregate(err, srcErr, dstErr)
			close(errC)
		})
	}

	wg.Add(2)

	go func() {
		var err error
		sent, err = io.Copy(src, dst)
		cleanup(trace.ConnectionProblem(
			err, "failed copy to source %s", src.RemoteAddr().String(),
		))
		wg.Done()
	}()

	go func() {
		var err error
		received, err = io.Copy(dst, src)
		cleanup(trace.ConnectionProblem(
			err, "failed copy to destination %s", dst.RemoteAddr().String(),
		))
		wg.Done()
	}()

	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()

	select {
	case <-ctx.Done():
		cleanup(nil)
	case <-wait:
	}

	<-wait
	err := <-errC
	return sent, received, trace.Wrap(err)
}
