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
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jonboulle/clockwork"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/listener"
	"github.com/gravitational/trace"
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

func TestIdleConnections(t *testing.T) {
	dbName := "local"
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:   dbName,
		Labels: map[string]string{"env": "test"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	dbConnsCh, dialDBFunc := newTestDatabaseServer(t)
	dbMCP := &dbmcp.Database{
		DB:           db,
		ClusterName:  "root",
		DatabaseUser: "postgres",
		DatabaseName: "postgres",
		LookupFunc: func(_ context.Context, _ string) (addrs []string, err error) {
			return []string{"memory"}, nil
		},
		DialContextFunc: dialDBFunc,
	}

	clock := clockwork.NewFakeClock()
	logger := slog.New(slog.DiscardHandler)
	rootServer := dbmcp.NewRootServer(logger)
	srv, err := NewServer(t.Context(), &dbmcp.NewServerConfig{
		Logger:     logger,
		RootServer: rootServer,
		Clock:      clock,
		Databases:  []*dbmcp.Database{dbMCP},
	})
	require.NoError(t, err)
	pgSrv := srv.(*Server)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		queryToolDatabaseParam: dbMCP.ResourceURI().WithoutParams().String(),
		queryToolQueryParam:    "INSERT (something) VALUES (other)",
	}

	// First query should initialize a new connection.
	runResult, err := pgSrv.runQuery(t.Context(), req)
	require.NoError(t, err)
	require.False(t, runResult.IsError, "expected query execution to succeed")

	var conn *testDatabaseConn
	select {
	case conn = <-dbConnsCh:
	case <-time.After(5 * time.Second):
		require.Fail(t, "expected database connection but got nothing")
	}

	require.True(t, conn.connected.Load(), "expected connection to be established: %s", conn.err.Load())
	require.False(t, conn.closed.Load(), "expected connection to not be closed: %s", conn.err.Load())

	// Issue no queries to server and advance clock to close the connection due
	// to inactivity.
	clock.Advance(connectionMaxIdleTime + 1)
	require.Eventually(t, func() bool {
		return conn.closed.Load()
	}, 5*time.Second, 100*time.Millisecond, "expected connection to be closed: %s", conn.err.Load())

	// Issuing a new query should bring a brand new connection alive.
	runResult, err = pgSrv.runQuery(t.Context(), req)
	require.NoError(t, err)
	require.False(t, runResult.IsError, "expected query execution to succeed")

	select {
	case conn = <-dbConnsCh:
	case <-time.After(5 * time.Second):
		require.Fail(t, "expected database connection but got nothing")
	}

	require.True(t, conn.connected.Load(), "expected connection to be established: %s", conn.err.Load())
	require.False(t, conn.closed.Load(), "expected connection to not be closed: %s", conn.err.Load())
}

type testDatabaseConn struct {
	closeOnce     sync.Once
	pgBackend     pgproto3.Backend
	rawClientConn net.Conn
	rawServerConn net.Conn

	err       atomic.Value
	closed    atomic.Bool
	connected atomic.Bool
}

func (td *testDatabaseConn) close() {
	td.closeOnce.Do(func() {
		td.rawClientConn.Close()
		td.rawServerConn.Close()
	})
	td.closed.Store(true)
}

func (td *testDatabaseConn) processMessages() {
	defer td.close()

	startupMessage, err := td.pgBackend.ReceiveStartupMessage()
	if err != nil {
		td.err.Store(err)
		return
	}

	switch msg := startupMessage.(type) {
	case *pgproto3.StartupMessage:
		// Accept auth and send ready for query.
		td.pgBackend.Send(&pgproto3.AuthenticationOk{})
		// Values on the backend key data are not relavant since we don't
		// support canceling requests.
		td.pgBackend.Send(&pgproto3.BackendKeyData{ProcessID: 0, SecretKey: 123})
		td.pgBackend.Send(&pgproto3.ReadyForQuery{})
		if err := td.pgBackend.Flush(); err != nil {
			td.err.Store(err)
			return
		}
	default:
		td.err.Store(trace.BadParameter("expected *pgproto3.StartupMessage, got: %T", msg))
		return
	}

	td.connected.Store(true)
	for {
		message, err := td.pgBackend.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return
			}

			td.err.Store(trace.Wrap(err))
			return
		}

		switch message.(type) {
		case *pgproto3.Query:
			td.pgBackend.Send(&pgproto3.CommandComplete{CommandTag: []byte("INSERT 0 1")})
			td.pgBackend.Send(&pgproto3.ReadyForQuery{})
			if err := td.pgBackend.Flush(); err != nil {
				td.err.Store(err)
				return
			}
		case *pgproto3.Terminate:
			return
		default:
			td.err.Store(trace.BadParameter("unsupported message %#v", message))
			return
		}
	}
}

func newTestDatabaseServer(t *testing.T) (chan *testDatabaseConn, dbmcp.DialContextFunc) {
	ch := make(chan *testDatabaseConn, 1)

	return ch, func(ctx context.Context, _, _ string) (net.Conn, error) {
		clientConn, serverConn := net.Pipe()
		pgBackend := pgproto3.NewBackend(serverConn, serverConn)
		td := &testDatabaseConn{
			rawClientConn: clientConn,
			rawServerConn: serverConn,
			pgBackend:     *pgBackend,
		}
		t.Cleanup(td.close)
		go td.processMessages()

		select {
		case <-t.Context().Done():
			return nil, trace.ConnectionProblem(t.Context().Err(), "connection failure")
		case ch <- td:
		}
		return clientConn, nil
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
