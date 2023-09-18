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
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/go-mysql-org/go-mysql/client"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func (e Engine) connectAsAdminUser(ctx context.Context, sessionCtx *common.Session) (*client.Conn, error) {
	adminSessionCtx := sessionCtx.WithUserAndDatabase(
		sessionCtx.Database.GetAdminUser(),
		// Always use "mysql" as database name to make sure procedures are
		// created and called from there.
		// This also avoids "No database selected" errors if client doesn't
		// provide one.
		"mysql",
	)
	conn, err := e.connect(ctx, adminSessionCtx)
	return conn, trace.Wrap(err)
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

	// Setup "teleport-auto-user" and stored procedures.
	if err := e.setupDatabaseForAutoUsers(conn, sessionCtx); err != nil {
		return trace.Wrap(err)
	}

	// Use "teleport-<hash>" in case DatabaseUser is over max username length.
	sessionCtx.DatabaseUser = maybeHashUsername(sessionCtx.DatabaseUser, maxUsernameLength(conn))

	e.Log.Infof("Activating MySQL user %q with roles %v for %v.", sessionCtx.DatabaseUser, sessionCtx.DatabaseRoles, sessionCtx.Identity.Username)

	details, err := makeActivateUserDetails(sessionCtx, sessionCtx.Identity.Username)
	if err != nil {
		return trace.Wrap(err)
	}

	callCommand := fmt.Sprintf("CALL %s('%s', '%s')", activateUserProcedureName, sessionCtx.DatabaseUser, details)

	_, err = conn.Execute(callCommand)
	if err != nil {
		if strings.Contains(err.Error(), "Operation CREATE USER failed") {
			return trace.AlreadyExists("user %q already exists in this MySQL database and is not managed by Teleport", sessionCtx.DatabaseUser)
		}
		return trace.Wrap(err)
	}
	return nil
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

	_, err = conn.Execute(fmt.Sprintf("CALL %s('%s')", deactivateUserProcedureName, sessionCtx.DatabaseUser))
	return trace.Wrap(err)
}

func isMariaDB(conn *client.Conn) bool {
	return strings.Contains(strings.ToLower(conn.GetServerVersion()), "mariadb")
}

// maxUsernameLength returns the username character limit.
func maxUsernameLength(conn *client.Conn) int {
	if isMariaDB(conn) {
		return mariadbMaxUsernameLength
	}
	return mysqlMaxUsernameLength
}

func maybeHashUsername(username string, maxUsernameLength int) string {
	if len(username) <= maxUsernameLength {
		return username
	}

	// 64 bit hash collision rates:
	// - 200 entries: 1 in 10^15
	// -  2k entries: 1 in 10 trillion
	// - 20k entries: 1 in 100 billion
	hash := fnv.New64()
	hash.Write([]byte(username))

	// Use a prefix to identify the user is managed by teleport.
	return "teleport-" + hex.EncodeToString(hash.Sum(nil))
}

// activateUserDetails contains details for activating an user.
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
	// Find a Teleport user (with "admin" privilege):
	// SELECT * FROM INFORMATION_SCHEMA.USER_ATTRIBUTES WHERE ATTRIBUTE->"$.user" = "teleport-user-name";
	//
	// Reference:
	// https://dev.mysql.com/doc/refman/8.0/en/information-schema-user-attributes-table.html
	Attributes struct {
		// User is the original Teleport user name.
		User string `json:"user"`
	} `json:"attributes"`
}

// makeActivateUserDetails creates a MySQL JSON string that can be single
// quoted and used for a stored procedure.
func makeActivateUserDetails(sessionCtx *common.Session, teleportUsername string) (string, error) {
	details := activateUserDetails{
		Roles:       sessionCtx.DatabaseRoles,
		AuthOptions: authOptions(sessionCtx),
	}

	// Save original username as user attributes in case the name is hashed.
	details.Attributes.User = teleportUsername

	json, err := json.Marshal(details)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Escape "\".
	return strings.ReplaceAll(string(json), "\\", "\\\\"), nil
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

func (e *Engine) setupDatabaseForAutoUsers(conn *client.Conn, sessionCtx *common.Session) error {
	// TODO MariaDB requires separate stored procedures to handle auto user:
	// - Max user length is different.
	// - MariaDB uses mysql.roles_mapping instead of mysql.role_edges.
	// - MariaDB cannot set all roles as default role at the same time.
	// - MariaDB does not have user attributes. Will need another way for
	//   saving original Teleport user names.
	if isMariaDB(conn) {
		return trace.NotImplemented("auto user provisioning is not supported for MariaDB yet")
	}

	_, err := conn.Execute(fmt.Sprintf("CREATE ROLE IF NOT EXISTS %q", teleportAutoUserRole))
	if err != nil {
		return trace.Wrap(err)
	}

	// There is no single command in MySQL to "CREATE OR REPLACE". Instead,
	// have to DROP first before CREATE.
	//
	// To speed up the setup, the "version" is stored as the procedure comment.
	// So check if an update is necessary first by checking these comments.
	//
	// To force an update, drop one of the procedures or update the comment:
	// ALTER PROCEDURE teleport_activate_user COMMENT 'need update'
	if required, err := isProcedureUpdateRequired(conn, procedureVersion); err != nil {
		return trace.Wrap(err)
	} else if !required {
		return nil
	}

	// If update is necessary, do a transaction.
	e.Log.Debugf("Updating stored procedures for MySQL server %s.", sessionCtx.Database.GetName())
	return trace.Wrap(doTransaction(conn, func() error {
		for _, procedure := range allProcedures {
			_, err := conn.Execute(fmt.Sprintf("DROP PROCEDURE IF EXISTS %s", procedure.name))
			if err != nil {
				return trace.Wrap(err)
			}

			_, err = conn.Execute(procedure.createCommand)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}))
}

func isProcedureUpdateRequired(conn *client.Conn, wantVersion string) (bool, error) {
	// information_schema.routines is accessible for users/roles with EXECUTE
	// permission.
	result, err := conn.Execute(fmt.Sprintf(
		"SELECT ROUTINE_NAME FROM information_schema.routines WHERE ROUTINE_SCHEMA = %q AND ROUTINE_COMMENT = %q",
		"mysql",
		wantVersion,
	))
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer result.Close()

	if result.RowNumber() < len(allProcedures) {
		return true, nil
	}

	// Paranoia. make sure the names match.
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

func doTransaction(conn *client.Conn, do func() error) error {
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
