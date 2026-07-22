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

package azure

import (
	"context"
	"log/slog"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"

	"github.com/gravitational/teleport/lib/defaults"
)

// DBServer represents an Azure DB Server.
// It exists to reduce code duplication, since Azure MySQL and PostgreSQL
// server fields are identical in all but type.
// TODO(gavin): Remove this in favor of generics when Go supports structural constraints
// on generic types.
type DBServer struct {
	// ID is the fully qualified resource ID for this resource.
	ID string
	// Location is the geo-location where the resource lives.
	Location string
	// Name is the name of the resource.
	Name string
	// Port is the port used to connect to this resource.
	Port string
	// Properties contains properties for an DB Server.
	Properties ServerProperties
	// Protocol is the DB protocol used for this DB Server.
	Protocol string
	// Tags are the resource tags associated with this resource.
	Tags map[string]string
}

// ServerProperties contains properties for an DB Server.
type ServerProperties struct {
	// FullyQualifiedDomainName is the fully qualified domain name which resolves to the DB Server address.
	FullyQualifiedDomainName string
	// UserVisibleState is the state of the DB Server that is visible to a user.
	UserVisibleState string
	// Version is the version of the Azure gateway which redirects traffic to the database servers.
	Version string
}

// ServerFromMySQLServer converts an Azure armmysql.Server into DBServer.
func ServerFromMySQLServer(server *armmysql.Server) *DBServer {
	result := &DBServer{
		ID:       StringVal(server.ID),
		Location: StringVal(server.Location),
		Name:     StringVal(server.Name),
		Port:     MySQLPort,
		Protocol: defaults.ProtocolMySQL,
		Tags:     ConvertTags(server.Tags),
	}
	if server.Properties != nil {
		result.Properties = ServerProperties{
			FullyQualifiedDomainName: StringVal(server.Properties.FullyQualifiedDomainName),
			UserVisibleState:         StringVal(server.Properties.UserVisibleState),
			Version:                  StringVal(server.Properties.Version),
		}
	}
	return result
}

// ServerFromPostgresServer converts an Azure armpostgresql.Server into DBServer.
func ServerFromPostgresServer(server *armpostgresql.Server) *DBServer {
	result := &DBServer{
		ID:       StringVal(server.ID),
		Location: StringVal(server.Location),
		Name:     StringVal(server.Name),
		Port:     PostgresPort,
		Protocol: defaults.ProtocolPostgres,
		Tags:     ConvertTags(server.Tags),
	}
	if server.Properties != nil {
		result.Properties = ServerProperties{
			FullyQualifiedDomainName: StringVal(server.Properties.FullyQualifiedDomainName),
			UserVisibleState:         StringVal(server.Properties.UserVisibleState),
			Version:                  StringVal(server.Properties.Version),
		}
	}
	return result
}

// IsSupported returns true if database supports AAD authentication.
// Only available for MySQL 5.7 and newer. All Azure managed PostgreSQL single-server
// instances support AAD auth.
func (s *DBServer) IsSupported() bool {
	switch s.Protocol {
	case defaults.ProtocolMySQL:
		return isMySQLVersionSupported(s)
	case defaults.ProtocolPostgres:
		return isPostgresVersionSupported(s)
	default:
		return false
	}
}

// IsAvailable returns whether the Azure DBServer is available.
func (s *DBServer) IsAvailable() bool {
	switch s.Properties.UserVisibleState {
	case "Ready":
		return true
	case "Inaccessible", "Dropping", "Disabled":
		return false
	default:
		slog.WarnContext(context.Background(), "Assuming Azure DB server with unknown server state is available",
			"state", s.Properties.UserVisibleState,
			"server", s.Name,
		)
		return true
	}
}

// StringVal converts a pointer of a string or a string alias to a string value.
func StringVal[T ~string](s *T) string {
	if s != nil {
		return string(*s)
	}
	return ""
}

// ConvertTags converts map of string pointers to map of strings.
func ConvertTags(tags map[string]*string) map[string]string {
	result := make(map[string]string, len(tags))
	for k, v := range tags {
		result[k] = StringVal(v)
	}
	return result
}
