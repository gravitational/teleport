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
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgproto3/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

const crlf = "\r\n"

var testSrv *testServer
var testSrvMu sync.Mutex

func TestMain(m *testing.M) {
	code := m.Run()

	// The "Ryuk" container will cleanup everything anyhow, but only after 10s
	// without any client connections.
	// We can speed up container cleanup after a -count=N run, e.g., a flaky
	// test detector run, by terminating them immediately after tests have run.
	// https://github.com/testcontainers/moby-ryuk
	testSrvMu.Lock()
	defer testSrvMu.Unlock()
	// if we don't clean up after 10 seconds, RYUK will get them anyway.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ignore errors, Ryuk or worst case VM termination in CI will deal with it
	if testSrv != nil {
		_ = testSrv.container.Terminate(ctx)
	}
	os.Exit(code)
}

func TestClose(t *testing.T) {
	t.Parallel()
	for name, tt := range map[string]struct {
		closeFunc              func(tc *testCtx, cancelCtx context.CancelFunc)
		expectTerminateMessage bool
	}{
		"closed by context": {
			closeFunc: func(_ *testCtx, cancelCtx context.CancelFunc) {
				cancelCtx()
			},
			expectTerminateMessage: true,
		},
		"closed by server": {
			closeFunc: func(tc *testCtx, _ context.CancelFunc) {
				tc.CloseServer()
			},
			expectTerminateMessage: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancelFunc := context.WithCancel(t.Context())
			defer cancelFunc()

			_, tc := StartWithServer(t, ctx)
			// Consume the REPL banner.
			_ = readUntilNextLead(t, tc)

			tt.closeFunc(tc, cancelFunc)
			// After closing the REPL session, we expect any read/write to
			// return error. In case the close wasn't effective we need to
			// execute the read on a Eventually block to avoid blocking the
			// test.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				var buf []byte
				_, err := tc.conn.Read(buf[0:])
				require.ErrorIs(t, err, io.EOF)
			}, 5*time.Second, time.Millisecond)

			if !tt.expectTerminateMessage {
				return
			}

			select {
			case <-tc.terminateChan:
			case <-time.After(5 * time.Second):
				require.Fail(t, "expected REPL to send terminate message but got nothing")
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc            string
		modifyTestCtx   func(tc *testCtx)
		wantErrContains string
	}{
		{
			desc: "closed server",
			// Force the server to be closed
			modifyTestCtx:   func(tc *testCtx) { tc.CloseServer() },
			wantErrContains: "failed to write startup message",
		},
		{
			desc:            "access denied",
			modifyTestCtx:   func(tc *testCtx) { tc.denyAccess = true },
			wantErrContains: "server error (ERROR: access to db denied (SQLSTATE 28000))",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			ctx := t.Context()
			instance, tc := StartWithServer(t, ctx, WithSkipREPLRun())

			test.modifyTestCtx(tc)
			err := instance.Run(ctx)
			require.Error(t, err)
			require.True(t, trace.IsConnectionProblem(err), "expected run to be a connection error but got %T", err)
			require.ErrorContains(t, err, test.wantErrContains)
		})
	}
}

// readUntilNextLead reads the contents from the client connection until we
// reach the next leading prompt.
func readUntilNextLead(t *testing.T, c *testCtx) string {
	t.Helper()

	var acc strings.Builder
	for {
		line := readLine(t, c)
		if strings.HasPrefix(line, lineLeading(c.route)) {
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
	// Given that the test connections are piped a problem with the writer side
	// would lead into blocking reading. To avoid this scenario we're using
	// the Eventually just to ensure a timeout on reading from the connections.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		var err error
		n, err = c.conn.Read(buf[0:])
		require.NoError(t, err)
		require.Greater(t, n, 0)
	}, 5*time.Second, time.Millisecond)
	return string(buf[:n])
}

// testCtx implements a minimal stub Postgres backend that can only PostgreSQL
// messages for startup and termination. Tests that require support for other
// message types should use a real Postgres server running in a test container.
type testCtx struct {
	cfg        *testCtxConfig
	ctx        context.Context
	cancelFunc context.CancelFunc
	// denyAccess controls whether access is denied during authentication
	denyAccess bool

	// conn is the connection used by tests to read/write from/to the REPL.
	conn net.Conn
	// clientConn is the connection passed to the REPL.
	clientConn net.Conn
	// serverConn is the fake database server connection (that works as a
	// PostgreSQL instance).
	serverConn net.Conn
	// rawPgConn is the underlaying net.Conn used by pgconn client.
	rawPgConn net.Conn

	route         clientproto.RouteToDatabase
	pgClient      *pgproto3.Backend
	errChan       chan error
	terminateChan chan struct{}
}

type testCtxConfig struct {
	// skipREPLRun when set to true the REPL instance won't be executed.
	skipREPLRun bool
}

// testCtxOption represents a testCtx option.
type testCtxOption func(*testCtxConfig)

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
		cfg:           cfg,
		ctx:           ctx,
		cancelFunc:    cancelFunc,
		conn:          conn,
		clientConn:    clientConn,
		serverConn:    serverConn,
		rawPgConn:     pgConn,
		pgClient:      client,
		errChan:       make(chan error, 1),
		terminateChan: make(chan struct{}),
	}

	t.Cleanup(func() {
		tc.close()

		select {
		case err := <-tc.errChan:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			require.Fail(t, "expected to receive the test server close result but got nothing")
		}
	})

	go func(c *testCtx) {
		defer close(c.errChan)
		if err := c.processMessages(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			c.errChan <- err
		}
	}(tc)

	instance, err := New(ctx, &dbrepl.NewREPLConfig{Client: tc.clientConn, ServerConn: tc.serverConn, Route: tc.route})
	require.NoError(t, err)

	if !cfg.skipREPLRun {
		// Start the REPL session and return to the caller a channel that will
		// receive the execution result so it can assert REPL executions.
		runCtx, cancelRun := context.WithCancel(ctx)
		runErrChan := make(chan error, 1)
		go func() {
			runErrChan <- instance.Run(runCtx)
		}()
		t.Cleanup(func() {
			cancelRun()

			select {
			case err := <-runErrChan:
				if !errors.Is(err, context.Canceled) && !errors.Is(err, io.ErrClosedPipe) {
					require.Fail(t, "expected the REPL instance to finish with context cancelation or server closed pipe but got %q", err)
				}
			case <-time.After(10 * time.Second):
				require.Fail(t, "timeout while waiting for REPL Run result")
			}
		})
	}

	return instance.(*REPL), tc
}

func (tc *testCtx) CloseServer() {
	tc.rawPgConn.Close()
}

func (tc *testCtx) close() {
	tc.serverConn.Close()
	tc.clientConn.Close()
}

func (tc *testCtx) processMessages() error {
	defer tc.close()

	startupMessage, err := tc.pgClient.ReceiveStartupMessage()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return trace.Wrap(err)
	}

	switch msg := startupMessage.(type) {
	case *pgproto3.StartupMessage:
		if tc.denyAccess {
			if err := tc.pgClient.Send(&pgproto3.ErrorResponse{
				Severity: "ERROR",
				Code:     pgerrcode.InvalidAuthorizationSpecification,
				Message:  "access to db denied",
			}); err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
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

		switch msg := message.(type) {
		case *pgproto3.Query:
			return trace.BadParameter("stub postgres server does not support queries, use a testcontainer to test query %q", msg.String)
		case *pgproto3.Terminate:
			close(tc.terminateChan)
			return nil
		default:
			return trace.BadParameter("unsupported message %#v", message)
		}
	}
}

func TestREPL(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("ENABLE_TESTCONTAINERS")); !run {
		// Docker Hub rate limits cause failures in CI, this test is disabled until we can set up an alternative to Docker Hub
		t.Skip("Test disabled in CI. Enable it by setting env variable ENABLE_TESTCONTAINERS")
	}
	testSrv := newTestServer(t)
	route := clientproto.RouteToDatabase{
		ServiceName: "postgres-test-container",
		Protocol:    defaults.ProtocolPostgres,
		Username:    testSrv.username,
		Database:    testSrv.database,
	}
	for _, test := range []struct {
		name  string
		input string
	}{
		{
			name:  "ddl",
			input: testDDL,
		},
		{
			name:  "all data types",
			input: testDataTypes,
		},
		{
			name:  "literals",
			input: testLiterals,
		},
		{
			name:  "commands",
			input: testCommands,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			input := strings.ReplaceAll(test.input, "\n", crlf) + crlf
			recorder := newRecordingClient(io.NopCloser(strings.NewReader(input)))

			t.Logf("test server: %v", testSrv.hostPort())
			testREPL, err := New(ctx, &dbrepl.NewREPLConfig{
				Client:     recorder,
				ServerConn: testSrv.connectTCP(t, 5*time.Second),
				Route:      route,
			})
			require.NoError(t, err)
			testREPL.(*REPL).connConfig.Password = testSrv.password
			testREPL.(*REPL).teleportVersion = "19.0.0-dev"
			err = testREPL.Run(t.Context())
			require.NoError(t, err)
			if golden.ShouldSet() {
				golden.SetNamed(t, "", recorder.buf.Bytes())
			}
			require.Equal(t, string(golden.GetNamed(t, "")), recorder.buf.String())
		})
	}
}

var (
	//go:embed fixtures/ddl.sql
	testDDL string
	//go:embed fixtures/all-data-types.sql
	testDataTypes string
	//go:embed fixtures/literals.sql
	testLiterals string
	//go:embed fixtures/commands.sql
	testCommands string
)

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ctx := t.Context()

	reuseName := cmp.Or(os.Getenv("POSTGRES_TEST_SERVER_REUSE_CONTAINER_BY_NAME"), "default-postgres-test-server")
	testSrvMu.Lock()
	// hold the lock for the entire func to avoid parallel container requests
	defer testSrvMu.Unlock()
	if testSrv != nil {
		return testSrv
	}

	user := cmp.Or(os.Getenv("POSTGRES_TEST_SERVER_USER"), "postgres")
	db := cmp.Or(os.Getenv("POSTGRES_TEST_SERVER_DB"), "postgres")
	pass := cmp.Or(os.Getenv("POSTGRES_TEST_SERVER_PASS"), rand.Text())
	opts := []testcontainers.ContainerCustomizer{
		postgres.WithDatabase(db),
		postgres.WithUsername(user),
		postgres.WithPassword(pass),
		testcontainers.WithReuseByName(reuseName),
		postgres.BasicWaitStrategies(),
		postgres.WithSQLDriver("pgx"),
	}

	// postgres 17
	const img = "postgres@sha256:feff5b24fedd610975a1f5e743c51a4b360437f4dc3a11acf740dcd708f413f6"
	container, err := postgres.Run(ctx, img, opts...)
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	srv := &testServer{
		container: container,
		host:      host,
		port:      mappedPort.Port(),
		username:  user,
		password:  pass,
		database:  db,
	}
	testSrv = srv
	return srv
}

// testServer is a PostgreSQL server for tests.
type testServer struct {
	// container is the container hosting the server.
	container testcontainers.Container
	// host is the PostgreSQL connection endpoint host.
	host string
	// port is the PostgreSQL connection endpoint port.
	port string
	// username is the database superuser.
	username string
	// password is the database superuser password.
	password string
	// database is a database schema provisioned in the database.
	database string
}

// hostPort returns the server host:port.
func (s *testServer) hostPort() string {
	return net.JoinHostPort(s.host, s.port)
}

func (s *testServer) connectTCP(t *testing.T, timeout time.Duration) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", s.hostPort(), timeout)
	require.NoError(t, err)
	return conn
}

func newRecordingClient(reader io.ReadCloser) *recordingClient {
	var buf bytes.Buffer
	return &recordingClient{
		ReadCloser: reader,
		Writer:     &buf,
		buf:        &buf,
	}
}

type recordingClient struct {
	io.ReadCloser
	io.Writer
	buf *bytes.Buffer
}
