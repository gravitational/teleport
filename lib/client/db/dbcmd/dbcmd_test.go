/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package dbcmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// fakeExec implements execer interface for mocking purposes.
type fakeExec struct {
	// execOutput maps binary name and output that should be returned on RunCommand().
	// Map is also being used to check if a binary exist. Command line args are not supported.
	execOutput map[string][]byte
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

func TestCLICommandBuilderGetConnectCommand(t *testing.T) {
	conf := &client.Config{
		Host:         "localhost",
		WebProxyAddr: "proxy.example.com",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
		ClientStore:  client.NewMemClientStore(),
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Dir:      "/tmp",
		Cluster:  "example.com",
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
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql.crt&" +
					"sslkey=/tmp/keys/example.com/bob-db/db.example.com/mysql.key&sslmode=verify-full"},
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
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql.crt&" +
					"sslkey=/tmp/keys/example.com/bob-db/db.example.com/mysql.key&sslmode=verify-full\""},
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
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql.crt&" +
					"sslkey=/tmp/keys/example.com/bob-db/db.example.com/mysql.key&sslmode=verify-full"},
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
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql.crt&" +
					"sslkey=/tmp/keys/example.com/bob-db/db.example.com/mysql.key&sslmode=verify-full\""},
			wantErr: false,
		},
		{
			name:         "cockroach psql fallback",
			dbProtocol:   defaults.ProtocolCockroachDB,
			databaseName: "mydb",
			execer:       &fakeExec{},
			cmd: []string{"psql",
				"postgres://myUser@localhost:12345/mydb?sslrootcert=/tmp/keys/example.com/cas/root.pem&" +
					"sslcert=/tmp/keys/example.com/bob-db/db.example.com/mysql.crt&" +
					"sslkey=/tmp/keys/example.com/bob-db/db.example.com/mysql.key&sslmode=verify-full"},
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
				"--ssl-key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
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
				"--ssl-key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
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
			name:         "mariadb (remote proxy)",
			dbProtocol:   defaults.ProtocolMySQL,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithLocalProxy("", 0, "") /* negate default WithLocalProxy*/},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mariadb": []byte(""),
				},
			},
			cmd: []string{"mariadb",
				"--user", "myUser",
				"--database", "mydb",
				"--port", "3036",
				"--host", "proxy.example.com",
				"--ssl-key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key",
				"--ssl-ca", "/tmp/keys/example.com/cas/root.pem",
				"--ssl-cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
				"--ssl-verify-server-cert"},
			wantErr: false,
		},
		{
			name:         "mongodb (legacy)",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{withMongoDBAtlasDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongo": []byte("legacy"),
				},
			},
			cmd: []string{"mongo",
				"--ssl",
				"--sslPEMKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
				"mongodb://localhost:12345/mydb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
			wantErr: false,
		},
		{
			name:         "mongodb no TLS (legacy)",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS(), withMongoDBAtlasDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongo": []byte("legacy"),
				},
			},
			cmd: []string{"mongo",
				"mongodb://localhost:12345/mydb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
			wantErr: false,
		},
		{
			name:         "mongosh no CA",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{withMongoDBAtlasDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--tls",
				"--tlsCertificateKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
				"--tlsUseSystemCA",
				"mongodb://localhost:12345/mydb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
		},
		{
			name:         "mongosh",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts: []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, "/tmp/keys/example.com/cas/example.com.pem"),
				withMongoDBAtlasDatabase(),
			},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"--tls",
				"--tlsCertificateKeyFile", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
				"--tlsCAFile", "/tmp/keys/example.com/cas/example.com.pem",
				"mongodb://localhost:12345/mydb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
		},
		{
			name:         "mongosh no TLS",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS(), withMongoDBAtlasDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"mongodb://localhost:12345/mydb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
		},
		{
			name:         "mongosh preferred",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS(), withMongoDBAtlasDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{}, // Cannot find either bin.
			},
			cmd: []string{"mongosh",
				"mongodb://localhost:12345/mydb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
		},
		{
			name:         "DocumentDB",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "docdb",
			opts:         []ConnectCommandFunc{WithNoTLS(), withDocumentDBDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					// When both are available, legacy mongo is preferred.
					"mongo":   []byte("legacy"),
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongo",
				"mongodb://localhost:12345/docdb?directConnection=true&serverSelectionTimeoutMS=5000",
			},
			wantErr: false,
		},
		{
			name:         "DocumentDB mongosh",
			dbProtocol:   defaults.ProtocolMongoDB,
			databaseName: "docdb",
			opts:         []ConnectCommandFunc{WithNoTLS(), withDocumentDBDatabase()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mongosh": []byte("1.1.6"),
				},
			},
			cmd: []string{"mongosh",
				"mongodb://localhost:12345/docdb?directConnection=true&serverSelectionTimeoutMS=5000",
				"--retryWrites=false",
			},
			wantErr: false,
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
			name:         "sqlserver sqlcmd",
			dbProtocol:   defaults.ProtocolSQLServer,
			databaseName: "mydb",
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"sqlcmd": {},
				},
			},
			cmd: []string{sqlcmdBin,
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
				"--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt"},
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
				"--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
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
			name:       "redis-cli remote web proxy",
			dbProtocol: defaults.ProtocolRedis,
			opts:       []ConnectCommandFunc{WithLocalProxy("", 0, "") /* negate default WithLocalProxy*/},
			execer:     &fakeExec{},
			cmd: []string{"redis-cli",
				"-h", "proxy.example.com",
				"-p", "3080",
				"--tls",
				"--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key",
				"--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt",
				"--sni", "proxy.example.com"},
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
		{
			name:         "elasticsearch no TLS",
			dbProtocol:   defaults.ProtocolElasticsearch,
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			cmd:          []string{"elasticsearch-sql-cli", "http://localhost:12345/"},
			wantErr:      false,
		},
		{
			name:         "opensearchsql not found",
			dbProtocol:   defaults.ProtocolOpenSearch,
			opts:         []ConnectCommandFunc{},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			wantErr:      true,
		},
		{
			name:         "opensearchsql TLS, fail",
			dbProtocol:   defaults.ProtocolOpenSearch,
			opts:         []ConnectCommandFunc{},
			execer:       &fakeExec{execOutput: map[string][]byte{"opensearchsql": []byte("")}},
			databaseName: "warehouse1",
			wantErr:      true,
		},
		{
			name:         "opensearchsql no TLS, success",
			dbProtocol:   defaults.ProtocolOpenSearch,
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer:       &fakeExec{execOutput: map[string][]byte{"opensearchsql": []byte("")}},
			databaseName: "warehouse1",
			cmd:          []string{"opensearchsql", "http://localhost:12345"},
			wantErr:      false,
		},
		{
			name:         "cassandra",
			dbProtocol:   defaults.ProtocolCassandra,
			opts:         []ConnectCommandFunc{WithLocalProxy("localhost", 12345, "")},
			execer:       &fakeExec{},
			databaseName: "cassandra01",
			cmd:          []string{"cqlsh", "-u", "myUser", "localhost", "12345"},
			wantErr:      false,
		},
		{
			name:         "cassandra with password",
			dbProtocol:   defaults.ProtocolCassandra,
			opts:         []ConnectCommandFunc{WithLocalProxy("localhost", 12345, ""), WithPassword("password")},
			execer:       &fakeExec{},
			databaseName: "cassandra01",
			cmd:          []string{"cqlsh", "-u", "myUser", "localhost", "12345", "-p", "password"},
			wantErr:      false,
		},
		{
			name:         "elasticsearch with TLS, errors",
			dbProtocol:   defaults.ProtocolElasticsearch,
			opts:         []ConnectCommandFunc{},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			cmd:          nil,
			wantErr:      true,
		},
		// If you find yourself changing this test so that generating a command for DynamoDB _doesn't_
		// fail if WithPrintFormat() is not provided, please remember to update lib/teleterm/cmd/db.go.
		{
			name:         "dynamodb for exec is an error",
			dbProtocol:   defaults.ProtocolDynamoDB,
			opts:         []ConnectCommandFunc{WithNoTLS(), WithLocalProxy("localhost", 12345, "")},
			execer:       &fakeExec{},
			databaseName: "",
			cmd:          nil,
			wantErr:      true,
		},
		{
			name:         "dynamodb without proxy is an error",
			dbProtocol:   defaults.ProtocolDynamoDB,
			opts:         []ConnectCommandFunc{WithPrintFormat(), WithNoTLS(), WithLocalProxy("", 0, "")},
			execer:       &fakeExec{},
			databaseName: "",
			cmd:          nil,
			wantErr:      true,
		},
		{
			name:         "dynamodb with TLS proxy is an error",
			dbProtocol:   defaults.ProtocolDynamoDB,
			opts:         []ConnectCommandFunc{WithPrintFormat(), WithLocalProxy("localhost", 12345, "")},
			execer:       &fakeExec{},
			databaseName: "",
			cmd:          nil,
			wantErr:      true,
		},
		{
			name:         "dynamodb with print format and no-TLS proxy is ok",
			dbProtocol:   defaults.ProtocolDynamoDB,
			opts:         []ConnectCommandFunc{WithPrintFormat(), WithNoTLS(), WithLocalProxy("localhost", 12345, "")},
			execer:       &fakeExec{},
			databaseName: "",
			cmd:          []string{"aws", "--endpoint", "http://localhost:12345/", "[dynamodb|dynamodbstreams|dax]", "<command>"},
			wantErr:      false,
		},
		{
			name:         "oracle",
			dbProtocol:   defaults.ProtocolOracle,
			opts:         []ConnectCommandFunc{WithLocalProxy("localhost", 12345, "")},
			execer:       &fakeExec{},
			databaseName: "oracle01",
			cmd:          []string{"sql", "-L", "jdbc:oracle:thin:@tcps://localhost:12345/oracle01?TNS_ADMIN=/tmp/keys/example.com/bob-db/db.example.com/mysql-wallet"},
			wantErr:      false,
		},
		{
			name:         "Oracle with print format",
			dbProtocol:   defaults.ProtocolOracle,
			opts:         []ConnectCommandFunc{WithLocalProxy("localhost", 12345, ""), WithPrintFormat()},
			execer:       &fakeExec{},
			databaseName: "oracle01",
			cmd:          []string{"sql", "-L", "'jdbc:oracle:thin:@tcps://localhost:12345/oracle01?TNS_ADMIN=/tmp/keys/example.com/bob-db/db.example.com/mysql-wallet'"},
			wantErr:      false,
		},
		{
			name:       "Spanner for exec is ok",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{ProjectID: "foo-proj", InstanceID: "bar-instance"}),
			},
			execer:       &fakeExec{},
			databaseName: "googlesql-db",
			cmd:          []string{"spanner-cli", "-p", "foo-proj", "-i", "bar-instance", "-d", "googlesql-db"},
			wantErr:      false,
		},
		{
			name:       "Spanner with print format is ok",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithPrintFormat(),
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{ProjectID: "foo-proj", InstanceID: "bar-instance"}),
			},
			execer:       &fakeExec{},
			databaseName: "googlesql-db",
			cmd:          []string{"spanner-cli", "-p", "foo-proj", "-i", "bar-instance", "-d", "googlesql-db"},
			wantErr:      false,
		},
		{
			name:       "Spanner with print format and placeholders is ok",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithPrintFormat(),
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{}),
			},
			execer:       &fakeExec{},
			databaseName: "",
			cmd:          []string{"spanner-cli", "-p", "<project>", "-i", "<instance>", "-d", "<database>"},
			wantErr:      false,
		},
		{
			name:       "Spanner for exec without GCP project is an error",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{InstanceID: "bar-instance"}),
			},
			execer:       &fakeExec{},
			databaseName: "googlesql-db",
			wantErr:      true,
		},
		{
			name:       "Spanner for exec without GCP instance is an error",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{ProjectID: "foo-proj"}),
			},
			execer:       &fakeExec{},
			databaseName: "googlesql-db",
			wantErr:      true,
		},
		{
			name:       "Spanner for exec without database name is an error",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{ProjectID: "foo-proj"}),
			},
			execer:       &fakeExec{},
			databaseName: "googlesql-db",
			wantErr:      true,
		},
		{
			name:       "Spanner without a local proxy is an error",
			dbProtocol: defaults.ProtocolSpanner,
			opts: []ConnectCommandFunc{
				WithLocalProxy("", 0, ""),
				WithNoTLS(),
				WithGCP(types.GCPCloudSQL{ProjectID: "foo-proj", InstanceID: "bar-instance"}),
			},
			execer:       &fakeExec{},
			databaseName: "googlesql-db",
			wantErr:      true,
		},
		{
			name:       "Spanner with TLS local proxy is an error",
			dbProtocol: defaults.ProtocolSpanner,
			opts:       []ConnectCommandFunc{WithPrintFormat(), WithLocalProxy("localhost", 12345, "")},
			execer:     &fakeExec{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			database := tlsca.RouteToDatabase{
				Protocol:    tt.dbProtocol,
				Database:    tt.databaseName,
				Username:    "myUser",
				ServiceName: "mysql",
			}

			opts := append([]ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithExecer(tt.execer),
			}, tt.opts...)

			c := NewCmdBuilder(tc, profile, database, "root", opts...)
			c.uid = utils.NewFakeUID()
			got, err := c.GetConnectCommand(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.cmd, got.Args)
		})
	}
}

func TestCLICommandBuilderGetConnectCommandAlternatives(t *testing.T) {
	conf := &client.Config{
		Host:         "localhost",
		WebProxyAddr: "proxy.example.com",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
		ClientStore:  client.NewMemClientStore(),
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
		cmd          map[string][]string
		wantErr      bool
	}{
		{
			name:         "postgres no TLS",
			dbProtocol:   defaults.ProtocolPostgres,
			databaseName: "mydb",
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer:       &fakeExec{},
			cmd:          map[string][]string{"default command": {"psql", "postgres://myUser@localhost:12345/mydb"}},
			wantErr:      false,
		},
		{
			name:         "elasticsearch with TLS",
			dbProtocol:   defaults.ProtocolElasticsearch,
			opts:         []ConnectCommandFunc{},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			cmd:          map[string][]string{"run single request with curl": {"curl", "https://localhost:12345/", "--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key", "--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt"}},
			wantErr:      false,
		},
		{
			name:       "elasticsearch with TLS and SQL",
			dbProtocol: defaults.ProtocolElasticsearch,
			opts:       []ConnectCommandFunc{},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"elasticsearch-sql-cli": {},
				},
			},
			databaseName: "warehouse1",
			cmd: map[string][]string{
				"run single request with curl": {"curl", "https://localhost:12345/", "--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key", "--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt"}},
			wantErr: false,
		},
		{
			name:       "elasticsearch with no TLS, with SQL",
			dbProtocol: defaults.ProtocolElasticsearch,
			opts:       []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"python":                {},
					"elasticsearch-sql-cli": {},
				},
			},
			databaseName: "warehouse1",
			cmd: map[string][]string{
				"interactive SQL connection":   {"elasticsearch-sql-cli", "http://localhost:12345/"},
				"run single request with curl": {"curl", "http://localhost:12345/"},
			},
			wantErr: false,
		},
		{
			name:         "opensearch, TLS, no binaries",
			dbProtocol:   defaults.ProtocolOpenSearch,
			opts:         []ConnectCommandFunc{},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			cmd: map[string][]string{
				"run request with curl": {"curl", "https://localhost:12345/", "--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key", "--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt"}},
			wantErr: false,
		},
		{
			name:         "opensearch, no TLS, no binaries",
			dbProtocol:   defaults.ProtocolOpenSearch,
			opts:         []ConnectCommandFunc{WithNoTLS()},
			execer:       &fakeExec{},
			databaseName: "warehouse1",
			cmd:          map[string][]string{"run request with curl": {"curl", "http://localhost:12345/"}},
			wantErr:      false,
		},
		{
			name:       "opensearch, TLS, all binaries",
			dbProtocol: defaults.ProtocolOpenSearch,
			opts:       []ConnectCommandFunc{},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"opensearch-cli": {},
					"opensearchsql":  {},
				},
			},
			databaseName: "warehouse1",
			cmd: map[string][]string{
				"run request with curl":           {"curl", "https://localhost:12345/", "--key", "/tmp/keys/example.com/bob-db/db.example.com/mysql.key", "--cert", "/tmp/keys/example.com/bob-db/db.example.com/mysql.crt"},
				"run request with opensearch-cli": {"opensearch-cli", "--profile", "teleport", "--config", "/tmp/mysql/opensearch-cli/7e266ec0.yml", "curl", "get", "--path", "/"}},

			wantErr: false,
		},
		{
			name:       "opensearch, no TLS, all binaries",
			dbProtocol: defaults.ProtocolOpenSearch,
			opts:       []ConnectCommandFunc{WithNoTLS()},
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"opensearch-cli": {},
					"opensearchsql":  {},
				},
			},
			databaseName: "warehouse1",
			cmd: map[string][]string{
				"run request with curl":                        {"curl", "http://localhost:12345/"},
				"run request with opensearch-cli":              {"opensearch-cli", "--profile", "teleport", "--config", "/tmp/mysql/opensearch-cli/e397f38c.yml", "curl", "get", "--path", "/"},
				"start interactive session with opensearchsql": {"opensearchsql", "http://localhost:12345"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			database := tlsca.RouteToDatabase{
				Protocol:    tt.dbProtocol,
				Database:    tt.databaseName,
				Username:    "myUser",
				ServiceName: "mysql",
			}

			opts := append([]ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithExecer(tt.execer),
			}, tt.opts...)

			c := NewCmdBuilder(tc, profile, database, "root", opts...)
			c.uid = utils.NewFakeUID()

			commandOptions, err := c.GetConnectCommandAlternatives(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			commands := map[string][]string{}
			for _, copt := range commandOptions {
				commands[copt.Description] = copt.Command.Args
			}

			require.Equal(t, tt.cmd, commands)
		})
	}
}

func TestConvertCommandError(t *testing.T) {
	t.Parallel()
	conf := &client.Config{
		Host:         "localhost",
		WebProxyAddr: "localhost",
		SiteName:     "db.example.com",
		Tracer:       tracing.NoopProvider().Tracer("test"),
		ClientStore:  client.NewMemClientStore(),
	}

	tc, err := client.NewClient(conf)
	require.NoError(t, err)

	profile := &client.ProfileStatus{
		Name:     "example.com",
		Username: "bob",
		Cluster:  "example.com",
	}

	tests := []struct {
		desc       string
		dbProtocol string
		execer     *fakeExec
		stderr     []byte
		wantBin    string
		wantStdErr string
	}{
		{
			desc:       "converts access denied to helpful message",
			dbProtocol: defaults.ProtocolMySQL,
			execer: &fakeExec{
				execOutput: map[string][]byte{
					"mysql": []byte("Ver 8.0.27-0ubuntu0.20.04.1 for Linux on x86_64 ((Ubuntu))"),
				},
			},
			stderr:     []byte("access to db denied"),
			wantBin:    mysqlBin,
			wantStdErr: "see your available logins, or ask your Teleport administrator",
		},
		{
			desc:       "converts unrecognized redis error to helpful message",
			dbProtocol: defaults.ProtocolRedis,
			execer:     &fakeExec{},
			stderr:     []byte("Unrecognized option or bad number of args for"),
			wantBin:    redisBin,
			wantStdErr: "Please make sure 'redis-cli' with version 6.2 or newer is installed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			database := tlsca.RouteToDatabase{
				Protocol:    tt.dbProtocol,
				Database:    "DBName",
				Username:    "myUser",
				ServiceName: "DBService",
			}

			opts := []ConnectCommandFunc{
				WithLocalProxy("localhost", 12345, ""),
				WithNoTLS(),
				WithExecer(tt.execer),
			}
			c := NewCmdBuilder(tc, profile, database, "root", opts...)
			c.uid = utils.NewFakeUID()

			cmd, err := c.GetConnectCommand(context.Background())
			require.NoError(t, err)

			// make sure the expected test bin is the command bin we got
			require.True(t, strings.HasSuffix(cmd.Path, tt.wantBin))

			peakStderr := utils.NewCaptureNBytesWriter(PeakStderrSize)
			_, peakErr := peakStderr.Write(tt.stderr)
			require.NoError(t, peakErr, "CaptureNBytesWriter should never return error")

			convertedErr := ConvertCommandError(cmd, nil, string(peakStderr.Bytes()))
			require.ErrorContains(t, convertedErr, tt.wantStdErr)
		})
	}
}

func withMongoDBAtlasDatabase() ConnectCommandFunc {
	return WithGetDatabaseFunc(func(context.Context, *client.TeleportClient, string) (types.Database, error) {
		db, err := types.NewDatabaseV3(
			types.Metadata{
				Name: "mongodb-atlas",
			},
			types.DatabaseSpecV3{
				Protocol: types.DatabaseProtocolMongoDB,
				URI:      "mongodb+srv://my-cluster.abcdefy.mongodb.net",
			},
		)
		return db, trace.Wrap(err)
	})
}

func withDocumentDBDatabase() ConnectCommandFunc {
	return WithGetDatabaseFunc(func(context.Context, *client.TeleportClient, string) (types.Database, error) {
		db, err := types.NewDatabaseV3(
			types.Metadata{
				Name: "docdb",
			},
			types.DatabaseSpecV3{
				Protocol: types.DatabaseProtocolMongoDB,
				URI:      "my-documentdb-cluster-id.cluster-abcdefghijklmnop.us-east-1.docdb.amazonaws.com:27017",
			},
		)
		return db, trace.Wrap(err)
	})
}
