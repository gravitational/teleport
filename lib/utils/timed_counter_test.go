// Copyright 2021 Gravitational, Inc
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

package utils

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestTimedCounterReturnsZeroOnConstruction(t *testing.T) {
	t.Parallel()

	uut := NewTimedCounter(clockwork.NewFakeClock(), time.Second)
	require.Zero(t, uut.Count())
}

func TestTimedCounterIncrement(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	uut := NewTimedCounter(clock, time.Second)
	require.Equal(t, uut.Increment(), 1)
}

func TestTimedCounterExpiresEvents(t *testing.T) {
	t.Parallel()

	// Given a counter with a 10-second cutoff, primed with events at 1 second
	// intervals
	clock := clockwork.NewFakeClock()
	uut := NewTimedCounter(clock, 10*time.Second)
	for i := 1; i <= 5; i++ {
		require.Equal(t, uut.Increment(), i)
		require.Equal(t, uut.Count(), i)
		clock.Advance(1 * time.Second)
	}

	// When I advance the clock to a time slightly earlier than the 10s
	// cutoff for the first event...
	clock.Advance(4900 * time.Millisecond) // 9.9s "after" the first event

	// Expect that the all the events are still counted
	require.Equal(t, uut.Count(), 5)

	// When I advance the clock to just *after* the first event's cutoff...
	clock.Advance(200 * time.Millisecond) // 10.1s "after" the first event

	// Expect that the first event has expired and is no longer considered
	// by the counter
	require.Equal(t, uut.Count(), 4)

	// When I advance the clock past another 3 events' cutoff times
	clock.Advance(3 * time.Second)

	// Expect that there is only one event left to count
	require.Equal(t, uut.Count(), 1)

	// When I advance the clock well past the final event's cutoff
	clock.Advance(100 * time.Second)

	// Expect that there are no events left to count
	require.Equal(t, uut.Count(), 0)
}

func TestTimedCounterIncrementExpiresValues(t *testing.T) {
	t.Parallel()

	// Given a counter with a 10-second cutoff, primed with 5 events at 1-
	// second intervals
	clock := clockwork.NewFakeClock()

	uut := NewTimedCounter(clock, 10*time.Second)
	for i := 1; i <= 5; i++ {
		require.Equal(t, uut.Increment(), i)
		require.Equal(t, uut.Count(), i)
		clock.Advance(1 * time.Second)
	}

	// When I advance the clock to 12.5s after the first event, by which time
	// events at 0s, 1s and 2s should have expired.
	clock.Advance(7500 * time.Millisecond)

	// Expect that incrementing the count handles expiring three events and
	// adding a new one (net result: 3 "live" events remaining)
	require.Equal(t, 3, uut.Increment())
}
