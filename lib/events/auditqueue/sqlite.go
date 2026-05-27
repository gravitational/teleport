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
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const (
	defaultMaxBatch                = 250
	defaultToBeWrittenBufferLen    = 1024
	pollInterval                   = 100 * time.Millisecond
	dequeueBatchSize               = 256
	sqlitePageSize                 = 4096 // Bytes
	defaultMaxAttempts             = 10
	defaultDeadLetterSweepInterval = 10 * time.Minute
	defaultDeadLetterTTL           = 30 * 24 * time.Hour // 30 days

	// busyTimeoutMillis sets the maximum time we will wait for a SQLite DB
	// operation.
	busyTimeoutMillis = 5000
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit_queue (
    id       INTEGER PRIMARY KEY,
    payload  BLOB    NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_dead_letter (
    id        INTEGER PRIMARY KEY,
    payload   BLOB    NOT NULL,
    failed_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dead_letter_failed_at ON audit_dead_letter(failed_at);

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
	db                      *sql.DB
	path                    string
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
	handlerMu               sync.Mutex
	softLimit               int64
	maxAttempts             int
	deadLetterSweepInterval time.Duration
	deadLetterTTL           time.Duration
}

// Ensure that we implement the interface Queue at compile time.
var _ Queue = (*sqliteQueue)(nil)

func bytesToPages(nBytes int64) int64 {
	return nBytes / sqlitePageSize
}

// addSharedParams adds the shared parameters between both the in-memory sqlite
// implementation and the on-disk implementation.
func addSharedParams(params url.Values, maxBytes int64) {
	params.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMillis))
	params.Add("_pragma", "temp_store(MEMORY)")
	params.Add("_pragma", fmt.Sprintf("max_page_count(%d)", bytesToPages(maxBytes)))
}

// newSqliteBaseQueue creates the common core of a sqliteQueue. It is used so
// that shared initialization between the `sqliteQueue` and the
// `sqliteInMemoryQueue` can re-use code.
func newBaseQueue(db *sql.DB, cfg Config) (*sqliteQueue, error) {
	if err := metrics.RegisterPrometheusCollectors(prometheusCollectors...); err != nil {
		return nil, trace.Wrap(err)
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
		toBeWritten:             make(chan writeRequest, defaultToBeWrittenBufferLen),
		maxBatch:                defaultMaxBatch,
		ctx:                     ctx,
		cancel:                  cancel,
		maxAttempts:             maxAttempts,
		deadLetterSweepInterval: deadLetterSweepInterval,
		deadLetterTTL:           deadLetterTTL,
	}

	q.wg.Go(q.writeLoop)

	return q, nil
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

func (q *sqliteQueue) runPollLoop(ctx context.Context, handler Handler) {
	pollTimer := time.NewTimer(pollInterval)
	defer pollTimer.Stop()

	for {
		if ctx.Err() != nil || q.ctx.Err() != nil {
			return
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
		if len(items) > 0 {
			// 2. Forward those events to the inner emitter.
			// This is done via the handler function.
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

			// 4. Handle delivery failures.
			q.handleDeliveryFailures(ctx, items, successfullyDelivered)
		} else {
			// If we don't have any events to forward, then we'll sleep for a
			// bit.
			select {
			case <-ctx.Done():
				return
			case <-q.ctx.Done():
				return
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

func (q *sqliteQueue) handleDeliveryFailures(ctx context.Context, items []Item, successfullyDelivered []Item) {
	failed := itemsNotIn(items, successfullyDelivered)
	if len(failed) > 0 {
		exhausted, err := q.incrementAttempts(failed)
		if err != nil {
			slog.ErrorContext(q.ctx,
				"Failed to increment audit event attempt counts.",
				"error", err,
			)
		}
		if len(exhausted) > 0 {
			if err := q.moveToDeadLetter(exhausted); err != nil {
				slog.ErrorContext(q.ctx,
					"Failed to move exhausted audit events to dead-letter queue.",
					"error", err,
				)
			} else {
				deadLetterPromotions.Add(float64(len(exhausted)))
				slog.WarnContext(q.ctx,
					"Audit events moved to dead-letter queue after exhausting retries.",
					"count", len(exhausted),
					"max_attempts", q.maxAttempts,
				)
			}
		}
		retryTotal.Add(float64(len(failed) - len(exhausted)))
	}
}

// itemsNotIn returns the subset of items from `all` that are not found in
// `delivered`, hence it returns the items that failed to be delivered.
func itemsNotIn(all, delivered []Item) []Item {
	if len(delivered) == 0 {
		return all
	}
	deliveredMap := make(map[int64]struct{}, len(delivered))
	for _, it := range delivered {
		deliveredMap[it.ID] = struct{}{}
	}
	var failed []Item
	for _, it := range all {
		if _, ok := deliveredMap[it.ID]; !ok {
			failed = append(failed, it)
		}
	}
	return failed
}

// incrementAttempts increments the attempt counter for each item in the DB and
// returns the subset whose counter has now reached maxAttempts.
func (q *sqliteQueue) incrementAttempts(items []Item) ([]Item, error) {
	tx, err := q.db.BeginTx(q.ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tx.Rollback()

	updateStmt, err := tx.PrepareContext(q.ctx,
		"UPDATE audit_queue SET attempts = attempts + 1 WHERE id = ? RETURNING attempts")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer updateStmt.Close()

	var exhausted []Item
	for _, item := range items {
		var attempts int
		if err := updateStmt.QueryRowContext(q.ctx, item.ID).Scan(&attempts); err != nil {
			return nil, trace.Wrap(err)
		}
		if attempts >= q.maxAttempts {
			exhausted = append(exhausted, item)
		}
	}
	return exhausted, trace.Wrap(tx.Commit())
}

// moveToDeadLetter atomically moves items from audit_queue to audit_dead_letter.
func (q *sqliteQueue) moveToDeadLetter(items []Item) error {
	now := time.Now().Unix()
	rows := make([]deadLetterRow, 0, len(items))
	for _, item := range items {
		oneOf, err := apievents.ToOneOf(item.Event)
		if err != nil {
			return trace.Wrap(err)
		}
		payload, err := oneOf.Marshal()
		if err != nil {
			return trace.Wrap(err)
		}
		rows = append(rows, deadLetterRow{payload: payload, failedAt: now})
	}

	tx, err := q.db.BeginTx(q.ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tx.Rollback()

	if err := insertDeadLetterTx(q.ctx, tx, rows); err != nil {
		return trace.Wrap(err)
	}

	deleteStmt, err := tx.PrepareContext(q.ctx,
		"DELETE FROM audit_queue WHERE id = ?")
	if err != nil {
		return trace.Wrap(err)
	}
	defer deleteStmt.Close()

	for _, item := range items {
		if _, err := deleteStmt.ExecContext(q.ctx, item.ID); err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(tx.Commit())
}

// deadLetterSweepLoop periodically re-attempts delivery of dead-letter events
// and deletes entries that have exceeded the TTL.
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
			q.sweepDeadLetter(ctx, handler)
			q.expireDeadLetter()
			timer.Reset(q.deadLetterSweepInterval)
		}
	}
}

func (q *sqliteQueue) sweepDeadLetter(ctx context.Context, handler Handler) {
	items, err := q.fetchDeadLetter(dequeueBatchSize)
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
	delivered := q.forwardBatch(ctx, handler, items)
	if len(delivered) == 0 {
		return
	}
	if err := q.ackDeadLetter(delivered); err != nil {
		slog.ErrorContext(q.ctx,
			"Failed to ack dead-letter audit events.",
			"error", err,
		)
	}
}

// fetchDeadLetter reads up to limit items from audit_dead_letter ordered by id.
func (q *sqliteQueue) fetchDeadLetter(limit int) ([]Item, error) {
	rows, err := q.db.QueryContext(q.ctx,
		"SELECT id, payload FROM audit_dead_letter ORDER BY id ASC LIMIT ?", limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return scanItems(rows)
}

// ackDeadLetter deletes successfully re-delivered items from audit_dead_letter.
func (q *sqliteQueue) ackDeadLetter(items []Item) error {
	return deleteByIDs(q.ctx, q.db, "audit_dead_letter", items)
}

// expireDeadLetter deletes dead-letter rows older than the configured TTL.
func (q *sqliteQueue) expireDeadLetter() {
	cutoff := time.Now().Add(-q.deadLetterTTL).Unix()
	if _, err := q.db.ExecContext(q.ctx,
		"DELETE FROM audit_dead_letter WHERE failed_at < ?", cutoff); err != nil && q.ctx.Err() == nil {
		slog.ErrorContext(q.ctx,
			"Failed to expire dead-letter audit events.",
			"error", err,
		)
	}
}

// deadLetterRow represents a row from the audit_dead_letter table.
type deadLetterRow struct {
	id       int64
	payload  []byte
	failedAt int64
}

func insertDeadLetterTx(ctx context.Context, tx *sql.Tx, rows []deadLetterRow) error {
	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO audit_dead_letter (payload, failed_at) VALUES (?, ?)")
	if err != nil {
		return trace.Wrap(err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.payload, r.failedAt); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (q *sqliteQueue) fetch(limit int) ([]Item, error) {
	return fetchDB(q.ctx, q.db, limit)
}

func (q *sqliteQueue) ack(items []Item) error {
	err := ackDB(q.ctx, q.db, items)
	if err != nil {
		// Once 'ack' succeeds, we have finished the entire end-to-end of
		// delivering these events.
		eventsDelivered.Add(float64(len(items)))
	}
	return err
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

		// TODO(kkloberdanz): What should we do about events that fail to
		// deserialize? If an event is corrupt then we will get stuck in a loop
		// where we cannot process any events with the corrupt on clogging up
		// the queue.
		//
		// To fix this: Let's add a table called `corrupt_events` and move
		// anything that fails to deserialize to this table. This PR is already
		// big enough, so we will cover this in a follow up.
		var oneOf apievents.OneOf
		if err := oneOf.Unmarshal(payload); err != nil {
			return nil, trace.Wrap(err)
		}
		event, err := apievents.FromOneOf(oneOf)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		items = append(items, Item{ID: id, Event: event})
	}
	return items, trace.Wrap(rows.Err())
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

func deleteIDsFromTable(ctx context.Context, db *sql.DB, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	switch table {
	case "audit_queue", "audit_dead_letter":
	default:
		return trace.BadParameter("unknown table %q", table)
	}
	placeholders := strings.Repeat("?,", len(ids))
	query := "DELETE FROM " + table + " WHERE id IN (" + placeholders[:len(placeholders)-1] + ")"
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
		ids[i] = item.ID
	}
	return deleteIDsFromTable(ctx, db, table, ids)
}

// ackDB deletes the rows for items from the audit_queue table.
func ackDB(ctx context.Context, db *sql.DB, items []Item) error {
	return deleteByIDs(ctx, db, "audit_queue", items)
}

func recordTeleportVersion(db *sql.DB, version string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO teleport_info (key, value) VALUES ('teleport_version', ?)",
		version,
	)
	return trace.Wrap(err)
}
