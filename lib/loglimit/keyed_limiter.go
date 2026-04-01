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
	"time"

	"github.com/jonboulle/clockwork"
)

const (
	// defaultKeyedSweepMultiplier is the default sweep cadence relative to
	// the smallest retained window (sweep every 3× the minimum window).
	defaultKeyedSweepMultiplier = 3
	// defaultKeyedStaleMultiplier is the default retention relative to each
	// key's admitted window (retain for 2× the window before eviction).
	defaultKeyedStaleMultiplier = 2
)

// KeyedLimiterConfig configures a [KeyedLimiter]. All fields are optional;
// zero values select sensible defaults.
type KeyedLimiterConfig struct {
	Clock clockwork.Clock
	// SweepInterval sets a fixed cadence for opportunistic stale-entry
	// sweeps. Sweeps piggyback on [KeyedLimiter.Allow] calls, so they
	// only run when the limiter is actively used. When zero (the default),
	// cadence is derived from SweepMultiplier × the smallest retained
	// window.
	SweepInterval time.Duration
	// SweepMultiplier derives sweep cadence as SweepMultiplier × the
	// smallest retained window when SweepInterval is zero. Ignored when
	// SweepInterval is set. Defaults to 3.
	SweepMultiplier int
	// StaleMultiplier controls how long an admitted key is retained:
	// StaleMultiplier × the key's last admitted window. After that
	// duration the entry is eligible for eviction. A zero or negative
	// value causes immediate eviction. Defaults to 2.
	StaleMultiplier int
}

// checkAndSetDefaults fills zero-valued fields with production defaults.
func (c *KeyedLimiterConfig) checkAndSetDefaults() {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.SweepMultiplier <= 0 {
		c.SweepMultiplier = defaultKeyedSweepMultiplier
	}
	if c.StaleMultiplier <= 0 {
		c.StaleMultiplier = defaultKeyedStaleMultiplier
	}
}

// KeyedLimiter suppresses repeated admissions for the same key within a
// caller-supplied window. All time comparisons are boundary-inclusive
// (>=), so a key becomes eligible for re-admission exactly when its
// window elapses.
type KeyedLimiter struct {
	*limiter
}

// NewKeyedLimiter creates a [KeyedLimiter] with the given configuration.
// Zero-valued config fields are replaced with sensible defaults.
func NewKeyedLimiter(config KeyedLimiterConfig) *KeyedLimiter {
	config.checkAndSetDefaults()

	return &KeyedLimiter{
		limiter: newLimiter(limiterConfig{
			clock:                 config.Clock,
			sweepInterval:         config.SweepInterval,
			sweepMultiplier:       config.SweepMultiplier,
			staleMultiplier:       config.StaleMultiplier,
			allowAtWindowBoundary: true,
			sweepAtBoundary:       true,
			staleAtBoundary:       true,
		}),
	}
}

// Allow reports whether key should be admitted for the given window.
//
// Empty keys are valid and behave like a shared global bucket.
// Non-positive windows are always admitted and do not retain key state
// (but may still trigger an opportunistic stale sweep).
//
// For a positive window, a key is admitted when it has never been seen
// or when now >= lastAllowedAt + lastAllowedWindow (boundary-inclusive).
// On admission, both timestamp and stored window are updated. On
// suppression, state is unchanged.
func (l *KeyedLimiter) Allow(key string, window time.Duration) bool {
	return l.limiter.allow(key, window)
}
