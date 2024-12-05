// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package repl

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/client/db/postgres/repl/testdata"
	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestStart(t *testing.T) {
	ctx := context.Background()
	_, tc := StartWithServer(t, ctx)

	// Consume the REPL banner.
	_ = readUntilNextLead(t, tc)

	writeLine(t, tc, singleRowQuery)
	singleRowQueryResult := readUntilNextLead(t, tc)
	if golden.ShouldSet() {
		golden.SetNamed(t, "single", []byte(singleRowQueryResult))
	}
	require.Equal(t, string(golden.GetNamed(t, "single")), singleRowQueryResult)

	writeLine(t, tc, multiRowQuery)
	multiRowQueryResult := readUntilNextLead(t, tc)
	if golden.ShouldSet() {
		golden.SetNamed(t, "multi", []byte(multiRowQueryResult))
	}
	require.Equal(t, string(golden.GetNamed(t, "multi")), multiRowQueryResult)

	writeLine(t, tc, errorQuery)
	errorQueryResult := readUntilNextLead(t, tc)
	if golden.ShouldSet() {
		golden.SetNamed(t, "err", []byte(errorQueryResult))
	}
	require.Equal(t, string(golden.GetNamed(t, "err")), errorQueryResult)

	writeLine(t, tc, dataTypesQuery)
	dataTypeQueryResult := readUntilNextLead(t, tc)
	if golden.ShouldSet() {
		golden.SetNamed(t, "data_type", []byte(dataTypeQueryResult))
	}
	require.Equal(t, string(golden.GetNamed(t, "data_type")), dataTypeQueryResult)
}

// TestQuery given some input lines, the REPL should execute the expected
// query on the PostgreSQL test server.
func TestQuery(t *testing.T) {
	ctx := context.Background()
	_, tc := StartWithServer(t, ctx, WithCustomQueries())

	// Consume the REPL banner.
	_ = readUntilNextLead(t, tc)

	for name, tt := range map[string]struct {
		lines         []string
		expectedQuery string
	}{
		"query":                               {lines: []string{"SELECT 1;"}, expectedQuery: "SELECT 1;"},
		"multiline query":                     {lines: []string{"SELECT", "1", ";"}, expectedQuery: "SELECT\r\n1\r\n;"},
		"malformatted":                        {lines: []string{"SELECT err;"}, expectedQuery: "SELECT err;"},
		"query with special characters":       {lines: []string{"SELECT 'special_chars_!@#$%^&*()';"}, expectedQuery: "SELECT 'special_chars_!@#$%^&*()';"},
		"leading and trailing whitespace":     {lines: []string{"   SELECT 1;   "}, expectedQuery: "SELECT 1;"},
		"multiline with excessive whitespace": {lines: []string{"   SELECT", "    1", "     ;"}, expectedQuery: "SELECT\r\n1\r\n;"},
		// Commands should only be executed if they are at the beginning of the
		// first line.
		"with command in the middle":              {lines: []string{"SELECT \\d 1;"}, expectedQuery: "SELECT \\d 1;"},
		"multiline with command in the middle":    {lines: []string{"SELECT", "\\d", ";"}, expectedQuery: "SELECT\r\n\\d\r\n;"},
		"multiline with command in the last line": {lines: []string{"SELECT", "1", "\\d;"}, expectedQuery: "SELECT\r\n1\r\n\\d;"},
	} {
		t.Run(name, func(t *testing.T) {
			for _, line := range tt.lines {
				writeLine(t, tc, line)
			}

			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				query := <-tc.QueryChan()
				require.Equal(t, tt.expectedQuery, query)
			}, time.Second, time.Millisecond)

			// Always expect a query reply from the server.
			_ = readUntilNextLead(t, tc)
		})
	}
}

func writeLine(t *testing.T, c *testCtx, line string) {
	t.Helper()
	data := []byte(line + lineBreak)

	// When writing to the connection, the terminal emulator always writes back.
	// If we don't consume those bytes, it will block the ReadLine call (as
	// we're net.Pipe).
	go func(conn net.Conn) {
		buf := make([]byte, len(data))
		for {
			n, err := conn.Read(buf[0:])
			if err != nil {
				t.Logf("Error while terminal reply on write: %s", err)
				break
			}

			if string(buf[:n]) == line+lineBreak {
				break
			}
		}
	}(c.conn)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err := c.conn.Write(data)
		require.NoError(t, err)
	}, 5*time.Second, time.Millisecond)
}

// readUntilNextLead reads the contents from the client connection until we
// reach the next leading prompt.
func readUntilNextLead(t *testing.T, c *testCtx) string {
	t.Helper()

	var acc strings.Builder
	for {
		line := readLine(t, c)
		if strings.HasPrefix(line, lineBreak+lineLeading(c.route)) {
			break
		}

		acc.WriteString(line)
	}
	return acc.String()
}

func readLine(t *testing.T, c *testCtx) string {
	t.Helper()

	var n int
	buf := make([]byte, 1024)
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		var err error
		n, err = c.conn.Read(buf[0:])
		require.NoError(t, err)
		require.Greater(t, n, 0)
	}, 5*time.Second, time.Millisecond)
	return string(buf[:n])
}

type testCtx struct {
	cfg        *testCtxConfig
	ctx        context.Context
	cancelFunc context.CancelFunc

	// conn is the connection used by tests to read/write from/to the REPL.
	conn net.Conn
	// clientConn is the connection passed to the REPL.
	clientConn net.Conn
	// serverConn is the fake database server connection (that works as a
	// PostgreSQL instance).
	serverConn net.Conn

	route    clientproto.RouteToDatabase
	pgClient *pgproto3.Backend
	errChan  chan error
	// queryChan handling custom queries is enabled the queries received by the
	// test server will be sent to this channel.
	queryChan chan string
}

type testCtxConfig struct {
	// skipREPLRun when set to true the REPL instance won't be executed.
	skipREPLRun bool
	// handleCustomQueries when set to true the PostgreSQL test server will
	// accept any query sent and reply with success.
	handleCustomQueries bool
}

// testCtxOption represents a testCtx option.
type testCtxOption func(*testCtxConfig)

// WithCustomQueries enables sending custom queries to the PostgreSQL test
// server. Note that when it is enabled, callers must consume the queries on the
// query channel.
func WithCustomQueries() testCtxOption {
	return func(cfg *testCtxConfig) {
		cfg.handleCustomQueries = true
	}
}

// WithSkipREPLRun disables automatically running the REPL instance.
func WithSkipREPLRun() testCtxOption {
	return func(cfg *testCtxConfig) {
		cfg.skipREPLRun = true
	}
}

// StartWithServer starts a REPL instance with a PostgreSQL test server capable
// of receiving and replying to queries.
func StartWithServer(t *testing.T, ctx context.Context, opts ...testCtxOption) (*REPL, *testCtx) {
	t.Helper()

	cfg := &testCtxConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	conn, clientConn := net.Pipe()
	serverConn, pgConn := net.Pipe()
	client := pgproto3.NewBackend(pgproto3.NewChunkReader(pgConn), pgConn)
	ctx, cancelFunc := context.WithCancel(ctx)
	tc := &testCtx{
		cfg:        cfg,
		ctx:        ctx,
		cancelFunc: cancelFunc,
		conn:       conn,
		clientConn: clientConn,
		serverConn: serverConn,
		pgClient:   client,
		errChan:    make(chan error, 1),
		queryChan:  make(chan string),
	}

	t.Cleanup(func() {
		tc.close()
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			err, _ := <-tc.errChan
			assert.NoError(t, err)
		}, time.Second, time.Millisecond)
	})

	go func(c *testCtx) {
		defer close(c.errChan)
		if err := c.processMessages(); err != nil {
			c.errChan <- err
		}
	}(tc)

	r, err := New(ctx, tc.clientConn, tc.serverConn, tc.route)
	require.NoError(t, err)

	if !cfg.skipREPLRun {
		// Start the REPL session and return to the caller a channel that will
		// receive the execution result so it can assert REPL executions.
		runCtx, cancelRun := context.WithCancel(ctx)
		runErrChan := make(chan error, 1)
		go func() {
			runErrChan <- r.Run(runCtx)
		}()
		t.Cleanup(func() {
			cancelRun()

			select {
			case err := <-runErrChan:
				require.ErrorIs(t, err, context.Canceled, "expected the REPL instance to finish running with error due to cancelation")
			case <-time.After(10 * time.Second):
				require.Fail(t, "timeout while waiting for REPL Run result")
			}
		})
	}

	return r, tc
}

func (tc *testCtx) QueryChan() chan string {
	return tc.queryChan
}

func (tc *testCtx) close() {
	tc.serverConn.Close()
	tc.clientConn.Close()
}

func (tc *testCtx) processMessages() error {
	defer tc.close()

	startupMessage, err := tc.pgClient.ReceiveStartupMessage()
	if err != nil {
		return trace.Wrap(err)
	}

	switch msg := startupMessage.(type) {
	case *pgproto3.StartupMessage:
		// Accept auth and send ready for query.
		if err := tc.pgClient.Send(&pgproto3.AuthenticationOk{}); err != nil {
			return trace.Wrap(err)
		}

		// Values on the backend key data are not relavant since we don't
		// support canceling requests.
		err := tc.pgClient.Send(&pgproto3.BackendKeyData{
			ProcessID: 0,
			SecretKey: 123,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if err := tc.pgClient.Send(&pgproto3.ReadyForQuery{}); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("expected *pgproto3.StartupMessage, got: %T", msg)
	}

	for {
		message, err := tc.pgClient.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}

			return trace.Wrap(err)
		}

		var messages []pgproto3.BackendMessage
		switch msg := message.(type) {
		case *pgproto3.Query:
			if tc.cfg.handleCustomQueries {
				select {
				case tc.queryChan <- msg.String:
					messages = []pgproto3.BackendMessage{
						&pgproto3.CommandComplete{CommandTag: pgconn.CommandTag("INSERT 0 1")},
						&pgproto3.ReadyForQuery{},
					}
				case <-tc.ctx.Done():
					return trace.Wrap(tc.ctx.Err())
				}

				break // breaks the message switch case.
			}

			switch msg.String {
			case singleRowQuery:
				messages = []pgproto3.BackendMessage{
					&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("id")}, {Name: []byte("email")}}},
					&pgproto3.DataRow{Values: [][]byte{[]byte("1"), []byte("alice@example.com")}},
					&pgproto3.CommandComplete{CommandTag: pgconn.CommandTag("SELECT")},
					&pgproto3.ReadyForQuery{},
				}
			case multiRowQuery:
				messages = []pgproto3.BackendMessage{
					&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("id")}, {Name: []byte("email")}}},
					&pgproto3.DataRow{Values: [][]byte{[]byte("1"), []byte("alice@example.com")}},
					&pgproto3.DataRow{Values: [][]byte{[]byte("2"), []byte("bob@example.com")}},
					&pgproto3.CommandComplete{CommandTag: pgconn.CommandTag("SELECT")},
					&pgproto3.ReadyForQuery{},
				}
			case dataTypesQuery:
				messages = testdata.TestDataQueryResult
			case errorQuery:
				messages = []pgproto3.BackendMessage{
					&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42703", Message: "error"},
					&pgproto3.ReadyForQuery{},
				}
			default:
				return trace.BadParameter("unsupported query %q", msg.String)

			}
		case *pgproto3.Terminate:
			return nil
		default:
			return trace.BadParameter("unsupported message %#v", message)
		}

		for _, message := range messages {
			err := tc.pgClient.Send(message)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

const (
	singleRowQuery = "SELECT * FROM users LIMIT 1;"
	multiRowQuery  = "SELECT * FROM users;"
	dataTypesQuery = "SELECT * FROM test_data_types;"
	errorQuery     = "SELECT err;"
)
