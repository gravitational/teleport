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
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	pollInterval                   = 100 * time.Millisecond
	incrementalVacuumInterval      = 10 * time.Minute
	defaultOrphanScanInterval      = 10 * time.Minute
	queueLockFile                  = "queue.lock"
	queueDBFile                    = "queue.db"
	auditQueueTable                = "audit_queue"
	auditDeadLetterTable           = "audit_dead_letter"
	tmpDirSuffix                   = ".tmp"
	defaultSoftLimit               = 100 * 1024 * 1024 // 100 MiB
	softLimitCheckInterval         = time.Minute
	sqlitePageSize                 = 4096 // Bytes
	defaultMaxAttempts             = 10
	defaultDeadLetterSweepInterval = 10 * time.Minute
	defaultDeadLetterTTL           = 30 * 24 * time.Hour // 30 days

	// We've run benchmarks and found a batch size of 25 to be a good middle
	// ground between insertion performance and memory overhead of
	// events-in-flight. We've observed peak performance is achieved with batch
	// sizes around 250, and we encounter diminishing returns above that.
	//
	// See: https://github.com/gravitational/teleport.e/blob/rfd/0254-sqlite-audit-log-event-queue/rfd/0254-sqlite-audit-log-event-queue.md#modernc-synchronousnormal-60-second-duration
	defaultMaxBatch  = 25
	dequeueBatchSize = 25

	// walJournalSizeLimit is the number of bytes the `-wal` file gets
	// truncated to in between checkpoints.
	walJournalSizeLimit = 64 * 1024 * 1024 // 64 MiB

	// busyTimeoutMillis sets the maximum time we will wait for a SQLite DB
	// operation.
	busyTimeoutMillis = 5000

	// defaultMaxBytes limits the size of the SQLite database file.
	defaultMaxBytes int64 = 5 * 1024 * 1024 * 1024 // 5 GiB

	// staleTmpThreshold is how old an in-progress <uuid>.tmp/ directory
	// must be before the orphan scanner removes it. Under normal circumstances,
	// A `.tmp` directory will only exist for a few milliseconds.
	staleTmpThreshold       = time.Hour
	initQueueDirMaxAttempts = 10
	initQueueDirRetryDelay  = 50 * time.Millisecond
)

// A note on `AUTOINCREMENT`, to quote the SQLite docs:
// > The AUTOINCREMENT keyword imposes extra CPU, memory, disk space, and disk
// > I/O overhead and should be avoided if not strictly needed.
// > It is usually not needed.
//
// In the context of audit_queue, AUTOINCREMENT is not strictly needed. IDs
// cannot be re-used during the fetch-deliver-ack window, because we only call
// delete on the rows that we have fetched from the database, and which still
// reside in the database.
//
// We are including AUTOINCREMENT here as a belt-and-suspenders approach to
// guard against potential future changes.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit_queue (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    payload  BLOB    NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE TABLE IF NOT EXISTS audit_dead_letter (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    payload   BLOB    NOT NULL,
    failed_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_dead_letter_failed_at ON audit_dead_letter(failed_at);

CREATE TABLE IF NOT EXISTS teleport_info (
    id    INTEGER PRIMARY KEY AUTOINCREMENT,
    key   TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL
) STRICT;

-- We need AUTOINCREMENT here to ensure the recoveryWatermark has a
-- monotonically incrementing id. We need to ensure that the 'id' is never
-- re-used for this table. Other tables do not have this requirement.
-- See: https://sqlite.org/autoinc.html
CREATE TABLE IF NOT EXISTS corrupt_events (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    payload   BLOB    NOT NULL,
    error     TEXT    NOT NULL,
    source    TEXT    NOT NULL,
    failed_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_corrupt_events_failed_at ON corrupt_events(failed_at);
`

// writeRequest is a single Enqueue waiting on a commit result. The writer
// goroutine reads requests off the input channel, commits a batch in one
// SQLite transaction, and signals each caller on its resp channel.
type writeRequest struct {
	payload []byte
	resp    chan error
}

type sqliteQueue struct {
	db   *sql.DB
	path string
	// runMu enforces a single consumer. Run only ever TryLocks it and returns
	// ErrAlreadyRunning if it's held. The lock is held for the entire lifetime
	// of Run, so a blocking Lock would hang until shutdown.
	runMu                   sync.Mutex
	toBeWritten             chan writeRequest
	maxBatch                int
	ctx                     context.Context
	cancel                  context.CancelFunc
	wg                      sync.WaitGroup
	closeOnce               sync.Once
	parentDir               string
	selfStat                os.FileInfo
	unlock                  func() error
	orphanScanInterval      time.Duration
	softLimit               int64
	maxAttempts             int
	deadLetterSweepInterval time.Duration
	deadLetterTTL           time.Duration
	// deadLetterKick wakes the dead-letter sweeper ahead of its timer. It has
	// capacity 1 so pending kicks coalesce into a single sweep.
	deadLetterKick chan struct{}
}

// Ensure that we implement the interface Queue at compile time.
var _ Queue = (*sqliteQueue)(nil)

func bytesToPages(nBytes int64) int64 {
	return nBytes / sqlitePageSize
}

func getDSN(dbPath string, maxBytes int64) string {
	maxPages := bytesToPages(maxBytes)
	params := url.Values{}
	params.Add("_pragma", "auto_vacuum(INCREMENTAL)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", fmt.Sprintf("journal_size_limit(%d)", walJournalSizeLimit))
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	params.Add("_pragma", "temp_store(MEMORY)")
	params.Add("_pragma", fmt.Sprintf("max_page_count(%d)", maxPages))
	u := url.URL{
		Scheme:   "file",
		OmitHost: true,
		Path:     dbPath,
		RawQuery: params.Encode(),
	}
	return u.String()
}

func newSQLiteQueue(cfg Config) (*sqliteQueue, error) {
	if cfg.Path == "" {
		return nil, trace.BadParameter("Path is required to create an sqlite queue")
	}

	if err := metrics.RegisterPrometheusCollectors(prometheusCollectors...); err != nil {
		return nil, trace.Wrap(err)
	}

	unlock, err := initQueueDir(cfg.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db, err := initializeDb(cfg.Path, cfg.MaxBytes)
	if err != nil {
		_ = unlock()
		_ = os.RemoveAll(cfg.Path)
		return nil, trace.Wrap(err)
	}

	selfStat, err := os.Stat(cfg.Path)
	if err != nil {
		db.Close()
		_ = unlock()
		_ = os.RemoveAll(cfg.Path)
		return nil, trace.ConvertSystemError(err)
	}

	scanInterval := cfg.OrphanScanInterval
	if scanInterval <= 0 {
		scanInterval = defaultOrphanScanInterval
	}

	softLimit := cfg.SoftLimit
	if softLimit <= 0 {
		softLimit = defaultSoftLimit
	}

	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}

	deadLetterSweepInterval := cfg.DeadLetterSweepInterval
	if deadLetterSweepInterval <= 0 {
		deadLetterSweepInterval = defaultDeadLetterSweepInterval
	}

	deadLetterTTL := cfg.DeadLetterTTL
	if deadLetterTTL <= 0 {
		deadLetterTTL = defaultDeadLetterTTL
	}

	ctx, cancel := context.WithCancel(context.Background())
	q := &sqliteQueue{
		db:                      db,
		path:                    cfg.Path,
		toBeWritten:             make(chan writeRequest),
		maxBatch:                defaultMaxBatch,
		ctx:                     ctx,
		cancel:                  cancel,
		parentDir:               filepath.Dir(cfg.Path),
		selfStat:                selfStat,
		unlock:                  unlock,
		orphanScanInterval:      scanInterval,
		softLimit:               softLimit,
		maxAttempts:             maxAttempts,
		deadLetterSweepInterval: deadLetterSweepInterval,
		deadLetterTTL:           deadLetterTTL,
		deadLetterKick:          make(chan struct{}, 1),
	}

	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	slog.InfoContext(q.ctx, "Audit queue initialized.",
		"path", cfg.Path,
		"max_bytes", maxBytes,
		"soft_limit", softLimit,
		"max_attempts", maxAttempts,
		"dead_letter_ttl", deadLetterTTL,
	)

	q.wg.Go(q.writeLoop)
	q.wg.Go(q.vacuumLoop)
	q.wg.Go(q.softLimitLoop)

	return q, nil
}

func initializeDb(path string, maxBytes int64) (*sql.DB, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	dbPath := filepath.Join(path, queueDBFile)
	db, err := sql.Open("sqlite", getDSN(dbPath, maxBytes))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}
	if err := recordTeleportVersion(db, teleport.Version); err != nil {
		db.Close()
		return nil, trace.Wrap(err)
	}

	return db, nil
}

// initQueueDir creates and initializes the directory where the audit log queue
// will reside. It does this in a way that is safe from race conditions.
func initQueueDir(path string) (func() error, error) {
	tmpPath := path + tmpDirSuffix
	var err error
	for attempt := range initQueueDirMaxAttempts {
		var unlock func() error
		unlock, err = tryInitQueueDir(path, tmpPath)
		if err == nil {
			return unlock, nil
		}

		isRetryableError := errors.Is(err, utils.ErrUnsuccessfulLockTry) || trace.IsNotFound(err)
		if !isRetryableError {
			return nil, trace.Wrap(err)
		}

		if attempt < initQueueDirMaxAttempts-1 {
			time.Sleep(initQueueDirRetryDelay)
		}
	}
	return nil, trace.Wrap(err, "failed to initialize audit-queue directory %q after %d attempts (is the system time set correctly?)", path, initQueueDirMaxAttempts)
}

func tryInitQueueDir(path, tmpPath string) (func() error, error) {
	if err := os.MkdirAll(tmpPath, 0o700); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Take the lock on the `queue.lock` file. This ensures that no other
	// instances try to adopt this queue.
	unlock, err := utils.FSTryWriteLock(filepath.Join(tmpPath, queueLockFile))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove the `.tmp` suffix, marking this as a live queue.
	if err := os.Rename(tmpPath, path); err != nil {
		// Only remove the directory if we own the flock.
		_ = os.RemoveAll(tmpPath)
		_ = unlock()
		return nil, trace.ConvertSystemError(err)
	}
	return unlock, nil
}

// vacuumLoop periodically cleans up deleted records from the SQLite database
// file.
func (q *sqliteQueue) vacuumLoop() {
	timer := time.NewTimer(incrementalVacuumInterval)
	defer timer.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-timer.C:
			if _, err := q.db.ExecContext(q.ctx, "PRAGMA incremental_vacuum"); err != nil {
				slog.ErrorContext(q.ctx, "Failed to run incremental_vacuum.", "error", err)
			}
			timer.Reset(incrementalVacuumInterval)
		}
	}
}

// softLimitLoop periodically stats queue.db and emits a warning when its
// size exceeds the configured soft limit.
func (q *sqliteQueue) softLimitLoop() {
	ticker := time.NewTicker(softLimitCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
		}
		above, size, err := q.aboveSoftLimit()
		if err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to stat audit queue file.",
				"path", q.path,
				"error", err,
			)
			continue
		}
		if above {
			slog.WarnContext(q.ctx,
				"audit event queue above soft limit",
				"path", q.path,
				"size_bytes", size,
				"soft_limit_bytes", q.softLimit,
			)
			softLimitWarnings.Inc()
		}
	}
}

func (q *sqliteQueue) aboveSoftLimit() (bool, int64, error) {
	info, err := os.Stat(filepath.Join(q.path, queueDBFile))
	if err != nil {
		return false, 0, trace.ConvertSystemError(err)
	}
	size := info.Size()
	return size > q.softLimit, size, nil
}

// Enqueue writes the event to the SQLite based queue. If you get a nil error
// return value, then the event has been durably committed to disk. This is the
// producer end of the audit queue. For the consumer end, see `writeLoop`.
//
// This function utilizes a group commit strategy to opportunistically batch
// commits together. This works by utilizing the latency of the commit in
// progress to enqueue and batch together subsequent events. These events will
// be committed together as a batch once the in-progress commit finishes.
//
// Events that are in-flight at the time of shutdown may be lost, as the queue
// context cancellation causes both this function and writeLoop to return before
// the commit completes.
func (q *sqliteQueue) Enqueue(event apievents.AuditEvent) error {
	// Serialize the event to bytes.
	oneOf, err := apievents.ToOneOf(event)
	if err != nil {
		return trace.Wrap(err)
	}
	payload, err := oneOf.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	req := writeRequest{
		payload: payload,
		resp:    make(chan error, 1),
	}

	// Send the event to the write queue. The channel `toBeWritten` will be
	// drained by the function `writeLoop`. We do not want to drop an audit log
	// event, so we block the caller until the event is stored durably.
	select {
	case q.toBeWritten <- req:
	case <-q.ctx.Done():
		return trace.Wrap(ErrClosed)
	}

	// Await for this event to be written to the DB. If there are no pending
	// events to be written to the DB, then this will be committed immediately.
	// Otherwise, we will wait until the in progress commit has finished, then
	// our event will be committed in the next batch.
	select {
	case err := <-req.resp:
		return trace.Wrap(err)
	case <-q.ctx.Done():
		return trace.Wrap(ErrClosed)
	}
}

// writeLoop is the single writer goroutine. It opportunistically batches
// together events produced by the `Enqueue` method. It drains the `toBeWritten`
// channel.
func (q *sqliteQueue) writeLoop() {
	for {
		// Wait until we get the first event.
		var first writeRequest
		select {
		case <-q.ctx.Done():
			return
		case first = <-q.toBeWritten:
		}

		// We got at least one event. Let's collect any additional events into
		// a buffer.
		batch := make([]writeRequest, 0, q.maxBatch)
		batch = append(batch, first)
	drain:
		for len(batch) < q.maxBatch {
			select {
			case req := <-q.toBeWritten:
				// While we can still take events, collect them into a batch.
				batch = append(batch, req)
			default:
				break drain
			}
		}

		// Update histogram metrics so we can observe what kind of batch sizes
		// we get.
		batchSize.Observe(float64(len(batch)))

		// Commit entire batch within a single transaction. We've observed that
		// larger batch sizes up to about 250 leads to noticeable performance
		// improvements.
		err := q.commitBatch(batch)
		if err == nil {
			eventsEnqueued.Add(float64(len(batch)))
		}
		for _, req := range batch {
			// If we got an error from the transaction, then we will propagate
			// that error to all callers. Otherwise, all callers receive a `nil`
			// error response and they can guarantee that the event they
			// enqueued has been durably committed to the database.
			req.resp <- err
		}
	}
}

func (q *sqliteQueue) commitBatch(batch []writeRequest) error {
	tx, err := q.db.BeginTx(q.ctx, nil)
	if err != nil {
		return mapCommitError(err)
	}

	// Preferred way to ensure we roll back the transaction if there is an error
	// This will be a no-op if the transaction succeeds.
	//
	// See: https://go.dev/doc/database/execute-transactions#example
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(q.ctx, "INSERT INTO audit_queue (payload) VALUES (?)")
	if err != nil {
		return mapCommitError(err)
	}
	defer stmt.Close()

	for _, req := range batch {
		if _, err := stmt.ExecContext(q.ctx, req.payload); err != nil {
			return mapCommitError(err)
		}
	}

	return mapCommitError(tx.Commit())
}

func mapCommitError(err error) error {
	if err == nil {
		return nil
	}
	if isSQLiteFullError(err) {
		return trace.Wrap(ErrQueueFull)
	}
	return trace.Wrap(err)
}

func isSQLiteFullError(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && (e.Code() == sqlite3.SQLITE_FULL)
}

// Run drains the queue. `handler` is the function called for each audit log
// event that is held within the queue. The audit log queue follows a single
// consumer model in order to batch events together and commit them as groups.
func (q *sqliteQueue) Run(ctx context.Context, handler Handler) error {
	// Ensure we only have a single consumer running.
	if !q.runMu.TryLock() {
		return trace.Wrap(ErrAlreadyRunning)
	}
	defer q.runMu.Unlock()

	// Startup the orphan scanner and dead-letter sweeper.
	var wg sync.WaitGroup
	wg.Go(func() {
		q.orphanScanLoop(ctx)
	})
	wg.Go(func() {
		q.deadLetterSweepLoop(ctx, handler)
	})
	defer wg.Wait()

	pollTimer := time.NewTimer(pollInterval)
	defer pollTimer.Stop()

	deliveryFailing := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := q.ctx.Err(); err != nil {
			return err
		}

		// This queue does not have an explicit "dequeue" method. This is
		// because we want to ensure reliable "at-least-once" delivery. Hence
		// events reside in the database queue until we "ack" that they have
		// been successfully delivered.
		//
		// Dequeueing events is a four stage process.
		// 1. Fetch a batch of events from the DB.
		// 2. Forward those events to the inner emitter.
		// 3. ACK the events that were successfully delivered, thus
		//    deleting them from the DB.
		// 4. For failed events, increment their attempt counter. Promote
		//    any that have hit max_attempts to the dead-letter table.

		// 1. Fetch a batch of events from the DB.
		items, err := q.fetch(dequeueBatchSize)
		if err != nil && q.ctx.Err() == nil {
			slog.ErrorContext(
				q.ctx,
				"Failed to fetch audit events.",
				"error", err,
			)
		}
		progressed := false
		if len(items) > 0 {
			// 2. Forward those events to the inner emitter.
			// This is done via the handler function.
			successfullyDelivered := handler(ctx, items)
			if len(successfullyDelivered) > 0 {
				// Delivery transitioned from failing to succeeding, which
				// may indicate that a downstream outage just ended. Kick the
				// dead-letter sweeper so events that failed during the outage
				// are re-delivered without waiting for the next sweep
				// interval.
				if deliveryFailing {
					q.kickDeadLetterSweep()
				}
				deliveryFailing = false

				// 3. ACK the delivered events, retrying without re-delivering
				//    on failure.
				if err := q.ackWithRetry(ctx, successfullyDelivered); err != nil {
					return err
				}
				progressed = true
			} else {
				deliveryFailing = true
			}

			// 4. Handle delivery failures.
			if ctx.Err() == nil && q.ctx.Err() == nil {
				q.handleDeliveryFailures(ctx, items, successfullyDelivered)
			}
		}

		// Back off when we made no forward progress to prevent hammering the
		// auth server in a hot loop.
		if !progressed {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-q.ctx.Done():
				return q.ctx.Err()
			case <-pollTimer.C:
			}
			pollTimer.Reset(pollInterval)
		}
	}
}

// ackWithRetry acks the delivered events. It intends to cover an error
// condition where if we experience local disk errors or database errors from
// SQLite, then we don't want to hammer the auth server with retries. In the
// event of disk errors, the best thing we can do is continue retrying until we
// *hopefully* succeed. It returns nil once the events are acked, or the context
// error if it stopped because the context was canceled.
func (q *sqliteQueue) ackWithRetry(ctx context.Context, items []Item) error {
	firstFailure := true
	backoff := time.NewTimer(pollInterval)
	defer backoff.Stop()
	for {
		err := q.ack(items)
		if err == nil {
			if !firstFailure {
				slog.InfoContext(
					q.ctx,
					"Recovered: ACKed previously stuck audit events.",
					"count", len(items),
				)
			}
			return nil
		}
		if firstFailure && ctx.Err() == nil && q.ctx.Err() == nil {
			slog.ErrorContext(
				q.ctx,
				"Failed to ACK delivered audit events. Retrying without re-delivering.",
				"error", err,
				"count", len(items),
			)
			firstFailure = false
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.ctx.Done():
			return q.ctx.Err()
		case <-backoff.C:
		}
		backoff.Reset(pollInterval)
	}
}

func (q *sqliteQueue) handleDeliveryFailures(ctx context.Context, items []Item, successfullyDelivered []Item) {
	failed := itemsNotIn(items, successfullyDelivered)
	if len(failed) == 0 {
		return
	}
	promoted, err := q.processFailedDeliveries(ctx, failed)
	if err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to process failed audit event deliveries.",
			"error", err,
		)
		return
	}
	if promoted > 0 {
		deadLetterPromotions.Add(float64(promoted))
		slog.WarnContext(q.ctx,
			"Audit events moved to dead-letter queue after exhausting retries.",
			"count", promoted,
			"max_attempts", q.maxAttempts,
		)
	}
	retryTotal.Add(float64(len(failed) - promoted))
}

// itemsNotIn returns the subset of items from `all` that are not found in
// `delivered`, hence it returns the items that failed to be delivered.
func itemsNotIn(all, delivered []Item) []Item {
	if len(delivered) == 0 {
		return all
	}
	deliveredMap := make(map[int64]struct{}, len(delivered))
	for _, it := range delivered {
		deliveredMap[it.id] = struct{}{}
	}
	var failed []Item
	for _, it := range all {
		if _, ok := deliveredMap[it.id]; !ok {
			failed = append(failed, it)
		}
	}
	return failed
}

func (q *sqliteQueue) processFailedDeliveries(ctx context.Context, failed []Item) (int, error) {
	if len(failed) == 0 {
		return 0, nil
	}

	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer tx.Rollback()

	ids := make([]any, len(failed))
	for i, item := range failed {
		ids[i] = item.id
	}

	rows, err := tx.QueryContext(ctx,
		"UPDATE audit_queue SET attempts = attempts + 1 WHERE id IN ("+placeholders(len(ids))+") RETURNING id, attempts",
		ids...,
	)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer rows.Close()

	var exhaustedEventIDs []any
	for rows.Next() {
		var id int64
		var attempts int
		if err := rows.Scan(&id, &attempts); err != nil {
			return 0, trace.Wrap(err)
		}
		if attempts >= q.maxAttempts {
			exhaustedEventIDs = append(exhaustedEventIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, trace.Wrap(err)
	}

	// If no events have exhausted their attempts, then we can skip the rest of
	// the function.
	if len(exhaustedEventIDs) == 0 {
		return 0, trace.Wrap(tx.Commit())
	}

	// Copy exhausted rows into audit_dead_letter then delete them from
	// audit_queue.
	exhaustedPlaceholders := placeholders(len(exhaustedEventIDs))

	insertArgs := make([]any, 0, len(exhaustedEventIDs)+1)
	insertArgs = append(insertArgs, time.Now().Unix())
	insertArgs = append(insertArgs, exhaustedEventIDs...)
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO audit_dead_letter (payload, failed_at) "+
			"SELECT payload, ? FROM audit_queue WHERE id IN ("+exhaustedPlaceholders+")",
		insertArgs...,
	); err != nil {
		return 0, trace.Wrap(err)
	}

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM audit_queue WHERE id IN ("+exhaustedPlaceholders+")",
		exhaustedEventIDs...,
	); err != nil {
		return 0, trace.Wrap(err)
	}

	return len(exhaustedEventIDs), trace.Wrap(tx.Commit())
}

// deadLetterSweepLoop periodically re-attempts delivery of dead-letter events
// and deletes entries that have exceeded the TTL. A kick on deadLetterKick
// triggers a sweep immediately instead of waiting for the timer.
func (q *sqliteQueue) deadLetterSweepLoop(ctx context.Context, handler Handler) {
	timer := time.NewTimer(q.deadLetterSweepInterval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.ctx.Done():
			return
		case <-timer.C:
		case <-q.deadLetterKick:
		}
		q.sweepDeadLetter(ctx, handler)
		q.expireDeadLetter()
		timer.Reset(q.deadLetterSweepInterval)
	}
}

// kickDeadLetterSweep requests an immediate dead-letter sweep. It never
// blocks: if a kick is already pending, the two coalesce.
func (q *sqliteQueue) kickDeadLetterSweep() {
	select {
	case q.deadLetterKick <- struct{}{}:
	default:
	}
}

func (q *sqliteQueue) sweepDeadLetter(ctx context.Context, handler Handler) {
	maxID, err := q.maxDeadLetterID()
	if err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to read dead-letter bounds.",
			"error", err,
		)
		return
	}

	var afterID int64
	for afterID < maxID {
		if ctx.Err() != nil || q.ctx.Err() != nil {
			return
		}

		items, err := q.fetchDeadLetterRange(afterID, maxID, dequeueBatchSize)
		if err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to fetch dead-letter audit events.",
				"error", err,
			)
			return
		}
		if len(items) == 0 {
			return
		}

		afterID = items[len(items)-1].id

		delivered := handler(ctx, items)
		if len(delivered) == 0 {
			continue
		}
		if err := q.ackDeadLetter(delivered); err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to ack dead-letter audit events.",
				"error", err,
			)
		}
	}
}

func (q *sqliteQueue) maxDeadLetterID() (int64, error) {
	var maxID int64
	if err := q.db.QueryRowContext(q.ctx,
		"SELECT COALESCE(MAX(id), 0) FROM audit_dead_letter").Scan(&maxID); err != nil {
		return 0, trace.Wrap(err)
	}
	return maxID, nil
}

func (q *sqliteQueue) fetchDeadLetterRange(afterID, maxID int64, limit int) ([]Item, error) {
	rows, err := q.db.QueryContext(q.ctx,
		"SELECT id, payload FROM audit_dead_letter WHERE id > ? AND id <= ? ORDER BY id ASC LIMIT ?",
		afterID, maxID, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return scanItems(rows)
}

// ackDeadLetter deletes successfully re-delivered items from audit_dead_letter.
func (q *sqliteQueue) ackDeadLetter(items []Item) error {
	return deleteByIDs(q.ctx, q.db, auditDeadLetterTable, items)
}

// expireDeadLetter deletes dead-letter rows older than the configured TTL.
func (q *sqliteQueue) expireDeadLetter() {
	cutoff := time.Now().Add(-q.deadLetterTTL).Unix()
	result, err := q.db.ExecContext(q.ctx,
		"DELETE FROM audit_dead_letter WHERE failed_at < ?", cutoff)
	if err != nil {
		if q.ctx.Err() == nil {
			slog.ErrorContext(q.ctx,
				"Failed to expire dead-letter audit events.",
				"error", err,
			)
		}
		return
	}

	numExpired, err := result.RowsAffected()
	if err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to read number of expired dead-letter audit events.",
			"error", err,
		)
		return
	}
	if numExpired > 0 {
		deadLetterExpired.Add(float64(numExpired))
		slog.WarnContext(q.ctx,
			"Permanently dropped dead-letter audit events that exceeded their TTL.",
			"count", numExpired,
			"dead_letter_ttl", q.deadLetterTTL,
		)
	}
}

func (q *sqliteQueue) orphanScanLoop(ctx context.Context) {
	ticker := time.NewTicker(q.orphanScanInterval)
	defer ticker.Stop()
	for {
		q.sweepStaleTmp()
		q.adoptOrphans(ctx)

		select {
		case <-ctx.Done():
			return
		case <-q.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// sweepStaleTmp cleans up any audit queue directories that have been orphaned
// during their initialization phase. This should be rare, but we still want to
// account for it.
func (q *sqliteQueue) sweepStaleTmp() {
	entries, err := os.ReadDir(q.parentDir)
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to list audit-queue parent directory.",
			"parent_dir", q.parentDir,
			"error", err,
		)
		return
	}
	cutoff := time.Now().Add(-staleTmpThreshold)
	for _, dirEntry := range entries {
		if !dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), tmpDirSuffix) {
			continue
		}
		info, err := dirEntry.Info()
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to stat audit-queue tmp directory.",
				"name", dirEntry.Name(),
				"error", err,
			)
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}

		// If we get to this point, then the directory is older than the cutoff,
		// which should not be the case for a tmp directory. These directories
		// should have their `*.tmp` suffix removed as soon as they are done
		// initializing, which should be a very quick process.
		stalePath := filepath.Join(q.parentDir, dirEntry.Name())
		tryRemoveStaleTmp(q.ctx, stalePath)
	}
}

func tryRemoveStaleTmp(ctx context.Context, stalePath string) {
	unlock, err := utils.FSTryWriteLock(filepath.Join(stalePath, queueLockFile))
	if err != nil {
		// The lock is held, so a creator is still active. Leave the directory
		// for its owner and try again on the next sweep.
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			slog.WarnContext(ctx,
				"Audit-queue tmp directory is still locked past the stale threshold. Leaving it for its owner.",
				"path", stalePath,
				"stale_threshold", staleTmpThreshold,
			)
			return
		}
		orphanScanErrors.Inc()
		slog.ErrorContext(ctx,
			"Failed to lock stale audit-queue tmp directory.",
			"path", stalePath,
			"error", err,
		)
		return
	}
	defer func() {
		if err := unlock(); err != nil {
			slog.ErrorContext(ctx,
				"Failed to release stale audit-queue tmp flock.",
				"path", stalePath,
				"error", err,
			)
		}
	}()

	// We hold the lock, so the directory is unowned. Tmp directories were
	// orphaned before they finished initializing, therefore they will have no
	// audit log events, hence they are safe to remove.
	if err := os.RemoveAll(stalePath); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(ctx,
			"Failed to remove stale audit-queue tmp directory.",
			"path", stalePath,
			"error", err,
		)
	}
}

func (q *sqliteQueue) adoptOrphans(ctx context.Context) {
	entries, err := os.ReadDir(q.parentDir)
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to list audit-queue parent directory.",
			"parent_dir", q.parentDir,
			"error", err,
		)
		return
	}
	for _, dirEntry := range entries {
		if ctx.Err() != nil {
			return
		}
		if !dirEntry.IsDir() || strings.HasSuffix(dirEntry.Name(), tmpDirSuffix) {
			continue
		}

		entryPath := filepath.Join(q.parentDir, dirEntry.Name())
		entryStat, err := os.Stat(entryPath)
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to stat audit-queue candidate directory.",
				"path", entryPath,
				"error", err,
			)
			continue
		}
		if os.SameFile(entryStat, q.selfStat) {
			continue
		}

		// If we got to this point, then we have found a directory that is not a
		// tmp directory and is not the same directory as the one this process
		// is already using. We are safe to attempt to adopt it.
		q.tryAdoptOrphan(ctx, entryPath)
	}
}

func (q *sqliteQueue) tryAdoptOrphan(ctx context.Context, path string) {
	unlock, err := utils.FSTryWriteLock(filepath.Join(path, queueLockFile))
	if err != nil {
		// This error indicates that the lock has already been taken, hence this
		// queue is not an orphan. We can skip it.
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return
		}

		// Otherwise, an error occurred while attempting to take the file lock.
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to attempt orphan flock.",
			"path", path,
			"error", err,
		)
		return
	}
	defer func() {
		if err := unlock(); err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to release orphan flock.",
				"path", path,
				"error", err,
			)
		}
	}()

	dbFilePath := filepath.Join(path, queueDBFile)
	db, err := sql.Open("sqlite", getDSN(dbFilePath, defaultMaxBytes))
	if err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx, "Failed to open orphan SQLite database.", "path", path, "error", err)
		return
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to connect to orphan SQLite database.",
			"path", path,
			"error", err,
		)
		_ = db.Close()
		return
	}

	migrateErr := q.migrateOrphanDB(ctx, db, filepath.Base(path))
	if err := db.Close(); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to close orphan SQLite database.",
			"path", path,
			"error", err,
		)
		return
	}

	if migrateErr != nil {
		// If we fail to migrate, we will try again on the next orphan adoption
		// cycle. Cancellation is a normal shutdown, not a migration failure.
		if ctx.Err() == nil && q.ctx.Err() == nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to migrate orphaned audit-queue database.",
				"path", path,
				"error", migrateErr,
			)
		}
		return
	}

	// If we got here, then we have successfully migrated the orphan. We can now
	// safely remove it.
	if err := os.RemoveAll(path); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to remove migrated orphan directory.",
			"path", path,
			"error", err,
		)
		return
	}
	q.clearOrphanWatermarks(ctx, filepath.Base(path))
	orphansAdopted.Inc()
	slog.InfoContext(q.ctx, "Adopted orphaned audit-queue directory.", "path", path)
}

func (q *sqliteQueue) migrateOrphanDB(ctx context.Context, db *sql.DB, name string) error {
	if err := q.migrateOrphanQueue(ctx, db, name); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(q.migrateOrphanDeadLetter(ctx, db, name))
}

func (q *sqliteQueue) migrateOrphanQueue(ctx context.Context, orphan *sql.DB, name string) error {
	return q.migrateOrphanTable(ctx, orphan, name, auditQueueTable,
		"SELECT id, payload, attempts FROM audit_queue WHERE id > ? ORDER BY id ASC LIMIT ?",
		"INSERT INTO audit_queue (payload, attempts) VALUES (?, ?)",
	)
}

func (q *sqliteQueue) migrateOrphanDeadLetter(ctx context.Context, orphan *sql.DB, name string) error {
	return q.migrateOrphanTable(ctx, orphan, name, auditDeadLetterTable,
		"SELECT id, payload, failed_at FROM audit_dead_letter WHERE id > ? ORDER BY id ASC LIMIT ?",
		"INSERT INTO audit_dead_letter (payload, failed_at) VALUES (?, ?)",
	)
}

func orphanWatermarkKey(name, table string) string {
	return "orphan_migration:" + name + ":" + table
}

func (q *sqliteQueue) migrateOrphanTable(ctx context.Context, orphan *sql.DB, name, table, selectSQL, insertSQL string) error {
	watermarkKey := orphanWatermarkKey(name, table)
	watermark, err := q.readOrphanWatermark(ctx, watermarkKey)
	if err != nil {
		return trace.Wrap(err, "reading orphan %s watermark", table)
	}
	for {
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		if err := q.ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		batch, err := fetchOrphanRows(ctx, orphan, selectSQL, watermark, dequeueBatchSize)
		if err != nil {
			return trace.Wrap(err, "fetching orphan %s rows", table)
		}
		if len(batch) == 0 {
			return nil
		}

		maxID := batch[len(batch)-1].id
		if err := q.insertMigratedBatch(ctx, insertSQL, batch, watermarkKey, maxID); err != nil {
			return trace.Wrap(err, "migrating orphan %s rows", table)
		}
		watermark = maxID

		ids := make([]int64, len(batch))
		for i, r := range batch {
			ids[i] = r.id
		}
		if err := deleteIDsFromTable(ctx, orphan, table, ids); err != nil {
			return trace.Wrap(err, "deleting migrated orphan %s rows", table)
		}
	}
}

func (q *sqliteQueue) readOrphanWatermark(ctx context.Context, key string) (int64, error) {
	var value string
	err := q.db.QueryRowContext(ctx,
		"SELECT value FROM teleport_info WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, trace.Wrap(err)
	}
	watermark, err := strconv.ParseInt(value, 10, 64)
	return watermark, trace.Wrap(err)
}

func (q *sqliteQueue) clearOrphanWatermarks(ctx context.Context, name string) {
	if _, err := q.db.ExecContext(ctx,
		"DELETE FROM teleport_info WHERE key IN (?, ?)",
		orphanWatermarkKey(name, auditQueueTable),
		orphanWatermarkKey(name, auditDeadLetterTable),
	); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to clear orphan migration watermarks.",
			"orphan", name,
			"error", err,
		)
	}
}

type migratedRow struct {
	id     int64
	values []any
}

func fetchOrphanRows(ctx context.Context, db *sql.DB, selectSQL string, afterID int64, limit int) ([]migratedRow, error) {
	rows, err := db.QueryContext(ctx, selectSQL, afterID, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []migratedRow
	for rows.Next() {
		r := migratedRow{values: make([]any, len(cols)-1)}
		targets := make([]any, 0, len(cols))
		targets = append(targets, &r.id)
		for i := range r.values {
			targets = append(targets, &r.values[i])
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, r)
	}
	return out, trace.Wrap(rows.Err())
}

func (q *sqliteQueue) insertMigratedBatch(ctx context.Context, insertSQL string, batch []migratedRow, watermarkKey string, maxID int64) error {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stmt.Close()
	for _, r := range batch {
		if _, err := stmt.ExecContext(ctx, r.values...); err != nil {
			return trace.Wrap(err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT OR REPLACE INTO teleport_info (key, value) VALUES (?, ?)",
		watermarkKey, strconv.FormatInt(maxID, 10),
	); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(tx.Commit())
}

func (q *sqliteQueue) fetch(limit int) ([]Item, error) {
	return fetchDB(q.ctx, q.db, limit)
}

func (q *sqliteQueue) ack(items []Item) error {
	err := ackDB(q.ctx, q.db, items)
	if err == nil {
		// Once 'ack' succeeds, we have finished the entire end-to-end of
		// delivering these events.
		eventsDelivered.Add(float64(len(items)))
	}
	return trace.Wrap(err)
}

// fetchDB reads up to `limit` oldest items from the table `audit_queue`.
func fetchDB(ctx context.Context, db *sql.DB, limit int) ([]Item, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, payload FROM audit_queue ORDER BY id ASC LIMIT ?", limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return scanItems(rows)
}

func scanItems(rows *sql.Rows) ([]Item, error) {
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var (
			id      int64
			payload []byte
		)
		if err := rows.Scan(&id, &payload); err != nil {
			return nil, trace.Wrap(err)
		}
		var oneOf apievents.OneOf
		if err := oneOf.Unmarshal(payload); err != nil {
			return nil, trace.Wrap(err)
		}
		event, err := apievents.FromOneOf(oneOf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items = append(items, Item{id: id, Event: event})
	}
	return items, trace.Wrap(rows.Err())
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func deleteIDsFromTable(ctx context.Context, db *sql.DB, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	switch table {
	case auditQueueTable, auditDeadLetterTable:
	default:
		return trace.BadParameter("unknown table %q", table)
	}
	query := "DELETE FROM " + table + " WHERE id IN (" + placeholders(len(ids)) + ")"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := db.ExecContext(ctx, query, args...)
	return trace.Wrap(err)
}

func deleteByIDs(ctx context.Context, db *sql.DB, table string, items []Item) error {
	ids := make([]int64, len(items))
	for i, item := range items {
		ids[i] = item.id
	}
	return deleteIDsFromTable(ctx, db, table, ids)
}

// ackDB deletes the rows for items from the audit_queue table.
func ackDB(ctx context.Context, db *sql.DB, items []Item) error {
	return deleteByIDs(ctx, db, auditQueueTable, items)
}

func (q *sqliteQueue) Close() error {
	var errs []error
	q.closeOnce.Do(func() {
		q.cancel()
		q.wg.Wait()

		// Flush the WAL file.
		if _, err := q.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to checkpoint WAL on close.",
				"path", q.path,
				"error", err,
			)
		}

		empty, err := isQueueEmpty(q.db)
		if err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to check whether audit queue is empty on close.",
				"path", q.path,
				"error", err,
			)
		}

		if err := q.db.Close(); err != nil {
			errs = append(errs, err)
		}

		// Remove the directory before releasing the lock. This ensures no other
		// process can adopt and start draining a queue we are about to delete.
		if empty {
			if err := os.RemoveAll(q.path); err != nil {
				slog.ErrorContext(q.ctx, "Failed to remove empty audit-queue directory on close.", "path", q.path, "error", err)
			}
		}

		if q.unlock != nil {
			if err := q.unlock(); err != nil {
				errs = append(errs, err)
			}
		}
	})
	return trace.NewAggregate(errs...)
}

func isQueueEmpty(db *sql.DB) (bool, error) {
	var hasRows int
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM audit_queue) OR EXISTS(SELECT 1 FROM audit_dead_letter)",
	).Scan(&hasRows)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return hasRows == 0, nil
}

func recordTeleportVersion(db *sql.DB, version string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO teleport_info (key, value) VALUES ('teleport_version', ?)",
		version,
	)
	return trace.Wrap(err)
}
