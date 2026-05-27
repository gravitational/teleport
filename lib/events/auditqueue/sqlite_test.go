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
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

const queueDir = "test-queue"

func newSqliteTestQueue(t *testing.T) *sqliteQueue {
	t.Helper()
	path := filepath.Join(t.TempDir(), queueDir)
	q, err := newSQLiteQueue(Config{Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Close()) })
	return q
}

func newQueueAt(t *testing.T, path string, scanInterval time.Duration) *sqliteQueue {
	t.Helper()
	q, err := newSQLiteQueue(Config{
		Path:               path,
		OrphanScanInterval: scanInterval,
	})
	require.NoError(t, err)
	return q
}

func histogramSampleCount(t *testing.T, h prometheus.Histogram) uint64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, h.Write(&m))
	require.NotNil(t, m.Histogram)
	return m.Histogram.GetSampleCount()
}

func TestEnqueueDequeue_FIFO(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
	}

	got, err := q.fetch(3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i, item := range got {
		require.Equal(t, int64(i), item.Event.GetIndex())
	}
	require.NoError(t, q.ack(got))
}

func TestDequeue_RespectsLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 5; i++ {
		require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
	}

	first, err := q.fetch(2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NoError(t, q.ack(first))

	rest, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, rest, 3)
	require.NoError(t, q.ack(rest))
}

func TestDequeue_EmptyReturnsImmediately(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestDequeue_QueueClosed(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	require.NoError(t, q.Close())

	items, err := q.fetch(1)
	require.Error(t, err)
	require.Empty(t, items)
}

func TestDequeue_WithoutAckRetainsEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	require.NoError(t, q.Enqueue(ctx, newTestEvent(42)))

	first, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, int64(42), first[0].Event.GetIndex())

	second, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, first[0].ID, second[0].ID)

	require.NoError(t, q.ack(second))

	third, err := q.fetch(1)
	require.NoError(t, err)
	require.Empty(t, third)
}

func TestRun_DeliversAndAcks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	var got []Item
	var mu sync.Mutex
	handler := func(_ context.Context, items []Item) []Item {
		mu.Lock()
		got = append(got, items...)
		mu.Unlock()
		return items
	}

	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(runCtx, handler) }()

	require.Eventually(t, func() bool {
		items, err := q.fetch(10)
		return err == nil && len(items) == 0
	}, time.Second, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-runErr)

	mu.Lock()
	require.Len(t, got, 3)
	for i, item := range got {
		require.Equal(t, int64(i), item.Event.GetIndex())
	}
	mu.Unlock()
}

func TestRun_HandlerSubsetIsAcked(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	var once sync.Once
	handler := func(_ context.Context, items []Item) []Item {
		once.Do(cancel)
		var ack []Item
		for _, it := range items {
			if it.Event.GetIndex()%2 == 0 {
				ack = append(ack, it)
			}
		}
		return ack
	}

	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(runCtx, handler) }()

	require.NoError(t, <-runErr)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), items[0].Event.GetIndex())
}

func TestClose(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), queueDir)
	q, err := newSQLiteQueue(Config{Path: path})
	require.NoError(t, err)
	require.NoError(t, q.Close())
}

func TestNew_ReturnsQueue(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), queueDir)
	q, err := New(KindSQLiteDisk, Config{Path: path})
	require.NoError(t, err)
	require.NotNil(t, q)
	t.Cleanup(func() { require.NoError(t, q.Close()) })
}

func TestNewSQLiteQueue_RequiresPath(t *testing.T) {
	t.Parallel()

	q, err := newSQLiteQueue(Config{})
	require.Error(t, err)
	require.Nil(t, q)
}

func TestEnqueue_ConcurrentCallersAllSucceed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	const N = 100
	var wg sync.WaitGroup
	errs := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = q.Enqueue(ctx, newTestEvent(int64(i)))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "Enqueue %d failed", i)
	}

	items, err := q.fetch(N + 10)
	require.NoError(t, err)
	require.Len(t, items, N)
}

func TestEnqueue_FIFOWithinSingleProducer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	const G = 10
	const Per = 5
	var wg sync.WaitGroup
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < Per; i++ {
				idx := int64(g*Per + i)
				require.NoError(t, q.Enqueue(ctx, newTestEvent(idx)))
			}
		}()
	}
	wg.Wait()

	items, err := q.fetch(G*Per + 10)
	require.NoError(t, err)
	require.Len(t, items, G*Per)

	perProducer := make(map[int][]int64)
	for _, item := range items {
		idx := item.Event.GetIndex()
		g := int(idx) / Per
		perProducer[g] = append(perProducer[g], idx)
	}
	for g, indexes := range perProducer {
		require.Len(t, indexes, Per, "producer %d had wrong number of events", g)
		for i, idx := range indexes {
			require.Equal(t, int64(g*Per+i), idx,
				"producer %d position %d: events out of order", g, i)
		}
	}
}

func TestEnqueue_VisibleImmediatelyAfterReturn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	require.NoError(t, q.Enqueue(ctx, newTestEvent(42)))

	items, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(42), items[0].Event.GetIndex())
}

func TestClose_PendingEnqueuesReturn(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), queueDir)
	q, err := newSQLiteQueue(Config{Path: path})
	require.NoError(t, err)

	const N = 50
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Enqueue(context.Background(), newTestEvent(int64(i)))
		}()
	}

	require.NoError(t, q.Close())

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Enqueue calls did not return after Close")
	}
}

func TestBatchSizeMetricRecorded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	before := histogramSampleCount(t, batchSize)

	require.NoError(t, q.Enqueue(ctx, newTestEvent(0)))

	after := histogramSampleCount(t, batchSize)
	require.Greater(t, after, before,
		"audit_queue_batch_size did not record the committed batch")
}

func TestTeleportInfoTable(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	var version string
	require.NoError(t, q.db.QueryRow(
		"SELECT value FROM teleport_info WHERE key = 'teleport_version'",
	).Scan(&version))
	require.Equal(t, teleport.Version, version)
}

func TestAboveSoftLimit_TripsAndUntrips(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	q, err := newSQLiteQueue(Config{
		Path:      filepath.Join(t.TempDir(), "queue"),
		SoftLimit: 16 * 1024,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	above, size, err := q.aboveSoftLimit()
	require.NoError(t, err)
	require.False(t, above, "fresh queue (size %d) should not exceed soft limit", size)

	for i := 0; i < 200; i++ {
		require.NoError(t, q.Enqueue(ctx, newTestEvent(int64(i))))
	}
	_, err = q.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	require.NoError(t, err)

	above, size, err = q.aboveSoftLimit()
	require.NoError(t, err)
	require.True(t, above, "after inserts queue.db size %d should exceed soft limit %d", size, q.softLimit)
}

func TestEnqueue_FullReturnsErrQueueFull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	q, err := newSQLiteQueue(Config{
		Path:     filepath.Join(t.TempDir(), "queue"),
		MaxBytes: 8 * int64(os.Getpagesize()),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	var got error
	for i := 0; i < 10000; i++ {
		got = q.Enqueue(ctx, newTestEvent(int64(i)))
		if got != nil {
			break
		}
	}
	require.Error(t, got, "expected an Enqueue to fail once max_page_count is hit")
	require.ErrorIs(t, got, ErrQueueFull,
		"Enqueue should map SQLITE_FULL to ErrQueueFull; got %v", got)
}

func TestOrphanAdoption_DrainsAndDeletes(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := context.Background()

	a := newQueueAt(t, filepath.Join(parent, "a"), time.Hour)
	for i := int64(0); i < 5; i++ {
		require.NoError(t, a.Enqueue(ctx, newTestEvent(i)))
	}
	require.NoError(t, a.Close())

	b := newQueueAt(t, filepath.Join(parent, "b"), 50*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	var (
		mu  sync.Mutex
		got []int64
	)
	handler := func(_ context.Context, items []Item) []Item {
		mu.Lock()
		for _, it := range items {
			got = append(got, it.Event.GetIndex())
		}
		mu.Unlock()
		return items
	}

	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(parent, "a"))
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "expected orphan A's directory to be removed")

	cancel()
	require.NoError(t, <-runErr)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, got, 5, "expected all 5 orphan events delivered through B's handler")
}

func TestOrphanAdoption_MigratesDeadLetter(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := context.Background()

	aPath := filepath.Join(parent, "a")
	a, err := newSQLiteQueue(Config{
		Path:                    aPath,
		MaxAttempts:             1,
		DeadLetterSweepInterval: time.Hour,
	})
	require.NoError(t, err)
	require.NoError(t, a.Enqueue(ctx, newTestEvent(42)))

	runCtx, cancelA := context.WithCancel(ctx)
	runErrA := make(chan error, 1)
	go func() { runErrA <- a.Run(runCtx, func(context.Context, []Item) []Item { return nil }) }()

	require.Eventually(t, func() bool {
		dl, err := a.fetchDeadLetter(10)
		return err == nil && len(dl) == 1
	}, 5*time.Second, 50*time.Millisecond, "expected A's event to land in its dead-letter queue")

	cancelA()
	require.NoError(t, <-runErrA)
	require.NoError(t, a.Close())

	_, err = os.Stat(aPath)
	require.NoError(t, err, "expected A's directory to remain after Close due to non-empty dead-letter")

	b, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(parent, "b"),
		OrphanScanInterval:      50 * time.Millisecond,
		DeadLetterSweepInterval: time.Hour, // prevent B from re-delivering during the test
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	bRunCtx, cancelB := context.WithCancel(ctx)
	t.Cleanup(cancelB)
	bRunErr := make(chan error, 1)
	go func() {
		bRunErr <- b.Run(bRunCtx, func(_ context.Context, items []Item) []Item { return items })
	}()

	require.Eventually(t, func() bool {
		dl, err := b.fetchDeadLetter(10)
		return err == nil && len(dl) == 1 && dl[0].Event.GetIndex() == 42
	}, 5*time.Second, 50*time.Millisecond, "expected B to migrate A's dead-letter row")

	require.Eventually(t, func() bool {
		_, err := os.Stat(aPath)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "expected A's directory to be removed after migration")

	cancelB()
	require.NoError(t, <-bRunErr)
}

func TestOrphanAdoption_SkipsLockedQueue(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := context.Background()

	a := newQueueAt(t, filepath.Join(parent, "a"), time.Hour)
	t.Cleanup(func() { _ = a.Close() })
	b := newQueueAt(t, filepath.Join(parent, "b"), 50*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, a.Enqueue(ctx, newTestEvent(0)))

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	var got int32
	handler := func(_ context.Context, items []Item) []Item {
		atomic.AddInt32(&got, int32(len(items)))
		return items
	}

	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	time.Sleep(300 * time.Millisecond)

	_, err := os.Stat(filepath.Join(parent, "a"))
	require.NoError(t, err, "A's directory should still exist while A holds its flock")
	require.Equal(t, int32(0), atomic.LoadInt32(&got),
		"B should not have delivered A's events while A is locked")

	cancel()
	require.NoError(t, <-runErr)
}

func TestOrphanAdoption_SkipsTmpSuffix(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()

	garbagePath := filepath.Join(parent, "garbage.tmp")
	require.NoError(t, os.MkdirAll(garbagePath, 0o700))

	b := newQueueAt(t, filepath.Join(parent, "b"), 50*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler := func(_ context.Context, items []Item) []Item { return items }
	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	time.Sleep(300 * time.Millisecond)

	_, err := os.Stat(garbagePath)
	require.NoError(t, err, "recent .tmp/ directory should not be adopted or swept")

	cancel()
	require.NoError(t, <-runErr)
}

func TestOrphanAdoption_StaleTmpSwept(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()

	oldPath := filepath.Join(parent, "old.tmp")
	require.NoError(t, os.MkdirAll(oldPath, 0o700))
	backdate := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldPath, backdate, backdate))

	b := newQueueAt(t, filepath.Join(parent, "b"), 50*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	runCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	handler := func(_ context.Context, items []Item) []Item { return items }
	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	require.Eventually(t, func() bool {
		_, err := os.Stat(oldPath)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "stale .tmp/ directory should be swept")

	cancel()
	require.NoError(t, <-runErr)
}

func TestAckDB_DeletesOnlyAckedItems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 5; i++ {
		require.NoError(t, q.Enqueue(ctx, newTestEvent(i)))
	}

	// Fetch 4 of the 5 items and ack them.
	items, err := q.fetch(4)
	require.NoError(t, err)
	require.Len(t, items, 4)

	ackedIDs := make(map[int64]struct{}, len(items))
	for _, item := range items {
		ackedIDs[item.ID] = struct{}{}
	}

	require.NoError(t, ackDB(ctx, q.db, items))

	// Only the one un-acked item should remain.
	remaining, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	_, wasAcked := ackedIDs[remaining[0].ID]
	require.False(t, wasAcked, "remaining item should not be one that was acked")
}

func TestIsSQLiteFullError(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", getDSN(dbPath, 10*int64(os.Getpagesize())))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Shouldn't cause an error.
	_, err = db.Exec(`CREATE TABLE test_table (v BLOB NOT NULL)`)
	require.NoError(t, err)
	require.False(t, isSQLiteFullError(nil))

	// Should make an SQLITE_ERROR, not SQLITE_FULL.
	_, err = db.Exec(`INSERT INTO nonexistent VALUES (1)`)
	require.Error(t, err)
	require.False(t, isSQLiteFullError(err))

	// Should make an SQLITE_FULL error once database fills up.
	payload := make([]byte, 4096)
	var fullErr error
	for i := 0; i < 10000; i++ {
		_, fullErr = db.Exec(`INSERT INTO test_table VALUES (?)`, payload)
		if fullErr != nil {
			break
		}
	}
	require.Error(t, fullErr, "expected an insert to fail with SQLITE_FULL")
	require.True(t, isSQLiteFullError(fullErr))
}

func TestIncrementAttempts_ReturnsExhausted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	q, err := newSQLiteQueue(Config{
		Path:        filepath.Join(t.TempDir(), queueDir),
		MaxAttempts: 3,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(ctx, newTestEvent(0)))
	require.NoError(t, q.Enqueue(ctx, newTestEvent(1)))

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 2)

	// First two increments should not exhaust either item.
	for i := 0; i < 2; i++ {
		exhausted, err := q.incrementAttempts(items)
		require.NoError(t, err)
		require.Empty(t, exhausted, "should not be exhausted after attempt %d", i+1)
	}

	// Third increment reaches max_attempts. Both items should be exhausted.
	exhausted, err := q.incrementAttempts(items)
	require.NoError(t, err)
	require.Len(t, exhausted, 2)
}

func TestRetry_ExhaustedMovesToDeadLetter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const maxAttempts = 3
	q, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(t.TempDir(), queueDir),
		MaxAttempts:             maxAttempts,
		DeadLetterSweepInterval: time.Hour, // don't interfere with this test
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(ctx, newTestEvent(42)))

	alwaysFail := func(_ context.Context, items []Item) []Item { return nil }

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(runCtx, alwaysFail) }()

	// Wait until audit_queue is empty, i.e., item promoted to dead-letter.
	require.Eventually(t, func() bool {
		items, err := q.fetch(1)
		return err == nil && len(items) == 0
	}, 10*time.Second, 50*time.Millisecond, "expected item to be promoted out of main queue")

	// Confirm it is in the dead-letter table.
	dlItems, err := q.fetchDeadLetter(10)
	require.NoError(t, err)
	require.Len(t, dlItems, 1)
	require.Equal(t, int64(42), dlItems[0].Event.GetIndex())

	cancel()
	require.NoError(t, <-runErr)
}

func TestDeadLetterSweep_RedeliversOnRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	sweepInterval := 50 * time.Millisecond
	q, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(t.TempDir(), queueDir),
		MaxAttempts:             1,
		DeadLetterSweepInterval: sweepInterval,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(ctx, newTestEvent(7)))

	var recovered atomic.Bool
	handler := func(_ context.Context, items []Item) []Item {
		if recovered.Load() {
			return items // succeed on re-delivery
		}
		return nil // fail initially
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(runCtx, handler) }()

	// Wait for item to land in dead-letter.
	require.Eventually(t, func() bool {
		items, _ := q.fetchDeadLetter(10)
		return len(items) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// Signal recovery. Sweep should redeliver and ack it.
	recovered.Store(true)

	require.Eventually(t, func() bool {
		items, _ := q.fetchDeadLetter(10)
		return len(items) == 0
	}, 5*time.Second, sweepInterval)

	cancel()
	require.NoError(t, <-runErr)
}

func TestDeadLetterTTL_ExpiresOldRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	q, err := newSQLiteQueue(Config{
		Path:          filepath.Join(t.TempDir(), queueDir),
		DeadLetterTTL: time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	// Enqueue one event, then move it to dead-letter directly so we can
	// control its failed_at timestamp.
	require.NoError(t, q.Enqueue(ctx, newTestEvent(99)))
	items, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.NoError(t, q.moveToDeadLetter(items))

	// Back-date the row to simulate an entry older than the TTL.
	pastTimestamp := time.Now().Add(-2 * time.Hour).Unix()
	_, err = q.db.Exec("UPDATE audit_dead_letter SET failed_at = ?", pastTimestamp)
	require.NoError(t, err)

	dlItems, err := q.fetchDeadLetter(10)
	require.NoError(t, err)
	require.Len(t, dlItems, 1, "row should exist before expiry")

	q.expireDeadLetter()

	dlItems, err = q.fetchDeadLetter(10)
	require.NoError(t, err)
	require.Empty(t, dlItems, "row should have been deleted by expireDeadLetter")
}
