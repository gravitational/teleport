/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"log/slog"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/discovery/common/cloudsql"
)

// setGCPDBName overrides the first name part from the GCP name-override label (if
// present) and sets the database name on the metadata.
func setGCPDBName(meta types.Metadata, firstNamePart string, extraNameParts ...string) types.Metadata {
	return setResourceName([]string{types.GCPDatabaseNameOverrideLabel}, meta, firstNamePart, extraNameParts...)
}

// DiscoverCloudSQLDatabases converts eligible Cloud SQL instances to
// types.Database. Ineligible instances are logged but discarded.
func DiscoverCloudSQLDatabases(ctx context.Context, logger *slog.Logger, instances []*sqladmin.DatabaseInstance) []types.Database {
	var databases []types.Database
	for _, instance := range instances {
		db, skipReason, err := cloudsql.NewDatabaseFromInstance(instance, func(meta types.Metadata) types.Metadata {
			return setGCPDBName(meta, instance.Name)
		})
		if err != nil {
			// a single bad instance must not abort discovery of the rest.
			logger.WarnContext(ctx, "Failed to build database object", "name", instance.Name, "project", instance.Project, "error", err)
			continue
		}
		if db == nil || skipReason != "" {
			logger.DebugContext(ctx, "Skipping Cloud SQL instance", "name", instance.Name, "project", instance.Project, "reason", skipReason)
			continue
		}
		databases = append(databases, db)
	}
	return databases
}
