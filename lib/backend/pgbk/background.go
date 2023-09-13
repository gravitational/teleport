// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgbk

import (
	"context"
	"encoding/hex"
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
	// we manually copy the pool configuration and connect because we don't want
	// to hit a connection limit or mess with the connection pool stats; we need
	// a separate, long-running connection here anyway.
	poolConfig := b.pool.Config()
	if poolConfig.BeforeConnect != nil {
		if err := poolConfig.BeforeConnect(ctx, poolConfig.ConnConfig); err != nil {
			return trace.Wrap(err)
		}
	}
	conn, err := pgx.ConnectConfig(ctx, poolConfig.ConnConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := conn.Close(ctx); err != nil && ctx.Err() != nil {
			b.log.WithError(err).Warn("Error closing change feed connection.")
		}
	}()

	// reading from a replication slot adds to the postgres log at "log" level
	// (right below "fatal") for every poll, and we poll every second here, so
	// we try to silence the logs for this connection; this can fail because of
	// permission issues, which would delete the temporary slot (it's deleted on
	// any error), so we have to do it before that
	if _, err := conn.Exec(ctx, "SET log_min_messages TO fatal", pgx.QueryExecModeExec); err != nil {
		b.log.WithError(err).Debug("Failed to silence log messages for change feed session.")
	}

	// this can be useful if we're some sort of admin but we haven't gotten the
	// REPLICATION attribute yet
	// HACK(espadolini): ALTER ROLE CURRENT_USER REPLICATION just crashes postgres on Azure
	if _, err := conn.Exec(ctx,
		fmt.Sprintf("ALTER ROLE %v REPLICATION", pgx.Identifier{poolConfig.ConnConfig.User}.Sanitize()),
		pgx.QueryExecModeExec,
	); err != nil {
		b.log.WithError(err).Debug("Failed to enable replication for the current user.")
	}

	u := uuid.New()
	slotName := hex.EncodeToString(u[:])

	b.log.WithField("slot_name", slotName).Info("Setting up change feed.")
	if _, err := conn.Exec(ctx,
		"SELECT * FROM pg_create_logical_replication_slot($1, 'wal2json', true)",
		pgx.QueryExecModeExec, slotName,
	); err != nil {
		return trace.Wrap(err)
	}

	b.log.WithField("slot_name", slotName).Info("Change feed started.")
	b.buf.SetInit()
	defer b.buf.Reset()

	for ctx.Err() == nil {
		messages, err := b.pollChangeFeed(ctx, conn, slotName, b.cfg.ChangeFeedBatchSize)
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
func (b *Backend) pollChangeFeed(ctx context.Context, conn *pgx.Conn, slotName string, batchSize int) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	t0 := time.Now()

	rows, _ := conn.Query(ctx,
		"SELECT data FROM pg_logical_slot_get_changes($1, NULL, $2, "+
			"'format-version', '2', 'add-tables', 'public.kv', 'include-transaction', 'false')",
		slotName, batchSize)

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
