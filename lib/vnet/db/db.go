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
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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
// success. The identifier is either the DNS-safe vnet_dns_name or the literal
// database resource name.
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

// MatchExpr returns a ListResources predicate expression that matches
// db_servers whose database resource has either status.vnet_dns_name (the
// DNS-safe hash) or metadata.name equal to identifier. Both forms are
// accepted so users can type whichever is more convenient.
func MatchExpr(identifier string) string {
	return fmt.Sprintf(`resource.status.vnet_dns_name == %q || name == %q`, identifier, identifier)
}

// ListServers queries clt for db_servers matching the given VNet identifier
// (status.vnet_dns_name or metadata.name).
func ListServers(ctx context.Context, clt apiclient.GetResourcesClient, identifier string) ([]types.DatabaseServer, error) {
	rsp, err := apiclient.GetResourcePage[types.DatabaseServer](ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: MatchExpr(identifier),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp.Resources, nil
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

// PickMatch chooses a single database from a list of db_servers returned by a
// VNet identifier query. It dedupes db_servers that advertise the same
// database, warns when multiple distinct databases share the same
// identifier, and gates the chosen database on IsUserOptional.
func PickMatch(ctx context.Context, log *slog.Logger, identifier string, servers []types.DatabaseServer) (types.Database, bool) {
	if len(servers) == 0 {
		log.DebugContext(ctx, "No matching database servers for VNet identifier",
			"identifier", identifier,
		)
		return nil, false
	}
	databases := types.DatabaseServers(servers).ToDatabases()
	if len(databases) > 1 {
		matchedNames := make([]string, 0, len(databases))
		for _, d := range databases {
			matchedNames = append(matchedNames, d.GetName())
		}
		log.WarnContext(ctx, "VNet identifier matched multiple databases; picking the first one",
			"identifier", identifier,
			"matched_db_names", matchedNames,
		)
	}
	chosen := databases[0]
	if !IsUserOptional(chosen.GetProtocol()) {
		log.InfoContext(ctx, "Database protocol not currently supported by VNet",
			"db_name", chosen.GetName(),
			"protocol", chosen.GetProtocol(),
		)
		return nil, false
	}
	return chosen, true
}
