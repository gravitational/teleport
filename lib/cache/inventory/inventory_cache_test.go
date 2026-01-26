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

package inventory

import (
	"cmp"
	"context"
	"encoding/base32"
	"fmt"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"rsc.io/ordered"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	inventoryv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/inventory/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/local/generic"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// testCache holds the resources needed for creating a test cache
type testCache struct {
	Backend *backend.Wrapper
	Cache   *cache.Cache
	EventsC chan cache.Event
}

func (tc *testCache) Close() error {
	var errors []error
	if tc.Cache != nil {
		errors = append(errors, tc.Cache.Close())
	}
	if tc.Backend != nil {
		errors = append(errors, tc.Backend.Close())
	}
	return trace.NewAggregate(errors...)
}

func getItemName(item *inventoryv1.UnifiedInstanceItem) string {
	if item == nil {
		return ""
	}
	if instance := item.GetInstance(); instance != nil {
		return instance.Spec.Hostname
	} else if bot := item.GetBotInstance(); bot != nil {
		return bot.Spec.BotName
	}
	return ""
}

func getItemVersion(item *inventoryv1.UnifiedInstanceItem) string {
	if item == nil {
		return ""
	}
	if instance := item.GetInstance(); instance != nil {
		return instance.Spec.Version
	} else if bot := item.GetBotInstance(); bot != nil {
		if len(bot.Status.LatestHeartbeats) > 0 {
			return bot.Status.LatestHeartbeats[0].Version
		}
	}
	return ""
}

// setupTestCache creates a new test cache
func setupTestCache(t *testing.T, setupConfig cache.SetupConfigFn) (*testCache, error) {
	t.Helper()
	ctx := t.Context()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Mirror:  true,
	})
	require.NoError(t, err)
	bkWrapper := backend.NewWrapper(bk)

	eventsC := make(chan cache.Event, 1024)

	clusterConfig, err := local.NewClusterConfigurationService(bkWrapper)
	require.NoError(t, err)

	idService, err := local.NewTestIdentityService(bkWrapper)
	require.NoError(t, err)

	dynamicWindowsDesktopService, err := local.NewDynamicWindowsDesktopService(bkWrapper)
	require.NoError(t, err)

	trustS := local.NewCAService(bkWrapper)
	provisionerS := local.NewProvisioningService(bkWrapper)
	eventsS := local.NewEventsService(bkWrapper)
	presenceS := local.NewPresenceService(bkWrapper)
	accessS := local.NewAccessService(bkWrapper)
	dynamicAccessS := local.NewDynamicAccessService(bkWrapper)
	restrictions := local.NewRestrictionsService(bkWrapper)
	apps := local.NewAppService(bkWrapper)
	kubernetes := local.NewKubernetesService(bkWrapper)
	databases := local.NewDatabasesService(bkWrapper)
	databaseServices := local.NewDatabaseServicesService(bkWrapper)
	windowsDesktops := local.NewWindowsDesktopService(bkWrapper)

	samlIDPServiceProviders, err := local.NewSAMLIdPServiceProviderService(bkWrapper)
	require.NoError(t, err)

	userGroups, err := local.NewUserGroupService(bkWrapper)
	require.NoError(t, err)

	oktaSvc, err := local.NewOktaService(bkWrapper, bkWrapper.Clock())
	require.NoError(t, err)

	igSvc, err := local.NewIntegrationsService(bkWrapper, local.WithIntegrationsServiceCacheMode(true))
	require.NoError(t, err)

	userTasksSvc, err := local.NewUserTasksService(bkWrapper)
	require.NoError(t, err)

	dcSvc, err := local.NewDiscoveryConfigService(bkWrapper)
	require.NoError(t, err)

	ulsSvc, err := local.NewUserLoginStateService(bkWrapper)
	require.NoError(t, err)

	secReportsSvc, err := local.NewSecReportsService(bkWrapper, bkWrapper.Clock())
	require.NoError(t, err)

	accessListsSvc, err := local.NewAccessListServiceV2(local.AccessListServiceConfig{
		Backend: bkWrapper,
		Modules: modulestest.OSSModules(),
	})
	require.NoError(t, err)

	accessMonitoringRuleService, err := local.NewAccessMonitoringRulesService(bkWrapper)
	require.NoError(t, err)

	crownJewelsSvc, err := local.NewCrownJewelsService(bkWrapper)
	require.NoError(t, err)

	spiffeFederationsSvc, err := local.NewSPIFFEFederationService(bkWrapper)
	require.NoError(t, err)

	workloadIdentitySvc, err := local.NewWorkloadIdentityService(bkWrapper)
	require.NoError(t, err)

	databaseObjectsSvc, err := local.NewDatabaseObjectService(bkWrapper)
	require.NoError(t, err)

	kubeWaitingContSvc, err := local.NewKubeWaitingContainerService(bkWrapper)
	require.NoError(t, err)

	notificationsSvc, err := local.NewNotificationsService(bkWrapper, bkWrapper.Clock())
	require.NoError(t, err)

	staticHostUserService, err := local.NewStaticHostUserService(bkWrapper)
	require.NoError(t, err)

	autoUpdateService, err := local.NewAutoUpdateService(bkWrapper)
	require.NoError(t, err)

	provisioningStates, err := local.NewProvisioningStateService(bkWrapper)
	require.NoError(t, err)

	identityCenter, err := local.NewIdentityCenterService(local.IdentityCenterServiceConfig{
		Backend: bkWrapper,
	})
	require.NoError(t, err)

	pluginStaticCredentials, err := local.NewPluginStaticCredentialsService(bkWrapper)
	require.NoError(t, err)

	gitServers, err := local.NewGitServerService(bkWrapper)
	require.NoError(t, err)

	healthCheckConfig, err := local.NewHealthCheckConfigService(bkWrapper)
	require.NoError(t, err)

	botInstanceService, err := local.NewBotInstanceService(bkWrapper, bkWrapper.Clock())
	require.NoError(t, err)

	recordingEncryption, err := local.NewRecordingEncryptionService(bkWrapper)
	require.NoError(t, err)

	plugin := local.NewPluginsService(bkWrapper)

	appAuthConfig, err := local.NewAppAuthConfigService(bkWrapper)
	require.NoError(t, err)

	c, err := cache.New(setupConfig(cache.Config{
		Context:                 ctx,
		Events:                  eventsS,
		ClusterConfig:           clusterConfig,
		Provisioner:             provisionerS,
		Trust:                   trustS,
		Users:                   idService,
		Access:                  accessS,
		DynamicAccess:           dynamicAccessS,
		Presence:                presenceS,
		AppSession:              idService,
		WebSession:              idService.WebSessions(),
		WebToken:                idService,
		SnowflakeSession:        idService,
		Restrictions:            restrictions,
		Apps:                    apps,
		Kubernetes:              kubernetes,
		DatabaseServices:        databaseServices,
		Databases:               databases,
		WindowsDesktops:         windowsDesktops,
		DynamicWindowsDesktops:  dynamicWindowsDesktopService,
		SAMLIdPServiceProviders: samlIDPServiceProviders,
		UserGroups:              userGroups,
		Okta:                    oktaSvc,
		Integrations:            igSvc,
		UserTasks:               userTasksSvc,
		DiscoveryConfigs:        dcSvc,
		UserLoginStates:         ulsSvc,
		SecReports:              secReportsSvc,
		AccessLists:             accessListsSvc,
		KubeWaitingContainers:   kubeWaitingContSvc,
		Notifications:           notificationsSvc,
		AccessMonitoringRules:   accessMonitoringRuleService,
		CrownJewels:             crownJewelsSvc,
		SPIFFEFederations:       spiffeFederationsSvc,
		DatabaseObjects:         databaseObjectsSvc,
		StaticHostUsers:         staticHostUserService,
		AutoUpdateService:       autoUpdateService,
		ProvisioningStates:      provisioningStates,
		IdentityCenter:          identityCenter,
		PluginStaticCredentials: pluginStaticCredentials,
		GitServers:              gitServers,
		HealthCheckConfig:       healthCheckConfig,
		WorkloadIdentity:        workloadIdentitySvc,
		BotInstanceService:      botInstanceService,
		RecordingEncryption:     recordingEncryption,
		Plugin:                  plugin,
		AppAuthConfig:           appAuthConfig,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 eventsC,
	}))
	require.NoError(t, err)

	select {
	case event := <-eventsC:
		require.Equal(t, cache.WatcherStarted, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for watcher to start")
	}

	return &testCache{
		Backend: bkWrapper,
		Cache:   c,
		EventsC: eventsC,
	}, nil
}

// TestInventoryCache tests the initialization and use of the inventory cache.
func TestInventoryCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		p, err := setupTestCache(t, cache.ForAuth)
		require.NoError(t, err)
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
					BotName:    "bot-1",
					InstanceId: "bot1",
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
					BotName:    "bot-2",
					InstanceId: "bot2",
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
					BotName:    "bot-3",
					InstanceId: "bot3",
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
					BotName:    "bot-4",
					InstanceId: "bot4",
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
			PrimaryCache:       p.Cache,
			Events:             local.NewEventsService(p.Backend),
			Inventory:          mockInventory,
			BotInstanceBackend: mockBotCache,
			TargetVersion:      "18.2.0",
		})
		require.NoError(t, err)
		defer inventoryCache.Close()

		// Wait for the inventory cache to initialize by blocking until all the initialization goroutines are durably blocked, which means
		// the cache has been initialized.
		synctest.Wait()
		require.True(t, inventoryCache.IsHealthy())

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

		require.Equal(t, expectedOrder, libslices.Map(listResp.Items, getItemName))

		// Verify pagination works as intended
		firstPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 5,
		})
		require.NoError(t, err)
		require.Len(t, firstPageResp.Items, 5, "first page should have 5 items, got %d", len(firstPageResp.Items))

		// The next page token should be the alphabetical key of the first item on the next page (6th item)
		expectedNextPageToken := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(string(ordered.Encode("bot-3", "bot3", types.KindBotInstance))))
		require.Equal(t, expectedNextPageToken, firstPageResp.NextPageToken)

		// Fetch another page of 5, this time using the page token from before
		secondPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize:  5,
			PageToken: firstPageResp.NextPageToken,
		})
		require.NoError(t, err)
		require.Len(t, secondPageResp.Items, 5, "second page should have 5 items, got %d", len(secondPageResp.Items))

		// The returned next page token should be the alphabetical key of the first item on the third page (11th item)
		expectedSecondPageToken := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(string(ordered.Encode("node1.example.com", "agent3", types.KindInstance))))
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
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, instancesOnlyResp.Items, 8, "should have 8 results when filtering by KindInstance")

		botsOnlyResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, botsOnlyResp.Items, 4, "should have 4 results when filtering by KindBotInstance")
	})
}

// TestInventoryCacheWatcher tests the inventory cache watcher.
func TestInventoryCacheWatcher(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		p, err := setupTestCache(t, cache.ForAuth)
		require.NoError(t, err)
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
					BotName:    "bot-1",
					InstanceId: "bot1",
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
					BotName:    "bot-2",
					InstanceId: "bot2",
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
			Inventory:          mockInventoryService,
			BotInstanceBackend: mockBotInstanceCache,
			Events:             local.NewEventsService(p.Backend),
			PrimaryCache:       p.Cache,
			TargetVersion:      "18.2.0",
		})
		require.NoError(t, err)
		defer inventoryCache.Close()

		// Wait for the inventory cache to initialize by blocking until all the initialization goroutines are durably blocked, which means
		// the cache has been initialized.
		synctest.Wait()
		require.True(t, inventoryCache.IsHealthy())

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
				BotName:    "new-bot",
				InstanceId: "bot3",
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

		newInstanceItem, err := generic.FastMarshal(backend.NewKey(instancePrefix, newInstance.GetName()), newInstance)
		require.NoError(t, err)
		_, err = p.Backend.Put(ctx, newInstanceItem)
		require.NoError(t, err)

		newBotBytes, err := services.MarshalBotInstance(newBot)
		require.NoError(t, err)
		_, err = p.Backend.Put(ctx, backend.Item{
			Key:   backend.NewKey(botInstancePrefix, newBot.Spec.BotName, newBot.Metadata.Name),
			Value: newBotBytes,
		})
		require.NoError(t, err)

		// Wait for the inventory cache watcher to process the put events
		synctest.Wait()
		listResp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
		})
		require.NoError(t, err)
		require.Len(t, listResp.Items, 6, "should have 6 items after adding 2")

		// Delete the newly added instances
		err = p.Backend.Delete(ctx, backend.NewKey(instancePrefix, newInstance.GetName()))
		require.NoError(t, err)

		err = p.Backend.Delete(ctx, backend.NewKey(botInstancePrefix, newBot.Spec.BotName, newBot.Metadata.Name))
		require.NoError(t, err)

		// Wait for the watcher to process the delete events
		synctest.Wait()
		listResp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
		})
		require.NoError(t, err)
		require.Len(t, listResp.Items, 4, "should have 4 items after deleting 2")
	})
}

// TestInventoryCacheRateLimiting tests the rate limiting behavior of the inventory cache.
func TestInventoryCacheRateLimiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, err := setupTestCache(t, cache.ForAuth)
		require.NoError(t, err)
		defer p.Close()

		// Create 300 instances
		const numInstances = 300
		instances := make([]*types.InstanceV1, numInstances)
		for i := range numInstances {
			instances[i] = &types.InstanceV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: fmt.Sprintf("agent%d", i),
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "agent.example.com",
					Version:  "18.2.0",
					Services: []types.SystemRole{types.RoleNode},
				},
			}
		}

		mockInventory := &mockInventoryService{instances: instances}
		mockBotCache := &mockBotInstanceCache{bots: nil}

		inventoryCache, err := NewInventoryCache(InventoryCacheConfig{
			PrimaryCache:       p.Cache,
			Events:             local.NewEventsService(p.Backend),
			Inventory:          mockInventory,
			BotInstanceBackend: mockBotCache,
			TargetVersion:      "18.2.0",
		})
		require.NoError(t, err)
		defer inventoryCache.Close()

		synctest.Wait()
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		// After only 100ms, not all instances should be loaded and the cache shouldn't be healthy yet.
		initialCount := inventoryCache.cache.Len()
		require.Greater(t, initialCount, 0, "expected some instances to be loaded")
		require.Less(t, initialCount, numInstances,
			"not all instances should be loaded immediately",
			numInstances, initialCount)
		require.False(t, inventoryCache.IsHealthy())

		// After another 50s, more instances should be loaded, but not all.
		time.Sleep(50 * time.Millisecond)
		synctest.Wait()

		require.Greater(t, inventoryCache.cache.Len(), initialCount,
			"expected more instances to be loaded now than before, but not all", initialCount, inventoryCache.cache.Len())
		require.Less(t, initialCount, numInstances,
			"not all instances should be loaded yet",
			numInstances, initialCount)
		require.False(t, inventoryCache.IsHealthy())

		time.Sleep(2 * time.Second)
		synctest.Wait()

		// After 2 seconds, all instances should be loaded and the cache should be healthy.
		require.True(t, inventoryCache.IsHealthy())
		finalCount := inventoryCache.cache.Len()
		require.Equal(t, numInstances, finalCount,
			"expected all %d instances to be loaded, got %d", numInstances, finalCount)
	})
}

// TestInventoryCacheFiltering tests the filtering for the inventory cache.
func TestInventoryCacheFiltering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		bk, err := memory.New(memory.Config{
			Context: ctx,
		})
		require.NoError(t, err)
		defer bk.Close()

		p, err := setupTestCache(t, cache.ForAuth)
		require.NoError(t, err)
		defer p.Close()

		instances := []*types.InstanceV1{
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "node1",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname:         "node1.example.com",
					Version:          "18.1.0",
					Services:         []types.SystemRole{types.RoleNode},
					UpdaterInfo:      &types.UpdaterV2Info{UpdateGroup: "group1"},
					ExternalUpgrader: "kube",
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "node2",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname:         "node2.example.com",
					Version:          "18.2.0",
					Services:         []types.SystemRole{types.RoleNode, types.RoleProxy},
					UpdaterInfo:      &types.UpdaterV2Info{UpdateGroup: "group1"},
					ExternalUpgrader: "unit",
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "auth1",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname:         "auth1.example.com",
					Version:          "19.0.0",
					Services:         []types.SystemRole{types.RoleAuth},
					UpdaterInfo:      &types.UpdaterV2Info{UpdateGroup: "group2"},
					ExternalUpgrader: "kube",
				},
			},
		}

		bots := []*machineidv1.BotInstance{
			{
				Kind:    types.KindBotInstance,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "bot1-instance1",
				},
				Spec: &machineidv1.BotInstanceSpec{
					BotName:    "bot1",
					InstanceId: "instance1",
				},
				Status: &machineidv1.BotInstanceStatus{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						{
							RecordedAt:      timestamppb.Now(),
							Version:         "18.1.5",
							Hostname:        "bot-host1.example.com",
							ExternalUpdater: "kube",
							UpdaterInfo:     &types.UpdaterV2Info{UpdateGroup: "bot-group1"},
						},
					},
				},
			},
			{
				Kind:    types.KindBotInstance,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: "bot2-instance2",
				},
				Spec: &machineidv1.BotInstanceSpec{
					BotName:    "bot2",
					InstanceId: "instance2",
				},
				Status: &machineidv1.BotInstanceStatus{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						{
							RecordedAt:      timestamppb.Now(),
							Version:         "19.0.1",
							Hostname:        "bot-host2.example.com",
							ExternalUpdater: "unit",
							UpdaterInfo:     &types.UpdaterV2Info{UpdateGroup: "bot-group2"},
						},
					},
				},
			},
		}

		mockInventory := &mockInventoryService{instances: instances}
		mockBotCache := &mockBotInstanceCache{bots: bots}

		inventoryCache, err := NewInventoryCache(InventoryCacheConfig{
			PrimaryCache:       p.Cache,
			Events:             local.NewEventsService(bk),
			Inventory:          mockInventory,
			BotInstanceBackend: mockBotCache,
			TargetVersion:      "19.0.0",
		})
		require.NoError(t, err)
		defer inventoryCache.Close()

		// Wait for the inventory cache to initialize by blocking until all the initialization goroutines are durably blocked, which means
		// the cache has been initialized.
		synctest.Wait()
		require.True(t, inventoryCache.IsHealthy())

		// Test searching by hostname
		resp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				Search: "node1",
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "node1.example.com", resp.Items[0].GetInstance().Spec.Hostname)

		// Test searching by bot name
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				Search:        "bot2",
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "bot2", resp.Items[0].GetBotInstance().Spec.BotName)

		// Test filtering by services
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				Services:      []string{string(types.RoleAuth), string(types.RoleProxy)},
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 2)
		require.Equal(t, "auth1.example.com", resp.Items[0].GetInstance().Spec.Hostname)
		require.Equal(t, "node2.example.com", resp.Items[1].GetInstance().Spec.Hostname)

		// Test filtering by updater groups
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				UpdaterGroups: []string{"group1"},
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 2)

		// Test filtering by external upgraders
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				Upgraders:     []string{"kube"},
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 2)
		for _, item := range resp.Items {
			require.Equal(t, "kube", item.GetInstance().Spec.ExternalUpgrader)
		}

		// Test predicate query filtering by version (less than)
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				PredicateExpression: `older_than(status.latest_heartbeat.version, "18.2.0")`,
				InstanceTypes:       []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "18.1.0", resp.Items[0].GetInstance().Spec.Version)

		// Test predicate query filtering by version (greater than) for both instance types.
		// This should return 2 instances and 1 bot instance.
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				PredicateExpression: `newer_than(status.latest_heartbeat.version, "18.1.6")`,
				InstanceTypes:       []inventoryv1.InstanceType{},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 3)

		// Test predicate query filtering by version (between)
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				PredicateExpression: `between(status.latest_heartbeat.version, "18.0.0", "19.0.0")`,
				InstanceTypes:       []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 2)

		// Test predicate query filtering by hostname
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				PredicateExpression: `hostname == "node2.example.com"`,
				InstanceTypes:       []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "node2.example.com", resp.Items[0].GetInstance().Spec.Hostname)

		// Test filtering with multiple filters.
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				Services:            []string{string(types.RoleNode)},
				UpdaterGroups:       []string{"group1"},
				PredicateExpression: `older_than(status.latest_heartbeat.version, "18.2.0")`,
				InstanceTypes:       []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "node1.example.com", resp.Items[0].GetInstance().Spec.Hostname)

		// Test filtering bot instances by updater group
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				UpdaterGroups: []string{"bot-group1"},
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "bot1", resp.Items[0].GetBotInstance().Spec.BotName)

		// Test filtering both instances and bot instances by upgrader
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				Upgraders: []string{"kube"},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 3)
		upgraders := make(map[string]bool)
		for _, item := range resp.Items {
			if item.GetInstance() != nil {
				upgraders[item.GetInstance().Spec.ExternalUpgrader] = true
			} else if item.GetBotInstance() != nil && len(item.GetBotInstance().Status.LatestHeartbeats) > 0 {
				upgraders[item.GetBotInstance().Status.LatestHeartbeats[0].ExternalUpdater] = true
			}
		}
		require.True(t, upgraders["kube"])
		require.Len(t, upgraders, 1)
	})
}

// TestInventoryCacheSorting tests the sorting functionality of the inventory cache
func TestInventoryCacheSorting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		bk, err := memory.New(memory.Config{
			Context: ctx,
		})
		require.NoError(t, err)
		defer bk.Close()

		p, err := setupTestCache(t, cache.ForAuth)
		require.NoError(t, err)
		defer p.Close()

		instances := []*types.InstanceV1{
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance1",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "zzzz",
					Version:  "19.0.0",
					Services: []types.SystemRole{types.RoleNode},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance2",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "aaaa",
					Version:  "18.1.0",
					Services: []types.SystemRole{types.RoleNode},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance3",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "mmmm",
					Version:  "18.2.5",
					Services: []types.SystemRole{types.RoleProxy},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance4",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "cccc",
					Version:  "19.1.0-beta.1", // Prerelease version
					Services: []types.SystemRole{types.RoleAuth},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance5",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "pppp",
					Version:  "19.1.0-alpha", // Prerelease version (should sort before beta)
					Services: []types.SystemRole{types.RoleNode},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance6",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "vvvv",
					Version:  "v18.0.0", // v-prefix (should be parsed correctly)
					Services: []types.SystemRole{types.RoleNode},
				},
			},
			{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: "instance7",
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: "iiii",
					Version:  "not-a-version", // Invalid version (should sort last)
					Services: []types.SystemRole{types.RoleNode},
				},
			},
		}

		// Create test bot instances with different bot names and versions
		bots := []*machineidv1.BotInstance{
			{
				Metadata: &headerv1.Metadata{
					Name: "bot1",
				},
				Spec: &machineidv1.BotInstanceSpec{
					BotName:    "yyyy",
					InstanceId: "bot1",
				},
				Status: &machineidv1.BotInstanceStatus{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						{
							RecordedAt: timestamppb.Now(),
							Version:    "18.0.0",
						},
					},
				},
			},
			{
				Metadata: &headerv1.Metadata{
					Name: "bot2",
				},
				Spec: &machineidv1.BotInstanceSpec{
					BotName:    "bbbb",
					InstanceId: "bot2",
				},
				Status: &machineidv1.BotInstanceStatus{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						{
							RecordedAt: timestamppb.Now(),
							Version:    "19.2.0",
						},
					},
				},
			},
			{
				Metadata: &headerv1.Metadata{
					Name: "bot3",
				},
				Spec: &machineidv1.BotInstanceSpec{
					BotName:    "dddd",
					InstanceId: "bot3",
				},
				Status: &machineidv1.BotInstanceStatus{
					LatestHeartbeats: []*machineidv1.BotInstanceStatusHeartbeat{
						{
							RecordedAt: timestamppb.Now(),
							Version:    "18.1.5",
						},
					},
				},
			},
		}

		mockInventory := &mockInventoryService{instances: instances}
		mockBotCache := &mockBotInstanceCache{bots: bots}

		inventoryCache, err := NewInventoryCache(InventoryCacheConfig{
			PrimaryCache:       p.Cache,
			Events:             local.NewEventsService(bk),
			Inventory:          mockInventory,
			BotInstanceBackend: mockBotCache,
			TargetVersion:      "19.0.0",
		})
		require.NoError(t, err)
		defer inventoryCache.Close()

		// Wait for the inventory cache to initialize by blocking until all the initialization goroutines are durably blocked, which means
		// the cache has been initialized.
		synctest.Wait()
		require.True(t, inventoryCache.IsHealthy())

		// Test sort by name ascending
		resp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_NAME,
			Order:    inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 10)

		expectedNames := []string{
			"aaaa",
			"bbbb",
			"cccc",
			"dddd",
			"iiii",
			"mmmm",
			"pppp",
			"vvvv",
			"yyyy",
			"zzzz",
		}

		require.Equal(t, expectedNames, libslices.Map(resp.Items, getItemName))

		// Test sort by name descending
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_NAME,
			Order:    inventoryv1.SortOrder_SORT_ORDER_DESCENDING,
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 10)

		expectedNames = []string{
			"zzzz",
			"yyyy",
			"vvvv",
			"pppp",
			"mmmm",
			"iiii",
			"dddd",
			"cccc",
			"bbbb",
			"aaaa",
		}

		require.Equal(t, expectedNames, libslices.Map(resp.Items, getItemName))

		// Test sort by type ascending
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_TYPE,
			Order:    inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 10)

		// Expect all bot instances first (sorted by name), then all instances (sorted by name)
		expectedNames = []string{
			"bbbb",
			"dddd",
			"yyyy",
			"aaaa",
			"cccc",
			"iiii",
			"mmmm",
			"pppp",
			"vvvv",
			"zzzz",
		}

		require.Equal(t, expectedNames, libslices.Map(resp.Items, getItemName))

		// Test sort by type descending
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_TYPE,
			Order:    inventoryv1.SortOrder_SORT_ORDER_DESCENDING,
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 10)

		// Expect all instances first (desc sorted by name), then all bot instances (desc sorted by name)
		expectedNames = []string{
			"zzzz",
			"vvvv",
			"pppp",
			"mmmm",
			"iiii",
			"cccc",
			"aaaa",
			"yyyy",
			"dddd",
			"bbbb",
		}

		require.Equal(t, expectedNames, libslices.Map(resp.Items, getItemName))

		// Test sorting by version ascending
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:    inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 10)

		expectedVersions := []string{
			"v18.0.0", // v-prefix version (treated as 18.0.0)
			"18.0.0",
			"18.1.0",
			"18.1.5",
			"18.2.5",
			"19.0.0",
			"19.1.0-alpha",
			"19.1.0-beta.1",
			"19.2.0",
			"not-a-version", // invalid version
		}

		require.Equal(t, expectedVersions, libslices.Map(resp.Items, getItemVersion))

		// Test sort by version descending
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:    inventoryv1.SortOrder_SORT_ORDER_DESCENDING,
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 10)

		expectedVersions = []string{
			"not-a-version",
			"19.2.0",
			"19.1.0-beta.1",
			"19.1.0-alpha",
			"19.0.0",
			"18.2.5",
			"18.1.5",
			"18.1.0",
			"18.0.0",
			"v18.0.0",
		}

		require.Equal(t, expectedVersions, libslices.Map(resp.Items, getItemVersion))

		// Test sorting by version with pagination
		firstPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 3,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:    inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, firstPageResp.Items, 3)
		require.NotEmpty(t, firstPageResp.NextPageToken)

		// Verify first page has the correct versions
		expectedFirstPageVersions := []string{"v18.0.0", "18.0.0", "18.1.0"}
		require.Equal(t, expectedFirstPageVersions, libslices.Map(firstPageResp.Items, getItemVersion))

		// Fetch second page
		secondPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize:  3,
			PageToken: firstPageResp.NextPageToken,
			Sort:      inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:     inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, secondPageResp.Items, 3)
		require.NotEmpty(t, secondPageResp.NextPageToken)

		// Verify second page has the correct versions
		expectedSecondPageVersions := []string{"18.1.5", "18.2.5", "19.0.0"}
		require.Equal(t, expectedSecondPageVersions, libslices.Map(secondPageResp.Items, getItemVersion))

		// Fetch third page
		thirdPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize:  3,
			PageToken: secondPageResp.NextPageToken,
			Sort:      inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:     inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, thirdPageResp.Items, 3)
		require.NotEmpty(t, thirdPageResp.NextPageToken)

		// Verify third page has the correct versions
		expectedThirdPageVersions := []string{"19.1.0-alpha", "19.1.0-beta.1", "19.2.0"}
		require.Equal(t, expectedThirdPageVersions, libslices.Map(thirdPageResp.Items, getItemVersion))

		// Fetch fourth page
		fourthPageResp, err := inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize:  3,
			PageToken: thirdPageResp.NextPageToken,
			Sort:      inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:     inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
		})
		require.NoError(t, err)
		require.Len(t, fourthPageResp.Items, 1)
		require.Empty(t, fourthPageResp.NextPageToken)

		// Verify fourth page has the invalid version
		var actualVersion string
		if instance := fourthPageResp.Items[0].GetInstance(); instance != nil {
			actualVersion = instance.Spec.Version
		} else if bot := fourthPageResp.Items[0].GetBotInstance(); bot != nil {
			actualVersion = bot.Status.LatestHeartbeats[0].Version
		}
		require.Equal(t, "not-a-version", actualVersion)

		// Test sorting by version while filtering for only instances
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:    inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 7)

		expectedVersions = []string{"v18.0.0", "18.1.0", "18.2.5", "19.0.0", "19.1.0-alpha", "19.1.0-beta.1", "not-a-version"}
		require.Equal(t, expectedVersions, libslices.Map(resp.Items, func(item *inventoryv1.UnifiedInstanceItem) string {
			return item.GetInstance().Spec.Version
		}))

		// Test sorting by version while filtering for only bot instances
		resp, err = inventoryCache.ListUnifiedInstances(ctx, &inventoryv1.ListUnifiedInstancesRequest{
			PageSize: 100,
			Sort:     inventoryv1.UnifiedInstanceSort_UNIFIED_INSTANCE_SORT_VERSION,
			Order:    inventoryv1.SortOrder_SORT_ORDER_ASCENDING,
			Filter: &inventoryv1.ListUnifiedInstancesFilter{
				InstanceTypes: []inventoryv1.InstanceType{inventoryv1.InstanceType_INSTANCE_TYPE_BOT_INSTANCE},
			},
		})
		require.NoError(t, err)
		require.Len(t, resp.Items, 3)

		expectedBotVersions := []string{"18.0.0", "18.1.5", "19.2.0"}
		require.Equal(t, expectedBotVersions, libslices.Map(resp.Items, func(item *inventoryv1.UnifiedInstanceItem) string {
			return item.GetBotInstance().Status.LatestHeartbeats[0].Version
		}))
	})
}

// TestSemverSorting tests that semver ordering works properly
func TestSemverSorting(t *testing.T) {
	// Expected ordering
	versions := []struct {
		version string
		desc    string
	}{
		{"1.0.0-1", "prerelease numeric 1"},
		{"1.0.0-2", "prerelease numeric 2"},
		{"1.0.0-11", "prerelease numeric 11"},
		{"1.0.0-alpha", "prerelease alpha"},
		{"1.0.0-alpha.1", "prerelease alpha.1"},
		{"1.0.0-beta", "prerelease beta"},
		{"1.0.0", "release"},
		{"1.0.1", "patch increment"},
		{"1.1.0", "minor version increment"},
		{"2.0.0", "major version increment"},
		{"invalid", "invalid version"},
	}

	// Create the instances
	instances := make([]*inventoryInstance, len(versions))
	for i, v := range versions {
		instances[i] = &inventoryInstance{
			instance: &types.InstanceV1{
				ResourceHeader: types.ResourceHeader{
					Metadata: types.Metadata{
						Name: fmt.Sprintf("instance-%d", i),
					},
				},
				Spec: types.InstanceSpecV1{
					Hostname: fmt.Sprintf("host-%d", i),
					Version:  v.version,
				},
			},
		}
	}

	// Sort instances by version key
	sorted := slices.SortedFunc(slices.Values(instances), func(a, b *inventoryInstance) int {
		keyA := a.getVersionKey()
		keyB := b.getVersionKey()
		if keyA < keyB {
			return -1
		} else if keyA > keyB {
			return 1
		}
		return 0
	})

	require.Equal(t, instances, sorted)
}

// TestGetVersionKeyOrdering verifies that the version keys returned by getVersionKey
// are correct and match semver.Compare
func TestGetVersionKeyOrdering(t *testing.T) {
	versions := []string{
		"1.0.0-alpha",
		"2.1.0-rc.1",
		"1.0.0-alpha.1",
		"v2.1.0",
		"1.1.0",
		"1.0.0-beta",
		"1.0.0",
		"1.0.0-beta.2",
		"1.0.0-alpha.beta",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.1",
		"1.2.0-alpha",
		"1.2.0",
		"2.0.0",
		"v3.0.0-beta",
	}

	// Sort with semver.Compare
	semverSorted := slices.Clone(versions)
	slices.SortFunc(semverSorted, func(a, b string) int {
		semverA := semver.New(strings.TrimPrefix(a, "v"))
		semverB := semver.New(strings.TrimPrefix(b, "v"))
		return semverA.Compare(*semverB)
	})

	// Sort with getVersionKey
	keySorted := slices.Clone(versions)
	slices.SortFunc(keySorted, func(a, b string) int {
		instA := &inventoryInstance{instance: &types.InstanceV1{Spec: types.InstanceSpecV1{Version: a}}}
		instB := &inventoryInstance{instance: &types.InstanceV1{Spec: types.InstanceSpecV1{Version: b}}}
		return cmp.Compare(instA.getVersionKey(), instB.getVersionKey())
	})

	require.Equal(t, semverSorted, keySorted)
}

// TestGetUnifiedExpressionParser tests that the parser can be initialized successfully
func TestGetUnifiedExpressionParser(t *testing.T) {
	parser := getUnifiedExpressionParser()
	require.NotNil(t, parser)

	expr, err := parser.Parse(`hostname == "test"`)
	require.NoError(t, err)
	require.NotNil(t, expr)

	// Verify the between function is available
	expr, err = parser.Parse(`between(version, "1.0.0", "2.0.0")`)
	require.NoError(t, err)
	require.NotNil(t, expr)

	// Test an invalid predicate expression
	expr, err = parser.Parse(`asdafafa("afafafa")`)
	require.Error(t, err)
	require.Nil(t, expr)
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
