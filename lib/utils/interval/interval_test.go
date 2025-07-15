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

// TestLastTick verifies that the LastTick method returns the last tick time as expected.
func TestLastTick(t *testing.T) {
	clock := clockwork.NewFakeClock()
	interval := New(Config{
		Duration: time.Minute,
		Clock:    clock,
	})

	_, ok := interval.LastTick()
	require.False(t, ok)

	timeout := time.After(time.Second * 30)
	for range 3 {
		clock.Advance(time.Minute)

		var tick time.Time
		select {
		case tick = <-interval.Next():
		case <-timeout:
			t.Fatal("timeout waiting for tick")
		}
		require.Equal(t, clock.Now(), tick)

		tick, ok = interval.LastTick()
		require.True(t, ok)
		require.Equal(t, clock.Now(), tick)
	}
}

// TestIntervalReset verifies the basic behavior of the interval reset functionality.
// Since time based tests tend to be flaky, this test passes if it has a >50% success
// rate (i.e. >50% of resets seemed to have actually extended the timer successfully).
func TestIntervalReset(t *testing.T) {
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

			interval := New(Config{
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
					interval.Reset()
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

// TestIntervalResetTo verifies the basic behavior of the interval ResetTo method.
// Since time based tests tend to be flaky, this test passes if it has a >50% success
// rate (i.e. >50% of ResetTo calls seemed to have changed the timer's behavior as expected).
func TestIntervalResetTo(t *testing.T) {
	const workers = 1_000
	const ticks = 12
	const longDuration = time.Millisecond * 800
	const shortDuration = time.Millisecond * 200
	t.Parallel()

	var success, failure atomic.Uint64
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			interval := New(Config{
				Duration: longDuration,
			})
			defer interval.Stop()

			start := time.Now()

			for range ticks {
				interval.ResetTo(shortDuration)
				<-interval.Next()
			}

			elapsed := time.Since(start)
			// if the above works completed before the expected minimum time
			// to complete all ticks as long ticks, we assume that ResetTo has
			// successfully shortened the interval.
			if elapsed < longDuration*time.Duration(ticks) {
				success.Add(1)
			} else {
				failure.Add(1)
			}
		}()
	}

	wg.Wait()

	require.Greater(t, success.Load(), failure.Load())
}

func TestNewNoop(t *testing.T) {
	t.Parallel()
	i := NewNoop()
	ch := i.Next()
	select {
	case <-ch:
		t.Fatalf("noop should not emit anything")
	default:
	}
	i.Stop()
	select {
	case <-ch:
		t.Fatalf("noop should not emit anything")
	default:
	}
}
