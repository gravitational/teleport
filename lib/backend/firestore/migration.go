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

	"cloud.google.com/go/firestore"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
)

// MigrateIncorrectKeyTypes migrates incorrect key types (backend.Key and string) to the correct type (bytes)
// in the backend. This is necessary because the backend was incorrectly storing keys as strings and backend.Key
// types and Firestore clients mapped them to different database types. This forces calling ReadRange 3 times.
// This migration will fix the issue by converting all keys to the correct type (bytes).
// TODO(tigrato|rosstimothy): DELETE In 18.0.0: Remove this migration in the next major release.
func MigrateIncorrectKeyTypes(ctx context.Context, b backend.Backend) error {
	firestore, ok := b.(*Backend)
	if !ok {
		return trace.BadParameter("expected firestore backend")
	}

	// backend.Key is converted to array of ints when sending to the db.
	toArray := func(key []byte) []any {
		arrKey := make([]any, len(key))
		for i, b := range key {
			arrKey[i] = int(b)
		}
		return arrKey
	}

	if err := migrateKeyType[[]any](ctx, firestore, toArray); err != nil {
		return trace.Wrap(err, "failed to migrate backend key")
	}

	stringKey := func(key []byte) string {
		return string(key)
	}
	if err := migrateKeyType[string](ctx, firestore, stringKey); err != nil {
		return trace.Wrap(err, "failed to migrate legacy key")
	}
	return nil
}

func migrateKeyType[T any](ctx context.Context, b *Backend, newKey func([]byte) T) error {
	limit := 500
	startKey := newKey([]byte("/"))

	bulkWriter := b.svc.BulkWriter(b.clientContext)
	defer bulkWriter.End()
	for {
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
			return trace.Wrap(err)
		}

		jobs := make([]*firestore.BulkWriterJob, len(docs))
		for i, dbDoc := range docs {
			newDoc, err := newRecordFromDoc(dbDoc)
			if err != nil {
				return trace.Wrap(err, "failed to convert document")
			}

			jobs[i], err = bulkWriter.Set(
				b.svc.Collection(b.CollectionName).
					Doc(b.keyToDocumentID(newDoc.Key)),
				newDoc,
			)
			if err != nil {
				return trace.Wrap(err, "failed stream bulk action")
			}

			startKey = newKey(newDoc.Key) // update start key
		}

		bulkWriter.Flush() // flush the buffer
		var errs []error
		for _, job := range jobs {
			if _, err := job.Results(); err != nil {
				errs = append(errs, err)
			}
		}
		if err := trace.NewAggregate(errs...); err != nil {
			return trace.Wrap(err, "failed to write bulk actions")
		}

		if len(docs) < limit {
			break
		}
	}
	return nil
}
