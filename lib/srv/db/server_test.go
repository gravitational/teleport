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

	apidefaults "github.com/gravitational/teleport/api/v7/defaults"
	"github.com/gravitational/teleport/api/v7/types"

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
		server types.DatabaseServer
	}{
		{
			server: testCtx.postgres["postgres"].server,
		},
		{
			server: testCtx.mysql["mysql"].server,
		},
		{
			server: testCtx.mongo["mongo"].server,
		},
	}

	for _, test := range tests {
		labels, ok := testCtx.server.dynamicLabels[test.server.GetName()]
		require.True(t, ok)
		require.Equal(t, "test", labels.Get()["echo"].GetResult())

		heartbeat, ok := testCtx.server.heartbeats[test.server.GetName()]
		require.True(t, ok)

		err = heartbeat.ForceSend(time.Second)
		require.NoError(t, err)
	}

	// Make sure servers were announced and their labels updated.
	servers, err := testCtx.authClient.GetDatabaseServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	for _, server := range servers {
		require.Equal(t, map[string]string{"echo": "test"}, server.GetAllLabels())
	}
}
