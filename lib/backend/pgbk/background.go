package pgbk

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
)

func (b *Backend) backgroundExpiry(ctx context.Context) {
	defer b.wg.Done()
	defer b.log.Info("Exited expiry loop.")

	for {
		// see DeleteRange; we run a tight loop here because it could be
		// possible to have more than 1k new items expire every second, so we
		// could end up not ever catching up
		for i := 0; i < backend.DefaultRangeLimit/deleteBatchSize; i++ {
			t0 := time.Now()
			var n int64
			if err := b.retry(ctx, func(p *pgxpool.Pool) error {
				tag, err := p.Exec(ctx,
					"DELETE FROM kv WHERE key = ANY(ARRAY(SELECT key FROM kv WHERE expires IS NOT NULL AND expires <= now() LIMIT $1))",
					deleteBatchSize,
				)
				if err != nil {
					return trace.Wrap(err)
				}
				n = tag.RowsAffected()
				return nil
			}); err != nil {
				b.log.WithError(err).Error("Failed to delete expired items.")
				break
			}

			if n > 0 {
				b.log.WithFields(logrus.Fields{
					"deleted": n,
					"elapsed": time.Since(t0).String(),
				}).Debug("Deleted expired items.")
			}

			if n < deleteBatchSize {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backend.DefaultPollStreamPeriod):
		}

	}
}

func (b *Backend) backgroundChangeFeed(ctx context.Context) {
	defer b.wg.Done()
	defer b.log.Info("Exited change feed loop.")
	defer b.buf.Close()

	for {
		b.log.Info("Starting change feed stream.")
		err := b.runChangeFeed(ctx)
		if err == nil {
			break
		}
		b.log.WithError(err).Error("Change feed stream lost.")

		select {
		case <-ctx.Done():
			return
		case <-time.After(defaults.HighResPollingPeriod):
		}
	}
}

// runChangeFeed will connect to the database, start a change feed (for
// Postgres, falling back to CockroachDB) and emit events. Assumes that b.buf is
// not initialized but not closed, and will reset it before returning.
func (b *Backend) runChangeFeed(ctx context.Context) error {
	poolConn, err := b.pool.Acquire(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// we hijack the connection from the pool because the temporary replication
	// slot is tied to the connection, so we want it to be cleaned up no matter
	// what happens here
	conn := poolConn.Hijack()
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := conn.Close(ctx); err != nil {
			b.log.WithError(err).Warn("Error closing change feed connection.")
		}
	}()

	slotUUID := uuid.New()
	slotName := hex.EncodeToString(slotUUID[:])

	b.log.WithField("slot_name", slotName).Info("Setting up change feed.")
	if _, err := conn.Exec(ctx,
		"SELECT * FROM pg_create_logical_replication_slot($1, 'wal2json', true)",
		pgx.QueryExecModeExec,
		slotName,
	); err != nil {
		return trace.Wrap(err)
	}

	b.log.WithField("slot_name", slotName).Info("Change feed started.")
	b.buf.SetInit()
	defer b.buf.Reset()

	poll := func() (pgx.Rows, error) {
		pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return conn.Query(pollCtx,
			`SELECT
  data->>'action',
  decode(COALESCE(data->'columns'->0->>'value', data->'identity'->0->>'value'), 'hex'),
  decode(data->'columns'->1->>'value', 'hex'),
  (data->'columns'->2->>'value')::timestamp
FROM (
  SELECT data::jsonb as data
  FROM pg_logical_slot_get_changes($1::text, NULL, NULL,
    'format-version', '2', 'add-tables', 'public.kv', 'include-transaction', 'false')
) AS jdata;`,
			slotName)
	}

	for {
		t0 := time.Now()
		rows, err := poll()
		if err != nil {
			return trace.Wrap(err)
		}

		for rows.Next() {
			var action string
			var key []byte
			var value []byte
			var expires zeronull.Timestamp
			if err := rows.Scan(&action, &key, &value, &expires); err != nil {
				return trace.Wrap(err)
			}

			switch action {
			case "I", "U":
				b.buf.Emit(backend.Event{
					Type: types.OpPut,
					Item: backend.Item{
						Key:     key,
						Value:   value,
						Expires: time.Time(expires),
					},
				})
			case "D":
				b.buf.Emit(backend.Event{
					Type: types.OpDelete,
					Item: backend.Item{
						Key: key,
					},
				})
			case "M":
				b.log.Debug("Received WAL message.")
			case "B", "C":
				b.log.Debug("Received transaction message in change feed (should not happen).")
			case "T":
				// it could be possible to just reset the event buffer and
				// continue from the next row but it's not worth the effort
				// compared to just killing this connection and reconnecting,
				// and this should never actually happen anyway - deleting
				// everything from the backend would leave Teleport in a very
				// broken state
				return trace.BadParameter("received truncate WAL message, can't continue")
			default:
				return trace.BadParameter("received unknown WAL message %q", action)
			}

		}
		if err := rows.Err(); err != nil {
			return trace.Wrap(err)
		}

		if n := rows.CommandTag().RowsAffected(); n > 0 {
			b.log.WithFields(logrus.Fields{
				"events":  n,
				"elapsed": time.Since(t0).String(),
			}).Debug("Fetched change feed events.")
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backend.DefaultPollStreamPeriod):
		}
	}
}
