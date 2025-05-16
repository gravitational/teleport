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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

const (
	// timeWindow is the time window over which logs are deduplicated.
	timeWindow = time.Minute
	// timeWindowCleanupInterval is the interval between cleanups of time
	// windows that have already ended.
	// Since the time window is set to 1 minute and the cleanup interval
	// is set to 10 seconds, time windows can be in fact slightly larger
	// than 1 minute (i.e., 1 minute and 10 seconds, in the worst case).
	timeWindowCleanupInterval = 10 * time.Second
)

// Config contains the log limiter config.
type Config struct {
	// Context is a context to signal stops, cancellations.
	Context context.Context
	// MessageSubstrings contains a list of substrings belonging to the
	// logs that should be deduplicated.
	MessageSubstrings []string
	// Clock is a clock to override in tests, set to real time clock
	// by default.
	Clock clockwork.Clock
}

// checkAndSetDefaults verifies configuration and sets defaults
func (c *Config) checkAndSetDefaults() error {
	if c.Context == nil {
		c.Context = context.Background()
	}
	if c.MessageSubstrings == nil {
		return trace.BadParameter("missing parameter MessageSubstrings")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// LogLimiter deduplicates logs over a certain time window.
type LogLimiter struct {
	// config is the log limiter config.
	config Config
	// mu synchronizes access to `windows`.
	mu sync.Mutex
	// windows is a mapping from log substring to an active
	// time window.
	windows map[string]*entryInfo
	// cleanupCh is used in tests to trigger `cleanup`.
	cleanupCh chan chan struct{}
}

// entryInfo contains information about a certain log entry.
// This is information is used to deduplicate a log entry
// reported within a certain time window.
type entryInfo struct {
	// entry is the logger entry.
	entry *log.Entry
	// level is the log level at which this log entry should be logged.
	level log.Level
	// message is the message message.
	message string
	// time is the at which the log reported.
	time time.Time
	// occurrences are the occurrences of logs entries (that share
	// the same log substring) within a time window.
	occurrences int
}

// New creates a new log limiter.
func New(config Config) (*LogLimiter, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	l := &LogLimiter{
		config:    config,
		windows:   make(map[string]*entryInfo, len(config.MessageSubstrings)),
		cleanupCh: make(chan chan struct{}),
	}
	// Start the goroutine responsible for removing expired time windows.
	go l.run()
	return l, nil
}

// Log deduplicates log entries that should be deduplicated.
func (l *LogLimiter) Log(entry *log.Entry, level log.Level, args ...any) {
	// Log right away if the log entry should not be deduplicated.
	message := fmt.Sprint(args...)
	deduplicate, messageSubstring := l.shouldDeduplicate(message)
	if !deduplicate {
		entry.Log(level, message)
		return
	}

	l.insert(entry, level, message, messageSubstring)
}

// Run runs the cleanup of expired time windows periodically.
func (l *LogLimiter) run() {
	t := l.config.Clock.NewTicker(timeWindowCleanupInterval)
	defer t.Stop()

	for {
		select {
		case <-t.Chan():
			l.cleanup()
		case notifyCh := <-l.cleanupCh:
			l.cleanup()
			notifyCh <- struct{}{}
		case <-l.config.Context.Done():
			return
		}
	}
}

// insert records the occurrence of a log entry within a certain time window.
// If it's the first occurrence, it also logs the entry.
func (l *LogLimiter) insert(entry *log.Entry, level log.Level, message, messageSubstring string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if there's an active window for the log entry
	// (i.e if it has been reported during the past minute).
	window, ok := l.windows[messageSubstring]
	if ok {
		// If the log has already been logged, simply increase the
		// number of occurrences.
		window.occurrences++
	} else {
		// If this is the first occurrence, log the entry and save it.
		entry.Log(level, message)
		l.windows[messageSubstring] = &entryInfo{
			entry:       entry,
			level:       level,
			message:     message,
			time:        l.config.Clock.Now(),
			occurrences: 1,
		}
	}
}

// cleanup removes time windows that have ended, logging the first log message
// again together with the number of occurrences (of logs that share the same
// log substring) during the window.
func (l *LogLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.config.Clock.Now()
	for logSubstring, e := range l.windows {
		if now.After(e.time.Add(timeWindow)) {
			if e.occurrences > 1 {
				e.entry.Log(
					e.level,
					fmt.Sprintf(
						"%s (logs containing %q were seen %d times in the past minute)",
						e.message,
						logSubstring,
						e.occurrences,
					),
				)
			}
			delete(l.windows, logSubstring)
		}
	}
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
