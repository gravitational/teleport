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
	"errors"
	"log/slog"
	"net"
	"testing"

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
		results        []*pgconn.Result
		expectedResult string
	}{
		"query results": {
			results:        []*pgconn.Result{newMockResult("SELECT 2", nil, []string{"name", "age"}, [][][]byte{{[]byte("Alice"), []byte("30")}, {[]byte("Bob"), []byte("31")}})},
			expectedResult: `{"results":[{"data":[{"age":"30","name":"Alice"},{"age":"31","name":"Bob"}],"rows_affected":2}]}`,
		},
		"multiple query results": {
			results: []*pgconn.Result{
				newMockResult("SELECT 2", nil, []string{"name", "age"}, [][][]byte{{[]byte("Alice"), []byte("30")}, {[]byte("Bob"), []byte("31")}}),
				newMockResult("SELECT 1", nil, []string{"id", "active"}, [][][]byte{{[]byte("1"), []byte("true")}}),
			},
			expectedResult: `{"results":[{"data":[{"age":"30","name":"Alice"},{"age":"31","name":"Bob"}],"rows_affected":2},{"data":[{"active":"true","id":"1"}],"rows_affected":1}]}`,
		},
		"multiple query results different types": {
			results: []*pgconn.Result{
				newMockResult("INSERT 5", nil, []string{}, [][][]byte{}),
				newMockResult("SELECT 2", nil, []string{"id", "active"}, [][][]byte{{[]byte("1"), []byte("true")}, {[]byte("2"), []byte("false")}}),
			},
			expectedResult: `{"results":[{"data":null,"rows_affected":5},{"data":[{"active":"true","id":"1"},{"active":"false","id":"2"}],"rows_affected":2}]}`,
		},
		"empty query results": {
			results:        []*pgconn.Result{newMockResult("SELECT 0", nil, []string{}, [][][]byte{})},
			expectedResult: `{"results":[{"data":[],"rows_affected":0}]}`,
		},
		"non-data results": {
			results:        []*pgconn.Result{newMockResult("INSERT 1", nil, []string{}, [][][]byte{})},
			expectedResult: `{"results":[{"data":null,"rows_affected":1}]}`,
		},
		"query with error": {
			results:        []*pgconn.Result{newMockResult("SELECT 0", errors.New("something wrong with query"), []string{}, [][][]byte{})},
			expectedResult: `{"results":[{"data":null,"rows_affected":0,"error":"something wrong with query"}]}`,
		},
		"multiple query with error": {
			results: []*pgconn.Result{
				newMockResult("INSERT 0", errors.New("constraint error"), []string{}, [][][]byte{}),
				newMockResult("SELECT 1", nil, []string{"id", "active"}, [][][]byte{{[]byte("1"), []byte("true")}}),
				newMockResult("SELECT 0", errors.New("something wrong with query"), []string{}, [][][]byte{}),
			},
			expectedResult: `{"results":[{"data":null,"rows_affected":0,"error":"constraint error"},{"data":[{"active":"true","id":"1"}],"rows_affected":1},{"data":null,"rows_affected":0,"error":"something wrong with query"}]}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := buildQueryResult(tc.results)
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
	randomDB, err := types.NewDatabaseV3(types.Metadata{
		Name:   "random-database-name",
		Labels: map[string]string{"env": "test"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	dbURI := clientmcp.NewDatabaseResourceURI("root", dbName).WithoutParams().String()

	for name, tc := range map[string]struct {
		databaseURI        string
		databases          []*dbmcp.Database
		expectErrorMessage require.ValueAssertionFunc
	}{
		"database not found": {
			databaseURI: "teleport://clusters/root/databases/not-found",
			databases: []*dbmcp.Database{
				{
					DB:           randomDB,
					ClusterName:  "root",
					DatabaseUser: "postgres",
					DatabaseName: "postgres",
				},
			},
			expectErrorMessage: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(t, dbmcp.DatabaseNotFoundError.Error(), i1)
			},
		},
		"malformed database uri": {
			databaseURI: "not-found",
			databases: []*dbmcp.Database{
				{
					DB:           randomDB,
					ClusterName:  "root",
					DatabaseUser: "postgres",
					DatabaseName: "postgres",
				},
			},
			expectErrorMessage: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(t, dbmcp.WrongDatabaseURIFormatError.Error(), i1)
			},
		},
		"local proxy rejects connection": {
			databaseURI: dbURI,
			databases: []*dbmcp.Database{
				{
					DB:           db,
					ClusterName:  "root",
					DatabaseUser: "postgres",
					DatabaseName: "postgres",
					LookupFunc: func(_ context.Context, _ string) (addrs []string, err error) {
						return []string{"memory"}, nil
					},
					DialContextFunc: listener.DialContext,
				},
			},
			expectErrorMessage: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(t, dbmcp.LocalProxyConnectionErrorMessage, i1)
			},
		},
		"relogin error": {
			databaseURI: dbURI,
			databases: []*dbmcp.Database{
				{
					DB:           db,
					ClusterName:  "root",
					DatabaseUser: "postgres",
					DatabaseName: "postgres",
					LookupFunc: func(_ context.Context, _ string) (addrs []string, err error) {
						return []string{"memory"}, nil
					},
					DialContextFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
						return nil, client.ErrClientCredentialsHaveExpired
					},
				},
			},
			expectErrorMessage: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Equal(t, clientmcp.ReloginRequiredErrorMessage, i1)
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
			runResult, err := pgSrv.runQuery(t.Context(), req)
			require.NoError(t, err)

			require.True(t, runResult.IsError)
			require.Len(t, runResult.Content, 1)
			require.IsType(t, mcp.TextContent{}, runResult.Content[0])

			content := runResult.Content[0].(mcp.TextContent)
			var res QueryResult
			require.NoError(t, json.Unmarshal([]byte(content.Text), &res), "expected result to be in JSON format")
			require.Empty(t, res.Data)
			tc.expectErrorMessage(t, res.ErrorMessage)
		})
	}
}

func newMockResult(commandTag string, err error, fields []string, rows [][][]byte) *pgconn.Result {
	var fds []pgconn.FieldDescription
	for _, fieldName := range fields {
		fds = append(fds, pgconn.FieldDescription{Name: fieldName})
	}
	return &pgconn.Result{
		FieldDescriptions: fds,
		Rows:              rows,
		CommandTag:        pgconn.NewCommandTag(commandTag),
		Err:               err,
	}
}
