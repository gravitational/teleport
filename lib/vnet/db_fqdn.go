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

// hasDBZoneSuffix checks if fqdn ends with .db.<zone>. Used for lightweight
func hasDBZoneSuffix(fqdn string, zone string) bool {
	return strings.HasSuffix(fqdn, dbFQDNInfix+fullyQualify(zone))
}

// splitDBUserAndName splits a prefix (the part before .db.<zone>) into a
// database user and database resource name.
func splitDBUserAndName(prefix string) (dbUser, dbName string) {
	if lastDot := strings.LastIndex(prefix, "."); lastDot >= 0 {
		return prefix[:lastDot], prefix[lastDot+1:]
	}
	// No dot — the entire prefix is the database name with no user
	// (auto-user provisioning or single allowed user).
	return "", prefix
}

// parseDatabaseFQDN attempts to parse fqdn as a database FQDN of the form
// [<db-user>.]<db-resource-name>.db.<zone>. where zone is a fully-qualified
// proxy address.
func parseDatabaseFQDN(fqdn string, zone string) (dbUser, dbName string, err error) {
	if !hasDBZoneSuffix(fqdn, zone) {
		return "", "", errNoMatch
	}
	prefix := strings.TrimSuffix(fqdn, dbFQDNInfix+fullyQualify(zone))
	if prefix == "" {
		return "", "", errNoMatch
	}

	dbUser, dbName = splitDBUserAndName(prefix)
	if dbName == "" {
		return "", "", errNoMatch
	}

	// Validate that dbName looks like a valid database resource name. This
	// avoids unnecessary cluster API calls for invalid names
	if err := types.ValidateDatabaseName(dbName); err != nil {
		return "", "", errNoMatch
	}

	return dbUser, dbName, nil
}
