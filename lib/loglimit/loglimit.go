/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package loglimit

import (
	"context"
	"fmt"
	"strings"
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
	// LogSubstrings contains a list of substrings belonging to the
	// logs that should be deduplicated.
	LogSubstrings []string
	// ChannelSize is the size of the channel used to send messages
	// to the log limiter.
	ChannelSize int
	// Clock is a clock to override in tests, set to real time clock
	// by default.
	Clock clockwork.Clock
}

// checkAndSetDefaults verifies configuration and sets defaults
func (c *Config) checkAndSetDefaults() error {
	if c.LogSubstrings == nil {
		return trace.BadParameter("missing parameter LogSubstrings")
	}
	if c.ChannelSize < 0 {
		return trace.BadParameter("ChannelSize must be at least 0")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// LogLimiter deduplicates logs over a certain time window.
type LogLimiter struct {
	Config
	// entryCh is used to send log entries to the log limiter.
	entryCh chan *entryInfo
	// windows is a mapping from log substring to an active
	// time window.
	windows map[string]*entryInfo
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
func New(cfg Config) (*LogLimiter, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &LogLimiter{
		Config:  cfg,
		entryCh: make(chan *entryInfo, cfg.ChannelSize),
		windows: make(map[string]*entryInfo, len(cfg.LogSubstrings)),
	}, nil
}

// Log sends a log to the log limiter.
func (l *LogLimiter) Log(entry *log.Entry, level log.Level, args ...any) {
	l.entryCh <- &entryInfo{
		entry:       entry,
		level:       level,
		message:     fmt.Sprint(args...),
		time:        l.Clock.Now(),
		occurrences: 1,
	}
}

// Run runs the log limiter.
func (l *LogLimiter) Run(ctx context.Context) {
	t := l.Clock.NewTicker(timeWindowCleanupInterval)
	defer t.Stop()

	for {
		select {
		case e := <-l.entryCh:
			l.deduplicate(e)
		case <-t.Chan():
			l.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

// deduplicate logs if they should not be deduplicated.
// Otherwise, it records log occurrences within a certain time window.
func (l *LogLimiter) deduplicate(e *entryInfo) {
	// Log right away if the log entry should not be deduplicated.
	deduplicate, logSubstring := l.shouldDeduplicate(e)
	if !deduplicate {
		e.entry.Log(e.level, e.message)
		return
	}

	// If the log should be deduplicated, check if there's an active
	// window for it (i.e if it has been reported during the past minute).
	window, ok := l.windows[logSubstring]
	if ok {
		// If the log has already been logged, simply increase the
		// number of occurrences.
		window.occurrences++
	} else {
		// If this is the first occurrence, save the log entry and log it.
		l.windows[logSubstring] = e
		e.entry.Log(e.level, e.message)
	}
}

// cleanup removes time windows that have ended, logging the first log message
// again together with the number of occurrences (of logs that share the same
// log substring) during the window.
func (l *LogLimiter) cleanup() {
	now := l.Clock.Now()
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
func (l *LogLimiter) shouldDeduplicate(e *entryInfo) (bool, string) {
	for _, logSubstring := range l.LogSubstrings {
		if strings.Contains(e.message, logSubstring) {
			return true, logSubstring
		}
	}
	return false, ""
}
