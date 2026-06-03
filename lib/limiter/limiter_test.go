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

package limiter

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestRateLimiter(t *testing.T) {
	clock := clockwork.NewFakeClock()

	limiter, err := NewLimiter(
		Config{
			Clock: clock,
			Rates: []Rate{
				{
					Period:  10 * time.Millisecond,
					Average: 10,
					Burst:   20,
				},
				{
					Period:  40 * time.Millisecond,
					Average: 10,
					Burst:   40,
				},
			},
		})
	require.NoError(t, err)

	for range 20 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	for range 20 {
		require.NoError(t, limiter.RegisterRequest("token2"))
	}

	require.Error(t, limiter.RegisterRequest("token1"))

	clock.Advance(10 * time.Millisecond)
	for range 10 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	require.Error(t, limiter.RegisterRequest("token1"))

	clock.Advance(10 * time.Millisecond)
	for range 10 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	require.Error(t, limiter.RegisterRequest("token1"))

	clock.Advance(10 * time.Millisecond)
	// the second rate is full
	err = nil
	for range 10 {
		err = limiter.RegisterRequest("token1")
		if err != nil {
			break
		}
	}
	require.Error(t, err)

	clock.Advance(10 * time.Millisecond)
	// Now the second rate has free space
	require.NoError(t, limiter.RegisterRequest("token1"))
	err = nil
	for range 15 {
		err = limiter.RegisterRequest("token1")
		if err != nil {
			break
		}
	}
	require.Error(t, err)
}

type mockAddr struct{}

func (a mockAddr) Network() string {
	return "tcp"
}

func (a mockAddr) String() string {
	return "127.0.0.1:1234"
}

func TestLimiter_UnaryServerInterceptor(t *testing.T) {
	ctx := peer.NewContext(t.Context(), &peer.Peer{Addr: mockAddr{}})
	req := "request"
	serverInfo := &grpc.UnaryServerInfo{FullMethod: "/method"}
	handler := func(context.Context, any) (any, error) { return nil, nil }

	limiter, err := NewLimiter(Config{
		MaxConnections: 1,
		Rates:          []Rate{{Period: time.Minute, Average: 1, Burst: 1}},
	})
	require.NoError(t, err)

	interceptor := limiter.UnaryServerInterceptor()

	_, err = interceptor(ctx, req, serverInfo, handler)
	require.NoError(t, err)

	for range 10 {
		_, err = interceptor(ctx, req, serverInfo, handler)
		if err != nil {
			break
		}
	}
	require.Error(t, err)
}

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s mockServerStream) Context() context.Context {
	return s.ctx
}

func TestLimiter_StreamServerInterceptor(t *testing.T) {
	limiter, err := NewLimiter(Config{
		MaxConnections: 1,
		Rates: []Rate{
			{
				Period:  time.Minute,
				Average: 1,
				Burst:   1,
			},
		},
	})
	require.NoError(t, err)

	ctx := peer.NewContext(t.Context(), &peer.Peer{Addr: mockAddr{}})
	ss := mockServerStream{
		ctx: ctx,
	}
	info := &grpc.StreamServerInfo{}
	handler := func(srv any, stream grpc.ServerStream) error { return nil }

	// pass at least once
	err = limiter.StreamServerInterceptor(nil, ss, info, handler)
	require.NoError(t, err)

	// should eventually fail, not testing the limiter behavior here
	for range 10 {
		err = limiter.StreamServerInterceptor(nil, ss, info, handler)
		if err != nil {
			break
		}
	}
	require.Error(t, err)
}

func TestListener(t *testing.T) {
	failedAcceptErr := errors.New("failed accept")

	t.Run("allows and releases connections", func(t *testing.T) {
		limiter := NewConnectionsLimiter(2)
		ln, err := NewListener(&fakeListener{
			acceptConns: []net.Conn{
				&fakeConn{addr: mockAddr{}},
				&fakeConn{addr: mockAddr{}},
				&fakeConn{addr: mockAddr{}},
			},
		}, limiter)
		require.NoError(t, err)
		defer ln.Close()

		conn1, err := ln.Accept()
		require.NoError(t, err)
		conn2, err := ln.Accept()
		require.NoError(t, err)

		n, err := limiter.GetNumConnection("127.0.0.1")
		require.NoError(t, err)
		require.Equal(t, int64(2), n)

		require.NoError(t, conn1.Close())
		require.NoError(t, conn2.Close())

		n, err = limiter.GetNumConnection("127.0.0.1")
		require.NoError(t, err)
		require.Zero(t, n)

		conn3, err := ln.Accept()
		require.NoError(t, err)
		require.NoError(t, conn3.Close())
	})

	t.Run("returns listener accept errors", func(t *testing.T) {
		limiter := NewConnectionsLimiter(0)
		ln, err := NewListener(&fakeListener{acceptError: failedAcceptErr}, limiter)
		require.NoError(t, err)
		defer ln.Close()

		conn, err := ln.Accept()
		require.ErrorIs(t, err, failedAcceptErr)
		require.Nil(t, conn)
	})

	t.Run("closes invalid remote address and keeps accepting", func(t *testing.T) {
		// 0 means no connection limit per cient IP
		limiter := NewConnectionsLimiter(0)
		invalidConn := &fakeConn{
			addr: &utils.NetAddr{
				Addr:        "abcd",
				AddrNetwork: "tcp",
			},
		}
		validConn := &fakeConn{addr: mockAddr{}}

		ln, err := NewListener(&fakeListener{
			acceptConns: []net.Conn{invalidConn, validConn},
		}, limiter)
		require.NoError(t, err)
		defer ln.Close()

		conn, err := ln.Accept()
		require.NoError(t, err)
		wrapped, ok := conn.(*wrappedConn)
		require.True(t, ok)
		require.Same(t, validConn, wrapped.NetConn())
		require.True(t, invalidConn.closed)
	})
}

func TestListener_LimitExceeded(t *testing.T) {
	limiter := NewConnectionsLimiter(1)
	require.NoError(t, limiter.AcquireConnection("127.0.0.1"))
	defer limiter.ReleaseConnection("127.0.0.1")

	rejectedConn := &fakeConn{addr: mockAddr{}}
	allowedConn := &fakeConn{
		addr: &utils.NetAddr{
			Addr:        "127.0.0.2:1234",
			AddrNetwork: "tcp",
		},
	}

	var callbackRemoteAddr string
	var callbackErr error
	ln, err := NewListener(
		&fakeListener{acceptConns: []net.Conn{rejectedConn, allowedConn}},
		limiter,
		WithLimitExceededCallback(func(remoteAddr string, err error) {
			callbackRemoteAddr = remoteAddr
			callbackErr = err
		}),
	)
	require.NoError(t, err)
	defer ln.Close()

	conn, err := ln.Accept()
	require.NoError(t, err)
	wrapped, ok := conn.(*wrappedConn)
	require.True(t, ok)
	require.Same(t, allowedConn, wrapped.NetConn())

	// The rejected connection should be closed, the callback should be called
	// and the listener should still accept the allowed connection.
	require.True(t, rejectedConn.closed)
	require.Equal(t, "127.0.0.1", callbackRemoteAddr)
	require.True(t, trace.IsLimitExceeded(callbackErr))

	require.NoError(t, conn.Close())
}

type fakeListener struct {
	net.Listener

	acceptConns []net.Conn
	acceptError error
}

func (f *fakeListener) Accept() (net.Conn, error) {
	if f.acceptError != nil {
		return nil, f.acceptError
	}
	if len(f.acceptConns) == 0 {
		return nil, errors.New("fake listener exhausted")
	}
	conn := f.acceptConns[0]
	f.acceptConns = f.acceptConns[1:]
	return conn, nil
}

func (f *fakeListener) Close() error {
	return nil
}

type fakeConn struct {
	net.Conn

	addr   net.Addr
	closed bool
}

func (f *fakeConn) RemoteAddr() net.Addr {
	return f.addr
}

func (f *fakeConn) Close() error {
	f.closed = true
	return nil
}

// wrappedListener signals every Accept call so tests can observe that the
// server's accept loop kept running, and wraps accepted connections so tests
// can observe when a rejected connection is closed.
type wrappedListener struct {
	net.Listener
	closed  chan struct{}
	accepts chan struct{}
}

func (l *wrappedListener) Accept() (net.Conn, error) {
	select {
	case l.accepts <- struct{}{}:
	default:
	}
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &wrappedListenerConn{
		Conn:   conn,
		closed: l.closed,
	}, nil
}

// wrappedListenerConn lets the caller know when the connection is closed by sending on a channel.
type wrappedListenerConn struct {
	net.Conn
	closed chan struct{}
}

func (c *wrappedListenerConn) Close() error {
	err := c.Conn.Close()
	select {
	case c.closed <- struct{}{}:
	default:
	}
	return err
}

func TestMakeMiddleware(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{
		MaxConnections: 1,
		Rates: []Rate{
			{
				Period:  time.Minute,
				Average: 1,
				Burst:   1,
			},
		},
	})
	require.NoError(t, err)

	middleware := MakeMiddleware(limiter)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	mustServeAndReceiveStatusCode(t, handler, http.StatusAccepted)
	mustServeAndReceiveStatusCode(t, handler, http.StatusTooManyRequests)
}

func mustServeAndReceiveStatusCode(t *testing.T, handler http.Handler, wantStatusCode int) {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))

	response := recorder.Result()
	defer response.Body.Close()

	require.Equal(t, wantStatusCode, response.StatusCode)
}

func TestNoRates_RegisterRequest(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{})
	require.NoError(t, err)

	// With no rates configured, RegisterRequest should never reject.
	for range 100 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
}

func TestNoRates_Middleware(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{})
	require.NoError(t, err)

	middleware := MakeMiddleware(limiter)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	// With no rates and no max connections, every request should pass through.
	for range 100 {
		mustServeAndReceiveStatusCode(t, handler, http.StatusAccepted)
	}
}

func TestNoRates_IsRateLimited(t *testing.T) {
	t.Parallel()

	limiter, err := NewRateLimiter(Config{})
	require.NoError(t, err)

	// With no rates configured, IsRateLimited should always return false.
	require.False(t, limiter.IsRateLimited("token1"))

	// RegisterRequest is a no-op, so even after many calls the token
	// should not appear rate-limited.
	for range 100 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	require.False(t, limiter.IsRateLimited("token1"))
}

func TestNoRates_UnaryServerInterceptor(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{})
	require.NoError(t, err)

	ctx := peer.NewContext(t.Context(), &peer.Peer{Addr: mockAddr{}})
	serverInfo := &grpc.UnaryServerInfo{FullMethod: "/method"}
	handler := func(context.Context, any) (any, error) { return "ok", nil }

	interceptor := limiter.UnaryServerInterceptor()

	// With no rates, the interceptor should never reject.
	for range 100 {
		resp, err := interceptor(ctx, "request", serverInfo, handler)
		require.NoError(t, err)
		require.Equal(t, "ok", resp)
	}
}

func TestNoRates_StreamServerInterceptor(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{})
	require.NoError(t, err)

	ctx := peer.NewContext(t.Context(), &peer.Peer{Addr: mockAddr{}})
	ss := mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{}
	handler := func(srv any, stream grpc.ServerStream) error { return nil }

	// With no rates, the interceptor should never reject.
	for range 100 {
		require.NoError(t, limiter.StreamServerInterceptor(nil, ss, info, handler))
	}
}

func TestNoRates_RegisterRequestAndConnection(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{})
	require.NoError(t, err)

	// With no rates and no max connections, should never reject.
	for range 100 {
		release, err := limiter.RegisterRequestAndConnection("127.0.0.1")
		require.NoError(t, err)
		release()
	}
}

func TestNoRates_WithMaxConnections(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{MaxConnections: 2})
	require.NoError(t, err)

	// Rate limiting should pass through (no rates), but connection
	// limiting should still enforce.
	for range 100 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}

	// Connection limit should still work.
	release1, err := limiter.RegisterRequestAndConnection("127.0.0.1")
	require.NoError(t, err)
	release2, err := limiter.RegisterRequestAndConnection("127.0.0.1")
	require.NoError(t, err)

	// Third connection should be rejected.
	_, err = limiter.RegisterRequestAndConnection("127.0.0.1")
	require.Error(t, err)
	require.True(t, trace.IsLimitExceeded(err))

	// Release one, then the third should succeed.
	release1()
	release3, err := limiter.RegisterRequestAndConnection("127.0.0.1")
	require.NoError(t, err)
	release2()
	release3()
}

func TestNoRates_MiddlewareWithMaxConnections(t *testing.T) {
	t.Parallel()

	limiter, err := NewLimiter(Config{MaxConnections: 1})
	require.NoError(t, err)

	// Verify rate limiting passes through in HTTP path by sending
	// many sequential requests (no concurrent connections).
	middleware := MakeMiddleware(limiter)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	for range 100 {
		mustServeAndReceiveStatusCode(t, handler, http.StatusAccepted)
	}
}

func TestRateLimiter_IsRateLimited(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	limiter, err := NewRateLimiter(Config{
		Clock: clock,
		Rates: []Rate{
			{
				Period:  time.Minute,
				Average: 10,
				Burst:   10,
			},
		},
	})
	require.NoError(t, err)
	require.False(t, limiter.IsRateLimited("token1"))

	// Consume some tokens but not all
	for range 5 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}

	require.False(t, limiter.IsRateLimited("token1"))

	// Consume the rest of the tokens
	for range 4 {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	require.False(t, limiter.IsRateLimited("token1"))

	// Consume the last token
	require.NoError(t, limiter.RegisterRequest("token1"))
	// Now token1 should be rate limited
	require.True(t, limiter.IsRateLimited("token1"))
	// token2 should not be rate limited
	require.False(t, limiter.IsRateLimited("token2"))

	clock.Advance(time.Minute)
	// After time passes, token1 should not be rate limited anymore
	require.False(t, limiter.IsRateLimited("token1"))
}

// TestIndependentLimiters verifies that two Limiter instances
// maintain independent token buckets for the same client IP.
func TestIndependentLimiters(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()

	defaultLimiter, err := NewLimiter(Config{
		Clock: clock,
		Rates: []Rate{{Period: 10 * time.Millisecond, Average: 10, Burst: 20}},
	})
	require.NoError(t, err)

	recoveryLimiter, err := NewLimiter(Config{
		Clock: clock,
		Rates: []Rate{{Period: time.Minute, Average: 1, Burst: 5}},
	})
	require.NoError(t, err)

	// Exhaust the recovery limiter (burst 5).
	for range 5 {
		require.NoError(t, recoveryLimiter.RegisterRequest("127.0.0.1"))
	}
	require.Error(t, recoveryLimiter.RegisterRequest("127.0.0.1"))

	// Default limiter for the same IP must still work.
	require.NoError(t, defaultLimiter.RegisterRequest("127.0.0.1"))

	// Recovery limiter must still be exhausted.
	require.Error(t, recoveryLimiter.RegisterRequest("127.0.0.1"))

	// Default limiter has its own independent bucket.
	for range 19 {
		require.NoError(t, defaultLimiter.RegisterRequest("127.0.0.1"))
	}
	require.Error(t, defaultLimiter.RegisterRequest("127.0.0.1"))
}

func TestListener_LimitExceededDoesNotTerminateServe(t *testing.T) {
	tests := []struct {
		name      string
		serve     func(t *testing.T, ln net.Listener) (stop func(), done <-chan error)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "grpc",
			serve: func(t *testing.T, ln net.Listener) (func(), <-chan error) {
				srv := grpc.NewServer()
				done := make(chan error, 1)
				go func() { done <- srv.Serve(ln) }()
				return srv.Stop, done
			},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "http",
			serve: func(t *testing.T, ln net.Listener) (func(), <-chan error) {
				srv := &http.Server{
					Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				}
				done := make(chan error, 1)
				go func() { done <- srv.Serve(ln) }()
				return func() {
					require.NoError(t, srv.Close())
				}, done
			},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, http.ErrServerClosed)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const timeout = 5 * time.Second
			base, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer base.Close()

			accepts := make(chan struct{}, 1)
			closed := make(chan struct{}, 1)
			cl := NewConnectionsLimiter(1)
			ln, err := NewListener(&wrappedListener{
				Listener: base,
				accepts:  accepts,
				closed:   closed,
			}, cl)
			require.NoError(t, err)
			defer ln.Close()

			// Saturate the limit so the next accepted connection is rejected.
			require.NoError(t, cl.AcquireConnection("127.0.0.1"))
			defer cl.ReleaseConnection("127.0.0.1")

			stop, done := tt.serve(t, ln)

			// Drain the Accept call Serve makes on startup.
			select {
			case <-accepts:
			case <-time.After(timeout):
				t.Fatal("timed out waiting for Serve to call Accept")
			}

			// Force a rejected Accept by dialing into the saturated limiter.
			c, err := net.Dial("tcp", base.Addr().String())
			require.NoError(t, err)
			defer c.Close()

			// Wait until the rejected connection is closed by Listener.Accept.
			select {
			case <-closed:
			case <-time.After(timeout):
				t.Fatal("timed out waiting for rejected connection to close")
			}

			// Check if server is still running after rejecting connection.
			select {
			case <-accepts:
				// Serve kept running after the rejected connection.
			case err := <-done:
				t.Fatalf("%s Server.Serve exited after rejecting one connection: %v", tt.name, err)
			case <-time.After(timeout):
				t.Fatal("timed out waiting for Serve to re-Accept after rejected connection")
			}

			stop()
			tt.assertErr(t, <-done)
		})
	}
}
