package dsqlbk

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/backendmetrics"
)

// AtomicWrite implements [backend.Backend].
func (b *Backend) AtomicWrite(ctx context.Context, condacts []backend.ConditionalAction) (revision string, err error) {
	if err := backend.ValidateAtomicWrite(condacts); err != nil {
		return "", trace.Wrap(err)
	}

	newRevision := randomRevision()

	type batchItem struct {
		query string
		args  []any
	}
	batchItems := make([]batchItem, 0, len(condacts))
	var batchHasPut bool

	for i := range condacts {
		ca := &condacts[i]
		switch ca.Condition.Kind {
		case backend.KindWhatever:
			switch ca.Action.Kind {
			case backend.KindNop:
				return "", trace.BadParameter("conditional action for item %+q is ineffectual (this is a bug)", ca.Key.String())
			case backend.KindPut:
				batchItems = append(batchItems, batchItem{
					query: "INSERT INTO " + b.kv + " AS kv (key, value, expiry, revision) VALUES ($1, $2, $3, $4) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiry = EXCLUDED.expiry, revision = EXCLUDED.revision",
					args:  []any{ca.Key.String(), nonNil(ca.Action.Item.Value), toExpiry(ca.Action.Item.Expires), newRevision},
				})
				batchHasPut = true
			case backend.KindDelete:
				// the weird formulation of this is so that we unconditionally
				// get a row back since we don't care if we delete the row or
				// not, so we don't have to distinguish this one case from every
				// other query (where we confirm success by checking that we got
				// a row back)
				batchItems = append(batchItems, batchItem{
					query: "WITH deleted AS (DELETE FROM " + b.kv + " AS kv WHERE kv.key = $1 AND kv.expiry > transaction_timestamp()) SELECT",
					args:  []any{ca.Key.String()},
				})
			default:
				return "", trace.BadParameter("unexpected action kind %v in conditional action against item %+q (this is a bug)", ca.Action.Kind, ca.Key.String())
			}
		case backend.KindExists:
			switch ca.Action.Kind {
			case backend.KindNop:
				batchItems = append(batchItems, batchItem{
					query: "SELECT FROM " + b.kv + " AS kv WHERE kv.key = $1 AND kv.expiry > transaction_timestamp() FOR UPDATE",
					args:  []any{ca.Key.String()},
				})
			case backend.KindPut:
				batchItems = append(batchItems, batchItem{
					query: "UPDATE " + b.kv + " AS kv SET value = $1, expiry = $2, revision = $3 WHERE kv.key = $4 AND kv.expiry > transaction_timestamp()",
					args:  []any{nonNil(ca.Action.Item.Value), toExpiry(ca.Action.Item.Expires), newRevision, ca.Key.String()},
				})
				batchHasPut = true
			case backend.KindDelete:
				batchItems = append(batchItems, batchItem{
					query: "DELETE FROM " + b.kv + " AS kv WHERE kv.key = $1 AND kv.expiry > transaction_timestamp()",
					args:  []any{ca.Key.String()},
				})
			default:
				return "", trace.BadParameter("unexpected action kind %v in conditional action against item %+q (this is a bug)", ca.Action.Kind, ca.Key.String())
			}
		case backend.KindNotExists:
			switch ca.Action.Kind {
			case backend.KindNop, backend.KindDelete:
				// Putting a condition on nonexistence is quite awkward in the
				// DSQL conflict model (which emulates the PostgreSQL locking
				// model), because if the row doesn't exist the best we can do
				// is to forcibly create it as a tombstone row that's guaranteed
				// to be expired (by having an expiry of -infinity), that's
				// understood by the change feed to not be a real item and that
				// will eventually get cleaned up by the expiry loop. We do this
				// slightly awkward union of two queries because if possible we
				// want to avoid generating spurious delete events on every
				// operation if lots of atomic writes happen to assert the
				// nonexistence of the same item (which can happen in the
				// pattern where a lock item can be acquired for large
				// operations by creating it, while smaller operations can
				// happen as atomic writes that assert that nobody is holding
				// the lock), and by creating the row if it truly doesn't exist
				// or by selecting it for update if it exists but it's expired,
				// we end up only generating one delete events per expiry period
				// (creating the tombstone does not generate an event, but
				// deleting it does).
				batchItems = append(batchItems, batchItem{
					query: "WITH inserter AS (INSERT INTO " + b.kv + " AS kv (key, value, expiry, revision) VALUES ($1, '', '-infinity', $2) ON CONFLICT (key) DO NOTHING RETURNING 1), selecter AS (SELECT 1 FROM " + b.kv + " AS kv WHERE kv.key = $1 AND kv.expiry <= transaction_timestamp() FOR UPDATE) SELECT FROM inserter UNION ALL SELECT FROM selecter",
					args:  []any{ca.Key.String(), newRevision},
				})
			case backend.KindPut:
				batchItems = append(batchItems, batchItem{
					query: "INSERT INTO " + b.kv + " AS kv (key, value, expiry, revision) VALUES ($1, $2, $3, $4) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, expiry = EXCLUDED.expiry, revision = EXCLUDED.revision WHERE kv.expiry <= transaction_timestamp()",
					args:  []any{ca.Key.String(), nonNil(ca.Action.Item.Value), toExpiry(ca.Action.Item.Expires), newRevision},
				})
				batchHasPut = true
			default:
				return "", trace.BadParameter("unexpected action kind %v in conditional action against item %+q (this is a bug)", ca.Action.Kind, ca.Key.String())
			}
		case backend.KindRevision:
			caRevision, ok := revisionFromString(ca.Condition.Revision)
			if !ok {
				return "", trace.Wrap(backend.ErrConditionFailed)
			}
			switch ca.Action.Kind {
			case backend.KindNop:
				batchItems = append(batchItems, batchItem{
					query: "SELECT FROM " + b.kv + " AS kv WHERE kv.key = $1 AND kv.revision = $2 AND kv.expiry > transaction_timestamp() FOR UPDATE",
					args:  []any{ca.Key.String(), caRevision},
				})
			case backend.KindPut:
				batchItems = append(batchItems, batchItem{
					query: "UPDATE " + b.kv + " AS kv SET value = $1, expiry = $2, revision = $3 WHERE kv.key = $4 AND kv.revision = $5 AND kv.expiry > transaction_timestamp()",
					args:  []any{nonNil(ca.Action.Item.Value), toExpiry(ca.Action.Item.Expires), newRevision, ca.Key.String(), caRevision},
				})
				batchHasPut = true
			case backend.KindDelete:
				batchItems = append(batchItems, batchItem{
					query: "DELETE FROM " + b.kv + " AS kv WHERE kv.key = $1 AND kv.revision = $2 AND kv.expiry > transaction_timestamp()",
					args:  []any{ca.Key.String(), caRevision},
				})
			default:
				return "", trace.BadParameter("unexpected action kind %v in conditional action against item %+q (this is a bug)", ca.Action.Kind, ca.Key.String())
			}
		default:
			return "", trace.BadParameter("unexpected condition kind %v in conditional action against item %+q (this is a bug)", ca.Condition.Kind, ca.Key.String())
		}
	}

	var attempts int
	const isIdempotentFalse = false
	success, err := retry(ctx, b.log, isIdempotentFalse, func() (bool, error) {
		attempts++

		conn, err := b.pool.Acquire(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		defer conn.Release()

		ctx, cancel := context.WithTimeout(ctx, time.Duration(b.cfg.QueryTimeout))
		defer cancel()

		tx, err := conn.Begin(ctx)
		if err != nil {
			return false, trace.Wrap(err)
		}
		defer tx.Rollback(ctx)

		success := true
		batch := &pgx.Batch{
			QueuedQueries: make([]*pgx.QueuedQuery, 0, len(batchItems)),
		}
		for _, bi := range batchItems {
			batch.Queue(bi.query, bi.args...).
				Exec(func(tag pgconn.CommandTag) error {
					success = success && tag.RowsAffected() > 0
					return nil
				})
		}

		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return false, trace.Wrap(err)
		}

		if !success {
			// rollback
			return false, nil
		}

		if err := tx.Commit(ctx); err != nil {
			return false, trace.Wrap(err)
		}

		return true, nil
	})

	if attempts > 1 {
		backendmetrics.AtomicWriteContention.WithLabelValues(b.GetName()).Add(float64(attempts - 1))
	}

	if attempts > 3 {
		b.log.WarnContext(ctx,
			"AtomicWrite was retried several times due to transaction contention. Some conflict is expected, but persistent conflict warnings may indicate an unhealthy state.",
			"attempts", attempts,
		)
	}

	if err != nil {
		return "", trace.Wrap(err)
	}

	if !success {
		return "", trace.Wrap(backend.ErrConditionFailed)
	}

	if !batchHasPut {
		return "", nil
	}

	return revisionToString(newRevision), nil
}
