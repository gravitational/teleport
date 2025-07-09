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
	"cmp"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// TokenBucketSet represents a set of buckets covering different time periods.
type TokenBucketSet struct {
	buckets   map[time.Duration]*tokenBucket
	maxPeriod time.Duration
	clock     clockwork.Clock
}

// NewTokenBucketSet creates a bucket set from the specified rates.
func NewTokenBucketSet(rates *RateSet, clock clockwork.Clock) *TokenBucketSet {
	tbs := &TokenBucketSet{
		// In the majority of cases we will have only one bucket.
		buckets: make(map[time.Duration]*tokenBucket, len(rates.m)),
		clock:   clock,
	}

	for _, rate := range rates.m {
		newBucket := newTokenBucket(rate, clock)
		tbs.buckets[rate.period] = newBucket
		tbs.maxPeriod = max(tbs.maxPeriod, rate.period)
	}
	return tbs
}

// Update brings the buckets in the set in accordance with the provided rates.
func (tbs *TokenBucketSet) Update(rates *RateSet) {
	// Update existing buckets and delete those that have no corresponding spec.
	for _, bucket := range tbs.buckets {
		if rate, ok := rates.m[bucket.period]; ok {
			bucket.update(rate)
		} else {
			delete(tbs.buckets, bucket.period)
		}
	}
	// Add missing buckets.
	for _, rate := range rates.m {
		if _, ok := tbs.buckets[rate.period]; !ok {
			newBucket := newTokenBucket(rate, tbs.clock)
			tbs.buckets[rate.period] = newBucket
		}
	}
	// Identify the maximum period in the set
	tbs.maxPeriod = 0
	for _, bucket := range tbs.buckets {
		tbs.maxPeriod = max(tbs.maxPeriod, bucket.period)
	}
}

// Consume makes an attempt to consume the specified number of tokens from the
// bucket. If there are enough tokens available then (0, nil) is returned.
// If tokens to consume is larger than the burst size, then an error is returned
// along with [UndefinedDelay]; otherwise return a non-zero delay that indicates
// how much time the caller needs to wait until the desired number of tokens
// will become available for consumption.
func (tbs *TokenBucketSet) Consume(tokens int64) (time.Duration, error) {
	var maxDelay time.Duration = UndefinedDelay
	var firstErr error = nil
	for _, tokenBucket := range tbs.buckets {
		// We keep calling Consume even after a error is returned for one of
		// buckets because that allows us to simplify the rollback procedure,
		// that is to just call Rollback for all buckets.
		delay, err := tokenBucket.consume(tokens)
		if firstErr == nil {
			if err != nil {
				firstErr = err
			} else {
				maxDelay = max(maxDelay, delay)
			}
		}
	}
	// If we could not make ALL buckets consume tokens for whatever reason,
	// then rollback consumption for all of them.
	if firstErr != nil || maxDelay > 0 {
		for _, tokenBucket := range tbs.buckets {
			tokenBucket.rollback()
		}
	}
	return maxDelay, firstErr
}

func (tbs *TokenBucketSet) GetMaxPeriod() time.Duration {
	return tbs.maxPeriod
}

// tokenBucket implements the token bucket algorithm.
type tokenBucket struct {
	// The time period controlled by the bucket.
	period time.Duration
	// The amount of time that it takes to add one more token to the total
	// number of available tokens. It effectively caches the value that could
	// have been otherwise deduced from refillRate.
	timePerToken time.Duration
	// The maximum number of tokens that can be accumulate in the bucket.
	burst int64
	// The number of tokens available for consumption at the moment. It can
	// never be larger than burst.
	availableTokens int64

	clock        clockwork.Clock
	lastRefresh  time.Time
	lastConsumed int64
}

// newTokenBucket crates a tokenBucket instance for the specified rate.
func newTokenBucket(rate *rate, clock clockwork.Clock) *tokenBucket {
	period := cmp.Or(rate.period, time.Nanosecond)
	return &tokenBucket{
		period:          period,
		timePerToken:    time.Duration(int64(period) / rate.average),
		burst:           rate.burst,
		clock:           clock,
		lastRefresh:     clock.Now().UTC(),
		availableTokens: rate.burst,
	}
}

const UndefinedDelay = -1

func (tb *tokenBucket) consume(tokens int64) (time.Duration, error) {
	tb.updateAvailableTokens()
	tb.lastConsumed = 0
	if tokens > tb.burst {
		return UndefinedDelay, fmt.Errorf("requested tokens larger than max tokens")
	}
	if tb.availableTokens < tokens {
		return tb.timeUntilAvailable(tokens), nil
	}
	tb.availableTokens -= tokens
	tb.lastConsumed = tokens
	return 0, nil
}

// rollback reverts effect of the most recent consumption. If the most recent
// consume resulted in an error or a burst overflow, and therefore did not
// modify the number of available tokens, then rollback won't do that either.
// It is safe to call this method multiple times, for the second and all
// following calls have no effect.
func (tb *tokenBucket) rollback() {
	tb.availableTokens += tb.lastConsumed
	tb.lastConsumed = 0
}

// Update modifies average and burst fields of the token bucket according
// to the provided rate.
func (tb *tokenBucket) update(rate *rate) error {
	if rate.period != tb.period {
		return trace.BadParameter("Period mismatch: %v != %v", tb.period, rate.period)
	}
	tb.timePerToken = time.Duration(int64(tb.period) / rate.average)
	tb.burst = rate.burst
	if tb.availableTokens > rate.burst {
		tb.availableTokens = rate.burst
	}
	return nil
}

// timeUntilAvailable returns the amount of time that we need to
// wait until the specified number of tokens becomes available for consumption.
func (tb *tokenBucket) timeUntilAvailable(tokens int64) time.Duration {
	missingTokens := tokens - tb.availableTokens
	return time.Duration(missingTokens) * tb.timePerToken
}

// updateAvailableTokens updates the number of tokens available for consumption.
// It is calculated based on the refill rate, the time passed since last refresh,
// and is limited by the bucket capacity.
func (tb *tokenBucket) updateAvailableTokens() {
	if tb.timePerToken == 0 {
		return
	}

	now := tb.clock.Now().UTC()
	timePassed := now.Sub(tb.lastRefresh)

	tokens := tb.availableTokens + int64(timePassed/tb.timePerToken)

	// If we haven't added any tokens that means that not enough time has passed,
	// in this case do not adjust last refill checkpoint, otherwise it will be
	// always moving in time in case of frequent requests that exceed the rate
	if tokens != tb.availableTokens {
		tb.lastRefresh = now
		tb.availableTokens = tokens
	}
	if tb.availableTokens > tb.burst {
		tb.availableTokens = tb.burst
	}
}
