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
	// timeWindow is the time window over which log errors are deduplicated.
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
	// Entry is the logger entry.
	Entry *log.Entry
	// LogLevel is the log level at which errors will be logged.
	// The default log level (0) is panic.
	LogLevel log.Level
	// DebugReport indicates whether a debug report should be generated
	// for the errors reported.
	DebugReport bool
	// ErrorSubstrings contains a list of substrings belonging to the
	// errors that should be deduplicated.
	ErrorSubstrings []string
	// ChannelSize is the size of the channel used to send messages
	// to the log limiter.
	ChannelSize int
	// Clock is a clock to override in tests, set to real time clock
	// by default.
	Clock clockwork.Clock
}

// checkAndSetDefaults verifies configuration and sets defaults
func (c *Config) checkAndSetDefaults() error {
	if c.Entry == nil {
		c.Entry = log.WithFields(log.Fields{
			trace.Component: "loglimit",
		})
	}
	if c.ErrorSubstrings == nil {
		return trace.BadParameter("missing parameter ErrorMessages")
	}
	if c.ChannelSize < 0 {
		return trace.BadParameter("ChannelSize must be at least 0")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// LogLimiter deduplicates log errors over a certain time window.
type LogLimiter struct {
	Config
	// errorCh is used to send errors to the log limiter.
	errorCh chan string
	// errorMap contains deduplicated errors reported over
	// certain time windows.
	errorMap map[string]*errorWindowInfo
}

// errorWindowInfo contains the information necessary to deduplicate an error
// reported within a certain time window.
type errorWindowInfo struct {
	// firstError is the first error reported within this window.
	firstError string
	// timeWindowStart is the beginning of the time window, i.e.,
	// the time at which the error was first reported (for this window).
	timeWindowStart time.Time
	// occurrences are the occurrences of errors (that share the same
	// error substring) within this window.
	occurrences int
}

// New creates a new log limiter.
func New(cfg Config) (*LogLimiter, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &LogLimiter{
		Config:   cfg,
		errorCh:  make(chan string, cfg.ChannelSize),
		errorMap: make(map[string]*errorWindowInfo, len(cfg.ErrorSubstrings)),
	}, nil
}

// Log sends an error to the log limiter.
func (e *LogLimiter) Log(err error) {
	var errMsg string
	if e.DebugReport {
		errMsg = trace.DebugReport(err)
	} else {
		errMsg = err.Error()
	}
	e.errorCh <- errMsg
}

// Run runs the log limiter.
func (e *LogLimiter) Run(ctx context.Context) {
	t := e.Clock.NewTicker(timeWindowCleanupInterval)
	defer t.Stop()

	for {
		select {
		case err := <-e.errorCh:
			e.deduplicate(err)
		case <-t.Chan():
			e.cleanup()
		case <-ctx.Done():
			return
		}
	}
}

// deduplicate logs errors if they should not be deduplicated.
// Otherwise, it records error occurrences within a certain time window.
func (e *LogLimiter) deduplicate(err string) {
	// Log the error right away if it should not be deduplicated.
	deduplicate, errSubstring := e.shouldDeduplicate(err)
	if !deduplicate {
		e.log(err)
		return
	}

	// If the error should be deduplicated, check if there's an active
	// window for it (i.e if it has been reported during the past minute).
	info, ok := e.errorMap[errSubstring]
	if ok {
		// If the error has already been logged, simply increase the
		// number of occurrences.
		info.occurrences++
	} else {
		// If this is the first occurrence, save the error and log it.
		e.errorMap[errSubstring] = &errorWindowInfo{
			firstError:      err,
			timeWindowStart: e.Clock.Now(),
			occurrences:     1,
		}
		e.log(err)
	}
}

// cleanup removes time windows that have ended, logging the first error again
// together with the number of occurrences (of errors that share the same error
// substring) during the window.
func (e *LogLimiter) cleanup() {
	for errSubstring, info := range e.errorMap {
		if e.Clock.Now().After(info.timeWindowStart.Add(timeWindow)) {
			if info.occurrences > 1 {
				e.log(fmt.Sprintf(
					"%s (errors containing %q were seen %d times in the past minute)",
					info.firstError,
					errSubstring,
					info.occurrences,
				))
			}
			delete(e.errorMap, errSubstring)
		}
	}
}

// shouldDeduplicate returns true if the error should be deduplicated
// (along with its error substring).
func (e *LogLimiter) shouldDeduplicate(err string) (bool, string) {
	for _, errSubstring := range e.ErrorSubstrings {
		if strings.Contains(err, errSubstring) {
			return true, errSubstring
		}
	}
	return false, ""
}

// log logs the error at the defined log level.
func (e *LogLimiter) log(err string) {
	e.Entry.Logln(e.LogLevel, err)
}
