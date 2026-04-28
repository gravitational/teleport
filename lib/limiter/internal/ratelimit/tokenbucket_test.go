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
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func mustRates(t *testing.T, specs ...rateSpec) *RateSet {
	t.Helper()
	rates := NewRateSet()
	for _, s := range specs {
		require.NoError(t, rates.Add(s.period, s.average, s.burst))
	}
	return rates
}

type rateSpec struct {
	period  time.Duration
	average int64
	burst   int64
}

func newTestTokenBucketSet(t *testing.T, rates *RateSet) (*TokenBucketSet, *clockwork.FakeClock) {
	t.Helper()
	clock := clockwork.NewFakeClock()
	return NewTokenBucketSet(rates, clock), clock
}

func requireConsumeOK(t *testing.T, set *TokenBucketSet, n int64) {
	t.Helper()
	delay, err := set.Consume(n)
	require.NoError(t, err)
	require.Zero(t, delay)
}

func requireConsumeDelayed(t *testing.T, set *TokenBucketSet, n int64, wantDelay time.Duration) {
	t.Helper()
	delay, err := set.Consume(n)
	require.NoError(t, err)
	require.Equal(t, wantDelay, delay)
}

func requireConsumeBurstOverflow(t *testing.T, set *TokenBucketSet, n int64) {
	t.Helper()
	delay, err := set.Consume(n)
	require.Error(t, err)
	require.Equal(t, time.Duration(UndefinedDelay), delay)
}

func TestTokenBucketSet_AllBurstThenRefill(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 5, burst: 10})
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 10)
	requireConsumeDelayed(t, set, 1, 200*time.Millisecond)
	clock.Advance(time.Second)
	requireConsumeOK(t, set, 5)
	requireConsumeDelayed(t, set, 1, 200*time.Millisecond)
}

func TestTokenBucketSet_RefillSubTimePerToken(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 10, burst: 10})
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 10)
	clock.Advance(40 * time.Millisecond)
	requireConsumeDelayed(t, set, 1, 60*time.Millisecond)
	clock.Advance(60 * time.Millisecond)
	requireConsumeOK(t, set, 1)
}

func TestTokenBucketSet_FractionalRefillCanPermitEarlierConsume(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 2, burst: 2})
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 2)
	clock.Advance(750 * time.Millisecond)
	requireConsumeOK(t, set, 1)
	clock.Advance(250 * time.Millisecond)
	requireConsumeOK(t, set, 1)
	requireConsumeDelayed(t, set, 1, 500*time.Millisecond)
}

func TestTokenBucketSet_BurstOverflowDoesNotConsume(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 10, burst: 10})
	set, _ := newTestTokenBucketSet(t, rates)

	requireConsumeBurstOverflow(t, set, 11)
	requireConsumeOK(t, set, 10)
}

func TestTokenBucketSet_BurstIncrease(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 10, burst: 10})
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 5)
	biggerRates := mustRates(t, rateSpec{period: time.Second, average: 10, burst: 20})
	set.Update(biggerRates)
	requireConsumeOK(t, set, 5)
	requireConsumeDelayed(t, set, 1, 100*time.Millisecond)
	clock.Advance(2 * time.Second)
	requireConsumeOK(t, set, 20)
}

func TestTokenBucketSet_BurstDecrease(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 100, burst: 100})
	set, clock := newTestTokenBucketSet(t, rates)

	smallerRates := mustRates(t, rateSpec{period: time.Second, average: 100, burst: 1})
	set.Update(smallerRates)
	requireConsumeBurstOverflow(t, set, 2)
	requireConsumeOK(t, set, 1)
	requireConsumeDelayed(t, set, 1, 10*time.Millisecond)
	clock.Advance(time.Second)
	requireConsumeBurstOverflow(t, set, 2)
	requireConsumeOK(t, set, 1)
}

func TestTokenBucketSet_UpdateAccruesElapsedTimeAtPreviousRate(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 1, burst: 10})
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 10)
	clock.Advance(time.Second)

	fasterRates := mustRates(t, rateSpec{period: time.Second, average: 10, burst: 10})
	set.Update(fasterRates)

	requireConsumeOK(t, set, 1)
	requireConsumeDelayed(t, set, 1, 100*time.Millisecond)
}

func TestTokenBucketSet_UpdateWithNewPeriod(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 10, burst: 10})
	set, _ := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 10)
	newPeriodRates := mustRates(t, rateSpec{period: time.Minute, average: 5, burst: 5})
	set.Update(newPeriodRates)
	requireConsumeOK(t, set, 5)
	requireConsumeDelayed(t, set, 1, 12*time.Second)
}

func TestTokenBucketSet_EmptySet(t *testing.T) {
	t.Parallel()
	set, _ := newTestTokenBucketSet(t, NewRateSet())
	delay, err := set.Consume(5)
	require.NoError(t, err)
	require.LessOrEqual(t, int64(delay), int64(0))
	require.False(t, set.IsRateLimited())
}

func TestTokenBucketSet_MultiPeriodSet(t *testing.T) {
	t.Parallel()
	rates := mustRates(t,
		rateSpec{period: 10 * time.Millisecond, average: 10, burst: 20}, // B1, 10 tokens / 10ms.
		rateSpec{period: 40 * time.Millisecond, average: 10, burst: 40}, // B2, 2.5 tokens / 10ms.
	)
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 20) // B1: 0, B2: 20.
	requireConsumeDelayed(t, set, 1, time.Millisecond)

	clock.Advance(10 * time.Millisecond) // B1: 10, B2: 22.5.
	requireConsumeOK(t, set, 10)         // B1: 0, B2: 12.5.
	requireConsumeDelayed(t, set, 1, time.Millisecond)

	clock.Advance(10 * time.Millisecond) // B1: 10, B2: 15.
	requireConsumeOK(t, set, 10)         // B1: 0, B2: 5.

	clock.Advance(10 * time.Millisecond) // B1: 10, B2: 7.5.
	requireConsumeOK(t, set, 7)          // B1: 3, B2: 0.5.
	requireConsumeDelayed(t, set, 1, 2*time.Millisecond)
}

func TestTokenBucketSet_UpdateNoOp(t *testing.T) {
	t.Parallel()
	rates := mustRates(t, rateSpec{period: time.Second, average: 100, burst: 100})
	set, clock := newTestTokenBucketSet(t, rates)

	requireConsumeOK(t, set, 99)

	for range 3 {
		clock.Advance(10 * time.Millisecond)
		set.Update(rates)
		set.Update(rates)
	}

	requireConsumeOK(t, set, 4)
	requireConsumeDelayed(t, set, 1, 10*time.Millisecond)
}

// These configs exercise token intervals that do not divide cleanly into
// common wall-clock durations, while still refilling to full burst after one
// configured period.
func TestTokenBucketSet_FractionalRefillConfigs(t *testing.T) {
	t.Parallel()
	configs := []rateSpec{
		{period: 10 * time.Millisecond, average: 3, burst: 3},
		{period: 7 * time.Millisecond, average: 11, burst: 11},
		{period: time.Millisecond, average: 1, burst: 1},
	}
	for _, spec := range configs {
		t.Run(spec.period.String()+"-"+strconv.FormatInt(spec.average, 10), func(t *testing.T) {
			t.Parallel()
			rates := mustRates(t, spec)
			set, clock := newTestTokenBucketSet(t, rates)

			requireConsumeOK(t, set, spec.burst)
			requireConsumeDelayed(t, set, 1, spec.period/time.Duration(spec.average))
			clock.Advance(spec.period)
			requireConsumeOK(t, set, spec.burst)
			requireConsumeDelayed(t, set, 1, spec.period/time.Duration(spec.average))
		})
	}
}

func TestTokenBucketSet_MultiPeriodBurstOverflowDoesNotConsume(t *testing.T) {
	t.Parallel()
	rates := mustRates(t,
		rateSpec{period: time.Millisecond, average: 1, burst: 1},
		rateSpec{period: time.Second, average: 10000, burst: 10000},
	)
	set, _ := newTestTokenBucketSet(t, rates)

	requireConsumeBurstOverflow(t, set, 2)
	requireConsumeOK(t, set, 1)
	requireConsumeDelayed(t, set, 1, time.Millisecond)
}

func TestRateSet_Add_RejectsBurstOverInt32(t *testing.T) {
	t.Parallel()
	rs := NewRateSet()

	require.NoError(t, rs.Add(time.Second, 1, math.MaxInt32))
	require.Error(t, rs.Add(time.Second, 1, math.MaxInt32+1))
	require.Error(t, rs.Add(time.Second, 1, math.MaxInt64))
}
