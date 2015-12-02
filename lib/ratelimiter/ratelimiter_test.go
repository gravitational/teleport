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
package ratelimiter

import (
	"testing"
	"time"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestRateLimiter(t *testing.T) { TestingT(t) }

type LimiterSuite struct {
}

var _ = Suite(&LimiterSuite{})

func (s *LimiterSuite) SetUpSuite(c *C) {
}

func (s *LimiterSuite) SetUpTest(c *C) {
}

func (s *LimiterSuite) TearDownTest(c *C) {
}

func (s *LimiterSuite) TestConnectionsLimiter(c *C) {
	limiter, err := NewRateLimiter(
		RateLimiterConfig{
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

	limiter, err = NewRateLimiter(
		RateLimiterConfig{
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

func (s *LimiterSuite) TestRates(c *C) {
	limiter, err := NewRateLimiter(
		RateLimiterConfig{
			Rates: []Rate{
				Rate{
					Period:  10 * time.Millisecond,
					Average: 10,
					Burst:   20,
				},
				Rate{
					Period:  40 * time.Millisecond,
					Average: 10,
					Burst:   40,
				},
			},
		},
	)
	c.Assert(err, IsNil)

	// When
	c.Assert(limiter.Consume("token1", 15), IsNil)
	c.Assert(limiter.Consume("token2", 20), IsNil)

	for i := 0; i < 5; i++ {
		c.Assert(limiter.Consume("token1", 1), IsNil)
	}
	c.Assert(limiter.Consume("token1", 1), NotNil)

	time.Sleep(10010 * time.Microsecond)
	for i := 0; i < 10; i++ {
		c.Assert(limiter.Consume("token1", 1), IsNil)
	}
	c.Assert(limiter.Consume("token1", 1), NotNil)

	time.Sleep(10010 * time.Microsecond)
	for i := 0; i < 10; i++ {
		c.Assert(limiter.Consume("token1", 1), IsNil)
	}
	c.Assert(limiter.Consume("token1", 1), NotNil)

	time.Sleep(10010 * time.Microsecond)
	// the second rate is full
	c.Assert(limiter.Consume("token1", 10), NotNil)

	time.Sleep(10010 * time.Microsecond)
	// Now the second rate have free space
	c.Assert(limiter.Consume("token1", 1), IsNil)
	c.Assert(limiter.Consume("token1", 10), NotNil)
}
