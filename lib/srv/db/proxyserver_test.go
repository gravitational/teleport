/*
Copyright 2021 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/jackc/pgconn"
	"github.com/stretchr/testify/require"
)

func TestProxyConnectionLimiting(t *testing.T) {
	const (
		user            = "bob"
		role            = "admin"
		dbName          = "postgres"
		dbUser          = user
		connLimitNumber = 3 // Arbitrary number
	)

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t, withSelfHostedPostgres("postgres"))

	connLimit, err := limiter.NewConnectionsLimiter(limiter.Config{MaxConnections: connLimitNumber})
	require.NoError(t, err)

	// Set proxy connection limiter
	testCtx.proxyServer.cfg.Limiter = connLimit

	go testCtx.startHandlingConnections()

	// Create user/role with the requested permissions.
	testCtx.createUserAndRole(ctx, t, user, role, []string{types.Wildcard}, []string{types.Wildcard})

	conns := make([]*pgconn.PgConn, 0)
	defer func() {
		for _, conn := range conns {
			err := conn.Close(ctx)
			require.NoError(t, err)
		}
	}()

	for i := 0; i < connLimitNumber; i++ {
		// Try to connect to the database as this user.
		pgConn, err := testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
		require.NoError(t, err)

		conns = append(conns, pgConn)
	}

	_, err = testCtx.postgresClient(ctx, user, "postgres", dbUser, dbName)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeded connection limit")
}
