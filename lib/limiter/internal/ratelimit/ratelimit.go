/*
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

package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// TokenLimiter implements rate limiting middleware.
type TokenLimiter struct {
	clock clockwork.Clock
	log   *slog.Logger

	defaultRates *RateSet

	mutex      sync.Mutex
	bucketSets *utils.FnCache
	next       http.Handler
}

type TokenLimiterConfig struct {
	Rates *RateSet
	Clock clockwork.Clock
}

func (t *TokenLimiterConfig) CheckAndSetDefaults() error {
	if len(t.Rates.m) == 0 {
		return trace.BadParameter("missing required rates")
	}

	if t.Clock == nil {
		t.Clock = clockwork.NewRealClock()
	}

	return nil
}

// New constructs a rate limiter.
func New(config TokenLimiterConfig) (*TokenLimiter, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	tl := &TokenLimiter{
		defaultRates: config.Rates,
		clock:        config.Clock,

		log: slog.With(teleport.ComponentKey, "ratelimiter"),
	}

	bucketSets, err := utils.NewFnCache(utils.FnCacheConfig{
		// The default TTL here is not super important because we set the
		// TTL explicitly for each entry we insert.
		TTL:   10 * time.Second,
		Clock: config.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tl.bucketSets = bucketSets
	return tl, nil
}

func (tl *TokenLimiter) Wrap(next http.Handler) {
	tl.next = next
}

func (tl *TokenLimiter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	clientIP, err := ExtractClientIP(req)
	if err != nil {
		ServeHTTPError(w, req, err)
		return
	}

	if err := tl.consumeRates(clientIP, 1 /* amount */); err != nil {
		tl.log.InfoContext(context.Background(), "limiting request",
			"method", req.Method, "url", req.URL.String(), "error", err)
		ServeHTTPError(w, req, err)
		return
	}

	tl.next.ServeHTTP(w, req)
}

func (tl *TokenLimiter) consumeRates(source string, amount int64) error {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	// We set the TTL as 10 times the rate period. E.g. if rate is 100 requests/second
	// per client IP, the counters for this IP will expire after 10 seconds of inactivity.
	ttl := tl.defaultRates.MaxPeriod()*10 + 1
	bucketSet, err := utils.FnCacheGetWithTTL(context.TODO(), tl.bucketSets, source, ttl,
		func(ctx context.Context) (*TokenBucketSet, error) {
			return NewTokenBucketSet(tl.defaultRates, tl.clock), nil
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	bucketSet.Update(tl.defaultRates)

	delay, err := bucketSet.Consume(amount)
	if err != nil {
		return err
	}
	if delay > 0 {
		return &limitExceededError{delay: delay}
	}
	return nil
}

// RateSet maintains a set of rates. It can contain only one rate per period at a time.
type RateSet struct {
	m map[time.Duration]*rate
}

// rate defines token bucket parameters.
type rate struct {
	period  time.Duration
	average int64
	burst   int64
}

// NewRateSet crates an empty rate set.
func NewRateSet() *RateSet {
	return &RateSet{m: make(map[time.Duration]*rate)}
}

// Add adds a rate to the set. If there is a rate with the same period in the
// set then the new rate overrides the old one.
func (rs *RateSet) Add(period time.Duration, average int64, burst int64) error {
	if period <= 0 {
		return trace.BadParameter("Invalid period: %v", period)
	}
	if average <= 0 {
		return trace.BadParameter("Invalid average: %v", average)
	}
	if burst <= 0 {
		return trace.BadParameter("Invalid burst: %v", burst)
	}
	rs.m[period] = &rate{period, average, burst}
	return nil
}

// MaxPeriod returns the maximum period in the rate set.
func (rs *RateSet) MaxPeriod() time.Duration {
	var result time.Duration
	for _, rate := range rs.m {
		result = max(result, rate.period)
	}
	return result
}

type limitExceededError struct {
	delay time.Duration
}

func (l *limitExceededError) Error() string {
	return fmt.Sprintf("rate limit exceeded, try again in %v", l.delay)
}

func (l *limitExceededError) As(e any) bool {
	switch err := e.(type) {
	case **trace.LimitExceededError:
		*err = &trace.LimitExceededError{
			Message: l.Error(),
		}
		return true
	default:
		return false
	}
}
