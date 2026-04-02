/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package discovery

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

func integrationDiscoveredSummaryFinished(found uint64, syncEnd *timestamppb.Timestamp) *discoveryconfig.IntegrationDiscoveredSummary {
	return &discoveryconfig.IntegrationDiscoveredSummary{
		IntegrationDiscoveredSummary: &discoveryconfigv1.IntegrationDiscoveredSummary{
			AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{
				Found:   found,
				SyncEnd: syncEnd,
			},
		},
	}
}

func integrationDiscoveredSummaryWithFound(found uint64) *discoveryconfig.IntegrationDiscoveredSummary {
	return &discoveryconfig.IntegrationDiscoveredSummary{
		IntegrationDiscoveredSummary: &discoveryconfigv1.IntegrationDiscoveredSummary{
			AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{
				Found: found,
			},
		},
	}
}

func discoveryStatusServerWithCurrent(found uint64, lastUpdate *timestamppb.Timestamp) *discoveryconfigv1.DiscoveryStatusServer {
	return &discoveryconfigv1.DiscoveryStatusServer{
		IntegrationSummaries: map[string]*discoveryconfigv1.DiscoverSummary{
			"my-integration": {
				AwsEc2: &discoveryconfigv1.ResourceSummary{
					Current: &discoveryconfigv1.ResourcesDiscoveredSummary{
						Found: found,
					},
				},
			},
		},
		LastUpdate:   lastUpdate,
		PollInterval: durationpb.New(5 * time.Minute),
	}
}

func discoveryStatusServerFinished(previous uint64, syncEnd *timestamppb.Timestamp) *discoveryconfigv1.DiscoveryStatusServer {
	return &discoveryconfigv1.DiscoveryStatusServer{
		IntegrationSummaries: map[string]*discoveryconfigv1.DiscoverSummary{
			"my-integration": {
				AwsEc2: &discoveryconfigv1.ResourceSummary{
					Previous: &discoveryconfigv1.ResourcesDiscoveredSummary{
						Found:   previous,
						SyncEnd: syncEnd,
					},
				},
			},
		},
		LastUpdate:   syncEnd,
		PollInterval: durationpb.New(5 * time.Minute),
	}
}

func TestDiscoveryConfigStatusUpdater_merger(t *testing.T) {
	t.Parallel()
	fakeClock := clockwork.NewFakeClock()

	syncEnd := timestamppb.New(fakeClock.Now())
	lastSync := timestamppb.New(fakeClock.Now())

	for _, tc := range []struct {
		name                    string
		serverID                string
		existingDiscoveryConfig *discoveryconfig.DiscoveryConfig
		newStatus               discoveryconfig.Status
		expectedStatus          discoveryconfig.Status
	}{
		{
			name:                    "no existing status: adds server iteration with the current summary",
			serverID:                "server1",
			existingDiscoveryConfig: nil,
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(10),
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(10),
				},
				ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
					"server1": {
						DiscoveryStatusServer: discoveryStatusServerWithCurrent(10, lastSync),
					},
				},
			},
		},
		{
			name:     "status is updated when summary changes",
			serverID: "server1",
			existingDiscoveryConfig: &discoveryconfig.DiscoveryConfig{
				Status: discoveryconfig.Status{
					IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
						"my-integration": integrationDiscoveredSummaryWithFound(5),
					},
					ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
						"server1": {
							DiscoveryStatusServer: discoveryStatusServerWithCurrent(5, lastSync),
						},
					},
				},
			},
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(10),
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(10),
				},
				ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
					"server1": {
						DiscoveryStatusServer: discoveryStatusServerWithCurrent(10, lastSync),
					},
				},
			},
		},
		{
			name:     "other server IDs are not affected when updating server iterations",
			serverID: "server1",
			existingDiscoveryConfig: &discoveryconfig.DiscoveryConfig{
				Status: discoveryconfig.Status{
					ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
						"server2": {
							DiscoveryStatusServer: discoveryStatusServerWithCurrent(5, lastSync),
						},
					},
				},
			},
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(10),
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(10),
				},
				ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
					"server2": {
						DiscoveryStatusServer: discoveryStatusServerWithCurrent(5, lastSync),
					},
					"server1": {
						DiscoveryStatusServer: discoveryStatusServerWithCurrent(10, lastSync),
					},
				},
			},
		},
		{
			name:     "when iteration ends, the current summary is emptied and the previous summary is populated with the last known summary",
			serverID: "server1",
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryFinished(10, syncEnd),
				},
			},
			existingDiscoveryConfig: &discoveryconfig.DiscoveryConfig{
				Status: discoveryconfig.Status{
					IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
						"my-integration": integrationDiscoveredSummaryWithFound(5),
					},
					ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
						"server1": {
							DiscoveryStatusServer: discoveryStatusServerWithCurrent(5, lastSync),
						},
					},
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryFinished(10, syncEnd),
				},
				ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
					"server1": {
						DiscoveryStatusServer: discoveryStatusServerFinished(10, syncEnd),
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updater := &discoveryConfigStatusUpdater{
				clock:        fakeClock,
				pollInterval: 5 * time.Minute,
				serverID:     tc.serverID,
			}
			newStatus := updater.updateServerStatus(tc.newStatus, tc.existingDiscoveryConfig)
			require.True(t, cmp.Equal(newStatus, tc.expectedStatus, protocmp.Transform()), "unexpected result: %s", cmp.Diff(newStatus, tc.expectedStatus, protocmp.Transform()))
		})
	}
}

func TestDiscoveryConfigStatusUpdater(t *testing.T) {
	t.Parallel()
	fakeClock := clockwork.NewFakeClock()

	syncEnd := timestamppb.New(fakeClock.Now())
	lastSync := timestamppb.New(fakeClock.Now())

	for _, tc := range []struct {
		name                     string
		discoveryConfigName      string
		discoveryConfigStatus    discoveryconfig.Status
		existingDiscoveryConfigs map[string]*discoveryconfig.DiscoveryConfig
		expectedDiscoveryConfigs map[string]*discoveryconfig.DiscoveryConfig
	}{
		{
			name:                "successfully update discovery config status when no existing config is found",
			discoveryConfigName: "test-config",
			discoveryConfigStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryWithFound(5),
				},
			},
			existingDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{},
			expectedDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
				"test-config": {
					Status: discoveryconfig.Status{
						IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
							"my-integration": integrationDiscoveredSummaryWithFound(5),
						},
						ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
							"test-server": {
								DiscoveryStatusServer: discoveryStatusServerWithCurrent(5, lastSync),
							},
						},
					},
				},
			},
		},
		{
			name:                "successfully updates the previous field when iteration ends",
			discoveryConfigName: "test-config",
			discoveryConfigStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
					"my-integration": integrationDiscoveredSummaryFinished(5, syncEnd),
				},
			},
			existingDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
				"test-config": {
					Status: discoveryconfig.Status{
						IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
							"my-integration": integrationDiscoveredSummaryWithFound(3),
						},
						ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
							"test-server": {
								DiscoveryStatusServer: discoveryStatusServerWithCurrent(3, syncEnd),
							},
						},
					},
				},
			},
			expectedDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
				"test-config": {
					Status: discoveryconfig.Status{
						IntegrationDiscoveredResources: map[string]*discoveryconfig.IntegrationDiscoveredSummary{
							"my-integration": integrationDiscoveredSummaryFinished(5, syncEnd),
						},
						ServerStatus: map[string]*discoveryconfig.DiscoveryStatusServer{
							"test-server": {
								DiscoveryStatusServer: discoveryStatusServerFinished(5, syncEnd),
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			discoveryConfigService := &mockDiscoveryConfigService{
				discoveryConfigs: tc.existingDiscoveryConfigs,
			}
			semaphoreService := &mockSemaphore{}

			updater := &discoveryConfigStatusUpdater{
				log:                    slog.New(slog.DiscardHandler),
				serverID:               "test-server",
				clock:                  fakeClock,
				pollInterval:           5 * time.Minute,
				semaphoreService:       semaphoreService,
				discoveryConfigService: discoveryConfigService,
			}

			err := updater.update(t.Context(), tc.discoveryConfigName, tc.discoveryConfigStatus)
			require.NoError(t, err)

			// Verify the final state of the discovery configs
			for name, expectedConfig := range tc.expectedDiscoveryConfigs {
				require.Contains(t, tc.existingDiscoveryConfigs, name)
				storedDiscoveryConfig := tc.existingDiscoveryConfigs[name]

				diff := cmp.Diff(expectedConfig.Status.ServerStatus, storedDiscoveryConfig.Status.ServerStatus, protocmp.Transform())
				require.Empty(t, diff, "mismatch between expected and stored discovery config: %s", diff)
			}

			require.False(t, semaphoreService.stateLocked, "semaphore should be released after update")
		})
	}
}

type mockDiscoveryConfigService struct {
	discoveryConfigs map[string]*discoveryconfig.DiscoveryConfig
}

func (m *mockDiscoveryConfigService) GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error) {
	if config, ok := m.discoveryConfigs[name]; ok {
		return config, nil
	}
	return nil, trace.NotFound("discovery config not found: %s", name)
}

func (m *mockDiscoveryConfigService) UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error) {
	config, ok := m.discoveryConfigs[name]
	if !ok {
		config = &discoveryconfig.DiscoveryConfig{}
	}
	config.Status = status
	m.discoveryConfigs[name] = config
	return config, nil
}

type mockSemaphore struct {
	mtx         sync.Mutex
	stateLocked bool
}

func (m *mockSemaphore) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	m.mtx.Lock()
	m.stateLocked = true
	return &types.SemaphoreLease{
		Expires: params.Expires,
	}, nil
}

func (m *mockSemaphore) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	m.stateLocked = false
	m.mtx.Unlock()
	return nil
}
