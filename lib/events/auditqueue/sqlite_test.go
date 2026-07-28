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
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
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

func fetchDeadLetter(ctx context.Context, db *sql.DB, limit int) ([]Item, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, payload FROM audit_dead_letter ORDER BY id ASC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	items, _, err := scanItems(rows)
	return items, err
}

func TestEnqueueDequeue_FIFO(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
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
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 5; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
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
	q := newSqliteTestQueue(t)

	require.NoError(t, q.Enqueue(newTestEvent(42)))

	first, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, int64(42), first[0].Event.GetIndex())

	second, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, first[0].id, second[0].id)

	require.NoError(t, q.ack(second))

	third, err := q.fetch(1)
	require.NoError(t, err)
	require.Empty(t, third)
}

func TestRun_DeliversAndAcks(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
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
	require.ErrorIs(t, <-runErr, context.Canceled)

	mu.Lock()
	require.Len(t, got, 3)
	for i, item := range got {
		require.Equal(t, int64(i), item.Event.GetIndex())
	}
	mu.Unlock()
}

func TestRun_HandlerSubsetIsAcked(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	handler := func(_ context.Context, items []Item) []Item {
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

	require.Eventually(t, func() bool {
		items, err := q.fetch(10)
		if err != nil || len(items) != 1 {
			return false
		}
		return items[0].Event.GetIndex() == 1
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
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
	q := newSqliteTestQueue(t)

	const N = 100
	var wg sync.WaitGroup
	errs := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = q.Enqueue(newTestEvent(int64(i)))
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
				require.NoError(t, q.Enqueue(newTestEvent(idx)))
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
	q := newSqliteTestQueue(t)

	require.NoError(t, q.Enqueue(newTestEvent(42)))

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
			_ = q.Enqueue(newTestEvent(int64(i)))
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
	q := newSqliteTestQueue(t)

	before := histogramSampleCount(t, batchSize)

	require.NoError(t, q.Enqueue(newTestEvent(0)))

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
		require.NoError(t, q.Enqueue(newTestEvent(int64(i))))
	}
	_, err = q.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	require.NoError(t, err)

	above, size, err = q.aboveSoftLimit()
	require.NoError(t, err)
	require.True(t, above, "after inserts queue.db size %d should exceed soft limit %d", size, q.softLimit)
}

func TestEnqueue_FullReturnsErrQueueFull(t *testing.T) {
	t.Parallel()

	q, err := newSQLiteQueue(Config{
		Path:     filepath.Join(t.TempDir(), "queue"),
		MaxBytes: 50 * sqlitePageSize,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	var got error
	for i := 0; i < 10000; i++ {
		got = q.Enqueue(newTestEvent(int64(i)))
		if got != nil {
			break
		}
	}
	require.Error(t, got, "expected an Enqueue to fail once max_page_count is hit")
	require.ErrorIs(t, got, ErrQueueFull,
		"Enqueue should map SQLITE_FULL to ErrQueueFull; got %v", got)
}

func TestEnqueue_FileSizeStaysWithinMaxBytes(t *testing.T) {
	t.Parallel()

	const maxBytes = 50 * sqlitePageSize
	path := filepath.Join(t.TempDir(), "queue")
	q, err := newSQLiteQueue(Config{
		Path:     path,
		MaxBytes: maxBytes,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	for i := range 10000 {
		if err := q.Enqueue(newTestEvent(int64(i))); err != nil {
			require.ErrorIs(t, err, ErrQueueFull)
			break
		}
	}

	info, err := os.Stat(filepath.Join(path, queueDBFile))
	require.NoError(t, err)
	require.LessOrEqual(t, info.Size(), int64(maxBytes),
		"queue.db size %d exceeded MaxBytes %d", info.Size(), maxBytes)
}

func TestOrphanAdoption_DrainsAndDeletes(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := t.Context()

	a := newQueueAt(t, filepath.Join(parent, "a"), time.Hour)
	for i := int64(0); i < 5; i++ {
		require.NoError(t, a.Enqueue(newTestEvent(i)))
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

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(got) == 5
	}, 5*time.Second, 50*time.Millisecond, "expected all 5 orphan events delivered through B's handler")

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestOrphanAdoption_MigratesDeadLetter(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := t.Context()

	aPath := filepath.Join(parent, "a")
	a, err := newSQLiteQueue(Config{
		Path:                    aPath,
		MaxAttempts:             1,
		DeadLetterSweepInterval: time.Hour,
	})
	require.NoError(t, err)
	require.NoError(t, a.Enqueue(newTestEvent(42)))

	runCtx, cancelA := context.WithCancel(ctx)
	runErrA := make(chan error, 1)
	go func() { runErrA <- a.Run(runCtx, func(context.Context, []Item) []Item { return nil }) }()

	require.Eventually(t, func() bool {
		dl, err := fetchDeadLetter(a.ctx, a.db, 10)
		return err == nil && len(dl) == 1
	}, 5*time.Second, 50*time.Millisecond, "expected A's event to land in its dead-letter queue")

	cancelA()
	require.ErrorIs(t, <-runErrA, context.Canceled)
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
		dl, err := fetchDeadLetter(b.ctx, b.db, 10)
		return err == nil && len(dl) == 1 && dl[0].Event.GetIndex() == 42
	}, 5*time.Second, 50*time.Millisecond, "expected B to migrate A's dead-letter row")

	require.Eventually(t, func() bool {
		_, err := os.Stat(aPath)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "expected A's directory to be removed after migration")

	cancelB()
	require.ErrorIs(t, <-bRunErr, context.Canceled)
}

func TestOrphanAdoption_PromotesFailedToDeadLetter(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := t.Context()

	aPath := filepath.Join(parent, "a")
	a, err := newSQLiteQueue(Config{Path: aPath})
	require.NoError(t, err)
	require.NoError(t, a.Enqueue(newTestEvent(42)))
	require.NoError(t, a.Close())

	_, err = os.Stat(aPath)
	require.NoError(t, err, "expected A's directory to remain after Close due to non-empty queue")

	b, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(parent, "b"),
		OrphanScanInterval:      50 * time.Millisecond,
		MaxAttempts:             1,
		DeadLetterSweepInterval: time.Hour, // prevent B from re-delivering during the test
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	runErr := make(chan error, 1)
	go func() {
		runErr <- b.Run(runCtx, func(context.Context, []Item) []Item { return nil })
	}()

	require.Eventually(t, func() bool {
		dl, err := fetchDeadLetter(b.ctx, b.db, 10)
		return err == nil && len(dl) == 1 && dl[0].Event.GetIndex() == 42
	}, 5*time.Second, 50*time.Millisecond,
		"expected A's failing event to be promoted and migrated into B's dead-letter queue")

	require.Eventually(t, func() bool {
		_, err := os.Stat(aPath)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "expected A's directory to be removed after promotion and migration")

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestMigrateOrphanQueue_PreservesAttempts(t *testing.T) {
	t.Parallel()

	a, err := newSQLiteQueue(Config{
		Path:        filepath.Join(t.TempDir(), "a"),
		MaxAttempts: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	require.NoError(t, a.Enqueue(newTestEvent(42)))

	items, err := a.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	for range 2 {
		promoted, err := a.processFailedDeliveries(t.Context(), items)
		require.NoError(t, err)
		require.Equal(t, 0, promoted)
	}

	b, err := newSQLiteQueue(Config{Path: filepath.Join(t.TempDir(), "b")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.migrateOrphanQueue(t.Context(), a.db, "a"))

	empty, err := isQueueEmpty(a.db)
	require.NoError(t, err)
	require.True(t, empty, "A's audit_queue should be empty after migration")

	migrated, err := b.fetch(10)
	require.NoError(t, err)
	require.Len(t, migrated, 1)
	require.Equal(t, int64(42), migrated[0].Event.GetIndex())

	var attempts int
	require.NoError(t, b.db.QueryRow("SELECT attempts FROM audit_queue").Scan(&attempts))
	require.Equal(t, 2, attempts, "attempt count should carry over during migration")
}

func TestMigrateOrphanQueue_WatermarkPreventsDuplicates(t *testing.T) {
	t.Parallel()

	a, err := newSQLiteQueue(Config{Path: filepath.Join(t.TempDir(), "a")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	for i := int64(0); i < 3; i++ {
		require.NoError(t, a.Enqueue(newTestEvent(i)))
	}

	original, err := fetchOrphanRows(t.Context(), a.db,
		"SELECT id, payload, attempts FROM audit_queue WHERE id > ? ORDER BY id ASC LIMIT ?", 0, 10)
	require.NoError(t, err)
	require.Len(t, original, 3)

	b, err := newSQLiteQueue(Config{Path: filepath.Join(t.TempDir(), "b")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.migrateOrphanQueue(t.Context(), a.db, "orphan-a"))

	// Restore the rows with their original ids, as if the orphan-side
	// deletes had never committed.
	for _, r := range original {
		args := append([]any{r.id}, r.values...)
		_, err := a.db.Exec("INSERT INTO audit_queue (id, payload, attempts) VALUES (?, ?, ?)", args...)
		require.NoError(t, err)
	}

	require.NoError(t, b.migrateOrphanQueue(t.Context(), a.db, "orphan-a"))

	var count int
	require.NoError(t, b.db.QueryRow("SELECT COUNT(*) FROM audit_queue").Scan(&count))
	require.Equal(t, 3, count, "watermark should prevent re-migrating already-copied rows")
}

func TestMigrateOrphanDB_RoundTrip(t *testing.T) {
	t.Parallel()

	a, err := newSQLiteQueue(Config{Path: filepath.Join(t.TempDir(), "a")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })
	for i := int64(0); i < 5; i++ {
		require.NoError(t, a.Enqueue(newTestEvent(i)))
	}

	// Give every row a distinct nonzero aux value so a column swap or row
	// mix-up during migration cannot compare equal.
	_, err = a.db.Exec("UPDATE audit_queue SET attempts = id * 3")
	require.NoError(t, err)
	_, err = a.db.Exec(
		"INSERT INTO audit_dead_letter (payload, failed_at) SELECT payload, id * 1000 FROM audit_queue WHERE id > 3")
	require.NoError(t, err)
	_, err = a.db.Exec("DELETE FROM audit_queue WHERE id > 3")
	require.NoError(t, err)

	const selectQueue = "SELECT id, payload, attempts FROM audit_queue WHERE id > ? ORDER BY id ASC LIMIT ?"
	const selectDeadLetter = "SELECT id, payload, failed_at FROM audit_dead_letter WHERE id > ? ORDER BY id ASC LIMIT ?"

	wantQueue, err := fetchOrphanRows(t.Context(), a.db, selectQueue, 0, 100)
	require.NoError(t, err)
	require.Len(t, wantQueue, 3)
	wantDeadLetter, err := fetchOrphanRows(t.Context(), a.db, selectDeadLetter, 0, 100)
	require.NoError(t, err)
	require.Len(t, wantDeadLetter, 2)

	b, err := newSQLiteQueue(Config{Path: filepath.Join(t.TempDir(), "b")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.migrateOrphanDB(t.Context(), a.db, "orphan-a"))

	gotQueue, err := fetchOrphanRows(t.Context(), b.db, selectQueue, 0, 100)
	require.NoError(t, err)
	gotDeadLetter, err := fetchOrphanRows(t.Context(), b.db, selectDeadLetter, 0, 100)
	require.NoError(t, err)

	values := func(rows []migratedRow) [][]any {
		out := make([][]any, len(rows))
		for i, r := range rows {
			out[i] = r.values
		}
		return out
	}
	require.Equal(t, values(wantQueue), values(gotQueue),
		"audit_queue payloads and attempts should round-trip byte-for-byte")
	require.Equal(t, values(wantDeadLetter), values(gotDeadLetter),
		"audit_dead_letter payloads and failed_at should round-trip byte-for-byte")

	items, err := b.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 3)
	for i, item := range items {
		require.Equal(t, int64(i), item.Event.GetIndex(),
			"migrated payloads should still unmarshal to the original events in order")
	}
}

func TestOrphanAdoption_SkipsLockedQueue(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := t.Context()

	a := newQueueAt(t, filepath.Join(parent, "a"), time.Hour)
	t.Cleanup(func() { _ = a.Close() })
	b := newQueueAt(t, filepath.Join(parent, "b"), 50*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, a.Enqueue(newTestEvent(0)))

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	var got int32
	handler := func(_ context.Context, items []Item) []Item {
		atomic.AddInt32(&got, int32(len(items)))
		return items
	}

	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	_, err := os.Stat(filepath.Join(parent, "a"))
	require.NoError(t, err, "A's directory should still exist while A holds its flock")
	require.Equal(t, int32(0), atomic.LoadInt32(&got),
		"B should not have delivered A's events while A is locked")

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestOrphanAdoption_SkipsTmpSuffix(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()

	garbagePath := filepath.Join(parent, "garbage.tmp")
	require.NoError(t, os.MkdirAll(garbagePath, 0o700))

	b := newQueueAt(t, filepath.Join(parent, "b"), 50*time.Millisecond)
	t.Cleanup(func() { _ = b.Close() })

	runCtx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	handler := func(_ context.Context, items []Item) []Item { return items }
	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	_, err := os.Stat(garbagePath)
	require.NoError(t, err, "recent .tmp/ directory should not be adopted or swept")

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
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

	runCtx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	handler := func(_ context.Context, items []Item) []Item { return items }
	runErr := make(chan error, 1)
	go func() { runErr <- b.Run(runCtx, handler) }()

	require.Eventually(t, func() bool {
		_, err := os.Stat(oldPath)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "stale .tmp/ directory should be swept")

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestAckDB_DeletesOnlyAckedItems(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	q := newSqliteTestQueue(t)

	for i := int64(0); i < 5; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
	}

	// Fetch 4 of the 5 items and ack them.
	items, err := q.fetch(4)
	require.NoError(t, err)
	require.Len(t, items, 4)

	ackedIDs := make(map[int64]struct{}, len(items))
	for _, item := range items {
		ackedIDs[item.id] = struct{}{}
	}

	require.NoError(t, ackDB(ctx, q.db, items))

	// Only the one un-acked item should remain.
	remaining, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	_, wasAcked := ackedIDs[remaining[0].id]
	require.False(t, wasAcked, "remaining item should not be one that was acked")
}

func TestIsSQLiteFullError(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", getDSN(dbPath, 50*sqlitePageSize, SynchronousNormal))
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

func TestPlaceholders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		n    int
		want string
	}{
		{n: -1, want: ""},
		{n: 0, want: ""},
		{n: 1, want: "?"},
		{n: 2, want: "?,?"},
		{n: 3, want: "?,?,?"},
		{n: 5, want: "?,?,?,?,?"},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, placeholders(tc.n), "placeholders(%d)", tc.n)
	}
}

func TestProcessFailedDelivery_PromotesExhausted(t *testing.T) {
	t.Parallel()

	q, err := newSQLiteQueue(Config{
		Path:        filepath.Join(t.TempDir(), queueDir),
		MaxAttempts: 3,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(newTestEvent(0)))
	require.NoError(t, q.Enqueue(newTestEvent(1)))

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 2)

	for i := 0; i < 2; i++ {
		promoted, err := q.processFailedDeliveries(t.Context(), items)
		require.NoError(t, err)
		require.Equal(t, 0, promoted, "should not be exhausted after attempt %d", i+1)
		remaining, err := q.fetch(10)
		require.NoError(t, err)
		require.Len(t, remaining, 2, "items should still be in audit_queue after attempt %d", i+1)
	}

	promoted, err := q.processFailedDeliveries(t.Context(), items)
	require.NoError(t, err)
	require.Equal(t, 2, promoted)

	remaining, err := q.fetch(10)
	require.NoError(t, err)
	require.Empty(t, remaining, "audit_queue should be empty after promotion")

	dlItems, err := fetchDeadLetter(q.ctx, q.db, 10)
	require.NoError(t, err)
	require.Len(t, dlItems, 2)
}

func TestRetry_ExhaustedMovesToDeadLetter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const maxAttempts = 3
	q, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(t.TempDir(), queueDir),
		MaxAttempts:             maxAttempts,
		DeadLetterSweepInterval: time.Hour, // don't interfere with this test
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(newTestEvent(42)))

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
	dlItems, err := fetchDeadLetter(q.ctx, q.db, 10)
	require.NoError(t, err)
	require.Len(t, dlItems, 1)
	require.Equal(t, int64(42), dlItems[0].Event.GetIndex())

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestDeadLetterSweep_RedeliversOnRecovery(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	sweepInterval := 50 * time.Millisecond
	q, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(t.TempDir(), queueDir),
		MaxAttempts:             1,
		DeadLetterSweepInterval: sweepInterval,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(newTestEvent(7)))

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
		items, _ := fetchDeadLetter(q.ctx, q.db, 10)
		return len(items) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// Signal recovery. Sweep should redeliver and ack it.
	recovered.Store(true)

	require.Eventually(t, func() bool {
		items, _ := fetchDeadLetter(q.ctx, q.db, 10)
		return len(items) == 0
	}, 5*time.Second, sweepInterval)

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestDeadLetterSweep_KickedOnDeliveryRecovery(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	q, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(t.TempDir(), queueDir),
		MaxAttempts:             1,
		DeadLetterSweepInterval: time.Hour, // only the recovery kick can trigger a sweep
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	require.NoError(t, q.Enqueue(newTestEvent(1)))

	var recovered atomic.Bool
	handler := func(_ context.Context, items []Item) []Item {
		if recovered.Load() {
			return items
		}
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	runErr := make(chan error, 1)
	go func() { runErr <- q.Run(runCtx, handler) }()

	// Wait for the first event to fail its delivery and land in dead-letter.
	require.Eventually(t, func() bool {
		items, _ := fetchDeadLetter(q.ctx, q.db, 10)
		return len(items) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// Recover, then enqueue a fresh event. Its successful delivery marks the
	// failing -> succeeding transition, which should kick an immediate sweep.
	recovered.Store(true)
	require.NoError(t, q.Enqueue(newTestEvent(2)))

	require.Eventually(t, func() bool {
		items, _ := fetchDeadLetter(q.ctx, q.db, 10)
		return len(items) == 0
	}, 5*time.Second, 50*time.Millisecond,
		"dead-letter queue should be swept on delivery recovery without waiting for the sweep interval")

	cancel()
	require.ErrorIs(t, <-runErr, context.Canceled)
}

func TestDeadLetterSweep_DrainsEntireBacklog(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	q, err := newSQLiteQueue(Config{
		Path:                    filepath.Join(t.TempDir(), queueDir),
		MaxAttempts:             1,         // promote on the first failure
		DeadLetterSweepInterval: time.Hour, // we drive the sweep manually
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	const total = dequeueBatchSize*2 + 10
	for i := int64(0); i < total; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
	}
	items, err := q.fetch(total)
	require.NoError(t, err)
	require.Len(t, items, total)
	promoted, err := q.processFailedDeliveries(ctx, items)
	require.NoError(t, err)
	require.Equal(t, total, promoted)

	dlItems, err := fetchDeadLetter(q.ctx, q.db, total)
	require.NoError(t, err)
	require.Len(t, dlItems, total, "all events should be in the dead-letter table before the sweep")

	var delivered atomic.Int64
	handler := func(_ context.Context, items []Item) []Item {
		delivered.Add(int64(len(items)))
		return items
	}
	q.sweepDeadLetter(ctx, handler)

	require.Equal(t, int64(total), delivered.Load(),
		"a single sweep should redeliver the entire dead-letter backlog")

	remaining, err := fetchDeadLetter(q.ctx, q.db, total)
	require.NoError(t, err)
	require.Empty(t, remaining, "dead-letter table should be empty after a full sweep")
}

func TestDeadLetterTTL_ExpiresOldRows(t *testing.T) {
	t.Parallel()

	q, err := newSQLiteQueue(Config{
		Path:          filepath.Join(t.TempDir(), queueDir),
		DeadLetterTTL: time.Hour,
		MaxAttempts:   1, // promote on the first failure
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	// Enqueue one event, then move it to dead-letter directly so we can
	// control its failed_at timestamp.
	require.NoError(t, q.Enqueue(newTestEvent(99)))
	items, err := q.fetch(1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	promoted, err := q.processFailedDeliveries(t.Context(), items)
	require.NoError(t, err)
	require.Equal(t, 1, promoted)

	// Back-date the row to simulate an entry older than the TTL.
	pastTimestamp := time.Now().Add(-2 * time.Hour).Unix()
	_, err = q.db.Exec("UPDATE audit_dead_letter SET failed_at = ?", pastTimestamp)
	require.NoError(t, err)

	dlItems, err := fetchDeadLetter(q.ctx, q.db, 10)
	require.NoError(t, err)
	require.Len(t, dlItems, 1, "row should exist before expiry")

	beforeExpired := testutil.ToFloat64(deadLetterExpired)

	q.expireDeadLetter()

	require.InDelta(t, beforeExpired+1, testutil.ToFloat64(deadLetterExpired), 0.0001,
		"deadLetterExpired counter should increment by the number of expired rows")

	dlItems, err = fetchDeadLetter(q.ctx, q.db, 10)
	require.NoError(t, err)
	require.Empty(t, dlItems, "row should have been deleted by expireDeadLetter")
}

var corruptPayload = []byte{0xff, 0xff, 0xff}

func countRows(t *testing.T, q *sqliteQueue, table string) int {
	t.Helper()
	var n int
	require.NoError(t, q.db.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&n))
	return n
}

func TestFetch_QuarantinesCorruptEvent(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	_, err := q.db.Exec("INSERT INTO audit_queue (payload) VALUES (?)", corruptPayload)
	require.NoError(t, err)
	for i := int64(0); i < 3; i++ {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
	}

	require.Equal(t, 1, countRows(t, q, "audit_queue WHERE payload = x'ffffff'"),
		"corrupt row should be removed from audit_queue")

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 3, "the three good events should still be returned")
	for i, item := range items {
		require.Equal(t, int64(i), item.Event.GetIndex())
	}

	require.Zero(t, countRows(t, q, "audit_queue WHERE payload = x'ffffff'"),
		"corrupt row should be removed from audit_queue")

	var (
		payload []byte
		errMsg  string
		source  string
	)
	require.NoError(t, q.db.QueryRow(
		"SELECT payload, error, source FROM corrupt_events").Scan(&payload, &errMsg, &source))
	require.Equal(t, corruptPayload, payload)
	require.NotEmpty(t, errMsg, "the deserialization error should be recorded")
	require.Equal(t, auditQueueTable, source)
}

func TestFetch_CorruptDoesNotBlockQueue(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	_, err := q.db.Exec("INSERT INTO audit_queue (payload) VALUES (?)", corruptPayload)
	require.NoError(t, err)
	for i := range int64(3) {
		require.NoError(t, q.Enqueue(newTestEvent(i)))
	}

	var delivered []int64
	for {
		items, err := q.fetch(10)
		require.NoError(t, err)
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			delivered = append(delivered, item.Event.GetIndex())
		}
		require.NoError(t, q.ack(items))
	}

	require.Equal(t, []int64{0, 1, 2}, delivered, "all good events should drain")
	require.Zero(t, countRows(t, q, auditQueueTable), "queue should be fully drained")
	require.Equal(t, 1, countRows(t, q, "corrupt_events"))
}

func TestFetchDeadLetter_QuarantinesCorrupt(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	_, err := q.db.Exec(
		"INSERT INTO audit_dead_letter (payload, failed_at) VALUES (?, ?)",
		corruptPayload, time.Now().Unix())
	require.NoError(t, err)

	items, err := q.fetchDeadLetter(10)
	require.NoError(t, err)
	require.Empty(t, items)

	require.Zero(t, countRows(t, q, auditDeadLetterTable),
		"corrupt row should be removed from audit_dead_letter")

	var source string
	require.NoError(t, q.db.QueryRow("SELECT source FROM corrupt_events").Scan(&source))
	require.Equal(t, auditDeadLetterTable, source)
}

func TestExpireCorruptEvents(t *testing.T) {
	t.Parallel()

	q, err := newSQLiteQueue(Config{
		Path:          filepath.Join(t.TempDir(), queueDir),
		DeadLetterTTL: time.Hour,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	old := time.Now().Add(-2 * time.Hour).Unix()
	recent := time.Now().Unix()
	_, err = q.db.Exec(
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'boom', 'audit_queue', ?), (?, 'boom', 'audit_queue', ?)",
		corruptPayload, old, corruptPayload, recent)
	require.NoError(t, err)

	q.expireCorruptEvents()

	require.Equal(t, 1, countRows(t, q, "corrupt_events"), "only the recent corrupt row should remain")
}

func TestRecoverCorruptEvents(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	// A genuinely corrupt payload that will never deserialize.
	_, err := q.db.Exec(
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'boom', 'audit_queue', ?)",
		corruptPayload, time.Now().Unix())
	require.NoError(t, err)

	// A valid payload that now deserializes, as if written by a newer binary
	// and quarantined before an upgrade made it readable.
	oneOf, err := apievents.ToOneOf(newTestEvent(7))
	require.NoError(t, err)
	validPayload, err := oneOf.Marshal()
	require.NoError(t, err)
	_, err = q.db.Exec(
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'was unknown event type', 'audit_queue', ?)",
		validPayload, time.Now().Unix())
	require.NoError(t, err)

	q.recoverCorruptEvents()

	// The recoverable event is back in audit_queue and deliverable.
	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].Event.GetIndex())

	// The genuinely corrupt row is retained for a future attempt.
	require.Equal(t, 1, countRows(t, q, "corrupt_events"))
}

func TestRecoverCorruptEvents_PagesLargeBacklog(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	const validCount = dequeueBatchSize*2 + 10 // spans multiple recovery pages
	const corruptCount = 5

	insert := func(payload []byte) {
		_, err := q.db.Exec(
			"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'x', 'audit_queue', 0)",
			payload)
		require.NoError(t, err)
	}

	for i := 0; i < validCount; i++ {
		oneOf, err := apievents.ToOneOf(newTestEvent(int64(i)))
		require.NoError(t, err)
		payload, err := oneOf.Marshal()
		require.NoError(t, err)
		insert(payload)
		// Interleave corrupt rows so the cursor must page past them.
		if i < corruptCount {
			insert(corruptPayload)
		}
	}

	q.recoverCorruptEvents()

	require.Equal(t, validCount, countRows(t, q, "audit_queue"),
		"all recoverable events should be re-queued across pages")
	require.Equal(t, corruptCount, countRows(t, q, "corrupt_events"),
		"only the genuinely corrupt rows should remain")
}

func TestRecoverCorruptEvents_WatermarkAdvances(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)

	// An un-recoverable row examined by the first pass.
	_, err := q.db.Exec(
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'boom', 'audit_queue', 0)",
		corruptPayload)
	require.NoError(t, err)

	q.recoverCorruptEvents()
	require.Positive(t, q.recoveryWatermark, "watermark should advance past the examined row")
	require.Zero(t, countRows(t, q, "audit_queue"))
	require.Equal(t, 1, countRows(t, q, "corrupt_events"))

	// A recoverable row inserted after the first pass gets a higher id, so it
	// lands above the watermark and is picked up on the next pass.
	oneOf, err := apievents.ToOneOf(newTestEvent(7))
	require.NoError(t, err)
	validPayload, err := oneOf.Marshal()
	require.NoError(t, err)
	_, err = q.db.Exec(
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'was unknown', 'audit_queue', 0)",
		validPayload)
	require.NoError(t, err)

	q.recoverCorruptEvents()

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(7), items[0].Event.GetIndex())
	require.Equal(t, 1, countRows(t, q, "corrupt_events"),
		"the un-recoverable row should remain, skipped by the watermark")
}

func TestOrphanAdoption_MigratesCorruptEvents(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	ctx := t.Context()

	aPath := filepath.Join(parent, "a")
	a, err := newSQLiteQueue(Config{Path: aPath})
	require.NoError(t, err)

	failedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	_, err = a.db.Exec(
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, 'boom', 'audit_queue', ?)",
		corruptPayload, failedAt)
	require.NoError(t, err)
	require.NoError(t, a.Close())

	_, err = os.Stat(aPath)
	require.NoError(t, err, "expected A's directory to remain after Close due to non-empty corrupt_events")

	b, err := newSQLiteQueue(Config{
		Path:               filepath.Join(parent, "b"),
		OrphanScanInterval: 50 * time.Millisecond,
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
		return countRows(t, b, "corrupt_events") == 1
	}, 5*time.Second, 50*time.Millisecond, "expected B to migrate A's corrupt row")

	var (
		payload    []byte
		errMsg     string
		source     string
		gotFaildAt int64
	)
	require.NoError(t, b.db.QueryRow(
		"SELECT payload, error, source, failed_at FROM corrupt_events").Scan(&payload, &errMsg, &source, &gotFaildAt))
	require.Equal(t, corruptPayload, payload)
	require.Equal(t, "boom", errMsg)
	require.Equal(t, "audit_queue", source)
	require.Equal(t, failedAt, gotFaildAt, "failed_at should be preserved across migration")

	require.Eventually(t, func() bool {
		_, err := os.Stat(aPath)
		return os.IsNotExist(err)
	}, 5*time.Second, 50*time.Millisecond, "expected A's directory to be removed after migration")

	cancelB()
	require.ErrorIs(t, <-bRunErr, context.Canceled)
}

func TestItemsNotIn(t *testing.T) {
	t.Parallel()

	mk := func(ids ...int64) []Item {
		items := make([]Item, len(ids))
		for i, id := range ids {
			items[i] = Item{id: id}
		}
		return items
	}

	tests := []struct {
		name      string
		all       []Item
		delivered []Item
		want      []Item
	}{
		{
			name:      "partial delivery returns only undelivered items",
			all:       mk(1, 2, 3),
			delivered: mk(1),
			want:      mk(2, 3),
		},
		{
			name:      "single undelivered item among many delivered",
			all:       mk(1, 2, 3, 4),
			delivered: mk(1, 2, 4),
			want:      mk(3),
		},
		{
			name:      "nothing delivered returns all",
			all:       mk(1, 2, 3),
			delivered: nil,
			want:      mk(1, 2, 3),
		},
		{
			name:      "everything delivered returns empty",
			all:       mk(1, 2, 3),
			delivered: mk(1, 2, 3),
			want:      nil,
		},
		{
			name:      "more delivered than all does not panic",
			all:       mk(1, 9),
			delivered: mk(1, 2, 3, 4, 5),
			want:      mk(9),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := itemsNotIn(tt.all, tt.delivered)
			require.Equal(t, tt.want, got)
			require.Len(t, got, len(tt.want))
		})
	}
}
