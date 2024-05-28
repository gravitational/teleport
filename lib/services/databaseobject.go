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

package services

import (
	"context"

	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
)

// DatabaseObjects manages DatabaseObject resources.
type DatabaseObjects interface {
	DatabaseObjectsGetter

	// CreateDatabaseObject will create a new DatabaseObject resource.
	CreateDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error)

	// UpsertDatabaseObject creates a new DatabaseObject or forcefully updates an existing DatabaseObject.
	UpsertDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error)

	// DeleteDatabaseObject will delete a DatabaseObject resource.
	DeleteDatabaseObject(ctx context.Context, name string) error

	// UpdateDatabaseObject updates an existing DatabaseObject.
	UpdateDatabaseObject(ctx context.Context, object *dbobjectv1.DatabaseObject) (*dbobjectv1.DatabaseObject, error)
}

// DatabaseObjectsGetter defines methods for fetching database objects.
type DatabaseObjectsGetter interface {
	// GetDatabaseObject will get a DatabaseObject resource by name.
	GetDatabaseObject(ctx context.Context, name string) (*dbobjectv1.DatabaseObject, error)

	// ListDatabaseObjects will list DatabaseObject resources.
	ListDatabaseObjects(ctx context.Context, size int, pageToken string) ([]*dbobjectv1.DatabaseObject, string, error)
}
