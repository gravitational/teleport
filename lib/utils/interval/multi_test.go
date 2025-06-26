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

package interval

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestMultiIntervalReset verifies the basic behavior of the multi interval reset functionality.
// Since time based tests tend to be flaky, this test passes if it has a >50% success
// rate (i.e. >50% of resets seemed to have actually extended the timer successfully).
func TestMultiIntervalReset(t *testing.T) {
	const iterations = 1_000
	const duration = time.Millisecond * 666
	t.Parallel()

	var success, failure atomic.Uint64

	var wg sync.WaitGroup

	for range iterations {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resetTimer := time.NewTimer(duration / 3)
			defer resetTimer.Stop()

			interval := NewMulti[string](
				clockwork.NewRealClock(),
				SubInterval[string]{
					Key:      "key",
					Duration: duration,
				})
			defer interval.Stop()

			start := time.Now()

			for range 6 {
				select {
				case <-interval.Next():
					failure.Add(1)
					return
				case <-resetTimer.C:
					interval.Reset("key")
					resetTimer.Reset(duration / 3)
				}
			}

			<-interval.Next()
			elapsed := time.Since(start)
			// we expect this test to produce elapsed times of
			// 3*duration if it is working properly. we accept a
			// margin or error of +/- 1 duration in order to
			// minimize flakiness.
			if elapsed > duration*2 && elapsed < duration*4 {
				success.Add(1)
			} else {
				failure.Add(1)
			}
		}()
	}

	wg.Wait()

	require.Greater(t, success.Load(), failure.Load())
}

// TestMultiIntervalBasics verifies basic expected multiinterval behavior. Due to the general
// flakiness of time-based tests, this test is designed to only care about which sub-intervals
// tick *more* relative to one another, and not about the specific relative frequency of observed
// ticks.
func TestMultiIntervalBasics(t *testing.T) {
	t.Parallel()
	interval := NewMulti[string](
		clockwork.NewRealClock(),
		SubInterval[string]{
			Key:      "fast",
			Duration: time.Millisecond * 8,
		},
		SubInterval[string]{
			Key:      "slow",
			Duration: time.Millisecond * 16,
		},
		SubInterval[string]{
			Key:           "once",
			Duration:      time.Hour,
			FirstDuration: time.Millisecond,
		},
	)
	defer interval.Stop()

	var fast, slow, once int
	var prevT time.Time
	for range 60 {
		tick := <-interval.Next()
		require.False(t, tick.Time.IsZero())
		require.True(t, tick.Time.After(prevT) || tick.Time.Equal(prevT))
		prevT = tick.Time
		switch tick.Key {
		case "fast":
			fast++
		case "slow":
			slow++
		case "once":
			once++
		}
	}
	require.Equal(t, 1, once)
	require.Greater(t, slow, once)
	require.Greater(t, fast, slow)
}

// TestMultiIntervalVariableDuration verifies that variable durations within a multiinterval function
// as expected.
func TestMultiIntervalVariableDuration(t *testing.T) {
	t.Parallel()

	foo := NewVariableDuration(VariableDurationConfig{
		MinDuration: time.Millisecond * 8,
		MaxDuration: time.Hour,
		Step:        1,
	})

	foo.counter.Store(1)

	bar := NewVariableDuration(VariableDurationConfig{
		MinDuration: time.Millisecond * 8,
		MaxDuration: time.Hour,
		Step:        1,
	})

	bar.counter.Store(1)

	interval := NewMulti[string](
		clockwork.NewRealClock(),
		SubInterval[string]{
			Key:              "foo",
			VariableDuration: foo,
		},
		SubInterval[string]{
			Key:              "bar",
			VariableDuration: bar,
		},
	)
	defer interval.Stop()

	var fooct, barct int
	var prevT time.Time
	for range 60 {
		tick := <-interval.Next()
		require.False(t, tick.Time.IsZero())
		require.True(t, tick.Time.After(prevT) || tick.Time.Equal(prevT))
		prevT = tick.Time
		switch tick.Key {
		case "foo":
			fooct++
		case "bar":
			barct++
		}
	}
	require.Equal(t, 60, fooct+barct, "fooct=%d, barct=%d", fooct, barct)
	// intervals should be firing at the same rate, but since this test involves concurrent
	// timing it is *very* inconsistent when running on our test infra. Instead, assert that
	// nether value is more than 2x the other. In combination with the other conditions checked
	// further down, this will let us verify with reasonable certainty that increasing the variable
	// duration does increase firing frequency as expected. The exact nature of the change is
	// covered by other unit tests that don't rely on timing. This is just a sanity check to
	// verify that the deterministic tests aren't passing in error (e.g. checking a duration
	// value that isn't actually being used to calculate the final tick rate).
	require.InDelta(t, fooct, barct, 20)

	foo.counter.Store(2)
	bar.counter.Store(200_000)

	fooct = 0
	barct = 0
	for range 60 {
		tick := <-interval.Next()
		switch tick.Key {
		case "foo":
			fooct++
		case "bar":
			barct++
		}
	}

	require.Equal(t, 60, fooct+barct, "fooct=%d, barct=%d", fooct, barct)

	// foo should have fired *way* more than twice as often, but time-based tests are flaky
	// so we're checking for a very conservative difference in frequency here. the point is just
	// to prove that when the variable duration increases the firing duration increases as well.
	// covering specifics are left to the variable duration output tests, which are not time-based.
	require.Greater(t, fooct, barct*2, "fooct=%d, barct=%d", fooct, barct)
}

// TestMultiIntervalPush verifies the expected behavior of MultiInterval.Push, both in terms of
// its ability to add new sub-intervals, and to overwrite existing sub-intervals.
func TestMultiIntervalPush(t *testing.T) {
	t.Parallel()
	interval := NewMulti[string](
		clockwork.NewRealClock(),
		SubInterval[string]{
			Key:      "foo",
			Duration: time.Millisecond * 6,
		},
	)
	defer interval.Stop()

	// verify that single-interval is working
	for range 3 {
		tick := <-interval.Next()
		require.Equal(t, "foo", tick.Key)
	}

	// push a new slower sub-interval
	interval.Push(SubInterval[string]{
		Key:      "bar",
		Duration: time.Millisecond * 12,
	})

	// aggregate rates of both sub-intervals
	var foo, bar int
	for range 60 {
		tick := <-interval.Next()
		switch tick.Key {
		case "foo":
			foo++
		case "bar":
			bar++
		}
	}
	// verify that both sub-intervals are firing, and that
	// foo is firing more frequently.
	require.NotZero(t, foo)
	require.NotZero(t, bar)
	require.Greater(t, foo, bar)

	// overwrite the old sub-intervals, inverting their respective
	// tick rates.
	interval.Push(SubInterval[string]{
		Key:      "foo",
		Duration: time.Millisecond * 12,
	})
	interval.Push(SubInterval[string]{
		Key:      "bar",
		Duration: time.Millisecond * 6,
	})

	// aggregate new rates for both sub-intervals
	foo = 0
	bar = 0
	for range 60 {
		tick := <-interval.Next()
		switch tick.Key {
		case "foo":
			foo++
		case "bar":
			bar++
		}
	}
	// verify that both sub-intervals are firing, and that
	// foo their relative rates have flipped, with bar now
	// firing more frequently.
	require.NotZero(t, foo)
	require.NotZero(t, bar)
	require.Greater(t, bar, foo)
}

// TestMultiIntervalFireNow verifies the expected behavior of MultiInterval.FireNow.
func TestMultiIntervalFireNow(t *testing.T) {
	t.Parallel()
	// set up one sub-interval that fires frequently, and another that will never
	// fire during this test unless we trigger with FireNow.
	interval := NewMulti[string](
		clockwork.NewRealClock(),
		SubInterval[string]{
			Key:      "slow",
			Duration: time.Hour,
		},
		SubInterval[string]{
			Key:      "fast",
			Duration: time.Millisecond * 10,
		},
	)
	defer interval.Stop()

	// verify that only the 'fast' interval is firing
	for range 10 {
		tick := <-interval.Next()
		require.Equal(t, "fast", tick.Key)
	}

	// trigger the slow interval
	interval.FireNow("slow")

	// make sure that we observe slow interval firing
	var seenSlow bool
	for range 60 {
		tick := <-interval.Next()
		if tick.Key == "slow" {
			seenSlow = true
			break
		}
	}

	require.True(t, seenSlow)
}

// TestPendingTicks tests the expected behavior of the pendingTicks helper.
func TestPendingTicks(t *testing.T) {
	// "backlog" of fake ticks with lots of duplicates
	tks := []string{
		"foo",
		"bar",
		"foo",
		"foo",
		"bin",
		"bar",
		"bar",
		"baz",
		"foo",
	}

	var pending pendingTicks[string]

	// insert out fake ticks
	for _, tk := range tks {
		pending.add(time.Now(), tk)
	}

	// we expect to have not stored any duplicate keys (important for
	// preventing runaway memory growth due to neglected interval).
	require.Len(t, pending.keys, 4)

	// expected order of ticks to be emitted
	expect := []string{
		"foo",
		"bar",
		"bin",
		"baz",
	}

	// verify that we see the ticks in the order we expect
	for _, exp := range expect {
		tick, ok := pending.next()
		require.True(t, ok)
		require.Equal(t, exp, tick.Key)
		pending.remove(exp)
	}

	// verify that no more ticks are available
	_, ok := pending.next()
	require.False(t, ok)
}
