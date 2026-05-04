/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package ratelimit

import (
	"errors"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	xrate "golang.org/x/time/rate"
)

// bucket is a rate.Limiter-backed token bucket.
type bucket struct {
	burst   int64
	clock   clockwork.Clock
	limiter *xrate.Limiter
}

// newBucket builds a bucket that matches the configured rate.
func newBucket(r *rate, clock clockwork.Clock) *bucket {
	return &bucket{
		burst:   r.burst,
		clock:   clock,
		limiter: xrate.NewLimiter(limitFromRate(r), int(r.burst)),
	}
}

// limitFromRate maps Teleport's (period, average) pair to
// rate.Limit (events/second).
func limitFromRate(r *rate) xrate.Limit {
	return xrate.Limit(float64(r.average) / r.period.Seconds())
}

// peek checks n tokens at now without consuming them. Multi-bucket
// sets use it to avoid consuming from any bucket until all buckets can
// consume. Returns:
//   - (UndefinedDelay, error) when n exceeds the bucket's burst
//   - (delay, nil) when n tokens are not yet available
//   - (0, nil) when n tokens are available
func (b *bucket) peek(n int64, now time.Time) (time.Duration, error) {
	if n > b.burst {
		return UndefinedDelay, trace.LimitExceeded("requested tokens larger than max tokens")
	}
	return b.durationUntilAvailable(n, now)
}

// consume consumes n tokens if they are available.
func (b *bucket) consume(n int64, now time.Time) (time.Duration, error) {
	if n > b.burst {
		return UndefinedDelay, trace.LimitExceeded("requested tokens larger than max tokens")
	}
	if b.limiter.AllowN(now, int(n)) {
		return 0, nil
	}
	return b.durationUntilAvailable(n, now)
}

// update changes the bucket's rate.
func (b *bucket) update(r *rate) {
	now := b.clock.Now()
	b.limiter.SetLimitAt(now, limitFromRate(r))
	b.limiter.SetBurstAt(now, int(r.burst))
	b.burst = r.burst
}

// isLimited reports whether the bucket has no tokens.
func (b *bucket) isLimited() bool {
	return b.limiter.TokensAt(b.clock.Now()) < 1.0
}

// durationUntilAvailable returns the delay before n tokens can be consumed.
func (b *bucket) durationUntilAvailable(n int64, now time.Time) (time.Duration, error) {
	available := b.limiter.TokensAt(now)
	if available >= float64(n) {
		return 0, nil
	}
	limit := b.limiter.Limit()
	if limit <= 0 {
		// RateSet.Add rejects average <= 0, so this is defensive.
		return UndefinedDelay, errors.New("rate limit is non-positive")
	}
	delay := time.Duration((float64(n) - available) / float64(limit) * float64(time.Second))
	return max(delay, time.Nanosecond), nil
}
