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

package mysql

import (
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// activateUserDetails contains details about the user activation request and
// will be marshaled into a JSON parameter for the stored procedure.
type activateUserDetails struct {
	// Roles is a list of roles to be assigned to the user.
	//
	// MySQL stored procedure does not accept array of strings thus using a
	// JSON to bypass this limit.
	Roles []string `json:"roles"`
	// AuthOptions specifies auth options like "IDENTIFIED xxx" used when
	// creating a new user.
	//
	// Using a JSON string can bypass VARCHAR character limit.
	AuthOptions string `json:"auth_options"`
	// Attributes specifies user attributes used when creating a new user.
	//
	// User attributes is a MySQL JSON in MySQL databases.
	//
	// To check current user's attribute:
	// SELECT * FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE CONCAT(USER, '@', HOST) = current_user()
	//
	// Reference:
	// https://dev.mysql.com/doc/refman/8.0/en/information-schema-user-attributes-table.html
	Attributes struct {
		// User is the original Teleport user name.
		//
		// Find a Teleport user (with "admin" privilege):
		// SELECT * FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE ATTRIBUTE->"$.user" = "teleport-user-name";
		User string `json:"user"`
	} `json:"attributes"`
}

// clientConn is a wrapper of client.Conn.
type clientConn struct {
	*client.Conn
}

func (c *clientConn) executeAndCloseResult(command string, args ...any) error {
	result, err := c.Execute(command, args...)
	if result != nil {
		result.Close()
	}
	return trace.Wrap(err)
}

// ActivateUser creates or enables the database user.
func (e *Engine) ActivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser() == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	conn, err := e.connectAsAdminUser(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	// Ensure the roles meet spec.
	if err := checkRoles(conn, sessionCtx.DatabaseRoles); err != nil {
		return trace.Wrap(err)
	}

	// Setup "teleport-auto-user" and stored procedures.
	if err := e.setupDatabaseForAutoUsers(conn, sessionCtx); err != nil {
		return trace.Wrap(err)
	}

	// Use "tp-<hash>" in case DatabaseUser is over max username length.
	sessionCtx.DatabaseUser = maybeHashUsername(sessionCtx.DatabaseUser, maxUsernameLength(conn))
	e.Log.Infof("Activating MySQL user %q with roles %v for %v.", sessionCtx.DatabaseUser, sessionCtx.DatabaseRoles, sessionCtx.Identity.Username)

	// Prep JSON.
	details, err := makeActivateUserDetails(sessionCtx, sessionCtx.Identity.Username)
	if err != nil {
		return trace.Wrap(err)
	}

	// Call activate.
	err = conn.executeAndCloseResult(
		fmt.Sprintf("CALL %s(?, ?)", activateUserProcedureName),
		sessionCtx.DatabaseUser,
		details,
	)
	if err == nil {
		return nil
	}

	e.Log.Debugf("Call teleport_activate_user failed: %v", err)
	return trace.Wrap(convertActivateError(sessionCtx, err))
}

// DeactivateUser disables the database user.
func (e *Engine) DeactivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser() == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	conn, err := e.connectAsAdminUser(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	e.Log.Infof("Deactivating MySQL user %q for %v.", sessionCtx.DatabaseUser, sessionCtx.Identity.Username)

	err = conn.executeAndCloseResult(
		fmt.Sprintf("CALL %s(?)", deactivateUserProcedureName),
		sessionCtx.DatabaseUser,
	)

	if getSQLState(err) == sqlStateActiveUser {
		e.Log.Debugf("Failed to deactivate user %q: %v.", sessionCtx.DatabaseUser, err)
		return nil
	}
	return trace.Wrap(err)
}

// DeleteUser deletes the database user.
func (e *Engine) DeleteUser(ctx context.Context, sessionCtx *common.Session) error {
	// TODO(gabrielcorado): implement delete database user. for now, just
	// fallback to deactivate user.
	return e.DeactivateUser(ctx, sessionCtx)
}

func (e *Engine) connectAsAdminUser(ctx context.Context, sessionCtx *common.Session) (*clientConn, error) {
	adminSessionCtx := sessionCtx.WithUserAndDatabase(
		sessionCtx.Database.GetAdminUser(),
		defaultSchema(sessionCtx),
	)
	conn, err := e.connect(ctx, adminSessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clientConn{
		Conn: conn,
	}, nil
}

func (e *Engine) setupDatabaseForAutoUsers(conn *clientConn, sessionCtx *common.Session) error {
	// TODO MariaDB requires separate stored procedures to handle auto user:
	// - Max user length is different.
	// - MariaDB uses mysql.roles_mapping instead of mysql.role_edges.
	// - MariaDB cannot set all roles as default role at the same time.
	// - MariaDB does not have user attributes. Will need another way for
	//   saving original Teleport user names. For example, a separate table can
	//   be used to track User -> JSON attribute mapping (protected view with
	//   row level security can be used in addition so each user can only read
	//   their own attributes, if needed).
	if isMariaDB(conn) {
		return trace.NotImplemented("auto user provisioning is not supported for MariaDB yet")
	}

	// Create "teleport-auto-user".
	err := conn.executeAndCloseResult(fmt.Sprintf("CREATE ROLE IF NOT EXISTS %q", teleportAutoUserRole))
	if err != nil {
		return trace.Wrap(err)
	}

	// There is no single command in MySQL to "CREATE OR REPLACE". Instead,
	// have to DROP first before CREATE.
	//
	// To speed up the setup, the procedure "version" is stored as the
	// procedure comment. So check if an update is necessary first by checking
	// these comments.
	//
	// To force an update, drop one of the procedures or update the comment:
	// ALTER PROCEDURE teleport_activate_user COMMENT 'need update'
	if required, err := isProcedureUpdateRequired(conn, defaultSchema(sessionCtx), procedureVersion); err != nil {
		return trace.Wrap(err)
	} else if !required {
		return nil
	}

	// If update is necessary, do a transaction.
	e.Log.Debugf("Updating stored procedures for MySQL server %s.", sessionCtx.Database.GetName())
	return trace.Wrap(doTransaction(conn, func() error {
		for _, procedure := range allProcedures {
			dropCommand := fmt.Sprintf("DROP PROCEDURE IF EXISTS %s", procedure.name)
			updateCommand := fmt.Sprintf("ALTER PROCEDURE %s COMMENT %q", procedure.name, procedureVersion)

			if err := conn.executeAndCloseResult(dropCommand); err != nil {
				return trace.Wrap(err)
			}
			if err := conn.executeAndCloseResult(procedure.createCommand); err != nil {
				return trace.Wrap(err)
			}
			if err := conn.executeAndCloseResult(updateCommand); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}))
}

func getSQLState(err error) string {
	var mysqlError *mysql.MyError
	if !errors.As(err, &mysqlError) {
		return ""
	}
	return mysqlError.State
}

func convertActivateError(sessionCtx *common.Session, err error) error {
	// This operation-failed message usually appear when the user already
	// exists. A different error would be raised if the admin user has no
	// permission to "CREATE USER".
	if strings.Contains(err.Error(), "Operation CREATE USER failed") {
		return trace.AlreadyExists("user %q already exists in this MySQL database and is not managed by Teleport", sessionCtx.DatabaseUser)
	}

	switch getSQLState(err) {
	case sqlStateUsernameDoesNotMatch:
		return trace.AlreadyExists("username %q (Teleport user %q) already exists in this MySQL database and is used for another Teleport user.", sessionCtx.Identity.Username, sessionCtx.DatabaseUser)

	case sqlStateRolesChanged:
		return trace.CompareFailed("roles for user %q has changed. Please quit all active connections and try again.", sessionCtx.Identity.Username)

	default:
		return trace.Wrap(err)
	}
}

// defaultSchema returns the default database to log into as the admin user.
//
// Use a default database/schema to make sure procedures are always created and
// called from there (and possibly store other data there in the future).
//
// This also avoids "No database selected" errors if client doesn't provide
// one.
func defaultSchema(_ *common.Session) string {
	// Use "mysql" as the default schema as both MySQL and Mariadb have it.
	//
	// TODO consider allowing user to specify the default database through database
	// definition.
	return "mysql"
}

func isMariaDB(conn *clientConn) bool {
	return strings.Contains(strings.ToLower(conn.GetServerVersion()), "mariadb")
}

// maxUsernameLength returns the username/role character limit.
func maxUsernameLength(conn *clientConn) int {
	if isMariaDB(conn) {
		return mariadbMaxUsernameLength
	}
	return mysqlMaxUsernameLength
}

func checkRoles(conn *clientConn, roles []string) error {
	maxRoleLength := maxUsernameLength(conn)
	for _, role := range roles {
		if len(role) > maxRoleLength {
			return trace.BadParameter("role %q exceeds maximum length limit of %d", role, maxRoleLength)
		}
	}
	return nil
}

func maybeHashUsername(teleportUser string, maxUsernameLength int) string {
	if len(teleportUser) <= maxUsernameLength {
		return teleportUser
	}

	// Use sha1 to reduce chance of collision.
	hash := sha1.New()
	hash.Write([]byte(teleportUser))

	// Use a prefix to identify the user is managed by Teleport.
	return "tp-" + base64.RawStdEncoding.EncodeToString(hash.Sum(nil))
}

func authOptions(sessionCtx *common.Session) string {
	switch sessionCtx.Database.GetType() {
	case types.DatabaseTypeRDS:
		return `IDENTIFIED WITH AWSAuthenticationPlugin AS "RDS"`

	case types.DatabaseTypeSelfHosted:
		return fmt.Sprintf(`REQUIRE SUBJECT "/CN=%s"`, sessionCtx.DatabaseUser)

	default:
		return ""
	}
}

func makeActivateUserDetails(sessionCtx *common.Session, teleportUser string) (json.RawMessage, error) {
	details := activateUserDetails{
		Roles:       sessionCtx.DatabaseRoles,
		AuthOptions: authOptions(sessionCtx),
	}

	// Save original username as user attributes in case the name is hashed.
	details.Attributes.User = teleportUser

	data, err := json.Marshal(details)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return json.RawMessage(data), nil
}

func isProcedureUpdateRequired(conn *clientConn, wantSchema, wantVersion string) (bool, error) {
	// information_schema.routines is accessible for users/roles with EXECUTE
	// permission.
	result, err := conn.Execute(fmt.Sprintf(
		"SELECT ROUTINE_NAME FROM information_schema.routines WHERE ROUTINE_SCHEMA = %q AND ROUTINE_COMMENT = %q",
		wantSchema,
		wantVersion,
	))
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer result.Close()

	if result.RowNumber() < len(allProcedures) {
		return true, nil
	}

	// Paranoia, make sure the names match.
	foundProcedures := make([]string, 0, result.RowNumber())
	for row := range result.Values {
		procedure, err := result.GetString(row, 0)
		if err != nil {
			return false, trace.Wrap(err)
		}

		foundProcedures = append(foundProcedures, procedure)
	}
	return !allProceduresFound(foundProcedures), nil
}

func allProceduresFound(foundProcedures []string) bool {
	for _, wantProcedure := range allProcedures {
		if !slices.Contains(foundProcedures, wantProcedure.name) {
			return false
		}
	}
	return true
}

func doTransaction(conn *clientConn, do func() error) error {
	if err := conn.Begin(); err != nil {
		return trace.Wrap(err)
	}

	if err := do(); err != nil {
		return trace.NewAggregate(err, conn.Rollback())
	}

	return trace.Wrap(conn.Commit())
}

const (
	// procedureVersion is a hard-coded string that is set as procedure
	// comments to indicate the procedure version.
	procedureVersion = "teleport-auto-user-v1"

	// mysqlMaxUsernameLength is the maximum username length for MySQL.
	//
	// https://dev.mysql.com/doc/refman/8.0/en/user-names.html
	mysqlMaxUsernameLength = 32
	// mariadbMaxUsernameLength is the maximum username length for MariaDB.
	//
	// https://mariadb.com/kb/en/identifier-names/#maximum-length
	mariadbMaxUsernameLength = 80

	// teleportAutoUserRole is the name of a MySQL role that all Teleport
	// managed users will be a part of.
	//
	// To find all users that assigned this role:
	// SELECT TO_USER AS 'Teleport Managed Users' FROM mysql.role_edges WHERE FROM_USER = 'teleport-auto-user'
	teleportAutoUserRole = "teleport-auto-user"

	// sqlStateActiveUser is the SQLSTATE raised by deactivation procedure when
	// user has active connections.
	//
	// SQLSTATE reference:
	// https://en.wikipedia.org/wiki/SQLSTATE
	sqlStateActiveUser = "TP000"
	// sqlStateUsernameDoesNotMatch is the SQLSTATE raised by activation
	// procedure when the Teleport username does not match user's attributes.
	//
	// Possibly there is a hash collision, or someone manually updated the user
	// attributes.
	sqlStateUsernameDoesNotMatch = "TP001"
	// sqlStateRolesChanged is the SQLSTATE raised by activation procedure when
	// the user has active connections but roles has changed.
	sqlStateRolesChanged = "TP002"

	revokeRolesProcedureName    = "teleport_revoke_roles"
	activateUserProcedureName   = "teleport_activate_user"
	deactivateUserProcedureName = "teleport_deactivate_user"
)

var (
	//go:embed mysql_activate_user.sql
	activateUserProcedure string
	//go:embed mysql_deactivate_user.sql
	deactivateUserProcedure string
	//go:embed mysql_revoke_roles.sql
	revokeRolesProcedure string

	allProcedures = []struct {
		name          string
		createCommand string
	}{
		{
			name:          revokeRolesProcedureName,
			createCommand: revokeRolesProcedure,
		},
		{
			name:          activateUserProcedureName,
			createCommand: activateUserProcedure,
		},
		{
			name:          deactivateUserProcedureName,
			createCommand: deactivateUserProcedure,
		},
	}
)
