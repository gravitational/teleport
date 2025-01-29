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

package server

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

type mockClients struct {
	cloud.Clients

	azureClient azure.VirtualMachinesClient
}

func (c *mockClients) GetAzureVirtualMachinesClient(subscription string) (azure.VirtualMachinesClient, error) {
	return c.azureClient, nil
}

func TestAzureWatcher(t *testing.T) {
	t.Parallel()

	clients := mockClients{
		azureClient: azure.NewVirtualMachinesClientByAPI(&azure.ARMComputeMock{
			VirtualMachines: map[string][]*armcompute.VirtualMachine{
				"rg1": {
					{
						ID:       to.Ptr("vm1"),
						Location: to.Ptr("location1"),
					},
					{
						ID:       to.Ptr("vm2"),
						Location: to.Ptr("location1"),
						Tags: map[string]*string{
							"teleport": to.Ptr("yes"),
						},
					},
					{
						ID:       to.Ptr("vm5"),
						Location: to.Ptr("location2"),
					},
				},
				"rg2": {
					{
						ID:       to.Ptr("vm3"),
						Location: to.Ptr("location1"),
					},
					{
						ID:       to.Ptr("vm4"),
						Location: to.Ptr("location1"),
						Tags: map[string]*string{
							"teleport": to.Ptr("yes"),
						},
					},
					{
						ID:       to.Ptr("vm6"),
						Location: to.Ptr("location2"),
					},
				},
			},
		}),
	}

	tests := []struct {
		name    string
		matcher types.AzureMatcher
		wantVMs []string
	}{
		{
			name: "all vms",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{"location1", "location2"},
				ResourceTags:   types.Labels{"*": []string{"*"}},
			},
			wantVMs: []string{"vm1", "vm2", "vm3", "vm4", "vm5", "vm6"},
		},
		{
			name: "filter by resource group",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1"},
				Regions:        []string{"location1", "location2"},
				ResourceTags:   types.Labels{"*": []string{"*"}},
			},
			wantVMs: []string{"vm1", "vm2", "vm5"},
		},
		{
			name: "filter by location",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{"location2"},
				ResourceTags:   types.Labels{"*": []string{"*"}},
			},
			wantVMs: []string{"vm5", "vm6"},
		},
		{
			name: "filter by tag",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{"location1", "location2"},
				ResourceTags:   types.Labels{"teleport": []string{"yes"}},
			},
			wantVMs: []string{"vm2", "vm4"},
		},
		{
			name: "location wildcard",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{types.Wildcard},
				ResourceTags:   types.Labels{"*": []string{"*"}},
			},
			wantVMs: []string{"vm1", "vm2", "vm3", "vm4", "vm5", "vm6"},
		},
	}

	for _, tc := range tests {
		tc.matcher.Types = []string{"vm"}
		tc.matcher.Subscriptions = []string{"sub1"}

		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)
			watcher, err := NewAzureWatcher(ctx, func() []Fetcher {
				return MatchersToAzureInstanceFetchers([]types.AzureMatcher{tc.matcher}, &clients, "" /* discovery config */)
			})
			require.NoError(t, err)

			go watcher.Run()
			t.Cleanup(watcher.Stop)

			var vmIDs []string

			for len(vmIDs) < len(tc.wantVMs) {
				select {
				case results := <-watcher.InstancesC:
					for _, vm := range results.Azure.Instances {
						vmIDs = append(vmIDs, *vm.ID)
					}
				case <-ctx.Done():
					require.Fail(t, "Expected %v VMs, got %v", tc.wantVMs, len(vmIDs))
				}
			}

			require.ElementsMatch(t, tc.wantVMs, vmIDs)
		})
	}
}
