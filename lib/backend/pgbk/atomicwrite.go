/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package pgbk

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype/zeronull"

	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
)

func (b *Backend) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	newRevision := newRevision()

	var includesPut bool

	var n int

	_, err = pgcommon.Retry(ctx, b.log, func() (struct{}, error) {
		n++
		var conditionFailed bool
		var condBatch, actBatch pgx.Batch

		for _, ca := range condacts {
			switch ca.Condition.Kind {
			case backend.KindWhatever:
				// no comparison to assert
			case backend.KindExists:
				condBatch.Queue(
					"SELECT EXISTS (SELECT FROM kv WHERE key = $1 AND (expires IS NULL OR expires >= now()))",
					nonNil(ca.Key),
				).QueryRow(func(row pgx.Row) error {
					var cond bool
					if err := row.Scan(&cond); err != nil {
						return trace.Wrap(err)
					}
					conditionFailed = conditionFailed || !cond
					return nil
				})
			case backend.KindNotExists:
				condBatch.Queue(
					"SELECT NOT EXISTS (SELECT FROM kv WHERE key = $1 AND (expires IS NULL OR expires >= now()))",
					nonNil(ca.Key),
				).QueryRow(func(row pgx.Row) error {
					var cond bool
					if err := row.Scan(&cond); err != nil {
						return trace.Wrap(err)
					}
					conditionFailed = conditionFailed || !cond
					return nil
				})
			case backend.KindRevision:
				expectedRevision, ok := revisionFromString(ca.Condition.Revision)
				if !ok {
					return struct{}{}, trace.Wrap(backend.ErrConditionFailed)
				}
				condBatch.Queue(
					"SELECT EXISTS (SELECT FROM kv WHERE key = $1 AND revision = $2 AND (expires IS NULL OR expires >= now()))",
					nonNil(ca.Key), expectedRevision,
				).QueryRow(func(row pgx.Row) error {
					var cond bool
					if err := row.Scan(&cond); err != nil {
						return trace.Wrap(err)
					}
					conditionFailed = conditionFailed || !cond
					return nil
				})
			default:
				return struct{}{}, trace.BadParameter("unexpected condition kind %v in conditional action against key %q", ca.Condition.Kind, ca.Key)
			}

			switch ca.Action.Kind {
			case backend.KindNop:
				// no action to be taken
			case backend.KindPut:
				includesPut = true
				actBatch.Queue(
					"INSERT INTO kv (key, value, expires, revision) VALUES ($1, $2, $3, $4)"+
						" ON CONFLICT (key) DO UPDATE SET"+
						" value = excluded.value, expires = excluded.expires, revision = excluded.revision",
					nonNil(ca.Key), nonNil(ca.Action.Item.Value), zeronull.Timestamptz(ca.Action.Item.Expires.UTC()), newRevision,
				)
			case backend.KindDelete:
				actBatch.Queue(
					"DELETE FROM kv WHERE kv.key = $1 AND (kv.expires IS NULL OR kv.expires > now())", nonNil(ca.Key),
				)
			default:
				return struct{}{}, trace.BadParameter("unexpected action kind %v in conditional action against key %q", ca.Action.Kind, ca.Key)
			}
		}

		tx, err := b.pool.Begin(ctx)
		if err != nil {
			return struct{}{}, trace.Wrap(err)
		}

		// Rollback is designed to be safe to defer and will have no effect if we end up
		// committing the transaction before returning.
		defer tx.Rollback(ctx)

		if condBatch.Len() != 0 {
			if err := tx.SendBatch(ctx, &condBatch).Close(); err != nil {
				return struct{}{}, trace.Wrap(err)
			}
		}

		if conditionFailed {
			return struct{}{}, trace.Wrap(backend.ErrConditionFailed)
		}

		if err := tx.SendBatch(ctx, &actBatch).Close(); err != nil {
			return struct{}{}, trace.Wrap(err)
		}

		if err := tx.Commit(ctx); err != nil {
			return struct{}{}, trace.Wrap(err)
		}

		return struct{}{}, nil
	})

	if n > 2 {
		// if we retried more than once, txn experienced non-trivial conflict and we should warn about it. Infrequent warnings of this kind
		// are nothing to be concerned about, but high volumes may indicate that an automatic process is creating excessive conflicts.
		b.log.Warnf("AtomicWrite retried %d times due to postgres transaction contention. Some conflict is expected, but persistent conflict warnings may indicate an unhealthy state.", n)
	}

	if err != nil {
		return "", trace.Wrap(err)
	}

	if !includesPut {
		return "", nil
	}

	return revisionToString(newRevision), nil
}
