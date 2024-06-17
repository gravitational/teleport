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

	log := utils.NewSlogLoggerForTests()
	clock := clockwork.NewFakeClock()
	callCount := atomic.Int64{}
	cfg := runOnIntervalConfig{
		name:  "test",
		clock: clock,
		log:   log,
		f: func(ctx context.Context) error {
			callCount.Add(1)
			return nil
		},
		retryLimit:           3,
		interval:             time.Minute * 10,
		exitOnRetryExhausted: true,
	}

	errCh := make(chan error)
	go func() {
		errCh <- runOnInterval(ctx, cfg)
	}()

	// Wait for three iterations to have been completed.
	clock.BlockUntil(1)
	clock.Advance(time.Minute * 11)
	clock.BlockUntil(1)
	clock.Advance(time.Minute * 11)
	clock.BlockUntil(1)

	// Cancel the ctx and make sure runOnInterval returns
	cancel()
	gotErr := <-errCh
	assert.NoError(t, gotErr)
	assert.Equal(t, int64(3), callCount.Load())
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
		retryLimit:           3,
		interval:             time.Second,
		exitOnRetryExhausted: true,
	}

	errCh := make(chan error)
	go func() {
		errCh <- runOnInterval(ctx, cfg)
	}()

	gotErr := <-errCh
	assert.ErrorIs(t, gotErr, testErr)
	assert.Equal(t, int64(3), callCount.Load())
}
