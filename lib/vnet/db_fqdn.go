// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package vnet

import (
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// dbFQDNInfix is the DNS label that separates the database-specific prefix
// (user and resource name) from the proxy address suffix in VNet database
// FQDNs.
const dbFQDNInfix = ".db."

// parseDatabaseFQDN attempts to parse fqdn as a database FQDN of the form
// [<db-user>.]<db-resource-name>.db.<zone>. where zone is a fully-qualified
// proxy address.
//
// It returns the database user (may be empty if omitted), the database resource
// name, or errNoMatch if fqdn does not match the expected pattern for the given
// zone.
//
// Since database resource names cannot contain dots (enforced by
// ValidateDatabaseName), parsing splits the prefix on the last dot to separate
// the optional database user from the resource name.
func parseDatabaseFQDN(fqdn string, zone string) (dbUser, dbName string, err error) {
	suffix := dbFQDNInfix + fullyQualify(zone)
	if !strings.HasSuffix(fqdn, suffix) {
		return "", "", errNoMatch
	}
	prefix := strings.TrimSuffix(fqdn, suffix)
	if prefix == "" {
		return "", "", errNoMatch
	}

	// Database resource names cannot contain dots. If there's a dot in the
	// prefix, everything after the last dot is the db name and everything
	// before is the db user (which can contain dots).
	if lastDot := strings.LastIndex(prefix, "."); lastDot >= 0 {
		dbUser = prefix[:lastDot]
		dbName = prefix[lastDot+1:]
	} else {
		// for auto-user provisioning, the db name is the prefix as there is no user
		dbName = prefix
	}

	if dbName == "" {
		return "", "", errNoMatch
	}

	// This avoids unnecessary cluster API calls for obviously invalid database names
	if err := types.ValidateDatabaseName(dbName); err != nil {
		return "", "", errNoMatch
	}

	return dbUser, dbName, nil
}
