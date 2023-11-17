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

package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

// TestAutoUsersPostgres verifies automatic database user creation for Postgres.
func TestAutoUsersPostgres(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		mode                 types.CreateDatabaseUserMode
		databaseRoles        []string
		adminDefaultDatabase string
		expectConnectionErr  bool
		expectAdminDatabase  string
	}{
		"activate/deactivate users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:       []string{"reader", "writer"},
			expectConnectionErr: false,
			expectAdminDatabase: "user-db",
		},
		"activate/delete users": {
			mode:                types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP,
			databaseRoles:       []string{"reader", "writer"},
			expectConnectionErr: false,
			expectAdminDatabase: "user-db",
		},
		"disabled": {
			mode:          types.CreateDatabaseUserMode_DB_USER_MODE_OFF,
			databaseRoles: []string{"reader", "writer"},
			// Given the "alice" user is not present on the database and
			// Teleport won't create it, this should fail with an access denied
			// error.
			expectConnectionErr: true,
		},
		"admin user default database": {
			mode:                 types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			databaseRoles:        []string{"reader", "writer"},
			adminDefaultDatabase: "admin-db",
			expectConnectionErr:  false,
			expectAdminDatabase:  "admin-db",
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

			// Create user with role that allows user provisioning.
			_, role, err := auth.CreateUserAndRole(testCtx.tlsServer.Auth(), "alice", []string{"auto"}, nil)
			require.NoError(t, err)
			options := role.GetOptions()
			options.CreateDatabaseUserMode = tc.mode
			role.SetOptions(options)
			role.SetDatabaseRoles(types.Allow, tc.databaseRoles)
			role.SetDatabaseNames(types.Allow, []string{"*"})
			_, err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// Try to connect to the database as this user.
			pgConn, err := testCtx.postgresClient(ctx, "alice", "postgres", "alice", "user-db")
			if tc.expectConnectionErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify there are two connections.
			requirePostgresConnection(t, testCtx.postgres["postgres"].db.ParametersCh(), "postgres", tc.expectAdminDatabase)
			requirePostgresConnection(t, testCtx.postgres["postgres"].db.ParametersCh(), "alice", "user-db")

			// Verify user was activated.
			select {
			case e := <-testCtx.postgres["postgres"].db.UserEventsCh():
				require.Equal(t, "alice", e.Name)
				require.Equal(t, []string{"reader", "writer"}, e.Roles)
				require.True(t, e.Active)
			case <-time.After(5 * time.Second):
				t.Fatal("user not activated after 5s")
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
		})
	}
}
