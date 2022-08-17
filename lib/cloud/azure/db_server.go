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
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
)

// DBServer represents an Azure DB Server.
// It exists to reduce code duplication, since Azure MySQL and PostgreSQL
// server fields are identical in all but type.
type DBServer struct {
	Name       string
	Location   string
	ID         string
	Properties struct {
		FullyQualifiedDomainName string
		UserVisibleState         string
		Version                  string
	}
	Tags           map[string]string
	versionChecker func(*DBServer) bool
}

// ServerFromMySQLServer converts an Azure armmysql.Server into DBServer
func ServerFromMySQLServer(server *armmysql.Server) (*DBServer, error) {
	return newDBServerFromARMServer(server, isMySQLVersionSupported)
}

// ServerFromPostgresServer converts an Azure armpostgresql.Server into DBServer
func ServerFromPostgresServer(server *armpostgresql.Server) (*DBServer, error) {
	return newDBServerFromARMServer(server, isPostgresVersionSupported)
}

// IsVersionSupported returns true if database supports AAD authentication.
// Only available for MySQL 5.7 and newer. All Azure managed PostgreSQL single-server
// instances support AAD auth.
func (s *DBServer) IsVersionSupported() bool {
	return s.versionChecker(s)
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

// newDBServerFromARMServer converts an ARM MySQL or PostgreSQL server into a DBServer.
func newDBServerFromARMServer(armServer interface{}, versionChecker func(*DBServer) bool) (*DBServer, error) {
	if armServer == nil {
		return nil, trace.BadParameter("nil server")
	}
	data, err := json.Marshal(armServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := DBServer{versionChecker: versionChecker}
	err = json.Unmarshal(data, &result)
	return &result, trace.Wrap(err)
}
