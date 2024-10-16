/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package lite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
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
				l.logger.WarnContext(l.ctx, "Error closing database", "error", err)
			}
			return
		case <-t.C:
			err := l.removeExpiredKeys()
			if err != nil {
				// connection problem means that database is closed
				// or is closing, downgrade the log to debug
				// to avoid polluting logs in production
				if trace.IsConnectionProblem(err) {
					l.logger.DebugContext(l.ctx, "Failed to run remove expired keys", "error", err)
				} else {
					l.logger.DebugContext(l.ctx, "Failed to run remove expired keys", "error", err)
				}
			}
			if !l.EventsOff {
				err = l.removeOldEvents()
				if err != nil {
					l.logger.WarnContext(l.ctx, "Failed to run remove old events", "error", err)
				}
				rowid, err = l.pollEvents(rowid)
				if err != nil {
					l.logger.WarnContext(l.ctx, "Failed to run poll events", "error", err)
				}
			}
		}
	}
}

func (l *Backend) removeExpiredKeys() error {
	now := l.clock.Now().UTC()
	return l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(l.ctx,
			"SELECT key FROM kv WHERE expires <= ? ORDER BY key LIMIT ?")
		if err != nil {
			return trace.Wrap(err)
		}
		defer q.Close()

		rows, err := q.QueryContext(l.ctx, now, l.BufferSize)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rows.Close()
		var keys []backend.Key
		for rows.Next() {
			var key backend.Key
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
		defer stmt.Close()

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
			defer q.Close()

			row := q.QueryRow()
			prevRowID := rowid
			if err := row.Scan(&rowid); err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
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
		l.logger.DebugContext(l.ctx, "Initialized event ID iterator", "event_id", rowid)
		l.buf.SetInit()
	}

	var events []backend.Event
	var lastID int64
	err := l.inTransaction(l.ctx, func(tx *sql.Tx) error {
		q, err := tx.PrepareContext(l.ctx,
			"SELECT id, type, kv_key, kv_value, kv_expires, kv_revision FROM events WHERE id > ? ORDER BY id LIMIT ?")
		if err != nil {
			return trace.Wrap(err)
		}
		defer q.Close()
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
			var expires sql.NullTime
			if err := rows.Scan(&lastID, &event.Type, &event.Item.Key, &event.Item.Value, &expires, &event.Item.Revision); err != nil {
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
