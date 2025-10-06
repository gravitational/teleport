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

package repl

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	_ "embed"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
	"golang.org/x/term"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	apiutils "github.com/gravitational/teleport/api/utils"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

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

func TestREPL(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("ENABLE_TESTCONTAINERS")); !run {
		// Docker Hub rate limits cause failures in CI, this test is disabled until we can set up an alternative to Docker Hub
		t.Skip("Test disabled in CI. Enable it by setting env variable ENABLE_TESTCONTAINERS")
	}
	testSrv := newTestServer(t)
	route := clientproto.RouteToDatabase{
		ServiceName: "mysql-test-container",
		Protocol:    defaults.ProtocolMySQL,
		Username:    testSrv.username,
		// Pass a blank database name, as if a user who didn't specify one.
		// The REPL should handle this case gracefully.
		Database: "",
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
			// the term only recognizes line breaks with a carriage-return
			const crlf = "\r\n"
			input := strings.ReplaceAll(test.input, "\n", crlf) + crlf
			recorder := newRecordingClient(t, io.NopCloser(strings.NewReader(input)))
			testREPL, err := New(t.Context(), &dbrepl.NewREPLConfig{
				Client:     recorder,
				ServerConn: testSrv.connectTCP(t, 10*time.Second),
				Route:      route,
			})
			require.NoError(t, err)
			testREPL.(*REPL).testPassword = testSrv.password
			testREPL.(*REPL).disableQueryTimings = true
			testREPL.(*REPL).teleportVersion = "19.0.0-dev"
			err = testREPL.Run(t.Context())
			require.NoError(t, err)
			goldenName := strings.ReplaceAll(test.name, " ", "-")
			if golden.ShouldSet() {
				golden.SetNamed(t, goldenName, recorder.buf.Bytes())
			}
			require.Equal(t, string(golden.GetNamed(t, goldenName)), recorder.buf.String())
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

func TestGetPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc         string
		inQuery      bool
		inComment    bool
		inStringKind string
		database     string
		want         string
	}{
		{
			desc: "no default database",
			want: "(none)> ",
		},
		{
			desc:     "default",
			database: "llama",
			want:     "llama> ",
		},
		{
			desc:     "in a query",
			inQuery:  true,
			database: "llama",
			want:     "    -> ",
		},
		{
			desc:      "in a comment",
			inComment: true,
			database:  "llama",
			want:      "   /*> ",
		},
		{
			desc:         "in a quote string",
			inStringKind: "'",
			database:     "llama",
			want:         "    '> ",
		},
		{
			desc:         "in a double quote string",
			inStringKind: `"`,
			database:     "llama",
			want:         `    "> `,
		},
		{
			desc:         "in a backtick string",
			inStringKind: "`",
			database:     "llama",
			want:         "    `> ",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			r := &REPL{
				route: clientproto.RouteToDatabase{
					Database: test.database,
				},
				parser: &parser{
					lex: lexer{
						inComment:     test.inComment,
						inStringToken: token{text: test.inStringKind},
					},
				},
				teleportVersion: "v19.0.0-dev",
			}
			if test.inQuery {
				r.parser.lex.queryBuf.WriteString("select")
			}
			require.Equal(t, test.want, r.getPrompt())
		})
	}
}

func FuzzEval(f *testing.F) {
	parser, err := newParser()
	require.NoError(f, err)

	for _, cmd := range parser.commands.byName {
		f.Add(cmd.name)
		f.Add(parser.commands.shortcutPrefix + string(cmd.shortcut))
	}

	f.Add("")  // eof
	f.Add(" ") // space

	// MySQL string literal quotes
	f.Add("'")
	f.Add(`"`)
	f.Add("`")

	// single line comments
	f.Add("--")
	f.Add("#")

	// multiline comments
	f.Add("/*")
	f.Add("*/")
	f.Add("*")

	// valid delimiters
	f.Add(";")
	f.Add("$")
	f.Add("/")

	// some statement syntax
	f.Add("select")
	f.Add("create")
	f.Add("drop")
	f.Add("replace")
	f.Add("alter")
	f.Add("(")
	f.Add(")")

	// multiline
	f.Add("\n")

	f.Fuzz(func(t *testing.T, line string) {
		repl := &REPL{
			myConn: &fakeMySQLConn{
				exec: func(command string, args ...any) (*mysql.Result, error) {
					return nil, io.EOF
				},
			},
			parser: parser,
		}
		require.NotPanics(t, func() {
			for line := range strings.SplitSeq(line, "\n") {
				for _, exit := range repl.eval(line) {
					if exit {
						return
					}
				}
			}
		})
	})
}

func TestInteractively(t *testing.T) {
	// You can get an interactive REPL to experiment with like this:
	// $ export MYSQL_TEST_REPL_INTERACTIVELY=1
	// $ go test -c -o ./test ./lib/client/db/mysql/repl
	// $ ./test -test.v -test.count=1 -test.run TestInteractively
	if os.Getenv("MYSQL_TEST_REPL_INTERACTIVELY") == "" {
		t.Skip()
	}
	testSrv := newTestServer(t)
	route := clientproto.RouteToDatabase{
		ServiceName: "mysql-test-container",
		Protocol:    defaults.ProtocolMySQL,
		Username:    testSrv.username,
		Database:    testSrv.database,
		Roles:       []string{},
	}
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	require.NoError(t, err)
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	recorder := newRecordingClient(t, os.Stdin, os.Stdout)
	repl, err := New(t.Context(), &dbrepl.NewREPLConfig{
		Client:     recorder,
		ServerConn: testSrv.connectTCP(t, 10*time.Second),
		Route:      route,
	})
	require.NoError(t, err)
	repl.(*REPL).testPassword = testSrv.password
	require.NoError(t, repl.Run(t.Context()))
}

func Test_summarizeResult(t *testing.T) {
	resultRows, err := mysql.BuildSimpleTextResultset(
		[]string{"one", "two", "three"},
		[][]any{{1, 2.2, "3"}, {4, 5.5, "6"}, {7, 8.8, "8"}},
	)
	require.NoError(t, err)

	noRows, err := mysql.BuildSimpleTextResultset(
		[]string{"one", "two", "three"},
		nil,
	)
	require.NoError(t, err)

	tests := []struct {
		desc    string
		input   *mysql.Result
		elapsed time.Duration
	}{
		{
			desc:    "result rows",
			input:   &mysql.Result{Resultset: resultRows},
			elapsed: time.Millisecond * 1234,
		},
		{
			desc:    "long elapsed time",
			input:   &mysql.Result{Resultset: resultRows},
			elapsed: time.Millisecond * 123456,
		},
		{
			desc:    "empty set",
			input:   &mysql.Result{Resultset: noRows},
			elapsed: time.Millisecond * 42,
		},
		{
			desc:    "nil set",
			input:   &mysql.Result{Warnings: 2, AffectedRows: 42},
			elapsed: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			mustParseResult(t, test.input)
			var numRows int
			if test.input.Resultset != nil {
				numRows = len(test.input.Resultset.Values)
			}
			got := summarizeResult(test.input, numRows, &test.elapsed)
			goldenName := strings.ReplaceAll(test.desc, " ", "-")
			if golden.ShouldSet() {
				golden.SetNamed(t, goldenName, []byte(got))
			}
			require.Equal(t, string(golden.GetNamed(t, goldenName)), got)
		})
	}
}

func mustParseResult(t *testing.T, result *mysql.Result) {
	t.Helper()
	if result.Resultset == nil {
		return
	}
	result.Values = make([][]mysql.FieldValue, len(result.RowDatas))
	for i := range result.RowDatas {
		const isBinary = false
		v, err := result.RowDatas[i].Parse(result.Fields, isBinary, result.Values[i])
		result.Values[i] = v
		require.NoError(t, err)
	}
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ctx := t.Context()

	reuseName := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_REUSE_CONTAINER_BY_NAME"), "default-mysql-test-server")
	testSrvMu.Lock()
	// hold the lock for the entire func to avoid parallel container requests
	defer testSrvMu.Unlock()
	if testSrv != nil {
		return testSrv
	}

	user := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_USER"), "root")
	db := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_DB"), "mysql")
	pass := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_PASS"), rand.Text())
	opts := []testcontainers.ContainerCustomizer{
		mysqlcontainer.WithDatabase(db),
		mysqlcontainer.WithUsername(user),
		mysqlcontainer.WithPassword(pass),
		testcontainers.WithReuseByName(reuseName),
	}

	// MySQL 8.4.6 index digest
	// https://docs.docker.com/dhi/core-concepts/digests/#multi-platform-images-and-manifests
	// https://hub.docker.com/layers/library/mysql/8.4.6/images/sha256-9b9413211d004062e6d5be1118e1051773fe66d832ec9ebcae1c2fcce5af5f5b
	const img = "mysql@sha256:d2c60b1b225c6d7845f0abdb596fc35c2d4122bcad6ec219588035a118f75d93"
	container, err := mysqlcontainer.Run(ctx, img, opts...)
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "3306/tcp")
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

// testServer is a MySQL server for tests.
type testServer struct {
	// container is the container hosting the server.
	container testcontainers.Container
	// host is the MySQL connection endpoint host.
	host string
	// port is the MySQL connection endpoint port.
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

func newRecordingClient(t *testing.T, reader io.ReadCloser, writers ...io.Writer) *recordingClient {
	t.Helper()
	var buf bytes.Buffer
	writers = append(writers, &buf)
	return &recordingClient{
		ReadCloser: reader,
		Writer:     io.MultiWriter(writers...),
		buf:        &buf,
	}
}

type recordingClient struct {
	io.ReadCloser
	io.Writer
	buf *bytes.Buffer
}
