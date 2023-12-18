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

package pgbk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/defaults"
)

func (b *Backend) backgroundExpiry(ctx context.Context) {
	defer b.log.Info("Exited expiry loop.")

	for ctx.Err() == nil {
		// "DELETE FROM kv WHERE expires <= now()" but more complicated: logical
		// decoding can become really really slow if a transaction is big enough
		// to spill on disk - max_changes_in_memory (4096) changes before
		// Postgres 13, or logical_decoding_work_mem (64MiB) bytes of total size
		// in Postgres 13 and later; thankfully, we can just limit our
		// transactions to a small-ish number of affected rows (1000 seems to
		// work ok) as we don't need atomicity for this; we run a tight loop
		// here because it could be possible to have more than ExpiryBatchSize
		// new items expire every ExpiryInterval, so we could end up not ever
		// catching up
		for i := 0; i < backend.DefaultRangeLimit/b.cfg.ExpiryBatchSize; i++ {
			t0 := time.Now()
			// TODO(espadolini): try getting keys in a read-only deferrable
			// transaction and deleting them later to reduce potential
			// serialization issues
			deleted, err := pgcommon.RetryIdempotent(ctx, b.log, func() (int64, error) {
				// LIMIT without ORDER BY might get executed poorly because the
				// planner doesn't have any idea of how many rows will be chosen
				// or skipped, and it's not necessary but it's a nice touch that
				// we'll be deleting expired items in expiration order
				tag, err := b.pool.Exec(ctx,
					"DELETE FROM kv WHERE kv.key IN (SELECT kv_inner.key FROM kv AS kv_inner"+
						" WHERE kv_inner.expires IS NOT NULL AND kv_inner.expires <= now()"+
						" ORDER BY kv_inner.expires LIMIT $1 FOR UPDATE)",
					b.cfg.ExpiryBatchSize,
				)
				if err != nil {
					return 0, trace.Wrap(err)
				}
				return tag.RowsAffected(), nil
			})
			if err != nil {
				b.log.WithError(err).Error("Failed to delete expired items.")
				break
			}

			if deleted > 0 {
				b.log.WithFields(logrus.Fields{
					"deleted": deleted,
					"elapsed": time.Since(t0).String(),
				}).Debug("Deleted expired items.")
			}

			if deleted < int64(b.cfg.ExpiryBatchSize) {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(b.cfg.ExpiryInterval)):
		}
	}
}

func (b *Backend) backgroundChangeFeed(ctx context.Context) {
	defer b.log.Info("Exited change feed loop.")
	defer b.buf.Close()

	for ctx.Err() == nil {
		b.log.Info("Starting change feed stream.")
		err := b.runChangeFeed(ctx)
		if ctx.Err() != nil {
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

// runChangeFeed will connect to the database, start a change feed and emit
// events. Assumes that b.buf is not initialized but not closed, and will reset
// it before returning.
func (b *Backend) runChangeFeed(ctx context.Context) error {
	connConfig := b.feedConfig.ConnConfig.Copy()
	if bc := b.feedConfig.BeforeConnect; bc != nil {
		if err := bc(ctx, connConfig); err != nil {
			return trace.Wrap(err)
		}
	}
	// TODO(espadolini): use a replication connection if
	// connConfig.RuntimeParams["replication"] == "database"
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := conn.Close(closeCtx); err != nil && closeCtx.Err() != nil {
			b.log.WithError(err).Warn("Error closing change feed connection.")
		}
	}()
	if ac := b.feedConfig.AfterConnect; ac != nil {
		if err := ac(ctx, conn); err != nil {
			return trace.Wrap(err)
		}
	}

	// 'kv'::regclass will get the oid for the kv table as searched given the
	// current search_path, which matches the behavior of any query that refers
	// to the kv table with no qualifier (like the rest of the pgbk code does)
	var schemaName string
	if err := conn.QueryRow(ctx,
		"SELECT nsp.nspname "+
			"FROM pg_class AS cl JOIN pg_namespace AS nsp ON cl.relnamespace = nsp.oid "+
			"WHERE cl.oid = 'kv'::regclass",
		pgx.QueryExecModeExec,
	).Scan(&schemaName); err != nil {
		return trace.Wrap(err)
	}
	addTables := wal2jsonEscape(schemaName) + ".kv"

	// reading from a replication slot adds to the postgres log at "log" level
	// (right below "fatal") for every poll, and we poll every second here, so
	// we try to silence the logs for this connection; this can fail because of
	// permission issues, which would delete the temporary slot (it's deleted on
	// any error), so we have to do it before that
	if _, err := conn.Exec(ctx, "SET log_min_messages TO fatal", pgx.QueryExecModeExec); err != nil {
		b.log.WithError(err).Debug("Failed to silence log messages for change feed session.")
	}

	// this can be useful on Azure if we have azure_pg_admin permissions but not
	// the REPLICATION attribute; in vanilla Postgres you have to be SUPERUSER
	// to grant REPLICATION, and if you are SUPERUSER you can do replication
	// things even without the attribute anyway
	//
	// HACK(espadolini): ALTER ROLE CURRENT_USER crashes Postgres on Azure, so
	// we have to use an explicit username
	if b.cfg.AuthMode == AzureADAuth && connConfig.User != "" {
		if _, err := conn.Exec(ctx,
			fmt.Sprintf("ALTER ROLE %v REPLICATION", pgx.Identifier{connConfig.User}.Sanitize()),
			pgx.QueryExecModeExec,
		); err != nil {
			b.log.WithError(err).Debug("Failed to enable replication for the current user.")
		}
	}

	// a replication slot must be 1-63 lowercase letters, numbers and
	// underscores, as per
	// https://github.com/postgres/postgres/blob/b0ec61c9c27fb932ae6524f92a18e0d1fadbc144/src/backend/replication/slot.c#L193-L194
	slotName := fmt.Sprintf("teleport_%x", [16]byte(uuid.New()))

	b.log.WithField("slot_name", slotName).Info("Setting up change feed.")

	// be noisy about pg_create_logical_replication_slot taking too long, since
	// hanging here leaves the backend non-functional
	createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	if _, err := conn.Exec(createCtx,
		"SELECT * FROM pg_create_logical_replication_slot($1, 'wal2json', true)",
		pgx.QueryExecModeExec, slotName,
	); err != nil {
		cancel()
		return trace.Wrap(err)
	}
	cancel()

	b.log.WithField("slot_name", slotName).Info("Change feed started.")
	b.buf.SetInit()
	defer b.buf.Reset()

	for ctx.Err() == nil {
		messages, err := b.pollChangeFeed(ctx, conn, addTables, slotName, b.cfg.ChangeFeedBatchSize)
		if err != nil {
			return trace.Wrap(err)
		}

		// tight loop if we hit the batch size
		if messages >= int64(b.cfg.ChangeFeedBatchSize) {
			continue
		}

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-time.After(time.Duration(b.cfg.ChangeFeedPollInterval)):
		}
	}
	return trace.Wrap(err)
}

// pollChangeFeed will poll the change feed and emit any fetched events, if any.
// It returns the count of received messages.
func (b *Backend) pollChangeFeed(ctx context.Context, conn *pgx.Conn, addTables, slotName string, batchSize int) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	t0 := time.Now()

	rows, _ := conn.Query(ctx,
		"SELECT data FROM pg_logical_slot_get_changes($1, NULL, $2, "+
			"'format-version', '2', 'add-tables', $3, 'include-transaction', 'false')",
		slotName, batchSize, addTables)

	var data []byte
	tag, err := pgx.ForEachRow(rows, []any{(*pgtype.DriverBytes)(&data)}, func() error {
		var w wal2jsonMessage
		if err := json.Unmarshal(data, &w); err != nil {
			return trace.Wrap(err, "unmarshaling wal2json message")
		}

		events, err := w.Events()
		if err != nil {
			return trace.Wrap(err, "processing wal2json message")
		}

		b.buf.Emit(events...)
		return nil
	})
	if err != nil {
		return 0, trace.Wrap(err)
	}

	messages := tag.RowsAffected()
	if messages > 0 {
		b.log.WithFields(logrus.Fields{
			"messages": messages,
			"elapsed":  time.Since(t0).String(),
		}).Debug("Fetched change feed events.")
	}

	return messages, nil
}
