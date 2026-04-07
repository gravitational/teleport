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
	"crypto/sha1"
	"encoding/base32"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

const (
	// maxDNSLabelLength is the maximum length of a single DNS label (the part
	// between dots in a FQDN)
	maxDNSLabelLength = 63

	// dbNameHashInfix separates the human-readable prefix from the hash suffix
	dbNameHashInfix = "-vnethash-"

	// dbNameHashLen is the number of base32 characters used from the SHA1 hash.
	// 12 base32 chars = 60 bits of entropy, which provides negligible collision
	// probability for database name sets
	dbNameHashLen = 12
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

	// For hashed names, skip ValidateDatabaseName since the hashed label
	// won't match the original database name regex.
	if isHashedDBName(dbName) {
		return dbUser, dbName, nil
	}

	// Validate that dbName looks like a valid database resource name. This
	// avoids unnecessary cluster API calls for invalid names
	if err := types.ValidateDatabaseName(dbName); err != nil {
		return "", "", errNoMatch
	}

	return dbUser, dbName, nil
}

// hashDBName returns a DNS-safe label for a database resource name. Names that
// fit within a single DNS label (<=63 chars) are returned unchanged. Longer
// names are truncated and suffixed with a hash to stay within DNS limits while
// remaining deterministic and human-readable.
//
// Format for hashed names: <prefix>-vnethash-<hash12>
// where <prefix> is the first portion of the original name and <hash12> is
// 12 lowercase base32 characters of the SHA1 hash of the full name.
func hashDBName(name string) string {
	if len(name) <= maxDNSLabelLength {
		return name
	}

	h := sha1.New()
	h.Write([]byte(name))
	hash := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(h.Sum(nil))

	maxPrefix := maxDNSLabelLength - len(dbNameHashInfix) - dbNameHashLen
	prefix := name[:maxPrefix]
	// Ensure prefix doesn't end with a hyphen (invalid DNS label ending).
	prefix = strings.TrimRight(prefix, "-")
	return prefix + dbNameHashInfix + strings.ToLower(hash[:dbNameHashLen])
}

func isHashedDBName(name string) bool {
	return strings.Contains(name, dbNameHashInfix)
}

func extractPrefixFromHashedDBName(name string) string {
	if idx := strings.Index(name, dbNameHashInfix); idx >= 0 {
		return name[:idx]
	}
	return name
}
