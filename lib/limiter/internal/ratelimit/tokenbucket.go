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
	"time"

	"github.com/jonboulle/clockwork"
)

// UndefinedDelay is the sentinel returned from Consume when the
// requested token count exceeds the bucket's burst.
const UndefinedDelay = -1

// TokenBucketSet represents a set of buckets covering different time periods.
type TokenBucketSet struct {
	buckets   map[time.Duration]*bucket
	maxPeriod time.Duration
	clock     clockwork.Clock
}

// NewTokenBucketSet creates a bucket set from the specified rates.
func NewTokenBucketSet(rates *RateSet, clock clockwork.Clock) *TokenBucketSet {
	tbs := &TokenBucketSet{
		// In the majority of cases we will have only one bucket.
		buckets: make(map[time.Duration]*bucket, len(rates.m)),
		clock:   clock,
	}

	for _, rate := range rates.m {
		tbs.buckets[rate.period] = newBucket(rate, clock)
		tbs.maxPeriod = max(tbs.maxPeriod, rate.period)
	}
	return tbs
}

// Update brings the buckets in the set in accordance with the provided rates.
func (tbs *TokenBucketSet) Update(rates *RateSet) {
	// Update existing buckets and delete those that have no corresponding spec.
	for period, bucket := range tbs.buckets {
		if rate, ok := rates.m[period]; ok {
			bucket.update(rate)
		} else {
			delete(tbs.buckets, period)
		}
	}
	// Add missing buckets.
	for _, rate := range rates.m {
		if _, ok := tbs.buckets[rate.period]; !ok {
			tbs.buckets[rate.period] = newBucket(rate, tbs.clock)
		}
	}
	// Identify the maximum period in the set.
	tbs.maxPeriod = 0
	for period := range tbs.buckets {
		tbs.maxPeriod = max(tbs.maxPeriod, period)
	}
}

// IsRateLimited checks if the bucket is currently rate-limited (no tokens available)
// without actually consuming any tokens. Returns true if rate-limited, false otherwise.
func (tbs *TokenBucketSet) IsRateLimited() bool {
	for _, bucket := range tbs.buckets {
		if bucket.isLimited() {
			return true
		}
	}
	return false
}

// Consume makes an attempt to consume the specified number of tokens from the
// bucket. If there are enough tokens available then (0, nil) is returned.
// If tokens to consume is larger than the burst size, then an error is returned
// along with [UndefinedDelay]; otherwise return a non-zero delay that indicates
// how much time the caller needs to wait until the desired number of tokens
// will become available for consumption.
func (tbs *TokenBucketSet) Consume(tokens int64) (time.Duration, error) {
	now := tbs.clock.Now().UTC()
	if len(tbs.buckets) == 1 {
		// Fast path. Range over a single-element map costs the same
		// as a struct field access in practice.
		for _, bucket := range tbs.buckets {
			return bucket.consume(tokens, now)
		}
	}
	// Multi-bucket path: peek every bucket at the same `now`. Only
	// consume if all peeks succeeded.
	var maxDelay time.Duration = UndefinedDelay
	var firstErr error
	for _, bucket := range tbs.buckets {
		delay, err := bucket.peek(tokens, now)
		firstErr = cmp.Or(firstErr, err)
		maxDelay = max(maxDelay, delay)
	}
	if firstErr != nil {
		return UndefinedDelay, firstErr
	}
	if maxDelay > 0 {
		return maxDelay, nil
	}
	for _, bucket := range tbs.buckets {
		delay, err := bucket.consume(tokens, now)
		if delay > 0 || err != nil {
			return delay, err
		}
	}
	return 0, nil
}

// GetMaxPeriod returns the longest period across all buckets in the
// set, or zero if the set is empty.
func (tbs *TokenBucketSet) GetMaxPeriod() time.Duration {
	return tbs.maxPeriod
}
