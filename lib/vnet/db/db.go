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

package db

import (
	"strings"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

// infix is the DNS label that separates the database identifier from the
// proxy-address suffix in VNet database FQDNs.
const infix = ".db."

// HasZoneSuffix reports whether fqdn ends with .db.<zone>.
func HasZoneSuffix(fqdn, zone string) bool {
	return strings.HasSuffix(fqdn, infix+dns.FullyQualify(zone))
}

// Parse attempts to parse fqdn as a VNet database FQDN of the form
// <identifier>.db.<zone>. and returns the parsed identifier with ok = true on
// success.
// The identifier is either the DNS-safe vnet_dns_name or the literal database
// resource name
func Parse(fqdn, zone string) (identifier string, ok bool) {
	if !HasZoneSuffix(fqdn, zone) {
		return "", false
	}
	prefix := strings.TrimSuffix(fqdn, infix+dns.FullyQualify(zone))
	if prefix == "" {
		return "", false
	}

	if strings.Contains(prefix, ".") {
		return "", false
	}
	return prefix, true
}

// IsUserOptional reports whether the database protocol's db_service extracts
// the database username from the wire protocol
func IsUserOptional(protocol string) bool {
	switch protocol {
	case defaults.ProtocolPostgres,
		defaults.ProtocolCockroachDB,
		defaults.ProtocolMySQL,
		defaults.ProtocolSQLServer:
		return true
	default:
		return false
	}
}
