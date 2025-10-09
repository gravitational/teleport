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
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogLimiter(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc             string
		logSubstrings    []string
		logsFirstBatch   []string
		logsSecondsBatch []string
		logsAssert       func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string)
	}{
		{
			desc: "logs that do not match log substrings are logged right away",
			logSubstrings: []string{
				"A",
			},
			logsFirstBatch: []string{
				"B log 1",
				"B log 2",
				"C log 1",
			},
			logsSecondsBatch: []string{},
			logsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string) {
				expectedLoggedFirstBatch := []string{
					"B log 1",
					"B log 2",
					"C log 1",
				}
				expectedLoggedSecondBatch := []string{}
				assert.Equal(t, expectedLoggedFirstBatch, loggedFirstBatch, "first batch elements mismatch")
				assert.Equal(t, expectedLoggedSecondBatch, loggedSecondBatch, "second batch elements mismatch")
			},
		},
		{
			desc: "logs that match log substrings are deduplicated",
			logSubstrings: []string{
				"A",
				"B",
			},
			logsFirstBatch: []string{
				"A log 1",
				"B log 1",
				"B log 2",
				"B log 3",
				"C log 1",
			},
			logsSecondsBatch: []string{},
			logsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string) {
				expectedLoggedFirstBatch := []string{
					"A log 1",
					"B log 1",
					"C log 1",
					"B log 1 (logs containing \"B\" were seen 3 times in the past minute)",
				}
				expectedLoggedSecondBatch := []string{}
				assert.Equal(t, expectedLoggedFirstBatch, loggedFirstBatch, "first batch elements mismatch")
				assert.Equal(t, expectedLoggedSecondBatch, loggedSecondBatch, "second batch elements mismatch")
			},
		},
		{
			desc: "logs are deduplicated over time windows",
			logSubstrings: []string{
				"A",
				"B",
			},
			logsFirstBatch: []string{
				"A log 1",
				"B log 1",
				"B log 2",
				"B log 3",
				"C log 1",
			},
			logsSecondsBatch: []string{
				"A log 1",
				"A log 2",
				"C log 1",
				"A log 3",
				"A log 4",
			},
			logsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string) {
				expectedLoggedFirstBatch := []string{
					"A log 1",
					"B log 1",
					"C log 1",
					"B log 1 (logs containing \"B\" were seen 3 times in the past minute)",
				}
				expectedLoggedSecondBatch := []string{
					"A log 1",
					"C log 1",
					"A log 1 (logs containing \"A\" were seen 4 times in the past minute)",
				}
				assert.Equal(t, expectedLoggedFirstBatch, loggedFirstBatch, "first batch elements mismatch")
				assert.Equal(t, expectedLoggedSecondBatch, loggedSecondBatch, "second batch elements mismatch")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Create log limiter.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			clock := clockwork.NewFakeClock()
			logLimiter, err := New(Config{
				Context:           ctx,
				MessageSubstrings: tc.logSubstrings,
				Clock:             clock,
			})
			require.NoError(t, err)

			// Create a log entry with a hook to capture logs.
			logger, hook := logtest.NewNullLogger()
			entry := logger.WithField("from", "loglimit")

			// Send first batch of logs to log limiter.
			for _, message := range tc.logsFirstBatch {
				logLimiter.Log(entry, log.InfoLevel, message)
			}

			notifyCh := make(chan struct{})
			// Move the clock forward and trigger a cleanup by sending a message
			// to the cleanup channel
			// (ensuring that a new window starts and prior windows are logged).
			clock.Advance(2 * time.Minute)
			logLimiter.cleanupCh <- notifyCh
			<-notifyCh

			// Retrieve what was logged after the first batch.
			loggedFirstBatch := toLogMessages(hook.AllEntries())
			hook.Reset()

			// Send second batch of logs to log limiter.
			for _, message := range tc.logsSecondsBatch {
				logLimiter.Log(entry, log.InfoLevel, message)
			}

			// Move the clock forward and trigger a cleanup by sending a message
			// to the cleanup channel
			// (ensuring that a new window starts and prior windows are logged).
			clock.Advance(2 * time.Minute)
			logLimiter.cleanupCh <- notifyCh
			<-notifyCh

			// Retrieve what was logged after the second batch.
			loggedSecondBatch := toLogMessages(hook.AllEntries())
			hook.Reset()

			// Run assert on what was logged.
			tc.logsAssert(t, loggedFirstBatch, loggedSecondBatch)
		})
	}
}

// toLogMessages retrieves the log messages from log entries.
func toLogMessages(entries []*log.Entry) []string {
	result := make([]string, len(entries))
	for i, entry := range entries {
		result[i] = entry.Message
	}
	return result
}
