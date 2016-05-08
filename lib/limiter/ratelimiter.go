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
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	"github.com/mailgun/ttlmap"
	"github.com/vulcand/oxy/ratelimit"
	"github.com/vulcand/oxy/utils"
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
func NewRateLimiter(config LimiterConfig) (*RateLimiter, error) {
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
// Returns error if there are too many requests with the provided token
func (l *RateLimiter) RegisterRequest(token string) error {
	l.Lock()
	defer l.Unlock()

	bucketSetI, exists := l.rateLimits.Get(token)
	var bucketSet *ratelimit.TokenBucketSet

	if exists {
		bucketSet = bucketSetI.(*ratelimit.TokenBucketSet)
		bucketSet.Update(l.rates)
	} else {
		bucketSet = ratelimit.NewTokenBucketSet(l.rates, l.clock)
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
