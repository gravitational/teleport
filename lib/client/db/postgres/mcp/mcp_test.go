// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/listener"
)

func TestFormatResult(t *testing.T) {
	for name, tc := range map[string]struct {
		rows           pgx.Rows
		expectedResult string
	}{
		"query results": {
			rows:           newMockRows("SELECT 2", []string{"name", "age"}, [][]any{{"Alice", 30}, {"Bob", 31}}),
			expectedResult: `{"data":[{"age":30,"name":"Alice"},{"age":31,"name":"Bob"}],"rowsCount":2}`,
		},
		"empty query results": {
			rows:           newMockRows("SELECT 0", []string{}, [][]any{}),
			expectedResult: `{"data":[],"rowsCount":0}`,
		},
		"non-data results": {
			rows:           newMockRows("INSERT 1", []string{}, [][]any{}),
			expectedResult: `{"data":null,"rowsCount":1}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := buildQueryResult(tc.rows)
			require.NoError(t, err)
			require.Equal(t, tc.expectedResult, res)
		})
	}
}

func TestFormatErrors(t *testing.T) {
	// Dummy listener that always drop connections.
	listener := listener.NewInMemoryListener()
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	dbName := "local"
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:   dbName,
		Labels: map[string]string{"env": "test"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	dbURI := clientmcp.NewDatabaseResourceURI("root", dbName).String()

	for name, tc := range map[string]struct {
		databaseURI            string
		databases              []*dbmcp.Database
		externalErrorRetriever dbmcp.ExternalErrorRetriever
		expectErrorMessage     require.ValueAssertionFunc
	}{
		"database not found": {
			databaseURI: "teleport://clusters/root/databases/not-found",
			expectErrorMessage: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.Equal(t, dbmcp.DatabaseNotFoundError.Error(), i1)
			},
		},
		"malformed database uri": {
			databaseURI: "not-found",
			expectErrorMessage: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.Equal(t, dbmcp.WrongDatabaseURIFormatError.Error(), i1)
			},
		},
		"local proxy rejects connection": {
			databaseURI: dbURI,
			databases: []*dbmcp.Database{
				&dbmcp.Database{
					DB:           db,
					ClusterName:  "root",
					DatabaseUser: "postgres",
					DatabaseName: "postgres",
					Addr:         listener.Addr().String(),
					LookupFunc: func(_ context.Context, _ string) (addrs []string, err error) {
						return []string{"memory"}, nil
					},
					DialContextFunc: listener.DialContext,
				},
			},
			expectErrorMessage: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.Equal(t, dbmcp.LocalProxyConnectionErrorMessage, i1)
			},
		},
		"relogin error": {
			databaseURI: dbURI,
			databases: []*dbmcp.Database{
				&dbmcp.Database{
					DB:                     db,
					ClusterName:            "root",
					DatabaseUser:           "postgres",
					DatabaseName:           "postgres",
					Addr:                   listener.Addr().String(),
					ExternalErrorRetriever: &mockErrorRetriever{err: client.ErrClientCredentialsHaveExpired},
					LookupFunc: func(_ context.Context, _ string) (addrs []string, err error) {
						return []string{"memory"}, nil
					},
					DialContextFunc: listener.DialContext,
				},
			},
			expectErrorMessage: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.Equal(t, dbmcp.ReloginRequiredErrorMessage, i1)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			logger := slog.New(slog.DiscardHandler)
			rootServer := dbmcp.NewRootServer(logger)
			srv, err := NewServer(t.Context(), &dbmcp.NewServerConfig{
				Logger:     logger,
				RootServer: rootServer,
				Databases:  tc.databases,
			})
			require.NoError(t, err)
			t.Cleanup(func() { srv.Close(t.Context()) })

			pgSrv := srv.(*Server)
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{
				queryToolDatabaseParam: tc.databaseURI,
				queryToolQueryParam:    "SELECT 1",
			}
			runResult, err := pgSrv.RunQuery(t.Context(), req)
			require.NoError(t, err)

			require.True(t, runResult.IsError)
			require.Len(t, runResult.Content, 1)
			require.IsType(t, mcp.TextContent{}, runResult.Content[0])

			content := runResult.Content[0].(mcp.TextContent)
			var res RunQueryResult
			require.NoError(t, json.Unmarshal([]byte(content.Text), &res), "expected result to be in JSON format")
			require.Empty(t, res.Data)
			tc.expectErrorMessage(t, res.ErrorMessage)
		})
	}
}

func newMockRows(commandTag string, fields []string, rows [][]any) pgx.Rows {
	var fds []pgconn.FieldDescription
	for _, fieldName := range fields {
		fds = append(fds, pgconn.FieldDescription{Name: fieldName})
	}
	return &mockRows{
		commandTag:   commandTag,
		descriptions: fds,
		rows:         rows,
	}
}

type mockRows struct {
	pgx.Rows

	started bool
	cursor  int

	commandTag   string
	descriptions []pgconn.FieldDescription
	rows         [][]any
}

func (mr *mockRows) FieldDescriptions() []pgconn.FieldDescription {
	return mr.descriptions
}

func (mr *mockRows) Next() bool {
	if !mr.started {
		mr.started = true
		return len(mr.rows) > 0
	}

	mr.cursor += 1
	return len(mr.rows) > mr.cursor
}

func (mr *mockRows) Values() ([]any, error) {
	return mr.rows[mr.cursor], nil
}

func (mr *mockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag(mr.commandTag)
}

func (mr *mockRows) Close() {}

type mockErrorRetriever struct {
	err error
}

func (mr *mockErrorRetriever) RetrieveError() error {
	return mr.err
}
