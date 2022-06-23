/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package role

import (
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// DatabaseRoleMatchers returns role matchers based on the database protocol.
func DatabaseRoleMatchers(dbProtocol string, user, database string) services.RoleMatchers {
	switch dbProtocol {
	case defaults.ProtocolMySQL:
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
		return services.RoleMatchers{
			&services.DatabaseUserMatcher{User: user},
		}
	case defaults.ProtocolCockroachDB:
		// Cockroach uses the same wire protocol as Postgres but handling of
		// databases is different and there's no way to prevent cross-database
		// queries so only apply RBAC to db_users.
		return services.RoleMatchers{
			&services.DatabaseUserMatcher{User: user},
		}
	case defaults.ProtocolRedis:
		// Redis integration doesn't support schema access control.
		return services.RoleMatchers{
			&services.DatabaseUserMatcher{User: user},
		}
	default:
		return services.RoleMatchers{
			&services.DatabaseUserMatcher{User: user},
			&services.DatabaseNameMatcher{Name: database},
		}
	}
}
