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
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
)

const testDefaultTimeout = 2 * time.Second

var allKinds []Kind = []Kind{KindSQLite}

func newTestQueue(t *testing.T, kind Kind) Queue {
	t.Helper()
	return newTestQueueWithConfig(t, kind, Config{})
}

func newTestQueueWithConfig(t *testing.T, kind Kind, cfg Config) Queue {
	t.Helper()
	cfg.Path = filepath.Join(t.TempDir(), queueDir)
	q, err := New(kind, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, q.Close())
	})
	return q
}

func newTestEvent(index int64) apievents.AuditEvent {
	return &apievents.UserLogin{
		Metadata: apievents.Metadata{Index: index},
	}
}

func TestNew_UnknownKind(t *testing.T) {
	t.Parallel()

	q, err := New(Kind("INVALID_AUDIT_QUEUE_KIND"), Config{})
	require.Error(t, err)
	require.Nil(t, q)
}

func TestRun_RejectsConcurrentRun(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			q := newTestQueue(t, kind)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			require.NoError(t, q.Enqueue(ctx, newTestEvent(0)))

			ready := make(chan struct{})
			first := make(chan error, 1)
			go func() {
				blockingHandler := func(c context.Context, items []Item) []Item {
					close(ready)
					<-c.Done()
					return nil
				}
				first <- q.Run(ctx, blockingHandler)
			}()

			select {
			case <-ready:
			case <-time.After(time.Second):
				t.Fatal("first Run never reached handler")
			}

			noop := func(_ context.Context, items []Item) []Item { return items }
			require.ErrorIs(t, q.Run(ctx, noop), ErrAlreadyRunning)

			cancel()
			require.NoError(t, <-first)

		})
	}
}

func TestEnqueue_CanceledContext(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			q := newTestQueue(t, kind)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.Error(t, q.Enqueue(ctx, newTestEvent(0)))
		})
	}
}

func TestEnqueue_AfterClose(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), queueDir)
			q, err := New(kind, Config{Path: path})
			require.NoError(t, err)
			require.NoError(t, q.Close())
			require.ErrorIs(t, q.Enqueue(context.Background(), newTestEvent(0)), ErrClosed)
		})
	}
}

func TestRun_DeliversEvents(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			q := newTestQueue(t, kind)

			for i := int64(0); i < 3; i++ {
				require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
			}

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			delivered := make(chan []Item, 1)
			handler := func(_ context.Context, items []Item) []Item {
				select {
				case delivered <- items:
				default:
				}
				return items
			}

			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, handler) }()

			select {
			case items := <-delivered:
				require.NotEmpty(t, items)
			case <-time.After(testDefaultTimeout):
				t.Fatal("handler was never called")
			}

			cancel()
			require.NoError(t, <-runErr)
		})
	}
}

func TestRun_AcksDeliveredEvents(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			q := newTestQueue(t, kind)

			require.NoError(t, q.Enqueue(ctx, newTestEvent(0)))
			require.NoError(t, q.Enqueue(ctx, newTestEvent(1)))

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			// Ack only event 0. Event 1 should be retried.
			var calls atomic.Int32
			handler := func(_ context.Context, items []Item) []Item {
				if calls.Add(1) == 1 {
					var acked []Item
					for _, it := range items {
						if it.Event.GetIndex() == 0 {
							acked = append(acked, it)
						}
					}
					return acked
				}
				// On the second call, cancel so Run exits.
				cancel()
				return items
			}

			require.NoError(t, q.Run(runCtx, handler))
			require.GreaterOrEqual(t, calls.Load(), int32(2))
		})
	}
}

func TestRun_NormalOperation(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			q := newTestQueue(t, kind)

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			var acked atomic.Int32
			handler := func(_ context.Context, items []Item) []Item {
				acked.Add(int32(len(items)))
				return items
			}

			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, handler) }()

			const firstBatchCount = 300

			var wg sync.WaitGroup
			for i := int64(0); i < firstBatchCount; i++ {
				wg.Add(1)
				go func(i int64) {
					defer wg.Done()
					require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
				}(i)
			}
			wg.Wait()

			cond := func() bool {
				return acked.Load() == int32(firstBatchCount)
			}
			require.Eventually(t, cond, testDefaultTimeout, 10*time.Millisecond)

			const finalBatchCount = 900

			for i := int64(firstBatchCount); i < finalBatchCount; i++ {
				wg.Add(1)
				go func(i int64) {
					defer wg.Done()
					require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
				}(i)
			}
			wg.Wait()

			cond = func() bool {
				return acked.Load() == int32(finalBatchCount)
			}
			require.Eventually(t, cond, testDefaultTimeout, 10*time.Millisecond)

			require.Equal(t, int32(finalBatchCount), acked.Load())

			cancel()
			require.NoError(t, <-runErr)
		})
	}
}

func TestRun_StopsCleanlyOnContextCancel(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			q := newTestQueue(t, kind)

			ctx, cancel := context.WithCancel(context.Background())
			runErr := make(chan error, 1)
			go func() {
				runErr <- q.Run(ctx, func(_ context.Context, items []Item) []Item {
					return items
				})
			}()

			cancel()

			select {
			case err := <-runErr:
				require.NoError(t, err)
			case <-time.After(time.Second):
				t.Fatal("Run did not stop after context cancel")
			}
		})
	}
}

func TestRun_DeadLetter_ExhaustedEventLeavesMainQueue(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			const maxAttempts = 2
			q := newTestQueueWithConfig(t, kind, Config{
				MaxAttempts:             maxAttempts,
				DeadLetterSweepInterval: time.Hour, // prevent sweep from interfering
			})

			require.NoError(t, q.Enqueue(ctx, newTestEvent(0)))

			var calls atomic.Int32
			alwaysFail := func(_ context.Context, items []Item) []Item {
				calls.Add(int32(len(items)))
				return nil
			}

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, alwaysFail) }()

			// Wait until the handler has been called exactly maxAttempts times.
			require.Eventually(t, func() bool {
				return calls.Load() >= maxAttempts
			}, testDefaultTimeout, 10*time.Millisecond)

			// Give the run loop a few more cycles and confirm the count does not grow.
			time.Sleep(5 * pollInterval)
			require.Equal(t, int32(maxAttempts), calls.Load(),
				"handler should not be called again after event is promoted to dead-letter")

			cancel()
			require.NoError(t, <-runErr)
		})
	}
}

func TestRun_DeadLetter_RedeliversAfterRecovery(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			const sweepInterval = 50 * time.Millisecond
			q := newTestQueueWithConfig(t, kind, Config{
				MaxAttempts:             1,
				DeadLetterSweepInterval: sweepInterval,
			})

			require.NoError(t, q.Enqueue(ctx, newTestEvent(99)))

			var recovered atomic.Bool
			delivered := make(chan apievents.AuditEvent, 1)
			handler := func(_ context.Context, items []Item) []Item {
				if !recovered.Load() {
					return nil
				}

				// Once we are here, this means that the event has been moved to
				// the dead letter queue. Let's now deliver it.
				for _, it := range items {
					select {
					case delivered <- it.Event:
					default:
					}
				}
				return items
			}

			runCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)

			runErr := make(chan error, 1)
			go func() { runErr <- q.Run(runCtx, handler) }()

			// Wait long enough for the event to exhaust its attempts and land in
			// the dead-letter queue, then signal recovery.
			time.Sleep(5 * pollInterval)
			recovered.Store(true)

			select {
			case event := <-delivered:
				require.Equal(t, int64(99), event.GetIndex(),
					"dead-letter sweep should re-deliver the original event")
			case <-time.After(testDefaultTimeout):
				t.Fatal("dead-letter sweep never re-delivered the event after recovery")
			}

			cancel()
			require.NoError(t, <-runErr)
		})
	}
}
