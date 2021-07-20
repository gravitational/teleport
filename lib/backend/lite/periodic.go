/*
Copyright 2018 Gravitational, Inc.

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

package lite

import (
	"database/sql"
	"time"

	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/trace"
)

const notSet = -2

func (l *Backend) runPeriodicOperations() {
	t := time.NewTicker(l.PollStreamPeriod)
	defer t.Stop()

	rowid := int64(notSet)
	for {
		select {
		case <-l.ctx.Done():
			if err := l.closeDatabase(); err != nil {
				l.Warningf("Error closing database: %v", err)
			}
			return
		case <-t.C:
			err := l.removeExpiredKeys()
			if err != nil {
				// connection problem means that database is closed
				// or is closing, downgrade the log to debug
				// to avoid polluting logs in production
				if trace.IsConnectionProblem(err) {
					l.Debugf("Failed to run remove expired keys: %v", err)
				} else {
					l.Warningf("Failed to run remove expired keys: %v", err)
				}
			}
			if !l.EventsOff {
				err = l.removeOldEvents()
				if err != nil {
					l.Warningf("Failed to run remove old events: %v", err)
				}
				rowid, err = l.pollEvents(rowid)
				if err != nil {
					l.Warningf("Failed to run poll events: %v", err)
				}
			}
		}
	}
}

func (l *Backend) removeExpiredKeys() error {
	// In mirror mode, don't expire any elements. This allows the cache to setup
	// a watch and expire elements as the events roll in.
	if l.Mirror {
		return nil
	}

	now := l.clock.Now().UTC()
	return l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(l.ctx,
			"SELECT key FROM kv WHERE expires <= ? ORDER BY key LIMIT ?")
		if err != nil {
			return trace.Wrap(err)
		}
		rows, err := q.QueryContext(l.ctx, now, l.BufferSize)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rows.Close()
		var keys [][]byte
		for rows.Next() {
			var key []byte
			if err := rows.Scan(&key); err != nil {
				return trace.Wrap(err)
			}
			keys = append(keys, key)
		}

		for i := range keys {
			if err := l.deleteInTransaction(l.ctx, keys[i], tx); err != nil {
				return trace.Wrap(err)
			}
		}

		return nil
	})
}

func (l *Backend) removeOldEvents() error {
	expiryTime := l.clock.Now().UTC().Add(-1 * backend.DefaultEventsTTL)
	return l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(l.ctx, "DELETE FROM events WHERE created <= ?")
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = stmt.ExecContext(l.ctx, expiryTime)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

func (l *Backend) pollEvents(rowid int64) (int64, error) {
	if rowid == notSet {
		err := l.inTransaction(l.ctx, func(tx *sql.Tx) error {
			q, err := tx.PrepareContext(
				l.ctx,
				"SELECT id from events ORDER BY id DESC LIMIT 1")
			if err != nil {
				return trace.Wrap(err)
			}
			row := q.QueryRow()
			prevRowID := rowid
			if err := row.Scan(&rowid); err != nil {
				if err != sql.ErrNoRows {
					// Scan does not explicitly promise not to modify its inputs if it returns an error (though this is likely
					// how it behaves).  Just in case, make sure that rowid is preserved so that we don't accidentally skip
					// some init logic on retry.
					rowid = prevRowID
					return trace.Wrap(err)
				}
				rowid = -1
			} else {
				rowid = rowid - 1
			}
			return nil
		})
		if err != nil {
			return rowid, trace.Wrap(err)
		}
		l.Debugf("Initialized event ID iterator to %v", rowid)
		l.signalWatchStart()
		l.buf.SetInit()
	}

	var events []backend.Event
	var lastID int64
	err := l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(l.ctx,
			"SELECT id, type, kv_key, kv_value, kv_modified, kv_expires FROM events WHERE id > ? ORDER BY id LIMIT ?")
		if err != nil {
			return trace.Wrap(err)
		}
		limit := l.BufferSize / 2
		if limit <= 0 {
			limit = 1
		}
		rows, err := q.QueryContext(l.ctx, rowid, limit)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rows.Close()
		for rows.Next() {
			var event backend.Event
			var expires NullTime
			if err := rows.Scan(&lastID, &event.Type, &event.Item.Key, &event.Item.Value, &event.Item.ID, &expires); err != nil {
				return trace.Wrap(err)
			}
			if expires.Valid {
				event.Item.Expires = expires.Time
			}
			events = append(events, event)
		}
		return nil
	})
	if err != nil {
		return rowid, trace.Wrap(err)
	}
	l.buf.Emit(events...)
	if len(events) != 0 {
		return lastID, nil
	}
	return rowid, nil
}
