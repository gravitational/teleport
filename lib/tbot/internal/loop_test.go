/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package internal

import (
	"context"
	"fmt"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func Test_RunOnInterval(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		start := time.Now()
		callTimes := make(chan time.Time)
		interval := time.Minute * 10
		cfg := RunOnIntervalConfig{
			Name: "test",
			Log:  logtest.NewLogger(),
			F: func(ctx context.Context) error {
				callTimes <- time.Now()
				return nil
			},
			RetryLimit: 3,
			Interval:   interval,
		}

		done := make(chan error, 1)
		go func() {
			done <- RunOnInterval(ctx, cfg)
		}()

		assert.Equal(t, start, <-callTimes, "first run should happen immediately")
		assert.Equal(t, start.Add(interval), <-callTimes, "second run should happen 10 minutes after first run")
		assert.Equal(t, start.Add(2*interval), <-callTimes, "third run should happen 20 minutes after the first run")

		cancel()
		assert.NoError(t, <-done)
	})
}

func Test_RunOnInterval_failureExit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		callCount := 0
		testErr := fmt.Errorf("test error")
		cfg := RunOnIntervalConfig{
			Name: "test",
			Log:  logtest.NewLogger(),
			F: func(ctx context.Context) error {
				callCount++
				return testErr
			},
			RetryLimit:           2,
			Interval:             time.Second,
			ExitOnRetryExhausted: true,
		}

		assert.ErrorIs(t, RunOnInterval(t.Context(), cfg), testErr)
		assert.Equal(t, 2, callCount)
	})
}
