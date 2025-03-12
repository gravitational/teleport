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
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
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
		logsAssert       func(t *testing.T, loggedFirstBatch, loggedSecondBatch string)
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
			logsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch string) {
				expectedLoggedFirstBatch := `msg="B log 1"
msg="B log 2"
msg="C log 1"
`
				expectedLoggedSecondBatch := ""
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
			logsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch string) {
				expectedLoggedFirstBatch := `msg="A log 1"
msg="B log 1"
msg="C log 1"
`
				expectedLoggedSecondBatch := ""
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
			logsAssert: func(t *testing.T, loggedFirstBatch, loggedSecondBatch string) {
				expectedLoggedFirstBatch := `msg="A log 1"
msg="B log 1"
msg="C log 1"
`
				expectedLoggedSecondBatch := `msg="A log 1"
msg="C log 1"
`
				assert.Equal(t, expectedLoggedFirstBatch, loggedFirstBatch, "first batch elements mismatch")
				assert.Equal(t, expectedLoggedSecondBatch, loggedSecondBatch, "second batch elements mismatch")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			ctx := context.Background()

			var sink bytes.Buffer
			logLimiter, err := New(Config{
				MessageSubstrings: tc.logSubstrings,
				Clock:             clock,
				Handler: slog.NewTextHandler(&sink, &slog.HandlerOptions{
					AddSource: false,
					Level:     nil,
					ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
						if a.Key == slog.TimeKey || a.Key == slog.LevelKey {
							return slog.Attr{}
						}

						return a
					},
				}),
			})
			require.NoError(t, err)

			logger := slog.New(logLimiter)

			// Send first batch of logs to log limiter.
			for _, message := range tc.logsFirstBatch {
				//nolint:sloglint // the messages are statically defined in the tests but the linter doesn't know that
				logger.InfoContext(ctx, message)
			}

			// Move the clock forward to mark entries as stale
			clock.Advance(2 * time.Minute)

			// Retrieve what was logged after the first batch.
			loggedFirstBatch := sink.String()

			// Reset the sink
			sink.Reset()

			// Send second batch of logs to log limiter.
			for _, message := range tc.logsSecondsBatch {
				//nolint:sloglint // the messages are statically defined in the tests but the linter doesn't know that
				logger.InfoContext(ctx, message)
			}

			// Move the clock forward to mark entries as stale
			clock.Advance(2 * time.Minute)

			// Retrieve what was logged after the second batch.
			loggedSecondBatch := sink.String()

			// Run assert on what was logged.
			tc.logsAssert(t, loggedFirstBatch, loggedSecondBatch)
		})
	}
}
