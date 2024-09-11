/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package migration

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/firestore"
)

// migrateFirestoreKeys performs a migration which transforms all incorrect
// key types (backend.Key and string) in the firestore backend to the correct type.
// This happens because the backend was incorrectly storing keys as strings and backend.Key
// types and Firestore clients mapped them to different database types. This forces calling ReadRange 3 times
// This migration will fix the issue by converting all keys to the correct type (bytes).
type migrateFirestoreKeys struct {
}

func (d migrateFirestoreKeys) Version() int64 {
	return 2
}

func (d migrateFirestoreKeys) Name() string {
	return "migrate_firestore_keys"
}

// Up scans the backend for keys that are stored as strings or backend.Key types
// and converts them to the correct type (bytes).
func (d migrateFirestoreKeys) Up(ctx context.Context, b backend.Backend) error {
	ctx, span := tracer.Start(ctx, "migrateFirestoreKeys/Up")
	defer span.End()

	// if the backend is not firestore, skip this migration
	if b.GetName() != firestore.GetName() {
		return nil
	}

	// migrate firestore keys
	return trace.Wrap(firestore.MigrateIncorrectKeyTypes(ctx, b))
}

// Down is a no-op for this migration.
func (d migrateFirestoreKeys) Down(ctx context.Context, _ backend.Backend) error {
	_, span := tracer.Start(ctx, "migrateFirestoreKeys/Down")
	defer span.End()
	return nil
}
