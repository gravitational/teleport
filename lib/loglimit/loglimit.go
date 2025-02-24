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

package loglimit

import (
	"context"
	"log/slog"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	// timeWindow is the time window over which logs are deduplicated.
	timeWindow = time.Minute
	// sweepInterval is how often the window mapping should be checked
	// for stale entries.
	sweepInterval = timeWindow * 3
	// staleDuration is the amount of time an entry can exist in the
	// window mapping before being evicted.
	staleDuration = timeWindow * 2
)

// Config contains the log limiter config.
type Config struct {
	// MessageSubstrings contains a list of substrings belonging to the
	// logs that should be deduplicated.
	MessageSubstrings []string
	// Clock is a clock to override in tests, set to real time clock
	// by default.
	Clock clockwork.Clock
	// Handler is the wrapped Handler that processes any messages that
	// should be sampled.
	Handler slog.Handler
}

// checkAndSetDefaults verifies configuration and sets defaults
func (c *Config) checkAndSetDefaults() error {
	if c.MessageSubstrings == nil {
		return trace.BadParameter("missing parameter MessageSubstrings")
	}
	if c.Handler == nil {
		return trace.BadParameter("missing parameter Handler")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// LogLimiter is a [slog.Handler] that prevents duplicate
// messages from being emitted over a certain time window.
type LogLimiter struct {
	// config is the log limiter config.
	config Config
	// mu synchronizes access to `windows`.
	mu sync.Mutex
	// windows is a mapping from log substring to an active
	// time window.
	windows map[string]time.Time

	// lastSweep indicates the last time the full windows mapping
	// was sweeped and purged of expired entries.
	lastSweep time.Time
}

// Enabled implements slog.Handler.
func (l *LogLimiter) Enabled(ctx context.Context, level slog.Level) bool {
	return l.config.Handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (l *LogLimiter) Handle(ctx context.Context, record slog.Record) error {
	deduplicate, messageSubstring := l.shouldDeduplicate(record.Message)
	if !deduplicate {
		return trace.Wrap(l.config.Handler.Handle(ctx, record))
	}

	shouldLog := func(now time.Time) bool {
		l.mu.Lock()
		defer func() {
			// Periodically attempt to clean up the last seen mapping.
			if now.After(l.lastSweep.Add(sweepInterval)) {
				for key, lastSeen := range l.windows {
					if now.After(lastSeen.Add(staleDuration)) {
						delete(l.windows, key)
					}
				}
			}

			l.lastSweep = l.config.Clock.Now()
			l.mu.Unlock()
		}()
		lastSeen, ok := l.windows[messageSubstring]

		switch {
		case !ok:
			// If this is the first occurrence, log the entry and save it.
			l.windows[messageSubstring] = now
			return true
		case now.After(lastSeen.Add(timeWindow)):
			// If this is NOT the first occurrence BUT the last occurrence,
			// has expired, then permit the log entry and update the window.
			l.windows[messageSubstring] = l.config.Clock.Now()
			return true
		default:
			return false
		}
	}(l.config.Clock.Now())

	if shouldLog {
		return trace.Wrap(l.config.Handler.Handle(ctx, record))
	}

	return nil

}

// WithAttrs implements slog.Handler.
func (l *LogLimiter) WithAttrs(attrs []slog.Attr) slog.Handler {
	l.mu.Lock()
	defer l.mu.Unlock()

	return &LogLimiter{
		config: Config{
			MessageSubstrings: l.config.MessageSubstrings,
			Clock:             l.config.Clock,
			Handler:           l.config.Handler.WithAttrs(attrs),
		},
		windows: maps.Clone(l.windows),
	}
}

// WithGroup implements slog.Handler.
func (l *LogLimiter) WithGroup(name string) slog.Handler {
	l.mu.Lock()
	defer l.mu.Unlock()

	return &LogLimiter{
		config: Config{
			MessageSubstrings: l.config.MessageSubstrings,
			Clock:             l.config.Clock,
			Handler:           l.config.Handler.WithGroup(name),
		},
		windows: maps.Clone(l.windows),
	}
}

// New creates a new log limiter.
func New(config Config) (*LogLimiter, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	l := &LogLimiter{
		config:  config,
		windows: make(map[string]time.Time, len(config.MessageSubstrings)),
	}

	return l, nil
}

// shouldDeduplicate returns true if the log should be deduplicated.
// In case true is returned, the log substring is also returned.
func (l *LogLimiter) shouldDeduplicate(message string) (bool, string) {
	for _, messageSubstring := range l.config.MessageSubstrings {
		if strings.Contains(message, messageSubstring) {
			return true, messageSubstring
		}
	}
	return false, ""
}
