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

func (c *fakeRawAdminClient) waitC() <-chan struct{} {
	return c.waitForDisconnect.Done()
}

func (c *fakeRawAdminClient) isDisconnected() bool {
	return c.waitForDisconnect.Err() != nil
}

func (c *fakeRawAdminClient) Disconnect(_ context.Context) error {
	c.waitForDisconnectCancel()
	return nil
}

func Test_ensure_adminClientFnCache(t *testing.T) {
	require.NotNil(t, adminClientFnCache)
}

func Test_getShareableAdminClient(t *testing.T) {
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

	db1WithAdmin1ShareableClient := mustGetShareableAdminClient(t, cache, fakeClock, db1WithAdmin1)
	db1WithAdmin2ShareableClient := mustGetShareableAdminClient(t, cache, fakeClock, db1WithAdmin2)
	db2WithAdminShareableClient := mustGetShareableAdminClient(t, cache, fakeClock, db2WithAdmin)

	t.Run("new client for different admin user", func(t *testing.T) {
		require.NotSame(t, db1WithAdmin1ShareableClient, db1WithAdmin2ShareableClient)
	})

	t.Run("new client for different database", func(t *testing.T) {
		require.NotSame(t, db1WithAdmin1ShareableClient, db2WithAdminShareableClient)
	})

	t.Run("same client for same database and admin user", func(t *testing.T) {
		client := mustGetShareableAdminClient(t, cache, fakeClock, db1WithAdmin1)
		require.Same(t, db1WithAdmin1ShareableClient, client)
	})

	require.NoError(t, db1WithAdmin1ShareableClient.Disconnect(ctx))
	require.NoError(t, db1WithAdmin2ShareableClient.Disconnect(ctx))
	require.NoError(t, db2WithAdminShareableClient.Disconnect(ctx))

	fakeClock.Advance(adminClientCleanupTTL * 2)

	t.Run("verify client cleanup after TTL", func(t *testing.T) {
		requireFakeRawAdminClientDisconnected(t, db1WithAdmin2ShareableClient)
		requireFakeRawAdminClientDisconnected(t, db2WithAdminShareableClient)
	})

	t.Run("verify client cleanup after Disconnect", func(t *testing.T) {
		fakeRawClient, ok := db1WithAdmin1ShareableClient.adminClient.(*fakeRawAdminClient)
		require.True(t, ok)

		// db1WithAdmin1ShareableClient was acquired twice so it should be waiting.
		require.False(t, fakeRawClient.isDisconnected())

		// Now release it.
		require.NoError(t, db1WithAdmin1ShareableClient.Disconnect(ctx))
		requireFakeRawAdminClientDisconnected(t, db1WithAdmin1ShareableClient)
	})

	t.Run("new client after FnCache TTL", func(t *testing.T) {
		client := mustGetShareableAdminClient(t, cache, fakeClock, db1WithAdmin1)
		t.Cleanup(func() {
			require.NoError(t, client.Disconnect(ctx))
		})
		require.NotSame(t, db1WithAdmin1ShareableClient, client)
	})
}

func mustGetShareableAdminClient(t *testing.T, cache *utils.FnCache, clock clockwork.Clock, db types.Database) *shareableAdminClient {
	t.Helper()

	client, err := getShareableAdminClient(
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
	return client
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

func requireFakeRawAdminClientDisconnected(t *testing.T, client *shareableAdminClient) {
	t.Helper()

	fakeRawClient, ok := client.adminClient.(*fakeRawAdminClient)
	require.True(t, ok)

	select {
	case <-fakeRawClient.waitC():
		return
	case <-time.After(time.Second):
		require.Fail(t, "Timed out waiting for fakeRawAdminClient to Disconnect")
	}
}
