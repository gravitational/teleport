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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

// TestAutoUsersPostgres verifies automatic database user creation for Postgres.
func TestAutoUsersPostgres(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres", func(db *types.DatabaseV3) {
		db.Spec.AdminUser = &types.DatabaseAdminUser{
			Name: "postgres",
		}
	}))
	go testCtx.startHandlingConnections()

	// Create user with role that allows user provisioning.
	_, role, err := auth.CreateUserAndRole(testCtx.tlsServer.Auth(), "alice", []string{"auto"}, nil)
	require.NoError(t, err)
	options := role.GetOptions()
	options.CreateDatabaseUser = types.NewBoolOption(true)
	role.SetOptions(options)
	role.SetDatabaseRoles(types.Allow, []string{"reader", "writer"})
	role.SetDatabaseNames(types.Allow, []string{"*"})
	err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	// Try to connect to the database as this user.
	pgConn, err := testCtx.postgresClient(ctx, "alice", "postgres", "alice", "postgres")
	require.NoError(t, err)

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
}
