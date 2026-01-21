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

package websocketupgradeproto

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBufferlessConn() *bufferlessConn {
	c := new(bufferlessConn)
	c.cond.L = &c.mu
	return c
}

func waitForWriteRequestLocked(t *testing.T, c *bufferlessConn) *request {
	t.Helper()
	for {
		if c.writeReq != nil && !c.writeReq.done {
			return c.writeReq
		}
		if c.localClosed {
			t.Fatalf("connection closed before write request was available")
		}
		c.cond.Wait()
	}
}

func waitForReadRequestLocked(t *testing.T, c *bufferlessConn) *request {
	t.Helper()
	for {
		if c.readReq != nil && !c.readReq.done {
			return c.readReq
		}
		if c.localClosed {
			t.Fatalf("connection closed before read request was available")
		}
		c.cond.Wait()
	}
}

func TestBufferlessConn(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		c := newBufferlessConn()
		t.Cleanup(func() { c.Close() })

		go func() {
			c.mu.Lock()
			defer c.mu.Unlock()

			const expected = "GET / HTTP/1.1\r\n"
			var captured bytes.Buffer
			for {
				req := waitForWriteRequestLocked(t, c)
				chunk := req.buf[req.n:]
				captured.Write(chunk)
				req.n = len(req.buf)
				req.done = true
				c.cond.Broadcast()

				if strings.Contains(captured.String(), "\r\n\r\n") {
					break
				}
			}

			reqLine := captured.String()
			require.True(t, strings.HasPrefix(reqLine, expected), "unexpected request: %q", reqLine)

			response := []byte("HTTP/1.0 200 OK\r\n" + "content-type:text/plain\r\n" + "content-length:5\r\n" + "\r\n" + "hello")
			sent := 0
			for sent < len(response) {
				req := waitForReadRequestLocked(t, c)
				n := copy(req.buf[req.n:], response[sent:])
				req.n += n
				req.done = true
				c.cond.Broadcast()
				sent += n
			}

			c.readErr = io.EOF
			c.remoteClosed = true
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
		c := newBufferlessConn()
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
		c := newBufferlessConn()
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
		c := newBufferlessConn()
		t.Cleanup(func() { c.Close() })

		go func() {
			c.mu.Lock()
			defer c.mu.Unlock()

			payload := []byte("hello")
			sent := 0
			for sent < len(payload) {
				req := waitForReadRequestLocked(t, c)
				n := copy(req.buf[req.n:], payload[sent:])
				req.n += n
				req.done = true
				c.cond.Broadcast()
				sent += n
			}

			c.readErr = io.EOF
			c.remoteClosed = true
			c.cond.Broadcast()
		}()

		b, err := io.ReadAll(c)
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), b)

		n, err := c.Write(b)
		require.ErrorIs(t, err, syscall.EPIPE)
		require.Zero(t, n)
	})

	t.Run("WriteBuffering", func(t *testing.T) {
		c := newBufferlessConn()
		t.Cleanup(func() { c.Close() })

		const testSize = 128 * 1024 * 10
		data := bytes.Repeat([]byte("a"), testSize)

		var consumed int
		done := make(chan struct{})
		go func() {
			defer close(done)
			c.mu.Lock()
			defer c.mu.Unlock()

			for consumed < testSize {
				req := waitForWriteRequestLocked(t, c)
				chunk := req.buf[req.n:]
				for _, b := range chunk {
					require.Equal(t, byte('a'), b)
				}
				consumed += len(chunk)
				req.n = len(req.buf)
				req.done = true
				c.cond.Broadcast()
			}
		}()

		n, err := c.Write(data)
		require.NoError(t, err)
		require.EqualValues(t, testSize, n)
		<-done
	})

	t.Run("ReadBuffering", func(t *testing.T) {
		c := newBufferlessConn()
		t.Cleanup(func() { c.Close() })

		const testSize = 128 * 1024 * 10
		var (
			copyErr error
			copied  int64
		)
		done := make(chan struct{})
		go func() {
			copied, copyErr = io.Copy(io.Discard, c)
			close(done)
		}()

		payload := bytes.Repeat([]byte("a"), testSize)

		c.mu.Lock()
		sent := 0
		for sent < len(payload) {
			req := waitForReadRequestLocked(t, c)
			n := copy(req.buf[req.n:], payload[sent:])
			req.n += n
			req.done = true
			c.cond.Broadcast()
			sent += n
		}

		c.readErr = io.EOF
		c.remoteClosed = true
		c.cond.Broadcast()
		c.mu.Unlock()

		<-done
		require.NoError(t, copyErr)
		require.EqualValues(t, testSize, copied)
	})
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
