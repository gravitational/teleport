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

package role

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// RoleMatchersConfig contains parameters for database role matchers.
type RoleMatchersConfig struct {
	// Database is the database that's being connected to.
	Database types.Database
	// DatabaseUser is the database username.
	DatabaseUser string
	// DatabaseName is the database name.
	DatabaseName string
	// AutoCreateUser is whether database user will be auto-created.
	AutoCreateUser bool
	// DisableDatabaseNameMatcher skips DatabaseNameMatcher even if the protocol requires it.
	DisableDatabaseNameMatcher bool
}

// GetDatabaseRoleMatchers returns database role matchers for the provided config.
func GetDatabaseRoleMatchers(conf RoleMatchersConfig) (matchers services.RoleMatchers) {
	// For automatic user provisioning, don't check against database users as
	// users will be connecting as their own Teleport username.
	disableDatabaseUserMatcher := conf.Database.SupportsAutoUsers() && conf.AutoCreateUser
	if !disableDatabaseUserMatcher {
		matchers = append(matchers, services.NewDatabaseUserMatcher(conf.Database, conf.DatabaseUser))
	}

	if !conf.DisableDatabaseNameMatcher {
		if matcher := databaseNameMatcher(conf.Database.GetProtocol(), conf.DatabaseName); matcher != nil {
			matchers = append(matchers, matcher)
		}
	}
	return
}

// RequireDatabaseUserMatcher returns true if databases with provided protocol
// require database users.
func RequireDatabaseUserMatcher(protocol string) bool {
	return true // Always required.
}

// RequireDatabaseNameMatcher returns true if databases with provided protocol
// require database names.
func RequireDatabaseNameMatcher(protocol string) bool {
	return databaseNameMatcher(protocol, "") != nil
}

func databaseNameMatcher(dbProtocol, database string) *services.DatabaseNameMatcher {
	switch dbProtocol {
	case
		// In MySQL, unlike Postgres, "database" and "schema" are the same thing
		// and there's no good way to prevent users from performing cross-database
		// queries once they're connected, apart from granting proper privileges
		// in MySQL itself.
		//
		// As such, checking db_names for MySQL is quite pointless, so we only
		// check db_users. In the future, if we implement some sort of access controls
		// on queries, we might be able to restrict db_names as well e.g. by
		// detecting full-qualified table names like db.table, until then the
		// proper way is to use MySQL grants system.
		defaults.ProtocolMySQL,
		// Cockroach uses the same wire protocol as Postgres but handling of
		// databases is different and there's no way to prevent cross-database
		// queries so only apply RBAC to db_users.
		defaults.ProtocolCockroachDB,
		// Most database protocols do not support schema access control.
		defaults.ProtocolRedis,
		defaults.ProtocolCassandra,
		defaults.ProtocolElasticsearch,
		defaults.ProtocolOpenSearch,
		defaults.ProtocolDynamoDB,
		defaults.ProtocolSnowflake,
		defaults.ProtocolOracle,
		defaults.ProtocolClickHouse,
		defaults.ProtocolClickHouseHTTP,
		defaults.ProtocolSQLServer:
		return nil
	default:
		// Protocols that do support database name matcher:
		// - postgres
		// - mongodb
		// - cloud spanner
		return &services.DatabaseNameMatcher{Name: database}
	}
}
