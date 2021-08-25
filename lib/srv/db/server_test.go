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
	"time"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"

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

	err := testCtx.server.Start()
	require.NoError(t, err)

	tests := []struct {
		database types.Database
	}{
		{
			database: testCtx.postgres["postgres"].resource,
		},
		{
			database: testCtx.mysql["mysql"].resource,
		},
		{
			database: testCtx.mongo["mongo"].resource,
		},
	}

	for _, test := range tests {
		labels, ok := testCtx.server.dynamicLabels[test.database.GetName()]
		require.True(t, ok)
		require.Equal(t, "test", labels.Get()["echo"].GetResult())
	}

	heartbeat, ok := testCtx.server.heartbeats[testCtx.server.cfg.Server.GetName()]
	require.True(t, ok)

	err = heartbeat.ForceSend(time.Second)
	require.NoError(t, err)

	// Make sure servers were announced and their labels updated.
	servers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Len(t, servers[0].GetDatabases(), 3)
	for _, database := range servers[0].GetDatabases() {
		require.Equal(t, map[string]string{"echo": "test"}, database.GetAllLabels())
	}
}
