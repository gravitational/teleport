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
	"maps"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// limiter tracks per-key admission windows and performs opportunistic
// stale-entry sweeps. It is the shared engine behind both [LogLimiter]
// and [KeyedLimiter].
type limiter struct {
	config limiterConfig

	mu          sync.Mutex
	windows     map[string]window
	lastSweep   time.Time
	sweepWindow time.Duration
}

// limiterConfig holds tuning knobs for admission, sweep cadence, and
// staleness. Boundary flags control whether comparisons use >= (true)
// or > (false).
type limiterConfig struct {
	clock clockwork.Clock

	// sweepInterval defines fixed cadence for opportunistic stale sweeps.
	// When zero, sweep cadence is derived from sweepMultiplier and the
	// current minimum retained window.
	sweepInterval time.Duration
	// sweepMultiplier derives sweep cadence as
	// sweepMultiplier * minimumRetainedWindow when sweepInterval is zero.
	// Ignored when sweepInterval is set.
	sweepMultiplier int
	// staleMultiplier controls how long an admitted key is retained:
	// staleMultiplier * lastAllowedWindow. A zero or negative value
	// causes immediate eviction.
	staleMultiplier int

	// allowAtWindowBoundary controls window expiry comparison. When true,
	// the exact boundary (now == windowEnd) counts as expired (>=).
	// When false, only strictly after counts (>).
	allowAtWindowBoundary bool
	// sweepAtBoundary controls sweep-due comparison. When true, the exact
	// boundary (now == sweepAt) triggers a sweep (>=). When false, only
	// strictly after triggers (>).
	sweepAtBoundary bool
	// staleAtBoundary controls staleness comparison. When true, the exact
	// boundary (now == staleAt) counts as stale (>=). When false, only
	// strictly after counts (>).
	staleAtBoundary bool
}

// window records the last admission time and duration for a single key.
type window struct {
	lastAllowedAt     time.Time
	lastAllowedWindow time.Duration
}

// newLimiter initializes a limiter with empty per-key state. The sweep
// timeline is anchored lazily on the first call to allow.
func newLimiter(config limiterConfig) *limiter {
	return &limiter{
		config:  config,
		windows: make(map[string]window),
	}
}

// allow reports whether key should be admitted for allowWindow and updates
// per-key state on admission. Calls with non-positive windows are always
// admitted and do not retain key state (but may still trigger a sweep).
// Opportunistic stale sweeps may run on every call depending on sweep cadence.
func (l *limiter) allow(key string, allowWindow time.Duration) bool {
	now := l.config.clock.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Anchor sweep timeline on first call.
	if l.lastSweep.IsZero() {
		l.lastSweep = now
	}

	if allowWindow <= 0 {
		l.maybeSweep(now)
		return true
	}

	storedWindow, ok := l.windows[key]
	if ok && !l.windowExpired(now, storedWindow) {
		l.maybeSweep(now)
		return false
	}

	l.windows[key] = window{
		lastAllowedAt:     now,
		lastAllowedWindow: allowWindow,
	}
	l.updateSweepWindowAfterAllow(ok, storedWindow.lastAllowedWindow, allowWindow)

	l.maybeSweep(now)
	return true
}

// windowExpired reports whether the admitted window has elapsed as of now.
// When allowAtWindowBoundary is true the exact boundary (now == windowEnd)
// counts as expired; otherwise only strictly after counts.
func (l *limiter) windowExpired(now time.Time, window window) bool {
	windowEnd := window.lastAllowedAt.Add(window.lastAllowedWindow)
	if l.config.allowAtWindowBoundary {
		return !now.Before(windowEnd)
	}

	return now.After(windowEnd)
}

// maybeSweep opportunistically evicts stale keys when the sweep cadence is due.
// It recalculates sweepWindow from retained entries after each sweep.
func (l *limiter) maybeSweep(now time.Time) {
	sweepInterval := l.currentSweepInterval()
	if sweepInterval <= 0 || !l.sweepDue(now, sweepInterval) {
		return
	}

	for key, window := range l.windows {
		if l.windowStale(now, window) {
			delete(l.windows, key)
		}
	}

	l.sweepWindow = minimumRetainedWindow(l.windows)
	l.lastSweep = now
}

// currentSweepInterval returns the active sweep cadence.
// A fixed interval takes precedence; otherwise cadence is derived from the
// current minimum retained window and sweepMultiplier.
func (l *limiter) currentSweepInterval() time.Duration {
	if l.config.sweepInterval > 0 {
		return l.config.sweepInterval
	}
	if l.config.sweepMultiplier <= 0 || l.sweepWindow <= 0 {
		return 0
	}

	return time.Duration(l.config.sweepMultiplier) * l.sweepWindow
}

// updateSweepWindowAfterAllow updates the tracked minimum retained window after
// allowing a key. This keeps the sweep cadence aligned with the smallest retained
// window. When a fixed sweepInterval is configured, this is a no-op.
//
// If re-admitting a retained key widens what used to be the minimum window,
// recompute the minimum from all retained entries.
func (l *limiter) updateSweepWindowAfterAllow(entryExisted bool, previousWindow, newWindow time.Duration) {
	if l.config.sweepInterval > 0 {
		return
	}

	if l.sweepWindow <= 0 || newWindow < l.sweepWindow {
		l.sweepWindow = newWindow
		return
	}

	// If we just widened the current minimum, recompute it from retained entries.
	if entryExisted && previousWindow == l.sweepWindow && newWindow > previousWindow {
		l.sweepWindow = minimumRetainedWindow(l.windows)
	}
}

// sweepDue reports whether a sweep should run as of now for sweepInterval.
// When sweepAtBoundary is true the exact boundary (now == sweepAt) triggers
// a sweep; otherwise only strictly after triggers.
func (l *limiter) sweepDue(now time.Time, sweepInterval time.Duration) bool {
	sweepAt := l.lastSweep.Add(sweepInterval)
	if l.config.sweepAtBoundary {
		return !now.Before(sweepAt)
	}

	return now.After(sweepAt)
}

// windowStale reports whether a retained key is stale and should be evicted.
// A key is retained for staleMultiplier * lastAllowedWindow after its last
// admission. A non-positive retention duration causes immediate eviction.
// When staleAtBoundary is true the exact boundary (now == staleAt) counts
// as stale; otherwise only strictly after counts.
func (l *limiter) windowStale(now time.Time, window window) bool {
	retainedDuration := time.Duration(l.config.staleMultiplier) * window.lastAllowedWindow
	if retainedDuration <= 0 {
		return true
	}

	staleAt := window.lastAllowedAt.Add(retainedDuration)
	if l.config.staleAtBoundary {
		return !now.Before(staleAt)
	}

	return now.After(staleAt)
}

// clone returns a thread-safe snapshot copy of limiter state.
func (l *limiter) clone() *limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	return &limiter{
		config:      l.config,
		windows:     maps.Clone(l.windows),
		lastSweep:   l.lastSweep,
		sweepWindow: l.sweepWindow,
	}
}

// minimumRetainedWindow returns the smallest positive retained window.
// Returns zero when there are no positive windows.
func minimumRetainedWindow(windows map[string]window) time.Duration {
	var minWindow time.Duration
	for _, window := range windows {
		if window.lastAllowedWindow <= 0 {
			continue
		}
		if minWindow <= 0 || window.lastAllowedWindow < minWindow {
			minWindow = window.lastAllowedWindow
		}
	}

	return minWindow
}
