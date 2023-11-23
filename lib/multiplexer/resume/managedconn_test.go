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

package resume

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
		req, err := http.NewRequest("GET", "http://127.0.0.1/", http.NoBody)
		require.NoError(t, err)

		resp, err := ht.RoundTrip(req)
		require.NoError(t, err)
		t.Cleanup(func() { resp.Body.Close() })
		require.Equal(t, http.StatusOK, resp.StatusCode)

		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), b)

		ht.CloseIdleConnections()
		require.True(t, c.localClosed)
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
		c.Close()

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
}

func TestBuffer(t *testing.T) {
	var b buffer
	require.Zero(t, b.len())

	b.append([]byte("a"))
	require.EqualValues(t, 1, b.len())

	b.append(bytes.Repeat([]byte("a"), 9999))
	require.EqualValues(t, 10000, b.len())
	require.EqualValues(t, 16384, len(b.data))

	b.advance(5000)
	require.EqualValues(t, 5000, b.len())
	require.EqualValues(t, 16384, len(b.data))

	b1, b2 := b.free()
	require.NotEmpty(t, b1)
	require.NotEmpty(t, b2)
	require.EqualValues(t, 16384-5000, len(b1)+len(b2))

	b.append(bytes.Repeat([]byte("a"), 7000))
	require.EqualValues(t, 12000, b.len())
	require.EqualValues(t, 16384, len(b.data))

	b1, b2 = b.free()
	require.NotEmpty(t, b1)
	require.Empty(t, b2)
	require.EqualValues(t, 16384-12000, len(b1))

	b1, b2 = b.buffered()
	require.NotEmpty(t, b1)
	require.NotEmpty(t, b2)
	require.EqualValues(t, 12000, len(b1)+len(b2))
}

func TestDeadline(t *testing.T) {
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
