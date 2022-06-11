// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbcmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type commandPathBehavior = int

const (
	system commandPathBehavior = iota
	forceAbsolutePath
	forceBasePath
)

// fakeExec implements execer interface for mocking purposes.
type fakeExec struct {
	// execOutput maps binary name and output that should be returned on RunCommand().
	// Map is also being used to check if a binary exist. Command line args are not supported.
	execOutput map[string][]byte
	// commandPathBehavior controls what kind of path will be returned from fakeExec.Command:
	// * system just calls exec.Command
	// * forceAbsolutePath guarantees that the returned cmd.Path will be absolute
	// * forceBasePath guarantees that the returned cmd.Path will be just the binary name
	commandPathBehavior commandPathBehavior
}

func (f fakeExec) RunCommand(cmd string, _ ...string) ([]byte, error) {
	out, found := f.execOutput[cmd]
	if !found {
		return nil, errors.New("binary not found")
	}

	return out, nil
}

func (f fakeExec) LookPath(path string) (string, error) {
	if _, found := f.execOutput[path]; found {
		return "", nil
	}
	return "", trace.NotFound("not found")
}

func (f fakeExec) Command(name string, arg ...string) *exec.Cmd {
	switch f.commandPathBehavior {
	case system:
		return exec.Command(name, arg...)
	case forceAbsolutePath:
		path, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		absolutePath := filepath.Join(path, name)
		cmd := exec.Command(absolutePath, arg...)

		return cmd
	case forceBasePath:
		cmd := exec.Command(name, arg...)
		cmd.Path = filepath.Base(cmd.Path)
		return cmd
	}
	panic("Unknown commandPathBehavior")
}

func TestCLICommandBuilderGetConnectCommand(t *testing.T) {
	conf := &client.Config{
		HomePath:     t.TempDir(),
		Host:         "localhost",
		WebProxyAddr: "localhost",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
	}

	tests := []struct {
		name         string
		opts         []ConnectCommandFunc
		dbProtocol   string
		databaseName string
		execer       *fakeExec
		cmd          []string
		wantErr      bool
	}{
		{
			name:         "postgres",
			dbProtocol:   defaults.ProtocolPostgres,
			databaseName: "mydb",
			execer:       &fakeExec{},
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:         "postgres no TLS",
			dbProtocol:   defaults.ProtocolPostgres,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer:       &fakeExec{},
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb"},
			wantErr: false,
		},
		{
			name:         "postgres print format",
			dbProtocol:   defaults.ProtocolPostgres,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithPrintFormat()},
			execer:       &fakeExec{},
			cmd: []string{"psql",
				"\"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full\""},
			wantErr: false,
		},
		{
			name:         "cockroach",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"cockroach": []byte(""),
				},
			},
			cmd: []string{"cockroach", "sql", "--url",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:         "cockroach no TLS",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"cockroach": []byte(""),
				},
			},
			cmd: []string{"cockroach", "sql", "--url",
				"postgres://myUser@localhost:12345/mydb"},
			wantErr: false,
		},
		{
			name:         "cockroach print format",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithPrintFormat()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"cockroach": []byte(""),
				},
			},
			cmd: []string{"cockroach", "sql", "--url",
				"\"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full\""},
			wantErr: false,
		},
		{
			name:         "cockroach psql fallback",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			execer:       &fakeExec{},
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem&" +
					"sslkey=/tmp/keys/example.com/bob&sslmode=verify-full"},
			wantErr: false,
		},
		{
			name:         "mariadb",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mariadb": []byte(""),
				},
			},
			cmd: []string{"mariadb",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP",
				"--ssl-key", "/tmp/keys/example.com/bob",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:         "mariadb no TLS",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mariadb": []byte(""),
				},
			},
			cmd: []string{"mariadb",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "mysql by mariadb",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("mysql  Ver 15.1 Distrib 10.3.32-MariaDB, for debian-linux-gnu (x86_64) using readline 5.2"),
				},
			},
			cmd: []string{"mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP",
				"--ssl-key", "/tmp/keys/example.com/bob",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:         "mysql by oracle",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("Ver 8.0.27-0ubuntu0.20.04.1 for Linux on x86_64 ((Ubuntu))"),
				},
			},
			cmd: []string{"mysql",
				"--defaults-group-suffix=_db.example.com-mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "mysql no TLS",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("Ver 8.0.27-0ubuntu0.20.04.1 for Linux on x86_64 ((Ubuntu))"),
				},
			},
			cmd: []string{"mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "no mysql nor mariadb",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd:     []string{},
			wantErr: true,
		},
		{
			name:         "no mysql nor mariadb with no TLS and tolerateMissingCLIClient",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS(), WithTolerateMissingCLIClient()},
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mysql",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "12345",
				"--host", "localhost",
				"--protocol", "TCP"},
			wantErr: false,
		},
		{
			name:         "mongodb (legacy)",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mongo",
				"--host", "localhost",
				"--port", "12345",
				"--ssl",
				"--sslPEMKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"mydb"},
			wantErr: false,
		},
		{
			name:         "mongodb no TLS",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{},
			},
			cmd: []string{"mongo",
				"--host", "localhost",
				"--port", "12345",
				"mydb"},
			wantErr: false,
		},
		{
			name:         "mongosh",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--host", "localhost",
				"--port", "12345",
				"--tls",
				"--tlsCertificateKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--tlsUseSystemCA",
				"mydb"},
		},
		{
			name:         "mongosh",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts: []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, "/tmp/keys/example.com/cas/example.com.pem")},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--host", "localhost",
				"--port", "12345",
				"--tls",
				"--tlsCertificateKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"--tlsCAFile", "/tmp/keys/example.com/cas/example.com.pem",
				"mydb"},
		},
		{
			name:         "mongosh no TLS",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--host", "localhost",
				"--port", "12345",
				"mydb"},
		},
		{
			name:         "sqlserver",
			dbProtocol:   defaults.ProtocolSQLServer,
			databaseName: "mydb",
			execer:       &fakeExec{},
			cmd: []string{mssqlBin,
				"-S", "localhost,12345",
				"-U", "myUser",
				"-P", fixtures.UUID,
				"-d", "mydb",
			},
			wantErr: false,
		},
		{
			name:       "redis-cli",
			dbProtocol: defaults.ProtocolRedis,
			execer:     &fakeExec{},
			cmd: []string{"redis-cli",
				"-h", "localhost",
				"-p", "12345",
				"--tls",
				"--key", "/tmp/keys/example.com/bob",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem"},
			wantErr: false,
		},
		{
			name:         "redis-cli with db",
			dbProtocol:   defaults.ProtocolRedis,
			databaseName: "2",
			execer:       &fakeExec{},
			cmd: []string{"redis-cli",
				"-h", "localhost",
				"-p", "12345",
				"--tls",
				"--key", "/tmp/keys/example.com/bob",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql-x509.pem",
				"-n", "2"},
			wantErr: false,
		},
		{
			name:       "redis-cli no TLS",
			dbProtocol: defaults.ProtocolRedis,
			opts:       []ConnectCommandFunc{WithNoTLS()},
			execer:     &fakeExec{},
			cmd: []string{"redis-cli",
				"-h", "localhost",
				"-p", "12345"},
			wantErr: false,
		},
		{
			name:       "snowsql no TLS",
			dbProtocol: defaults.ProtocolSnowflake,
			opts:       []ConnectCommandFunc{WithNoTLS()},
			execer:     &fakeExec{},
			cmd: []string{"snowsql",
				"-a", "teleport",
				"-u", "myUser",
				"-h", "localhost",
				"-p", "12345"},
			wantErr: false,
		},
		{
			name:         "snowsql db-name no TLS",
			dbProtocol:   defaults.ProtocolSnowflake,
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			cmd: []string{"snowsql",
				"-a", "teleport",
				"-u", "myUser",
				"-h", "localhost",
				"-p", "12345",
				"-w", "warehouse1"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			database := &tlsca.RouteToDatabase{
				Protocol:    tt.dbProtocol,
				Database:    tt.databaseName,
				Username:    "myUser",
				ServiceName: "mysql",
			}

			opts := append([]ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
			}, tt.opts...)

			c := NewCmdBuilder(tc, profile, database, "root", opts...)
			c.uid = utils.NewFakeUID()
			c.exe = tt.execer
			got, err := c.GetConnectCommand()
			if tt.wantErr {
				if err == nil {
					t.Errorf("getConnectCommand() should return an error, but it didn't")
				}
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.cmd, got.Args)
		})
	}
}

func TestGetConnectCommandNoAbsPathConvertsAbsolutePathToRelative(t *testing.T) {
	conf := &client.Config{
		HomePath:     t.TempDir(),
		Host:         "localhost",
		WebProxyAddr: "localhost",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
	}

	database := &tlsca.RouteToDatabase{
		Protocol:    defaults.ProtocolPostgres,
		Database:    "mydb",
		Username:    "myUser",
		ServiceName: "postgres",
	}

	opts := []ConnectCommandFunc{
		WithLocalProxy("localhost", 12345, ""),
		WithNoTLS(),
	}

	c := NewCmdBuilder(tc, profile, database, "root", opts...)
	c.uid = utils.NewFakeUID()
	c.exe = &fakeExec{commandPathBehavior: forceAbsolutePath}

	got, err := c.GetConnectCommandNoAbsPath()
	require.NoError(t, err)
	require.Equal(t, "psql postgres://myUser@localhost:12345/mydb", got.String())
}

func TestGetConnectCommandNoAbsPathIsNoopWhenGivenRelativePath(t *testing.T) {
	conf := &client.Config{
		HomePath:     t.TempDir(),
		Host:         "localhost",
		WebProxyAddr: "localhost",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
	}

	database := &tlsca.RouteToDatabase{
		Protocol:    defaults.ProtocolPostgres,
		Database:    "mydb",
		Username:    "myUser",
		ServiceName: "postgres",
	}

	opts := []ConnectCommandFunc{
		WithLocalProxy("localhost", 12345, ""),
		WithNoTLS(),
	}

	c := NewCmdBuilder(tc, profile, database, "root", opts...)
	c.uid = utils.NewFakeUID()
	c.exe = &fakeExec{commandPathBehavior: forceBasePath}

	got, err := c.GetConnectCommandNoAbsPath()
	require.NoError(t, err)
	require.Equal(t, "psql postgres://myUser@localhost:12345/mydb", got.String())
}
