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

package postgres

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	cloudawsconfig "github.com/gravitational/teleport/lib/cloud/aws/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

var (
	//go:embed sql/reassign-objects.sql
	reassignObjectsProcedure string

	postgresImages []string = []string{
		"postgres:14",
		"postgres:15",
		"postgres:16",
		"postgres:17",
		"postgres:18",
	}
)

// noopAuth implements common.Auth for testing. GetTLSConfig returns the
// embedded tlsConfig — nil for testcontainers (plain TCP), or a config
// pointing at the AWS RDS root CAs for RDS. Only the methods called during
// ActivateUser/DeactivateUser/DeleteUser are implemented; any other call
// will panic. This is a benefit, since a future panic signals a new code
// path that needs a real implementation.
type noopAuth struct {
	common.Auth
	tlsConfig *tls.Config
}

func (a *noopAuth) GetTLSConfig(_ context.Context, _ time.Time, _ types.Database, _ string) (*tls.Config, error) {
	return a.tlsConfig, nil
}
func (a *noopAuth) WithLogger(_ func(*slog.Logger) *slog.Logger) common.Auth { return a }
func (a *noopAuth) WithSession(_ *common.Session) common.Auth                { return a }

// noopChecker implements services.AccessChecker for testing. It returns empty
// database permissions, which causes applyPermissions to exit early.
type noopChecker struct {
	services.AccessChecker
}

func (c *noopChecker) GetDatabasePermissions(_ types.Database) (types.DatabasePermissions, types.DatabasePermissions, error) {
	return nil, nil, nil
}

// noopAudit implements common.Audit for testing. Only the methods called
// during ActivateUser/DeactivateUser/DeleteUser are implemented; any other
// call will panic.
type noopAudit struct {
	common.Audit
}

func (a *noopAudit) OnDatabaseUserCreate(_ context.Context, _ *common.Session, _ error) {}
func (a *noopAudit) OnDatabaseUserDeactivate(_ context.Context, _ *common.Session, _ bool, _ error) {
}

// makeSession returns a Session for the given database, username, and roles.
// A nil roles slice is normalised to an empty slice to avoid passing SQL NULL
// to the activate procedure's varchar[] parameter.
func makeSession(db types.Database, username string, roles []string) *common.Session {
	if roles == nil {
		roles = []string{}
	}
	return &common.Session{
		Database:      db,
		DatabaseUser:  username,
		DatabaseName:  "postgres",
		DatabaseRoles: roles,
		Checker:       &noopChecker{},
	}
}

// isMember reports whether username is a member of role in the database
// reached via conn.
func isMember(t *testing.T, conn *pgx.Conn, username, role string) bool {
	t.Helper()
	var exists bool
	err := conn.QueryRow(t.Context(),
		`SELECT true FROM pg_auth_members m
		 JOIN pg_roles member_role ON m.member = member_role.oid
		 JOIN pg_roles grant_role  ON m.roleid = grant_role.oid
		 WHERE member_role.rolname = $1 AND grant_role.rolname = $2`,
		username, role).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false
	}
	require.NoError(t, err)
	return exists
}

// canLogin reports whether the named role has the LOGIN attribute.
func canLogin(t *testing.T, conn *pgx.Conn, username string) bool {
	t.Helper()
	var login bool
	err := conn.QueryRow(t.Context(),
		"SELECT rolcanlogin FROM pg_roles WHERE rolname = $1", username).Scan(&login)
	require.NoError(t, err)
	return login
}

// userExists reports whether a role with the given name exists in pg_roles.
func userExists(t *testing.T, conn *pgx.Conn, username string) bool {
	t.Helper()
	var count int
	err := conn.QueryRow(t.Context(),
		"SELECT COUNT(*) FROM pg_roles WHERE rolname = $1", username).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

// makeReassignSession returns a Session like makeSession but with
// AutoCreateUserMode set to BEST_EFFORT_REASSIGN_AND_DROP so DeleteUser
// runs the per-object reassignment path.
func makeReassignSession(db types.Database, username string, roles []string) *common.Session {
	s := makeSession(db, username, roles)
	s.AutoCreateUserMode = types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP
	return s
}

// tableOwner returns the role that owns schema.name.
func tableOwner(t *testing.T, conn *pgx.Conn, schema, name string) string {
	t.Helper()
	var owner string
	err := conn.QueryRow(t.Context(), `
		SELECT pg_get_userbyid(c.relowner)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2`,
		schema, name).Scan(&owner)
	require.NoError(t, err)
	return owner
}

// hasExecuteOnReassignProc reports whether role has EXECUTE on the
// teleport_reassign_objects procedure, as seen through conn.
func hasExecuteOnReassignProc(t *testing.T, conn *pgx.Conn, role string) bool {
	t.Helper()
	var ok bool
	err := conn.QueryRow(t.Context(),
		`SELECT has_function_privilege($1, 'teleport_objects.teleport_reassign_objects(varchar, varchar)', 'EXECUTE')`,
		role).Scan(&ok)
	require.NoError(t, err)
	return ok
}

// hasSchemaPrivilege reports whether role has the given privilege ("USAGE" or
// "CREATE") on the teleport_objects schema, as seen through conn.
func hasSchemaPrivilege(t *testing.T, conn *pgx.Conn, role, privilege string) bool {
	t.Helper()
	var ok bool
	err := conn.QueryRow(t.Context(),
		`SELECT has_schema_privilege($1, 'teleport_objects', $2)`,
		role, privilege).Scan(&ok)
	require.NoError(t, err)
	return ok
}

// sourceStillOwnsAnything reports whether username has any owner-deptype
// rows in pg_shdepend for the current database via conn. After a refused
// reassignment, the savepoint rollback in delete-user.sql should leave
// at least one such row.
//
// Reads pg_roles (the public view over pg_authid; pg_authid itself is
// restricted to superusers, so teleport-admin can't read it).
func sourceStillOwnsAnything(t *testing.T, conn *pgx.Conn, username string) bool {
	t.Helper()
	var n int
	err := conn.QueryRow(t.Context(), `
		SELECT COUNT(*) FROM pg_shdepend sd
		JOIN pg_roles r ON r.oid = sd.refobjid
		WHERE r.rolname = $1
		AND sd.deptype = 'o'
		AND sd.dbid = (SELECT oid FROM pg_database WHERE datname = current_database())`,
		username).Scan(&n)
	require.NoError(t, err)
	return n > 0
}

// testEnv is the shared fixture returned by setupTestEnv: the test
// container or RDS instance, the database resource, the engine, and the
// four pgx connections used throughout (two as the bootstrap user for
// bootstrap-only operations, two as teleport-admin for everything under
// test). All connection cleanup is registered on t.Cleanup, so callers
// receive a ready-to-use env and never need to close anything explicitly.
type testEnv struct {
	db                   types.Database
	engine               *Engine
	bootstrapConn        *pgx.Conn // bootstrap user, postgres db
	bootstrapOtherDBConn *pgx.Conn // bootstrap user, other_db
	adminConn            *pgx.Conn // teleport-admin, postgres db
	otherDBConn          *pgx.Conn // teleport-admin, other_db

	// connectAsUser opens a fresh pgx connection authenticating with the
	// given username and password against the given database. The caller is
	// responsible for closing the returned connection (typically via defer).
	// The closure hides the difference between the testcontainers
	// (localhost, plain TCP) and RDS (endpoint, TLS) connection paths.
	connectAsUser func(t *testing.T, username, password, dbName string) (*pgx.Conn, error)

	// connectAsBootstrap opens a fresh pgx connection as the bootstrap user
	// (postgres superuser for testcontainers, master for RDS). Used by tests
	// that need a session distinct from bootstrapConn — e.g. to hold a row
	// lock across other operations driven by bootstrapConn.
	connectAsBootstrap func(t *testing.T, dbName string) (*pgx.Conn, error)
}

// setupSelfHostedTestEnv spins up the postgres testcontainer, provisions the
// docs-prescribed least-privilege teleport-admin user and the Teleport
// roles/grants the tests rely on, and returns the assembled fixture.
// The container, the second database, and every connection are cleaned
// up via t.Cleanup.
func setupSelfHostedTestEnv(t *testing.T, postgresImage string) *testEnv {
	pgContainer, err := postgres.Run(t.Context(), postgresImage,
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithDatabase("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, pgContainer.Terminate(context.Background()))
	})

	mappedPort, err := pgContainer.MappedPort(t.Context(), "5432/tcp")
	require.NoError(t, err)
	port := mappedPort.Port()

	connectAsUser := func(t *testing.T, user, password, dbName string) (*pgx.Conn, error) {
		return pgx.Connect(t.Context(),
			fmt.Sprintf("postgres://%s:%s@localhost:%s/%s", user, password, port, dbName))
	}
	connectAsBootstrap := func(t *testing.T, dbName string) (*pgx.Conn, error) {
		return connectAsUser(t, "postgres", "postgres", dbName)
	}

	// The engine connects as the docs-prescribed least-privilege admin
	// user "teleport-admin", with CREATEROLE only — not SUPERUSER. The
	// URI includes credentials because the connector prepends
	// "postgres://" before parsing.
	dbURI := fmt.Sprintf("teleport-admin:admin@localhost:%s", port)

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-pg",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      dbURI,
		AdminUser: &types.DatabaseAdminUser{
			Name: "teleport-admin",
		},
	})
	require.NoError(t, err)

	engine := &Engine{
		EngineConfig: common.EngineConfig{
			Log:   slog.Default(),
			Auth:  &noopAuth{},
			Audit: &noopAudit{},
		},
	}

	// bootstrapConn is the postgres superuser, kept alive for the test's
	// duration. It is used only for:
	//   (1) one-time setup of the teleport-admin user and the roles and
	//       grants Teleport's docs prescribe; and
	//   (2) per-test fixture operations that teleport-admin cannot do
	//       itself. In PG16+, the implicit grant from CREATEROLE-creating-
	//       a-role gives the creator ADMIN OPTION only — not SET ROLE.
	//       That means teleport-admin cannot ALTER … OWNER TO an
	//       auto-provisioned user even though it created the user.
	// Nothing under test runs through bootstrapConn; the engine, every
	// assertion query, and adminConn-driven DDL all use teleport-admin.
	//
	// TODO: update this comment; it's now inaccurate. And we want the postgres
	// connection to be used for many of the above things anyways, not
	// teleport-admin.
	bootstrapConn, err := connectAsBootstrap(t, "postgres")
	require.NoError(t, err)
	t.Cleanup(func() { bootstrapConn.Close(context.Background()) })

	_, err = bootstrapConn.Exec(t.Context(), `
		-- Docs-prescribed admin: CREATEROLE + LOGIN. CREATEDB lets the
		-- test create the second database.
		CREATE USER "teleport-admin" LOGIN CREATEROLE PASSWORD 'admin';

		-- postgres_fdw is bundled; install as superuser and grant USAGE so
		-- the foreign-table refused-case test can CREATE SERVER as
		-- teleport-admin.
		CREATE EXTENSION postgres_fdw;
		GRANT USAGE ON FOREIGN DATA WRAPPER postgres_fdw TO "teleport-admin";
	`)
	require.NoError(t, err)

	// CREATE DATABASE cannot be batched with other statements (it cannot
	// run inside a transaction block), so it has its own Exec call.
	_, err = bootstrapConn.Exec(t.Context(), `CREATE DATABASE other_db`)
	require.NoError(t, err)

	// Database and schema grants do not propagate across databases, so
	// re-grant teleport-admin inside other_db using a superuser connection
	// there. teleport-object-inheritor needs no other-db grants because
	// the dbid filter in reassign-objects.sql confines reassignment to the
	// current database.
	bootstrapOtherDBConn, err := connectAsBootstrap(t, "other_db")
	require.NoError(t, err)
	t.Cleanup(func() { bootstrapOtherDBConn.Close(context.Background()) })
	_, err = bootstrapOtherDBConn.Exec(t.Context(), `
	       GRANT CREATE, CONNECT ON DATABASE other_db TO "teleport-admin";
	       GRANT USAGE,  CREATE  ON SCHEMA   public   TO "teleport-admin";
	`)
	require.NoError(t, err)

	// adminConn drives every operation under test from teleport-admin's
	// vantage point: assertions, queries, and (via the dbURI above) the
	// engine itself.
	adminConn, err := connectAsUser(t, "teleport-admin", "admin", "postgres")
	require.NoError(t, err)
	t.Cleanup(func() { adminConn.Close(context.Background()) })

	// otherDBConn is the teleport-admin peer for other_db, used by the
	// cross-db reassignment tests' assertions and fixtures that
	// teleport-admin can do itself.
	otherDBConn, err := connectAsUser(t, "teleport-admin", "admin", "other_db")
	require.NoError(t, err)
	t.Cleanup(func() { otherDBConn.Close(context.Background()) })

	return &testEnv{
		db:                   db,
		engine:               engine,
		bootstrapConn:        bootstrapConn,
		bootstrapOtherDBConn: bootstrapOtherDBConn,
		adminConn:            adminConn,
		otherDBConn:          otherDBConn,
		connectAsUser:        connectAsUser,
		connectAsBootstrap:   connectAsBootstrap,
	}
}

// rdsTestEnv constants and env var names. The env var names match those
// used by the e2e/aws test suite so a single AWS_REGION + RDS_POSTGRES_
// INSTANCE_NAME configuration drives both.
const (
	awsRegionEnv               = "AWS_REGION"
	rdsPostgresInstanceNameEnv = "RDS_POSTGRES_INSTANCE_NAME"
	// rdsAdminUser is the name of the least-privilege admin user this test
	// provisions inside the RDS instance. Same identifier as the
	// testcontainers path so the procedures and grants the engine creates
	// look identical.
	rdsAdminUser = "teleport-admin"
	// rdsAdminPassword is the password for rdsAdminUser. Hardcoded because
	// the RDS instance is expected to be a dedicated, isolated test
	// resource — the credential never leaves the test process.
	rdsAdminPassword = "teleport-admin-password-not-a-secret"
)

// awsRDSCertBundleURL is the URL serving the AWS RDS global CA bundle. The
// bundle is required to validate TLS connections to RDS instances.
const awsRDSCertBundleURL = "https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem"

var (
	rdsCertPoolOnce sync.Once
	rdsCertPool     *x509.CertPool
	rdsCertPoolErr  error
)

// getRDSCertPool returns an x509 cert pool containing the AWS RDS global CA
// bundle. The bundle is downloaded once per process and cached.
func getRDSCertPool(t *testing.T) *x509.CertPool {
	t.Helper()
	rdsCertPoolOnce.Do(func() {
		resp, err := http.Get(awsRDSCertBundleURL)
		if err != nil {
			rdsCertPoolErr = trace.Wrap(err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			rdsCertPoolErr = trace.Errorf("got status %d fetching %s", resp.StatusCode, awsRDSCertBundleURL)
			return
		}
		bundle, err := io.ReadAll(resp.Body)
		if err != nil {
			rdsCertPoolErr = trace.Wrap(err)
			return
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(bundle) {
			rdsCertPoolErr = trace.Errorf("failed to parse AWS RDS cert bundle")
			return
		}
		rdsCertPool = pool
	})
	require.NoError(t, rdsCertPoolErr)
	return rdsCertPool
}

// rdsInstanceInfo bundles the connection metadata pulled from
// DescribeDBInstances + Secrets Manager.
type rdsInstanceInfo struct {
	endpoint       string
	port           int
	masterUsername string
	masterPassword string
}

// describeRDSInstance fetches the endpoint, master username, and master
// password for the named RDS Postgres instance. The instance must have a
// managed master user secret configured (MasterUserSecret).
func describeRDSInstance(t *testing.T, ctx context.Context, region, instanceID string) rdsInstanceInfo {
	t.Helper()
	awsCfg, err := cloudawsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	require.NoError(t, err)

	rdsClt := rds.NewFromConfig(awsCfg)
	descOut, err := rdsClt.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: &instanceID,
	})
	require.NoError(t, err)
	require.Len(t, descOut.DBInstances, 1, "expected exactly one DB instance for %q", instanceID)
	inst := descOut.DBInstances[0]
	require.NotNil(t, inst.MasterUsername)
	require.NotNil(t, inst.Endpoint)
	require.NotNil(t, inst.Endpoint.Address)
	require.NotNil(t, inst.Endpoint.Port)
	require.NotNil(t, inst.MasterUserSecret, "instance %q must have a managed master user secret", instanceID)
	require.NotNil(t, inst.MasterUserSecret.SecretArn)

	secretsClt := secretsmanager.NewFromConfig(awsCfg)
	secretVal, err := secretsClt.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: inst.MasterUserSecret.SecretArn,
	})
	require.NoError(t, err)
	require.NotNil(t, secretVal.SecretString)
	var secret struct {
		Password string `json:"password"`
	}
	require.NoError(t, json.Unmarshal([]byte(*secretVal.SecretString), &secret))
	require.NotEmpty(t, secret.Password, "empty master password in secret %q", *inst.MasterUserSecret.SecretArn)

	return rdsInstanceInfo{
		endpoint:       *inst.Endpoint.Address,
		port:           int(*inst.Endpoint.Port),
		masterUsername: *inst.MasterUsername,
		masterPassword: secret.Password,
	}
}

// cleanupSQL registers statement as a SQL command to execute via conn at
// test cleanup. Errors are logged with t.Logf instead of failing the test,
// because cleanup runs after the test body has already finished asserting —
// a noisy cleanup failure should not mask the underlying test result.
//
// Uses context.Background() for the Exec, not t.Context(), because the
// test's context is already canceled by the time t.Cleanup runs.
func cleanupSQL(t *testing.T, conn *pgx.Conn, statement string) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), statement); err != nil {
			t.Logf("cleanup statement failed: %s: %v", statement, err)
		}
	})
}

// setupRDSTestEnv provisions the same fixture that setupSelfHostedTestEnv
// provides — a docs-prescribed least-privilege teleport-admin user, the
// teleport-managed roles, a second database for cross-db reassignment
// tests, and the four pgx connections — but against a real RDS Postgres
// instance addressed by the AWS_REGION and RDS_POSTGRES_INSTANCE_NAME
// environment variables.
//
// Because an RDS instance typically outlives a single test run, every
// server-side change is paired with a t.Cleanup that undoes it. The
// pairing is registered at the point of mutation so a future reader can
// see the create/undo together. t.Cleanup runs in LIFO, so the cleanup
// statements naturally unwind in reverse: each undo runs while every
// later step is still intact.
//
// The pre-clean at the top is the one statement that breaks the pattern:
// it recovers from leftover state created by a previous run that crashed
// before its cleanups could run, so the CREATE statements below succeed.
func setupRDSTestEnv(t *testing.T) *testEnv {
	ctx := t.Context()
	region := mustGetRDSEnv(t, awsRegionEnv)
	instanceID := mustGetRDSEnv(t, rdsPostgresInstanceNameEnv)

	info := describeRDSInstance(t, ctx, region, instanceID)
	tlsConfig := &tls.Config{
		ServerName: info.endpoint,
		RootCAs:    getRDSCertPool(t),
	}

	// connectWith builds a pgconn config with TLS attached and opens the
	// connection. The user/password are NOT embedded in the URL because
	// RDS master passwords come straight from Secrets Manager and often
	// contain characters that aren't valid URL userinfo (e.g. '/', '+',
	// ':'); we set them on the parsed config instead.
	connectWith := func(t *testing.T, username, password, dbName string) (*pgx.Conn, error) {
		cfg, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s:%d/%s",
			info.endpoint, info.port, dbName))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.User = username
		cfg.Password = password
		cfg.TLSConfig = tlsConfig.Clone()
		return pgx.ConnectConfig(t.Context(), cfg)
	}
	connectAsBootstrap := func(t *testing.T, dbName string) (*pgx.Conn, error) {
		return connectWith(t, info.masterUsername, info.masterPassword, dbName)
	}
	connectAsUser := func(t *testing.T, username, password, dbName string) (*pgx.Conn, error) {
		return connectWith(t, username, password, dbName)
	}

	// ── bootstrapConn — close last in cleanup so every other cleanup
	// statement below still has a connection to run through.
	bootstrapConn, err := connectAsBootstrap(t, "postgres")
	require.NoError(t, err)
	t.Cleanup(func() { bootstrapConn.Close(context.Background()) })

	// ── postgres_fdw extension. CASCADE on the drop because the "Refused:
	// foreign table" subtest creates a foreign server hanging off this
	// extension; if a prior run crashed before that subtest's cleanups
	// fired, the server is still there and a plain DROP EXTENSION fails.
	// IF NOT EXISTS lets a recovery run reuse a leftover extension.
	cleanupSQL(t, bootstrapConn, `DROP EXTENSION IF EXISTS postgres_fdw CASCADE`)
	_, err = bootstrapConn.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS postgres_fdw`)
	require.NoError(t, err)

	// ── teleport-admin user. DROP USER fails while the role still holds
	// grants (e.g. USAGE on the FDW about to be granted), so each GRANT
	// is paired with a REVOKE cleanup registered after the DROP USER
	// cleanup. t.Cleanup is LIFO, so each REVOKE runs first and DROP
	// USER fires with no dependencies remaining.
	cleanupSQL(t, bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, rdsAdminUser))
	_, err = bootstrapConn.Exec(ctx, fmt.Sprintf(
		`CREATE USER %q LOGIN CREATEROLE PASSWORD '%s'`,
		rdsAdminUser, rdsAdminPassword))
	require.NoError(t, err)
	_, err = bootstrapConn.Exec(ctx, fmt.Sprintf(
		`GRANT USAGE ON FOREIGN DATA WRAPPER postgres_fdw TO %q`, rdsAdminUser))
	require.NoError(t, err)
	cleanupSQL(t, bootstrapConn, fmt.Sprintf(
		`REVOKE USAGE ON FOREIGN DATA WRAPPER postgres_fdw FROM %q`, rdsAdminUser))

	// ── Cleanups for the engine-managed roles that ensureTeleportRole
	// creates lazily during the test run. teleport-auto-user is added on
	// the first ActivateUser; teleport-object-inheritor on the first
	// reassignment-mode DeleteUser. Whether either is created depends on
	// which subtests execute, so IF EXISTS guards a run that skips those
	// code paths. Alice users themselves are paired one-to-one with
	// cleanupSQL at each creation site, so no sweep is registered here.
	cleanupSQL(t, bootstrapConn, `DROP ROLE IF EXISTS "teleport-auto-user"`)
	cleanupSQL(t, bootstrapConn, `DROP ROLE IF EXISTS "teleport-object-inheritor"`)

	// ── other_db. FORCE in the cleanup terminates any still-live sessions
	// instead of having us close them in a particular order.
	cleanupSQL(t, bootstrapConn, `DROP DATABASE IF EXISTS other_db WITH (FORCE)`)
	_, err = bootstrapConn.Exec(ctx, `CREATE DATABASE other_db`)
	require.NoError(t, err)

	// ── bootstrapOtherDBConn. Grants applied via this connection are
	// owned by teleport-admin and so are revoked by DROP OWNED above; no
	// extra cleanup needed beyond closing the connection.
	bootstrapOtherDBConn, err := connectAsBootstrap(t, "other_db")
	require.NoError(t, err)
	t.Cleanup(func() { bootstrapOtherDBConn.Close(context.Background()) })
	_, err = bootstrapOtherDBConn.Exec(ctx, fmt.Sprintf(`
		GRANT CREATE, CONNECT ON DATABASE other_db TO %q;
		GRANT USAGE,  CREATE  ON SCHEMA   public   TO %q`,
		rdsAdminUser, rdsAdminUser))
	require.NoError(t, err)

	// ── adminConn and otherDBConn. teleport-admin peer connections for
	// assertions and fixtures. Closed before DROP USER fires.
	adminConn, err := connectAsUser(t, rdsAdminUser, rdsAdminPassword, "postgres")
	require.NoError(t, err)
	t.Cleanup(func() { adminConn.Close(context.Background()) })

	otherDBConn, err := connectAsUser(t, rdsAdminUser, rdsAdminPassword, "other_db")
	require.NoError(t, err)
	t.Cleanup(func() { otherDBConn.Close(context.Background()) })

	// Engine-side database resource. URI includes credentials because the
	// connector prepends "postgres://" and parses; TLS is provided by the
	// noopAuth via tlsConfig.
	dbURI := fmt.Sprintf("%s:%s@%s:%d", rdsAdminUser, rdsAdminPassword, info.endpoint, info.port)
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-pg",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      dbURI,
		AdminUser: &types.DatabaseAdminUser{
			Name: rdsAdminUser,
		},
	})
	require.NoError(t, err)

	engine := &Engine{
		EngineConfig: common.EngineConfig{
			Log:   slog.Default(),
			Auth:  &noopAuth{tlsConfig: tlsConfig},
			Audit: &noopAudit{},
		},
	}

	return &testEnv{
		db:                   db,
		engine:               engine,
		bootstrapConn:        bootstrapConn,
		bootstrapOtherDBConn: bootstrapOtherDBConn,
		adminConn:            adminConn,
		otherDBConn:          otherDBConn,
		connectAsUser:        connectAsUser,
		connectAsBootstrap:   connectAsBootstrap,
	}
}

// mustGetRDSEnv is the test-helper equivalent of os.Getenv that fails the
// test immediately when the named variable is missing.
func mustGetRDSEnv(t *testing.T, key string) string {
	t.Helper()
	val := os.Getenv(key)
	require.NotEmpty(t, val, "%s environment variable must be set when ENABLE_RDS_TESTS=true", key)
	return val
}

func installReassignObjectsProcedure(t *testing.T, conn *pgx.Conn) {
	_, err := conn.Exec(t.Context(), reassignObjectsProcedure)
	require.NoError(t, err)
	cleanupSQL(t, conn, `DROP SCHEMA IF EXISTS teleport_objects`)
	cleanupSQL(t, conn, `DROP PROCEDURE IF EXISTS teleport_objects.teleport_reassign_objects(varchar, varchar)`)
}

func TestUserAutoProvisioning(t *testing.T) {
	t.Run("self-hosted postgres", func(t *testing.T) {
		if run, _ := apiutils.ParseBool(os.Getenv("ENABLE_TESTCONTAINERS")); !run {
			// Docker Hub rate limits cause failures in CI, this test is disabled until we can set up an alternative to Docker Hub
			t.Skip("Test disabled in CI. Enable it by setting env variable ENABLE_TESTCONTAINERS")
		}
		for _, postgresImage := range postgresImages {
			t.Run(postgresImage, func(t *testing.T) {
				env := setupSelfHostedTestEnv(t, postgresImage)
				runUserAutoProvisioningTests(t, env)
			})
		}
	})
	t.Run("RDS postgres", func(t *testing.T) {
		if run, _ := apiutils.ParseBool(os.Getenv("TEST_AWS_DB")); !run {
			t.Skip("Test disabled in CI. Enable it by setting env variable TEST_AWS_DB")
		}
		env := setupRDSTestEnv(t)
		runUserAutoProvisioningTests(t, env)
	})
}

func runUserAutoProvisioningTests(t *testing.T, env *testEnv) {

	t.Run("ActivateUser", func(t *testing.T) {
		engine, db, adminConn := env.engine, env.db, env.adminConn

		t.Run("creates new user", func(t *testing.T) {
			username := "alice_new"
			err := engine.ActivateUser(t.Context(), makeSession(db, username, nil))
			require.NoError(t, err)
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))

			var exists bool
			err = adminConn.QueryRow(t.Context(),
				"SELECT true FROM pg_catalog.pg_user WHERE usename = $1", "alice_new").Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists)
		})

		t.Run("reactivates deactivated user", func(t *testing.T) {
			username := "alice_reactivate"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, username, nil)))
			require.False(t, canLogin(t, adminConn, username), "precondition: login should be disabled")
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, canLogin(t, adminConn, username), "user should be able to log in after reactivation")
		})

		t.Run("assigns roles", func(t *testing.T) {
			username := "alice_roles"
			cleanupSQL(t, adminConn, `DROP ROLE IF EXISTS testrole`)
			_, err := adminConn.Exec(t.Context(), `CREATE ROLE testrole`)
			require.NoError(t, err)
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, []string{"testrole"})))
			require.True(t, isMember(t, adminConn, username, "testrole"), "user should be member of testrole")
		})

		t.Run("preserves teleport-auto-user on reactivation", func(t *testing.T) {
			username := "alice_preserve"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, isMember(t, adminConn, username, "teleport-auto-user"),
				"precondition: user should be member of teleport-auto-user")
			require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, isMember(t, adminConn, username, "teleport-auto-user"),
				"teleport-auto-user membership must survive deactivate")
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, isMember(t, adminConn, username, "teleport-auto-user"),
				"teleport-auto-user membership must survive deactivate/reactivate cycle")
		})

		t.Run("rejects pre-existing non-teleport user", func(t *testing.T) {
			username := "alice_external"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			_, err := adminConn.Exec(t.Context(), fmt.Sprintf(`CREATE USER %q`, username))
			require.NoError(t, err)
			err = engine.ActivateUser(t.Context(), makeSession(db, username, nil))
			require.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists error, got: %v", err)
		})

		t.Run("active connection same roles succeeds", func(t *testing.T) {
			username := "alice_active_same"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			_, err := adminConn.Exec(t.Context(), fmt.Sprintf(`ALTER USER %q WITH PASSWORD 'testpass' LOGIN`, username))
			require.NoError(t, err)
			userConn, err := env.connectAsUser(t, username, "testpass", "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())
			err = engine.ActivateUser(t.Context(), makeSession(db, username, nil))
			require.NoError(t, err)
		})

		t.Run("active connection different roles fails", func(t *testing.T) {
			_, err := adminConn.Exec(t.Context(), `CREATE ROLE role_for_diff`)
			require.NoError(t, err)
			cleanupSQL(t, adminConn, `DROP ROLE IF EXISTS role_for_diff`)

			// Activate the user without any roles, then assign a password.
			username := "alice_active_diff"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			_, err = adminConn.Exec(t.Context(), fmt.Sprintf(`ALTER USER %q WITH PASSWORD 'testpass' LOGIN`, username))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, "testpass", "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			// ActivateUser with a new role while a connection is open should fail
			// because the active session's roles would need to change.
			err = engine.ActivateUser(t.Context(), makeSession(db, username, []string{"role_for_diff"}))
			require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got: %v", err)
		})

		t.Run("self-heals teleport-auto-user role", func(t *testing.T) {
			_, err := adminConn.Exec(t.Context(), `DROP ROLE IF EXISTS "teleport-auto-user"`)
			require.NoError(t, err)
			username := "alice_selfheal"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, isMember(t, adminConn, username, "teleport-auto-user"),
				"ActivateUser should recreate teleport-auto-user and add the user to it")
		})
	})

	t.Run("DeactivateUser", func(t *testing.T) {
		engine, db, adminConn := env.engine, env.db, env.adminConn

		t.Run("strips roles and disables login", func(t *testing.T) {
			_, err := adminConn.Exec(t.Context(), `CREATE ROLE testrole_deact`)
			require.NoError(t, err)
			cleanupSQL(t, adminConn, `DROP ROLE testrole_deact`)

			username := "alice_deact_roles"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, []string{"testrole_deact"})))
			require.True(t, isMember(t, adminConn, username, "testrole_deact"), "precondition")
			require.True(t, canLogin(t, adminConn, username), "precondition")

			require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, username, nil)))

			require.False(t, isMember(t, adminConn, username, "testrole_deact"),
				"non-teleport role should be revoked after deactivation")
			require.False(t, canLogin(t, adminConn, username),
				"login should be disabled after deactivation")
		})

		t.Run("is no-op when user has active connections", func(t *testing.T) {
			username := "alice_deact_active"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			_, err := adminConn.Exec(t.Context(),
				fmt.Sprintf(`ALTER USER %q WITH PASSWORD 'testpass'`, username))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, "testpass", "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, canLogin(t, adminConn, username),
				"login should remain enabled while an active connection is open")
		})

		t.Run("preserves teleport-auto-user membership", func(t *testing.T) {
			username := "alice_deact_preserve"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, isMember(t, adminConn, username, "teleport-auto-user"),
				"teleport-auto-user membership must be preserved after deactivation")
		})
	})

	t.Run("DeleteUser", func(t *testing.T) {
		engine, db, adminConn, bootstrapConn := env.engine, env.db, env.adminConn, env.bootstrapConn

		t.Run("drops user with no owned objects", func(t *testing.T) {
			username := "alice_delete_clean"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			require.NoError(t, engine.DeleteUser(t.Context(), makeSession(db, username, nil)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped")
		})

		t.Run("does not drop user with active connections", func(t *testing.T) {
			username := "alice_delete_active"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			_, err := adminConn.Exec(t.Context(),
				fmt.Sprintf(`ALTER USER %q WITH PASSWORD 'testpass' LOGIN`, username))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, "testpass", "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			require.NoError(t, engine.DeleteUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, userExists(t, adminConn, username), "user should still exist")
			require.True(t, canLogin(t, adminConn, username), "user should still have login")
		})

		t.Run("deactivates instead of dropping user who owns objects", func(t *testing.T) {
			username := "alice_delete_owns"
			cleanupSQL(t, env.adminConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_delete_owns_tbl`)
			require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, username, nil)))
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_delete_owns_tbl (id int);
				ALTER TABLE alice_delete_owns_tbl OWNER TO "alice_delete_owns"`)
			require.NoError(t, err)

			// db has no OrphanedResourceOwner, so reassignment is skipped; the user
			// cannot be dropped and is deactivated instead.
			require.NoError(t, engine.DeleteUser(t.Context(), makeSession(db, username, nil)))
			require.True(t, userExists(t, adminConn, username),
				"user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, adminConn, username),
				"user should have login disabled")
		})
	})

	t.Run("DeleteUser with reassignment", func(t *testing.T) {
		installReassignObjectsProcedure(t, env.bootstrapConn)

		engine, db := env.engine, env.db
		bootstrapConn, bootstrapOtherDBConn := env.bootstrapConn, env.bootstrapOtherDBConn
		adminConn, otherDBConn := env.adminConn, env.otherDBConn

		reassignSession := func(name string) *common.Session {
			return makeReassignSession(db, name, nil)
		}

		// ─ Successful reassignment ──────────────────────────────────────────────

		t.Run("Reassign: bare table", func(t *testing.T) {
			// Canonical happy-path. Explicitly checks the inheritor is the
			// destination owner so this file pins down the target role.
			username := "alice_re_bare"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_bare;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_bare_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_bare_tbl (id int);
				ALTER TABLE alice_re_bare_tbl OWNER TO "alice_re_bare"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username))
			require.Equal(t, "teleport-object-inheritor", tableOwner(t, adminConn, "public", "alice_re_bare_tbl"))
		})

		t.Run("Reassign: table with index", func(t *testing.T) {
			username := "alice_re_idx"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_idx;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_idx_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_idx_tbl (id int);
				CREATE INDEX alice_re_idx_tbl_idx ON alice_re_idx_tbl (id);
				ALTER TABLE alice_re_idx_tbl OWNER TO "alice_re_idx"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: table with identity sequence", func(t *testing.T) {
			// Identity columns create OWNED BY sequences. ALTER TABLE OWNER TO
			// does not propagate to such sequences, so the sequence loop in
			// reassign-objects.sql is what moves them.
			username := "alice_re_idseq"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_idseq;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_idseq_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_idseq_tbl (id int GENERATED ALWAYS AS IDENTITY);
				ALTER TABLE alice_re_idseq_tbl OWNER TO "alice_re_idseq";
				ALTER SEQUENCE alice_re_idseq_tbl_id_seq OWNER TO "alice_re_idseq"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: composite row type follows", func(t *testing.T) {
			// Every regular table has an auto-generated composite row type in
			// pg_type (typtype='c', typrelid pointing at the table). Verify
			// must exclude these or it would always fire.
			username := "alice_re_rowtype"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_rowtype;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_rowtype_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_rowtype_tbl (a int, b text, c timestamptz);
				ALTER TABLE alice_re_rowtype_tbl OWNER TO "alice_re_rowtype"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: array type follows", func(t *testing.T) {
			// Every table also has an auto-generated array type (typname
			// '_<tbl>'). Verify must exclude these; they are recognized by
			// catalog linkage (the row type's typarray points at the array
			// type), not by the '_' name prefix.
			username := "alice_re_arr"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_arr;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_arr_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_arr_tbl (vals int[]);
				ALTER TABLE alice_re_arr_tbl OWNER TO "alice_re_arr"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: FK constraint internal triggers are ignored", func(t *testing.T) {
			// Foreign-key constraints add pg_trigger rows with
			// tgisinternal=true. The safety guard ignores internal triggers,
			// so the tables are still reassigned.
			username := "alice_re_fk"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_fk;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `
				DROP TABLE IF EXISTS alice_re_fk_child;
				DROP TABLE IF EXISTS alice_re_fk_parent;
			`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_fk_parent (id int PRIMARY KEY);
				CREATE TABLE alice_re_fk_child (id int REFERENCES alice_re_fk_parent (id));
				ALTER TABLE alice_re_fk_parent OWNER TO "alice_re_fk";
				ALTER TABLE alice_re_fk_child  OWNER TO "alice_re_fk"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: standalone sequence", func(t *testing.T) {
			username := "alice_re_seq"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_seq;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP SEQUENCE IF EXISTS alice_re_seq_seq`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE SEQUENCE alice_re_seq_seq;
				ALTER SEQUENCE alice_re_seq_seq OWNER TO "alice_re_seq"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: mixed safe types", func(t *testing.T) {
			username := "alice_re_mix"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_mix;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `
				DROP TABLE IF EXISTS alice_re_mix_tbl;
				DROP SEQUENCE IF EXISTS alice_re_mix_seq;
			`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_mix_tbl (id int);
				CREATE SEQUENCE alice_re_mix_seq;
				ALTER TABLE alice_re_mix_tbl OWNER TO "alice_re_mix";
				ALTER SEQUENCE alice_re_mix_seq OWNER TO "alice_re_mix"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: unlogged table", func(t *testing.T) {
			// relpersistence 'u' (unlogged) is allowed; only temp ('t') is
			// rejected, and a temp table cannot survive to reassignment anyway.
			username := "alice_re_unlogged"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_unlogged;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_unlogged_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE UNLOGGED TABLE alice_re_unlogged_tbl (id int);
				ALTER TABLE alice_re_unlogged_tbl OWNER TO "alice_re_unlogged"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: table with primary key and unique constraints", func(t *testing.T) {
			// PRIMARY KEY and UNIQUE constraints (contype 'p'/'u') are allowed;
			// their backing indexes are plain and use built-in operator classes.
			username := "alice_re_uniq"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_uniq;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_uniq_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_uniq_tbl (id int PRIMARY KEY, email text UNIQUE);
				ALTER TABLE alice_re_uniq_tbl OWNER TO "alice_re_uniq"`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
		})

		t.Run("Reassign: bare table with a grant-option chain", func(t *testing.T) {
			// The user can plant a grant-option dependency chain on a table that
			// otherwise passes the whitelist: grant a privilege WITH GRANT OPTION
			// to a role it can act as (teleport-auto-user, which every managed
			// user is a member of), then grant onward as that role. A plain
			// REVOKE during reassignment would error with "dependent privileges
			// exist" and abort; the REVOKE ... CASCADE removes the dependent
			// grants too, so the table is still reassigned and the user dropped.
			username := "alice_re_goc"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_goc;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_goc_tbl`)
			// bootstrapConn is the superuser, so it can SET ROLE to either role
			// to make the grantors match what the user itself could produce.
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_goc_tbl (id int);
				ALTER TABLE alice_re_goc_tbl OWNER TO "alice_re_goc";
				SET ROLE "alice_re_goc";
				GRANT SELECT ON alice_re_goc_tbl TO "teleport-auto-user" WITH GRANT OPTION;
				SET ROLE "teleport-auto-user";
				GRANT SELECT ON alice_re_goc_tbl TO PUBLIC;
				RESET ROLE;`)
			require.NoError(t, err)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username), "user should be dropped after reassignment")
			require.Equal(t, "teleport-object-inheritor", tableOwner(t, adminConn, "public", "alice_re_goc_tbl"))
		})

		// ─ Reassignment refused → user deactivated ──────────────────────────────

		t.Run("Refused: partition child", func(t *testing.T) {
			username := "alice_no_pchild"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_pchild;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_pchild_p1;
				DROP TABLE IF EXISTS alice_no_pchild_parent;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_pchild_parent (id int) PARTITION BY RANGE (id);
				CREATE TABLE alice_no_pchild_p1 PARTITION OF alice_no_pchild_parent
					FOR VALUES FROM (1) TO (10);
				ALTER TABLE alice_no_pchild_p1 OWNER TO "alice_no_pchild";`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: partitioned parent", func(t *testing.T) {
			username := "alice_no_pparent"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_pparent;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_pparent_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_pparent_tbl (id int) PARTITION BY RANGE (id);
				ALTER TABLE alice_no_pparent_tbl OWNER TO "alice_no_pparent"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with row-level security", func(t *testing.T) {
			username := "alice_no_rls"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_rls;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_rls_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_rls_tbl (id int);
				ALTER TABLE alice_no_rls_tbl ENABLE ROW LEVEL SECURITY;
				ALTER TABLE alice_no_rls_tbl OWNER TO "alice_no_rls"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with SECURITY INVOKER trigger", func(t *testing.T) {
			username := "alice_no_tinv"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_tinv;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_tinv_tbl;
				DROP FUNCTION IF EXISTS alice_no_tinv_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_tinv_fn() RETURNS trigger LANGUAGE plpgsql AS $$
					BEGIN RETURN NEW; END $$;
				CREATE TABLE alice_no_tinv_tbl (id int);
				CREATE TRIGGER alice_no_tinv_tg BEFORE INSERT ON alice_no_tinv_tbl
					FOR EACH ROW EXECUTE FUNCTION alice_no_tinv_fn();
				ALTER TABLE alice_no_tinv_tbl OWNER TO "alice_no_tinv"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with SECURITY DEFINER trigger", func(t *testing.T) {
			username := "alice_no_tdef"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_tdef;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_tdef_tbl;
				DROP FUNCTION IF EXISTS alice_no_tdef_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_tdef_fn() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
					BEGIN RETURN NEW; END $$;
				CREATE TABLE alice_no_tdef_tbl (id int);
				CREATE TRIGGER alice_no_tdef_tg BEFORE INSERT ON alice_no_tdef_tbl
					FOR EACH ROW EXECUTE FUNCTION alice_no_tdef_fn();
				ALTER TABLE alice_no_tdef_tbl OWNER TO "alice_no_tdef"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with rewrite rule", func(t *testing.T) {
			username := "alice_no_rule"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_rule;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_rule_tbl`)
			// A DML rewrite rule runs with the table owner's privileges for
			// referenced relations, so a rule-bearing table must not be
			// reassigned to the inheritor.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_rule_tbl (id int);
				CREATE RULE alice_no_rule_r AS ON INSERT TO alice_no_rule_tbl DO INSTEAD NOTHING;
				ALTER TABLE alice_no_rule_tbl OWNER TO "alice_no_rule"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with a policy", func(t *testing.T) {
			// A policy can exist while row-level security is disabled (dormant),
			// so relrowsecurity is false here and the relrowsecurity guard does
			// not fire; the pg_policy guard is what refuses the table.
			username := "alice_no_pol"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_pol;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_pol_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_pol_tbl (id int);
				CREATE POLICY alice_no_pol_p ON alice_no_pol_tbl USING (true);
				ALTER TABLE alice_no_pol_tbl OWNER TO "alice_no_pol"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with expression index on a user function", func(t *testing.T) {
			// ANALYZE evaluates an expression index with the table owner's
			// identity, so a table indexing a non-system function must not be
			// reassigned to the inheritor. cleanupSQL runs LIFO, so the table
			// (registered last) is dropped before the function it depends on.
			username := "alice_no_exidx"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_exidx;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_exidx_tbl;
				DROP FUNCTION IF EXISTS alice_no_exidx_fn(int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_exidx_fn(i int) RETURNS int LANGUAGE sql IMMUTABLE AS 'SELECT i + 1';
				CREATE TABLE alice_no_exidx_tbl (id int);
				CREATE INDEX alice_no_exidx_idx ON alice_no_exidx_tbl (alice_no_exidx_fn(id));
				ALTER TABLE alice_no_exidx_tbl OWNER TO "alice_no_exidx"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with default calling a user function", func(t *testing.T) {
			// Any column default disqualifies the table; the whitelist requires
			// pg_attrdef to be empty. This user-function default is one instance;
			// the built-in-default case nearby covers ordinary defaults like now().
			username := "alice_no_def"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_def;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_def_tbl;
				DROP FUNCTION IF EXISTS alice_no_def_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_def_fn() RETURNS int LANGUAGE sql AS 'SELECT 42';
				CREATE TABLE alice_no_def_tbl (id int DEFAULT alice_no_def_fn());
				ALTER TABLE alice_no_def_tbl OWNER TO "alice_no_def"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with check constraint using a user operator", func(t *testing.T) {
			// A non-system operator runs its oprcode with the table owner's
			// identity; its dependency is on pg_operator, not pg_proc.
			username := "alice_no_op"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_op;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_op_tbl;
				DROP OPERATOR IF EXISTS === (int, int);
				DROP FUNCTION IF EXISTS alice_no_op_fn(int, int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_op_fn(a int, b int) RETURNS bool LANGUAGE sql IMMUTABLE AS 'SELECT a > b';
				CREATE OPERATOR === (LEFTARG = int, RIGHTARG = int, FUNCTION = alice_no_op_fn);
				CREATE TABLE alice_no_op_tbl (id int, CONSTRAINT alice_no_op_ck CHECK (id === 0));
				ALTER TABLE alice_no_op_tbl OWNER TO "alice_no_op"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with check constraint casting to a user type", func(t *testing.T) {
			// A custom type in an expression can run custom I/O or cast
			// functions with the table owner's identity; its dependency is on
			// pg_type, not pg_proc.
			username := "alice_no_typ"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_typ;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_typ_tbl;
				DROP TYPE IF EXISTS alice_no_typ_e;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TYPE alice_no_typ_e AS ENUM ('x', 'y');
				CREATE TABLE alice_no_typ_tbl (v text, CONSTRAINT alice_no_typ_ck CHECK (v::alice_no_typ_e = 'x'));
				ALTER TABLE alice_no_typ_tbl OWNER TO "alice_no_typ"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with extended statistics on a user function", func(t *testing.T) {
			// Expression statistics are evaluated during ANALYZE with the table
			// owner's identity; the dependency is on pg_statistic_ext. Expression
			// statistics require PostgreSQL 14+ (our minimum).
			username := "alice_no_stx"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_stx;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_stx_tbl;
				DROP FUNCTION IF EXISTS alice_no_stx_fn(int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_stx_fn(i int) RETURNS int LANGUAGE sql IMMUTABLE AS 'SELECT i + 1';
				CREATE TABLE alice_no_stx_tbl (a int, b int);
				CREATE STATISTICS alice_no_stx_s ON (alice_no_stx_fn(a)) FROM alice_no_stx_tbl;
				ALTER TABLE alice_no_stx_tbl OWNER TO "alice_no_stx"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with a column of a user domain", func(t *testing.T) {
			// A domain's CHECK runs on every cast to it, in the writer's
			// context. Its function dependency lives on the domain, reached
			// through the column's type rather than a table expression, so the
			// built-in-only column-type guard is what catches it.
			username := "alice_no_dom"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_dom;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_dom_tbl;
				DROP DOMAIN IF EXISTS alice_no_dom_d;
				DROP FUNCTION IF EXISTS alice_no_dom_fn(int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_dom_fn(i int) RETURNS bool LANGUAGE sql IMMUTABLE AS 'SELECT i >= 0';
				CREATE DOMAIN alice_no_dom_d AS int CHECK (alice_no_dom_fn(VALUE));
				CREATE TABLE alice_no_dom_tbl (id alice_no_dom_d);
				ALTER TABLE alice_no_dom_tbl OWNER TO "alice_no_dom"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with a user enum column", func(t *testing.T) {
			// Enums are inert, but the built-in-only column-type rule rejects
			// every user-defined type to keep the invariant simple, so an enum
			// column still blocks reassignment. This pins that decision.
			username := "alice_no_enumcol"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_enumcol;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_enumcol_tbl;
				DROP TYPE IF EXISTS alice_no_enumcol_e;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TYPE alice_no_enumcol_e AS ENUM ('a', 'b');
				CREATE TABLE alice_no_enumcol_tbl (status alice_no_enumcol_e);
				ALTER TABLE alice_no_enumcol_tbl OWNER TO "alice_no_enumcol"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with a built-in default", func(t *testing.T) {
			// The whitelist rejects ALL column defaults, even built-in ones such as
			// now(); pg_attrdef must be empty. Previously such defaults were allowed.
			username := "alice_no_bdef"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_bdef;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_bdef_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_bdef_tbl (id int, created timestamptz DEFAULT now());
				ALTER TABLE alice_no_bdef_tbl OWNER TO "alice_no_bdef"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with a built-in check constraint", func(t *testing.T) {
			// The whitelist rejects ALL CHECK constraints (contype 'c'), even those
			// using only built-in operators. Previously built-in checks were allowed.
			username := "alice_no_bck"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_bck;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_bck_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_bck_tbl (id int, CONSTRAINT alice_no_bck_ck CHECK (id > 0));
				ALTER TABLE alice_no_bck_tbl OWNER TO "alice_no_bck"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with a generated column", func(t *testing.T) {
			// A generated column's expression lives in pg_attrdef (attgenerated='s')
			// and is evaluated with the owner's identity; rejected even when the
			// expression uses only built-in operators.
			username := "alice_no_gen"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_gen;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_gen_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_gen_tbl (id int, doubled int GENERATED ALWAYS AS (id * 2) STORED);
				ALTER TABLE alice_no_gen_tbl OWNER TO "alice_no_gen"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: table with an exclusion constraint", func(t *testing.T) {
			// EXCLUDE constraints (contype 'x') are rejected; only PK/UNIQUE/FK are
			// allowed. int4range with the built-in && operator and range GiST opclass
			// needs no extension.
			username := "alice_no_excl"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_excl;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_excl_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_no_excl_tbl (during int4range, EXCLUDE USING gist (during WITH &&));
				ALTER TABLE alice_no_excl_tbl OWNER TO "alice_no_excl"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: typed table", func(t *testing.T) {
			// A table defined OF a composite type (reloftype <> 0) is rejected: it is
			// bound to a type that can carry behavior. The same cleanupSQL call drops
			// the table before the type (statements run in order).
			username := "alice_no_typed"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_typed;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_typed_tbl;
				DROP TYPE IF EXISTS alice_no_typed_t;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TYPE alice_no_typed_t AS (a int, b text);
				CREATE TABLE alice_no_typed_tbl OF alice_no_typed_t;
				ALTER TABLE alice_no_typed_tbl OWNER TO "alice_no_typed"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: index using a user-defined operator class", func(t *testing.T) {
			// A plain index over a built-in column can still bind a user-defined
			// operator class, whose support functions run custom code. Creating an
			// operator class requires superuser, so skip where unavailable (e.g. RDS),
			// where the scenario is unreachable anyway.
			var isSuper bool
			require.NoError(t, env.bootstrapConn.QueryRow(t.Context(),
				"SELECT rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&isSuper))
			if !isSuper {
				t.Skip("CREATE OPERATOR CLASS requires superuser")
			}
			username := "alice_no_opc"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_opc;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_opc_tbl;
				DROP OPERATOR CLASS IF EXISTS alice_no_opc_ops USING btree;
			`)
			// The opclass reuses built-in operators and btint4cmp but lives outside
			// pg_catalog, so the built-in-opclass guard (not the expr/predicate guard)
			// is what refuses the plain index.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OPERATOR CLASS alice_no_opc_ops FOR TYPE int4 USING btree AS
					OPERATOR 1 <, OPERATOR 2 <=, OPERATOR 3 =, OPERATOR 4 >=, OPERATOR 5 >,
					FUNCTION 1 btint4cmp(int4, int4);
				CREATE TABLE alice_no_opc_tbl (id int);
				CREATE INDEX alice_no_opc_idx ON alice_no_opc_tbl (id alice_no_opc_ops);
				ALTER TABLE alice_no_opc_tbl OWNER TO "alice_no_opc"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: function owned by user", func(t *testing.T) {
			username := "alice_no_fn"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_fn;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP FUNCTION IF EXISTS alice_no_fn_fn()`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE FUNCTION alice_no_fn_fn() RETURNS int LANGUAGE sql AS 'SELECT 1';
				ALTER FUNCTION alice_no_fn_fn() OWNER TO "alice_no_fn"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: view owned by user", func(t *testing.T) {
			username := "alice_no_view"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_view;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP VIEW IF EXISTS alice_no_view_v`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE VIEW alice_no_view_v AS SELECT 1 AS x;
				ALTER VIEW alice_no_view_v OWNER TO "alice_no_view"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: materialized view owned by user", func(t *testing.T) {
			username := "alice_no_mv"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_mv;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP MATERIALIZED VIEW IF EXISTS alice_no_mv_mv`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE MATERIALIZED VIEW alice_no_mv_mv AS SELECT 1 AS x;
				ALTER MATERIALIZED VIEW alice_no_mv_mv OWNER TO "alice_no_mv"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: foreign table owned by user", func(t *testing.T) {
			username := "alice_no_ft"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_ft;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP FOREIGN TABLE IF EXISTS alice_no_ft_ft;
				DROP SERVER IF EXISTS alice_no_ft_srv;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE EXTENSION IF NOT EXISTS postgres_fdw;
				CREATE SERVER alice_no_ft_srv FOREIGN DATA WRAPPER postgres_fdw;
				CREATE FOREIGN TABLE alice_no_ft_ft (id int) SERVER alice_no_ft_srv;
				ALTER FOREIGN TABLE alice_no_ft_ft OWNER TO "alice_no_ft"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: standalone composite type", func(t *testing.T) {
			username := "alice_no_ct"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_ct;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TYPE IF EXISTS alice_no_ct_t`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TYPE alice_no_ct_t AS (a int, b text);
				ALTER TYPE alice_no_ct_t OWNER TO "alice_no_ct"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: domain owned by user", func(t *testing.T) {
			username := "alice_no_dom"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_dom;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP DOMAIN IF EXISTS alice_no_dom_d`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE DOMAIN alice_no_dom_d AS int;
				ALTER DOMAIN alice_no_dom_d OWNER TO "alice_no_dom"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: enum type owned by user", func(t *testing.T) {
			username := "alice_no_enum"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_enum;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TYPE IF EXISTS alice_no_enum_e`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TYPE alice_no_enum_e AS ENUM ('a', 'b', 'c');
				ALTER TYPE alice_no_enum_e OWNER TO "alice_no_enum"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: schema owned by user", func(t *testing.T) {
			// Schemas are not reassigned, so a user that owns one still owns it
			// afterward and is deactivated rather than dropped.
			username := "alice_no_sch"
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, username, nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_no_sch;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS alice_no_sch_s`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE SCHEMA alice_no_sch_s;
				ALTER SCHEMA alice_no_sch_s OWNER TO "alice_no_sch"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, username), "user should have login disabled")
			require.True(t, sourceStillOwnsAnything(t, env.adminConn, username), "source should still own the offending object (savepoint rollback)")
		})

		t.Run("Refused: source user is not a Teleport user", func(t *testing.T) {
			// The procedure reassigns objects only for auto-provisioned
			// users. A role that owns a table but is not a member of
			// teleport-auto-user is rejected before anything is reassigned.
			// Called directly because DeleteUser only ever passes a member.
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession("alice_member")))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_member;
				DROP USER IF EXISTS non_teleport_user;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS non_teleport_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE USER non_teleport_user;
				CREATE TABLE non_teleport_tbl (id int);
				ALTER TABLE non_teleport_tbl OWNER TO non_teleport_user`)
			require.NoError(t, err)

			_, err = adminConn.Exec(t.Context(),
				`CALL teleport_objects.teleport_reassign_objects('non_teleport_user', 'teleport-object-inheritor')`)
			require.ErrorContains(t, err, "not a Teleport-managed user")
			require.Equal(t, "non_teleport_user", tableOwner(t, adminConn, "public", "non_teleport_tbl"),
				"non-teleport user's table must not be reassigned")
		})

		t.Run("Refused: destination is not teleport-object-inheritor", func(t *testing.T) {
			// Called directly because DeleteUser only ever passes
			// teleport-object-inheritor.
			username := "alice_re_baddest"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_baddest;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_baddest_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_re_baddest_tbl (id int);
				ALTER TABLE alice_re_baddest_tbl OWNER TO "alice_re_baddest"`)
			require.NoError(t, err)

			_, err = adminConn.Exec(t.Context(),
				`CALL teleport_objects.teleport_reassign_objects('alice_re_baddest', 'teleport-admin')`)
			require.ErrorContains(t, err, "destination_user must be teleport-object-inheritor")
			require.Equal(t, "alice_re_baddest", tableOwner(t, adminConn, "public", "alice_re_baddest_tbl"),
				"object must not be reassigned when the destination is rejected")
		})

		// ─ Atomicity ────────────────────────────────────────────────────────────

		t.Run("Refused: mixed reassignable + non-reassignable rolls back", func(t *testing.T) {
			// alice owns a bare table (would be reassigned) AND a function
			// (which the procedure does not touch, and verify catches). The
			// BEGIN/EXCEPTION around CALL teleport_reassign_objects in
			// delete-user.sql rolls the whole reassignment savepoint back;
			// the table must still be owned by alice after the failed delete.
			username := "alice_atomic"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_atomic;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `
				DROP TABLE IF EXISTS alice_atomic_tbl;
				DROP FUNCTION IF EXISTS alice_atomic_fn();
			`)
			_, err := bootstrapConn.Exec(t.Context(), `
				CREATE TABLE alice_atomic_tbl (id int);
				ALTER TABLE alice_atomic_tbl OWNER TO "alice_atomic";
				CREATE FUNCTION alice_atomic_fn() RETURNS int LANGUAGE sql AS 'SELECT 1';
				ALTER FUNCTION alice_atomic_fn() OWNER TO "alice_atomic"`)
			require.NoError(t, err)

			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.True(t, userExists(t, adminConn, username))
			require.False(t, canLogin(t, adminConn, username))
			require.Equal(t, "alice_atomic", tableOwner(t, adminConn, "public", "alice_atomic_tbl"),
				"table ownership must roll back when reassignment is refused")
		})

		// ─ Concurrency ──────────────────────────────────────────────────────────

		t.Run("Reassign: login attempt during reassignment is rejected", func(t *testing.T) {
			username := "alice_login_race"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_login_race;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_login_race_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				ALTER USER "alice_login_race" WITH PASSWORD 'testpass' LOGIN;
				CREATE TABLE alice_login_race_tbl (id int);
				ALTER TABLE alice_login_race_tbl OWNER TO "alice_login_race"`)
			require.NoError(t, err)

			// Hold AccessShare on the user's table from a fresh superuser
			// connection. teleport-admin has no INHERIT on alice in PG16+
			// so an admin-owned lockConn wouldn't have SELECT; and if we
			// opened the lock as alice, alice's session in pg_stat_activity
			// would trip delete-user.sql's "active connections on current
			// database" early-bail before reassignment ever runs. The
			// superuser sits outside both constraints, can LOCK any table,
			// and isn't usename='alice' to the procedure's check.
			lockConn, err := env.connectAsBootstrap(t, "postgres")
			require.NoError(t, err)
			defer lockConn.Close(context.Background())
			tx, err := lockConn.Begin(t.Context())
			require.NoError(t, err)
			_, err = tx.Exec(t.Context(), `LOCK TABLE alice_login_race_tbl IN ACCESS SHARE MODE`)
			require.NoError(t, err)

			deleteDone := make(chan error, 1)
			go func() {
				deleteDone <- engine.DeleteUser(t.Context(), reassignSession("alice_login_race"))
			}()

			// Wait until DeleteUser's backend is waiting on the relation lock.
			lockPID := lockConn.PgConn().PID()
			require.Eventually(t, func() bool {
				var n int
				if err := adminConn.QueryRow(t.Context(), `
					SELECT COUNT(*) FROM pg_stat_activity
					WHERE wait_event_type = 'Lock'
					AND wait_event = 'relation'
					AND pid != pg_backend_pid()
					AND pid != $1`, lockPID).Scan(&n); err != nil {
					return false
				}
				return n > 0
			}, 5*time.Second, 50*time.Millisecond, "DeleteUser should be waiting on the relation lock")

			// NOLOGIN was committed by delete-user.sql before reassignment
			// began, so a fresh login attempt must be rejected.
			_, loginErr := env.connectAsUser(t, "alice_login_race", "testpass", "postgres")
			require.Error(t, loginErr, "login should be rejected while NOLOGIN is in effect")
			require.Contains(t, loginErr.Error(), "not permitted to log in")

			// Release the lock; DeleteUser completes and drops the user.
			require.NoError(t, tx.Commit(t.Context()))
			require.NoError(t, <-deleteDone)
			require.False(t, userExists(t, adminConn, "alice_login_race"))
		})

		t.Run("Reassign: current-db active session bails before reassign", func(t *testing.T) {
			username := "alice_re_active"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_active;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_active_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				ALTER USER "alice_re_active" WITH PASSWORD 'testpass' LOGIN;
				CREATE TABLE alice_re_active_tbl (id int);
				ALTER TABLE alice_re_active_tbl OWNER TO "alice_re_active"`)
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, "testpass", "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.True(t, userExists(t, adminConn, username))
			require.True(t, canLogin(t, adminConn, username),
				"LOGIN should be restored when an active current-db session blocks delete")
			// Table ownership untouched: reassignment never ran.
			require.Equal(t, "alice_re_active", tableOwner(t, adminConn, "public", "alice_re_active_tbl"))
		})

		// ─ Edge cases ───────────────────────────────────────────────────────────

		t.Run("Reassign: user owns nothing", func(t *testing.T) {
			username := "alice_re_clean"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_clean;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.False(t, userExists(t, adminConn, username))
		})

		t.Run("Reassign: other-db active session restores LOGIN", func(t *testing.T) {
			username := "alice_re_otherdb"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_otherdb;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			cleanupSQL(t, bootstrapConn, `DROP TABLE IF EXISTS alice_re_otherdb_tbl`)
			_, err := bootstrapConn.Exec(t.Context(), `
				ALTER USER "alice_re_otherdb" WITH PASSWORD 'testpass' LOGIN;
				CREATE TABLE alice_re_otherdb_tbl (id int);
				ALTER TABLE alice_re_otherdb_tbl OWNER TO "alice_re_otherdb"`)
			require.NoError(t, err)

			// Hold a session in other_db. CONNECT to a newly-created database
			// is granted to PUBLIC by default, so no extra GRANT is needed.
			otherSessionConn, err := env.connectAsUser(t, username, "testpass", "other_db")
			require.NoError(t, err)
			defer otherSessionConn.Close(context.Background())

			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			require.True(t, userExists(t, adminConn, username))
			require.True(t, canLogin(t, adminConn, username),
				"LOGIN should be restored when an other-db session blocks drop")
			// Distinct from the current-db case: reassignment ran and committed
			// before the second active-sessions check fired.
			require.Equal(t, "teleport-object-inheritor",
				tableOwner(t, adminConn, "public", "alice_re_otherdb_tbl"))
		})

		t.Run("Refused: object in another database is ignored by dbid filter", func(t *testing.T) {
			username := "alice_re_otherobj"
			require.NoError(t, engine.ActivateUser(t.Context(), reassignSession(username)))
			cleanupSQL(t, bootstrapConn, `
				DROP USER IF EXISTS alice_re_otherobj;
				DROP USER IF EXISTS "teleport-auto-user";
			`)
			// The object alice owns lives in other_db, so its DROP TABLE
			// runs through bootstrapOtherDBConn. LIFO orders this drop
			// before the DROP USER above so the cluster-wide pg_shdepend
			// row is gone by the time DROP USER fires.
			cleanupSQL(t, bootstrapOtherDBConn, `DROP TABLE IF EXISTS alice_re_otherobj_tbl`)
			// Create an object owned by alice in other_db. Reassignment in
			// postgres-db must not touch it (dbid filter), and verify must not
			// flag it.
			_, err := bootstrapOtherDBConn.Exec(t.Context(), `
				CREATE TABLE alice_re_otherobj_tbl (id int);
				ALTER TABLE alice_re_otherobj_tbl OWNER TO "alice_re_otherobj"`)
			require.NoError(t, err)

			require.NoError(t, engine.DeleteUser(t.Context(), reassignSession(username)))
			// reassign-objects.sql returned cleanly because the dbid filter
			// excluded the other-db object. DROP USER then failed because the
			// cluster-wide pg_shdepend lookup still sees the row, so alice was
			// deactivated.
			require.True(t, userExists(t, adminConn, username))
			require.False(t, canLogin(t, adminConn, username))
			// The other-db object is untouched.
			require.Equal(t, "alice_re_otherobj",
				tableOwner(t, otherDBConn, "public", "alice_re_otherobj_tbl"))
		})
	})

	t.Run("reassign-objects.sql grant hardening", func(t *testing.T) {
		// A server-level ALTER DEFAULT PRIVILEGES can put grants on new
		// functions and schemas directly in their ACLs, not through PUBLIC, so
		// the script must revoke the whole ACL on both the procedure and its
		// schema -- not just from PUBLIC.
		bootstrapConn := env.bootstrapConn

		// Cleanups run LIFO; the ADP resets must precede the adp_grantee drop,
		// as a role referenced by pg_default_acl cannot be dropped.
		cleanupSQL(t, bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
		cleanupSQL(t, bootstrapConn, `DROP ROLE IF EXISTS adp_grantee`)
		cleanupSQL(t, bootstrapConn, `DROP ROLE IF EXISTS plain_role`)
		cleanupSQL(t, bootstrapConn, `ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM adp_grantee`)
		cleanupSQL(t, bootstrapConn, `ALTER DEFAULT PRIVILEGES REVOKE ALL ON SCHEMAS FROM adp_grantee`)

		_, err := bootstrapConn.Exec(t.Context(), `
			CREATE ROLE adp_grantee;
			CREATE ROLE plain_role`)
		require.NoError(t, err)

		// FUNCTIONS also covers procedures.
		_, err = bootstrapConn.Exec(t.Context(), `
			ALTER DEFAULT PRIVILEGES GRANT EXECUTE ON FUNCTIONS TO adp_grantee;
			ALTER DEFAULT PRIVILEGES GRANT USAGE, CREATE ON SCHEMAS TO adp_grantee`)
		require.NoError(t, err)
		_, err = bootstrapConn.Exec(t.Context(), reassignObjectsProcedure)
		require.NoError(t, err)

		t.Run("procedure: only teleport-admin can EXECUTE", func(t *testing.T) {
			require.True(t, hasExecuteOnReassignProc(t, bootstrapConn, "teleport-admin"),
				"teleport-admin must keep EXECUTE")
			require.False(t, hasExecuteOnReassignProc(t, bootstrapConn, "adp_grantee"),
				"the ALTER DEFAULT PRIVILEGES grant must be revoked")
			require.False(t, hasExecuteOnReassignProc(t, bootstrapConn, "plain_role"),
				"PUBLIC's default EXECUTE must be revoked")
		})

		t.Run("schema: only teleport-admin has USAGE", func(t *testing.T) {
			require.True(t, hasSchemaPrivilege(t, bootstrapConn, "teleport-admin", "USAGE"),
				"teleport-admin must keep USAGE")
			require.False(t, hasSchemaPrivilege(t, bootstrapConn, "teleport-admin", "CREATE"),
				"teleport-admin must not be granted CREATE")
			require.False(t, hasSchemaPrivilege(t, bootstrapConn, "adp_grantee", "USAGE"),
				"the ALTER DEFAULT PRIVILEGES grant must be revoked")
			require.False(t, hasSchemaPrivilege(t, bootstrapConn, "plain_role", "USAGE"),
				"PUBLIC must have no USAGE")
		})
	})
}
