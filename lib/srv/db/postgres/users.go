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

package postgres

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"github.com/lib/pq"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/srv/db/common/permissions"
	"github.com/gravitational/teleport/lib/srv/db/objects"
)

// connectAsAdmin connects as the admin user to the default database, per database settings, or as a fallback to the one specified in sessionCtx.
func (e *Engine) connectAsAdminDefaultDatabase(ctx context.Context, sessionCtx *common.Session) (*pgx.Conn, error) {
	return e.newConnector(sessionCtx).withDefaultDatabase().connectAsAdmin(ctx)
}

// connectAsAdmin connects as the admin user to the database specified in sessionCtx.
func (e *Engine) connectAsAdminSessionDatabase(ctx context.Context, sessionCtx *common.Session) (*pgx.Conn, error) {
	return e.newConnector(sessionCtx).connectAsAdmin(ctx)
}

// ActivateUser creates or enables the database user.
func (e *Engine) ActivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	if sessionCtx.Database.IsRDS() &&
		sessionCtx.Database.GetEndpointType() == apiawsutils.RDSEndpointTypeReader {
		return trace.BadParameter("auto-user provisioning is not supported for RDS reader endpoints")
	}

	conn, err := e.connectAsAdminDefaultDatabase(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)

	// We could call this once when the database is being initialized but
	// doing it here has a nice "self-healing" property in case the Teleport
	// bookkeeping group or stored procedures get deleted or changed offband.
	logger := e.Log.With("user", sessionCtx.DatabaseUser)
	err = withRetry(ctx, logger, func() error {
		return trace.Wrap(e.updateAutoUsersRole(ctx, conn, sessionCtx.Database.GetAdminUser().Name))
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = withRetry(ctx, logger, func() error {
		err := e.createProcedures(ctx, sessionCtx, conn, []string{activateProcName, deactivateProcName})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	roles, err := prepareRoles(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	logger.InfoContext(ctx, "Activating PostgreSQL user", "roles", roles)
	err = withRetry(ctx, logger, func() error {
		return trace.Wrap(e.callProcedure(ctx, sessionCtx, conn, activateProcName, sessionCtx.DatabaseUser, roles))
	})
	if err != nil {
		logger.DebugContext(ctx, "Call teleport_activate_user failed.", "error", err)
		errOut := convertActivateError(sessionCtx, err)
		e.Audit.OnDatabaseUserCreate(ctx, sessionCtx, errOut)
		return trace.Wrap(errOut)
	}
	e.Audit.OnDatabaseUserCreate(ctx, sessionCtx, nil)

	err = e.applyPermissions(ctx, sessionCtx)
	if err != nil {
		logger.WarnContext(e.Context, "Failed to apply permissions.", "error", err)
		return trace.Wrap(err)
	}
	return nil
}

// TablePermission is the represents a permission for particular database table.
type TablePermission struct {
	Privilege string `json:"privilege"`
	Schema    string `json:"schema"`
	Table     string `json:"table"`
}

type Permissions struct {
	Tables []TablePermission `json:"tables"`
}

var pgTablePerms = map[string]struct{}{
	"DELETE":     {},
	"INSERT":     {},
	"REFERENCES": {},
	"SELECT":     {},
	"TRIGGER":    {},
	"TRUNCATE":   {},
	"UPDATE":     {},
}

func checkPgPermission(objKind, perm string) error {
	// for now, only tables are supported. ignore other kinds of objects.
	if objKind != databaseobjectimportrule.ObjectKindTable {
		return nil
	}

	normalized := strings.ToUpper(strings.TrimSpace(perm))
	_, found := pgTablePerms[normalized]
	if !found {
		return trace.BadParameter("unrecognized %q Postgres permission: %q", objKind, perm)
	}
	return nil
}

// convertPermissions converts the permissions into a stable format expected by the stored procedure. It also filters out any unsupported objects.
func convertPermissions(perms permissions.PermissionSet) (*Permissions, error) {
	var out Permissions
	var errors []error
	for permission, objects := range perms {
		for _, obj := range objects {
			if err := checkPgPermission(obj.GetSpec().ObjectKind, permission); err != nil {
				errors = append(errors, err)
				continue
			}
			if obj.GetSpec().ObjectKind == databaseobjectimportrule.ObjectKindTable {
				out.Tables = append(out.Tables, TablePermission{
					Privilege: permission,
					Schema:    obj.GetSpec().Schema,
					Table:     obj.GetSpec().Name,
				})
			}
		}
	}
	if len(errors) > 0 {
		return nil, trace.NewAggregate(errors...)
	}
	return &out, nil
}

func (e *Engine) granularPermissionsEnabled(sessionCtx *common.Session) bool {
	allow, _, _ := sessionCtx.Checker.GetDatabasePermissions(sessionCtx.Database)
	if len(allow) == 0 {
		return false
	}

	if len(sessionCtx.DatabaseRoles) > 0 {
		return false
	}

	return true
}

func (e *Engine) applyPermissions(ctx context.Context, sessionCtx *common.Session) error {
	allow, _, err := sessionCtx.Checker.GetDatabasePermissions(sessionCtx.Database)
	if err != nil {
		e.Log.ErrorContext(e.Context, "Failed to calculate effective database permissions.", "error", err)
		return trace.Wrap(err)
	}
	if len(allow) == 0 {
		e.Log.InfoContext(e.Context, "Skipping applying fine-grained permissions: none to apply.")
		return nil
	}

	if len(sessionCtx.DatabaseRoles) > 0 {
		e.Log.ErrorContext(ctx, "Cannot apply fine-grained permissions: non-empty list of database roles.", "roles", sessionCtx.DatabaseRoles)
		return trace.BadParameter("fine-grained database permissions and database roles are mutually exclusive, yet both were provided.")
	}

	fetcher, err := objects.GetObjectFetcher(ctx, sessionCtx.Database, objects.ObjectFetcherConfig{
		ImportRules:  e.AuthClient,
		Auth:         e.Auth,
		CloudClients: e.CloudClients,
		Log:          e.Log,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	objsImported, err := fetcher.FetchOneDatabase(ctx, sessionCtx.DatabaseName)
	if err != nil {
		return trace.Wrap(err)
	}

	permissionSet, err := permissions.CalculatePermissions(sessionCtx.Checker, sessionCtx.Database, objsImported)
	if err != nil {
		return trace.Wrap(err)
	}

	summary, eventData := permissions.SummarizePermissions(permissionSet)
	e.Log.InfoContext(ctx, "Calculated database permissions.", "summary", summary, "user", sessionCtx.DatabaseUser)
	e.auditUserPermissions(sessionCtx, eventData)

	perms, err := convertPermissions(permissionSet)
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := e.connectAsAdminSessionDatabase(ctx, sessionCtx)
	if err != nil {
		e.Log.ErrorContext(ctx, "Failed to connect to the database.", "error", err)
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)

	// teleport_remove_permissions and teleport_update_permissions are created in pg_temp table of the session database.
	// teleport_remove_permissions gets called by teleport_update_permissions as needed.
	logger := e.Log.With("user", sessionCtx.DatabaseUser)
	err = withRetry(ctx, logger, func() error {
		err := e.createProcedures(ctx, sessionCtx, conn, []string{removePermissionsProcName, updatePermissionsProcName})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = withRetry(ctx, logger, func() error {
		err := e.callProcedure(ctx, sessionCtx, conn, updatePermissionsProcName, sessionCtx.DatabaseUser, perms)
		return trace.Wrap(err)
	})
	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			if pgErr.Code == common.SQLStatePermissionsChanged {
				logger.ErrorContext(ctx, "User permissions have changed, rejecting connection",
					"error", err,
				)
			}
		} else {
			logger.ErrorContext(ctx, "Failed to update user permissions",
				"error", err,
			)
		}
	}
	return nil
}

func (e *Engine) removePermissions(ctx context.Context, sessionCtx *common.Session) error {
	logger := e.Log.With("user", sessionCtx.DatabaseUser)
	if !e.granularPermissionsEnabled(sessionCtx) {
		logger.InfoContext(ctx, "Granular database permissions not enabled, skipping removal step.")
		return nil
	}

	logger.InfoContext(ctx, "Removing permissions from PostgreSQL user.")
	conn, err := e.connectAsAdminSessionDatabase(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)

	// teleport_remove_permissions is created in pg_temp table of the session database.
	err = withRetry(ctx, logger, func() error {
		err := e.createProcedures(ctx, sessionCtx, conn, []string{removePermissionsProcName})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = withRetry(ctx, logger, func() error {
		err := e.callProcedure(ctx, sessionCtx, conn, removePermissionsProcName, sessionCtx.DatabaseUser)
		return trace.Wrap(err)
	})
	if err != nil {
		logger.ErrorContext(ctx, "Removing permissions from user failed",
			"error", err,
		)
	}
	return nil
}

// DeactivateUser disables the database user.
func (e *Engine) DeactivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	// removal may yield errors, but we will still attempt to deactivate the user.
	errRemove := trace.Wrap(e.removePermissions(ctx, sessionCtx))

	conn, err := e.connectAsAdminDefaultDatabase(ctx, sessionCtx)
	if err != nil {
		return trace.NewAggregate(errRemove, trace.Wrap(err))
	}
	defer conn.Close(ctx)

	logger := e.Log.With("user", sessionCtx.DatabaseUser)
	logger.InfoContext(ctx, "Deactivating PostgreSQL user.")
	err = withRetry(ctx, logger, func() error {
		err := e.createProcedures(ctx, sessionCtx, conn, []string{deactivateProcName})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.NewAggregate(errRemove, trace.Wrap(err))
	}

	err = withRetry(ctx, logger, func() error {
		return trace.Wrap(e.callProcedure(ctx, sessionCtx, conn, deactivateProcName, sessionCtx.DatabaseUser))
	})
	if err != nil {
		e.Audit.OnDatabaseUserDeactivate(ctx, sessionCtx, false, err)
		return trace.NewAggregate(errRemove, trace.Wrap(err))
	}
	e.Audit.OnDatabaseUserDeactivate(ctx, sessionCtx, false, nil)

	return errRemove
}

// DeleteUser deletes the database user.
func (e *Engine) DeleteUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	// removal may yield errors, but we will still attempt to delete the user.
	errRemove := trace.Wrap(e.removePermissions(ctx, sessionCtx))

	conn, err := e.connectAsAdminDefaultDatabase(ctx, sessionCtx)
	if err != nil {
		return trace.NewAggregate(errRemove, trace.Wrap(err))
	}
	defer conn.Close(ctx)

	logger := e.Log.With("user", sessionCtx.DatabaseUser)
	logger.InfoContext(ctx, "Deleting PostgreSQL user.")
	err = withRetry(ctx, logger, func() error {
		err := e.createProcedures(ctx, sessionCtx, conn, []string{deleteProcName, deactivateProcName})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var state string
	err = withRetry(ctx, logger, func() error {
		switch {
		case sessionCtx.Database.IsRedshift():
			return trace.Wrap(e.deleteUserRedshift(ctx, sessionCtx, conn, &state))
		default:
			deleteQuery, err := buildCallQuery(sessionCtx, deleteProcName)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(conn.QueryRow(ctx, deleteQuery, sessionCtx.DatabaseUser).Scan(&state))
		}
	})
	if err != nil {
		return trace.NewAggregate(errRemove, trace.Wrap(err))
	}

	deleted := true
	switch state {
	case common.SQLStateUserDropped:
		logger.DebugContext(ctx, "User deleted successfully.")
	case common.SQLStateUserDeactivated:
		deleted = false
		logger.InfoContext(ctx, "Unable to delete user, it was disabled instead.")
	default:
		logger.WarnContext(ctx, "Unable to determine user deletion state.")
	}
	e.Audit.OnDatabaseUserDeactivate(ctx, sessionCtx, deleted, nil)

	return errRemove
}

// deleteUserRedshift deletes the Redshift database user.
//
// Failures inside Redshift default procedures are always rethrown exceptions if
// the exception handler completes successfully. Given this, we need to assert
// into the returned error instead of doing this on state returned (like regular
// PostgreSQL).
func (e *Engine) deleteUserRedshift(ctx context.Context, sessionCtx *common.Session, conn *pgx.Conn, state *string) error {
	err := e.callProcedure(ctx, sessionCtx, conn, deleteProcName, sessionCtx.DatabaseUser)
	if err == nil {
		*state = common.SQLStateUserDropped
		return nil
	}

	// Redshift returns SQLSTATE 55006 (object_in_use) when DROP USER fails due
	// to user owning resources.
	// https://docs.aws.amazon.com/redshift/latest/dg/r_DROP_USER.html#r_DROP_USER-notes
	if strings.Contains(err.Error(), "55006") {
		*state = common.SQLStateUserDeactivated
		return nil
	}

	return trace.Wrap(err)
}

// updateAutoUsersRole ensures the bookkeeping role for auto-provisioned users
// is present.
func (e *Engine) updateAutoUsersRole(ctx context.Context, conn *pgx.Conn, adminUser string) error {
	_, err := conn.Exec(ctx, fmt.Sprintf("create role %q", teleportAutoUserRole))
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return trace.Wrap(err)
		}
		e.Log.DebugContext(ctx, "PostgreSQL role already exists", "role", teleportAutoUserRole)
	} else {
		e.Log.DebugContext(ctx, "Created PostgreSQL role", "role", teleportAutoUserRole)
	}

	// v16 Postgres changed the role grant permissions model such that you can
	// no longer grant non-superuser role membership just by having the
	// CREATEROLE attribute.
	// On v16 Postgres, when a role is created the creator is automatically
	// granted that role with "INHERIT FALSE, SET FALSE, ADMIN OPTION" options.
	// Prior to v16 Postgres that grant is not automatically made, because
	// the CREATEROLE attribute alone was sufficient to grant the role to
	// others.
	// This is the only role that is created and granted to others by the
	// Teleport database admin.
	// It grants the auto user role to every role it provisions.
	// To avoid breaking user auto-provisioning for customers who upgrade from
	// v15 postgres to v16, we should grant this role with the admin option to
	// ourselves after creating it.
	// Also note that the grant syntax in v15 postgres and below does not
	// support WITH INHERIT FALSE or WITH SET FALSE syntax, so we only specify
	// WITH ADMIN OPTION.
	// See: https://www.postgresql.org/docs/16/release-16.html
	stmt := fmt.Sprintf("grant %q to %q WITH ADMIN OPTION", teleportAutoUserRole, adminUser)
	_, err = conn.Exec(ctx, stmt)
	if err != nil {
		if !strings.Contains(err.Error(), "cannot be granted back") && !strings.Contains(err.Error(), "already") {
			e.Log.DebugContext(ctx, "Failed to grant required role to the Teleport database admin, user auto-provisioning may not work until the database admin is granted the role by a superuser",
				"role", teleportAutoUserRole,
				"database_admin", adminUser,
				"error", err,
			)
		}
	}
	return nil
}

// callProcedure calls the procedure with the provided arguments.
func (e *Engine) callProcedure(ctx context.Context, sessionCtx *common.Session, conn *pgx.Conn, procName string, args ...any) error {
	query, err := buildCallQuery(sessionCtx, procName)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = conn.Exec(ctx, query, args...)
	return trace.Wrap(err)
}

// createProcedures executes the create procedures for the provided list of
// procedures.
func (e *Engine) createProcedures(ctx context.Context, sessionCtx *common.Session, conn *pgx.Conn, procNames []string) error {
	selectedProcs := pickProcedures(sessionCtx)

	for _, procName := range procNames {
		proc, ok := selectedProcs[procName]
		if !ok {
			return trace.NotImplemented("procedure %q is not available for %s databases", procName, sessionCtx.Database.GetType())
		}

		logger := e.Log.With("procedure", procName)

		if _, err := conn.Exec(ctx, proc); err != nil {
			logger.ErrorContext(ctx, "Failed to install procedure.")
			return trace.Wrap(err)
		}

		logger.DebugContext(ctx, "Installed procedure.")
	}

	return nil
}

// buildCallQuery builds the call query based on the procedure name and session.
func buildCallQuery(sessionCtx *common.Session, procName string) (string, error) {
	if _, ok := pickProcedures(sessionCtx)[procName]; !ok {
		return "", trace.NotImplemented("procedure %q is not available for %s databases", procName, sessionCtx.Database.GetType())
	}

	var schema string
	switch {
	case sessionCtx.Database.IsRedshift():
		// TODO(gabrielcorado): support customizing the schema the procedures
		// will be stored on RedShift. For now, let the database decide where
		// to store them.
		schema = ""
	default:
		// Always use `pg_temp` if the database type supports it. This reduces
		// the number of permissions required by the admin user.
		schema = "pg_temp"
	}

	procCall, ok := procsCall[procName]
	if !ok {
		return "", trace.BadParameter("procedure %q doesn't have a call statement", procName)
	}

	if schema != "" {
		return fmt.Sprintf("call %s.%s", schema, procCall), nil
	}

	return "call " + procCall, nil
}

func prepareRoles(sessionCtx *common.Session) (any, error) {
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeRDS:
		return append(sessionCtx.DatabaseRoles, "rds_iam"), nil

	case types.DatabaseTypeRedshift:
		// Redshift does not support array. Encode roles in JSON (type text).
		rolesJSON, err := json.Marshal(sessionCtx.DatabaseRoles)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return string(rolesJSON), nil

	default:
		return sessionCtx.DatabaseRoles, nil
	}
}

func convertActivateError(sessionCtx *common.Session, err error) error {
	switch {
	case strings.Contains(err.Error(), "already exists"):
		return trace.AlreadyExists("user %q already exists in this PostgreSQL database and is not managed by Teleport", sessionCtx.DatabaseUser)

	case strings.Contains(err.Error(), "TP002: User has active connections and roles have changed"):
		return trace.CompareFailed("roles for user %q has changed. Please quit all active connections and try again.", sessionCtx.DatabaseUser)

	default:
		return trace.Wrap(err)
	}
}

func pickProcedures(sessionCtx *common.Session) map[string]string {
	if sessionCtx.Database.IsRedshift() {
		return redshiftProcs
	}
	return procs
}

const (
	// activateProcName is the name of the stored procedure Teleport will use
	// to automatically provision/activate database users.
	activateProcName = "teleport_activate_user"
	// deactivateProcName is the name of the stored procedure Teleport will use
	// to automatically deactivate database users after session ends.
	deactivateProcName = "teleport_deactivate_user"
	// deleteProcName is the name of the stored procedure Teleport will use to
	// automatically delete database users after session ends.
	deleteProcName = "teleport_delete_user"
	// updatePermissionsProcName is the name of the stored procedure Teleport will use
	// to automatically update database permissions.
	updatePermissionsProcName = "teleport_update_permissions"
	// removePermissionsProcName is the name of the stored procedure Teleport will use
	// to automatically remove all database permissions.
	removePermissionsProcName = "teleport_remove_permissions"
	// teleportAutoUserRole is the name of a PostgreSQL role that all Teleport
	// managed users will be a part of.
	teleportAutoUserRole = "teleport-auto-user"
)

var (
	//go:embed sql/activate-user.sql
	activateProc string
	// activateProcCall contains the procedure name and arguments used to call
	// the activate user procedure.
	activateProcCall = fmt.Sprintf(`%v($1, $2)`, activateProcName)

	//go:embed sql/deactivate-user.sql
	deactivateProc string
	// deactivateProcCall contains the procedure name and arguments used to call
	// the deactivate user procedure.
	deactivateProcCall = fmt.Sprintf(`%v($1)`, deactivateProcName)

	//go:embed sql/delete-user.sql
	deleteProc string
	// deleteProcCall contains the procedure name and arguments used to call
	// the delete user procedure.
	deleteProcCall = fmt.Sprintf(`%v($1)`, deleteProcName)

	//go:embed sql/redshift-activate-user.sql
	redshiftActivateProc string
	//go:embed sql/redshift-deactivate-user.sql
	redshiftDeactivateProc string
	//go:embed sql/redshift-delete-user.sql
	redshiftDeleteProc string

	//go:embed sql/update-permissions.sql
	updatePermissionsProc string
	// updatePermissionsProcCall contains the procedure name and arguments used
	// to call the update permissions procedure.
	updatePermissionsProcCall = fmt.Sprintf(`%v($1, $2::jsonb)`, updatePermissionsProcName)

	//go:embed sql/remove-permissions.sql
	removePermissionsProc string
	// removePermissionsProcCall contains the procedure name and arguments used
	// to call the remove permissions procedure.
	removePermissionsProcCall = fmt.Sprintf(`%v($1)`, removePermissionsProcName)

	procs = map[string]string{
		activateProcName:          activateProc,
		deactivateProcName:        deactivateProc,
		deleteProcName:            deleteProc,
		updatePermissionsProcName: updatePermissionsProc,
		removePermissionsProcName: removePermissionsProc,
	}

	redshiftProcs = map[string]string{
		activateProcName:   redshiftActivateProc,
		deactivateProcName: redshiftDeactivateProc,
		deleteProcName:     redshiftDeleteProc,
	}

	// procsCall maps procedures names to their call statements.
	procsCall = map[string]string{
		activateProcName:          activateProcCall,
		deactivateProcName:        deactivateProcCall,
		deleteProcName:            deleteProcCall,
		updatePermissionsProcName: updatePermissionsProcCall,
		removePermissionsProcName: removePermissionsProcCall,
	}
)

// withRetry is a helper for auto user operations that runs a given func a
// finite number of times until it returns nil error or the given context is
// done.
func withRetry(ctx context.Context, log *slog.Logger, f func() error) error {
	linear, err := retryutils.NewLinear(retryutils.LinearConfig{
		// arbitrarily copied settings from retry logic in lib/backend/pgbk.
		First:  0,
		Step:   100 * time.Millisecond,
		Max:    750 * time.Millisecond,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// retry a finite number of times before giving up.
	for i := 0; i < 10; i++ {
		err := f()
		if err == nil {
			return nil
		}

		if isRetryable(err) {
			log.DebugContext(ctx, "User operation failed, retrying", "error", err)
		} else {
			return trace.Wrap(err)
		}

		linear.Inc()
		select {
		case <-linear.After():
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
	return trace.Wrap(err, "too many retries")
}

// isRetryable returns true if an error can be retried.
func isRetryable(err error) bool {
	var pgErr *pgconn.PgError
	err = trace.Unwrap(err)
	if errors.As(err, &pgErr) {
		// https://www.postgresql.org/docs/current/mvcc-serialization-failure-handling.html
		switch pgErr.Code {
		case pgerrcode.DeadlockDetected, pgerrcode.SerializationFailure,
			pgerrcode.UniqueViolation, pgerrcode.ExclusionViolation:
			return true
		case pgerrcode.InternalError:
			if isInternalErrorRetryable(pgErr) {
				return true
			}
		}
	}
	return pgconn.SafeToRetry(err)
}

// isInternalErrorRetryable returns true if an internal error (code XX000)
// should be retried.
func isInternalErrorRetryable(err error) bool {
	errMsg := err.Error()
	// Redshift reports this with a vague SQLSTATE XX000, which is the internal
	// error code, but this is a serialization error that rolls back the
	// transaction, so it should be retried.
	if strings.Contains(errMsg, "conflict with concurrent transaction") {
		return true
	}
	// Postgres this can happen if transaction A tries to revoke or grant privileges
	// concurrent with transaction B.
	if strings.Contains(errMsg, "tuple concurrently updated") {
		return true
	}
	return false
}
