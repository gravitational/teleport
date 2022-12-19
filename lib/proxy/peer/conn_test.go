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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/utils"
)

type mockStream struct {
	ctx  context.Context
	conn net.Conn
}

func newMockStream(ctx context.Context, conn net.Conn) *mockStream {
	return &mockStream{
		ctx:  ctx,
		conn: conn,
	}
}

func (m *mockStream) Context() context.Context {
	return m.ctx
}

func (m *mockStream) Send(f *proto.Frame) error {
	_, err := m.conn.Write(f.GetData().Bytes)
	return err
}

func (m *mockStream) Recv() (*proto.Frame, error) {
	b := make([]byte, 2*maxChunkSize)
	n, err := m.conn.Read(b)
	return &proto.Frame{Message: &proto.Frame_Data{Data: &proto.Data{
		Bytes: b[:n],
	}}}, err
}

func newStreamPipe() (*streamConn, net.Conn) {
	local, remote := net.Pipe()
	stream := newMockStream(context.Background(), remote)

	timeout := time.Now().Add(time.Second * 5)
	local.SetReadDeadline(timeout)
	local.SetWriteDeadline(timeout)
	remote.SetReadDeadline(timeout)
	remote.SetWriteDeadline(timeout)

	src := &utils.NetAddr{Addr: "src"}
	dst := &utils.NetAddr{Addr: "dst"}
	streamConn := newStreamConn(stream, src, dst)

	return streamConn, local
}

func TestStreamConnWrite(t *testing.T) {
	streamConn, local := newStreamPipe()
	wg := &sync.WaitGroup{}
	wg.Add(2)

	data := []byte("hello world!")
	go func() {
		defer wg.Done()
		n, err := streamConn.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
	}()
	go func() {
		defer wg.Done()
		b := make([]byte, 2*maxChunkSize)
		n, err := local.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, b[:n])
	}()

	wg.Wait()
}

func TestStreamConnWriteChunk(t *testing.T) {
	streamConn, local := newStreamPipe()
	wg := &sync.WaitGroup{}
	wg.Add(2)

	data := make([]byte, maxChunkSize+1)
	go func() {
		defer wg.Done()
		n, err := streamConn.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
	}()
	go func() {
		defer wg.Done()
		b := make([]byte, 2*maxChunkSize)
		n, err := local.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, maxChunkSize, n)
		assert.Equal(t, data[:n], b[:n])

		n, err = local.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, data[:n], b[:n])
	}()

	wg.Wait()
}

func TestStreamConnRead(t *testing.T) {
	streamConn, local := newStreamPipe()
	wg := &sync.WaitGroup{}
	wg.Add(2)

	data := make([]byte, maxChunkSize+1)
	go func() {
		b := make([]byte, 2*maxChunkSize)
		defer wg.Done()
		n, err := streamConn.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, data, b[:n])
	}()
	go func() {
		defer wg.Done()
		n, err := local.Write(data)
		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
	}()

	wg.Wait()
}

func TestPipeConn(t *testing.T) {
	local1, remote1 := net.Pipe()
	local2, remote2 := net.Pipe()

	timeout := time.Now().Add(time.Second * 5)
	local1.SetReadDeadline(timeout)
	local1.SetWriteDeadline(timeout)
	local2.SetReadDeadline(timeout)
	local2.SetWriteDeadline(timeout)

	var (
		sent int64
		recv int64
		err  error
	)

	c := make(chan struct{})
	go func() {
		sent, recv, err = pipeConn(context.Background(), remote1, remote2)
		close(c)
	}()

	data1 := []byte("hello world!")
	n1, err := local1.Write(data1)
	require.NoError(t, err)
	require.Equal(t, len(data1), n1)

	data2 := make([]byte, 2*len(data1))
	n2, err := local2.Read(data2)
	require.NoError(t, err)
	require.Equal(t, data1, data2[:n2])

	data3 := []byte("goodbye.")
	n3, err := local2.Write(data3)
	require.NoError(t, err)
	require.Equal(t, len(data3), n3)

	data4 := make([]byte, 2*len(data3))
	n4, err := local1.Read(data4)
	require.NoError(t, err)
	require.Equal(t, data3, data4[:n4])

	local1.Close()
	timer := time.NewTimer(time.Second * 5)
	select {
	case <-c:
	case <-timer.C:
		require.FailNow(t, "timeout waiting for pipeConn to return")
	}

	require.Equal(t, int64(len(data3)), sent)
	require.Equal(t, int64(len(data1)), recv)
}
