/*
Copyright 2022 Gravitational, Inc.

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

package azure

import (
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
)

// DBServer represents an Azure DB Server.
// It exists to reduce code duplication, since Azure MySQL and PostgreSQL.
// server fields are identical in all but type.
// TODO(gavin): Remove this in favor of generics when Go supports structural constraints
// on generic types.
type DBServer struct {
	ID         string
	Location   string
	Name       string
	Port       string
	Properties ServerProperties
	Protocol   string
	Tags       map[string]string
}

// ServerProperties contains properties for an Azure DB Server.
type ServerProperties struct {
	FullyQualifiedDomainName string
	UserVisibleState         string
	Version                  string
}

// ServerFromMySQLServer converts an Azure armmysql.Server into DBServer.
func ServerFromMySQLServer(server *armmysql.Server) *DBServer {
	result := &DBServer{
		ID:       stringVal(server.ID),
		Location: stringVal(server.Location),
		Name:     stringVal(server.Name),
		Port:     MySQLPort,
		Protocol: defaults.ProtocolMySQL,
		Tags:     convertTags(server.Tags),
	}
	if server.Properties != nil {
		result.Properties = ServerProperties{
			FullyQualifiedDomainName: stringVal(server.Properties.FullyQualifiedDomainName),
			UserVisibleState:         stringVal(server.Properties.UserVisibleState),
			Version:                  stringVal(server.Properties.Version),
		}
	}
	return result
}

// ServerFromPostgresServer converts an Azure armpostgresql.Server into DBServer.
func ServerFromPostgresServer(server *armpostgresql.Server) *DBServer {
	result := &DBServer{
		ID:       stringVal(server.ID),
		Location: stringVal(server.Location),
		Name:     stringVal(server.Name),
		Port:     PostgresPort,
		Protocol: defaults.ProtocolPostgres,
		Tags:     convertTags(server.Tags),
	}
	if server.Properties != nil {
		result.Properties = ServerProperties{
			FullyQualifiedDomainName: stringVal(server.Properties.FullyQualifiedDomainName),
			UserVisibleState:         stringVal(server.Properties.UserVisibleState),
			Version:                  stringVal(server.Properties.Version),
		}
	}
	return result
}

// IsVersionSupported returns true if database supports AAD authentication.
// Only available for MySQL 5.7 and newer. All Azure managed PostgreSQL single-server.
// instances support AAD auth.
func (s *DBServer) IsVersionSupported() bool {
	switch s.Protocol {
	case defaults.ProtocolMySQL:
		return isMySQLVersionSupported(s)
	case defaults.ProtocolPostgres:
		return isPostgresVersionSupported(s)
	default:
		panic("unreachable")
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
		return true
	}
}

func stringVal[T ~string](s *T) string {
	if s != nil {
		return string(*s)
	}
	return ""
}

func convertTags(tags map[string]*string) map[string]string {
	result := make(map[string]string, len(tags))
	for k, v := range tags {
		result[k] = stringVal(v)
	}
	x := armmysql.ServerStateReady
	stringVal(&x)
	return result
}
