/*
Copyright 2022 Gravitational, Inc.

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

package server

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
)

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
	}

	for _, tc := range tests {
		tc.matcher.Types = []string{"vm"}
		tc.matcher.Subscriptions = []string{"sub1"}

		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)
			watcher, err := NewAzureWatcher(ctx, []types.AzureMatcher{tc.matcher}, &clients)
			require.NoError(t, err)

			go watcher.Run()
			t.Cleanup(watcher.Stop)

			var vmIDs []string

			for len(vmIDs) < len(tc.wantVMs) {
				select {
				case results := <-watcher.InstancesC:
					for _, vm := range results.AzureInstances.Instances {
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
