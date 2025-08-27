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

package backoff

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestDecorr(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClockAt(time.Unix(0, 0))
	base := 20 * time.Millisecond
	cap := 2 * time.Second
	backoff := NewDecorr(base, cap, clock)

	// Check exponential bounds.
	for max := 3 * base; max < cap; max = 3 * max {
		dur, err := measure(ctx, clock, func() error { return backoff.Do(ctx) })
		require.NoError(t, err)
		require.Greater(t, dur, base)
		require.LessOrEqual(t, dur, max)
	}

	// Check that exponential growth threshold.
	for range 2 {
		dur, err := measure(ctx, clock, func() error { return backoff.Do(ctx) })
		require.NoError(t, err)
		require.Greater(t, dur, base)
		require.LessOrEqual(t, dur, cap)
	}
}

func measure(ctx context.Context, clock *clockwork.FakeClock, fn func() error) (time.Duration, error) {
	done := make(chan struct{})
	var dur time.Duration
	var err error
	go func() {
		before := clock.Now()
		err = fn()
		after := clock.Now()
		dur = after.Sub(before)
		close(done)
	}()
	clock.BlockUntil(1)
	for {
		/*
			What does runtime.Gosched() do?
			> Gosched yields the processor, allowing other goroutines to run. It does not
			> suspend the current goroutine, so execution resumes automatically.

			Why do we need it?
			There are two concurrent goroutines at this point:
			- this one
			- the one that executes `fn()`
			When this one is scheduled to run it advances the clock a bit more.
			It might happen that this one keeps running over and over, while the other one is not scheduled.
			When that happens, the other 'select' (the one in decorr.Do) gets called and returns nil,
			the goroutine sets the `dur` value.
			However, it's too late because the observed time (`dur`) is already larger than expected.

			If both goroutines ran sequentially, this would work.
			Calling runtime.Gosched here, tries to give priority to the other goroutine.
			So, when the other goroutine's select is ready (the clock.After returns), it immediately returns and
			`dur` has the expected value.
		*/
		runtime.Gosched()
		select {
		case <-done:
			return dur, trace.Wrap(err)
		case <-ctx.Done():
			return time.Duration(0), trace.Wrap(ctx.Err())
		default:
			clock.Advance(5 * time.Millisecond)
			runtime.Gosched()
		}
	}
}
