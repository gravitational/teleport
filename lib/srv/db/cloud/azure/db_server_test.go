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
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysql"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresql"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/teleport/api/types"

	"github.com/stretchr/testify/require"
)

func TestListServers(t *testing.T) {
	myServer1, myDBServer1 := makeMySQLServer(t, "mysql1", "group1", "Ready", "5.7")
	myServer2, myDBServer2 := makeMySQLServer(t, "mysql2", "group2", "Ready", "5.7")
	pgServer1, pgDBServer1 := makePostgresServer(t, "pgres1", "group1", "Ready", "11")
	pgServer2, pgDBServer2 := makePostgresServer(t, "pgres2", "group2", "Ready", "11")
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
	maxPages := 10
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servers, err := tt.client.ListServers(ctx, tt.group, maxPages)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(tt.want, servers, cmpopts.IgnoreFields(DBServer{}, "versionChecker")))
		})
	}
}

func TestGetServer(t *testing.T) {
	myServer1, _ := makeMySQLServer(t, "a", "group1", "5.7", "Ready")
	myServer2, myDBServer2 := makeMySQLServer(t, "a", "group2", "5.7", "Ready")
	myServer3, _ := makeMySQLServer(t, "b", "group1", "5.7", "Ready")
	myServer4, _ := makeMySQLServer(t, "b", "group2", "5.7", "Ready")
	pgServer1, pgDBServer1 := makePostgresServer(t, "a", "group1", "5.7", "Ready")
	pgServer2, _ := makePostgresServer(t, "a", "group2", "5.7", "Ready")
	pgServer3, _ := makePostgresServer(t, "b", "group1", "5.7", "Ready")
	pgServer4, _ := makePostgresServer(t, "b", "group2", "5.7", "Ready")
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
			require.Empty(t, cmp.Diff(tt.want, s, cmpopts.IgnoreFields(DBServer{}, "versionChecker")))
		})
	}
}

func TestServerConversion(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		version       string
		state         string
		wantAvailable bool
		wantSupported bool
	}{
		{
			name:          "mysql available and supported",
			provider:      MySQLNamespace,
			version:       "5.7",
			state:         "Ready",
			wantAvailable: true,
			wantSupported: true,
		},
		{
			name:          "mysql unavailable and unsupported",
			provider:      MySQLNamespace,
			version:       "5.6",
			state:         "Disabled",
			wantAvailable: false,
			wantSupported: false,
		},
		{
			name:          "postgres available and supported",
			provider:      PostgreSQLNamespace,
			version:       "11",
			state:         "Ready",
			wantAvailable: true,
			wantSupported: true,
		},
		{
			name:          "postgres unavailable",
			provider:      PostgreSQLNamespace,
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
				server *DBServer
				fqdn   string
			)
			tags := map[string]string{"foo": "bar", "baz": "qux"}
			switch tt.provider {
			case MySQLNamespace:
				fqdn = fmt.Sprintf("%v.mysql.database.azure.com", dbName)
				_, server = makeMySQLServer(t, dbName, group, tt.state, tt.version)
			case PostgreSQLNamespace:
				fqdn = fmt.Sprintf("%v.postgres.database.azure.com", dbName)
				_, server = makePostgresServer(t, dbName, group, tt.state, tt.version)
			default:
				require.FailNow(t, "unknown db namespace specified by test")
			}

			rid, err := arm.ParseResourceID(server.ID)
			require.NoError(t, err)

			require.Equal(t, dbName, server.Name)
			require.Equal(t, dbName, rid.Name)
			require.Equal(t, region, server.Location)
			require.Equal(t, tags, server.Tags)
			require.Equal(t, fqdn, server.Properties.FullyQualifiedDomainName)
			require.Equal(t, tt.state, server.Properties.UserVisibleState)
			require.Equal(t, tt.version, server.Properties.Version)
			require.Equal(t, group, rid.ResourceGroupName)
			require.Equal(t, "subid", rid.SubscriptionID)
			require.Equal(t, tt.provider, rid.ResourceType.Namespace)
			require.Equal(t, "servers", rid.ResourceType.Type)
			require.Equal(t, tt.wantAvailable, server.IsAvailable())
			require.Equal(t, tt.wantSupported, server.IsVersionSupported())
		})
	}
}

func makeMySQLServer(t *testing.T, name, group, state, version string) (*armmysql.Server, *DBServer) {
	id := fmt.Sprintf("/subscriptions/subid/resourceGroups/%v/providers/Microsoft.DBforMySQL/servers/%v",
		group, name)
	fqdn := fmt.Sprintf("%v.mysql.database.azure.com", name)
	server := &armmysql.Server{
		Location: to.Ptr("eastus"),
		Properties: &armmysql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         (*armmysql.ServerState)(&state),
			Version:                  (*armmysql.ServerVersion)(&version),
		},
		Tags: map[string]*string{"foo": to.Ptr("bar"), "baz": to.Ptr("qux")},
		ID:   to.Ptr(id),
		Name: &name,
		Type: to.Ptr("Microsoft.DBforMySQL/servers"),
	}
	dbServer, err := ServerFromMySQLServer(server)
	require.NoError(t, err)
	return server, dbServer
}

func makePostgresServer(t *testing.T, name, group, state, version string) (*armpostgresql.Server, *DBServer) {
	id := fmt.Sprintf("/subscriptions/subid/resourceGroups/%v/providers/Microsoft.DBforPostgreSQL/servers/%v",
		group, name)
	fqdn := fmt.Sprintf("%v.postgres.database.azure.com", name)
	server := &armpostgresql.Server{
		Location: to.Ptr("eastus"),
		Properties: &armpostgresql.ServerProperties{
			FullyQualifiedDomainName: &fqdn,
			UserVisibleState:         (*armpostgresql.ServerState)(&state),
			Version:                  (*armpostgresql.ServerVersion)(&version),
		},
		Tags: map[string]*string{"foo": to.Ptr("bar"), "baz": to.Ptr("qux")},
		ID:   to.Ptr(id),
		Name: &name,
		Type: to.Ptr("Microsoft.DBforPostgreSQL/servers"),
	}
	dbServer, err := ServerFromPostgresServer(server)
	require.NoError(t, err)
	return server, dbServer
}
