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

package dbfqdn

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

const (
	// infix is the DNS label that separates the vnet_dns_name from the proxy
	// address suffix in VNet database FQDNs.
	infix = ".db."
	// vnetDNSNameLen is the length of the DNS-safe identifier
	vnetDNSNameLen = 13
)

// ErrNoMatch is returned by Parse when fqdn is not a VNet database FQDN for
// the given zone.
var ErrNoMatch = errors.New("fqdn does not match a VNet database for the zone")

// ErrNotDBFQDN is returned by Parse when fqdn does not have the .db.<zone>
// suffix expected for a VNet database FQDN.
var ErrNotDBFQDN = fmt.Errorf("not a VNet database FQDN: %w", ErrNoMatch)

// ErrInvalidVNetDNSName is returned by Parse when fqdn has the .db.<zone>
// suffix but the prefix is not a well-formed vnet_dns_name
var ErrInvalidVNetDNSName = fmt.Errorf("invalid vnet_dns_name: %w", ErrNoMatch)

// HasZoneSuffix reports whether fqdn ends with .db.<zone>.
func HasZoneSuffix(fqdn, zone string) bool {
	return strings.HasSuffix(fqdn, infix+dns.FullyQualify(zone))
}

// Parse attempts to parse fqdn as a VNet database FQDN of the form
// <vnet-dns-name>.db.<zone>. and returns the parsed vnet_dns_name. The caller
// resolves the underlying database resource by querying for a db_server whose
// status.vnet_dns_name matches.
func Parse(fqdn, zone string) (vnetDNSName string, err error) {
	if !HasZoneSuffix(fqdn, zone) {
		return "", ErrNotDBFQDN
	}
	prefix := strings.TrimSuffix(fqdn, infix+dns.FullyQualify(zone))
	if !isValidVNetDNSName(prefix) {
		return "", ErrInvalidVNetDNSName
	}
	return prefix, nil
}

// IsSupportedProtocol reports whether VNet currently supports a database
// protocol. Only protocols whose db_service extracts the username from the
// wire protocol are supported.
func IsSupportedProtocol(protocol string) bool {
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

// isValidVNetDNSName reports whether vnetDNSName has the shape of a vnet_dns_name
func isValidVNetDNSName(vnetDNSName string) bool {
	if len(vnetDNSName) != vnetDNSNameLen {
		return false
	}
	for _, c := range vnetDNSName {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'v':
		default:
			return false
		}
	}
	return true
}
