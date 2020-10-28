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

	"github.com/gravitational/teleport/lib/defaults"

	"github.com/stretchr/testify/require"
)

// TestDatabaseServerStart validates that started database server updates its
// dynamic labels and heartbeats its presence to the auth server.
func TestDatabaseServerStart(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	t.Cleanup(func() { testCtx.Close() })

	err := testCtx.server.Start()
	require.NoError(t, err)
	t.Cleanup(func() { testCtx.server.Close() })

	labels, ok := testCtx.server.dynamicLabels[testCtx.dbServer.GetName()]
	require.True(t, ok)
	require.Equal(t, "test", labels.Get()["echo"].GetResult())

	heartbeat, ok := testCtx.server.heartbeats[testCtx.dbServer.GetName()]
	require.True(t, ok)

	err = heartbeat.ForceSend(time.Second)
	require.NoError(t, err)

	servers, err := testCtx.authClient.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, testCtx.server.getServers(), servers)

	// Update the server, force the heartbeat and make sure it was re-announced.
	testCtx.dbServer.SetStaticLabels(map[string]string{"a": "b"})
	err = heartbeat.ForceSend(time.Second)
	require.NoError(t, err)

	servers, err = testCtx.authClient.GetDatabaseServers(ctx, defaults.Namespace)
	require.NoError(t, err)
	require.EqualValues(t, testCtx.server.getServers(), servers)
}
