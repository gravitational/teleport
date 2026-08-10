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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func Test_RunOnInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	taskCh := make(chan struct{}, 3)
	log := logtest.NewLogger()
	clock := clockwork.NewFakeClock()
	cfg := RunOnIntervalConfig{
		Name:  "test",
		Clock: clock,
		Log:   log,
		F: func(ctx context.Context) error {
			taskCh <- struct{}{}
			return nil
		},
		RetryLimit: 3,
		Interval:   time.Minute * 10,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, RunOnInterval(ctx, cfg))
	}()

	// Wait for three iterations to have been completed.
	for i := 0; i < 3; i++ {
		<-taskCh
		clock.Advance(time.Minute * 11)
	}

	// Cancel the ctx and make sure RunOnInterval returns
	cancel()
	wg.Wait()
}

func Test_RunOnInterval_failureExit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	callCount := atomic.Int64{}

	log := logtest.NewLogger()
	testErr := fmt.Errorf("test error")
	cfg := RunOnIntervalConfig{
		Name:  "test",
		Clock: clockwork.NewRealClock(),
		Log:   log,
		F: func(ctx context.Context) error {
			callCount.Add(1)
			return testErr
		},
		RetryLimit:           2,
		Interval:             time.Second,
		ExitOnRetryExhausted: true,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.ErrorIs(t, RunOnInterval(ctx, cfg), testErr)
	}()

	wg.Wait()
	assert.Equal(t, int64(2), callCount.Load())
}
