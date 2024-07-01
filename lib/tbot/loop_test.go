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

package tbot

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/utils"
)

func Test_runOnInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	taskCh := make(chan struct{}, 3)
	log := utils.NewSlogLoggerForTests()
	clock := clockwork.NewFakeClock()
	cfg := runOnIntervalConfig{
		name:  "test",
		clock: clock,
		log:   log,
		f: func(ctx context.Context) error {
			taskCh <- struct{}{}
			return nil
		},
		retryLimit: 3,
		interval:   time.Minute * 10,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.NoError(t, runOnInterval(ctx, cfg))
	}()

	// Wait for three iterations to have been completed.
	for i := 0; i < 3; i++ {
		<-taskCh
		clock.Advance(time.Minute * 11)
	}

	// Cancel the ctx and make sure runOnInterval returns
	cancel()
	wg.Wait()
}

func Test_runOnInterval_failureExit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	callCount := atomic.Int64{}

	log := utils.NewSlogLoggerForTests()
	testErr := fmt.Errorf("test error")
	cfg := runOnIntervalConfig{
		name:  "test",
		clock: clockwork.NewRealClock(),
		log:   log,
		f: func(ctx context.Context) error {
			callCount.Add(1)
			return testErr
		},
		retryLimit:           2,
		interval:             time.Second,
		exitOnRetryExhausted: true,
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		assert.ErrorIs(t, runOnInterval(ctx, cfg), testErr)
	}()

	wg.Wait()
	assert.Equal(t, int64(2), callCount.Load())
}
