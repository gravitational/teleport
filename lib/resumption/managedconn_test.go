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
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagedConn(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		c := newManagedConn()
		t.Cleanup(func() { c.Close() })

		go func() {
			c.mu.Lock()
			defer c.mu.Unlock()

			const expected = "GET / HTTP/1.1\r\n"
			for {
				if c.localClosed {
					assert.FailNow(t, "connection locally closed before receiving request")
					return
				}

				b, _ := c.sendBuffer.buffered()
				if len(b) >= len(expected) {
					break
				}

				c.cond.Wait()
			}

			b, _ := c.sendBuffer.buffered()
			if !assert.Equal(t, []byte(expected), b[:len(expected)]) {
				c.remoteClosed = true
				c.cond.Broadcast()
				return
			}

			c.receiveBuffer.append([]byte("HTTP/1.0 200 OK\r\n" + "content-type:text/plain\r\n" + "content-length:5\r\n" + "\r\n" + "hello"))
			c.cond.Broadcast()
		}()

		ht := http.DefaultTransport.(*http.Transport).Clone()
		ht.DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
			return c, nil
		}
		ht.ResponseHeaderTimeout = 5 * time.Second
		ht.IdleConnTimeout = time.Nanosecond
		req, err := http.NewRequest("GET", "http://127.0.0.1/", http.NoBody)
		require.NoError(t, err)

		resp, err := ht.RoundTrip(req)
		require.NoError(t, err)
		t.Cleanup(func() { resp.Body.Close() })
		require.Equal(t, http.StatusOK, resp.StatusCode)

		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), b)
		require.NoError(t, resp.Body.Close())

		c.mu.Lock()
		for !c.localClosed {
			c.cond.Wait()
		}
		c.mu.Unlock()
	})

	t.Run("Deadline", func(t *testing.T) {
		c := newManagedConn()
		t.Cleanup(func() { c.Close() })

		require.NoError(t, c.SetReadDeadline(time.Now().Add(time.Hour)))
		go func() {
			// XXX: this may or may not give enough time to reach the c.Read
			// below, but either way the result doesn't change
			time.Sleep(50 * time.Millisecond)
			assert.NoError(t, c.SetReadDeadline(time.Unix(0, 1)))
		}()

		var b [1]byte
		n, err := c.Read(b[:])
		require.ErrorIs(t, err, os.ErrDeadlineExceeded)
		require.Zero(t, n)

		require.True(t, c.readDeadline.timeout)
	})

	t.Run("LocalClosed", func(t *testing.T) {
		c := newManagedConn()
		c.SetDeadline(time.Now().Add(time.Hour))
		c.Close()

		// deadline timers are stopped after Close
		require.NotNil(t, c.readDeadline.timer)
		require.False(t, c.readDeadline.timer.Stop())
		require.NotNil(t, c.writeDeadline.timer)
		require.False(t, c.writeDeadline.timer.Stop())

		var b [1]byte
		n, err := c.Read(b[:])
		require.ErrorIs(t, err, net.ErrClosed)
		require.Zero(t, n)

		n, err = c.Write(b[:])
		require.ErrorIs(t, err, net.ErrClosed)
		require.Zero(t, n)
	})

	t.Run("RemoteClosed", func(t *testing.T) {
		c := newManagedConn()
		t.Cleanup(func() { c.Close() })
		c.receiveBuffer.append([]byte("hello"))
		c.remoteClosed = true

		b, err := io.ReadAll(c)
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), b)

		n, err := c.Write(b)
		require.ErrorIs(t, err, syscall.EPIPE)
		require.Zero(t, n)
	})

	t.Run("WriteBuffering", func(t *testing.T) {
		c := newManagedConn()
		t.Cleanup(func() { c.Close() })

		const testSize = sendBufferSize * 10
		go func() {
			defer c.Close()
			_, err := c.Write(bytes.Repeat([]byte("a"), testSize))
			assert.NoError(t, err)
		}()

		var n uint64
		c.mu.Lock()
		defer c.mu.Unlock()
		for {
			require.LessOrEqual(t, len(c.sendBuffer.data), sendBufferSize)

			n += c.sendBuffer.len()

			c.sendBuffer.advance(c.sendBuffer.len())
			c.cond.Broadcast()

			if c.localClosed {
				break
			}

			c.cond.Wait()
		}
		require.EqualValues(t, testSize, n)
	})

	t.Run("ReadBuffering", func(t *testing.T) {
		c := newManagedConn()
		t.Cleanup(func() { c.Close() })

		const testSize = sendBufferSize * 10
		go func() {
			defer c.Close()

			n, err := io.Copy(io.Discard, c)
			assert.NoError(t, err)
			assert.EqualValues(t, testSize, n)
		}()

		b := bytes.Repeat([]byte("a"), testSize)

		c.mu.Lock()
		defer c.mu.Unlock()

		for !c.localClosed {
			s := c.receiveBuffer.write(b, receiveBufferSize)
			if s > 0 {
				c.cond.Broadcast()

				require.LessOrEqual(t, len(c.receiveBuffer.data), receiveBufferSize)

				b = b[s:]
				if len(b) == 0 {
					c.remoteClosed = true
				}
			}

			c.cond.Wait()
		}

		require.Empty(t, b)
	})
}

func TestBuffer(t *testing.T) {
	t.Parallel()

	var b buffer
	require.Zero(t, b.len())

	b.append([]byte("a"))
	require.EqualValues(t, 1, b.len())

	b.append(bytes.Repeat([]byte("a"), 9999))
	require.EqualValues(t, 10000, b.len())
	require.Len(t, b.data, 16384)

	b.advance(5000)
	require.EqualValues(t, 5000, b.len())
	require.Len(t, b.data, 16384)

	b1, b2 := b.free()
	require.NotEmpty(t, b1)
	require.NotEmpty(t, b2)
	require.EqualValues(t, 16384-5000, len(b1)+len(b2))

	b.append(bytes.Repeat([]byte("a"), 7000))
	require.EqualValues(t, 12000, b.len())
	require.Len(t, b.data, 16384)

	b1, b2 = b.free()
	require.NotEmpty(t, b1)
	require.Empty(t, b2)
	require.Len(t, b1, 16384-12000)

	b1, b2 = b.buffered()
	require.NotEmpty(t, b1)
	require.NotEmpty(t, b2)
	require.EqualValues(t, 12000, len(b1)+len(b2))
}

func TestDeadline(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	cond := sync.Cond{L: &mu}
	var d deadline
	clock := clockwork.NewFakeClock()

	setDeadlineLocked := func(t time.Time) { d.setDeadlineLocked(t, &cond, clock) }

	mu.Lock()
	defer mu.Unlock()

	setDeadlineLocked(clock.Now().Add(time.Minute))
	require.False(t, d.timeout)
	require.NotNil(t, d.timer)
	require.False(t, d.stopped)

	clock.Advance(time.Minute)
	for !d.timeout {
		cond.Wait()
	}
	require.True(t, d.stopped)

	setDeadlineLocked(time.Time{})
	require.False(t, d.timeout)
	require.True(t, d.stopped)

	setDeadlineLocked(time.Unix(0, 1))
	require.True(t, d.timeout)
	require.True(t, d.stopped)

	setDeadlineLocked(clock.Now().Add(time.Minute))
	require.False(t, d.timeout)
	require.False(t, d.stopped)

	setDeadlineLocked(time.Time{})
	require.False(t, d.timeout)
	require.True(t, d.stopped)
}
