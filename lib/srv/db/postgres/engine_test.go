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
	"crypto/rand"
	"crypto/tls"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
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

// noopAuth implements only the common.Auth methods used during
// ActivateUser/DeactivateUser/DeleteUser; any other call panics, surfacing a
// new code path that needs a real implementation.
type noopAuth struct {
	common.Auth
	tlsConfig *tls.Config
}

func (a *noopAuth) GetTLSConfig(_ context.Context, _ time.Time, _ types.Database, _ string) (*tls.Config, error) {
	return a.tlsConfig, nil
}
func (a *noopAuth) WithLogger(_ func(*slog.Logger) *slog.Logger) common.Auth { return a }
func (a *noopAuth) WithSession(_ *common.Session) common.Auth                { return a }

// noopChecker returns empty database permissions so applyPermissions exits early.
type noopChecker struct {
	services.AccessChecker
}

func (c *noopChecker) GetDatabasePermissions(_ types.Database) (types.DatabasePermissions, types.DatabasePermissions, error) {
	return nil, nil, nil
}

// noopAudit implements only the common.Audit methods used during the user
// lifecycle; any other call panics.
type noopAudit struct {
	common.Audit
}

func (a *noopAudit) OnDatabaseUserCreate(_ context.Context, _ *common.Session, _ error) {}
func (a *noopAudit) OnDatabaseUserDeactivate(_ context.Context, _ *common.Session, _ bool, _ error) {
}

// makeSession normalises a nil roles slice to empty, to avoid passing SQL NULL
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

func canLogin(t *testing.T, conn *pgx.Conn, username string) bool {
	t.Helper()
	var login bool
	err := conn.QueryRow(t.Context(),
		"SELECT rolcanlogin FROM pg_roles WHERE rolname = $1", username).Scan(&login)
	require.NoError(t, err)
	return login
}

func userExists(t *testing.T, conn *pgx.Conn, username string) bool {
	t.Helper()
	var count int
	err := conn.QueryRow(t.Context(),
		"SELECT COUNT(*) FROM pg_roles WHERE rolname = $1", username).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

// makeReassignSession sets AutoCreateUserMode to BEST_EFFORT_REASSIGN_AND_DROP
// so DeleteUser runs the per-object reassignment path.
func makeReassignSession(db types.Database, username string, roles []string) *common.Session {
	s := makeSession(db, username, roles)
	s.AutoCreateUserMode = types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_REASSIGN_AND_DROP
	return s
}

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

func hasExecuteOnReassignProc(t *testing.T, conn *pgx.Conn, role string) bool {
	t.Helper()
	var ok bool
	err := conn.QueryRow(t.Context(),
		`SELECT has_function_privilege($1, 'teleport_objects.teleport_reassign_objects(varchar, varchar)', 'EXECUTE')`,
		role).Scan(&ok)
	require.NoError(t, err)
	return ok
}

func hasSchemaPrivilege(t *testing.T, conn *pgx.Conn, role, privilege string) bool {
	t.Helper()
	var ok bool
	err := conn.QueryRow(t.Context(),
		`SELECT has_schema_privilege($1, 'teleport_objects', $2)`,
		role, privilege).Scan(&ok)
	require.NoError(t, err)
	return ok
}

// After a refused reassignment, delete-user.sql's savepoint rollback should
// leave username still owning at least one object. Reads pg_roles, not the
// superuser-only pg_authid.
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

// requireDeactivatedNotDropped asserts the state a refused reassignment leaves
// behind: the user still exists, cannot log in, and still owns objects.
func requireDeactivatedNotDropped(t *testing.T, conn *pgx.Conn, username string) {
	t.Helper()
	require.True(t, userExists(t, conn, username), "user should still exist (deactivated, not dropped)")
	require.False(t, canLogin(t, conn, username), "user should have login disabled")
	require.True(t, sourceStillOwnsAnything(t, conn, username), "source should still own the offending object (savepoint rollback)")
}

// testEnv is the shared fixture for the provisioning tests.
type testEnv struct {
	db     types.Database
	engine *Engine
	// Bootstrap user on postgres db. Closed automatically via t.Cleanup.
	bootstrapConn *pgx.Conn
	// Bootstrap user on other_db db. Closed automatically via t.Cleanup.
	bootstrapOtherDBConn *pgx.Conn
	// Teleport admin user on postgres db. Closed automatically via t.Cleanup.
	adminConn *pgx.Conn
	// Teleport admin user on other_db db. Closed automatically via t.Cleanup.
	otherDBConn *pgx.Conn

	// userPassword is the login password for the auto-provisioned test users,
	// generated per run so no credential is hardcoded.
	userPassword string

	// connectAsUser opens a fresh connection as an arbitrary user; the caller
	// closes it. A closure so it can hide the testcontainers/RDS differences.
	connectAsUser func(t *testing.T, username, password, dbName string) (*pgx.Conn, error)

	// connectAsBootstrap opens a fresh bootstrap-user connection, for tests
	// needing a session distinct from bootstrapConn (e.g. to hold a lock).
	connectAsBootstrap func(t *testing.T, dbName string) (*pgx.Conn, error)
}

// ensureRole idempotently creates role name and registers its teardown.
func ensureRole(t *testing.T, conn *pgx.Conn, name string) {
	t.Helper()
	cleanupSQL(t, conn, fmt.Sprintf(`DROP ROLE IF EXISTS %q`, name))
	_, err := conn.Exec(t.Context(), fmt.Sprintf(`
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%[1]s') THEN
				CREATE ROLE %[1]q;
			END IF;
		END $$`, name))
	require.NoError(t, err)
}

// randomPassword returns a cryptographically random hex password. Generated per
// run so no test credential is hardcoded. Hex is safe in both a URL and a SQL
// PASSWORD literal.
func randomPassword(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 32)
	_, err := rand.Read(buf)
	require.NoError(t, err)
	return hex.EncodeToString(buf)
}

// setupSelfHostedTestEnv provisions a postgres testcontainer with the
// teleport-admin user and roles the tests rely on.
func setupSelfHostedTestEnv(t *testing.T, postgresImage string) *testEnv {
	bootstrapPassword := randomPassword(t)
	adminPassword := randomPassword(t)

	pgContainer, err := postgres.Run(t.Context(), postgresImage,
		postgres.WithUsername("postgres"),
		postgres.WithPassword(bootstrapPassword),
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
		return connectAsUser(t, "postgres", bootstrapPassword, dbName)
	}

	// The engine connects as teleport-admin (CREATEROLE, not SUPERUSER). The
	// URI carries credentials because the connector prepends "postgres://"
	// before parsing.
	dbURI := fmt.Sprintf("teleport-admin:%s@localhost:%s", adminPassword, port)

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

	// bootstrapConn is the postgres superuser. All test setup and cleanup
	// runs through it.
	bootstrapConn, err := connectAsBootstrap(t, "postgres")
	require.NoError(t, err)
	t.Cleanup(func() { bootstrapConn.Close(context.Background()) })

	_, err = bootstrapConn.Exec(t.Context(), fmt.Sprintf(`
		-- Docs-prescribed admin: CREATEROLE + LOGIN.
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'teleport-admin') THEN
				CREATE USER "teleport-admin" LOGIN CREATEROLE PASSWORD '%s';
			END IF;
		END $$;

		-- postgres_fdw is bundled; install as superuser and grant USAGE so
		-- the foreign-table refused-case test can CREATE SERVER as
		-- teleport-admin.
		CREATE EXTENSION IF NOT EXISTS postgres_fdw;
		GRANT USAGE ON FOREIGN DATA WRAPPER postgres_fdw TO "teleport-admin";
	`, adminPassword))
	require.NoError(t, err)

	// teleport-auto-user is created lazily by the engine; drop it once here.
	cleanupSQL(t, bootstrapConn, `DROP ROLE IF EXISTS "teleport-auto-user"`)

	// CREATE DATABASE cannot be batched with other statements (it cannot
	// run inside a transaction block), so it and the DROP that makes it
	// idempotent across crashed runs each get their own Exec call.
	_, err = bootstrapConn.Exec(t.Context(), `DROP DATABASE IF EXISTS other_db WITH (FORCE)`)
	require.NoError(t, err)
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

	// adminConn is teleport-admin — the user under test. Assertions and (via
	// dbURI) the engine run through it, never scaffolding.
	adminConn, err := connectAsUser(t, "teleport-admin", adminPassword, "postgres")
	require.NoError(t, err)
	t.Cleanup(func() { adminConn.Close(context.Background()) })

	// otherDBConn is teleport-admin's peer for other_db (cross-db assertions).
	otherDBConn, err := connectAsUser(t, "teleport-admin", adminPassword, "other_db")
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
		userPassword:         randomPassword(t),
	}
}

// cleanupSQL runs statement via conn at test cleanup. Errors are logged, not
// fatal, so a noisy cleanup can't mask the test result. Uses
// context.Background() because t.Context() is already canceled by cleanup time.
func cleanupSQL(t *testing.T, conn *pgx.Conn, statement string) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), statement); err != nil {
			t.Logf("cleanup statement failed: %s: %v", statement, err)
		}
	})
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
				t.Parallel()
				env := setupSelfHostedTestEnv(t, postgresImage)
				runUserAutoProvisioningTests(t, env)
			})
		}
	})
	// TODO: add tests for RDS using procedure owned by role that is member of rds_superuser
}

func runUserAutoProvisioningTests(t *testing.T, env *testEnv) {

	t.Run("ActivateUser", func(t *testing.T) {

		t.Run("creates new user", func(t *testing.T) {
			username := "alice_new"
			err := env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil))
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.True(t, userExists(t, env.adminConn, username), "user should be created")
		})

		t.Run("reactivates deactivated user", func(t *testing.T) {
			username := "alice_reactivate"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.NoError(t, env.engine.DeactivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.False(t, canLogin(t, env.adminConn, username), "precondition: login should be disabled")
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, canLogin(t, env.adminConn, username), "user should be able to log in after reactivation")
		})

		t.Run("reactivation strips leftover roles", func(t *testing.T) {
			ensureRole(t, env.bootstrapConn, "leftover_role")
			_, err := env.bootstrapConn.Exec(t.Context(), `GRANT leftover_role TO "teleport-admin" WITH ADMIN OPTION`)
			require.NoError(t, err)
			username := "alice_leftover"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, []string{"leftover_role"})))
			require.True(t, isMember(t, env.adminConn, username, "leftover_role"), "precondition: user has the role")
			// Reactivating with no roles must strip roles not in the new set,
			// recovering from a crashed agent that left roles behind.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.False(t, isMember(t, env.adminConn, username, "leftover_role"),
				"reactivation should strip the leftover role")
		})

		t.Run("assigns roles", func(t *testing.T) {
			username := "alice_roles"
			ensureRole(t, env.bootstrapConn, "testrole")
			_, err := env.bootstrapConn.Exec(t.Context(), `GRANT testrole TO "teleport-admin" WITH ADMIN OPTION`)
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, []string{"testrole"})))
			require.True(t, isMember(t, env.adminConn, username, "testrole"), "user should be member of testrole")
		})

		t.Run("preserves teleport-auto-user on reactivation", func(t *testing.T) {
			username := "alice_preserve"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, isMember(t, env.adminConn, username, "teleport-auto-user"),
				"precondition: user should be member of teleport-auto-user")
			require.NoError(t, env.engine.DeactivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, isMember(t, env.adminConn, username, "teleport-auto-user"),
				"teleport-auto-user membership must survive deactivate")
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, isMember(t, env.adminConn, username, "teleport-auto-user"),
				"teleport-auto-user membership must survive deactivate/reactivate cycle")
		})

		t.Run("rejects pre-existing non-teleport user", func(t *testing.T) {
			username := "alice_external"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			_, err := env.bootstrapConn.Exec(t.Context(), fmt.Sprintf(`
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '%[1]s') THEN
						CREATE USER %[1]q;
					END IF;
				END $$`, username))
			require.NoError(t, err)
			err = env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil))
			require.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists error, got: %v", err)
		})

		t.Run("active connection same roles succeeds", func(t *testing.T) {
			username := "alice_active_same"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			_, err := env.bootstrapConn.Exec(t.Context(), fmt.Sprintf(`ALTER USER %q WITH PASSWORD '%s' LOGIN`, username, env.userPassword))
			require.NoError(t, err)
			userConn, err := env.connectAsUser(t, username, env.userPassword, "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())
			err = env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil))
			require.NoError(t, err)
		})

		t.Run("active connection different roles fails", func(t *testing.T) {
			ensureRole(t, env.bootstrapConn, "role_for_diff")

			username := "alice_active_diff"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			_, err := env.bootstrapConn.Exec(t.Context(), fmt.Sprintf(`ALTER USER %q WITH PASSWORD '%s' LOGIN`, username, env.userPassword))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, env.userPassword, "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			// ActivateUser with a new role while a connection is open should fail
			// because the active session's roles would need to change.
			err = env.engine.ActivateUser(t.Context(), makeSession(env.db, username, []string{"role_for_diff"}))
			require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got: %v", err)
		})

		t.Run("self-heals teleport-auto-user role", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `DROP ROLE IF EXISTS "teleport-auto-user"`)
			require.NoError(t, err)
			username := "alice_selfheal"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, isMember(t, env.adminConn, username, "teleport-auto-user"),
				"ActivateUser should recreate teleport-auto-user and add the user to it")
		})
	})

	t.Run("DeactivateUser", func(t *testing.T) {

		t.Run("strips roles and disables login", func(t *testing.T) {
			ensureRole(t, env.bootstrapConn, "testrole_deact")
			_, err := env.bootstrapConn.Exec(t.Context(), `GRANT testrole_deact TO "teleport-admin" WITH ADMIN OPTION`)
			require.NoError(t, err)

			username := "alice_deact_roles"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, []string{"testrole_deact"})))
			require.True(t, isMember(t, env.adminConn, username, "testrole_deact"), "precondition")
			require.True(t, canLogin(t, env.adminConn, username), "precondition")

			require.NoError(t, env.engine.DeactivateUser(t.Context(), makeSession(env.db, username, nil)))

			require.False(t, isMember(t, env.adminConn, username, "testrole_deact"),
				"non-teleport role should be revoked after deactivation")
			require.False(t, canLogin(t, env.adminConn, username),
				"login should be disabled after deactivation")
		})

		t.Run("is no-op when user has active connections", func(t *testing.T) {
			username := "alice_deact_active"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			_, err := env.bootstrapConn.Exec(t.Context(),
				fmt.Sprintf(`ALTER USER %q WITH PASSWORD '%s'`, username, env.userPassword))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, env.userPassword, "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			require.NoError(t, env.engine.DeactivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, canLogin(t, env.adminConn, username),
				"login should remain enabled while an active connection is open")
		})

		t.Run("preserves teleport-auto-user membership", func(t *testing.T) {
			username := "alice_deact_preserve"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.NoError(t, env.engine.DeactivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, isMember(t, env.adminConn, username, "teleport-auto-user"),
				"teleport-auto-user membership must be preserved after deactivation")
		})
	})

	t.Run("DeleteUser", func(t *testing.T) {

		t.Run("drops user with no owned objects", func(t *testing.T) {
			username := "alice_delete_clean"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeSession(env.db, username, nil)))
			require.False(t, userExists(t, env.adminConn, username), "user should be dropped")
		})

		t.Run("does not drop user with active connections", func(t *testing.T) {
			username := "alice_delete_active"
			cleanupSQL(t, env.bootstrapConn, fmt.Sprintf(`DROP USER IF EXISTS %q`, username))
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, username, nil)))
			_, err := env.bootstrapConn.Exec(t.Context(),
				fmt.Sprintf(`ALTER USER %q WITH PASSWORD '%s' LOGIN`, username, env.userPassword))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, username, env.userPassword, "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			require.NoError(t, env.engine.DeleteUser(t.Context(), makeSession(env.db, username, nil)))
			require.True(t, userExists(t, env.adminConn, username), "user should still exist")
			require.True(t, canLogin(t, env.adminConn, username), "user should still have login")
		})

		t.Run("deactivates instead of dropping user who owns objects", func(t *testing.T) {
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_delete_owns`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_delete_owns_tbl`)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeSession(env.db, "alice_delete_owns", nil)))
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_delete_owns_tbl (id int);
				ALTER TABLE alice_delete_owns_tbl OWNER TO "alice_delete_owns"`)
			require.NoError(t, err)

			// db has no OrphanedResourceOwner, so reassignment is skipped; the user
			// cannot be dropped and is deactivated instead.
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeSession(env.db, "alice_delete_owns", nil)))
			require.True(t, userExists(t, env.adminConn, "alice_delete_owns"),
				"user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, "alice_delete_owns"),
				"user should have login disabled")
		})
	})

	t.Run("DeleteUser with reassignment", func(t *testing.T) {
		installReassignObjectsProcedure(t, env.bootstrapConn)
		ensureRole(t, env.bootstrapConn, "teleport-object-inheritor")

		t.Run("Reassign: bare table", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_bare", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_bare`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_bare_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_bare_tbl (id int);
				ALTER TABLE alice_re_bare_tbl OWNER TO "alice_re_bare"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_bare", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_bare"), "user should be dropped after reassignment")
			require.Equal(t, "teleport-object-inheritor", tableOwner(t, env.adminConn, "public", "alice_re_bare_tbl"))
		})

		t.Run("Reassign: table with index", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_idx", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_idx`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_idx_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_idx_tbl (id int);
				CREATE INDEX IF NOT EXISTS alice_re_idx_tbl_idx ON alice_re_idx_tbl (id);
				ALTER TABLE alice_re_idx_tbl OWNER TO "alice_re_idx"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_idx", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_idx"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: table with identity sequence", func(t *testing.T) {
			// Identity columns create OWNED BY sequences. ALTER TABLE OWNER TO
			// does not propagate to such sequences, so the sequence loop in
			// reassign-objects.sql is what moves them.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_idseq", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_idseq`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_idseq_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_idseq_tbl (id int GENERATED ALWAYS AS IDENTITY);
				ALTER TABLE alice_re_idseq_tbl OWNER TO "alice_re_idseq";
				ALTER SEQUENCE alice_re_idseq_tbl_id_seq OWNER TO "alice_re_idseq"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_idseq", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_idseq"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: composite row type follows", func(t *testing.T) {
			// Every regular table has an auto-generated composite row type in
			// pg_type (typtype='c', typrelid pointing at the table). Verify
			// must exclude these or it would always fire.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_rowtype", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_rowtype`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_rowtype_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_rowtype_tbl (a int, b text, c timestamptz);
				ALTER TABLE alice_re_rowtype_tbl OWNER TO "alice_re_rowtype"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_rowtype", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_rowtype"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: array type follows", func(t *testing.T) {
			// Every table also has an auto-generated array type (typname
			// '_<tbl>'). Verify must exclude these; they are recognized by
			// catalog linkage (the row type's typarray points at the array
			// type), not by the '_' name prefix.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_arr", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_arr`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_arr_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_arr_tbl (vals int[]);
				ALTER TABLE alice_re_arr_tbl OWNER TO "alice_re_arr"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_arr", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_arr"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: FK constraint internal triggers are ignored", func(t *testing.T) {
			// Foreign-key constraints add pg_trigger rows with
			// tgisinternal=true. The safety guard ignores internal triggers,
			// so the tables are still reassigned.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_fk", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_fk`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_re_fk_child;
				DROP TABLE IF EXISTS alice_re_fk_parent;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_fk_parent (id int PRIMARY KEY);
				CREATE TABLE IF NOT EXISTS alice_re_fk_child (id int REFERENCES alice_re_fk_parent (id));
				ALTER TABLE alice_re_fk_parent OWNER TO "alice_re_fk";
				ALTER TABLE alice_re_fk_child  OWNER TO "alice_re_fk"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_fk", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_fk"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: standalone sequence", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_seq", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_seq`)
			cleanupSQL(t, env.bootstrapConn, `DROP SEQUENCE IF EXISTS alice_re_seq_seq`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE SEQUENCE IF NOT EXISTS alice_re_seq_seq;
				ALTER SEQUENCE alice_re_seq_seq OWNER TO "alice_re_seq"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_seq", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_seq"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: mixed safe types", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_mix", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_mix`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_re_mix_tbl;
				DROP SEQUENCE IF EXISTS alice_re_mix_seq;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_mix_tbl (id int);
				CREATE SEQUENCE IF NOT EXISTS alice_re_mix_seq;
				ALTER TABLE alice_re_mix_tbl OWNER TO "alice_re_mix";
				ALTER SEQUENCE alice_re_mix_seq OWNER TO "alice_re_mix"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_mix", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_mix"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: unlogged table", func(t *testing.T) {
			// relpersistence 'u' (unlogged) is allowed; only temp ('t') is
			// rejected, and a temp table cannot survive to reassignment anyway.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_unlogged", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_unlogged`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_unlogged_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE UNLOGGED TABLE IF NOT EXISTS alice_re_unlogged_tbl (id int);
				ALTER TABLE alice_re_unlogged_tbl OWNER TO "alice_re_unlogged"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_unlogged", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_unlogged"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: table with primary key and unique constraints", func(t *testing.T) {
			// PRIMARY KEY and UNIQUE constraints (contype 'p'/'u') are allowed;
			// their backing indexes are plain and use built-in operator classes.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_uniq", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_uniq`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_uniq_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_uniq_tbl (id int PRIMARY KEY, email text UNIQUE);
				ALTER TABLE alice_re_uniq_tbl OWNER TO "alice_re_uniq"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_uniq", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_uniq"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: bare table with a grant-option chain", func(t *testing.T) {
			// The user can plant a grant-option dependency chain on a table that
			// otherwise passes the whitelist: grant a privilege WITH GRANT OPTION
			// to a role it can act as (teleport-auto-user, which every managed
			// user is a member of), then grant onward as that role. A plain
			// REVOKE during reassignment would error with "dependent privileges
			// exist" and abort; the REVOKE ... CASCADE removes the dependent
			// grants too, so the table is still reassigned and the user dropped.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_goc", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_goc`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_goc_tbl`)
			// bootstrapConn is the superuser, so it can SET ROLE to either role
			// to make the grantors match what the user itself could produce.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_goc_tbl (id int);
				ALTER TABLE alice_re_goc_tbl OWNER TO "alice_re_goc";
				SET ROLE "alice_re_goc";
				GRANT SELECT ON alice_re_goc_tbl TO "teleport-auto-user" WITH GRANT OPTION;
				SET ROLE "teleport-auto-user";
				GRANT SELECT ON alice_re_goc_tbl TO PUBLIC;
				RESET ROLE;`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_goc", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_goc"), "user should be dropped after reassignment")
			require.Equal(t, "teleport-object-inheritor", tableOwner(t, env.adminConn, "public", "alice_re_goc_tbl"))
		})

		t.Run("Refused: partition child", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_pchild", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_pchild`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_pchild_p1;
				DROP TABLE IF EXISTS alice_no_pchild_parent;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_pchild_parent (id int) PARTITION BY RANGE (id);
				CREATE TABLE IF NOT EXISTS alice_no_pchild_p1 PARTITION OF alice_no_pchild_parent
					FOR VALUES FROM (1) TO (10);
				ALTER TABLE alice_no_pchild_p1 OWNER TO "alice_no_pchild";`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_pchild", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_pchild")
		})

		t.Run("Refused: legacy inheritance child", func(t *testing.T) {
			// Legacy CREATE TABLE ... INHERITS creates a pg_inherits row just like
			// declarative partitioning; the inheritance guard rejects either.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_inh", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_inh`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_inh_child;
				DROP TABLE IF EXISTS alice_no_inh_parent;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_inh_parent (id int);
				CREATE TABLE IF NOT EXISTS alice_no_inh_child (extra int) INHERITS (alice_no_inh_parent);
				ALTER TABLE alice_no_inh_child OWNER TO "alice_no_inh"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_inh", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_inh")
		})

		t.Run("Refused: partitioned parent", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_pparent", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_pparent`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_pparent_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_pparent_tbl (id int) PARTITION BY RANGE (id);
				ALTER TABLE alice_no_pparent_tbl OWNER TO "alice_no_pparent"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_pparent", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_pparent")
		})

		t.Run("Refused: table with forced row-level security", func(t *testing.T) {
			// FORCE ROW LEVEL SECURITY sets relforcerowsecurity independently of
			// the ENABLE flag (relrowsecurity); the guard rejects either.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_frls", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_frls`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_frls_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_frls_tbl (id int);
				ALTER TABLE alice_no_frls_tbl FORCE ROW LEVEL SECURITY;
				ALTER TABLE alice_no_frls_tbl OWNER TO "alice_no_frls"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_frls", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_frls")
		})

		t.Run("Refused: table with row-level security", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_rls", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_rls`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_rls_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_rls_tbl (id int);
				ALTER TABLE alice_no_rls_tbl ENABLE ROW LEVEL SECURITY;
				ALTER TABLE alice_no_rls_tbl OWNER TO "alice_no_rls"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_rls", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_rls")
		})

		t.Run("Refused: table with SECURITY INVOKER trigger", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_tinv", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_tinv`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_tinv_tbl;
				DROP FUNCTION IF EXISTS alice_no_tinv_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_tinv_fn() RETURNS trigger LANGUAGE plpgsql AS $$
					BEGIN RETURN NEW; END $$;
				CREATE TABLE IF NOT EXISTS alice_no_tinv_tbl (id int);
				CREATE OR REPLACE TRIGGER alice_no_tinv_tg BEFORE INSERT ON alice_no_tinv_tbl
					FOR EACH ROW EXECUTE FUNCTION alice_no_tinv_fn();
				ALTER TABLE alice_no_tinv_tbl OWNER TO "alice_no_tinv"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_tinv", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_tinv")
		})

		t.Run("Refused: table with SECURITY DEFINER trigger", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_tdef", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_tdef`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_tdef_tbl;
				DROP FUNCTION IF EXISTS alice_no_tdef_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_tdef_fn() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS $$
					BEGIN RETURN NEW; END $$;
				CREATE TABLE IF NOT EXISTS alice_no_tdef_tbl (id int);
				CREATE OR REPLACE TRIGGER alice_no_tdef_tg BEFORE INSERT ON alice_no_tdef_tbl
					FOR EACH ROW EXECUTE FUNCTION alice_no_tdef_fn();
				ALTER TABLE alice_no_tdef_tbl OWNER TO "alice_no_tdef"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_tdef", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_tdef")
		})

		t.Run("Refused: table with rewrite rule", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_rule", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_rule`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_rule_tbl`)
			// A DML rewrite rule runs with the table owner's privileges for
			// referenced relations, so a rule-bearing table must not be
			// reassigned to the inheritor.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_rule_tbl (id int);
				CREATE OR REPLACE RULE alice_no_rule_r AS ON INSERT TO alice_no_rule_tbl DO INSTEAD NOTHING;
				ALTER TABLE alice_no_rule_tbl OWNER TO "alice_no_rule"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_rule", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_rule")
		})

		t.Run("Refused: table with a policy", func(t *testing.T) {
			// A policy can exist while row-level security is disabled (dormant),
			// so relrowsecurity is false here and the relrowsecurity guard does
			// not fire; the pg_policy guard is what refuses the table.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_pol", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_pol`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_pol_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_pol_tbl (id int);
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_policies WHERE tablename = 'alice_no_pol_tbl' AND policyname = 'alice_no_pol_p') THEN
						CREATE POLICY alice_no_pol_p ON alice_no_pol_tbl USING (true);
					END IF;
				END $$;
				ALTER TABLE alice_no_pol_tbl OWNER TO "alice_no_pol"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_pol", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_pol")
		})

		t.Run("Refused: table with expression index on a user function", func(t *testing.T) {
			// ANALYZE evaluates an expression index with the table owner's
			// identity, so a table indexing a non-system function must not be
			// reassigned to the inheritor. cleanupSQL runs LIFO, so the table
			// (registered last) is dropped before the function it depends on.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_exidx", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_exidx`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_exidx_tbl;
				DROP FUNCTION IF EXISTS alice_no_exidx_fn(int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_exidx_fn(i int) RETURNS int LANGUAGE sql IMMUTABLE AS 'SELECT i + 1';
				CREATE TABLE IF NOT EXISTS alice_no_exidx_tbl (id int);
				CREATE INDEX IF NOT EXISTS alice_no_exidx_idx ON alice_no_exidx_tbl (alice_no_exidx_fn(id));
				ALTER TABLE alice_no_exidx_tbl OWNER TO "alice_no_exidx"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_exidx", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_exidx")
		})

		t.Run("Refused: table with a partial index", func(t *testing.T) {
			// A partial index carries a WHERE predicate (pg_index.indpred), which
			// the index guard rejects even when columns and opclass are built-in.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_pidx", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_pidx`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_pidx_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_pidx_tbl (id int);
				CREATE INDEX IF NOT EXISTS alice_no_pidx_idx ON alice_no_pidx_tbl (id) WHERE id > 0;
				ALTER TABLE alice_no_pidx_tbl OWNER TO "alice_no_pidx"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_pidx", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_pidx")
		})

		t.Run("Refused: table with default calling a user function", func(t *testing.T) {
			// Any column default disqualifies the table; the whitelist requires
			// pg_attrdef to be empty. This user-function default is one instance;
			// the built-in-default case nearby covers ordinary defaults like now().
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_def", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_def`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_def_tbl;
				DROP FUNCTION IF EXISTS alice_no_def_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_def_fn() RETURNS int LANGUAGE sql AS 'SELECT 42';
				CREATE TABLE IF NOT EXISTS alice_no_def_tbl (id int DEFAULT alice_no_def_fn());
				ALTER TABLE alice_no_def_tbl OWNER TO "alice_no_def"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_def", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_def")
		})

		t.Run("Refused: table with check constraint using a user operator", func(t *testing.T) {
			// A non-system operator runs its oprcode with the table owner's
			// identity; its dependency is on pg_operator, not pg_proc.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_op", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_op`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_op_tbl;
				DROP OPERATOR IF EXISTS === (int, int);
				DROP FUNCTION IF EXISTS alice_no_op_fn(int, int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_op_fn(a int, b int) RETURNS bool LANGUAGE sql IMMUTABLE AS 'SELECT a > b';
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_operator WHERE oprname = '===' AND oprleft = 'int4'::regtype AND oprright = 'int4'::regtype) THEN
						CREATE OPERATOR === (LEFTARG = int, RIGHTARG = int, FUNCTION = alice_no_op_fn);
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_op_tbl (id int, CONSTRAINT alice_no_op_ck CHECK (id === 0));
				ALTER TABLE alice_no_op_tbl OWNER TO "alice_no_op"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_op", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_op")
		})

		t.Run("Refused: table with check constraint casting to a user type", func(t *testing.T) {
			// A custom type in an expression can run custom I/O or cast
			// functions with the table owner's identity; its dependency is on
			// pg_type, not pg_proc.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_typ", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_typ`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_typ_tbl;
				DROP TYPE IF EXISTS alice_no_typ_e;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_typ_e') THEN
						CREATE TYPE alice_no_typ_e AS ENUM ('x', 'y');
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_typ_tbl (v text, CONSTRAINT alice_no_typ_ck CHECK (v::alice_no_typ_e = 'x'));
				ALTER TABLE alice_no_typ_tbl OWNER TO "alice_no_typ"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_typ", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_typ")
		})

		t.Run("Refused: table with extended statistics on a user function", func(t *testing.T) {
			// Expression statistics are evaluated during ANALYZE with the table
			// owner's identity; the dependency is on pg_statistic_ext. Expression
			// statistics require PostgreSQL 14+ (our minimum).
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_stx", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_stx`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_stx_tbl;
				DROP FUNCTION IF EXISTS alice_no_stx_fn(int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_stx_fn(i int) RETURNS int LANGUAGE sql IMMUTABLE AS 'SELECT i + 1';
				CREATE TABLE IF NOT EXISTS alice_no_stx_tbl (a int, b int);
				CREATE STATISTICS IF NOT EXISTS alice_no_stx_s ON (alice_no_stx_fn(a)) FROM alice_no_stx_tbl;
				ALTER TABLE alice_no_stx_tbl OWNER TO "alice_no_stx"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_stx", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_stx")
		})

		t.Run("Refused: table with a column of a user domain", func(t *testing.T) {
			// A domain's CHECK runs on every cast to it, in the writer's
			// context. Its function dependency lives on the domain, reached
			// through the column's type rather than a table expression, so the
			// built-in-only column-type guard is what catches it.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_dom", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_dom`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_dom_tbl;
				DROP DOMAIN IF EXISTS alice_no_dom_d;
				DROP FUNCTION IF EXISTS alice_no_dom_fn(int);
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_dom_fn(i int) RETURNS bool LANGUAGE sql IMMUTABLE AS 'SELECT i >= 0';
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_dom_d') THEN
						CREATE DOMAIN alice_no_dom_d AS int CHECK (alice_no_dom_fn(VALUE));
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_dom_tbl (id alice_no_dom_d);
				ALTER TABLE alice_no_dom_tbl OWNER TO "alice_no_dom"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_dom", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_dom")
		})

		t.Run("Refused: table with a composite-type column", func(t *testing.T) {
			// The built-in-only-column guard rejects a column of a user composite
			// type, whose I/O can run code as the owner, like the enum/domain cases.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_ctcol", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_ctcol`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_ctcol_tbl;
				DROP TYPE IF EXISTS alice_no_ctcol_t;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_ctcol_t') THEN
						CREATE TYPE alice_no_ctcol_t AS (a int, b text);
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_ctcol_tbl (val alice_no_ctcol_t);
				ALTER TABLE alice_no_ctcol_tbl OWNER TO "alice_no_ctcol"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_ctcol", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_ctcol")
		})

		t.Run("Refused: table with a user enum column", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_enumcol", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_enumcol`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_enumcol_tbl;
				DROP TYPE IF EXISTS alice_no_enumcol_e;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_enumcol_e') THEN
						CREATE TYPE alice_no_enumcol_e AS ENUM ('a', 'b');
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_enumcol_tbl (status alice_no_enumcol_e);
				ALTER TABLE alice_no_enumcol_tbl OWNER TO "alice_no_enumcol"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_enumcol", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_enumcol")
		})

		t.Run("Refused: table with a built-in default", func(t *testing.T) {
			// The whitelist rejects ALL column defaults, even built-in ones such as
			// now(); pg_attrdef must be empty.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_bdef", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_bdef`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_bdef_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_bdef_tbl (id int, created timestamptz DEFAULT now());
				ALTER TABLE alice_no_bdef_tbl OWNER TO "alice_no_bdef"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_bdef", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_bdef")
		})

		t.Run("Refused: table with a built-in check constraint", func(t *testing.T) {
			// The whitelist rejects ALL CHECK constraints (contype 'c'), even those
			// using only built-in operators.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_bck", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_bck`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_bck_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_bck_tbl (id int, CONSTRAINT alice_no_bck_ck CHECK (id > 0));
				ALTER TABLE alice_no_bck_tbl OWNER TO "alice_no_bck"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_bck", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_bck")
		})

		t.Run("Refused: table with a generated column", func(t *testing.T) {
			// A generated column's expression lives in pg_attrdef (attgenerated='s')
			// and is evaluated with the owner's identity.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_gen", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_gen`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_gen_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_gen_tbl (id int, doubled int GENERATED ALWAYS AS (id * 2) STORED);
				ALTER TABLE alice_no_gen_tbl OWNER TO "alice_no_gen"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_gen", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_gen")
		})

		t.Run("Refused: table with an exclusion constraint", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_excl", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_excl`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_no_excl_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_no_excl_tbl (during int4range, EXCLUDE USING gist (during WITH &&));
				ALTER TABLE alice_no_excl_tbl OWNER TO "alice_no_excl"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_excl", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_excl")
		})

		t.Run("Refused: typed table", func(t *testing.T) {
			// A table defined OF a composite type (reloftype <> 0) is rejected: it is
			// bound to a type that can carry behavior.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_typed", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_typed`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_typed_tbl;
				DROP TYPE IF EXISTS alice_no_typed_t;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_typed_t') THEN
						CREATE TYPE alice_no_typed_t AS (a int, b text);
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_typed_tbl OF alice_no_typed_t;
				ALTER TABLE alice_no_typed_tbl OWNER TO "alice_no_typed"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_typed", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_typed")
		})

		t.Run("Refused: index using a user-defined operator class", func(t *testing.T) {
			// A plain index over a built-in column can still bind a user-defined
			// operator class, whose support functions run custom code.
			var isSuper bool
			require.NoError(t, env.bootstrapConn.QueryRow(t.Context(),
				"SELECT rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&isSuper))
			if !isSuper {
				t.Skip("CREATE OPERATOR CLASS requires superuser")
			}
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_opc", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_opc`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_no_opc_tbl;
				DROP OPERATOR CLASS IF EXISTS alice_no_opc_ops USING btree;
			`)
			// The opclass reuses built-in operators and btint4cmp but lives outside
			// pg_catalog, so the built-in-opclass guard (not the expr/predicate guard)
			// is what refuses the plain index.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_opclass WHERE opcname = 'alice_no_opc_ops' AND opcmethod = (SELECT oid FROM pg_am WHERE amname = 'btree')) THEN
						CREATE OPERATOR CLASS alice_no_opc_ops FOR TYPE int4 USING btree AS
							OPERATOR 1 <, OPERATOR 2 <=, OPERATOR 3 =, OPERATOR 4 >=, OPERATOR 5 >,
							FUNCTION 1 btint4cmp(int4, int4);
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS alice_no_opc_tbl (id int);
				CREATE INDEX IF NOT EXISTS alice_no_opc_idx ON alice_no_opc_tbl (id alice_no_opc_ops);
				ALTER TABLE alice_no_opc_tbl OWNER TO "alice_no_opc"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_opc", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_opc")
		})

		t.Run("Refused: function owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_fn", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_fn`)
			cleanupSQL(t, env.bootstrapConn, `DROP FUNCTION IF EXISTS alice_no_fn_fn()`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE FUNCTION alice_no_fn_fn() RETURNS int LANGUAGE sql AS 'SELECT 1';
				ALTER FUNCTION alice_no_fn_fn() OWNER TO "alice_no_fn"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_fn", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_fn")
		})

		t.Run("Refused: view owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_view", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_view`)
			cleanupSQL(t, env.bootstrapConn, `DROP VIEW IF EXISTS alice_no_view_v`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE OR REPLACE VIEW alice_no_view_v AS SELECT 1 AS x;
				ALTER VIEW alice_no_view_v OWNER TO "alice_no_view"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_view", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_view")
		})

		t.Run("Refused: materialized view owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_mv", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_mv`)
			cleanupSQL(t, env.bootstrapConn, `DROP MATERIALIZED VIEW IF EXISTS alice_no_mv_mv`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE MATERIALIZED VIEW IF NOT EXISTS alice_no_mv_mv AS SELECT 1 AS x;
				ALTER MATERIALIZED VIEW alice_no_mv_mv OWNER TO "alice_no_mv"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_mv", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_mv")
		})

		t.Run("Refused: foreign table owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_ft", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_ft`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP FOREIGN TABLE IF EXISTS alice_no_ft_ft;
				DROP SERVER IF EXISTS alice_no_ft_srv;
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE EXTENSION IF NOT EXISTS postgres_fdw;
				CREATE SERVER IF NOT EXISTS alice_no_ft_srv FOREIGN DATA WRAPPER postgres_fdw;
				CREATE FOREIGN TABLE IF NOT EXISTS alice_no_ft_ft (id int) SERVER alice_no_ft_srv;
				ALTER FOREIGN TABLE alice_no_ft_ft OWNER TO "alice_no_ft"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_ft", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_ft")
		})

		t.Run("Refused: standalone composite type", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_ct", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_ct`)
			cleanupSQL(t, env.bootstrapConn, `DROP TYPE IF EXISTS alice_no_ct_t`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_ct_t') THEN
						CREATE TYPE alice_no_ct_t AS (a int, b text);
					END IF;
				END $$;
				ALTER TYPE alice_no_ct_t OWNER TO "alice_no_ct"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_ct", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_ct")
		})

		t.Run("Refused: domain owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_dom", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_dom`)
			cleanupSQL(t, env.bootstrapConn, `DROP DOMAIN IF EXISTS alice_no_dom_d`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_dom_d') THEN
						CREATE DOMAIN alice_no_dom_d AS int;
					END IF;
				END $$;
				ALTER DOMAIN alice_no_dom_d OWNER TO "alice_no_dom"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_dom", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_dom")
		})

		t.Run("Refused: enum type owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_enum", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_enum`)
			cleanupSQL(t, env.bootstrapConn, `DROP TYPE IF EXISTS alice_no_enum_e`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_type WHERE typname = 'alice_no_enum_e') THEN
						CREATE TYPE alice_no_enum_e AS ENUM ('a', 'b', 'c');
					END IF;
				END $$;
				ALTER TYPE alice_no_enum_e OWNER TO "alice_no_enum"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_enum", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_enum")
		})

		t.Run("Refused: schema owned by user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_no_sch", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_no_sch`)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS alice_no_sch_s`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE SCHEMA IF NOT EXISTS alice_no_sch_s;
				ALTER SCHEMA alice_no_sch_s OWNER TO "alice_no_sch"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_no_sch", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_no_sch")
		})

		t.Run("Refused: source user is not a Teleport user", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_member", nil)))
			cleanupSQL(t, env.bootstrapConn, `
				DROP USER IF EXISTS alice_member;
				DROP USER IF EXISTS non_teleport_user;
			`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS non_teleport_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DO $$ BEGIN
					IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'non_teleport_user') THEN
						CREATE USER non_teleport_user;
					END IF;
				END $$;
				CREATE TABLE IF NOT EXISTS non_teleport_tbl (id int);
				ALTER TABLE non_teleport_tbl OWNER TO non_teleport_user`)
			require.NoError(t, err)

			_, err = env.adminConn.Exec(t.Context(),
				`CALL teleport_objects.teleport_reassign_objects('non_teleport_user', 'teleport-object-inheritor')`)
			require.ErrorContains(t, err, "not a Teleport-managed user")
			require.Equal(t, "non_teleport_user", tableOwner(t, env.adminConn, "public", "non_teleport_tbl"),
				"non-teleport user's table must not be reassigned")
		})

		t.Run("Refused: source user does not exist", func(t *testing.T) {
			_, err := env.adminConn.Exec(t.Context(),
				`CALL teleport_objects.teleport_reassign_objects('alice_does_not_exist', 'teleport-object-inheritor')`)
			require.ErrorContains(t, err, "not found")
		})

		t.Run("Refused: destination is not teleport-object-inheritor", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_baddest", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_baddest`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_baddest_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_baddest_tbl (id int);
				ALTER TABLE alice_re_baddest_tbl OWNER TO "alice_re_baddest"`)
			require.NoError(t, err)

			_, err = env.adminConn.Exec(t.Context(),
				`CALL teleport_objects.teleport_reassign_objects('alice_re_baddest', 'teleport-admin')`)
			require.ErrorContains(t, err, "destination_user must be teleport-object-inheritor")
			require.Equal(t, "alice_re_baddest", tableOwner(t, env.adminConn, "public", "alice_re_baddest_tbl"),
				"object must not be reassigned when the destination is rejected")
		})

		t.Run("Refused: mixed reassignable + non-reassignable rolls back", func(t *testing.T) {
			// alice owns a bare table (would be reassigned) AND a function
			// (which the procedure does not touch, and verify catches).
			// The table must still be owned by alice after the failed delete.
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_atomic", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_atomic`)
			cleanupSQL(t, env.bootstrapConn, `
				DROP TABLE IF EXISTS alice_atomic_tbl;
				DROP FUNCTION IF EXISTS alice_atomic_fn();
			`)
			_, err := env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_atomic_tbl (id int);
				ALTER TABLE alice_atomic_tbl OWNER TO "alice_atomic";
				CREATE OR REPLACE FUNCTION alice_atomic_fn() RETURNS int LANGUAGE sql AS 'SELECT 1';
				ALTER FUNCTION alice_atomic_fn() OWNER TO "alice_atomic"`)
			require.NoError(t, err)

			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_atomic", nil)))
			require.True(t, userExists(t, env.adminConn, "alice_atomic"), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, "alice_atomic"), "user should have login disabled")
			require.Equal(t, "alice_atomic", tableOwner(t, env.adminConn, "public", "alice_atomic_tbl"),
				"table ownership must roll back when reassignment is refused")
		})

		t.Run("Reassign: login attempt during reassignment is rejected", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_login_race", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_login_race`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_login_race_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), fmt.Sprintf(`
				ALTER USER "alice_login_race" WITH PASSWORD '%s' LOGIN;
				CREATE TABLE IF NOT EXISTS alice_login_race_tbl (id int);
				ALTER TABLE alice_login_race_tbl OWNER TO "alice_login_race"`, env.userPassword))
			require.NoError(t, err)

			// Hold AccessShare on the user's table from a fresh superuser
			// connection.
			lockConn, err := env.connectAsBootstrap(t, "postgres")
			require.NoError(t, err)
			defer lockConn.Close(context.Background())
			tx, err := lockConn.Begin(t.Context())
			require.NoError(t, err)
			_, err = tx.Exec(t.Context(), `LOCK TABLE alice_login_race_tbl IN ACCESS SHARE MODE`)
			require.NoError(t, err)

			deleteDone := make(chan error, 1)
			go func() {
				deleteDone <- env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_login_race", nil))
			}()

			// Wait until DeleteUser's backend is waiting on the relation lock.
			lockPID := lockConn.PgConn().PID()
			require.Eventually(t, func() bool {
				var n int
				if err := env.adminConn.QueryRow(t.Context(), `
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
			_, loginErr := env.connectAsUser(t, "alice_login_race", env.userPassword, "postgres")
			require.Error(t, loginErr, "login should be rejected while NOLOGIN is in effect")
			require.Contains(t, loginErr.Error(), "not permitted to log in")

			// Release the lock; DeleteUser completes and drops the user.
			require.NoError(t, tx.Commit(t.Context()))
			require.NoError(t, <-deleteDone)
			require.False(t, userExists(t, env.adminConn, "alice_login_race"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: current-db active session bails before reassign", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_active", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_active`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_active_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), fmt.Sprintf(`
				ALTER USER "alice_re_active" WITH PASSWORD '%s' LOGIN;
				CREATE TABLE IF NOT EXISTS alice_re_active_tbl (id int);
				ALTER TABLE alice_re_active_tbl OWNER TO "alice_re_active"`, env.userPassword))
			require.NoError(t, err)

			userConn, err := env.connectAsUser(t, "alice_re_active", env.userPassword, "postgres")
			require.NoError(t, err)
			defer userConn.Close(context.Background())

			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_active", nil)))
			require.True(t, userExists(t, env.adminConn, "alice_re_active"))
			require.True(t, canLogin(t, env.adminConn, "alice_re_active"),
				"LOGIN should be restored when an active current-db session blocks delete")
			// Table ownership untouched: reassignment never ran.
			require.Equal(t, "alice_re_active", tableOwner(t, env.adminConn, "public", "alice_re_active_tbl"))
		})

		t.Run("Reassign: user owns nothing", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_clean", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_clean`)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_clean", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_re_clean"), "user should be dropped after reassignment")
		})

		t.Run("Reassign: other-db active session restores LOGIN", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_otherdb", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_otherdb`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_re_otherdb_tbl`)
			_, err := env.bootstrapConn.Exec(t.Context(), fmt.Sprintf(`
				ALTER USER "alice_re_otherdb" WITH PASSWORD '%s' LOGIN;
				CREATE TABLE IF NOT EXISTS alice_re_otherdb_tbl (id int);
				ALTER TABLE alice_re_otherdb_tbl OWNER TO "alice_re_otherdb"`, env.userPassword))
			require.NoError(t, err)

			// Hold a session in other_db. CONNECT to a newly-created database
			// is granted to PUBLIC by default, so no extra GRANT is needed.
			otherSessionConn, err := env.connectAsUser(t, "alice_re_otherdb", env.userPassword, "other_db")
			require.NoError(t, err)
			defer otherSessionConn.Close(context.Background())

			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_otherdb", nil)))
			require.True(t, userExists(t, env.adminConn, "alice_re_otherdb"))
			require.True(t, canLogin(t, env.adminConn, "alice_re_otherdb"),
				"LOGIN should be restored when an other-db session blocks drop")
			// Distinct from the current-db case: reassignment ran and committed
			// before the second active-sessions check fired.
			require.Equal(t, "teleport-object-inheritor",
				tableOwner(t, env.adminConn, "public", "alice_re_otherdb_tbl"))
		})

		t.Run("Refused: object in another database is ignored by dbid filter", func(t *testing.T) {
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_re_otherobj", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_re_otherobj`)
			// The object alice owns lives in other_db, so its DROP TABLE
			// runs through bootstrapOtherDBConn. LIFO orders this drop
			// before the DROP USER above so the cluster-wide pg_shdepend
			// row is gone by the time DROP USER fires.
			cleanupSQL(t, env.bootstrapOtherDBConn, `DROP TABLE IF EXISTS alice_re_otherobj_tbl`)
			// Create an object owned by alice in other_db. Reassignment in
			// postgres-db must not touch it (dbid filter), and verify must not
			// flag it.
			_, err := env.bootstrapOtherDBConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_re_otherobj_tbl (id int);
				ALTER TABLE alice_re_otherobj_tbl OWNER TO "alice_re_otherobj"`)
			require.NoError(t, err)

			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_re_otherobj", nil)))
			// reassign-objects.sql returned cleanly because the dbid filter
			// excluded the other-db object. DROP USER then failed because the
			// cluster-wide pg_shdepend lookup still sees the row, so alice was
			// deactivated.
			require.True(t, userExists(t, env.adminConn, "alice_re_otherobj"), "user should still exist (deactivated, not dropped)")
			require.False(t, canLogin(t, env.adminConn, "alice_re_otherobj"), "user should have login disabled")
			// The other-db object is untouched.
			require.Equal(t, "alice_re_otherobj",
				tableOwner(t, env.otherDBConn, "public", "alice_re_otherobj_tbl"))
		})
	})

	t.Run("DeleteUser reassignment: misconfigured or custom procedure", func(t *testing.T) {
		t.Run("Procedure absent: object-owning user is deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, err)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_noproc_owns", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_noproc_owns`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_noproc_owns_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_noproc_owns_tbl (id int);
				ALTER TABLE alice_noproc_owns_tbl OWNER TO "alice_noproc_owns"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_noproc_owns", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_noproc_owns")
		})

		t.Run("Procedure absent: user owning nothing is still dropped", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, err)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_noproc_clean", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_noproc_clean`)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_noproc_clean", nil)))
			require.False(t, userExists(t, env.adminConn, "alice_noproc_clean"), "user should be dropped")
		})

		t.Run("Schema present but procedure missing: object-owning user is deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DROP SCHEMA IF EXISTS teleport_objects CASCADE;
				CREATE SCHEMA teleport_objects`)
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_noproc_sch", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_noproc_sch`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_noproc_sch_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_noproc_sch_tbl (id int);
				ALTER TABLE alice_noproc_sch_tbl OWNER TO "alice_noproc_sch"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_noproc_sch", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_noproc_sch")
		})

		t.Run("Inheritor role absent: reassignable table is not reassigned, user deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, err)
			installReassignObjectsProcedure(t, env.bootstrapConn)
			_, err = env.bootstrapConn.Exec(t.Context(), `DROP ROLE IF EXISTS "teleport-object-inheritor"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_norole", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_norole`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_norole_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_norole_tbl (id int);
				ALTER TABLE alice_norole_tbl OWNER TO "alice_norole"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_norole", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_norole")
		})

		t.Run("Custom no-op procedure: object-owning user is deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DROP SCHEMA IF EXISTS teleport_objects CASCADE;
				CREATE SCHEMA teleport_objects;
				CREATE PROCEDURE teleport_objects.teleport_reassign_objects(source_user varchar, destination_user varchar)
					LANGUAGE plpgsql AS $$ BEGIN END $$;
				GRANT USAGE ON SCHEMA teleport_objects TO "teleport-admin";
				GRANT EXECUTE ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) TO "teleport-admin"`)
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_custom_noop", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_custom_noop`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_custom_noop_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_custom_noop_tbl (id int);
				ALTER TABLE alice_custom_noop_tbl OWNER TO "alice_custom_noop"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_custom_noop", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_custom_noop")
		})

		t.Run("Custom procedure with wrong signature: object-owning user is deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DROP SCHEMA IF EXISTS teleport_objects CASCADE;
				CREATE SCHEMA teleport_objects;
				CREATE PROCEDURE teleport_objects.teleport_reassign_objects(source_user varchar)
					LANGUAGE plpgsql AS $$ BEGIN END $$;
				GRANT USAGE ON SCHEMA teleport_objects TO "teleport-admin"`)
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_custom_sig", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_custom_sig`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_custom_sig_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_custom_sig_tbl (id int);
				ALTER TABLE alice_custom_sig_tbl OWNER TO "alice_custom_sig"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_custom_sig", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_custom_sig")
		})

		t.Run("teleport-admin lacks USAGE on schema: object-owning user is deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, err)
			installReassignObjectsProcedure(t, env.bootstrapConn)
			_, err = env.bootstrapConn.Exec(t.Context(), `REVOKE USAGE ON SCHEMA teleport_objects FROM "teleport-admin"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_adm_nousage", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_adm_nousage`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_adm_nousage_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_adm_nousage_tbl (id int);
				ALTER TABLE alice_adm_nousage_tbl OWNER TO "alice_adm_nousage"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_adm_nousage", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_adm_nousage")
		})

		t.Run("teleport-admin lacks EXECUTE on procedure: object-owning user is deactivated", func(t *testing.T) {
			_, err := env.bootstrapConn.Exec(t.Context(), `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, err)
			installReassignObjectsProcedure(t, env.bootstrapConn)
			_, err = env.bootstrapConn.Exec(t.Context(),
				`REVOKE EXECUTE ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) FROM "teleport-admin"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_adm_noexec", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_adm_noexec`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_adm_noexec_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_adm_noexec_tbl (id int);
				ALTER TABLE alice_adm_noexec_tbl OWNER TO "alice_adm_noexec"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_adm_noexec", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_adm_noexec")
		})

		t.Run("Procedure owner lacks privileges: object-owning user is deactivated", func(t *testing.T) {
			// Coarse stand-in for an owner that lacks the privileges to reassign:
			// it raises the insufficient_privilege error Postgres itself would.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DROP SCHEMA IF EXISTS teleport_objects CASCADE;
				CREATE SCHEMA teleport_objects;
				CREATE PROCEDURE teleport_objects.teleport_reassign_objects(source_user varchar, destination_user varchar)
					LANGUAGE plpgsql AS $$
					BEGIN
						RAISE EXCEPTION 'permission denied for table (simulated)' USING ERRCODE = 'insufficient_privilege';
					END $$;
				GRANT USAGE ON SCHEMA teleport_objects TO "teleport-admin";
				GRANT EXECUTE ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) TO "teleport-admin"`)
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_owner_noperm", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_owner_noperm`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_owner_noperm_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_owner_noperm_tbl (id int);
				ALTER TABLE alice_owner_noperm_tbl OWNER TO "alice_owner_noperm"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_owner_noperm", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_owner_noperm")
		})

		t.Run("Custom SECURITY INVOKER procedure: object-owning user is deactivated", func(t *testing.T) {
			// SECURITY INVOKER runs the body as teleport-admin, not the (superuser)
			// owner, so the superuser-only pg_authid read fails.
			_, err := env.bootstrapConn.Exec(t.Context(), `
				DROP SCHEMA IF EXISTS teleport_objects CASCADE;
				CREATE SCHEMA teleport_objects;
				CREATE PROCEDURE teleport_objects.teleport_reassign_objects(source_user varchar, destination_user varchar)
					LANGUAGE plpgsql SECURITY INVOKER AS $$
					DECLARE n int;
					BEGIN
						SELECT count(*) INTO n FROM pg_authid;
					END $$;
				GRANT USAGE ON SCHEMA teleport_objects TO "teleport-admin";
				GRANT EXECUTE ON PROCEDURE teleport_objects.teleport_reassign_objects(varchar, varchar) TO "teleport-admin"`)
			require.NoError(t, err)
			cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
			require.NoError(t, env.engine.ActivateUser(t.Context(), makeReassignSession(env.db, "alice_sec_invoker", nil)))
			cleanupSQL(t, env.bootstrapConn, `DROP USER IF EXISTS alice_sec_invoker`)
			cleanupSQL(t, env.bootstrapConn, `DROP TABLE IF EXISTS alice_sec_invoker_tbl`)
			_, err = env.bootstrapConn.Exec(t.Context(), `
				CREATE TABLE IF NOT EXISTS alice_sec_invoker_tbl (id int);
				ALTER TABLE alice_sec_invoker_tbl OWNER TO "alice_sec_invoker"`)
			require.NoError(t, err)
			require.NoError(t, env.engine.DeleteUser(t.Context(), makeReassignSession(env.db, "alice_sec_invoker", nil)))
			requireDeactivatedNotDropped(t, env.adminConn, "alice_sec_invoker")
		})
	})

	t.Run("reassign-objects.sql grant hardening", func(t *testing.T) {
		// A server-level ALTER DEFAULT PRIVILEGES can put grants on new
		// functions and schemas directly in their ACLs, not through PUBLIC, so
		// the script must revoke the whole ACL on both the procedure and its
		// schema -- not just from PUBLIC.

		// Cleanups run LIFO; the ADP resets must precede the adp_grantee drop,
		// as a role referenced by pg_default_acl cannot be dropped.
		cleanupSQL(t, env.bootstrapConn, `DROP SCHEMA IF EXISTS teleport_objects CASCADE`)
		cleanupSQL(t, env.bootstrapConn, `DROP ROLE IF EXISTS adp_grantee`)
		cleanupSQL(t, env.bootstrapConn, `DROP ROLE IF EXISTS plain_role`)
		cleanupSQL(t, env.bootstrapConn, `ALTER DEFAULT PRIVILEGES REVOKE EXECUTE ON FUNCTIONS FROM adp_grantee`)
		cleanupSQL(t, env.bootstrapConn, `ALTER DEFAULT PRIVILEGES REVOKE ALL ON SCHEMAS FROM adp_grantee`)

		_, err := env.bootstrapConn.Exec(t.Context(), `
			DO $$ BEGIN
				IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'adp_grantee') THEN
					CREATE ROLE adp_grantee;
				END IF;
				IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'plain_role') THEN
					CREATE ROLE plain_role;
				END IF;
			END $$`)
		require.NoError(t, err)

		// FUNCTIONS also covers procedures.
		_, err = env.bootstrapConn.Exec(t.Context(), `
			ALTER DEFAULT PRIVILEGES GRANT EXECUTE ON FUNCTIONS TO adp_grantee;
			ALTER DEFAULT PRIVILEGES GRANT USAGE, CREATE ON SCHEMAS TO adp_grantee`)
		require.NoError(t, err)
		_, err = env.bootstrapConn.Exec(t.Context(), reassignObjectsProcedure)
		require.NoError(t, err)

		t.Run("procedure: only teleport-admin can EXECUTE", func(t *testing.T) {
			require.True(t, hasExecuteOnReassignProc(t, env.bootstrapConn, "teleport-admin"),
				"teleport-admin must keep EXECUTE")
			require.False(t, hasExecuteOnReassignProc(t, env.bootstrapConn, "adp_grantee"),
				"the ALTER DEFAULT PRIVILEGES grant must be revoked")
			require.False(t, hasExecuteOnReassignProc(t, env.bootstrapConn, "plain_role"),
				"PUBLIC's default EXECUTE must be revoked")
		})

		t.Run("schema: only teleport-admin has USAGE", func(t *testing.T) {
			require.True(t, hasSchemaPrivilege(t, env.bootstrapConn, "teleport-admin", "USAGE"),
				"teleport-admin must keep USAGE")
			require.False(t, hasSchemaPrivilege(t, env.bootstrapConn, "teleport-admin", "CREATE"),
				"teleport-admin must not be granted CREATE")
			require.False(t, hasSchemaPrivilege(t, env.bootstrapConn, "adp_grantee", "USAGE"),
				"the ALTER DEFAULT PRIVILEGES grant must be revoked")
			require.False(t, hasSchemaPrivilege(t, env.bootstrapConn, "plain_role", "USAGE"),
				"PUBLIC must have no USAGE")
		})
	})
}
