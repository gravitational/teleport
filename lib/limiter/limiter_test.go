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

	"github.com/gravitational/teleport/lib/limiter/internal/ratelimit"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
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

	for i := 0; i < 20; i++ {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	for i := 0; i < 20; i++ {
		require.NoError(t, limiter.RegisterRequest("token2"))
	}

	require.Error(t, limiter.RegisterRequest("token1"))

	clock.Advance(10 * time.Millisecond)
	for i := 0; i < 10; i++ {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	require.Error(t, limiter.RegisterRequest("token1"))

	clock.Advance(10 * time.Millisecond)
	for i := 0; i < 10; i++ {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
	require.Error(t, limiter.RegisterRequest("token1"))

	clock.Advance(10 * time.Millisecond)
	// the second rate is full
	err = nil
	for i := 0; i < 10; i++ {
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
	for i := 0; i < 15; i++ {
		err = limiter.RegisterRequest("token1")
		if err != nil {
			break
		}
	}
	require.Error(t, err)
}

func TestCustomRate(t *testing.T) {
	clock := clockwork.NewFakeClock()

	limiter, err := NewLimiter(
		Config{
			Clock: clock,
			Rates: []Rate{
				// Default rate
				{
					Period:  10 * time.Millisecond,
					Average: 10,
					Burst:   20,
				},
			},
		})
	require.NoError(t, err)

	customRate := ratelimit.NewRateSet()
	err = customRate.Add(time.Minute, 1, 5)
	require.NoError(t, err)

	// Max out custom rate.
	for i := 0; i < 5; i++ {
		require.NoError(t, limiter.RegisterRequestWithCustomRate("token1", customRate))
	}

	// Test rate limit exceeded with custom rate.
	require.Error(t, limiter.RegisterRequestWithCustomRate("token1", customRate))

	// Test default rate still works.
	for i := 0; i < 20; i++ {
		require.NoError(t, limiter.RegisterRequest("token1"))
	}
}

type mockAddr struct{}

func (a mockAddr) Network() string {
	return "tcp"
}

func (a mockAddr) String() string {
	return "127.0.0.1:1234"
}

func TestLimiter_UnaryServerInterceptor(t *testing.T) {
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

	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: mockAddr{}})
	req := "request"
	serverInfo := &grpc.UnaryServerInfo{
		FullMethod: "/method",
	}
	handler := func(context.Context, interface{}) (interface{}, error) { return nil, nil }

	unaryInterceptor := limiter.UnaryServerInterceptor()

	// pass at least once
	_, err = unaryInterceptor(ctx, req, serverInfo, handler)
	require.NoError(t, err)

	// should eventually fail, not testing the limiter behavior here
	for i := 0; i < 10; i++ {
		_, err = unaryInterceptor(ctx, req, serverInfo, handler)
		if err != nil {
			break
		}
	}
	require.Error(t, err)

	getCustomRate := func(endpoint string) *ratelimit.RateSet {
		rates := ratelimit.NewRateSet()
		err := rates.Add(2*time.Minute, 1, 2)
		require.NoError(t, err)
		return rates
	}

	unaryInterceptor = limiter.UnaryServerInterceptorWithCustomRate(getCustomRate)

	// should pass at least once
	_, err = unaryInterceptor(ctx, req, serverInfo, handler)
	require.NoError(t, err)

	// should eventually fail, not testing the limiter behavior here
	for i := 0; i < 10; i++ {
		_, err = unaryInterceptor(ctx, req, serverInfo, handler)
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

	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: mockAddr{}})
	ss := mockServerStream{
		ctx: ctx,
	}
	info := &grpc.StreamServerInfo{}
	handler := func(srv interface{}, stream grpc.ServerStream) error { return nil }

	// pass at least once
	err = limiter.StreamServerInterceptor(nil, ss, info, handler)
	require.NoError(t, err)

	// should eventually fail, not testing the limiter behavior here
	for i := 0; i < 10; i++ {
		err = limiter.StreamServerInterceptor(nil, ss, info, handler)
		if err != nil {
			break
		}
	}
	require.Error(t, err)
}

// TestListener verifies that a [Listener] only accepts
// connections if the connection limit has not been exceeded.
func TestListener(t *testing.T) {
	const connLimit = 5
	failedAcceptErr := errors.New("failed accept")
	tooManyConnectionsErr := trace.LimitExceeded("too many connections from 127.0.0.1: 2, max is 2")

	tests := []struct {
		name             string
		config           Config
		listener         *fakeListener
		acceptAssertion  func(t *testing.T, iteration int, conn net.Conn, err error)
		numConnAssertion func(t *testing.T, num int64)
	}{
		{
			name:   "all connections allowed",
			config: Config{MaxConnections: 0},
			listener: &fakeListener{
				acceptConn: &fakeConn{
					addr: mockAddr{},
				},
			},
			acceptAssertion: func(t *testing.T, _ int, conn net.Conn, err error) {
				require.NoError(t, err)
				require.NotNil(t, conn)
			},
			numConnAssertion: func(t *testing.T, num int64) {
				// MaxConnections == 0 prevents any connections from being accumulated
				require.Zero(t, num)
			},
		},
		{
			name:   "accept failure",
			config: Config{MaxConnections: 0},
			listener: &fakeListener{
				acceptError: failedAcceptErr,
			},
			acceptAssertion: func(t *testing.T, _ int, conn net.Conn, err error) {
				require.ErrorIs(t, err, failedAcceptErr)
				require.Nil(t, conn)
			},
			numConnAssertion: func(t *testing.T, num int64) {
				require.Zero(t, num)
			},
		},
		{
			name:   "invalid remote address",
			config: Config{MaxConnections: 0},
			listener: &fakeListener{
				acceptConn: &fakeConn{
					addr: &utils.NetAddr{
						Addr:        "abcd",
						AddrNetwork: "tcp",
					},
				},
			},
			acceptAssertion: func(t *testing.T, _ int, conn net.Conn, err error) {
				require.Error(t, err)
				require.Nil(t, conn)
			},
			numConnAssertion: func(t *testing.T, num int64) {
				require.Zero(t, num)
			},
		},
		{
			name:   "max connections exceeded",
			config: Config{MaxConnections: 2},
			listener: &fakeListener{
				acceptConn: &fakeConn{
					addr: mockAddr{},
				},
			},
			acceptAssertion: func(t *testing.T, i int, conn net.Conn, err error) {
				if i < 2 {
					require.NoError(t, err)
					require.NotNil(t, conn)
					return
				}
				require.Error(t, err)
				require.ErrorIs(t, err, tooManyConnectionsErr)
				require.True(t, trace.IsLimitExceeded(err))
				require.Nil(t, conn)
			},
			numConnAssertion: func(t *testing.T, num int64) {
				require.Equal(t, int64(2), num)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limiter := NewConnectionsLimiter(test.config.MaxConnections)

			ln, err := NewListener(test.listener, limiter)
			require.NoError(t, err)

			// open connections without closing to enforce limits
			conns := make([]net.Conn, 0, connLimit)
			for i := 0; i < connLimit; i++ {
				conn, err := ln.Accept()
				test.acceptAssertion(t, i, conn, err)

				if conn != nil {
					conns = append(conns, conn)
				}
			}

			// validate limits were enforced
			n, err := limiter.GetNumConnection("127.0.0.1")
			require.NoError(t, err)
			test.numConnAssertion(t, n)

			// close connections to reset limits
			for _, conn := range conns {
				require.NoError(t, conn.Close())
			}

			// ensure closing connections resets count
			n, err = limiter.GetNumConnection("127.0.0.1")
			if test.config.MaxConnections == 0 {
				require.NoError(t, err)
				require.Zero(t, n)
			} else {
				require.True(t, trace.IsBadParameter(err))
				require.Equal(t, int64(-1), n)
			}

			// open connections again after closing to
			// ensure that closing reset limits
			for i := 0; i < 5; i++ {
				conn, err := ln.Accept()
				test.acceptAssertion(t, i, conn, err)

				if conn != nil {
					t.Cleanup(func() {
						require.NoError(t, err)
					})
				}
			}
		})
	}
}

type fakeListener struct {
	net.Listener

	acceptConn  net.Conn
	acceptError error
}

func (f *fakeListener) Accept() (net.Conn, error) {
	return f.acceptConn, f.acceptError
}

type fakeConn struct {
	net.Conn

	addr net.Addr
}

func (f *fakeConn) RemoteAddr() net.Addr {
	return f.addr
}

func (f *fakeConn) Close() error {
	return nil
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
