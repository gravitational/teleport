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
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/oxy/utils"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	"github.com/mailgun/ttlmap"
)

// RateLimiter controls connection rate, it uses token bucket algo
// https://en.wikipedia.org/wiki/Token_bucket
type RateLimiter struct {
	*ratelimit.TokenLimiter
	rateLimits *ttlmap.TtlMap
	*sync.Mutex
	rates *ratelimit.RateSet
	clock timetools.TimeProvider
}

// Rate defines connection rate
type Rate struct {
	Period  time.Duration
	Average int64
	Burst   int64
}

// NewRateLimiter returns new request rate controller
func NewRateLimiter(config Config) (*RateLimiter, error) {
	limiter := RateLimiter{
		Mutex: &sync.Mutex{},
	}

	ipExtractor, err := utils.NewExtractor("client.ip")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	limiter.rates = ratelimit.NewRateSet()
	for _, rate := range config.Rates {
		err := limiter.rates.Add(rate.Period, rate.Average, rate.Burst)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if len(config.Rates) == 0 {
		err := limiter.rates.Add(time.Second, DefaultRate, DefaultRate)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if config.Clock == nil {
		config.Clock = &timetools.RealTime{}
	}
	limiter.clock = config.Clock

	limiter.TokenLimiter, err = ratelimit.New(nil, ipExtractor,
		limiter.rates, ratelimit.Clock(config.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	maxNumberOfUsers := config.MaxNumberOfUsers
	if maxNumberOfUsers <= 0 {
		maxNumberOfUsers = DefaultMaxNumberOfUsers
	}
	limiter.rateLimits, err = ttlmap.NewMap(
		maxNumberOfUsers, ttlmap.Clock(config.Clock))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &limiter, nil
}

// RegisterRequest increases number of requests for the provided token
// Returns error if there are too many requests with the provided token.
func (l *RateLimiter) RegisterRequest(token string, customRate *ratelimit.RateSet) error {
	l.Lock()
	defer l.Unlock()

	rate := customRate
	if rate == nil {
		// Set rate to default.
		rate = l.rates
	}

	bucketSetI, exists := l.rateLimits.Get(token)
	var bucketSet *ratelimit.TokenBucketSet

	if exists {
		bucketSet = bucketSetI.(*ratelimit.TokenBucketSet)
		bucketSet.Update(rate)
	} else {
		bucketSet = ratelimit.NewTokenBucketSet(rate, l.clock)
		// We set ttl as 10 times rate period. E.g. if rate is 100 requests/second per client ip
		// the counters for this ip will expire after 10 seconds of inactivity
		err := l.rateLimits.Set(token, bucketSet, int(bucketSet.GetMaxPeriod()/time.Second)*10+1)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	delay, err := bucketSet.Consume(1)
	if err != nil {
		return err
	}
	if delay > 0 {
		return &ratelimit.MaxRateError{}
	}
	return nil
}

// Add rate limiter to the handle
func (l *RateLimiter) WrapHandle(h http.Handler) {
	l.TokenLimiter.Wrap(h)
}

func (r *Rate) UnmarshalJSON(value []byte) error {
	type rate struct {
		Period  string
		Average int64
		Burst   int64
	}

	var x rate
	err := json.Unmarshal(value, &x)
	if err != nil {
		return trace.Wrap(err)
	}

	period, err := time.ParseDuration(x.Period)
	if err != nil {
		return trace.Wrap(err)
	}

	*r = Rate{
		Period:  period,
		Average: x.Average,
		Burst:   x.Burst,
	}
	return nil
}

const (
	DefaultMaxNumberOfUsers = 100000
	DefaultRate             = 100000000
)
