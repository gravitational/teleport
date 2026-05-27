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
	defaultToBeWrittenBufferLen = 1024
	pollInterval                = 100 * time.Millisecond
	dequeueBatchSize            = 256
	incrementalVacuumInterval   = 10 * time.Minute
	defaultOrphanScanInterval   = 10 * time.Minute
	queueLockFile               = "queue.lock"
	queueDBFile                 = "queue.db"
	tmpDirSuffix                = ".tmp"
	defaultSoftLimit            = 100 * 1024 * 1024 // 100 MiB
	softLimitCheckInterval      = time.Minute
	sqlitePageSize              = 4096 // Bytes

	// We've run benchmarks and found a batch size of 25 to be a good middle
	// ground between insertion performance and memory overhead of
	// events-in-flight. We've observed peak performance is achieved with batch
	// sizes around 250, and we encounter diminishing returns above that.
	//
	// See: https://github.com/gravitational/teleport.e/blob/rfd/0254-sqlite-audit-log-event-queue/rfd/0254-sqlite-audit-log-event-queue.md#modernc-synchronousnormal-60-second-duration
	defaultMaxBatch = 25

	// walJournalSizeLimit is the number of bytes the `-wal` file gets
	// truncated to in between checkpoints.
	walJournalSizeLimit = 64 * 1024 * 1024 // 64 MiB

	// busyTimeoutMillis sets the maximum time we will wait for a SQLite DB
	// operation.
	busyTimeoutMillis = 5000

	// defaultMaxBytes limits the size of the SQLite database file.
	defaultMaxBytes int64 = 5 * 1024 * 1024 * 1024 // 5 GiB

	// staleTmpThreshold is how old an in-progress <uuid>.tmp/ directory
	// must be before the orphan scanner removes it. Anything younger may
	// still belong to a peer that's mid-creation.
	staleTmpThreshold = time.Hour
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit_queue (
    id      INTEGER PRIMARY KEY,
    payload BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS teleport_info (
    id    INTEGER PRIMARY KEY,
    key   TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL
);
`

// writeRequest is a single Enqueue waiting on a commit result. The writer
// goroutine reads requests off the input channel, commits a batch in one
// SQLite transaction, and signals each caller on its resp channel.
type writeRequest struct {
	payload []byte
	resp    chan error
}

type sqliteQueue struct {
	db                 *sql.DB
	path               string
	runMu              sync.Mutex
	toBeWritten        chan writeRequest
	maxBatch           int
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	closeOnce          sync.Once
	parentDir          string
	selfStat           os.FileInfo
	unlock             func() error
	orphanScanInterval time.Duration
	handlerMu          sync.Mutex
	softLimit          int64
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

	ctx, cancel := context.WithCancel(context.Background())
	q := &sqliteQueue{
		db:                 db,
		path:               cfg.Path,
		toBeWritten:        make(chan writeRequest, defaultToBeWrittenBufferLen),
		maxBatch:           defaultMaxBatch,
		ctx:                ctx,
		cancel:             cancel,
		parentDir:          filepath.Dir(cfg.Path),
		selfStat:           selfStat,
		unlock:             unlock,
		orphanScanInterval: scanInterval,
		softLimit:          softLimit,
	}

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
	// Create the tmp directory with the suffix `.tmp`
	// This avoids a race conditions from other audit log instances attempting
	// to claim this as an orphaned directory before it is fully initialized.
	tmpPath := path + tmpDirSuffix
	if err := os.MkdirAll(tmpPath, 0o700); err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Take the lock on the `queue.lock` file. This ensures that no other
	// instances try to adopt this queue.
	unlock, err := utils.FSTryWriteLock(filepath.Join(tmpPath, queueLockFile))
	if err != nil {
		_ = os.RemoveAll(tmpPath)
		return nil, trace.Wrap(err)
	}

	// Remove the `.tmp` suffix, marking this as a live queue.
	if err := os.Rename(tmpPath, path); err != nil {
		_ = unlock()
		_ = os.RemoveAll(tmpPath)
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
func (q *sqliteQueue) Enqueue(ctx context.Context, event apievents.AuditEvent) error {
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
	// drained by the function `writeLoop`. We intentionally ignore the caller's
	// context cancellation. We do not want to drop an audit log event even if
	// the caller cancels their context.
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
			q.drainShutdown()
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

func (q *sqliteQueue) drainShutdown() {
	for {
		select {
		case req := <-q.toBeWritten:
			// During shutdown, respond to all calls to `Enqueue` with an error
			// to indicate that we are shutting down and are unable to take any
			// more events.
			req.resp <- trace.Wrap(ErrClosed)
		default:
			return
		}
	}
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

	// Startup the orphan scanner.
	var wg sync.WaitGroup
	wg.Go(func() {
		q.orphanScanLoop(ctx, handler)
	})
	defer wg.Wait()

	pollTimer := time.NewTimer(pollInterval)
	defer pollTimer.Stop()

	for {
		if ctx.Err() != nil || q.ctx.Err() != nil {
			return nil
		}

		// This queue does not have an explicit "dequeue" method. This is
		// because we want to ensure reliable "at-least-once" delivery. Hence
		// events reside in the database queue until we "ack" that they have
		// been successfully delivered.
		//
		// Dequeueing events is a three stage process.
		// 1. Fetch a batch of events from the DB.
		// 2. Forward those events to the inner emitter.
		// 3. ACK the events that were successfully delivered, thus
		//    deleting them from the DB.

		// 1. Fetch a batch of events from the DB.
		items, err := q.fetch(dequeueBatchSize)
		if err != nil && q.ctx.Err() == nil {
			slog.ErrorContext(
				q.ctx,
				"Failed to fetch audit events.",
				"error", err,
			)
		}
		if len(items) > 0 {
			// 2. Forward those events to the inner emitter.
			// This is done via the the handler function.
			successfullyDelivered := q.forwardBatch(ctx, handler, items)
			if len(successfullyDelivered) > 0 {
				// 3. ACK the events that were successfully delivered, thus
				//    deleting them from the DB.
				if err := q.ack(successfullyDelivered); err != nil {
					slog.ErrorContext(
						q.ctx,
						"Failed to ack audit events.",
						"error", err,
					)
				}
			}
		} else {
			// If we don't have any events to forward, then we'll sleep for a
			// bit.
			select {
			case <-ctx.Done():
				return nil
			case <-q.ctx.Done():
				return nil
			case <-pollTimer.C:
			}
			pollTimer.Reset(pollInterval)
		}
	}
}

func (q *sqliteQueue) forwardBatch(ctx context.Context, handler Handler, items []Item) []Item {
	// The handler function is implemented by the caller of the auditqueue. We
	// do not wish to impose constraints such as thread safety on the user of
	// this interface. Therefore we handle thread safety issues via this mutex.
	q.handlerMu.Lock()
	defer q.handlerMu.Unlock()

	successfullyDelivered := handler(ctx, items)

	return successfullyDelivered
}

func (q *sqliteQueue) orphanScanLoop(ctx context.Context, handler Handler) {
	q.sweepStaleTmp()
	q.adoptOrphans(ctx, handler)

	ticker := time.NewTicker(q.orphanScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.sweepStaleTmp()
			q.adoptOrphans(ctx, handler)
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

		// Tmp directories were orphaned before they finished initializing,
		// therefore they will have no audit log events, hence they are safe to
		// remove.
		if err := os.RemoveAll(stalePath); err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx,
				"Failed to remove stale audit-queue tmp directory.",
				"path", stalePath,
				"error", err,
			)
		}
	}
}

func (q *sqliteQueue) adoptOrphans(ctx context.Context, handler Handler) {
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
		q.tryAdoptOrphan(entryPath, handler)
	}
}

func (q *sqliteQueue) tryAdoptOrphan(path string, handler Handler) {
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

	drained := q.drainOrphanDB(db, handler)
	if err := db.Close(); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to close orphan SQLite database.",
			"path", path,
			"error", err,
		)
		return
	}

	if !drained {
		// If we fail to drain, we will try again on the next orphan adoption
		// cycle.
		return
	}

	// If we got here, then we have successfully drained the orphan. We can now
	// safely remove it.
	if err := os.RemoveAll(path); err != nil {
		orphanScanErrors.Inc()
		slog.ErrorContext(q.ctx,
			"Failed to remove drained orphan directory.",
			"path", path,
			"error", err,
		)
		return
	}
	orphansAdopted.Inc()
	slog.InfoContext(q.ctx, "Adopted orphaned audit-queue directory.", "path", path)
}

func (q *sqliteQueue) drainOrphanDB(db *sql.DB, handler Handler) bool {
	for {
		if q.ctx.Err() != nil {
			return false
		}
		items, err := fetchDB(q.ctx, db, dequeueBatchSize)
		if err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx, "Failed to fetch orphan events.", "error", err)
			return false
		}
		if len(items) == 0 {
			return true
		}
		successfullyDelivered := q.forwardBatch(q.ctx, handler, items)
		if len(successfullyDelivered) == 0 {
			return false
		}

		// Notice: We do not want to call q.ack() here. This is because this db
		// is an adopted db. Hence it is a different database than the one that
		// is currently held by `q`.
		if err := ackDB(q.ctx, db, successfullyDelivered); err != nil {
			orphanScanErrors.Inc()
			slog.ErrorContext(q.ctx, "Failed to ack orphan events.", "error", err)
			return false
		}

		// We didn't manage to drain the entire orphan. We will re-attempt to
		// drain it on the next orphan sweep.
		if len(successfullyDelivered) < len(items) {
			return false
		}
	}
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

	rows, err := db.QueryContext(
		ctx,
		"SELECT id, payload FROM audit_queue ORDER BY id ASC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
		item := Item{ID: id, Event: event}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, trace.Wrap(err)
	}
	return items, nil
}

// ackDB deletes the rows for items from the audit_queue table.
func ackDB(ctx context.Context, db *sql.DB, items []Item) error {
	if len(items) == 0 {
		return nil
	}

	placeholders := strings.Repeat("?,", len(items))
	query := "DELETE FROM audit_queue WHERE id IN (" + placeholders[:len(placeholders)-1] + ")"

	args := make([]any, len(items))
	for i, item := range items {
		args[i] = item.ID
	}

	_, err := db.ExecContext(ctx, query, args...)
	return trace.Wrap(err)
}

func (q *sqliteQueue) Close() error {
	var firstErr error
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
			firstErr = err
		}

		// Remove the directory before releasing the lock. This ensures no other
		// process can adopt and start draining a queue we are about to delete.
		if empty {
			if err := os.RemoveAll(q.path); err != nil {
				slog.ErrorContext(q.ctx, "Failed to remove empty audit-queue directory on close.", "path", q.path, "error", err)
			}
		}

		if q.unlock != nil {
			if err := q.unlock(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	})
	return trace.Wrap(firstErr)
}

func isQueueEmpty(db *sql.DB) (bool, error) {
	var hasRows int
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM audit_queue)").Scan(&hasRows)
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
