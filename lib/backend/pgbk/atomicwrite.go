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

	type batchItem struct {
		query     string
		arguments []any
	}
	var condBatchItems, actBatchItems []batchItem
	var actBatchIncludesPut bool

	for _, ca := range condacts {
		switch ca.Condition.Kind {
		case backend.KindWhatever:
			// no comparison to assert
		case backend.KindExists:
			condBatchItems = append(condBatchItems, batchItem{
				"SELECT EXISTS (SELECT FROM kv WHERE key = $1 AND (expires IS NULL OR expires >= now()))",
				[]any{nonNil(ca.Key)},
			})
		case backend.KindNotExists:
			condBatchItems = append(condBatchItems, batchItem{
				"SELECT NOT EXISTS (SELECT FROM kv WHERE key = $1 AND (expires IS NULL OR expires >= now()))",
				[]any{nonNil(ca.Key)},
			})
		case backend.KindRevision:
			expectedRevision, ok := revisionFromString(ca.Condition.Revision)
			if !ok {
				return "", trace.Wrap(backend.ErrConditionFailed)
			}
			condBatchItems = append(condBatchItems, batchItem{
				"SELECT EXISTS (SELECT FROM kv WHERE key = $1 AND revision = $2 AND (expires IS NULL OR expires >= now()))",
				[]any{nonNil(ca.Key), expectedRevision},
			})
		default:
			// condacts was already checked for validity
			return "", trace.BadParameter("unexpected condition kind %v in conditional action against key %q (this is a bug)", ca.Condition.Kind, ca.Key)
		}

		switch ca.Action.Kind {
		case backend.KindNop:
			// no action to be taken
		case backend.KindPut:
			actBatchIncludesPut = true
			actBatchItems = append(actBatchItems, batchItem{
				"INSERT INTO kv (key, value, expires, revision) VALUES ($1, $2, $3, $4)" +
					" ON CONFLICT (key) DO UPDATE SET" +
					" value = excluded.value, expires = excluded.expires, revision = excluded.revision",
				[]any{nonNil(ca.Key), nonNil(ca.Action.Item.Value), zeronull.Timestamptz(ca.Action.Item.Expires.UTC()), newRevision},
			})
		case backend.KindDelete:
			actBatchItems = append(actBatchItems, batchItem{
				"DELETE FROM kv WHERE kv.key = $1 AND (kv.expires IS NULL OR kv.expires > now())",
				[]any{nonNil(ca.Key)},
			})
		default:
			// condacts was already checked for validity
			return "", trace.BadParameter("unexpected action kind %v in conditional action against key %q (this is a bug)", ca.Action.Kind, ca.Key)
		}
	}

	var success bool
	querySuccess := func(row pgx.Row) error {
		if !success {
			return nil
		}
		return trace.Wrap(row.Scan(&success))
	}

	var tries int
	err = pgcommon.RetryTx(ctx, b.log, b.pool, pgx.TxOptions{}, false, func(tx pgx.Tx) error {
		tries++

		var condBatch, actBatch pgx.Batch
		for _, bi := range condBatchItems {
			condBatch.Queue(bi.query, bi.arguments...).QueryRow(querySuccess)
		}
		for _, bi := range actBatchItems {
			actBatch.Queue(bi.query, bi.arguments...)
		}

		success = true
		if condBatch.Len() > 0 {
			if err := tx.SendBatch(ctx, &condBatch).Close(); err != nil {
				return trace.Wrap(err)
			}
			if !success {
				return nil
			}
		}

		if err := tx.SendBatch(ctx, &actBatch).Close(); err != nil {
			return trace.Wrap(err)
		}

		return nil
	})

	if tries > 1 {
		backend.AtomicWriteContention.WithLabelValues(b.GetName()).Add(float64(tries - 1))
	}

	if tries > 2 {
		// if we retried more than once, txn experienced non-trivial conflict and we should warn about it. Infrequent warnings of this kind
		// are nothing to be concerned about, but high volumes may indicate that an automatic process is creating excessive conflicts.
		b.log.Warnf("AtomicWrite retried %d times due to postgres transaction contention. Some conflict is expected, but persistent conflict warnings may indicate an unhealthy state.", tries)
	}

	if err != nil {
		return "", trace.Wrap(err)
	}

	if !success {
		return "", trace.Wrap(backend.ErrConditionFailed)
	}

	if !actBatchIncludesPut {
		return "", nil
	}

	return revisionToString(newRevision), nil
}
