/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/sqlbk"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"
)

// pgTx implements sqlbk.Tx for postgres.
type pgTx struct {
	*pgDB
	opts  *sql.TxOptions
	sqlTx *sql.Tx
	ctx   context.Context
	err   error // must be set by calling rollback()
}

// Err returns a transaction error. An error does not change once the
// transaction is in an error state. Calling other Tx methods has no effect
// on the state of the transaction.
func (tx *pgTx) Err() error {
	return tx.err
}

// Commit the transaction. The same error returned from the Err method is
// returned from Commit when the transaction is in an error state.
func (tx *pgTx) Commit() error {
	if tx.err != nil {
		return tx.err
	}
	tx.err = convertError(tx.sqlTx.Commit())
	return tx.err
}

// Rollback the transaction with an error. The error passed to Rollback is
// converted to a trace error and set as the transaction error returned from
// Err. If the transaction is already in an error state, the error is
// overridden by the error passed. Passing a nil error is considered a bug,
// but the rollback will continue with a generated error if the transaction
// is not already in an error state.
func (tx *pgTx) Rollback(err error) error {
	switch {
	case err != nil && tx.err == nil:
		// Standard path. Rollback with error.
		tx.rollback(err)

	case err != nil && tx.err != nil:
		// Tx in failed state. Override error (e.g. CompareAndSwap).
		tx.err = err

	case err == nil && tx.err == nil:
		// Tx OK. Rollback called with nil err. Rollback since that was the intent.
		tx.rollback(trace.BadParameter("[BUG] Rollback called with nil error."))
		tx.cfg.Log.Debug(tx.err)

	case err == nil && tx.err != nil:
		// Tx in failed state. Rollback called with nil error. Override error to signal issue.
		tx.err = trace.BadParameter("[BUG] Rollback called with nil error on failed transaction.")
		tx.cfg.Log.Debug(tx.err)
	}
	return trace.Wrap(tx.err)
}

// rollback and return true when err is not nil.
//
// This method is a control-flow helper for the other pgTx methods. Accepting
// nil errors and returning true or false improves readability in the same
// manner as trace.Wrap. Its name is purposefully short and discreet.
func (tx *pgTx) rollback(err error) bool {
	if err == nil {
		return false
	}
	if e := tx.sqlTx.Rollback(); e != nil {
		tx.cfg.Log.WithError(err).Errorf("Failed to rollback: %v", e)
	}
	tx.err = convertError(err)
	return true
}

// DeleteEvents created before expiryTime.
func (tx *pgTx) DeleteEvents(expiryTime time.Time) {
	if tx.err != nil {
		return
	}

	const query = `DELETE FROM event WHERE created < $1`
	_, err := tx.sqlTx.ExecContext(tx.ctx, query, expiryTime)
	tx.rollback(err)
}

// DeleteExpiredLeases removes leases whose expires column is not null and is
// less than the current time.
func (tx *pgTx) DeleteExpiredLeases() {
	if tx.err != nil {
		return
	}

	const query = `DELETE FROM lease WHERE (expires IS NOT NULL AND expires < $1)`
	_, err := tx.sqlTx.ExecContext(tx.ctx, query, tx.now())
	tx.rollback(err)
}

// DeleteItems not referencing an event or a valid lease.
func (tx *pgTx) DeleteItems() {
	if tx.err != nil {
		return
	}

	const query = `
		DELETE FROM item WHERE (key, id) IN (
			SELECT key, id
			FROM item
			LEFT JOIN lease USING (key, id)
			LEFT JOIN event USING (key, id)
			WHERE event.key IS NULL
		    AND (lease.key IS NULL OR lease.expires < $1)
		)`
	_, err := tx.sqlTx.ExecContext(tx.ctx, query, tx.now())
	tx.rollback(err)
}

// DeleteLease by key returning the backend item ID from the deleted lease.
// Zero is returned when the delete fails.
func (tx *pgTx) DeleteLease(key []byte) int64 {
	if tx.err != nil {
		return 0
	}

	var id int64
	const query = `DELETE FROM lease WHERE key=$1 RETURNING id`
	err := tx.sqlTx.QueryRowContext(tx.ctx, query, key).Scan(&id)
	if tx.rollback(err) {
		return 0
	}
	return id
}

// DeleteLeaseRange removes all leases inclusively between startKey
// and endKey. It returns the set of backend items deleted. The returned
// items include only Key and ID.
func (tx *pgTx) DeleteLeaseRange(startKey, endKey []byte) []backend.Item {
	if tx.err != nil {
		return nil
	}

	const query = `DELETE FROM lease WHERE key >= $1 AND key <= $2 RETURNING key, id`
	rows, err := tx.sqlTx.QueryContext(tx.ctx, query, startKey, endKey)
	if tx.rollback(err) {
		return nil
	}

	var items []backend.Item
	for rows.Next() {
		var item backend.Item
		err = rows.Scan(&item.Key, &item.ID)
		if tx.rollback(err) {
			return nil
		}
		items = append(items, item)
	}
	if tx.rollback(rows.Err()) {
		return nil
	}

	return items
}

// GetEvents returns an ordered set of events up to limit whose ID is
// greater than fromEventID.
func (tx *pgTx) GetEvents(fromEventID int64, limit int) sqlbk.Events {
	events := sqlbk.Events{LastID: fromEventID}
	if tx.err != nil {
		return events
	}

	const query = `
		SELECT event.eventid, event.key, event.id, event.type, item.value 
		FROM event JOIN item USING (key, id)
		WHERE event.eventid > $1
		ORDER BY event.eventid LIMIT $2`
	rows, err := tx.sqlTx.QueryContext(tx.ctx, query, fromEventID, limit)
	if tx.rollback(err) {
		return events
	}

	var lastEventID int64
	var backendEvents []backend.Event
	for rows.Next() {
		var event backend.Event
		err = rows.Scan(&lastEventID, &event.Item.Key, &event.Item.ID, &event.Type, &event.Item.Value)
		if tx.rollback(err) {
			return events
		}
		backendEvents = append(backendEvents, event)
	}
	if tx.rollback(rows.Err()) {
		return events
	}

	events.LastID = lastEventID
	events.BackendEvents = backendEvents
	if len(events.BackendEvents) == limit {
		events.Remaining = tx.getEventsRemaining(lastEventID)
	}

	return events
}

// getEventsRemaining returns the number of events that exist whose ID is
// greater than fromEventID.
func (tx *pgTx) getEventsRemaining(fromEventID int64) int {
	if tx.err != nil {
		return 0
	}

	const query = `SELECT count(*) FROM event WHERE event.eventid > $1`
	var eventsRemaining int
	err := tx.sqlTx.QueryRowContext(tx.ctx, query, fromEventID).Scan(&eventsRemaining)
	if tx.rollback(err) {
		return 0
	}
	return eventsRemaining
}

// GetExpiredLeases returns all leases whose expires field is less than
// or equal to the current time.
func (tx *pgTx) GetExpiredLeases() []backend.Lease {
	if tx.err != nil {
		return nil
	}

	const query = `SELECT key, id FROM lease WHERE expires <= $1`
	rows, err := tx.sqlTx.QueryContext(tx.ctx, query, tx.now())
	if tx.rollback(err) {
		return nil
	}

	var leases []backend.Lease
	for rows.Next() {
		var lease backend.Lease
		err = rows.Scan(&lease.Key, &lease.ID)
		if tx.rollback(err) {
			return nil
		}
		leases = append(leases, lease)
	}
	if tx.rollback(rows.Err()) {
		return nil
	}

	return leases
}

// GetItem by key. Nil is returned if the item has expired.
func (tx *pgTx) GetItem(key []byte) *backend.Item {
	if tx.err != nil {
		return nil
	}

	const query = `
		SELECT item.id, lease.expires, item.value
		FROM item JOIN lease USING (key, id)
		WHERE item.key = $1
		  AND (lease.expires IS NULL OR lease.expires > $2)`

	var expires sql.NullTime
	item := &backend.Item{Key: key}
	row := tx.sqlTx.QueryRowContext(tx.ctx, query, key, tx.now())
	err := row.Scan(&item.ID, &expires, &item.Value)
	if tx.rollback(err) {
		return nil
	}
	if expires.Valid {
		item.Expires = expires.Time
	}
	return item
}

// GetItemRange returns a set of backend items whose key is inclusively between
// startKey and endKey. The returned items are ordered by key, will not exceed
// limit, and does not include expired items.
func (tx *pgTx) GetItemRange(startKey, endKey []byte, limit int) []backend.Item {
	if tx.err != nil {
		return nil
	}

	const query = `
		SELECT lease.key, lease.id, lease.expires, item.value
		FROM lease JOIN item USING (key, id)
		WHERE (lease.key >= $1 and lease.key <= $2)
		  AND (lease.expires IS NULL OR lease.expires > $3)
		ORDER BY lease.key LIMIT $4`

	rows, err := tx.sqlTx.QueryContext(tx.ctx, query, startKey, endKey, tx.now(), limit)
	if tx.rollback(err) {
		return nil
	}

	var items []backend.Item
	for rows.Next() {
		var item backend.Item
		var expires sql.NullTime
		err = rows.Scan(&item.Key, &item.ID, &expires, &item.Value)
		if tx.rollback(err) {
			return nil
		}
		if expires.Valid {
			item.Expires = expires.Time
		}
		items = append(items, item)
	}
	if tx.rollback(rows.Err()) {
		return nil
	}

	return items
}

// GetItemValue returns an item's value by key if the item has not expired.
func (tx *pgTx) GetItemValue(key []byte) []byte {
	if tx.err != nil {
		return nil
	}

	const query = `
		SELECT item.value
		FROM lease JOIN item USING (key, id)
		WHERE lease.key = $1
		  AND (lease.expires IS NULL OR lease.expires > $2)`

	var value []byte
	err := tx.sqlTx.QueryRowContext(tx.ctx, query, key, tx.now()).Scan(&value)
	if tx.rollback(err) {
		return nil
	}
	return value
}

// GetLastEventID returns the most recent eventid. Zero is returned when the
// event table is empty.
func (tx *pgTx) GetLastEventID() int64 {
	if tx.err != nil {
		return 0
	}

	const query = `SELECT eventid FROM event ORDER BY eventid DESC LIMIT 1`

	var eventID int64
	err := tx.sqlTx.QueryRowContext(tx.ctx, query).Scan(&eventID)
	if err != nil {
		if err = convertError(err); !errors.Is(err, sqlbk.ErrNotFound) {
			tx.rollback(err)
			return 0
		}
	}
	return eventID
}

// InsertEvent for backend item with evenType.
func (tx *pgTx) InsertEvent(eventType types.OpType, item backend.Item) {
	if tx.err != nil {
		return
	}
	const query = `INSERT INTO event (eventid, created, key, id, type) VALUES ((select coalesce(max(eventid), 0)+1 from event),$1,$2,$3,$4)`
	_, err := tx.sqlTx.ExecContext(tx.ctx, query, tx.now(), item.Key, item.ID, eventType)
	tx.rollback(err)
}

// InsertItem creates a new backend item ID, inserts the item, and returns the
// new ID. The transaction will be set to an ErrRetry failed state if the ID
// generated is already taken, which can happen when multiple transactions
// are attempting to add the same item (the test suite's concurrent test
// produces this scenario).
func (tx *pgTx) InsertItem(item backend.Item) int64 {
	if tx.err != nil {
		return 0
	}

	item.ID = newID()
	const query = `INSERT INTO item (key, id, value) VALUES ($1,$2,$3)`
	_, err := tx.sqlTx.ExecContext(tx.ctx, query, item.Key, item.ID, item.Value)
	if tx.rollback(err) && errors.Is(tx.err, sqlbk.ErrAlreadyExists) {
		tx.err = sqlbk.ErrRetry
	}
	return item.ID
}

// LeaseExists returns true if a lease exists for key that has not expired.
func (tx *pgTx) LeaseExists(key []byte) bool {
	if tx.err != nil {
		return false
	}

	// Using EXISTS means we don't have to check for no rows.
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM lease
			WHERE key = $1
			AND (expires IS NULL OR expires > $2)
		)`

	var exists bool
	err := tx.sqlTx.QueryRowContext(tx.ctx, query, key, tx.now()).Scan(&exists)
	if tx.rollback(err) {
		return false
	}
	return exists
}

// UpdateLease for backend item. The transaction is set to a NotFound error
// state if the backend item does not exist.
func (tx *pgTx) UpdateLease(item backend.Item) {
	if tx.err != nil {
		return
	}

	const query = `UPDATE lease SET id=$2, expires=$3 WHERE key=$1`
	result, err := tx.sqlTx.ExecContext(tx.ctx, query, item.Key, item.ID, sqlNullTime(item.Expires))
	if tx.rollback(err) {
		return
	}
	rowsAffected, err := result.RowsAffected()
	if tx.rollback(err) {
		return
	}
	if rowsAffected == 0 {
		tx.rollback(sqlbk.ErrNotFound)
	}
}

// UpsertLease creates or updates a backend item.
func (tx *pgTx) UpsertLease(item backend.Item) {
	if tx.err != nil {
		return
	}

	// CockroachDB has UPSERT, but use ON CONFLICT to support PostgreSQL.
	const query = `
		INSERT INTO lease (key, id, expires) VALUES ($1,$2,$3)
		ON CONFLICT (key) DO UPDATE SET
		     id=EXCLUDED.id,
		expires=EXCLUDED.expires`

	_, err := tx.sqlTx.ExecContext(tx.ctx, query, item.Key, item.ID, sqlNullTime(item.Expires))
	tx.rollback(err)
}

// now returns the current clock time.
func (tx *pgTx) now() time.Time {
	return tx.cfg.Clock.Now()
}

// pgxLogger implements pgx.Logger to allow tests to log all SQL queries.
type pgxLogger struct {
	level logrus.Level
}

// Log to the logrus standard logger.
func (l *pgxLogger) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
	logrus.WithFields(data).Log(l.level, msg)
}
