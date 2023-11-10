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
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (m *mockStream) Send(b []byte) error {
	_, err := m.conn.Write(b)
	return err
}

func (m *mockStream) Recv() ([]byte, error) {
	b := make([]byte, 2*MaxChunkSize)
	n, err := m.conn.Read(b)
	return b[:n], err
}

func newStreamPipe(t *testing.T) (*ReadWriter, net.Conn) {
	local, remote := net.Pipe()
	stream := newMockStream(context.Background(), remote)

	timeout := time.Now().Add(time.Second * 5)

	require.NoError(t, local.SetReadDeadline(timeout))
	require.NoError(t, local.SetWriteDeadline(timeout))
	require.NoError(t, remote.SetReadDeadline(timeout))
	require.NoError(t, remote.SetWriteDeadline(timeout))

	streamConn, err := NewReadWriter(stream)
	require.NoError(t, err)

	return streamConn, local
}

func TestReadWriter_Write(t *testing.T) {
	streamConn, local := newStreamPipe(t)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	data := []byte("hello world!")
	go func() {
		defer wg.Done()
		n, err := streamConn.Write(data)
		assert.NoError(t, err)
		assert.Len(t, data, n)
	}()
	go func() {
		defer wg.Done()
		b := make([]byte, 2*MaxChunkSize)
		n, err := local.Read(b)
		assert.NoError(t, err)
		assert.Len(t, data, n)
		assert.Equal(t, data, b[:n])
	}()

	wg.Wait()
}

func TestReadWriter_WriteChunk(t *testing.T) {
	streamConn, local := newStreamPipe(t)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	data := make([]byte, MaxChunkSize+1)
	go func() {
		defer wg.Done()
		n, err := streamConn.Write(data)
		assert.NoError(t, err)
		assert.Len(t, data, n)
	}()
	go func() {
		defer wg.Done()
		b := make([]byte, 2*MaxChunkSize)
		n, err := local.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, MaxChunkSize, n)
		assert.Equal(t, data[:n], b[:n])

		n, err = local.Read(b)
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, data[:n], b[:n])
	}()

	wg.Wait()
}

func TestReadWriter_Read(t *testing.T) {
	streamConn, local := newStreamPipe(t)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	data := make([]byte, MaxChunkSize+1)
	go func() {
		b := make([]byte, 2*MaxChunkSize)
		defer wg.Done()
		n, err := streamConn.Read(b)
		assert.NoError(t, err)
		assert.Len(t, data, n)
		assert.Equal(t, data, b[:n])
	}()
	go func() {
		defer wg.Done()
		n, err := local.Write(data)
		assert.NoError(t, err)
		assert.Len(t, data, n)
	}()

	wg.Wait()
}
