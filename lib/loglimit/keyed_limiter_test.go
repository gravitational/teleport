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

func TestKeyedLimiterSuppressesWithinWindow(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("ec2", 10*time.Second))
	require.False(t, limiter.Allow("ec2", 10*time.Second))

	fakeClock.Advance(9 * time.Second)
	require.False(t, limiter.Allow("ec2", 10*time.Second))

	fakeClock.Advance(1 * time.Second)
	require.True(t, limiter.Allow("ec2", 10*time.Second))
}

func TestKeyedLimiterSupportsVariableWindows(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("ec2", 10*time.Second))

	fakeClock.Advance(10 * time.Second)
	require.True(t, limiter.Allow("ec2", 20*time.Second))

	fakeClock.Advance(19 * time.Second)
	require.False(t, limiter.Allow("ec2", 20*time.Second))

	fakeClock.Advance(1 * time.Second)
	require.True(t, limiter.Allow("ec2", 20*time.Second))
}

func TestKeyedLimiterSuppressedCallDoesNotRefreshStoredState(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("ec2", 5*time.Second))

	fakeClock.Advance(4 * time.Second)
	require.False(t, limiter.Allow("ec2", 30*time.Second))

	fakeClock.Advance(1 * time.Second)
	require.True(t, limiter.Allow("ec2", 5*time.Second))
}

func TestKeyedLimiterSuppressedCallDoesNotShrinkStoredWindow(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("ec2", 30*time.Second))

	fakeClock.Advance(10 * time.Second)
	require.False(t, limiter.Allow("ec2", 5*time.Second))

	fakeClock.Advance(19 * time.Second)
	require.False(t, limiter.Allow("ec2", 30*time.Second))

	fakeClock.Advance(1 * time.Second)
	require.True(t, limiter.Allow("ec2", 30*time.Second))
}

func TestKeyedLimiterSuppressionBoundaryIsInclusiveAtWindowEnd(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("ec2", 10*time.Second))

	fakeClock.Advance(9*time.Second + 999*time.Millisecond)
	require.False(t, limiter.Allow("ec2", 10*time.Second))

	fakeClock.Advance(1 * time.Millisecond)
	require.True(t, limiter.Allow("ec2", 10*time.Second))
}

func TestKeyedLimiterTracksStatePerKeyIndependently(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("short", 5*time.Second))
	require.True(t, limiter.Allow("long", 20*time.Second))

	fakeClock.Advance(15 * time.Second)
	require.True(t, limiter.Allow("short", 5*time.Second))
	require.False(t, limiter.Allow("long", 20*time.Second))
}

func TestKeyedLimiterAllowsEmptyKey(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("", 10*time.Second))
	require.False(t, limiter.Allow("", 10*time.Second))
}

func TestKeyedLimiterNonPositiveWindowAlwaysAllowsWithoutRetention(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{Clock: fakeClock})

	require.True(t, limiter.Allow("ec2", 10*time.Second))

	require.True(t, limiter.Allow("ec2", 0))
	require.False(t, limiter.Allow("ec2", 10*time.Second))

	require.True(t, limiter.Allow("ec2", -time.Second))
	require.False(t, limiter.Allow("ec2", 10*time.Second))

	fakeClock.Advance(10 * time.Second)
	require.True(t, limiter.Allow("ec2", 10*time.Second))
}

func TestKeyedLimiterCustomMultipliers(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, time.March, 23, 0, 0, 0, 0, time.UTC))
	limiter := NewKeyedLimiter(KeyedLimiterConfig{
		Clock:           fakeClock,
		SweepMultiplier: 2,
		StaleMultiplier: 3,
	})

	require.True(t, limiter.Allow("A", 10*time.Second))

	// Key should be retained for staleMultiplier * window = 30s.
	// Sweep cadence = sweepMultiplier * minWindow = 2*10s = 20s.
	// At t=20s: sweep runs, but A is not stale yet (30s threshold).
	fakeClock.Advance(20 * time.Second)
	require.True(t, limiter.Allow("trigger", 10*time.Second))
	_, aExists := limiter.limiter.windows["A"]
	require.True(t, aExists, "A should still be retained before stale threshold")

	// At t=40s: another sweep runs (20s since last), A is stale (40s > 30s).
	fakeClock.Advance(20 * time.Second)
	require.True(t, limiter.Allow("trigger2", 10*time.Second))
	_, aExists = limiter.limiter.windows["A"]
	require.False(t, aExists, "A should be evicted after stale threshold + sweep")
}
