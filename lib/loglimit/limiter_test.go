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

package loglimit

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestLimiterSweepCleansUpStaleWindows(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	limiter := newLimiter(limiterConfig{
		clock:                 clock,
		sweepMultiplier:       3,
		staleMultiplier:       2,
		allowAtWindowBoundary: false,
		sweepAtBoundary:       false,
		staleAtBoundary:       false,
	})

	require.True(t, limiter.allow("A", time.Minute))
	require.Len(t, limiter.windows, 1)

	// Simulate frequent calls that occur between key "A" going
	// stale and the sweep interval being reached.
	for elapsed := time.Duration(0); elapsed < 5*time.Minute; elapsed += time.Minute {
		clock.Advance(time.Minute)
		limiter.allow("B", time.Minute)
	}

	require.Len(t, limiter.windows, 1, "stale window for A should have been swept")
	require.Contains(t, limiter.windows, "B", "only the fresh B window should remain")
}

func TestLimiterZeroOrNegativeAllowWindowStillRunsSweep(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := newLimiter(limiterConfig{
		clock:                 clock,
		sweepInterval:         10 * time.Second,
		sweepMultiplier:       2,
		staleMultiplier:       2,
		allowAtWindowBoundary: true,
		sweepAtBoundary:       true,
		staleAtBoundary:       true,
	})

	require.True(t, limiter.allow("A", 5*time.Second))

	clock.Advance(10 * time.Second)
	require.True(t, limiter.allow("cleanup", 0))

	_, existsAtBoundary := limiter.windows["A"]
	require.False(t, existsAtBoundary)

	clock.Advance(10 * time.Second)
	require.True(t, limiter.allow("cleanup", -time.Second))
}

func TestLimiterDerivedSweepIntervalTracksMinimumRetainedWindow(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := newLimiter(limiterConfig{
		clock:                 clock,
		sweepMultiplier:       2,
		staleMultiplier:       2,
		allowAtWindowBoundary: true,
		sweepAtBoundary:       true,
		staleAtBoundary:       true,
	})

	require.True(t, limiter.allow("short", 5*time.Second))
	require.True(t, limiter.allow("long", 60*time.Second))

	clock.Advance(10 * time.Second)
	require.True(t, limiter.allow("trigger", 60*time.Second))

	_, shortExists := limiter.windows["short"]
	require.False(t, shortExists)
	require.Equal(t, 60*time.Second, limiter.sweepWindow)
}

func TestLimiterFixedSweepIntervalIgnoresCallerWindowMix(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := newLimiter(limiterConfig{
		clock:                 clock,
		sweepInterval:         10 * time.Second,
		sweepMultiplier:       100,
		staleMultiplier:       2,
		allowAtWindowBoundary: true,
		sweepAtBoundary:       true,
		staleAtBoundary:       true,
	})

	require.True(t, limiter.allow("short", 5*time.Second))
	require.True(t, limiter.allow("long", 60*time.Second))

	clock.Advance(10 * time.Second)
	require.True(t, limiter.allow("trigger", 60*time.Second))

	_, shortExists := limiter.windows["short"]
	require.False(t, shortExists)

	require.Equal(t, 10*time.Second, limiter.currentSweepInterval())
}

func TestLimiterSweepWindowWidensOnReAdmission(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := newLimiter(limiterConfig{
		clock:                 clock,
		sweepMultiplier:       2,
		staleMultiplier:       2,
		allowAtWindowBoundary: true,
		sweepAtBoundary:       true,
		staleAtBoundary:       true,
	})

	require.True(t, limiter.allow("A", 5*time.Second))
	require.True(t, limiter.allow("B", 5*time.Second))
	require.Equal(t, 5*time.Second, limiter.sweepWindow)

	// Re-admit A with a wider window after its original window expires.
	clock.Advance(5 * time.Second)
	require.True(t, limiter.allow("A", 20*time.Second))

	// sweepWindow should recompute to B's 5s (the remaining minimum), not A's new 20s.
	require.Equal(t, 5*time.Second, limiter.sweepWindow)
}
