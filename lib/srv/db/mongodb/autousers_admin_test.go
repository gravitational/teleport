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

package mongodb

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

type fakeRawAdminClient struct {
	adminClient

	waitForDisconnect       context.Context
	waitForDisconnectCancel context.CancelFunc
}

func newFakeRawAdminClient() *fakeRawAdminClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &fakeRawAdminClient{
		waitForDisconnect:       ctx,
		waitForDisconnectCancel: cancel,
	}
}

func (c *fakeRawAdminClient) Disconnect(_ context.Context) error {
	c.waitForDisconnectCancel()
	return nil
}

func Test_ensure_adminClientFnCache(t *testing.T) {
	require.NotNil(t, adminClientFnCache)
}

func Test_getSharedAdminClient(t *testing.T) {
	ctx := context.Background()
	fakeClock := clockwork.NewFakeClock()
	require.NotNil(t, fakeClock)

	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   adminClientFnCacheTTL,
		Clock: fakeClock,
	})
	require.NoError(t, err)

	db1WithAdmin1 := mustMakeMongoDatabase(t, "db1", "admin1")
	db1WithAdmin2 := mustMakeMongoDatabase(t, "db1", "admin2")
	db2WithAdmin := mustMakeMongoDatabase(t, "db2", "admin")

	db1WithAdmin1SharedClient := mustGetSharedAdminClient(t, cache, fakeClock, db1WithAdmin1)
	db1WithAdmin2SharedClient := mustGetSharedAdminClient(t, cache, fakeClock, db1WithAdmin2)
	db2WithAdminSharedClient := mustGetSharedAdminClient(t, cache, fakeClock, db2WithAdmin)

	t.Run("new client for different admin user", func(t *testing.T) {
		require.NotSame(t, db1WithAdmin1SharedClient, db1WithAdmin2SharedClient)
	})

	t.Run("new client for different database", func(t *testing.T) {
		require.NotSame(t, db1WithAdmin1SharedClient, db2WithAdminSharedClient)
	})

	t.Run("same client for same database and admin user", func(t *testing.T) {
		sharedClient := mustGetSharedAdminClient(t, cache, fakeClock, db1WithAdmin1)
		require.Same(t, db1WithAdmin1SharedClient, sharedClient)
	})

	db1WithAdmin1SharedClient.Disconnect(ctx)
	db1WithAdmin2SharedClient.Disconnect(ctx)
	db2WithAdminSharedClient.Disconnect(ctx)

	fakeClock.Advance(adminClientCleanupTTL * 2)

	t.Run("verify client cleanup after TTL", func(t *testing.T) {
		requireFakeRawAdminClientDisconnected(t, db1WithAdmin2SharedClient)
		requireFakeRawAdminClientDisconnected(t, db2WithAdminSharedClient)
	})

	t.Run("verify client cleanup after Disconnect", func(t *testing.T) {
		fakeRawClient, ok := db1WithAdmin1SharedClient.adminClient.(*fakeRawAdminClient)
		require.True(t, ok)

		// db1WithAdmin1SharedClient was acquired twice so it should be waiting.
		require.Nil(t, fakeRawClient.waitForDisconnect.Err())

		// Now release it.
		db1WithAdmin1SharedClient.Disconnect(ctx)
		requireFakeRawAdminClientDisconnected(t, db1WithAdmin1SharedClient)
	})

	t.Run("new client after FnCache TTL", func(t *testing.T) {
		sharedClient := mustGetSharedAdminClient(t, cache, fakeClock, db1WithAdmin1)
		t.Cleanup(func() {
			sharedClient.Disconnect(ctx)
		})
		require.NotSame(t, db1WithAdmin1SharedClient, sharedClient)
	})
}

func mustGetSharedAdminClient(t *testing.T, cache *utils.FnCache, clock clockwork.Clock, db types.Database) *sharedAdminClient {
	t.Helper()

	sharedClient, err := getSharedAdminClient(
		context.Background(),
		cache,
		&common.Session{
			Database: db,
		},
		&Engine{
			EngineConfig: common.EngineConfig{
				Clock: clock,
			},
		},
		func(context.Context, *common.Session, *Engine) (adminClient, error) {
			return newFakeRawAdminClient(), nil
		},
	)
	require.NoError(t, err)
	return sharedClient
}

func mustMakeMongoDatabase(t *testing.T, name, adminUser string) types.Database {
	t.Helper()
	db, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMongoDB,
		URI:      "localhost:8001",
		AdminUser: &types.DatabaseAdminUser{
			Name: adminUser,
		},
	})
	require.NoError(t, err)
	return db
}

func requireFakeRawAdminClientDisconnected(t *testing.T, sharedClient *sharedAdminClient) {
	t.Helper()

	fakeRawClient, ok := sharedClient.adminClient.(*fakeRawAdminClient)
	require.True(t, ok)

	select {
	case <-fakeRawClient.waitForDisconnect.Done():
		return
	case <-time.After(time.Second):
		require.Fail(t, "Timed out waiting for fakeRawAdminClient to Disconnect")
	}
}
