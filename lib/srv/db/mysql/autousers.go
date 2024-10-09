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

package mysql

import (
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/go-mysql-org/go-mysql/client"
	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
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
	// To check current user's attribute for MySQL:
	// SELECT * FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE CONCAT(USER, '@', HOST) = current_user()
	//
	// Reference:
	// https://dev.mysql.com/doc/refman/8.0/en/information-schema-user-attributes-table.html
	Attributes struct {
		// User is the original Teleport user name.
		//
		// Find a Teleport user (with "admin" privilege) for MySQL:
		// SELECT * FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE ATTRIBUTE->"$.user" = "teleport-user-name";
		//
		// Find a Teleport user (with "admin" privilege) for MariaDB:
		// SELECT * FROM teleport.user_attributes WHERE JSON_VALUE(Attributes,"$.user") = "teleport-user-name";
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

func (c *clientConn) isMariaDB() bool {
	return strings.Contains(strings.ToLower(c.GetServerVersion()), "mariadb")
}

// maxUsernameLength returns the username character limit.
func (c *clientConn) maxUsernameLength() int {
	if c.isMariaDB() {
		return mariadbMaxUsernameLength
	}
	return mysqlMaxUsernameLength
}

// maxRoleLength returns the role character limit.
func (c *clientConn) maxRoleLength() int {
	if c.isMariaDB() {
		return mariadbMaxRoleLength
	}
	// Same username vs role length for MySQL.
	return mysqlMaxUsernameLength
}

// Close calls conn.Quit to send COM_QUIT then close the conn.
func (c *clientConn) Close() error {
	return trace.Wrap(c.Conn.Quit())
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

	conn, err := e.connectAsAdminUser(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	// Ensure version is supported.
	if err := checkSupportedVersion(ctx, e.Log, conn); err != nil {
		return trace.Wrap(err)
	}

	// Ensure the roles meet spec.
	if err := checkRoles(conn, sessionCtx.DatabaseRoles); err != nil {
		return trace.Wrap(err)
	}

	// Setup "teleport-auto-user" and stored procedures.
	if err := e.setupDatabaseForAutoUsers(conn, sessionCtx); err != nil {
		return trace.Wrap(err)
	}

	// Use "tp-<hash>" in case DatabaseUser is over max username length.
	sessionCtx.DatabaseUser = maybeHashUsername(sessionCtx.DatabaseUser, conn.maxUsernameLength())
	e.Log.InfoContext(e.Context, "Activating MySQL user.", "user", sessionCtx.DatabaseUser, "roles", sessionCtx.DatabaseRoles, "identity", sessionCtx.Identity.Username)

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
	if err != nil {
		e.Log.DebugContext(e.Context, "Call teleport_activate_user failed.", "error", err)
		err = convertActivateError(sessionCtx, err)
		e.Audit.OnDatabaseUserCreate(ctx, sessionCtx, err)
		return trace.Wrap(err)
	}
	e.Audit.OnDatabaseUserCreate(ctx, sessionCtx, nil)
	return nil
}

// DeactivateUser disables the database user.
func (e *Engine) DeactivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	conn, err := e.connectAsAdminUser(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	e.Log.InfoContext(e.Context, "Deactivating MySQL user.", "user", sessionCtx.DatabaseUser, "identity", sessionCtx.Identity.Username)

	err = conn.executeAndCloseResult(
		fmt.Sprintf("CALL %s(?)", deactivateUserProcedureName),
		sessionCtx.DatabaseUser,
	)

	if getSQLState(err) == common.SQLStateActiveUser {
		e.Log.DebugContext(e.Context, "Failed to deactivate user.", "user", sessionCtx.DatabaseUser, "error", err)
		return nil
	}

	e.Audit.OnDatabaseUserDeactivate(ctx, sessionCtx, false, err)
	return trace.Wrap(err)
}

// DeleteUser deletes the database user.
func (e *Engine) DeleteUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	conn, err := e.connectAsAdminUser(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	e.Log.InfoContext(e.Context, "Deleting MySQL user.", "database_user", sessionCtx.DatabaseUser, "identity", sessionCtx.Identity.Username)

	result, err := conn.Execute(fmt.Sprintf("CALL %s(?)", deleteUserProcedureName), sessionCtx.DatabaseUser)
	if err != nil {
		if getSQLState(err) == common.SQLStateActiveUser {
			e.Log.DebugContext(e.Context, "Failed to delete user.", "user", sessionCtx.DatabaseUser, "error", err)
			return nil
		}

		e.Audit.OnDatabaseUserDeactivate(ctx, sessionCtx, true, err)
		return trace.Wrap(err)
	}
	defer result.Close()

	deleted := true
	switch readDeleteUserResult(result) {
	case common.SQLStateUserDropped:
		e.Log.DebugContext(e.Context, "User deleted successfully.", "user", sessionCtx.DatabaseUser)
	case common.SQLStateUserDeactivated:
		e.Log.InfoContext(e.Context, "Unable to delete user, it was disabled instead.", "user", sessionCtx.DatabaseUser)
		deleted = false
	default:
		e.Log.WarnContext(e.Context, "Unable to determine user deletion state.", "user", sessionCtx.DatabaseUser)
	}
	e.Audit.OnDatabaseUserDeactivate(ctx, sessionCtx, deleted, nil)

	return trace.Wrap(err)
}

func (e *Engine) connectAsAdminUser(ctx context.Context, sessionCtx *common.Session) (*clientConn, error) {
	adminSessionCtx := sessionCtx.WithUserAndDatabase(
		sessionCtx.Database.GetAdminUser().Name,
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
	e.Log.DebugContext(e.Context, "Updating stored procedures for MySQL server.", "database", sessionCtx.Database.GetName())
	return trace.Wrap(doTransaction(conn, func() error {
		for _, procedureName := range allProcedureNames {
			dropCommand := fmt.Sprintf("DROP PROCEDURE IF EXISTS %s", procedureName)
			createCommand, found := getCreateProcedureCommand(conn, procedureName)
			updateCommand := fmt.Sprintf("ALTER PROCEDURE %s COMMENT %q", procedureName, procedureVersion)

			if !found {
				continue
			}

			if err := conn.executeAndCloseResult(dropCommand); err != nil {
				return trace.Wrap(err)
			}
			if err := conn.executeAndCloseResult(createCommand); err != nil {
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
	case common.SQLStateUsernameDoesNotMatch:
		return trace.AlreadyExists("username %q (Teleport user %q) already exists in this MySQL database and is used for another Teleport user.", sessionCtx.Identity.Username, sessionCtx.DatabaseUser)

	case common.SQLStateRolesChanged:
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
func defaultSchema(sessionCtx *common.Session) string {
	// Aurora MySQL does not allow procedures on built-in "mysql" database.
	// Technically we can use another built-in database like "sys". However,
	// AWS (or database admins for self-hosted) may restrict permissions on
	// these built-in databases eventually. Well, each built-in database has
	// its own purpose.
	//
	// Thus lets use a teleport-specific database. This database should be
	// created when configuring the admin user. The admin user should be
	// granted the following permissions for this database:
	// GRANT ALTER ROUTINE, CREATE ROUTINE, EXECUTE ON teleport.* TO '<admin-user>'
	adminUser := sessionCtx.Database.GetAdminUser()
	if adminUser.DefaultDatabase != "" {
		return adminUser.DefaultDatabase
	}
	return "teleport"
}

func checkRoles(conn *clientConn, roles []string) error {
	maxRoleLength := conn.maxRoleLength()
	for _, role := range roles {
		if len(role) > maxRoleLength {
			return trace.BadParameter("role %q exceeds maximum length limit of %d", role, maxRoleLength)
		}
	}
	return nil
}

func checkSupportedVersion(ctx context.Context, log *slog.Logger, conn *clientConn) error {
	if conn.isMariaDB() {
		return trace.Wrap(checkMariaDBSupportedVersion(ctx, log, conn.GetServerVersion()))
	}
	return trace.Wrap(checkMySQLSupportedVersion(ctx, log, conn.GetServerVersion()))
}

func checkMySQLSupportedVersion(ctx context.Context, log *slog.Logger, serverVersion string) error {
	ver, err := semver.NewVersion(serverVersion)
	switch {
	case err != nil:
		log.DebugContext(ctx, "Invalid MySQL server version. Assuming role management is supported.", "server_version", serverVersion)
		return nil

	// Reference:
	// https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-0.html#mysqld-8-0-0-account-management
	case ver.Major < 8:
		return trace.BadParameter("automatic user provisioning is not supported for MySQL servers older than 8.0")

	default:
		return nil
	}
}

func checkMariaDBSupportedVersion(ctx context.Context, log *slog.Logger, serverVersion string) error {
	// serverVersion may look like these:
	// 5.5.5-10.7.8-MariaDB-1:10.7.8+maria~ubu2004
	// 5.5.5-10.11.5-MariaDB
	// 11.0.3-MariaDB-1:11.0.3+maria~ubu2204
	//
	// Note that the "5.5.5-" prefix (aka "replication version hack") was
	// introduced when MariaDB bumped the major version to 10. The prefix is
	// removed in version 11. References:
	// https://stackoverflow.com/questions/56601304/what-does-the-first-part-of-the-mariadb-version-string-mean
	// https://github.com/php/php-src/pull/7963
	serverVersion, _, _ = strings.Cut(serverVersion, "-MariaDB")
	serverVersion = strings.TrimPrefix(serverVersion, "5.5.5-")

	ver, err := semver.NewVersion(serverVersion)
	switch {
	case err != nil:
		log.DebugContext(ctx, "Invalid MariaDB server version. Assuming role management is supported.", "server_version", serverVersion)
		return nil

	case ver.Major > 10:
		return nil
	case ver.Major < 10:
		return trace.BadParameter("automatic user provisioning is not supported for MariaDB servers older than 10")

	// ver.Major == 10
	//
	// Versions below 10.3.3, 10.2.11 get a weird syntax error when running the
	// stored procedures. These are fairly old versions from 2017.
	case ver.Minor == 3 && ver.Patch < 3:
		return trace.BadParameter("automatic user provisioning is not supported for MariaDB servers older than 10.3.3")
	case ver.Minor == 2 && ver.Patch < 11:
		return trace.BadParameter("automatic user provisioning is not supported for MariaDB servers older than 10.2.11")
	case ver.Minor < 2:
		return trace.BadParameter("automatic user provisioning is not supported for MariaDB servers older than 10.2")

	default:
		return nil
	}
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

	if result.RowNumber() < len(allProcedureNames) {
		return true, nil
	}

	// Double check if all procedures are in place, this ensures that newly
	// added procedures will be created even when there is no version bump.
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
	for _, wantProcedureName := range allProcedureNames {
		if !slices.Contains(foundProcedures, wantProcedureName) {
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

func readDeleteUserResult(res *mysql.Result) string {
	if res == nil || res.Resultset == nil ||
		len(res.Resultset.Values) != 1 || len(res.Resultset.Values[0]) != 1 {
		return ""
	}
	return string(res.Resultset.Values[0][0].AsString())
}

func getCreateProcedureCommand(conn *clientConn, procedureName string) (string, bool) {
	if conn.isMariaDB() {
		command, found := mariadbProcedures[procedureName]
		return command, found
	}
	command, found := mysqlProcedures[procedureName]
	return command, found
}

const (
	// procedureVersion is a hard-coded string that is set as procedure
	// comments to indicate the procedure version.
	procedureVersion = "teleport-auto-user-v4"

	// mysqlMaxUsernameLength is the maximum username/role length for MySQL.
	//
	// https://dev.mysql.com/doc/refman/8.0/en/user-names.html
	mysqlMaxUsernameLength = 32
	// mariadbMaxUsernameLength is the maximum username length for MariaDB.
	//
	// https://mariadb.com/kb/en/identifier-names/#maximum-length
	mariadbMaxUsernameLength = 80
	// mariadbMaxRoleLength is the maximum role length for MariaDB.
	mariadbMaxRoleLength = 128

	// teleportAutoUserRole is the name of a MySQL role that all Teleport
	// managed users will be a part of.
	//
	// To find all users that assigned this role for MySQL:
	// SELECT TO_USER AS 'Teleport Managed Users' FROM mysql.role_edges WHERE FROM_USER = 'teleport-auto-user'
	//
	// To find all users that assigned this role for MariaDB:
	// SELECT USER AS 'Teleport Managed Users' FROM mysql.roles_mapping WHERE ROLE = 'teleport-auto-user' AND Admin_option = 'N'
	teleportAutoUserRole = "teleport-auto-user"

	revokeRolesProcedureName    = "teleport_revoke_roles"
	activateUserProcedureName   = "teleport_activate_user"
	deactivateUserProcedureName = "teleport_deactivate_user"
	deleteUserProcedureName     = "teleport_delete_user"
)

var (
	//go:embed sql/mysql_activate_user.sql
	activateUserProcedure string
	//go:embed sql/mysql_deactivate_user.sql
	deactivateUserProcedure string
	//go:embed sql/mysql_revoke_roles.sql
	revokeRolesProcedure string
	//go:embed sql/mysql_delete_user.sql
	deleteProcedure string

	//go:embed sql/mariadb_activate_user.sql
	mariadbActivateUserProcedure string
	//go:embed sql/mariadb_deactivate_user.sql
	mariadbDeactivateUserProcedure string
	//go:embed sql/mariadb_revoke_roles.sql
	mariadbRevokeRolesProcedure string
	//go:embed sql/mariadb_delete_user.sql
	mariadbDeleteProcedure string

	// allProcedureNames contains a list of all procedures required to setup
	// auto-user provisioning. Note that order matters here as later procedures
	// may depend on the earlier ones.
	allProcedureNames = []string{
		revokeRolesProcedureName,
		activateUserProcedureName,
		deactivateUserProcedureName,
		deleteUserProcedureName,
	}

	// mysqlProcedures maps procedure names to the procedures used for MySQL.
	mysqlProcedures = map[string]string{
		activateUserProcedureName:   activateUserProcedure,
		deactivateUserProcedureName: deactivateUserProcedure,
		revokeRolesProcedureName:    revokeRolesProcedure,
		deleteUserProcedureName:     deleteProcedure,
	}

	// mariadbProcedures maps procedure names to the procedures used for MariaDB.
	//
	// MariaDB requires separate procedures from MySQL because:
	// - Max length of username/role is 80/128 instead of 32.
	// - MariaDB uses mysql.roles_mapping instead of mysql.role_edges.
	//   Note that roles_mapping tracks both role assignments and role "owners".
	//   "Owners" are tracked in rows with Admin_option = 'Y'.
	// - MariaDB does not have built-in user attributes field. Instead, the
	//   procedure will create an `user_attributes` table for tracking this.
	// - MariaDB cannot `SET DEFAULT ROLE ALL`. To workaround this, a role
	//   `tp-role-<username>` is created and assigned to the database user
	//   while all roles are assigned to this all-in-one role. Then `SET
	//   DEFAULT ROLE tp-role-<username>` before each session.
	//
	// Other MariaDB quirks:
	// - In order to be able to grant a role, the grantor doing so must have
	//   permission to do so (see WITH ADMIN in the CREATE ROLE article).
	//   (Quoted from https://mariadb.com/kb/en/grant/#roles)
	// - To set a default role for another user one needs to have write access
	//   to the mysql database.
	//   (Quoted from https://mariadb.com/kb/en/set-default-role/)
	mariadbProcedures = map[string]string{
		activateUserProcedureName:   mariadbActivateUserProcedure,
		deactivateUserProcedureName: mariadbDeactivateUserProcedure,
		revokeRolesProcedureName:    mariadbRevokeRolesProcedure,
		deleteUserProcedureName:     mariadbDeleteProcedure,
	}
)
