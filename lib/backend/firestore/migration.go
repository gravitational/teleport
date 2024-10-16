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

package firestore

import (
	"context"
	"log/slog"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
)

// migrateIncorrectKeyTypes migrates incorrect key types (backend.Key and string) to the correct type (bytes)
// in the backend. This is necessary because the backend was incorrectly storing keys as strings and backend.Key
// types and Firestore clients mapped them to different database types. This forces calling ReadRange 3 times.
// This migration will fix the issue by converting all keys to the correct type (bytes).
// TODO(tigrato|rosstimothy): DELETE In 19.0.0: Remove this migration in 19.0.0.
func (b *Backend) migrateIncorrectKeyTypes() {
	var (
		numberOfDocsMigrated int
		duration             time.Duration
	)
	err := backend.RunWhileLocked(
		b.clientContext,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				LockNameComponents: []string{"firestore_migrate_incorrect_key_types"},
				Backend:            b,
				TTL:                5 * time.Minute,
				RetryInterval:      time.Minute,
			},
			ReleaseCtxTimeout:   10 * time.Second,
			RefreshLockInterval: time.Minute,
		},
		func(ctx context.Context) error {
			start := time.Now()
			defer func() {
				duration = time.Since(start)
			}()
			// backend.Key is converted to array of ints when sending to the db.
			toArray := func(key []byte) []any {
				arrKey := make([]any, len(key))
				for i, b := range key {
					arrKey[i] = int(b)
				}
				return arrKey
			}
			nDocs, err := migrateKeyType[[]any](ctx, b, toArray)
			numberOfDocsMigrated += nDocs
			if err != nil {
				return trace.Wrap(err, "failed to migrate backend key")
			}

			stringKey := func(key []byte) string {
				return string(key)
			}
			nDocs, err = migrateKeyType[string](ctx, b, stringKey)
			numberOfDocsMigrated += nDocs
			if err != nil {
				return trace.Wrap(err, "failed to migrate legacy key")
			}
			return nil
		})

	entry := b.logger.With(
		slog.Duration("duration", duration),
		slog.Int("migrated", numberOfDocsMigrated),
	)
	if err != nil {
		entry.ErrorContext(b.clientContext, "Failed to migrate incorrect key types.", "error", err)
		return
	}
	entry.InfoContext(b.clientContext, "Successfully migrated incorrect key types.")
}

func migrateKeyType[T any](ctx context.Context, b *Backend, newKey func([]byte) T) (int, error) {
	limit := 300
	startKey := newKey([]byte("/"))

	bulkWriter := b.svc.BulkWriter(b.clientContext)
	defer bulkWriter.End()

	nDocs := 0
	// handle the migration in batches of 300 documents per second
	t := time.NewTimer(time.Second)
	defer t.Stop()
	for {

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-t.C:
		}

		docs, err := b.svc.Collection(b.CollectionName).
			// passing the key type here forces the client to map the key to the underlying type
			// and return all the keys in that share the same underlying type.
			// backend.Key is mapped to Array in Firestore.
			// []byte is mapped to Bytes in Firestore.
			// string is mapped to String in Firestore.
			// Searching for keys with the same underlying type will return all keys with the same type.
			Where(keyDocProperty, ">", startKey).
			Limit(limit).
			Documents(ctx).GetAll()
		if err != nil {
			return nDocs, trace.Wrap(err)
		}

		jobs := make([]*firestore.BulkWriterJob, len(docs))
		for i, dbDoc := range docs {
			newDoc, err := newRecordFromDoc(dbDoc)
			if err != nil {
				return nDocs, trace.Wrap(err, "failed to convert document")
			}

			// use conditional update to ensure that the document has not been updated since the read
			jobs[i], err = bulkWriter.Update(
				b.svc.Collection(b.CollectionName).
					Doc(b.keyToDocumentID(newDoc.backendItem().Key)),
				newDoc.updates(),
				firestore.LastUpdateTime(dbDoc.UpdateTime),
			)
			if err != nil {
				return nDocs, trace.Wrap(err, "failed stream bulk action")
			}

			startKey = newKey(newDoc.Key) // update start key
		}

		bulkWriter.Flush() // flush the buffer

		for _, job := range jobs {
			if _, err := job.Results(); err != nil {
				// log the error and continue
				b.logger.ErrorContext(ctx, "failed to write bulk action", "error", err)
			}
		}

		nDocs += len(docs)
		if len(docs) < limit {
			break
		}

		t.Reset(time.Second)
	}
	return nDocs, nil
}
