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
	"crypto/rand"
	_ "embed"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
	"golang.org/x/term"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestREPL(t *testing.T) {
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
	repl.Run(t.Context())
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ctx := t.Context()

	user := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_USER"), "root")
	db := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_DB"), "mysql")
	pass := cmp.Or(os.Getenv("MYSQL_TEST_SERVER_PASS"), rand.Text())
	reuseName := os.Getenv("MYSQL_TEST_SERVER_REUSE_CONTAINER_BY_NAME")

	opts := []testcontainers.ContainerCustomizer{
		mysqlcontainer.WithDatabase(db),
		mysqlcontainer.WithUsername(user),
		mysqlcontainer.WithPassword(pass),
	}
	if reuseName != "" {
		opts = append(opts, testcontainers.WithReuseByName(reuseName))
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}

	// MySQL 8.4.6 index digest
	// https://docs.docker.com/dhi/core-concepts/digests/#multi-platform-images-and-manifests
	// https://hub.docker.com/layers/library/mysql/8.4.6/images/sha256-9b9413211d004062e6d5be1118e1051773fe66d832ec9ebcae1c2fcce5af5f5b
	const img = "mysql@sha256:d2c60b1b225c6d7845f0abdb596fc35c2d4122bcad6ec219588035a118f75d93"
	container, err := mysqlcontainer.Run(ctx, img, opts...)
	if reuseName == "" {
		defer testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "3306/tcp")
	require.NoError(t, err)

	return &testServer{
		host:     host,
		port:     mappedPort.Port(),
		username: user,
		password: pass,
		database: db,
	}
}

// testServer is a MySQL server for tests.
type testServer struct {
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
