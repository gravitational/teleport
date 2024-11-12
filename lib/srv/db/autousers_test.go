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

package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/label"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobjectimportrule"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
)

// TestAutoUsersPostgres verifies automatic database user creation for Postgres.
func TestAutoUsersPostgres(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		mode          types.CreateDatabaseUserMode
		databaseRoles []string

		databasePermissions types.DatabasePermissions
		expectedPermissions postgres.Permissions
		importRuleSpec      *dbobjectimportrulev1.DatabaseObjectImportRuleSpec

		adminDefaultDatabase   string
		connectionErrorMessage string
		expectAdminDatabase    string
	}{
		"activate/deactivate users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			expectAdminDatabase: "user-db",
		},
		"activate/delete users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			databaseRoles:       []string{"reader", "writer"},
			expectAdminDatabase: "user-db",
		},
		"disabled": {
			mode:          types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
			databaseRoles: []string{"reader", "writer"},
			// Given the "alice" user is not present on the database and
			// Teleport won't create it, this should fail with access denied
			// error.
			connectionErrorMessage: "access to db denied. User does not have permissions. Confirm database user and name.",
		},
		"admin user default database": {
			mode:                 types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:        []string{"reader", "writer"},
			adminDefaultDatabase: "admin-db",
			expectAdminDatabase:  "admin-db",
		},
		"roles and permissions are mutually exclusive": {
			mode:          types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			databaseRoles: []string{"reader", "writer"},
			databasePermissions: types.DatabasePermissions{
				{
					Permissions: []string{"SELECT"},
					Match: map[string]apiutils.Strings{
						"can_select": []string{"true"},
					},
				},
			},
			connectionErrorMessage: "fine-grained database permissions and database roles are mutually exclusive, yet both were provided",
			expectAdminDatabase:    "user-db",
		},
		"database permissions are granted": {
			mode: types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			databasePermissions: types.DatabasePermissions{
				{
					Permissions: []string{"SELECT"},
					Match: map[string]apiutils.Strings{
						"can_select": []string{"true"},
					},
				},
			},
			importRuleSpec: &dbobjectimportrulev1.DatabaseObjectImportRuleSpec{
				Priority:       0,
				DatabaseLabels: label.FromMap(map[string][]string{"*": {"*"}}),
				Mappings: []*dbobjectimportrulev1.DatabaseObjectImportRuleMapping{
					{
						// select three tables out of five in the public schema.
						// see handleSchemaInfo() for the effective schema.
						Match: &dbobjectimportrulev1.DatabaseObjectImportMatch{
							TableNames: []string{"orders", "departments", "projects"},
						},
						// select public schema, skipping the hr schema.
						Scope: &dbobjectimportrulev1.DatabaseObjectImportScope{
							SchemaNames: []string{"public"},
						},
						AddLabels: map[string]string{"can_select": "true"},
					},
				},
			},
			expectedPermissions: postgres.Permissions{
				Tables: []postgres.TablePermission{
					{Privilege: "SELECT", Schema: "public", Table: "orders"},
					{Privilege: "SELECT", Schema: "public", Table: "departments"},
					{Privilege: "SELECT", Schema: "public", Table: "projects"},
				},
			},
			expectAdminDatabase: "user-db",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			// At initial setup, only allows postgres (used to create execute the procedures).
			testCtx := setupTestContext(ctx, t, withSelfHostedPostgresUsers("postgres", []string{"postgres"}, func(db *types.DatabaseV3) {
				db.Spec.AdminUser = &types.DatabaseAdminUser{
					Name:            "postgres",
					DefaultDatabase: tc.adminDefaultDatabase,
				}
			}))
			go testCtx.startHandlingConnections()

			// Populate the global database object import rule, if provided.
			if tc.importRuleSpec != nil {
				rule, err := databaseobjectimportrule.NewDatabaseObjectImportRule("dummy", tc.importRuleSpec)
				require.NoError(t, err)
				_, err = testCtx.tlsServer.Auth().CreateDatabaseObjectImportRule(ctx, rule)
				require.NoError(t, err)
			}

			// Create user with role that allows user provisioning.
			_, role, err := auth.CreateUserAndRole(testCtx.tlsServer.Auth(), "alice", []string{"auto"}, nil)
			require.NoError(t, err)
			options := role.GetOptions()
			options.CreateDatabaseUserMode = tc.mode
			role.SetOptions(options)
			role.SetDatabaseRoles(types.Allow, tc.databaseRoles)
			role.SetDatabasePermissions(types.Allow, tc.databasePermissions)
			role.SetDatabaseNames(types.Allow, []string{"*"})
			_, err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// Try to connect to the database as this user.
			pgConn, err := testCtx.postgresClient(ctx, "alice", "postgres", "alice", "user-db")
			if tc.connectionErrorMessage != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.connectionErrorMessage)
				return
			}
			require.NoError(t, err)

			// Verify incoming connections.
			// 1. Admin connecting to admin database
			requirePostgresConnection(t, testCtx.postgres["postgres"].db.ParametersCh(), "postgres", tc.expectAdminDatabase)

			// 2. If there are any database permissions: admin connecting to session database.
			if len(tc.databasePermissions) > 0 {
				// expect two connections to be made.
				requirePostgresConnection(t, testCtx.postgres["postgres"].db.ParametersCh(), "postgres", "user-db")
				requirePostgresConnection(t, testCtx.postgres["postgres"].db.ParametersCh(), "postgres", "user-db")
			}

			// 3. User connecting to session database.
			requirePostgresConnection(t, testCtx.postgres["postgres"].db.ParametersCh(), "alice", "user-db")

			// Verify user was activated.
			select {
			case e := <-testCtx.postgres["postgres"].db.UserEventsCh():
				require.Equal(t, "alice", e.Name)
				require.Equal(t, tc.databaseRoles, e.Roles)
				require.True(t, e.Active)
			case <-time.After(5 * time.Second):
				t.Fatal("user not activated after 5s")
			}

			// Verify proper permissions were granted
			if len(tc.databasePermissions) > 0 {
				select {
				case e := <-testCtx.postgres["postgres"].db.UserPermissionsCh():
					require.Equal(t, "alice", e.Name)
					require.Equal(t, tc.expectedPermissions, e.Permissions)
				case <-time.After(5 * time.Second):
					t.Fatal("user permissions not updated after 5s")
				}
			}

			// Disconnect.
			err = pgConn.Close(ctx)
			require.NoError(t, err)

			// Verify user was deactivated.
			select {
			case e := <-testCtx.postgres["postgres"].db.UserEventsCh():
				require.Equal(t, "alice", e.Name)
				require.False(t, e.Active)
			case <-time.After(5 * time.Second):
				t.Fatal("user not deactivated after 5s")
			}

			ev := waitForDatabaseUserDeactivateEvent(t, testCtx)
			require.Equal(t, "alice", ev.User)
			require.Equal(t, "alice", ev.DatabaseUser)
		})
	}
}

func requirePostgresConnection(t *testing.T, parametersCh chan map[string]string, expectUser, expectDatabase string) {
	t.Helper()
	select {
	case parameters := <-parametersCh:
		require.NotNil(t, parameters)
		require.Equal(t, expectUser, parameters["user"])
		require.Equal(t, expectDatabase, parameters["database"])
	case <-time.After(5 * time.Second):
		t.Fatal("no connection after 5s")
	}
}

func TestAutoUsersMySQL(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		mode                types.CreateDatabaseUserMode
		databaseRoles       []string
		teleportUser        string
		serverVersion       string
		expectConnectionErr bool
		expectDatabaseUser  string
	}{
		"MySQL activate/deactivate users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "8.0.28",
			teleportUser:        "a.very.long.name@teleport.example.com",
			expectConnectionErr: false,
			expectDatabaseUser:  "tp-ZLhdP1FgxXsUvcVpG8ucVm/PCHg",
		},
		"MySQL activate/delete users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "8.0.28",
			teleportUser:        "user1",
			expectConnectionErr: false,
			expectDatabaseUser:  "user1",
		},
		"MySQL auto-user off": {
			mode:          types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
			databaseRoles: []string{"reader", "writer"},
			serverVersion: "8.0.28",
			teleportUser:  "a.very.long.name@teleport.example.com",
			// Given the "alice" user is not present on the database and
			// Teleport won't create it, this should fail with an access denied
			// error.
			expectConnectionErr: true,
			expectDatabaseUser:  "user1",
		},
		"MySQL version not supported": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "5.7.42",
			teleportUser:        "user1",
			expectConnectionErr: true,
			expectDatabaseUser:  "user1",
		},
		"MariaDB activate/deactivate users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "5.5.5-10.11.0-MariaDB",
			teleportUser:        "user1",
			expectConnectionErr: false,
			expectDatabaseUser:  "user1",
		},
		"MariaDB activate/delete users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "5.5.5-10.11.0-MariaDB",
			teleportUser:        "user1",
			expectConnectionErr: false,
			expectDatabaseUser:  "user1",
		},
		"MariaDB long name": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "5.5.5-10.11.0-MariaDB",
			teleportUser:        strings.Repeat("even-longer-name", 5) + "@teleport.example.com",
			expectConnectionErr: false,
			expectDatabaseUser:  "tp-W+34lSjdNvyLfzOejQLRcbe0Rrs",
		},
		"MariaDB version not supported ": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			serverVersion:       "5.5.5-10.0.0-MariaDB",
			teleportUser:        "user1",
			expectConnectionErr: true,
			expectDatabaseUser:  "user1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc := tc
			t.Parallel()

			testCtx := setupTestContext(
				ctx,
				t,
				withSelfHostedMySQL("mysql",
					withMySQLAdminUser("admin"),
					withMySQLServerVersion(tc.serverVersion),
				),
			)
			go testCtx.startHandlingConnections()

			// Create user with role that allows user provisioning.
			_, role, err := auth.CreateUserAndRole(testCtx.tlsServer.Auth(), tc.teleportUser, []string{"auto"}, nil)
			require.NoError(t, err)
			options := role.GetOptions()
			options.CreateDatabaseUserMode = tc.mode
			role.SetOptions(options)
			role.SetDatabaseRoles(types.Allow, []string{"reader", "writer"})
			role.SetDatabaseNames(types.Allow, []string{"*"})
			_, err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// DatabaseUser must match identity.
			_, err = testCtx.mysqlClient(tc.teleportUser, "mysql", "some-other-username")
			require.Error(t, err)

			// Try to connect to the database as this user.
			mysqlConn, err := testCtx.mysqlClient(tc.teleportUser, "mysql", tc.teleportUser)
			if tc.expectConnectionErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			select {
			case e := <-testCtx.mysql["mysql"].db.UserEventsCh():
				require.Equal(t, tc.teleportUser, e.TeleportUser)
				require.Equal(t, tc.expectDatabaseUser, e.DatabaseUser)
				require.Equal(t, []string{"reader", "writer"}, e.Roles)
				require.True(t, e.Active)
			case <-time.After(5 * time.Second):
				t.Fatal("user not activated after 5s")
			}

			// Disconnect.
			err = mysqlConn.Close()
			require.NoError(t, err)

			// Verify user was deactivated.
			select {
			case e := <-testCtx.mysql["mysql"].db.UserEventsCh():
				require.Equal(t, tc.teleportUser, e.TeleportUser)
				require.Equal(t, tc.expectDatabaseUser, e.DatabaseUser)
				require.False(t, e.Active)
			case <-time.After(5 * time.Second):
				t.Fatal("user not deactivated after 5s")
			}

			ev := waitForDatabaseUserDeactivateEvent(t, testCtx)
			require.Equal(t, tc.teleportUser, ev.User)
			require.Equal(t, tc.expectDatabaseUser, ev.DatabaseUser)
		})
	}
}

func TestAutoUsersMongoDB(t *testing.T) {
	t.Setenv("TELEPORT_DISABLE_MONGODB_ADMIN_CLIENT_CACHE", "true")
	ctx := context.Background()
	username := "alice"

	tests := []struct {
		name             string
		mode             types.CreateDatabaseUserMode
		wantTeardownType mongodb.UserEventType
	}{
		{
			name:             "keep",
			mode:             types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			wantTeardownType: mongodb.UserEventDeactivate,
		},
		{
			name:             "best_effort_drop",
			mode:             types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			wantTeardownType: mongodb.UserEventDelete,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			testCtx := setupTestContext(ctx, t, withSelfHostedMongoWithAdminUser("mongo-server", "teleport-admin"))
			go testCtx.startHandlingConnections()

			// Create user with role that allows user provisioning.
			_, role, err := auth.CreateUserAndRole(testCtx.tlsServer.Auth(), username, []string{"auto"}, nil)
			require.NoError(t, err)
			options := role.GetOptions()
			options.CreateDatabaseUserMode = test.mode
			role.SetOptions(options)
			role.SetDatabaseRoles(types.Allow, []string{"readWrite@db1", "readAnyDatabase@admin"})
			role.SetDatabaseNames(types.Allow, []string{"db1", "db2", "admin"})
			_, err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// DatabaseUser must match identity.
			_, err = testCtx.mongoClient(ctx, username, "mongo-server", "some-other-username")
			require.Error(t, err)

			// Try to connect to the database as this user.
			mongoClient, err := testCtx.mongoClient(ctx, username, "mongo-server", username)
			t.Cleanup(func() {
				if mongoClient != nil {
					err := mongoClient.Disconnect(ctx)
					if !strings.Contains(err.Error(), "client is disconnected") {
						require.NoError(t, err)
					}
				}
			})
			require.NoError(t, err)

			// Verify user was activated.
			select {
			case e := <-testCtx.mongo["mongo-server"].db.UserEventsCh():
				require.Equal(t, "CN=alice", e.DatabaseUser)
				require.Equal(t, []string{"readAnyDatabase@admin", "readWrite@db1"}, e.Roles)
				require.Equal(t, mongodb.UserEventActivate, e.Type)
			case <-time.After(5 * time.Second):
				t.Fatal("user not activated after 5s")
			}

			// Disconnect.
			err = mongoClient.Disconnect(ctx)
			require.NoError(t, err)

			// Verify user was teared down.
			select {
			case e := <-testCtx.mongo["mongo-server"].db.UserEventsCh():
				require.Equal(t, "CN=alice", e.DatabaseUser)
				require.Equal(t, test.wantTeardownType, e.Type)
			case <-time.After(5 * time.Second):
				t.Fatal("user not deactivated after 5s")
			}

			ev := waitForDatabaseUserDeactivateEvent(t, testCtx)
			require.Equal(t, username, ev.User)
			require.Equal(t, "alice", ev.DatabaseUser)
		})
	}
}

func waitForDatabaseUserDeactivateEvent(t *testing.T, testCtx *testContext) *apievents.DatabaseUserDeactivate {
	t.Helper()
	const code = libevents.DatabaseSessionUserDeactivateCode
	event := waitForEvent(t, testCtx, code)
	require.Equal(t, code, event.GetCode())

	ev, ok := event.(*apievents.DatabaseUserDeactivate)
	require.True(t, ok)
	return ev
}
