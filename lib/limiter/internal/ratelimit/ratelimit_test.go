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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestLimitExceededError(t *testing.T) {
	t.Parallel()
	err := &limitExceededError{delay: 3 * time.Second}

	require.True(t, trace.IsLimitExceeded(err))

	var le *trace.LimitExceededError
	require.ErrorAs(t, err, &le)
	require.Equal(t, err.Error(), le.Message)
}

func TestTokenBucketSet_IsRateLimited(t *testing.T) {
	t.Parallel()

	t.Run("not rate limited when tokens available", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		rates := NewRateSet()
		require.NoError(t, rates.Add(time.Minute, 10, 10))

		tbs := NewTokenBucketSet(rates, clock)
		require.False(t, tbs.IsRateLimited(), "should not be rate limited with full bucket")
	})

	t.Run("rate limited when no tokens available", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		rates := NewRateSet()
		require.NoError(t, rates.Add(time.Minute, 10, 10))

		tbs := NewTokenBucketSet(rates, clock)

		// Consume all tokens
		delay, err := tbs.Consume(10)
		require.NoError(t, err)
		require.Zero(t, delay)

		// Now should be rate limited
		require.True(t, tbs.IsRateLimited(), "should be rate limited after consuming all tokens")
	})

	t.Run("not rate limited after time passes", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		rates := NewRateSet()
		require.NoError(t, rates.Add(time.Minute, 10, 10))

		tbs := NewTokenBucketSet(rates, clock)

		// Consume all tokens
		delay, err := tbs.Consume(10)
		require.NoError(t, err)
		require.Zero(t, delay)

		require.True(t, tbs.IsRateLimited(), "should be rate limited after consuming all tokens")

		// Advance time to refill tokens
		clock.Advance(time.Minute)

		require.False(t, tbs.IsRateLimited())
	})

	t.Run("rate limited when any bucket is exhausted", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		rates := NewRateSet()
		require.NoError(t, rates.Add(time.Minute, 10, 10))
		require.NoError(t, rates.Add(time.Hour, 100, 100))

		tbs := NewTokenBucketSet(rates, clock)

		// Consume all tokens from the minute bucket
		delay, err := tbs.Consume(10)
		require.NoError(t, err)
		require.Zero(t, delay)

		// Should be rate limited even though hour bucket has tokens
		require.True(t, tbs.IsRateLimited())
	})

	t.Run("does not consume tokens", func(t *testing.T) {
		clock := clockwork.NewFakeClock()
		rates := NewRateSet()
		require.NoError(t, rates.Add(time.Minute, 10, 10))

		tbs := NewTokenBucketSet(rates, clock)

		// Check multiple times - should not consume tokens
		require.False(t, tbs.IsRateLimited())
		require.False(t, tbs.IsRateLimited())
		require.False(t, tbs.IsRateLimited())

		// Should still be able to consume all tokens
		delay, err := tbs.Consume(10)
		require.NoError(t, err)
		require.Zero(t, delay)

		// Now should be rate limited
		require.True(t, tbs.IsRateLimited())
	})
}
