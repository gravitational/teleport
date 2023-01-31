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
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
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
			desc: "logs that do not match log substrings are logged",
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
				require.Len(t, loggedFirstBatch, 3)
				require.Len(t, loggedSecondBatch, 0)

				require.Equal(t, loggedFirstBatch[0], "B log 1")
				require.Equal(t, loggedFirstBatch[1], "B log 2")
				require.Equal(t, loggedFirstBatch[2], "C log 1")
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
				require.Len(t, loggedFirstBatch, 4)
				require.Len(t, loggedSecondBatch, 0)

				require.Equal(t, loggedFirstBatch[0], "A log 1")
				require.Equal(t, loggedFirstBatch[1], "B log 1")
				require.Equal(t, loggedFirstBatch[2], "C log 1")
				// "A log 1" does not get logged again as the number of occurrences is just 1
				require.Equal(t, loggedFirstBatch[3], "B log 1 (logs containing \"B\" were seen 3 times in the past minute)")
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
				require.Len(t, loggedFirstBatch, 4)
				require.Len(t, loggedSecondBatch, 3)

				require.Equal(t, loggedFirstBatch[0], "A log 1")
				require.Equal(t, loggedFirstBatch[1], "B log 1")
				require.Equal(t, loggedFirstBatch[2], "C log 1")
				// "A log 1" does not get logged again as the number of occurrences is just 1
				require.Equal(t, loggedFirstBatch[3], "B log 1 (logs containing \"B\" were seen 3 times in the past minute)")

				require.Equal(t, loggedSecondBatch[0], "A log 1")
				require.Equal(t, loggedSecondBatch[1], "C log 1")
				require.Equal(t, loggedSecondBatch[2], "A log 1 (logs containing \"A\" were seen 4 times in the past minute)")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Create log limiter.
			// We purposely do not call Run (running the log limiter in a goroutine)
			// so that we can manually control which actions are performed.
			logger, hook := logtest.NewNullLogger()
			entry := logger.WithField("from", "loglimit")
			clock := clockwork.NewFakeClock()
			logLimiter, err := New(Config{
				LogSubstrings: tc.logSubstrings,
				Clock:         clock,
			})
			require.NoError(t, err)

			// Send first batch of logs to log limiter.
			for _, message := range tc.logsFirstBatch {
				logLimiter.deduplicate(entryInfo{
					entry:   entry,
					level:   log.InfoLevel,
					message: message,
					time:    clock.Now(),
				})
			}

			// Make enough time pass and run cleanup
			// (ensuring that a new window starts and prior windows are logged).
			clock.Advance(2 * time.Minute)
			logLimiter.cleanup()

			// Retrieve what was logged after the first batch.
			loggedFirstBatch := toLogMessages(hook.AllEntries())
			hook.Reset()

			// Send second batch of logs to log limiter.
			for _, message := range tc.logsSecondsBatch {
				logLimiter.deduplicate(entryInfo{
					entry:   entry,
					level:   log.InfoLevel,
					message: message,
					time:    clock.Now(),
				})
			}

			// Make enough time pass so that a new window starts and prior windows are logged.
			clock.Advance(2 * time.Minute)
			logLimiter.cleanup()

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
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Message)
	}
	return result
}
