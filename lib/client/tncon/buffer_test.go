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
			desc:     "read with equal buffer",
			buffer:   10,
			writeLen: 10,
			readLen:  10,
			expectN:  10,
		}, {
			desc:     "large read with large buffer",
			buffer:   100,
			writeLen: 10000,
			readLen:  10000,
			expectN:  10000,
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
			n, err := io.ReadFull(buffer, p)
			require.NoError(t, err)
			require.Equal(t, tc.expectN, n)
			require.Equal(t, write[:n], p[:n])
		})
	}
}

func BenchmarkBufferedChannelPipe(b *testing.B) {
	for _, s := range []int{1, 10, 100, 500, 1000} {
		data := make([]byte, 1000)
		b.Run(fmt.Sprintf("size=%d", s), func(b *testing.B) {
			buffer := newBufferedChannelPipe(s)
			b.Cleanup(func() { require.NoError(b, buffer.Close()) })

			errCh := make(chan error)
			go func() {
				readBuffer := make([]byte, b.N*len(data))
				_, err := io.ReadFull(buffer, readBuffer)
				errCh <- err
			}()

			// benchmark write+read
			for b.Loop() {
				written, err := buffer.Write(data)
				require.NoError(b, err)
				require.Len(b, data, written)
			}
			require.NoError(b, <-errCh)
			b.StopTimer()
		})
	}
}
