package crdb

import (
	"context"
	"errors"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/itertools"
	"github.com/gravitational/teleport/lib/utils/slices"
)

const (
	// defaultDeleteBatchChunk is the default number of keys to include in
	// each DELETE batch. The shouldReduceBatchSize mechanism will dynamically
	// reduce this if needed.
	defaultDeleteBatchChunk = 200
)

// DeleteBatch deletes multiple keys from CockroachDB in a single batch.
//
// This implementation handles batch size dynamically to work with various CRDB
// configurations. If an error indicates the batch is too large, it automatically
// splits the batch and retries with smaller chunks:
func (b *Backend) DeleteBatch(ctx context.Context, keys []backend.Key) error {
	if len(keys) == 0 {
		return nil
	}
	keys = slices.DeduplicateKey(keys, func(k backend.Key) string { return k.String() })

	chunks := itertools.DynamicBatchSize(keys, defaultDeleteBatchChunk)
	for chunks.Next() {
		subChunk := chunks.Chunk()
		keyBytes := make([][]byte, 0, len(subChunk))
		for _, key := range subChunk {
			keyBytes = append(keyBytes, nonNilKey(key))
		}

		if _, err := pgcommon.Retry(ctx, b.log, func() (struct{}, error) {
			err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
				defer cancel()

				deleteStmt := `DELETE FROM kv WHERE kv.key = ANY($1::bytea[]) AND (kv.expires IS NULL OR kv.expires > now())`
				if _, err := c.Exec(ctx, deleteStmt, keyBytes); err != nil {
					return trace.Wrap(err)
				}
				return nil
			})

			if err != nil && shouldReduceBatchSize(err) {
				// Wrap batch-size-related errors to prevent the Retry helper from
				// retrying them. We'll handle these by reducing the batch size instead.
				return struct{}{}, &batchTooLargeError{err: err}
			}
			return struct{}{}, trace.Wrap(err)
		}); err != nil {
			var batchErr *batchTooLargeError
			if errors.As(err, &batchErr) {
				b.log.WarnContext(ctx, "Reducing batch size due to error.", "error", err)
				if err := chunks.ReduceSize(); err != nil {
					return trace.Wrap(batchErr.err)
				}

				continue
			}
			return trace.Wrap(err)
		}
	}

	return nil
}
