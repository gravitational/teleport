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
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	// timeWindow is the time window over which logs are deduplicated.
	timeWindow = time.Minute
	// sweepInterval is how often the window mapping should be checked for stale entries.
	sweepInterval = timeWindow * 3
	// staleDuration is the amount of time an entry can exist in the
	// window mapping before being evicted.
	staleDuration = timeWindow * 2
)

// Config contains the log limiter config.
type Config struct {
	// MessageSubstrings contains a list of substrings belonging to the logs that should be deduplicated.
	MessageSubstrings []string
	// Clock is a clock to override in tests, set to real time clock by default.
	Clock clockwork.Clock
	// Handler is the wrapped Handler that processes any messages that should be sampled.
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
	// limiter stores fixed-window suppression state for matched substrings.
	limiter *limiter
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

	if l.limiter.allow(messageSubstring, timeWindow) {
		return trace.Wrap(l.config.Handler.Handle(ctx, record))
	}

	return nil

}

// WithAttrs implements slog.Handler.
func (l *LogLimiter) WithAttrs(attrs []slog.Attr) slog.Handler {
	// slog.Handler values are immutable; derived handlers must behave as
	// independent values. We clone suppression state at derivation time so the
	// new handler starts with the same snapshot but then diverges independently.
	return &LogLimiter{
		config: Config{
			MessageSubstrings: l.config.MessageSubstrings,
			Clock:             l.config.Clock,
			Handler:           l.config.Handler.WithAttrs(attrs),
		},
		limiter: l.limiter.clone(),
	}
}

// WithGroup implements slog.Handler.
func (l *LogLimiter) WithGroup(name string) slog.Handler {
	// Keep the same clone semantics as WithAttrs: copy current limiter state,
	// then let parent/child handlers evolve independently.
	return &LogLimiter{
		config: Config{
			MessageSubstrings: l.config.MessageSubstrings,
			Clock:             l.config.Clock,
			Handler:           l.config.Handler.WithGroup(name),
		},
		limiter: l.limiter.clone(),
	}
}

// New creates a new log limiter.
func New(config Config) (*LogLimiter, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	l := &LogLimiter{
		config: config,
		limiter: newLimiter(limiterConfig{
			clock:                 config.Clock,
			sweepInterval:         sweepInterval,
			sweepMultiplier:       int(sweepInterval / timeWindow),
			staleMultiplier:       int(staleDuration / timeWindow),
			allowAtWindowBoundary: false,
			sweepAtBoundary:       false,
			staleAtBoundary:       false,
		}),
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
