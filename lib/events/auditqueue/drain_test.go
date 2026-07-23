/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package auditqueue

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDrain_DrainsMainQueue(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			q := newTestQueue(t, kind)

			const n = 25
			for i := int64(0); i < n; i++ {
				require.NoError(t, q.Enqueue(newTestEvent(i)))
			}

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			release := make(chan struct{})
			entered := make(chan struct{}, 1)
			var delivered atomic.Int64
			handler := func(_ context.Context, items []Item) []Item {
				select {
				case entered <- struct{}{}:
				default:
				}
				<-release
				delivered.Add(int64(len(items)))
				return items
			}
			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, handler) }()

			<-entered

			drainCtx, drainCancel := context.WithTimeout(ctx, testDefaultTimeout)
			t.Cleanup(drainCancel)

			drainDone := make(chan error, 1)
			go func() { drainDone <- q.Drain(drainCtx) }()

			select {
			case <-drainDone:
				t.Fatal("Drain returned before the queue was drained")
			case <-time.After(100 * time.Millisecond):
			}

			close(release)
			select {
			case err := <-drainDone:
				require.NoError(t, err)
			case <-time.After(testDefaultTimeout):
				t.Fatal("Drain did not return after the queue drained")
			}
			require.Equal(t, int64(n), delivered.Load())

			cancel()
			require.ErrorIs(t, <-runErr, context.Canceled)
		})
	}
}

func TestDrain_FlushesDeadLetter(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			q := newTestQueueWithConfig(t, kind, Config{
				MaxAttempts:             1,
				DeadLetterSweepInterval: time.Hour, // only Drain may trigger sweeps
			})

			require.NoError(t, q.Enqueue(newTestEvent(0)))

			var failing atomic.Bool
			failing.Store(true)
			var delivered atomic.Int64
			handler := func(_ context.Context, items []Item) []Item {
				if failing.Load() {
					return nil
				}
				delivered.Add(int64(len(items)))
				return items
			}

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, handler) }()

			// Wait for the event to exhaust its attempts and land in the
			// dead-letter queue, emptying the main queue.
			sq := underlyingQueue(t, q)
			require.Eventually(t, func() bool {
				dl, err := fetchDeadLetter(ctx, sq.db, 10)
				return err == nil && len(dl) == 1
			}, testDefaultTimeout, 10*time.Millisecond)

			drainCtx, drainCancel := context.WithTimeout(ctx, testDefaultTimeout)
			t.Cleanup(drainCancel)

			drainDone := make(chan error, 1)
			go func() { drainDone <- q.Drain(drainCtx) }()

			select {
			case <-drainDone:
				t.Fatal("Drain returned while dead-letter events were pending")
			case <-time.After(100 * time.Millisecond):
			}

			failing.Store(false)
			select {
			case err := <-drainDone:
				require.NoError(t, err)
			case <-time.After(testDefaultTimeout):
				t.Fatal("Drain did not return after the dead-letter queue drained")
			}
			require.Equal(t, int64(1), delivered.Load())

			cancel()
			require.ErrorIs(t, <-runErr, context.Canceled)
		})
	}
}

func TestDrain_RespectsContextDeadline(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			q := newTestQueue(t, kind)

			const n = 5
			for i := int64(0); i < n; i++ {
				require.NoError(t, q.Enqueue(newTestEvent(i)))
			}

			drainCtx, drainCancel := context.WithTimeout(ctx, 100*time.Millisecond)
			t.Cleanup(drainCancel)

			drainDone := make(chan error, 1)
			go func() { drainDone <- q.Drain(drainCtx) }()
			select {
			case err := <-drainDone:
				require.ErrorIs(t, err, context.DeadlineExceeded)
			case <-time.After(testDefaultTimeout):
				t.Fatal("Drain did not return after its deadline expired")
			}

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			var delivered atomic.Int64
			handler := func(_ context.Context, items []Item) []Item {
				delivered.Add(int64(len(items)))
				return items
			}
			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, handler) }()

			require.Eventually(t, func() bool {
				return delivered.Load() == int64(n)
			}, testDefaultTimeout, 10*time.Millisecond)

			cancel()
			require.ErrorIs(t, <-runErr, context.Canceled)
		})
	}
}
