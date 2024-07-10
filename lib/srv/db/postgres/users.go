/*
Copyright 2023 Gravitational, Inc.

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
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func (e *Engine) connectAsAdmin(ctx context.Context, sessionCtx *common.Session) (*pgx.Conn, error) {
	// Log into GetAdminUser().DefaultDatabase if specified, otherwise use
	// database name from db route.
	loginDatabase := sessionCtx.DatabaseName
	if sessionCtx.Database.GetAdminUser().DefaultDatabase != "" {
		loginDatabase = sessionCtx.Database.GetAdminUser().DefaultDatabase
	}
	conn, err := e.pgxConnect(ctx, sessionCtx.WithUserAndDatabase(sessionCtx.Database.GetAdminUser().Name, loginDatabase))
	return conn, trace.Wrap(err)
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

	conn, err := e.connectAsAdmin(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)

	// We could call this once when the database is being initialized but
	// doing it here has a nice "self-healing" property in case the Teleport
	// bookkeeping group or stored procedures get deleted or changed offband.
	logger := e.Log.WithField("user", sessionCtx.DatabaseUser)
	err = withRetry(ctx, logger, func() error {
		return trace.Wrap(e.initAutoUsers(ctx, sessionCtx, conn))
	})
	if err != nil {
		return trace.Wrap(err)
	}

	roles, err := prepareRoles(sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	logger.WithField("roles", roles).Info("Activating PostgreSQL user")
	err = withRetry(ctx, logger, func() error {
		_, err = conn.Exec(ctx, activateQuery, sessionCtx.DatabaseUser, roles)
		return trace.Wrap(err)
	})
	if err != nil {
		logger.WithError(err).Debug("Call teleport_activate_user failed.")
		errOut := convertActivateError(sessionCtx, err)
		return trace.Wrap(errOut)
	}
	return nil
}

// DeactivateUser disables the database user.
func (e *Engine) DeactivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	conn, err := e.connectAsAdmin(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)

	logger := e.Log.WithField("user", sessionCtx.DatabaseUser)
	logger.Info("Deactivating PostgreSQL user.")
	err = withRetry(ctx, logger, func() error {
		_, err = conn.Exec(ctx, deactivateQuery, sessionCtx.DatabaseUser)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteUser deletes the database user.
func (e *Engine) DeleteUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	conn, err := e.connectAsAdmin(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close(ctx)

	logger := e.Log.WithField("user", sessionCtx.DatabaseUser)
	logger.Info("Deleting PostgreSQL user.")

	var state string
	err = withRetry(ctx, logger, func() error {
		switch {
		case sessionCtx.Database.IsRedshift():
			return trace.Wrap(e.deleteUserRedshift(ctx, sessionCtx, conn, &state))
		default:
			return trace.Wrap(conn.QueryRow(ctx, deleteQuery, sessionCtx.DatabaseUser).Scan(&state))
		}
	})
	if err != nil {
		return trace.Wrap(err)
	}

	switch state {
	case common.SQLStateUserDropped:
		logger.Debug("User deleted successfully.")
	case common.SQLStateUserDeactivated:
		logger.Info("Unable to delete user, it was disabled instead.")
	default:
		logger.Warn("Unable to determine user deletion state.")
	}

	return nil
}

// deleteUserRedshift deletes the Redshift database user.
//
// Failures inside Redshift default procedures are always rethrown exceptions if
// the exception handler completes successfully. Given this, we need to assert
// into the returned error instead of doing this on state returned (like regular
// PostgreSQL).
func (e *Engine) deleteUserRedshift(ctx context.Context, sessionCtx *common.Session, conn *pgx.Conn, state *string) error {
	_, err := conn.Exec(ctx, deleteQuery, sessionCtx.DatabaseUser)
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

// initAutoUsers installs procedures for activating and deactivating users and
// creates the bookkeeping role for auto-provisioned users.
func (e *Engine) initAutoUsers(ctx context.Context, sessionCtx *common.Session, conn *pgx.Conn) error {
	// Create a role/group which all auto-created users will be a part of.
	_, err := conn.Exec(ctx, fmt.Sprintf("create role %q", teleportAutoUserRole))
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return trace.Wrap(err)
		}
		e.Log.Debugf("PostgreSQL role %q already exists.", teleportAutoUserRole)
	} else {
		e.Log.Debugf("Created PostgreSQL role %q.", teleportAutoUserRole)
	}

	// Install stored procedures for creating and disabling database users.
	for name, sql := range pickProcedures(sessionCtx) {
		_, err := conn.Exec(ctx, sql)
		if err != nil {
			return trace.Wrap(err)
		}
		e.Log.Debugf("Installed PostgreSQL stored procedure %q.", name)
	}
	return nil
}

// pgxConnect connects to the database using pgx driver which is higher-level
// than pgconn and is easier to use for executing queries.
func (e *Engine) pgxConnect(ctx context.Context, sessionCtx *common.Session) (*pgx.Conn, error) {
	config, err := e.getConnectConfig(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgxConf, err := pgx.ParseConfig("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgxConf.Config = *config
	return pgx.ConnectConfig(ctx, pgxConf)
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

	// teleportAutoUserRole is the name of a PostgreSQL role that all Teleport
	// managed users will be a part of.
	teleportAutoUserRole = "teleport-auto-user"
)

var (
	//go:embed sql/activate-user.sql
	activateProc string
	// activateQuery is the query for calling user activation procedure.
	activateQuery = fmt.Sprintf(`call %v($1, $2)`, activateProcName)

	//go:embed sql/deactivate-user.sql
	deactivateProc string
	// deactivateQuery is the query for calling user deactivation procedure.
	deactivateQuery = fmt.Sprintf(`call %v($1)`, deactivateProcName)

	//go:embed sql/delete-user.sql
	deleteProc string
	// deleteQuery is the query for calling user deletion procedure.
	deleteQuery = fmt.Sprintf(`call %v($1)`, deleteProcName)

	//go:embed sql/redshift-activate-user.sql
	redshiftActivateProc string
	//go:embed sql/redshift-deactivate-user.sql
	redshiftDeactivateProc string
	//go:embed sql/redshift-delete-user.sql
	redshiftDeleteProc string

	procs = map[string]string{
		activateProcName:   activateProc,
		deactivateProcName: deactivateProc,
		deleteProcName:     deleteProc,
	}
	redshiftProcs = map[string]string{
		activateProcName:   redshiftActivateProc,
		deactivateProcName: redshiftDeactivateProc,
		deleteProcName:     redshiftDeleteProc,
	}
)

// withRetry is a helper for auto user operations that runs a given func a
// finite number of times until it returns nil error or the given context is
// done.
func withRetry(ctx context.Context, log logrus.FieldLogger, f func() error) error {
	linear, err := retryutils.NewLinear(retryutils.LinearConfig{
		// arbitrarily copied settings from retry logic in lib/backend/pgbk.
		First:  0,
		Step:   100 * time.Millisecond,
		Max:    750 * time.Millisecond,
		Jitter: retryutils.NewHalfJitter(),
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
			log.WithError(err).Debug("User operation failed, retrying")
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
		}
	}
	// Redshift reports this with a vague SQLSTATE XX000, which is the internal
	// error code, but this is a serialization error that rolls back the
	// transaction, so it should be retried.
	if strings.Contains(err.Error(), "conflict with concurrent transaction") {
		return true
	}
	return pgconn.SafeToRetry(err)
}
