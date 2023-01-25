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
package errordedup

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"
)

func TestErrorDeduplicator(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc              string
		errorSubstrings   []string
		errorsFirstBatch  []string
		errorsSecondBatch []string
		errorsAssert      func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string)
	}{
		{
			desc: "errors that do not match error substrings are logged",
			errorSubstrings: []string{
				"A",
			},
			errorsFirstBatch: []string{
				"B error 1",
				"B error 2",
				"C error 1",
			},
			errorsSecondBatch: []string{},
			errorsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string) {
				require.Len(t, loggedFirstBatch, 3)
				require.Len(t, loggedSecondBatch, 0)

				require.Equal(t, loggedFirstBatch[0], "B error 1")
				require.Equal(t, loggedFirstBatch[1], "B error 2")
				require.Equal(t, loggedFirstBatch[2], "C error 1")
			},
		},
		{
			desc: "errors that match error substrings are deduplicated",
			errorSubstrings: []string{
				"A",
				"B",
			},
			errorsFirstBatch: []string{
				"A error 1",
				"B error 1",
				"B error 2",
				"B error 3",
				"C error 1",
			},
			errorsSecondBatch: []string{},
			errorsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string) {
				require.Len(t, loggedFirstBatch, 4)
				require.Len(t, loggedSecondBatch, 0)

				require.Equal(t, loggedFirstBatch[0], "A error 1")
				require.Equal(t, loggedFirstBatch[1], "B error 1")
				require.Equal(t, loggedFirstBatch[2], "C error 1")
				// "A error 1" does not get logged again as the number of occurrences is just 1
				require.Equal(t, loggedFirstBatch[3], "B error 1 (errors containing \"B\" were seen 3 times in the past minute)")
			},
		},
		{
			desc: "errors are deduplicated over time windows",
			errorSubstrings: []string{
				"A",
				"B",
			},
			errorsFirstBatch: []string{
				"A error 1",
				"B error 1",
				"B error 2",
				"B error 3",
				"C error 1",
			},
			errorsSecondBatch: []string{
				"A error 1",
				"A error 2",
				"C error 1",
				"A error 3",
				"A error 4",
			},
			errorsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch []string) {
				require.Len(t, loggedFirstBatch, 4)
				require.Len(t, loggedSecondBatch, 3)

				require.Equal(t, loggedFirstBatch[0], "A error 1")
				require.Equal(t, loggedFirstBatch[1], "B error 1")
				require.Equal(t, loggedFirstBatch[2], "C error 1")
				// "A error 1" does not get logged again as the number of occurrences is just 1
				require.Equal(t, loggedFirstBatch[3], "B error 1 (errors containing \"B\" were seen 3 times in the past minute)")

				require.Equal(t, loggedSecondBatch[0], "A error 1")
				require.Equal(t, loggedSecondBatch[1], "C error 1")
				require.Equal(t, loggedSecondBatch[2], "A error 1 (errors containing \"A\" were seen 4 times in the past minute)")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Create error deduplicator.
			// We purposely do not call Run (running the deduplicator in a goroutine)
			// so that we can manually control which actions are performed.
			logger, hook := logtest.NewNullLogger()
			clock := clockwork.NewFakeClock()
			errordedup, err := New(Config{
				Entry:           logger.WithField("from", "errordedup"),
				LogLevel:        log.InfoLevel,
				ErrorSubstrings: tc.errorSubstrings,
				Clock:           clock,
			})
			require.NoError(t, err)

			// Send first batch of errors to deduplicator.
			for _, err := range tc.errorsFirstBatch {
				errordedup.deduplicate(err)
			}

			// Make enough time pass and run cleanup
			// (ensuring that a new window starts and prior windows are logged).
			clock.Advance(2 * time.Minute)
			errordedup.cleanup()

			// Retrieved what was logged after the first batch.
			loggedFirstBatch := toErrorMessages(hook.AllEntries())
			hook.Reset()

			// Send second batch of errors to deduplicator.
			for _, err := range tc.errorsSecondBatch {
				errordedup.deduplicate(err)
			}

			// Make enough time pass so that a new window starts and prior windows are logged.
			clock.Advance(2 * time.Minute)
			errordedup.cleanup()

			// Retrieved what was logged after the second batch.
			loggedSecondBatch := toErrorMessages(hook.AllEntries())
			hook.Reset()

			// Run assert on what was logged.
			tc.errorsAssert(t, loggedFirstBatch, loggedSecondBatch)
		})
	}
}

// toErrorMessages retries the error messages from log entries.
func toErrorMessages(entries []*log.Entry) []string {
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry.Message)
	}
	return result
}
