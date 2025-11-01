/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"slices"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgtype/zeronull"

	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
)

const (
	defaultUpsertBatchChunk = 100
	putBatchStmt            = `
INSERT INTO kv (key, value, expires, revision)
SELECT * FROM UNNEST(
  $1::bytea[],
  $2::bytea[],
  $3::timestamptz[],
  $4::uuid[]
)
ON CONFLICT (key) DO UPDATE
SET
	value    = EXCLUDED.value,
	expires  = EXCLUDED.expires,
	revision = EXCLUDED.revision;
`
)

// PutBatch puts multiple items into the backend in a single transaction.
func (b *Backend) PutBatch(ctx context.Context, items []backend.Item) ([]string, error) {
	if len(items) == 0 {
		return []string{}, nil
	}
	revOut := make([]string, 0, len(items))
	for chunk := range slices.Chunk(items, defaultUpsertBatchChunk) {
		keys := make([][]byte, 0, len(chunk))
		values := make([][]byte, 0, len(chunk))
		expires := make([]zeronull.Timestamptz, 0, len(chunk))
		revs := make([]revision, 0, len(chunk))

		for _, item := range chunk {
			keys = append(keys, nonNilKey(item.Key))
			values = append(values, nonNil(item.Value))
			expires = append(expires, zeronull.Timestamptz(item.Expires.UTC()))

			revVal := newRevision()
			revs = append(revs, revVal)
			revOut = append(revOut, revisionToString(revVal))
		}

		if _, err := pgcommon.Retry(ctx, b.log, func() (struct{}, error) {
			_, err := b.pool.Exec(ctx, putBatchStmt, keys, values, expires, revs)
			return struct{}{}, trace.Wrap(err)
		}); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return revOut, nil
}
