package crdb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/itertools"
)

const (
	// defaultUpsertBatchChunk is the default number of items to include in
	// each UPSERT batch. The shouldReduceBatchSize mechanism will dynamically
	// reduce this if needed.
	defaultUpsertBatchChunk = 200
)

// batchTooLargeError wraps a postgres error that indicates the batch size
// should be reduced. This custom error type prevents the Retry helper from
// automatically retrying the error, allowing us to handle it by reducing
// the batch size instead.
//
// Note: We intentionally do NOT implement Unwrap() to prevent errors.As from
// finding the underlying PgError, which would cause the Retry helper to retry it.
type batchTooLargeError struct {
	err error
}

func (e *batchTooLargeError) Error() string {
	return fmt.Sprintf("batch too large: %s", e.err)
}

// PutBatch inserts or updates multiple items into CockroachDB in a single batch.
//
// This implementation handles batch size dynamically to work with various CRDB
// configurations. If an error indicates the batch is too large, it automatically
// splits the batch and retries with smaller chunks:
//
//   - 08P01 (Protocol Violation): The data size exceeds the wire protocol limits
//   - 40001 (Serialization Failure) with "commit deadline exceeded": The transaction
//     is too large/slow and should be broken up rather than retried immediately.
//     See https://www.cockroachlabs.com/docs/v25.3/transaction-retry-error-reference#retry_commit_deadline_exceeded
//
// See also: https://www.cockroachlabs.com/docs/v25.3/performance-best-practices-overview.html#bulk-insert-best-practices
func (b *Backend) PutBatch(ctx context.Context, items []backend.Item) ([]string, error) {
	if len(items) == 0 {
		return []string{}, nil
	}

	revOut := make([]string, 0, len(items))
	chunks := itertools.DynamicBatchSize(items, defaultUpsertBatchChunk)
	for chunks.Next() {
		subChunk := chunks.Chunk()
		keys := make([][]byte, 0, len(subChunk))
		values := make([][]byte, 0, len(subChunk))
		expires := make([]zeronull.Timestamptz, 0, len(subChunk))
		revs := make([]revision, 0, len(subChunk))

		for _, item := range subChunk {
			keys = append(keys, nonNilKey(item.Key))
			values = append(values, nonNil(item.Value))
			expires = append(expires, zeronull.Timestamptz(item.Expires.UTC()))

			revs = append(revs, newRevision())
		}
		if _, err := pgcommon.Retry(ctx, b.log, func() (struct{}, error) {
			err := b.pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
				// Timers can be expensive at scale. To mitigate this allocate the
				// timeout context after a connection has been acquired. This limits
				// the number of timers to at most the size of the connection pool.
				ctx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
				defer cancel()

				upsertStmt := `UPSERT INTO kv (key, value, expires, revision) SELECT * FROM UNNEST($1::bytea[], $2::bytea[], $3::timestamptz[], $4::uuid[]);`
				if _, err := c.Exec(ctx, upsertStmt, keys, values, expires, revs); err != nil {
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
					return nil, trace.Wrap(batchErr.err)
				}

				continue
			}
			return nil, trace.Wrap(err)
		}

		for _, rev := range revs {
			revOut = append(revOut, revisionToString(rev))
		}
		// Advance to the next batch
		items = items[len(subChunk):]
	}

	return revOut, nil
}

// shouldReduceBatchSize checks if the error indicates that the batch is too large
// and should be retried with a smaller batch size.
func shouldReduceBatchSize(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	// 08P01: Protocol violation. The data is too large for the wire protocol.
	// We should reduce batch size and retry.
	if pgErr.Code == pgerrcode.ProtocolViolation {
		return true
	}

	// 40001 (SerializationFailure) with "commit deadline exceeded": Transaction is too large/slow.
	// This error is wrapped in batchTooLargeError inside the Retry closure to prevent
	// the Retry helper from retrying it automatically.
	// https://www.cockroachlabs.com/docs/v25.3/transaction-retry-error-reference#retry_commit_deadline_exceeded
	if pgErr.Code == pgerrcode.SerializationFailure && strings.Contains(pgErr.Message, "commit deadline exceeded") {
		return true
	}

	return false
}
