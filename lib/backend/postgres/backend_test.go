/*
Copyright 2018-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgres

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/sqlbk"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
)

var (
	// DatabaseURL is the connection string the SQL backend uses to connect to a test
	// database instance. The database must not already exist (it is created by the
	// test suite). Set the URL using the environment variable
	// TELEPORT_TEST_BACKEND_POSTGRES_URL.
	//
	//    # Use in-memory CockroachDB server (if cockroach is on PATH).
	//    $ go test -v
	//
	//    # Connect to local PostgreSQL socket:
	//    $ TELEPORT_TEST_BACKEND_POSTGRES_URL='postgres:///teleport?sslmode=disable' \
	//      go test -v
	//
	//    # Connect to PostgreSQL server with mTLS:
	//    $ postgres://postgres.example.com:5432/teleport?sslmode=verify-full&sslrootcert=postgres.cas&sslcert=client.crt&sslkey=client.key \
	//      go test -v
	//
	DatabaseURL *url.URL

	// NoDrop prevents the test teleport database from being dropped at the end
	// of the test. This setting has no impact for the in-memory database test.
	// Enable the setting using the environment variable
	// TELEPORT_TEST_BACKEND_POSTGRES_NODROP=y.
	NoDrop bool

	// LogSQL logs all SQL statements executed during the test. Logging SQL
	// statements may require setting the logrus standard logger's log level.
	// Enable the setting using the environment variable
	// TELEPORT_TEST_BACKEND_POSTGRES_LOGSQL=y.
	LogSQL bool
)

const (
	// envDatabaseURL is the environment variable that sets DatabaseURL.
	envDatabaseURL = "TELEPORT_TEST_BACKEND_POSTGRES_URL"

	// envDatabaseURL is the environment variable that sets NoDrop.
	envNoDrop = "TELEPORT_TEST_BACKEND_POSTGRES_NODROP"

	// envDatabaseURL is the environment variable that sets LogSQL.
	envLogSQL = "TELEPORT_TEST_BACKEND_POSTGRES_LOGSQL"
)

// TestMain attempts to start a CockroachDB server if it is available
// and no DatabaseURL has been set by an environment variable.
func TestMain(m *testing.M) {
	initTestConfig()
	stopServerFn := maybeStartRoachServer()
	code := m.Run()
	stopServerFn()
	os.Exit(code)
}

// TestBackend runs the backend test suite for the postgres driver.
func TestBackend(t *testing.T) {
	if DatabaseURL == nil {
		t.Skip("Postgres backend test suite is disabled. Set TELEPORT_TEST_BACKEND_POSTGRES_URL to enable or ensure the CockroachDB binary is on PATH.")
	} else {
		t.Logf("NoDrop=%t LogSQL=%t URL=%q", NoDrop, LogSQL, DatabaseURL)
	}

	cfg := &Config{}
	cfg.Log = logrus.WithFields(logrus.Fields{trace.Component: BackendName})
	cfg.Addr = "-"
	cfg.TLS.CAFile = "-"
	cfg.TLS.ClientKeyFile = "-"
	cfg.TLS.ClientCertFile = "-"
	require.NoError(t, cfg.CheckAndSetDefaults())

	sqlbk.TestDriver(t, &testDriver{
		t:        t,
		pgDriver: pgDriver{cfg: cfg},
	})
}

// TestConfig verifies the storage section of the YAML configuration file
// supports nested sections.
func TestConfig(t *testing.T) {
	const tmpl = `---
storage:
  type: postgres
  addr: %q
  database: %q
  tls:
    ca_file: %q
    client_cert_file: %q
    client_key_file: %q`

	expect := &Config{}
	expect.Addr = "postgres.example.com:5432"
	expect.Database = "teleport"
	expect.TLS.CAFile = "postgres.cas"
	expect.TLS.ClientCertFile = "root.crt"
	expect.TLS.ClientKeyFile = "root.key"

	source := fmt.Sprintf(tmpl,
		expect.Addr,
		expect.Database,
		expect.TLS.CAFile,
		expect.TLS.ClientCertFile,
		expect.TLS.ClientKeyFile)

	var doc struct {
		Storage struct {
			Params backend.Params `yaml:",inline"`
		} `yaml:"storage"`
	}
	err := yaml.UnmarshalStrict([]byte(source), &doc)
	require.NoError(t, err)

	doc.Storage.Params.Cleanse()

	var cfg *Config
	err = utils.ObjectToStruct(doc.Storage.Params, &cfg)
	require.NoError(t, err)
	require.Equal(t, expect, cfg)
}

// TestDriverURL verifies the correct connection string URL
// is created from a Config.
func TestDriverURL(t *testing.T) {
	driver := pgDriver{cfg: &Config{}}
	driver.cfg.Addr = "host:123"
	driver.cfg.Database = "database"
	driver.cfg.TLS.CAFile = "cafile"
	driver.cfg.TLS.ClientCertFile = "certfile"
	driver.cfg.TLS.ClientKeyFile = "keyfile"

	expect, err := url.Parse("postgres://host:123/database?sslmode=verify-full&sslrootcert=cafile&sslcert=certfile&sslkey=keyfile")
	require.NoError(t, err)
	expectQuery := expect.Query()
	expect.RawQuery = ""

	got := driver.url()
	gotQuery := got.Query()
	got.RawQuery = ""

	require.Equal(t, expect, got)
	require.Equal(t, expectQuery, gotQuery)
}

func TestValidateDatabaseName(t *testing.T) {
	testCases := []struct {
		valid bool
		name  string
	}{
		{valid: true, name: "a"},
		{valid: true, name: "A"},
		{valid: true, name: "_"},
		{valid: true, name: "aa"},
		{valid: true, name: "aA"},
		{valid: true, name: "a_"},
		{valid: true, name: "a$"},
		{valid: false, name: "0"},
		{valid: false, name: "0a"},
		{valid: false, name: "$a"},
		{valid: false, name: "a*"},
		{valid: false, name: "a%"},
		{valid: false, name: "a;"},
		{valid: false, name: "; drop database postgres;"},
		{valid: false, name: "This_table_name_is_one_more_byte_than_the_63_byte_maximum_limit"},
		{valid: true, name: "This_table_name_is_exactly_the_63_byte_maximum_limit__________"},
	}
	for i, test := range testCases {
		err := validateDatabaseName(test.name)
		require.True(t, test.valid == (err == nil), "Test case %d: %q", i, test.name)
	}
}

// testDriver wraps pgDriver with a new Open method that creates a test database
// and applies test configurations.
type testDriver struct {
	pgDriver
	t *testing.T
}

// Open the test database.
func (d *testDriver) Open(ctx context.Context) (sqlbk.DB, error) {
	t := d.t

	// Verify test URL.
	require.NotNil(t, DatabaseURL)
	require.Greaterf(t, len(DatabaseURL.Path), 1, DatabaseURL.Path)
	require.Equal(t, byte('/'), DatabaseURL.Path[0])
	dbName := DatabaseURL.Path[1:]

	// Connect to the postgres database to create the test database. Create a
	// connection string for the postgres database by copying DatabaseURL and
	// changing the path (database). Leave the connection open to delete the
	// test database after the test suite completes.
	pgURL := *DatabaseURL
	pgURL.Path = "/postgres"
	pgConn, err := pgx.Connect(ctx, pgURL.String())
	require.NoError(t, err)
	t.Cleanup(func() { pgConn.Close(ctx) })

	// Make sure the test database does not alread exist.
	dbExists, err := databaseExists(ctx, pgConn, dbName)
	require.NoError(t, err)
	require.False(t, dbExists, "Database %v already exists. Tests will not use an existing database.", dbName)

	if LogSQL {
		d.sqlLogger = maybeSQLLogger(t)
	}
	if !NoDrop {
		t.Cleanup(func() {
			_, err := pgConn.Exec(ctx, fmt.Sprintf("DROP DATABASE %v", dbName))
			require.NoError(t, err, "Failed to drop %v database", dbName)
		})
	}

	return d.open(ctx, DatabaseURL)
}

// maybeSQLLogger returns a new logger when log levels are supported (logrus
// and pgx have different log levels).
func maybeSQLLogger(t *testing.T) pgx.Logger {
	level := logrus.GetLevel()
	if level >= logrus.DebugLevel {
		return &pgxLogger{level: logrus.DebugLevel}
	} else if level == logrus.InfoLevel {
		return &pgxLogger{level: logrus.InfoLevel}
	}
	t.Logf("SQL logging is disabled. Logging level must be greater than 'info' but is set to %q", level)
	return nil
}

// maybeStartRoachServer will attempt to search a single-node CockroachDB
// server if the test URL is empty. The returned function should be called after
// all tests have executed to stop the server.
func maybeStartRoachServer() (stopServerFn func()) {
	if DatabaseURL != nil {
		return func() {}
	}

	// Don't start server unless executing TestBackend.
	// Or, don't start when -bench flag exists or -run != TestBackend.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.bench=") {
			return func() {}
		}
		if strings.HasPrefix(arg, "-test.run=") {
			if strings.HasSuffix(arg, "=TestBackend") {
				break
			}
			return func() {}
		}
	}

	var roach roachServer
	err := roach.Start()
	if err != nil {
		if !trace.IsNotFound(err) {
			logrus.Warnf("Failed to start cockroach test server: %v", err)
		}
		return func() {}
	}
	return func() { <-roach.Stop() }
}

// roachServer wraps a CockroachDB subprocess.
type roachServer struct {
	Stop func() <-chan struct{}
}

// Start a CockroachDB single-node server for testing. It returns a NotFound
// error if the cockroach executable is not in PATH. Stop field is set when
// a non-nil error is returned.
func (r *roachServer) Start() error {
	cockroachPath, err := exec.LookPath("cockroach")
	if err != nil {
		return trace.NotFound("cockroach executable not found")
	}

	// Create io.Writer that will read log messages from the cockroach
	// server to determine when it is ready to accept connections and
	// extract the client connection string (we need the port).
	started := false
	startErr := make(chan error)
	writer := &peekWriter{
		Writer: io.Discard, // Change to os.Stdout to see log messages.
		Peek: func(b []byte) {
			if started {
				return
			}
			// I220310 16:23:02.762587 11 1@cli/start.go:759  [-] 83  node startup completed:
			// ...
			// I220310 16:23:02.762587 11 1@cli/start.go:759  [-] 83 +sql: postgresql://root@name.local:26257/defaultdb?sslmode=disable
			if !bytes.Contains(b, []byte("node startup completed:")) {
				return
			}
			const left = " +sql: "
			const right = "sslmode=disable"
			i := bytes.Index(b, []byte(left))
			if i == -1 {
				return
			}
			j := bytes.Index(b[i:], []byte(right))
			if j == -1 {
				return
			}
			connStr := string(bytes.TrimSpace(b[i+len(left) : i+j+len(right)]))
			u, err := url.Parse(connStr)
			if err != nil {
				err = trace.BadParameter("failed to parse client connection string for CockroachDB %q: %v", connStr, err)
			}
			DatabaseURL = u
			DatabaseURL.Path = "/teleport"
			startErr <- err
			started = true
		},
	}

	logrus.Info("Starting CockroachDB in-memory server")
	cmd := exec.Command(
		cockroachPath,
		"start-single-node",
		"--insecure",
		"--store=type=mem,size=1G", // Size must be greater than 640 MiB
		"--listen-addr=localhost:0")
	cmd.Stderr = writer
	cmd.Stdout = io.Discard
	err = cmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	shutdownComplete := make(chan struct{})
	r.Stop = func() <-chan struct{} {
		cmd.Process.Signal(os.Interrupt)
		return shutdownComplete
	}

	go func() {
		cmd.Wait()
		r.cleanup()
		close(shutdownComplete)
	}()

	// Wait for cockroach server to be ready for connections.
	select {
	case err = <-startErr:
	case <-time.After(time.Second * 5):
		return trace.LimitExceeded("Timeout waiting for the CockroachDB server to accept connections.")
	}
	return err
}

// cleanup removes empty directories cockroach leaves behind.
func (r *roachServer) cleanup() {
	wd, err := os.Getwd()
	if err != nil {
		logrus.Error(err)
	}
	for _, dir := range []string{"goroutine_dump", "inflight_trace_dump", "heap_profiler"} {
		err = os.RemoveAll(path.Join(wd, dir))
		if err != nil {
			logrus.Error(err)
		}
	}
}

// peekWriter wraps an io.Writer and calls peek on each write.
type peekWriter struct {
	Writer io.Writer
	Peek   func([]byte)
}

// Write implements io.Writer.
func (s *peekWriter) Write(b []byte) (n int, err error) {
	s.Peek(b)
	return s.Writer.Write(b)
}

// initTestConfig sets configuration variables based on environment variable
// settings.
func initTestConfig() {
	NoDrop = os.Getenv(envNoDrop) == "y"
	LogSQL = os.Getenv(envLogSQL) == "y"

	// init DatabaseURL
	if envURL := os.Getenv(envDatabaseURL); envURL != "" {
		u, err := url.Parse(envURL)
		if err != nil {
			logrus.Errorf("Failed to parse %v=%q: %v", envDatabaseURL, envURL, err)
		}
		DatabaseURL = u
	}
}

var (
	_ sqlbk.Driver = (*pgDriver)(nil)
	_ sqlbk.DB     = (*pgDB)(nil)
	_ sqlbk.Tx     = (*pgTx)(nil)
)
