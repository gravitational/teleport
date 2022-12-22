// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package interval

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			resetTimer := time.NewTimer(duration / 3)
			defer resetTimer.Stop()

			interval := NewMulti[string](SubInterval[string]{
				Key:      "key",
				Duration: duration,
			})
			defer interval.Stop()

			start := time.Now()

			for i := 0; i < 6; i++ {
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
	for i := 0; i < 60; i++ {
		tick := <-interval.Next()
		require.True(t, !tick.Time.IsZero())
		require.True(t, tick.Time.After(prevT) || tick.Time == prevT)
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

// TestMultiIntervalPush verifies the expected behavior of MultiInterval.Push, both in terms of
// its ability to add new sub-intervals, and to overwrite existing sub-intervals.
func TestMultiIntervalPush(t *testing.T) {
	t.Parallel()
	interval := NewMulti[string](
		SubInterval[string]{
			Key:      "foo",
			Duration: time.Millisecond * 6,
		},
	)
	defer interval.Stop()

	// verify that single-interval is working
	for i := 0; i < 3; i++ {
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
	for i := 0; i < 60; i++ {
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
	for i := 0; i < 60; i++ {
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
	for i := 0; i < 10; i++ {
		tick := <-interval.Next()
		require.Equal(t, "fast", tick.Key)
	}

	// trigger the slow interval
	interval.FireNow("slow")

	// make sure that we observe slow interval firing
	var seenSlow bool
	for i := 0; i < 60; i++ {
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
