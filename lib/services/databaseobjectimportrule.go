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

package services

import (
	"context"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
)

// DatabaseObjectImportRules manages DatabaseObjectImportRule resources.
type DatabaseObjectImportRules interface {
	// CreateDatabaseObjectImportRule will create a new DatabaseObjectImportRule resource.
	CreateDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// UpsertDatabaseObjectImportRule creates a new DatabaseObjectImportRule or forcefully updates an existing DatabaseObjectImportRule.
	UpsertDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// GetDatabaseObjectImportRule will get a DatabaseObjectImportRule resource by name.
	GetDatabaseObjectImportRule(ctx context.Context, name string) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// DeleteDatabaseObjectImportRule will delete a DatabaseObjectImportRule resource.
	DeleteDatabaseObjectImportRule(ctx context.Context, name string) error

	// UpdateDatabaseObjectImportRule updates an existing DatabaseObjectImportRule.
	UpdateDatabaseObjectImportRule(ctx context.Context, rule *dbobjectimportrulev1.DatabaseObjectImportRule) (*dbobjectimportrulev1.DatabaseObjectImportRule, error)

	// ListDatabaseObjectImportRules will list DatabaseObjectImportRule resources.
	ListDatabaseObjectImportRules(ctx context.Context, size int, pageToken string) ([]*dbobjectimportrulev1.DatabaseObjectImportRule, string, error)
}
