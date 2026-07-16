package dsqlbk

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (b *Backend) runExpiry(ctx context.Context) {
	defer b.log.InfoContext(ctx, "Exited expiry loop")

	scanQuery := "SELECT kv.key FROM " + b.kv + " AS kv WHERE kv.expiry <= transaction_timestamp() - $1::interval"
	deleteQuery := "DELETE FROM " + b.kv + " AS kv WHERE kv.key = ANY ($1::text[]) AND kv.expiry <= transaction_timestamp() - $2::interval"
	expiryGraceInterval := pgtype.Interval{
		Microseconds: time.Duration(b.cfg.ExpiryGraceInterval).Microseconds(),
		Valid:        true,
	}

	for ctx.Err() == nil {
		t0 := time.Now()

		// we scan for keys that have been expired for a little while to reduce
		// the chance of conflicts with other operations (which will generally
		// ignore expired items)
		var deleted, errored int64
		candidateKeys, err := collectRowsIdempotent(b, ctx,
			scanQuery,
			[]any{expiryGraceInterval},
			pgx.RowTo[string],
		)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.ErrorContext(ctx, "Failed to scan for expired items", "error", err, "elapsed", time.Since(t0))
		} else {
			shuffleSlice(candidateKeys)
			for candidateChunk := range slices.Chunk(candidateKeys, b.cfg.ExpiryChunkMaxRows) {
				if ctx.Err() != nil {
					break
				}
				tag, err := b.exec(ctx, deleteQuery, candidateChunk, expiryGraceInterval)
				if err != nil {
					b.log.ErrorContext(ctx, "Failed to delete candidate expired items", "error", err)
					errored += int64(len(candidateChunk))
				} else {
					deleted += tag.RowsAffected()
				}
			}
			if !b.cfg.DisableMetrics {
				expiredItemsDeletedTotal.Add(float64(deleted))
				expiredItemsFailedTotal.Add(float64(errored))
				expiredItemsSkippedTotal.Add(float64(int64(len(candidateKeys)) - deleted - errored))
			}
			if errored > 0 {
				b.log.WarnContext(ctx,
					"Failed to delete some candidate expired items",
					"candidates", len(candidateKeys),
					"deleted", deleted,
					"errored", errored,
					"elapsed", time.Since(t0),
				)
			} else if deleted > 0 {
				b.log.DebugContext(ctx,
					"Deleted expired items",
					"candidates", len(candidateKeys),
					"deleted", deleted,
					"elapsed", time.Since(t0),
				)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(b.cfg.ExpiryInterval)):
		}
	}
}
