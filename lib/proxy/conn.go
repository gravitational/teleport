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

package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
)

const (
	// bufferSize is the connection buffer size.
	bufferSize = 4 * 1024
)

// Stream is a common interface for grpc client and server streams.
type Stream interface {
	Context() context.Context
	Send(*proto.Frame) error
	Recv() (*proto.Frame, error)
}

// Check that streamConn implements net.Conn.
var _ net.Conn = &streamConn{}

// streamConn wraps a grpc stream with a net.streamConn interface.
type streamConn struct {
	stream Stream
	local  net.Conn
	remote net.Conn

	src net.Addr
	dst net.Addr

	once sync.Once
	wg   sync.WaitGroup
}

// newStreamConn creates a new streamConn.
func newStreamConn(stream Stream, src net.Addr, dst net.Addr) *streamConn {
	local, remote := net.Pipe()
	return &streamConn{
		stream: stream,
		local:  local,
		remote: remote,
		src:    src,
		dst:    dst,
	}
}

// start begins copying data between the grpc stream and internal pipe.
func (c *streamConn) start() {
	c.wg.Add(2)
	go func() {
		defer c.Close()
		c.receive(c.stream)
		c.wg.Done()
	}()
	go func() {
		defer c.Close()
		c.send(c.stream)
		c.wg.Done()
	}()

}

// receive reveives data from the stream and copies it to the internal pipe.
func (c *streamConn) receive(stream Stream) error {
	var (
		frame *proto.Frame
		err   error
	)

	for {
		frame, err = stream.Recv()
		if err != nil {
			return trace.Wrap(err)
		}

		data := frame.GetData()
		if data == nil {
			return trace.Errorf("failed to get data from tunnel frame")
		}

		_, err := c.remote.Write(data.Bytes)
		if err != nil {
			return trace.Wrap(err)
		}

		frame = nil
	}
}

// send reads data from the internal pipe and sends it over the stream.
func (c *streamConn) send(stream Stream) error {
	var frame *proto.Frame
	b := make([]byte, bufferSize)

	for {
		n, err := c.remote.Read(b)
		if err != nil {
			return trace.Wrap(err)
		}

		frame = &proto.Frame{Message: &proto.Frame_Data{Data: &proto.Data{Bytes: b[:n]}}}
		err = stream.Send(frame)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

// Read reads data reveived over the grpc stream.
func (c *streamConn) Read(b []byte) (n int, err error) {
	return c.local.Read(b)
}

// Write sends data over the grpc stream.
func (c *streamConn) Write(b []byte) (n int, err error) {
	return c.local.Write(b)
}

// Close cleans up resources used by the connection.
func (c *streamConn) Close() error {
	var err error
	c.once.Do(func() {
		err = c.close()
	})

	c.wg.Wait()
	return trace.Wrap(err)
}

// close cleans up resources used by the connection.
func (c *streamConn) close() error {
	c.local.Close()
	c.remote.Close()
	return nil
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
	return c.local.SetDeadline(t)
}

func (c *streamConn) SetReadDeadline(t time.Time) error {
	return c.local.SetReadDeadline(t)
}

func (c *streamConn) SetWriteDeadline(t time.Time) error {
	return c.local.SetWriteDeadline(t)
}

// pipeConn copies between two ReadWriteCloser and closes them when done.
func pipeConn(ctx context.Context, src io.ReadWriteCloser, dst io.ReadWriteCloser) (int64, int64) {
	var (
		sent, received int64
		wg             sync.WaitGroup
		o              sync.Once
	)

	cleanup := func() {
		o.Do(func() {
			src.Close()
			dst.Close()
		})
	}

	wg.Add(2)

	go func() {
		sent, _ = io.Copy(src, dst)
		cleanup()
		wg.Done()
	}()

	go func() {
		received, _ = io.Copy(dst, src)
		cleanup()
		wg.Done()
	}()

	wait := make(chan struct{})
	go func() {
		wg.Wait()
		close(wait)
	}()

	select {
	case <-ctx.Done():
	case <-wait:
	}

	cleanup()
	<-wait

	return sent, received
}
