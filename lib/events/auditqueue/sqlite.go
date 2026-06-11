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
	pollInterval                   = 100 * time.Millisecond
	sqlitePageSize                 = 4096 // Bytes
	auditQueueTable                = "audit_queue"
	auditDeadLetterTable           = "audit_dead_letter"
	corruptEventsTable             = "corrupt_events"
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

	// busyTimeoutMillis sets the maximum time we will wait for a SQLite DB
	// operation.
	busyTimeoutMillis = 5000
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS audit_queue (
    id       INTEGER PRIMARY KEY,
    payload  BLOB    NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE TABLE IF NOT EXISTS audit_dead_letter (
    id        INTEGER PRIMARY KEY,
    payload   BLOB    NOT NULL,
    failed_at INTEGER NOT NULL
) STRICT;

CREATE INDEX IF NOT EXISTS idx_dead_letter_failed_at ON audit_dead_letter(failed_at);

CREATE TABLE IF NOT EXISTS teleport_info (
    id    INTEGER PRIMARY KEY,
    key   TEXT NOT NULL UNIQUE,
    value TEXT NOT NULL
) STRICT;

-- We need AUTOINCREMENT here to ensure the recoveryWatermark has a
-- monotonically incrementing id. We need to ensure that the 'id' is never
-- re-used for this table. Other tables do not have this requirement, which is
-- why this is the only table that requires AUTOINCREMENT.
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
	softLimit               int64
	maxAttempts             int
	deadLetterSweepInterval time.Duration
	deadLetterTTL           time.Duration
	synchronous             SynchronousMode

	// recoveryWatermark is the highest corrupt_events id that
	// recoverCorruptEvents has already examined in this process. It only ever
	// climbs, so each pass skips rows it has already tried.
	recoveryWatermark int64
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
		toBeWritten:             make(chan writeRequest),
		maxBatch:                defaultMaxBatch,
		ctx:                     ctx,
		cancel:                  cancel,
		maxAttempts:             maxAttempts,
		deadLetterSweepInterval: deadLetterSweepInterval,
		deadLetterTTL:           deadLetterTTL,
		synchronous:             cfg.Synchronous,
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
			successfullyDelivered := handler(ctx, items)
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

func (q *sqliteQueue) handleDeliveryFailures(ctx context.Context, items []Item, successfullyDelivered []Item) {
	failed := itemsNotIn(items, successfullyDelivered)
	if len(failed) == 0 {
		return
	}
	promoted, err := q.processFailedDeliveries(failed)
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

func (q *sqliteQueue) processFailedDeliveries(failed []Item) (int, error) {
	if len(failed) == 0 {
		return 0, nil
	}

	tx, err := q.db.BeginTx(q.ctx, nil)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer tx.Rollback()

	ids := make([]any, len(failed))
	for i, item := range failed {
		ids[i] = item.ID
	}

	rows, err := tx.QueryContext(q.ctx,
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
	if _, err := tx.ExecContext(q.ctx,
		"INSERT INTO audit_dead_letter (payload, failed_at) "+
			"SELECT payload, ? FROM audit_queue WHERE id IN ("+exhaustedPlaceholders+")",
		insertArgs...,
	); err != nil {
		return 0, trace.Wrap(err)
	}

	if _, err := tx.ExecContext(q.ctx,
		"DELETE FROM audit_queue WHERE id IN ("+exhaustedPlaceholders+")",
		exhaustedEventIDs...,
	); err != nil {
		return 0, trace.Wrap(err)
	}

	return len(exhaustedEventIDs), trace.Wrap(tx.Commit())
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
			q.recoverCorruptEvents()
			q.expireCorruptEvents()
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
	delivered := handler(ctx, items)
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
	items, corrupt, err := scanItems(rows)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handleCorrupt(q.ctx, q.db, auditDeadLetterTable, corrupt)
	return items, nil
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

func (q *sqliteQueue) expireCorruptEvents() {
	cutoff := time.Now().Add(-q.deadLetterTTL).Unix()
	res, err := q.db.ExecContext(q.ctx,
		"DELETE FROM corrupt_events WHERE failed_at < ?", cutoff)
	if err != nil {
		if q.ctx.Err() == nil {
			slog.ErrorContext(q.ctx,
				"Failed to expire corrupt audit events.",
				"error", err,
			)
		}
		return
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		return
	}
	corruptExpired.Add(float64(n))
	slog.WarnContext(q.ctx,
		"Permanently dropped corrupt audit events that exceeded the retention TTL.",
		"count", n,
		"ttl", q.deadLetterTTL,
	)
}

func (q *sqliteQueue) recoverCorruptEvents() {
	var total int
	for {
		if q.ctx.Err() != nil {
			return
		}
		batch, err := fetchCorruptForRecovery(q.ctx, q.db, q.recoveryWatermark, dequeueBatchSize)
		if err != nil {
			if q.ctx.Err() == nil {
				slog.ErrorContext(q.ctx,
					"Failed to read corrupt audit events for recovery.",
					"error", err,
				)
			}
			return
		}
		if len(batch) == 0 {
			break
		}

		var recovered []recoveredEvent
		for _, r := range batch {
			var oneOf apievents.OneOf
			if err := oneOf.Unmarshal(r.payload); err != nil {
				continue
			}
			if _, err := apievents.FromOneOf(oneOf); err != nil {
				continue
			}
			// The event deserializes, so it is no longer corrupt and can be
			// re-sent.
			recovered = append(recovered, r)
		}
		if len(recovered) > 0 {
			if err := q.reinjectRecovered(recovered); err != nil {
				if q.ctx.Err() == nil {
					slog.ErrorContext(q.ctx,
						"Failed to re-queue recovered audit events.",
						"error", err,
						"count", len(recovered),
					)
				}
				// Leave the watermark unmoved so this batch is retried next pass
				// rather than skipped until the next process restart.
				return
			}
			corruptRecovered.Add(float64(len(recovered)))
			total += len(recovered)
		}
		// Advance only after the batch is fully handled.
		q.recoveryWatermark = batch[len(batch)-1].id
	}
	if total > 0 {
		slog.InfoContext(q.ctx,
			"Recovered previously-corrupt audit events and re-queued them for delivery.",
			"count", total,
		)
	}
}

type recoveredEvent struct {
	id      int64
	payload []byte
}

func fetchCorruptForRecovery(ctx context.Context, db *sql.DB, afterID int64, limit int) ([]recoveredEvent, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, payload FROM corrupt_events WHERE id > ? ORDER BY id ASC LIMIT ?", afterID, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rows.Close()
	var out []recoveredEvent
	for rows.Next() {
		var r recoveredEvent
		if err := rows.Scan(&r.id, &r.payload); err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, r)
	}
	return out, trace.Wrap(rows.Err())
}

// reinjectRecovered atomically moves recovered events back into audit_queue and
// removes them from corrupt_events.
func (q *sqliteQueue) reinjectRecovered(events []recoveredEvent) error {
	tx, err := q.db.BeginTx(q.ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.PrepareContext(q.ctx, "INSERT INTO audit_queue (payload) VALUES (?)")
	if err != nil {
		return trace.Wrap(err)
	}
	defer insertStmt.Close()

	ids := make([]int64, 0, len(events))
	for _, e := range events {
		if _, err := insertStmt.ExecContext(q.ctx, e.payload); err != nil {
			return trace.Wrap(err)
		}
		ids = append(ids, e.id)
	}

	if err := deleteIDsFromTable(q.ctx, tx, corruptEventsTable, ids); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(tx.Commit())
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
	if err == nil {
		// Once 'ack' succeeds, we have finished the entire end-to-end of
		// delivering these events.
		eventsDelivered.Add(float64(len(items)))
	}
	return err
}

// corruptRow is a queue row whose payload failed to deserialize. Such rows are
// moved to the corrupt_events table so they do not clog the queue.
type corruptRow struct {
	id      int64
	payload []byte
	err     error
}

func scanItems(rows *sql.Rows) (items []Item, corrupt []corruptRow, err error) {
	defer rows.Close()
	for rows.Next() {
		var (
			id      int64
			payload []byte
		)
		if err := rows.Scan(&id, &payload); err != nil {
			return nil, nil, trace.Wrap(err)
		}

		var oneOf apievents.OneOf
		if err := oneOf.Unmarshal(payload); err != nil {
			corrupt = append(corrupt, corruptRow{id: id, payload: payload, err: err})
			continue
		}
		event, err := apievents.FromOneOf(oneOf)
		if err != nil {
			corrupt = append(corrupt, corruptRow{id: id, payload: payload, err: err})
			continue
		}
		items = append(items, Item{ID: id, Event: event})
	}
	return items, corrupt, trace.Wrap(rows.Err())
}

func quarantineCorrupt(ctx context.Context, db *sql.DB, sourceTable string, corrupt []corruptRow) error {
	if len(corrupt) == 0 {
		return nil
	}
	switch sourceTable {
	case auditQueueTable, auditDeadLetterTable:
	default:
		return trace.BadParameter("unknown table %q", sourceTable)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.PrepareContext(ctx,
		"INSERT INTO corrupt_events (payload, error, source, failed_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return trace.Wrap(err)
	}
	defer insertStmt.Close()

	ids := make([]int64, 0, len(corrupt))
	failedAt := time.Now().Unix()
	for _, c := range corrupt {
		if _, err := insertStmt.ExecContext(ctx, c.payload, c.err.Error(), sourceTable, failedAt); err != nil {
			return trace.Wrap(err)
		}
		ids = append(ids, c.id)
	}

	if err := deleteIDsFromTable(ctx, tx, sourceTable, ids); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(tx.Commit())
}

func handleCorrupt(ctx context.Context, db *sql.DB, sourceTable string, corrupt []corruptRow) {
	if len(corrupt) == 0 {
		return
	}
	if err := quarantineCorrupt(ctx, db, sourceTable, corrupt); err != nil {
		slog.ErrorContext(ctx,
			"Failed to quarantine corrupt audit events.",
			"error", err,
			"source_table", sourceTable,
			"count", len(corrupt),
		)
		return
	}
	corruptEvents.Add(float64(len(corrupt)))
	slog.WarnContext(ctx,
		"Quarantined corrupt audit events that failed to deserialize.",
		"source_table", sourceTable,
		"count", len(corrupt),
		"first_error", corrupt[0].err.Error(),
	)
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
	items, corrupt, err := scanItems(rows)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	handleCorrupt(ctx, db, auditQueueTable, corrupt)
	return items, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

// execer is satisfied by both *sql.DB and *sql.Tx, so deleteIDsFromTable can
// run standalone or inside a caller's transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func deleteIDsFromTable(ctx context.Context, e execer, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	switch table {
	case auditQueueTable, auditDeadLetterTable, corruptEventsTable:
	default:
		return trace.BadParameter("unknown table %q", table)
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := e.ExecContext(ctx,
		"DELETE FROM "+table+" WHERE id IN ("+placeholders(len(ids))+")", args...)
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
	return deleteByIDs(ctx, db, auditQueueTable, items)
}

func recordTeleportVersion(db *sql.DB, version string) error {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO teleport_info (key, value) VALUES ('teleport_version', ?)",
		version,
	)
	return trace.Wrap(err)
}
