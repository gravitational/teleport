/*
Copyright 2015 Gravitational, Inc.

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
package limiter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mailgun/timetools"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestLimiter(t *testing.T) { TestingT(t) }

type LimiterSuite struct {
}

var _ = Suite(&LimiterSuite{})

func (s *LimiterSuite) TestConnectionsLimiter(c *C) {
	limiter, err := NewLimiter(
		Config{
			MaxConnections: 0,
		},
	)
	c.Assert(err, IsNil)

	for i := 0; i < 10; i++ {
		c.Assert(limiter.AcquireConnection("token1"), IsNil)
	}
	for i := 0; i < 5; i++ {
		c.Assert(limiter.AcquireConnection("token2"), IsNil)
	}

	for i := 0; i < 10; i++ {
		limiter.ReleaseConnection("token1")
	}
	for i := 0; i < 5; i++ {
		limiter.ReleaseConnection("token2")
	}

	limiter, err = NewLimiter(
		Config{
			MaxConnections: 5,
		},
	)
	c.Assert(err, IsNil)

	for i := 0; i < 5; i++ {
		c.Assert(limiter.AcquireConnection("token1"), IsNil)
	}

	for i := 0; i < 5; i++ {
		c.Assert(limiter.AcquireConnection("token2"), IsNil)
	}
	for i := 0; i < 5; i++ {
		c.Assert(limiter.AcquireConnection("token2"), NotNil)
	}

	for i := 0; i < 10; i++ {
		limiter.ReleaseConnection("token1")
		c.Assert(limiter.AcquireConnection("token1"), IsNil)
	}

	for i := 0; i < 5; i++ {
		limiter.ReleaseConnection("token2")
	}
	for i := 0; i < 5; i++ {
		c.Assert(limiter.AcquireConnection("token2"), IsNil)
	}
}

func (s *LimiterSuite) TestRateLimiter(c *C) {
	// TODO: this test fails
	clock := &timetools.FreezedTime{
		CurrentTime: time.Date(2016, 6, 5, 4, 3, 2, 1, time.UTC),
	}

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
	c.Assert(err, IsNil)

	for i := 0; i < 20; i++ {
		c.Assert(limiter.RegisterRequest("token1"), IsNil)
	}
	for i := 0; i < 20; i++ {
		c.Assert(limiter.RegisterRequest("token2"), IsNil)
	}

	c.Assert(limiter.RegisterRequest("token1"), NotNil)

	clock.Sleep(10 * time.Millisecond)
	for i := 0; i < 10; i++ {
		c.Assert(limiter.RegisterRequest("token1"), IsNil)
	}
	c.Assert(limiter.RegisterRequest("token1"), NotNil)

	clock.Sleep(10 * time.Millisecond)
	for i := 0; i < 10; i++ {
		c.Assert(limiter.RegisterRequest("token1"), IsNil)
	}
	c.Assert(limiter.RegisterRequest("token1"), NotNil)

	clock.Sleep(10 * time.Millisecond)
	// the second rate is full
	err = nil
	for i := 0; i < 10; i++ {
		err = limiter.RegisterRequest("token1")
		if err != nil {
			break
		}
	}
	c.Assert(err, NotNil)

	clock.Sleep(10 * time.Millisecond)
	// Now the second rate has free space
	c.Assert(limiter.RegisterRequest("token1"), IsNil)
	err = nil
	for i := 0; i < 15; i++ {
		err = limiter.RegisterRequest("token1")
		if err != nil {
			break
		}
	}
	c.Assert(err, NotNil)
}

func (s *LimiterSuite) TestCustomRate(c *C) {
	clock := &timetools.FreezedTime{
		CurrentTime: time.Date(2016, 6, 5, 4, 3, 2, 1, time.UTC),
	}

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
	c.Assert(err, IsNil)

	customRate := ratelimit.NewRateSet()
	err = customRate.Add(time.Minute, 1, 5)
	c.Assert(err, IsNil)

	// Max out custom rate.
	for i := 0; i < 5; i++ {
		c.Assert(limiter.RegisterRequestWithCustomRate("token1", customRate), IsNil)
	}

	// Test rate limit exceeded with custom rate.
	c.Assert(limiter.RegisterRequestWithCustomRate("token1", customRate), NotNil)

	// Test default rate still works.
	for i := 0; i < 20; i++ {
		c.Assert(limiter.RegisterRequest("token1"), IsNil)
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
