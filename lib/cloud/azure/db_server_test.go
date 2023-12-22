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
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestListServers(t *testing.T) {
	t.Parallel()
	myServer1, myDBServer1 := makeMySQLServer("mysql1", "group1")
	myServer2, myDBServer2 := makeMySQLServer("mysql2", "group2")
	pgServer1, pgDBServer1 := makePostgresServer("pgres1", "group1")
	pgServer2, pgDBServer2 := makePostgresServer("pgres2", "group2")
	mySQLClient := NewMySQLServersClient(&ARMMySQLMock{
		DBServers: []*armmysql.Server{myServer1, myServer2},
	})
	pgClient := NewPostgresServerClient(&ARMPostgresMock{
		DBServers: []*armpostgresql.Server{pgServer1, pgServer2},
	})
	tests := []struct {
		name   string
		client DBServersClient
		group  string
		want   []*DBServer
	}{
		{
			name:   "list all MySQL servers",
			client: mySQLClient,
			group:  types.Wildcard,
			want:   []*DBServer{myDBServer1, myDBServer2},
		},
		{
			name:   "list MySQL servers in a resource group",
			client: mySQLClient,
			group:  "group1",
			want:   []*DBServer{myDBServer1},
		},
		{
			name:   "list all PostgreSQL servers",
			client: pgClient,
			group:  types.Wildcard,
			want:   []*DBServer{pgDBServer1, pgDBServer2},
		},
		{
			name:   "list PostgreSQL servers in a resource group",
			client: pgClient,
			group:  "group2",
			want:   []*DBServer{pgDBServer2},
		},
	}
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				servers []*DBServer
				err     error
			)
			if tt.group == types.Wildcard {
				servers, err = tt.client.ListAll(ctx)
			} else {
				servers, err = tt.client.ListWithinGroup(ctx, tt.group)
			}
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.want, servers))
		})
	}
}

func TestGetServer(t *testing.T) {
	t.Parallel()
	myServer1, _ := makeMySQLServer("a", "group1")
	myServer2, myDBServer2 := makeMySQLServer("a", "group2")
	myServer3, _ := makeMySQLServer("b", "group1")
	myServer4, _ := makeMySQLServer("b", "group2")
	pgServer1, pgDBServer1 := makePostgresServer("a", "group1")
	pgServer2, _ := makePostgresServer("a", "group2")
	pgServer3, _ := makePostgresServer("b", "group1")
	pgServer4, _ := makePostgresServer("b", "group2")
	mySQLClient := NewMySQLServersClient(&ARMMySQLMock{
		DBServers: []*armmysql.Server{myServer1, myServer2, myServer3, myServer4},
	})
	pgClient := NewPostgresServerClient(&ARMPostgresMock{
		DBServers: []*armpostgresql.Server{pgServer1, pgServer2, pgServer3, pgServer4},
	})

	tests := []struct {
		name   string
		client DBServersClient
		dbName string
		group  string
		want   *DBServer
	}{
		{
			name:   "get a mysql server",
			client: mySQLClient,
			dbName: "a",
			group:  "group2",
			want:   myDBServer2,
		},
		{
			name:   "get a postgres server",
			client: pgClient,
			dbName: "a",
			group:  "group1",
			want:   pgDBServer1,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := tt.client.Get(ctx, tt.group, tt.dbName)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.want, s))
		})
	}
}

func TestServerConversion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		protocol      string
		version       string
		state         string
		wantAvailable bool
		wantSupported bool
	}{
		{
			name:          "mysql available and supported",
			protocol:      defaults.ProtocolMySQL,
			version:       "5.7",
			state:         "Ready",
			wantAvailable: true,
			wantSupported: true,
		},
		{
			name:          "mysql unavailable and unsupported",
			protocol:      defaults.ProtocolMySQL,
			version:       "5.6",
			state:         "Disabled",
			wantAvailable: false,
			wantSupported: false,
		},
		{
			name:          "postgres available and supported",
			protocol:      defaults.ProtocolPostgres,
			version:       "11",
			state:         "Ready",
			wantAvailable: true,
			wantSupported: true,
		},
		{
			name:          "postgres unavailable",
			protocol:      defaults.ProtocolPostgres,
			version:       "11",
			state:         "Disabled",
			wantAvailable: false,
			wantSupported: true,
		},
	}

	const (
		region = "eastus"
		dbName = "dbName"
		group  = "resourceGroup"
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				dbServer *DBServer
				fqdn     string
				id       string
				port     string
			)
			tags := map[string]string{"foo": "bar", "baz": "qux"}
			switch tt.protocol {
			case defaults.ProtocolMySQL:
				armServer, _ := makeMySQLServer(dbName, group, withMySQLState(tt.state), withMySQLVersion(tt.version))
				dbServer = ServerFromMySQLServer(armServer)
				fqdn = *armServer.Properties.FullyQualifiedDomainName
				id = *armServer.ID
				port = "3306"
			case defaults.ProtocolPostgres:
				armServer, _ := makePostgresServer(dbName, group, withPostgresState(tt.state), withPostgresVersion(tt.version))
				dbServer = ServerFromPostgresServer(armServer)
				fqdn = *armServer.Properties.FullyQualifiedDomainName
				id = *armServer.ID
				port = "5432"
			default:
				require.FailNow(t, "unknown db protocol specified by test")
			}

			expected := DBServer{
				ID:       id,
				Location: region,
				Name:     dbName,
				Port:     port,
				Properties: ServerProperties{
					FullyQualifiedDomainName: fqdn,
					UserVisibleState:         tt.state,
					Version:                  tt.version,
				},
				Protocol: tt.protocol,
				Tags:     tags,
			}

			_, err := arm.ParseResourceID(dbServer.ID)
			require.NoError(t, err)

			require.Equal(t, expected, *dbServer)
			require.Equal(t, tt.wantAvailable, dbServer.IsAvailable())
			require.Equal(t, tt.wantSupported, dbServer.IsSupported())
		})
	}
}

type MySQLServerOptFn func(*armmysql.Server)
type PostgresServerOptFn func(*armpostgresql.Server)

func withMySQLState(state string) MySQLServerOptFn {
	return func(s *armmysql.Server) {
		*s.Properties.UserVisibleState = armmysql.ServerState(state)
	}
}

func withMySQLVersion(version string) MySQLServerOptFn {
	return func(s *armmysql.Server) {
		*s.Properties.Version = armmysql.ServerVersion(version)
	}
}

func withPostgresState(state string) PostgresServerOptFn {
	return func(s *armpostgresql.Server) {
		*s.Properties.UserVisibleState = armpostgresql.ServerState(state)
	}
}

func withPostgresVersion(version string) PostgresServerOptFn {
	return func(s *armpostgresql.Server) {
		*s.Properties.Version = armpostgresql.ServerVersion(version)
	}
}

func makeMySQLServer(name, group string, opts ...MySQLServerOptFn) (*armmysql.Server, *DBServer) {
	id := fmt.Sprintf("/subscriptions/subid/resourceGroups/%v/providers/Microsoft.DBforMySQL/servers/%v",
		group, name)
	fqdn := fmt.Sprintf("%v.mysql.database.azure.com", name)
	server := &armmysql.Server{
		Location: to.Ptr("eastus"),
		Properties: &armmysql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         to.Ptr(armmysql.ServerStateReady),
			Version:                  to.Ptr(armmysql.ServerVersionFive7),
		},
		Tags: map[string]*string{"foo": to.Ptr("bar"), "baz": to.Ptr("qux")},
		ID:   to.Ptr(id),
		Name: &name,
		Type: to.Ptr("Microsoft.DBforMySQL/servers"),
	}
	for _, optFn := range opts {
		optFn(server)
	}
	dbServer := ServerFromMySQLServer(server)
	return server, dbServer
}

func makePostgresServer(name, group string, opts ...PostgresServerOptFn) (*armpostgresql.Server, *DBServer) {
	id := fmt.Sprintf("/subscriptions/subid/resourceGroups/%v/providers/Microsoft.DBforPostgreSQL/servers/%v",
		group, name)
	fqdn := fmt.Sprintf("%v.postgres.database.azure.com", name)
	server := &armpostgresql.Server{
		Location: to.Ptr("eastus"),
		Properties: &armpostgresql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         to.Ptr(armpostgresql.ServerStateReady),
			Version:                  to.Ptr(armpostgresql.ServerVersionEleven),
		},
		Tags: map[string]*string{"foo": to.Ptr("bar"), "baz": to.Ptr("qux")},
		ID:   to.Ptr(id),
		Name: &name,
		Type: to.Ptr("Microsoft.DBforPostgreSQL/servers"),
	}
	for _, optFn := range opts {
		optFn(server)
	}
	dbServer := ServerFromPostgresServer(server)
	return server, dbServer
}
