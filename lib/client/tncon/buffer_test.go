/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tncon

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBufferedChannelPipeClose(t *testing.T) {
	buffer := newBufferedChannelPipe(0)
	require.NoError(t, buffer.Close())

	// Reading from a closed channel should return EOF
	n, err := buffer.Read(make([]byte, 1))
	require.Equal(t, 0, n)
	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF)

	// Reading from a closed channel should return ErrClosedPipe
	n, err = buffer.Write(make([]byte, 1))
	require.Equal(t, 0, n)
	require.Error(t, err)
	require.ErrorIs(t, err, io.EOF)
}

func TestBufferedChannelPipeWrite(t *testing.T) {
	// With a sufficient buffer, write should successfully
	// write to the channel without blocking
	for _, tc := range []struct {
		buffer int
		len    int
	}{
		{
			buffer: 0,
			len:    0,
		}, {
			buffer: 0,
			len:    10,
		}, {
			buffer: 10,
			len:    10,
		}, {
			buffer: 10,
			len:    100,
		}, {
			buffer: 100,
			len:    100,
		},
	} {
		t.Run(fmt.Sprintf("buffer=%v, len=%v", tc.buffer, tc.len), func(t *testing.T) {
			buffer := newBufferedChannelPipe(tc.buffer)
			t.Cleanup(func() { require.NoError(t, buffer.Close()) })

			// drain channel
			rc := make(chan []byte)
			go func() {
				read := make([]byte, tc.len)
				for i := range read {
					read[i] = <-buffer.ch
				}
				rc <- read
			}()

			p := make([]byte, tc.len)
			for n := 0; n < tc.len; n++ {
				p[n] = byte(n)
			}

			n, err := buffer.Write(p)
			require.NoError(t, err)
			require.Equal(t, tc.len, n)
			require.Equal(t, p, <-rc)
		})
	}
}

func TestBufferedChannelPipeRead(t *testing.T) {
	for _, tc := range []struct {
		desc     string
		buffer   int
		writeLen int
		readLen  int
		expectN  int
	}{
		{
			desc:     "empty read",
			buffer:   0,
			writeLen: 0,
			readLen:  0,
			expectN:  0,
		}, {
			desc:     "one byte read",
			buffer:   0,
			writeLen: 1,
			readLen:  1,
			expectN:  1,
		}, {
			desc:     "read with sufficient buffer",
			buffer:   10,
			writeLen: 10,
			readLen:  10,
			expectN:  10,
		}, {
			desc:     "large read with sufficient buffer",
			buffer:   255,
			writeLen: 255,
			readLen:  255,
			expectN:  255,
		}, {
			desc:     "read with smaller write",
			buffer:   100,
			writeLen: 10,
			readLen:  100,
			expectN:  10,
		}, {
			desc:     "read with larger write",
			buffer:   100,
			writeLen: 100,
			readLen:  10,
			expectN:  10,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			buffer := newBufferedChannelPipe(tc.buffer)
			t.Cleanup(func() { require.NoError(t, buffer.Close()) })

			write := make([]byte, tc.writeLen)
			for i := 0; i < tc.writeLen; i++ {
				write[i] = byte(i)
			}

			// fill channel
			go buffer.Write(write)

			p := make([]byte, tc.readLen)
			n, err := buffer.Read(p)
			require.NoError(t, err)
			require.Equal(t, tc.expectN, n)
			require.Equal(t, write[:n], p[:n])
		})
	}
}

func BenchmarkBufferedChannelPipe(b *testing.B) {
	buffer := newBufferedChannelPipe(sequenceBufferSize)

	go func() {
		buffer.Write(make([]byte, b.N))
		buffer.Close()
	}()

	_, err := io.ReadAll(buffer)
	require.NoError(b, err)
}
