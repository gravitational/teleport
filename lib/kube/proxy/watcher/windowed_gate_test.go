// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package watcher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

func newGate(t *testing.T, parent context.Context, window time.Duration) *WindowedGate {
	t.Helper()

	g, err := NewWindowedGate(parent, WindowedGateConfig{
		Window: window,
	})
	require.NoError(t, err)
	return g
}

func TestDo_ConcurrentSharedExecution(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		g := newGate(t, ctx, 100*time.Millisecond)

		var calls atomic.Int32
		expectedErr := errors.New("boom")

		block := make(chan struct{})

		fn := func(ctx context.Context) error {
			calls.Add(1)
			<-block
			return expectedErr
		}

		const n = 5
		results := make([]error, n)

		for i := range n {
			go func() {
				_, results[i] = g.Do(ctx, fn)
			}()
		}

		synctest.Wait()

		require.Equal(t, int32(1), calls.Load(), " expected single execution")
		close(block)
		synctest.Wait()

		for _, err := range results {
			require.ErrorIs(t, err, expectedErr)
		}
	})
}

func TestDo_WindowRespected(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		g := newGate(t, ctx, time.Second)

		var calls atomic.Int32

		fn := func(context.Context) error {
			calls.Add(1)
			return nil
		}

		const numberOfCalls = 16
		for range numberOfCalls {
			go func() {
				g.Do(ctx, fn)
			}()
		}

		synctest.Wait()

		require.Equal(t, int32(1), calls.Load(), " expected single execution within the window")
	})
}

func TestDo_WindowExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		g := newGate(t, ctx, time.Second)

		var calls atomic.Int32

		fn := func(context.Context) error {
			calls.Add(1)
			return nil
		}

		const numberOfCalls = 16
		for i := range numberOfCalls {
			go func() {
				g.Do(ctx, fn)
			}()

			if (i+1)%4 == 0 { // Every 4th call advance window
				time.Sleep(time.Second + time.Millisecond)
			}
		}

		require.Equal(t, int32(4), calls.Load(), "Expected 4 calls total across 4 windows")
	})
}

func TestDo_CallerCancellationWhileWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		g := newGate(t, ctx, time.Second)

		block := make(chan struct{})
		cctx, cancel := context.WithCancel(ctx)

		fn := func(ctx context.Context) error {
			<-block
			return nil
		}

		go func() {
			// first goroutine holds execution
			g.Do(ctx, fn)
		}()

		synctest.Wait()

		cancel()
		ran, err := g.Do(cctx, fn)
		close(block)

		require.ErrorContains(t, err, "context canceled")
		require.ErrorContains(t, err, "caller")
		require.False(t, ran)
	})
}

func TestDo_ParentCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parent, cancel := context.WithCancel(t.Context())
		g := newGate(t, parent, time.Second)

		block := make(chan struct{})

		fn := func(ctx context.Context) error {
			<-block
			return nil
		}

		errC := make(chan error)
		go func() {
			g.Do(t.Context(), fn)
		}()

		synctest.Wait()

		go func() {
			// this call should be blocked
			_, err := g.Do(t.Context(), func(context.Context) error { return nil })
			errC <- err
		}()

		synctest.Wait()

		cancel()
		close(block)

		err := <-errC
		require.Error(t, err)
		require.ErrorContains(t, err, "context canceled")
		require.ErrorContains(t, err, "parent")
	})
}
