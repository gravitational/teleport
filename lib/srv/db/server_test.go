/*
Copyright 2020 Gravitational, Inc.

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

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/limiter"

	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseServerStart validates that started database server updates its
// dynamic labels and heartbeats its presence to the auth server.
func TestDatabaseServerStart(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
		withSelfHostedMySQL("mysql"),
		withSelfHostedMongo("mongo"))

	tests := []struct {
		database types.Database
	}{
		{database: testCtx.postgres["postgres"].resource},
		{database: testCtx.mysql["mysql"].resource},
		{database: testCtx.mongo["mongo"].resource},
	}

	for _, test := range tests {
		labels := testCtx.server.getDynamicLabels(test.database.GetName())
		require.NotNil(t, labels)
		require.Equal(t, "test", labels.Get()["echo"].GetResult())
	}

	// Make sure servers were announced and their labels updated.
	servers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 3)
	for _, server := range servers {
		require.Equal(t, map[string]string{"echo": "test"},
			server.GetDatabase().GetAllLabels())
	}
}

func TestDatabaseServerLimiting(t *testing.T) {
	const (
		user      = "bob"
		role      = "admin"
		dbName    = "postgres"
		dbUser    = user
		connLimit = int64(5) // Arbitrary number
	)

	ctx := context.Background()
	allowDbUsers := []string{types.Wildcard}
	allowDbNames := []string{types.Wildcard}

	testCtx := setupTestContext(ctx, t,
		withSelfHostedPostgres("postgres"),
	)

	connLimiter, err := limiter.NewLimiter(limiter.Config{MaxConnections: connLimit})
	require.NoError(t, err)

	// Set connection limit
	testCtx.server.limiter = connLimiter.ConnectionsLimiter

	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, user, role, allowDbUsers, allowDbNames)

	dbConns := make([]*pgconn.PgConn, 0)

	// Connect the maximum allowed number of clients.
	for i := int64(0); i < connLimit; i++ {
		pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
		require.NoError(t, err)

		// Save all connection, so we can close them later.
		dbConns = append(dbConns, pgConn)
	}

	// We keep the previous connections open, so this one should be rejected, because we exhausted the limit.
	_, err = testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect")

	// Disconnect all clients.
	for _, pgConn := range dbConns {
		err = pgConn.Close(ctx)
		require.NoError(t, err)
	}
}
