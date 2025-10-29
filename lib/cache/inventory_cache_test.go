// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

// TestInventoryCache tests the initialization and use of the inventory cache.
func TestInventoryCache(t *testing.T) {
	ctx := context.Background()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	defer bk.Close()

	p := newTestPack(t, ForAuth)
	defer p.Close()

	// Create mock instances
	instances := []*types.InstanceV1{
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent1",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "agent1.example.com",
				Version:          "18.1.0",
				Services:         []types.SystemRole{types.RoleNode, types.RoleDatabase, types.RoleApp},
				ExternalUpgrader: "unit",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group1",
				},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent2",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "kube1.example.com",
				Version:          "18.2.0",
				Services:         []types.SystemRole{types.RoleKube},
				ExternalUpgrader: "kube",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group2",
				},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent3",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "node1.example.com",
				Version:          "18.2.0",
				Services:         []types.SystemRole{types.RoleNode, types.RoleDatabase},
				ExternalUpgrader: "unit",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group1",
				},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent4",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "app1.example.com",
				Version:          "18.2.0",
				Services:         []types.SystemRole{types.RoleApp},
				ExternalUpgrader: "unit",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group2",
				},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent5",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "desktop1.example.com",
				Version:          "18.1.0",
				Services:         []types.SystemRole{types.RoleWindowsDesktop},
				ExternalUpgrader: "unit",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group1",
				},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent6",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "proxy1.example.com",
				Version:          "18.1.0",
				Services:         []types.SystemRole{types.RoleProxy},
				ExternalUpgrader: "unit",
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent7",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "auth1.example.com",
				Version:  "18.1.0",
				Services: []types.SystemRole{types.RoleAuth},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent8",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname: "discovery1.example.com",
				Version:  "18.2.0",
				Services: []types.SystemRole{types.RoleDiscovery},
			},
		},
	}

	// Create mock bot instances
	bots := []*machineidv1.BotInstance{
		{
			Metadata: &headerv1.Metadata{
				Name: "bot1",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName: "bot-1",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.1.0",
					},
				},
			},
		},
		{
			Metadata: &headerv1.Metadata{
				Name: "bot2",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName: "bot-2",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.2.0",
					},
				},
			},
		},
		{
			Metadata: &headerv1.Metadata{
				Name: "bot3",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName: "bot-3",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.2.0",
					},
				},
			},
		},
		{
			Metadata: &headerv1.Metadata{
				Name: "bot4",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName: "bot-4",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.1.0",
					},
				},
			},
		},
	}

	mockInventory := &mockInventoryService{instances: instances}
	mockBotCache := &mockBotInstanceCache{bots: bots}

	// Create inventory cache
	inventoryCache, err := NewInventoryCache(InventoryCacheConfig{
		PrimaryCache:     p.cache,
		Backend:          bk,
		Inventory:        mockInventory,
		BotInstanceCache: mockBotCache,
		TargetVersion:    "18.2.0",
	})
	require.NoError(t, err)
	defer inventoryCache.Close()

	// The inventory cache should not be healthy immediately because it needs to wait for `waitForPrimaryCacheInit`.
	require.False(t, inventoryCache.IsHealthy())

	// Wait for the inventory cache to initialize
	require.Eventually(t, func() bool {
		return inventoryCache.IsHealthy()
	}, 5*time.Second, 50*time.Millisecond)

	// Verify all the instances were loaded into the cache
	listResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize: 100,
	})
	require.NoError(t, err)
	require.Len(t, listResp.Items, 12, "should have 8 instances + 4 bots")

	// Verify results are correctly alphabetically sorted
	expectedOrder := []string{
		"agent1.example.com",
		"app1.example.com",
		"auth1.example.com",
		"bot-1",
		"bot-2",
		"bot-3",
		"bot-4",
		"desktop1.example.com",
		"discovery1.example.com",
		"kube1.example.com",
		"node1.example.com",
		"proxy1.example.com",
	}

	for i, item := range listResp.Items {
		var actualName string
		if instance := item.GetInstance(); instance != nil {
			actualName = instance.Spec.Hostname
		} else if bot := item.GetBotInstance(); bot != nil {
			actualName = bot.Spec.BotName
		}
		require.Equal(t, expectedOrder[i], actualName, "item %d should be %s", i, expectedOrder[i])
	}

	// Verify pagination works as intended
	firstPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize: 5,
	})
	require.NoError(t, err)
	require.Len(t, firstPageResp.Items, 5, "first page should have 5 items, got %d", len(firstPageResp.Items))

	// The next page token should be the key of the first item on the next page (6th item)
	expectedNextPageToken := "bot-3/bot3/bot_instance"
	require.Equal(t, expectedNextPageToken, firstPageResp.NextPageToken)

	// Fetch another page of 5, this time using the page token from before
	secondPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize:  5,
		PageToken: firstPageResp.NextPageToken,
	})
	require.NoError(t, err)
	require.Len(t, secondPageResp.Items, 5, "second page should have 5 items, got %d", len(secondPageResp.Items))

	// The returned next page token should be the alphabetical key of the first item on the third page (11th item)
	expectedSecondPageToken := "node1.example.com/agent3/instance"
	require.Equal(t, expectedSecondPageToken, secondPageResp.NextPageToken, "second page next token should match expected format")

	// Fetch another page of 5, using the page token from before
	thirdPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize:  5,
		PageToken: secondPageResp.NextPageToken,
	})
	require.NoError(t, err)
	// We should only get 2 items, and no next page token
	require.Len(t, thirdPageResp.Items, 2, "third page should have 2 items, got %d", len(thirdPageResp.Items))
	require.Empty(t, thirdPageResp.NextPageToken, "third page should not have a next page token")

	// Verify filtering by kind works
	instancesOnlyResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize: 100,
		Filter: &inventoryv1.ListUnifiedInstancesFilter{
			Kinds: []string{types.KindInstance},
		},
	})
	require.NoError(t, err)
	require.Len(t, instancesOnlyResp.Items, 8, "should have 8 results when filtering by KindInstance")

	botsOnlyResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize: 100,
		Filter: &inventoryv1.ListUnifiedInstancesFilter{
			Kinds: []string{types.KindBotInstance},
		},
	})
	require.NoError(t, err)
	require.Len(t, botsOnlyResp.Items, 4, "should have 4 results when filtering by KindBotInstance")
}

// TestInventoryCacheWatcher tests the inventory cache watcher.
func TestInventoryCacheWatcher(t *testing.T) {
	ctx := context.Background()

	bk, err := memory.New(memory.Config{
		Context: ctx,
	})
	require.NoError(t, err)
	defer bk.Close()

	p := newTestPack(t, ForAuth)
	defer p.Close()

	// Create 2 instances
	instances := []*types.InstanceV1{
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent1",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "agent1.example.com",
				Version:          "18.1.0",
				Services:         []types.SystemRole{types.RoleNode, types.RoleDatabase, types.RoleApp},
				ExternalUpgrader: "unit",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group1",
				},
			},
		},
		{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: "agent2",
				},
			},
			Spec: types.InstanceSpecV1{
				Hostname:         "agent2.example.com",
				Version:          "18.2.0",
				Services:         []types.SystemRole{types.RoleNode},
				ExternalUpgrader: "unit",
				UpdaterInfo: &types.UpdaterV2Info{
					UpdateGroup: "group1",
				},
			},
		},
	}

	// Create 2 bot instances
	bots := []*machineidv1.BotInstance{
		{
			Metadata: &headerv1.Metadata{
				Name: "bot1",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName: "bot-1",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.1.0",
					},
				},
			},
		},
		{
			Metadata: &headerv1.Metadata{
				Name: "bot2",
			},
			Spec: &machineidv1.BotInstanceSpec{
				BotName: "bot-2",
			},
			Status: &machineidv1.BotInstanceStatus{
				LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
					{
						RecordedAt: timestamppb.Now(),
						Version:    "18.2.0",
					},
				},
			},
		},
	}

	mockInventoryService := &mockInventoryService{
		instances: instances,
	}

	mockBotInstanceCache := &mockBotInstanceCache{
		bots: bots,
	}

	inventoryCache, err := NewInventoryCache(InventoryCacheConfig{
		Inventory:        mockInventoryService,
		BotInstanceCache: mockBotInstanceCache,
		Backend:          bk,
		PrimaryCache:     p.cache,
		TargetVersion:    "18.2.0",
	})
	require.NoError(t, err)
	defer inventoryCache.Close()

	// Wait for the inventory cache to initialize
	require.Eventually(t, func() bool {
		return inventoryCache.IsHealthy()
	}, 5*time.Second, 50*time.Millisecond)

	listResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
		PageSize: 100,
	})
	require.NoError(t, err)
	require.Len(t, listResp.Items, 4, "should start with 4 items (2 instances + 2 bots)")

	// Add a new instance and a bot instance to the backend
	newInstance := &types.InstanceV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: "agent3",
			},
		},
		Spec: types.InstanceSpecV1{
			Hostname:         "newagent.example.com",
			Version:          "18.2.0",
			Services:         []types.SystemRole{types.RoleNode},
			ExternalUpgrader: "unit",
			UpdaterInfo: &types.UpdaterV2Info{
				UpdateGroup: "group1",
			},
		},
	}

	newBot := &machineidv1.BotInstance{
		Metadata: &headerv1.Metadata{
			Name: "bot3",
		},
		Spec: &machineidv1.BotInstanceSpec{
			BotName: "new-bot",
		},
		Status: &machineidv1.BotInstanceStatus{
			LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
				{
					RecordedAt: timestamppb.Now(),
					Version:    "18.2.0",
				},
			},
		},
	}

	newInstanceBytes, err := newInstance.Marshal()
	require.NoError(t, err)
	_, err = bk.Put(ctx, backend.Item{
		Key:   backend.NewKey(instancePrefix, newInstance.GetName()),
		Value: newInstanceBytes,
	})
	require.NoError(t, err)

	newBotBytes, err := proto.Marshal(newBot)
	require.NoError(t, err)
	_, err = bk.Put(ctx, backend.Item{
		Key:   backend.NewKey(botInstancePrefix, newBot.Metadata.Name),
		Value: newBotBytes,
	})
	require.NoError(t, err)

	// Wait for the inventory cache watcher to process the put events
	require.Eventually(t, func() bool {
		listResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
		})
		if err != nil {
			return false
		}
		// Should now have 6 items
		return len(listResp.Items) == 6
	}, 5*time.Second, 50*time.Millisecond)

	// Delete the newly added instances
	err = bk.Delete(ctx, backend.NewKey(instancePrefix, newInstance.GetName()))
	require.NoError(t, err)

	err = bk.Delete(ctx, backend.NewKey(botInstancePrefix, newBot.Metadata.Name))
	require.NoError(t, err)

	// Wait for the watcher to process the delete events
	require.Eventually(t, func() bool {
		listResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
		})
		if err != nil {
			t.Logf("Error listing: %v", err)
			return false
		}
		// Should now have 4 items
		return len(listResp.Items) == 4
	}, 5*time.Second, 50*time.Millisecond)
}

// mockInventoryService is a mock implementation of services.Inventory.
type mockInventoryService struct {
	instances []*types.InstanceV1
}

func (m *mockInventoryService) GetInstances(ctx context.Context, filter types.InstanceFilter) stream.Stream[types.Instance] {
	items := make([]types.Instance, len(m.instances))
	for i, inst := range m.instances {
		items[i] = inst
	}
	return stream.Slice(items)
}

// mockBotInstanceCache is a mock implementation of services.BotInstance.
type mockBotInstanceCache struct {
	bots []*machineidv1.BotInstance
}

func (m *mockBotInstanceCache) ListBotInstances(ctx context.Context, pageSize int, pageToken string, opts *services.ListBotInstancesRequestOptions) ([]*machineidv1.BotInstance, string, error) {
	return m.bots, "", nil
}

func (m *mockBotInstanceCache) GetBotInstance(ctx context.Context, botName, instanceID string) (*machineidv1.BotInstance, error) {
	return nil, nil
}

func (m *mockBotInstanceCache) DeleteBotInstance(ctx context.Context, botName, instanceID string) error {
	return nil
}

func (m *mockBotInstanceCache) PatchBotInstance(ctx context.Context, botName, instanceID string, update func(*machineidv1.BotInstance) (*machineidv1.BotInstance, error)) (*machineidv1.BotInstance, error) {
	return nil, nil
}

func (m *mockBotInstanceCache) DeleteAllBotInstances(ctx context.Context) error {
	return nil
}

func (m *mockBotInstanceCache) CreateBotInstance(ctx context.Context, instance *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
	return instance, nil
}

func (m *mockBotInstanceCache) GetBotInstancesCount(ctx context.Context, botName string) (int, error) {
	return len(m.bots), nil
}

func (m *mockBotInstanceCache) SubmitHeartbeat(ctx context.Context, heartbeat *machineidv1.SubmitHeartbeatRequest) (*machineidv1.SubmitHeartbeatResponse, error) {
	return nil, nil
}
