/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"context"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

type mockAuthClientForDBCommand struct {
	authclient.ClientI

	dbServers []types.DatabaseServer
	dbs       []types.Database
}

func (m *mockAuthClientForDBCommand) GetResources(_ context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	filter, err := services.MatchResourceFilterFromListResourceRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered, err := services.MatchResourcesByFilters(m.dbServers, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, dbServer := range filtered {
		if _, ok := dbServer.(*types.DatabaseServerV3); !ok {
			return nil, trace.BadParameter("expected types.DatabaseServerV3, got %T", dbServer)
		}
	}
	return &proto.ListResourcesResponse{
		Resources: sliceutils.Map(filtered, func(dbServer types.DatabaseServer) *proto.PaginatedResource {
			return &proto.PaginatedResource{
				Resource: &proto.PaginatedResource_DatabaseServer{
					DatabaseServer: dbServer.(*types.DatabaseServerV3),
				},
			}
		}),
		TotalCount: int32(len(filtered)),
	}, nil
}

func (m *mockAuthClientForDBCommand) ListDatabases(context.Context, int, string) ([]types.Database, string, error) {
	return slices.Clone(m.dbs), "", nil
}

func TestDBCommand_listDatabases(t *testing.T) {
	db1 := mustCreateNewDatabase(t, "db1", "postgres", "localhost:5432", map[string]string{"env": "dev"})
	db2 := mustCreateNewDatabase(t, "db2", "postgres", "localhost:5432", map[string]string{"env": "staging"})
	db3 := mustCreateNewDatabase(t, "db3", "mysql", "localhost:5432", map[string]string{"env": "prod"})
	db4 := mustCreateNewDatabase(t, "db4", "postgres", "localhost:5432", map[string]string{"env": "dev"})

	db1Server1 := mustCreateDatabaseServerFromDB(t, db1, "server1")
	db1Server2 := mustCreateDatabaseServerFromDB(t, db1, "server2")
	db2Server1 := mustCreateDatabaseServerFromDB(t, db2, "server1")

	// These don't exist in backend but used for validating output.
	db3Server, err := toUnregisteredDBServer(db3)
	require.NoError(t, err)
	db4Server, err := toUnregisteredDBServer(db4)
	require.NoError(t, err)

	authClient := &mockAuthClientForDBCommand{
		dbServers: []types.DatabaseServer{db1Server1, db1Server2, db2Server1},
		dbs:       []types.Database{db1, db2, db3, db4},
	}

	tests := []struct {
		name      string
		dbCommand *DBCommand
		wantDBs   []types.DatabaseServer
		wantErr   bool
	}{
		{
			name: "registered",
			dbCommand: &DBCommand{
				listRegistered: true,
			},
			wantDBs: []types.DatabaseServer{db1Server1, db1Server2, db2Server1},
		},
		{
			name: "unregistered",
			dbCommand: &DBCommand{
				listUnregistered: true,
			},
			wantDBs: []types.DatabaseServer{db3Server, db4Server},
		},
		{
			name: "registered and unregistered",
			dbCommand: &DBCommand{
				listRegistered:   true,
				listUnregistered: true,
			},
			wantDBs: []types.DatabaseServer{db1Server1, db1Server2, db2Server1, db3Server, db4Server},
		},
		{
			name: "unregistered with search keywords",
			dbCommand: &DBCommand{
				listUnregistered: true,
				searchKeywords:   "mysql",
			},
			wantDBs: []types.DatabaseServer{db3Server},
		},
		{
			name: "unregistered with predicate",
			dbCommand: &DBCommand{
				listUnregistered: true,
				predicateExpr:    `name == "db4"`,
			},
			wantDBs: []types.DatabaseServer{db4Server},
		},
		{
			name: "registered and unregistered with label",
			dbCommand: &DBCommand{
				listUnregistered: true,
				listRegistered:   true,
				labels:           "env=dev",
			},
			wantDBs: []types.DatabaseServer{db1Server1, db1Server2, db4Server},
		},
		{
			name:      "no registered or unregistered",
			dbCommand: &DBCommand{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actualDBs, err := tt.dbCommand.listDatabases(t.Context(), authClient)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.ElementsMatch(t,
					slices.Collect(types.ResourceNames(tt.wantDBs)),
					slices.Collect(types.ResourceNames(actualDBs)),
				)
			}
		})
	}
}

func mustCreateDatabaseServerFromDB(t *testing.T, db types.Database, host string) types.DatabaseServer {
	t.Helper()
	dbV3, ok := db.(*types.DatabaseV3)
	require.True(t, ok)
	dbServer, err := types.NewDatabaseServerV3(
		types.Metadata{
			Name: db.GetName(),
		},
		types.DatabaseServerSpecV3{
			Version:  teleport.Version,
			Hostname: host,
			HostID:   host,
			Database: dbV3,
		},
	)
	require.NoError(t, err)
	return dbServer
}
