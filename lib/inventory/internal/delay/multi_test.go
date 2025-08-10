// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package delay

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/interval"
)

func TestMultiBasics(t *testing.T) {
	const interval = time.Millisecond * 20
	t.Parallel()

	multi := NewMulti[int](MultiParams{
		FixedInterval: interval,
	})

	// verify that delay is in an initial state that will never fire
	require.Nil(t, multi.Elapsed())

	for i := 1; i <= 10; i++ {
		// add a subinterval
		multi.Add(i)
	}

	for i := range 30 {
		now := <-multi.Elapsed()
		require.Equal(t, i%10+1, multi.Tick(now))
	}

	// remove some subintervals
	for i := 1; i <= 8; i++ {
		multi.Remove(i)
	}

	// verify that remaining subintervals are still being serviced
	for i := range 30 {
		k := 10
		if i%2 == 0 {
			k = 9
		}
		now := <-multi.Elapsed()
		require.Equal(t, k, multi.Tick(now))
	}

	multi.Remove(9)
	multi.Remove(10)

	// verify complete removal of all sub-intervals
	select {
	case <-multi.Elapsed():
		t.Fatal("expected no more ticks")
	case <-time.After(interval * 3):
	}

	// verify that the multi is still usable after having been
	// fully drained.
	multi.Add(777)
	select {
	case now := <-multi.Elapsed():
		require.Equal(t, 777, multi.Tick(now))
	case <-time.After(interval * 3):
		t.Fatal("timeout waiting for re-added delay to fire")
	}
}

func TestMultiJitter(t *testing.T) {
	t.Parallel()

	var jitterCalled atomic.Bool
	fakeJitter := func(d time.Duration) time.Duration {
		jitterCalled.Store(true)
		return time.Millisecond * 20
	}

	multi := NewMulti[int](MultiParams{
		FixedInterval: time.Hour,
		Jitter:        fakeJitter,
	})

	for i := range 10 {
		multi.Add(i + 1)
	}

	for range 10 {
		select {
		case now := <-multi.Elapsed():
			multi.Tick(now)
		case <-time.After(time.Second * 10):
			t.Fatal("timeout waiting for delay to fire")
		}
		require.True(t, jitterCalled.Swap(false))
	}
}

func TestMultiVariable(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()
	start := clock.Now()

	ivl := interval.NewVariableDuration(interval.VariableDurationConfig{
		MinDuration: 2 * time.Minute,
		MaxDuration: 4 * time.Minute,
		Step:        1,
	})

	// deterministic jitter, always half the actual time
	multi := NewMulti[int](MultiParams{
		VariableInterval: ivl,
		Jitter: func(d time.Duration) time.Duration {
			return d / 2
		},
		// deterministic reset jitter, always 8/7 of the time
		ResetJitter: func(d time.Duration) time.Duration {
			return d + d/7
		},
		clock: clock,
	})
	defer multi.Stop()

	key := 1
	multi.Add(key)

	clock.BlockUntilContext(ctx, 1)
	clock.Advance(time.Minute)

	// this is enough to saturate the VariableDuration, so we are going to hit
	// the max duration every time
	ivl.Add(100)

	ts := <-multi.Elapsed()
	multi.Tick(ts)
	require.Equal(t, start.Add(time.Minute), ts)

	clock.BlockUntilContext(ctx, 1)
	clock.Advance(2 * time.Minute)

	ts = <-multi.Elapsed()
	multi.Tick(ts)
	require.Equal(t, start.Add(3*time.Minute), ts)

	multi.Reset(key, 7*time.Minute)
	clock.BlockUntilContext(ctx, 1)
	clock.Advance(8 * time.Minute)
	ts = <-multi.Elapsed()
	require.Equal(t, start.Add(11*time.Minute), ts)
}

func TestMultiReset(t *testing.T) {
	t.Parallel()
	const (
		// each interval's timer waits this long
		interval = 60 * time.Minute
		// initially space the intervals apart by this much
		step = interval / 10
		// this many intervals
		intervals = 3
	)

	ctx := t.Context()
	clock := clockwork.NewFakeClock()
	start := clock.Now()
	multi := NewMulti[int](MultiParams{
		FixedInterval: interval,
		clock:         clock,
	})
	defer multi.Stop()

	// verify that delay is in an initial state that will never fire
	require.Nil(t, multi.Elapsed())

	for i := range intervals {
		multi.Add(i)
		clock.Advance(step)
	}
	// initially the interval queue should be: [1 2 0]

	clock.BlockUntilContext(ctx, 1)
	clock.Advance(clock.Until(start.Add(interval)))
	now := mustElapse(t, multi)
	require.Equal(t, start.Add(interval), now)
	require.Equal(t, 0, multi.Tick(now))
	// queue should now be: [1 2 0]

	// sift 2 up, ahead of 1
	multi.Reset(2, time.Millisecond)
	// queue should now be: [2 1 0]
	clock.BlockUntilContext(ctx, 1)
	clock.Advance(time.Millisecond)
	now = mustElapse(t, multi)
	require.Equal(t, start.Add(interval+time.Millisecond), now)
	require.Equal(t, 2, multi.Tick(now))
	// queue should now be: [1 0 2]
	// fix up the spacing between 0 and 2
	multi.Reset(2, interval-time.Millisecond)

	clock.BlockUntilContext(ctx, 1)
	clock.Advance(clock.Until(start.Add(interval + step)))
	now = mustElapse(t, multi)
	require.Equal(t, start.Add(interval+step), now)
	require.Equal(t, 1, multi.Tick(now))
	// queue should now be: [0 2 1]

	// sift 0 down, after 1
	multi.Reset(0, interval)
	// queue should now be: [2 1 0]
	clock.BlockUntilContext(ctx, 1)
	clock.Advance(clock.Until(start.Add(2 * interval)))
	now = mustElapse(t, multi)
	require.Equal(t, start.Add(2*interval), now)
	require.Equal(t, 2, multi.Tick(now))
	// queue should now be: [1 0 2]

	clock.BlockUntilContext(ctx, 1)
	clock.Advance(clock.Until(start.Add(2*interval + step)))
	now = mustElapse(t, multi)
	require.Equal(t, start.Add(2*interval+step), now)
	require.Equal(t, 1, multi.Tick(now))
}

func mustElapse[T comparable](t *testing.T, multi *Multi[T]) time.Time {
	t.Helper()
	select {
	case now := <-multi.Elapsed():
		return now
	default:
		require.FailNow(t, "multi timer did not fire")
		return time.Time{}
	}
}
