// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package gcp

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// AdjustDatabaseUsername appends the GCP IAM suffix (`@<project-id>.iam`) to a
// username if it's for a GCP-hosted Postgres database and the username lacks
// a domain. It returns whether the username was adjusted and the new username.
func AdjustDatabaseUsername(username string, db types.Database) (bool, string) {
	// As a convenience, we will append the `@<project-id>.iam` suffix to a username
	// when connecting to Postgres GCP databases and the domain is missing.
	//
	// Commonly, the service account and the database share the same project ID,
	// which means we can try to guess the intended suffix.
	//
	// This is only applied for Postgres (CloudSQL Postgres or AlloyDB) because:
	// - CloudSQL MySQL still supports "classical" (one-time password) users;
	//   otherwise it would have used the `@<project>.iam.gserviceaccount.com` suffix.
	// - Spanner applies the `@<project>.iam.gserviceaccount.com` suffix directly in the engine.

	if !db.IsGCPHosted() {
		return false, ""
	}

	if db.GetProtocol() != defaults.ProtocolPostgres {
		return false, ""
	}

	// already has domain part, nothing to fix.
	if strings.Contains(username, "@") {
		return false, ""
	}

	// keep empty username unchanged: we don't want to have "@project-123.iam" as username.
	if username == "" {
		return false, ""
	}

	// the error is very unlikely; quietly treat it as missing data.
	projectID, _ := db.GetGCPProjectID()
	if projectID == "" {
		return false, ""
	}

	return true, fmt.Sprintf("%s@%s.iam", strings.TrimSpace(username), projectID)
}
