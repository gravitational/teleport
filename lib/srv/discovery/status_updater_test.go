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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

func statusFoundInstances(found uint64) *discoveryconfigv1.IntegrationDiscoveredSummary {
	return &discoveryconfigv1.IntegrationDiscoveredSummary{
		AwsEc2: &discoveryconfigv1.ResourcesDiscoveredSummary{
			Found: found,
		},
	}
}

func TestMergeSummaries(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name                    string
		newStatus               discoveryconfig.Status
		existingDiscoveryConfig *discoveryconfig.DiscoveryConfig
		expectedStatus          discoveryconfig.Status
	}{
		{
			name: "merging into discoveryconfig",
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(10),
				},
			},
			existingDiscoveryConfig: nil,
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(10),
				},
			},
		},
		{
			name: "existing has no history, a new history entry is created",
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(10),
				},
			},
			existingDiscoveryConfig: &discoveryconfig.DiscoveryConfig{
				Status: discoveryconfig.Status{
					IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
						"my-integration": statusFoundInstances(5),
					},
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(10),
				},
				IntegrationDiscoveredResourcesHistory: map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
					"my-integration": {
						Summaries: []*discoveryconfigv1.IntegrationDiscoveredSummary{
							statusFoundInstances(5),
						},
					},
				},
			},
		},
		{
			name: "existing has history, a new history entry is added",
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(15),
				},
			},
			existingDiscoveryConfig: &discoveryconfig.DiscoveryConfig{
				Status: discoveryconfig.Status{
					IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
						"my-integration": statusFoundInstances(10),
					},
					IntegrationDiscoveredResourcesHistory: map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
						"my-integration": {
							Summaries: []*discoveryconfigv1.IntegrationDiscoveredSummary{
								statusFoundInstances(5),
							},
						},
					},
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(15),
				},
				IntegrationDiscoveredResourcesHistory: map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
					"my-integration": {
						Summaries: []*discoveryconfigv1.IntegrationDiscoveredSummary{
							statusFoundInstances(10),
							statusFoundInstances(5),
						},
					},
				},
			},
		},
		{
			name: "a max of 5 items are kept in history",
			newStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(100),
				},
			},
			existingDiscoveryConfig: &discoveryconfig.DiscoveryConfig{
				Status: discoveryconfig.Status{
					IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
						"my-integration": statusFoundInstances(90),
					},
					IntegrationDiscoveredResourcesHistory: map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
						"my-integration": {
							Summaries: []*discoveryconfigv1.IntegrationDiscoveredSummary{
								statusFoundInstances(80),
								statusFoundInstances(70),
								statusFoundInstances(60),
								statusFoundInstances(50),
								statusFoundInstances(40),
							},
						},
					},
				},
			},
			expectedStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(100),
				},
				IntegrationDiscoveredResourcesHistory: map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
					"my-integration": {
						Summaries: []*discoveryconfigv1.IntegrationDiscoveredSummary{
							statusFoundInstances(90),
							statusFoundInstances(80),
							statusFoundInstances(70),
							statusFoundInstances(60),
							statusFoundInstances(50),
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeSummaries(tc.newStatus, tc.existingDiscoveryConfig)
			require.True(t, cmp.Equal(got, tc.expectedStatus, protocmp.Transform()), "unexpected result: %s", cmp.Diff(got, tc.expectedStatus, protocmp.Transform()))
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

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
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(5),
				},
			},
			existingDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{},
			expectedDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
				"test-config": {
					Status: discoveryconfig.Status{
						IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
							"my-integration": statusFoundInstances(5),
						},
					},
				},
			},
		},
		{
			name:                "successfully updates an existing discovery config and adds historical data",
			discoveryConfigName: "test-config",
			discoveryConfigStatus: discoveryconfig.Status{
				IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
					"my-integration": statusFoundInstances(5),
				},
			},
			existingDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
				"test-config": {
					Status: discoveryconfig.Status{
						IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
							"my-integration": statusFoundInstances(3),
						},
					},
				},
			},
			expectedDiscoveryConfigs: map[string]*discoveryconfig.DiscoveryConfig{
				"test-config": {
					Status: discoveryconfig.Status{
						IntegrationDiscoveredResources: map[string]*discoveryconfigv1.IntegrationDiscoveredSummary{
							"my-integration": statusFoundInstances(5),
						},
						IntegrationDiscoveredResourcesHistory: map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
							"my-integration": {
								Summaries: []*discoveryconfigv1.IntegrationDiscoveredSummary{
									statusFoundInstances(3),
								},
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
				clock:                  clockwork.NewFakeClock(),
				semaphoreService:       semaphoreService,
				discoveryConfigService: discoveryConfigService,
			}

			err := updater.update(t.Context(), tc.discoveryConfigName, tc.discoveryConfigStatus)
			require.NoError(t, err)

			// Verify the final state of the discovery configs
			for name, expectedConfig := range tc.expectedDiscoveryConfigs {
				require.Contains(t, tc.existingDiscoveryConfigs, name)
				storedDiscoveryConfig := tc.existingDiscoveryConfigs[name]

				require.True(t, cmp.Equal(expectedConfig, storedDiscoveryConfig, protocmp.Transform()), "unexpected discovery config for %s: %s", name)
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
