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

// noopAuth implements common.Auth for testing. GetTLSConfig returns nil so
// that the engine connects over plain TCP, as required for testcontainers.
// Only the methods called during ActivateUser/DeactivateUser/DeleteUser are
// implemented; any other call will panic. This is a benefit, since a future
// panic signals a new code path that needs a real implementation.
type noopAuth struct {
	common.Auth
}

func (a *noopAuth) GetTLSConfig(_ context.Context, _ time.Time, _ types.Database, _ string) (*tls.Config, error) {
	return nil, nil
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

func TestUserAutoProvisioning(t *testing.T) {
	if run, _ := apiutils.ParseBool(os.Getenv("ENABLE_TESTCONTAINERS")); !run {
		// Docker Hub rate limits cause failures in CI, this test is disabled until we can set up an alternative to Docker Hub
		t.Skip("Test disabled in CI. Enable it by setting env variable ENABLE_TESTCONTAINERS")
	}
	pgContainer, err := postgres.Run(t.Context(), "postgres:18",
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

	port, err := pgContainer.MappedPort(t.Context(), "5432/tcp")
	require.NoError(t, err)
	// Include credentials in the URI so the engine's admin connection can
	// authenticate. The connector prepends "postgres://" before parsing, so
	// the result is "postgres://postgres:postgres@localhost:PORT".
	dbURI := fmt.Sprintf("postgres:postgres@localhost:%s", port.Port())

	db, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-pg",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      dbURI,
		AdminUser: &types.DatabaseAdminUser{
			Name: "postgres",
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

	connStr, err := pgContainer.ConnectionString(t.Context())
	require.NoError(t, err)

	// adminConn runs DDL and assertion queries between engine calls.
	adminConn, err := pgx.Connect(t.Context(), connStr)
	require.NoError(t, err)
	t.Cleanup(func() { adminConn.Close(context.Background()) })

	// ── ActivateUser ─────────────────────────────────────────────────────────

	t.Run("creates new user", func(t *testing.T) {
		err := engine.ActivateUser(t.Context(), makeSession(db, "alice_new", nil))
		require.NoError(t, err)

		var exists bool
		err = adminConn.QueryRow(t.Context(),
			"SELECT true FROM pg_catalog.pg_user WHERE usename = $1", "alice_new").Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("reactivates deactivated user", func(t *testing.T) {
		// Use the engine for initial creation so that teleport-auto-user is set up.
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_reactivate", nil)))
		// Deactivate properly: revokes roles and sets NOLOGIN.
		require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, "alice_reactivate", nil)))
		require.False(t, canLogin(t, adminConn, "alice_reactivate"), "precondition: login should be disabled")

		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_reactivate", nil)))
		require.True(t, canLogin(t, adminConn, "alice_reactivate"), "user should be able to log in after reactivation")
	})

	t.Run("assigns roles", func(t *testing.T) {
		_, err := adminConn.Exec(t.Context(), `CREATE ROLE testrole`)
		require.NoError(t, err)

		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_roles", []string{"testrole"})))
		require.True(t, isMember(t, adminConn, "alice_roles", "testrole"), "user should be member of testrole")
	})

	t.Run("preserves teleport-auto-user on reactivation", func(t *testing.T) {
		// First activation creates the user as a member of teleport-auto-user.
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_preserve", nil)))
		require.True(t, isMember(t, adminConn, "alice_preserve", "teleport-auto-user"),
			"precondition: user should be member of teleport-auto-user")

		// Deactivate properly: the deactivate procedure revokes roles but must
		// preserve teleport-auto-user membership.
		require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, "alice_preserve", nil)))
		require.True(t, isMember(t, adminConn, "alice_preserve", "teleport-auto-user"),
			"teleport-auto-user membership must survive deactivate")

		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_preserve", nil)))
		require.True(t, isMember(t, adminConn, "alice_preserve", "teleport-auto-user"),
			"teleport-auto-user membership must survive deactivate/reactivate cycle")
	})

	t.Run("rejects pre-existing non-teleport user", func(t *testing.T) {
		// Create a user that Teleport did not provision (not in teleport-auto-user).
		_, err := adminConn.Exec(t.Context(), `CREATE USER "alice_external"`)
		require.NoError(t, err)

		err = engine.ActivateUser(t.Context(), makeSession(db, "alice_external", nil))
		require.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists error, got: %v", err)
	})

	t.Run("active connection same roles succeeds", func(t *testing.T) {
		// Activate the user, then assign a password so we can open a connection as them.
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_active_same", nil)))
		_, err := adminConn.Exec(t.Context(),
			`ALTER USER "alice_active_same" WITH PASSWORD 'testpass' LOGIN`)
		require.NoError(t, err)

		userConn, err := pgx.Connect(t.Context(),
			fmt.Sprintf("postgres://alice_active_same:testpass@localhost:%s/postgres", port.Port()))
		require.NoError(t, err)
		defer userConn.Close(context.Background())

		// ActivateUser with the same (empty) role set while a connection is open
		// should succeed because nothing changed.
		err = engine.ActivateUser(t.Context(), makeSession(db, "alice_active_same", nil))
		require.NoError(t, err)
	})

	t.Run("active connection different roles fails", func(t *testing.T) {
		_, err := adminConn.Exec(t.Context(), `CREATE ROLE role_for_diff`)
		require.NoError(t, err)

		// Activate the user without any roles, then assign a password.
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_active_diff", nil)))
		_, err = adminConn.Exec(t.Context(),
			`ALTER USER "alice_active_diff" WITH PASSWORD 'testpass' LOGIN`)
		require.NoError(t, err)

		userConn, err := pgx.Connect(t.Context(),
			fmt.Sprintf("postgres://alice_active_diff:testpass@localhost:%s/postgres", port.Port()))
		require.NoError(t, err)
		defer userConn.Close(context.Background())

		// ActivateUser with a new role while a connection is open should fail
		// because the active session's roles would need to change.
		err = engine.ActivateUser(t.Context(), makeSession(db, "alice_active_diff", []string{"role_for_diff"}))
		require.True(t, trace.IsCompareFailed(err), "expected CompareFailed error, got: %v", err)
	})

	// ── DeactivateUser ───────────────────────────────────────────────────────

	t.Run("DeactivateUser strips roles and disables login", func(t *testing.T) {
		_, err := adminConn.Exec(t.Context(), `CREATE ROLE testrole_deact`)
		require.NoError(t, err)

		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_deact_roles", []string{"testrole_deact"})))
		require.True(t, isMember(t, adminConn, "alice_deact_roles", "testrole_deact"), "precondition")
		require.True(t, canLogin(t, adminConn, "alice_deact_roles"), "precondition")

		require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, "alice_deact_roles", nil)))

		require.False(t, isMember(t, adminConn, "alice_deact_roles", "testrole_deact"),
			"non-teleport role should be revoked after deactivation")
		require.False(t, canLogin(t, adminConn, "alice_deact_roles"),
			"login should be disabled after deactivation")
	})

	t.Run("DeactivateUser is no-op when user has active connections", func(t *testing.T) {
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_deact_active", nil)))
		_, err := adminConn.Exec(t.Context(),
			`ALTER USER "alice_deact_active" WITH PASSWORD 'testpass'`)
		require.NoError(t, err)

		userConn, err := pgx.Connect(t.Context(),
			fmt.Sprintf("postgres://alice_deact_active:testpass@localhost:%s/postgres", port.Port()))
		require.NoError(t, err)
		defer userConn.Close(context.Background())

		require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, "alice_deact_active", nil)))
		require.True(t, canLogin(t, adminConn, "alice_deact_active"),
			"login should remain enabled while an active connection is open")
	})

	t.Run("DeactivateUser preserves teleport-auto-user membership", func(t *testing.T) {
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_deact_preserve", nil)))
		require.NoError(t, engine.DeactivateUser(t.Context(), makeSession(db, "alice_deact_preserve", nil)))
		require.True(t, isMember(t, adminConn, "alice_deact_preserve", "teleport-auto-user"),
			"teleport-auto-user membership must be preserved after deactivation")
	})

	// ── DeleteUser ───────────────────────────────────────────────────────────

	t.Run("DeleteUser drops user with no owned objects", func(t *testing.T) {
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_delete_clean", nil)))
		require.NoError(t, engine.DeleteUser(t.Context(), makeSession(db, "alice_delete_clean", nil)))
		require.False(t, userExists(t, adminConn, "alice_delete_clean"), "user should be dropped")
	})

	t.Run("DeleteUser does not drop user with active connections", func(t *testing.T) {
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_delete_active", nil)))
		_, err := adminConn.Exec(t.Context(),
			`ALTER USER "alice_delete_active" WITH PASSWORD 'testpass' LOGIN`)
		require.NoError(t, err)

		userConn, err := pgx.Connect(t.Context(),
			fmt.Sprintf("postgres://alice_delete_active:testpass@localhost:%s/postgres", port.Port()))
		require.NoError(t, err)
		defer userConn.Close(context.Background())

		require.NoError(t, engine.DeleteUser(t.Context(), makeSession(db, "alice_delete_active", nil)))
		require.True(t, userExists(t, adminConn, "alice_delete_active"), "user should still exist")
		require.True(t, canLogin(t, adminConn, "alice_delete_active"), "user should still have login")
	})

	t.Run("DeleteUser deactivates instead of dropping user who owns objects", func(t *testing.T) {
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_delete_owns", nil)))
		_, err := adminConn.Exec(t.Context(), `
			CREATE TABLE alice_delete_owns_tbl (id int);
			ALTER TABLE alice_delete_owns_tbl OWNER TO "alice_delete_owns"`)
		require.NoError(t, err)

		// db has no OrphanedResourceOwner, so reassignment is skipped; the user
		// cannot be dropped and is deactivated instead.
		require.NoError(t, engine.DeleteUser(t.Context(), makeSession(db, "alice_delete_owns", nil)))
		require.True(t, userExists(t, adminConn, "alice_delete_owns"),
			"user should still exist (deactivated, not dropped)")
		require.False(t, canLogin(t, adminConn, "alice_delete_owns"),
			"user should have login disabled")
	})

	// ── Lifecycle ────────────────────────────────────────────────────────────

	t.Run("teleport-auto-user role is self-healing", func(t *testing.T) {
		// Drop teleport-auto-user to simulate it being removed out-of-band.
		// All members must be revoked before the role can be dropped.
		_, err := adminConn.Exec(t.Context(), `
			DO $$
			DECLARE r text;
			BEGIN
				FOR r IN
					SELECT member_role.rolname
					FROM pg_auth_members m
					JOIN pg_roles member_role ON m.member = member_role.oid
					WHERE m.roleid = (SELECT oid FROM pg_roles WHERE rolname = 'teleport-auto-user')
				LOOP
					EXECUTE FORMAT('REVOKE "teleport-auto-user" FROM %I CASCADE', r);
				END LOOP;
			END $$;
			DROP ROLE "teleport-auto-user"`)
		require.NoError(t, err)

		// ActivateUser must recreate teleport-auto-user and assign the new user.
		require.NoError(t, engine.ActivateUser(t.Context(), makeSession(db, "alice_selfheal", nil)))
		require.True(t, isMember(t, adminConn, "alice_selfheal", "teleport-auto-user"),
			"ActivateUser should recreate teleport-auto-user and add the user to it")
	})
}
