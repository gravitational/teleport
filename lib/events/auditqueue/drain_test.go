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
	"os"
	"path/filepath"
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
				require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
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
			require.NoError(t, <-runErr)
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
				require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
			}

			drainCtx, drainCancel := context.WithTimeout(ctx, 100*time.Millisecond)
			t.Cleanup(drainCancel)

			drainDone := make(chan error, 1)
			go func() { drainDone <- q.Drain(drainCtx) }()
			select {
			case err := <-drainDone:
				require.NoError(t, err)
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
			require.NoError(t, <-runErr)
		})
	}
}

func TestDrain_StopsOrphanAdoption(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := t.Context()

	a := newQueueAt(t, filepath.Join(parent, "a"), time.Hour)
	for i := int64(0); i < 5; i++ {
		require.NoError(t, a.Enqueue(ctx, newTestEvent(i)))
	}
	require.NoError(t, a.Close())

	b := newQueueAt(t, filepath.Join(parent, "b"), 20*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.Drain(ctx))

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	var delivered atomic.Int64
	handler := func(_ context.Context, items []Item) []Item {
		delivered.Add(int64(len(items)))
		return items
	}
	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	require.Never(t, func() bool {
		_, err := os.Stat(filepath.Join(parent, "a"))
		return os.IsNotExist(err)
	}, 300*time.Millisecond, 30*time.Millisecond, "orphan A must not be adopted after Drain")
	require.Zero(t, delivered.Load())

	cancel()
	require.NoError(t, <-runErr)
}
