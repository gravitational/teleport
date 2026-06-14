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
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/cloud/azure"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type mockClients struct {
	azure.Clients
	resourceGraphClient azure.ResourceGraphClient

	vmClients map[string]azure.VirtualMachinesClient
}

func (c *mockClients) GetVirtualMachinesClient(ctx context.Context, subscription string) (azure.VirtualMachinesClient, error) {
	vmClient, ok := c.vmClients[subscription]
	if !ok {
		return nil, trace.NotFound("subscription %s not found", subscription)
	}
	return vmClient, nil
}

func (c *mockClients) GetResourceGraphClient(ctx context.Context) (azure.ResourceGraphClient, error) {
	if c.resourceGraphClient == nil {
		return nil, trace.NotFound("resource graph client not found")
	}
	return c.resourceGraphClient, nil
}

// fakeResourceGraphClient is an in-memory implementation of argResourcesAPI that lets each
// test pin the exact response sequence (or error) for the Resources call.
type fakeResourceGraphClient struct {
	vmsBySubscriptionAndResourceGroup map[string]map[string][]*azure.VirtualMachine
	queryError                        error
}

func (f *fakeResourceGraphClient) QueryLinuxVMs(_ context.Context, params azure.QueryLinuxVMsParams) ([]*azure.VirtualMachine, error) {
	if f.queryError != nil {
		return nil, f.queryError
	}
	vmsBySubscription, ok := f.vmsBySubscriptionAndResourceGroup[params.SubscriptionID]
	if !ok {
		return nil, trace.NotFound("subscription %s not found", params.SubscriptionID)
	}

	if params.ResourceGroup != "" && params.ResourceGroup != "*" {
		vmsByResourceGroup, ok := vmsBySubscription[params.ResourceGroup]
		if !ok {
			return nil, trace.NotFound("resource group %s not found in subscription %s", params.ResourceGroup, params.SubscriptionID)
		}

		return vmsByResourceGroup, nil
	}

	// return vms from all resource groups in the subscription
	var vms []*azure.VirtualMachine
	for _, vmsInResourceGroup := range vmsBySubscription {
		vms = append(vms, vmsInResourceGroup...)
	}

	return vms, nil
}

func makeAzureVM(subscription, location, resourceGroup, vmName string, tags map[string]string) *azure.VirtualMachine {
	return &azure.VirtualMachine{
		ID:       makeAzureVMID(subscription, resourceGroup, vmName),
		Name:     vmName,
		Location: location,
		Tags:     tags,
	}
}

func TestAzureWatcher(t *testing.T) {
	t.Parallel()

	const (
		sub1 = "00000000-0000-0000-0000-000000000000"
		sub2 = "11111111-1111-1111-1111-111111111111"
	)

	clients := mockClients{
		resourceGraphClient: &fakeResourceGraphClient{
			vmsBySubscriptionAndResourceGroup: map[string]map[string][]*azure.VirtualMachine{
				sub1: {
					"rg1": {
						makeAzureVM(sub1, "location1", "rg1", "vm1", nil),
						makeAzureVM(sub1, "location1", "rg1", "vm2", map[string]string{"teleport": "yes"}),
						makeAzureVM(sub1, "location2", "rg1", "vm5", nil),
					},
					"rg2": {
						makeAzureVM(sub1, "location1", "rg2", "vm3", nil),
						makeAzureVM(sub1, "location1", "rg2", "vm4", map[string]string{"teleport": "yes"}),
						makeAzureVM(sub1, "location2", "rg2", "vm6", nil),
					},
				},
				sub2: {
					"rg3": {
						makeAzureVM(sub2, "location1", "rg3", "vm7", nil),
						makeAzureVM(sub2, "location1", "rg3", "vm8", map[string]string{"teleport": "yes"}),
						makeAzureVM(sub2, "location2", "rg3", "vm9", nil),
					},
					"rg4": {
						makeAzureVM(sub2, "location1", "rg4", "vm10", nil),
						makeAzureVM(sub2, "location1", "rg4", "vm11", map[string]string{"teleport": "yes"}),
						makeAzureVM(sub2, "location2", "rg4", "vm12", nil),
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		matcher types.AzureMatcher
		wantVMs []string
	}{
		{
			name: "all vms in a subscription",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{"location1", "location2"},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{sub1},
			},
			wantVMs: []string{"vm1", "vm2", "vm3", "vm4", "vm5", "vm6"},
		},
		{
			name: "filter by resource group",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1"},
				Regions:        []string{"location1", "location2"},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{sub1},
			},
			wantVMs: []string{"vm1", "vm2", "vm5"},
		},
		{
			name: "filter by location",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{"location2"},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{sub1},
			},
			wantVMs: []string{"vm5", "vm6"},
		},
		{
			name: "filter by tag",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{"location1", "location2"},
				ResourceTags:   types.Labels{"teleport": []string{"yes"}},
				Subscriptions:  []string{sub1},
			},
			wantVMs: []string{"vm2", "vm4"},
		},
		{
			name: "location wildcard",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg2"},
				Regions:        []string{types.Wildcard},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{sub1},
			},
			wantVMs: []string{"vm1", "vm2", "vm3", "vm4", "vm5", "vm6"},
		},
		{
			name: "resource group wildcard",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"*"},
				Regions:        []string{types.Wildcard},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{sub1},
			},
			wantVMs: []string{"vm1", "vm2", "vm3", "vm4", "vm5", "vm6"},
		},
		{
			name: "subscription wildcard",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"rg1", "rg4"},
				Regions:        []string{types.Wildcard},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{"*"},
			},
			wantVMs: []string{"vm1", "vm2", "vm5", "vm10", "vm11", "vm12"},
		},
		{
			name: "subscription wildcard with resource group wildcard",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{"*"},
				Regions:        []string{types.Wildcard},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{"*"},
			},
			wantVMs: []string{"vm1", "vm2", "vm3", "vm4", "vm5", "vm6", "vm7", "vm8", "vm9", "vm10", "vm11", "vm12"},
		},
	}

	logger := logtest.NewLogger()
	for _, tc := range tests {
		tc.matcher.Types = []string{"vm"}

		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)
			watcher := NewWatcher[*AzureInstances](ctx, logger)

			const noDiscoveryConfig = ""
			watcher.SetFetchers(noDiscoveryConfig,
				MatchersToAzureInstanceFetchers(
					t.Context(),
					logger,
					[]types.AzureMatcher{tc.matcher},
					func(ctx context.Context, integration string) (azure.Clients, error) {
						return &clients, nil
					},
					noDiscoveryConfig,
					func(ctx context.Context, integration string) (subscriptions []string, err error) {
						return []string{sub1, sub2}, nil
					},
				),
			)

			go watcher.Run()
			t.Cleanup(watcher.Stop)

			var vmIDs []string

			for len(vmIDs) < len(tc.wantVMs) {
				select {
				case results := <-watcher.InstancesC:
					for _, vm := range results.Instances {
						parsedResource, err := arm.ParseResourceID(vm.ID)
						require.NoError(t, err)
						vmID := parsedResource.Name
						vmIDs = append(vmIDs, vmID)
					}
					require.NotEqual(t, "*", results.Metadata.ResourceGroup, "Discovered VM's ResourceGroup should never be the wildcard")
					require.NotEqual(t, "*", results.Metadata.SubscriptionID, "Discovered VM's SubscriptionID should never be the wildcard")
				case <-ctx.Done():
					require.ElementsMatch(t, tc.wantVMs, vmIDs, "timed out while waiting for expected VMs")
				}
			}

			require.ElementsMatch(t, tc.wantVMs, vmIDs)
		})
	}
}

func TestAzureInstancesDisablesResourceGraphUsageOnEnvVariable(t *testing.T) {
	const (
		subscription  = "00000000-0000-0000-0000-000000000000"
		resourceGroup = "rg1"
		location      = "eastus"
	)

	// Each backend returns a distinctly named VM so the test can tell which API
	// the fetcher used purely from the discovered instance.
	const (
		resourceGraphVMName = "from-resource-graph"
		listAPIVMName       = "from-list-api"
	)

	resourceGraphVM := makeAzureVM(subscription, location, resourceGroup, resourceGraphVMName, nil)
	listAPIVM := &armcompute.VirtualMachine{
		ID:       to.Ptr(makeAzureVMID(subscription, resourceGroup, listAPIVMName)),
		Name:     to.Ptr(listAPIVMName),
		Location: to.Ptr(location),
		Properties: &armcompute.VirtualMachineProperties{
			VMID: to.Ptr("list-api-vm-id"),
		},
	}

	clients := &mockClients{
		resourceGraphClient: &fakeResourceGraphClient{
			vmsBySubscriptionAndResourceGroup: map[string]map[string][]*azure.VirtualMachine{
				subscription: {resourceGroup: {resourceGraphVM}},
			},
		},
		vmClients: map[string]azure.VirtualMachinesClient{
			subscription: azure.NewVirtualMachinesClientByAPI(azure.VirtualMachinesClientConfig{
				VirtualMachineAPI: &azure.ARMComputeMock{
					VirtualMachines: map[string][]*armcompute.VirtualMachine{
						resourceGroup: {listAPIVM},
					},
				},
				ScaleSetsAPI:   &azure.ARMScaleSetsMock{},
				ScaleSetVMsAPI: &azure.ARMScaleSetVMsMock{},
			}),
		},
	}

	newFetcher := func() *azureInstanceFetcher {
		return newAzureInstanceFetcher(azureFetcherConfig{
			Matcher: types.AzureMatcher{
				Regions:      []string{types.Wildcard},
				ResourceTags: types.Labels{"*": []string{"*"}},
			},
			Subscription:  subscription,
			ResourceGroup: resourceGroup,
			AzureClientGetter: func(context.Context, string) (azure.Clients, error) {
				return clients, nil
			},
			Logger: logtest.NewLogger(),
		})
	}

	discoveredVMNames := func(t *testing.T, instanceGroups []*AzureInstances) []string {
		t.Helper()
		var names []string
		for _, group := range instanceGroups {
			for _, vm := range group.Instances {
				names = append(names, vm.Name)
			}
		}
		return names
	}

	t.Run("resource graph used by default", func(t *testing.T) {
		instanceGroups, err := newFetcher().GetInstances(t.Context(), false)
		require.NoError(t, err)
		require.Equal(t, []string{resourceGraphVMName}, discoveredVMNames(t, instanceGroups))
	})

	t.Run("uses ListVirtualMachines API when env var is set", func(t *testing.T) {
		t.Setenv("TELEPORT_UNSTABLE_DISABLE_AZURE_RESOURCEGRAPH", "yes")
		instanceGroups, err := newFetcher().GetInstances(t.Context(), false)
		require.NoError(t, err)
		require.Equal(t, []string{listAPIVMName}, discoveredVMNames(t, instanceGroups))
	})
}

func TestAzureInstances_FilterExistingNodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		instances     *AzureInstances
		existingNodes []types.Server
		expectedVMIDs []string
	}{
		{
			name: "no existing nodes",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: "sub-1",
				},
				Instances: []*azure.VirtualMachine{
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
						VMID: "vm-id-1",
					},
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm2",
						VMID: "vm-id-2",
					},
				},
			},
			existingNodes: []types.Server{},
			expectedVMIDs: []string{"vm-id-1", "vm-id-2"},
		},
		{
			name: "filter out matching node",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: "sub-1",
				},
				Instances: []*azure.VirtualMachine{
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
						VMID: "vm-id-1",
					},
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm2",
						VMID: "vm-id-2",
					},
				},
			},
			existingNodes: []types.Server{
				makeAzureNode(t, "node-1", "sub-1", "vm-id-1"),
			},
			expectedVMIDs: []string{"vm-id-2"},
		},
		{
			name: "filter out all matching nodes",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: "sub-1",
				},
				Instances: []*azure.VirtualMachine{
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
						VMID: "vm-id-1",
					},
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm2",
						VMID: "vm-id-2",
					},
				},
			},
			existingNodes: []types.Server{
				makeAzureNode(t, "node-1", "sub-1", "vm-id-1"),
				makeAzureNode(t, "node-2", "sub-1", "vm-id-2"),
			},
			expectedVMIDs: []string{},
		},
		{
			name: "different subscription is not filtered",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: "sub-1",
				},
				Instances: []*azure.VirtualMachine{
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
						VMID: "vm-id-1",
					},
				},
			},
			existingNodes: []types.Server{
				makeAzureNode(t, "node-1", "sub-2", "vm-id-1"),
			},
			expectedVMIDs: []string{"vm-id-1"},
		},
		{
			name: "node without vm id is not used for filtering",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: "sub-1",
				},
				Instances: []*azure.VirtualMachine{
					{
						ID:   "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
						VMID: "vm-id-1",
					},
				},
			},
			existingNodes: []types.Server{
				makeAzureNode(t, "node-1", "sub-1", ""),
			},
			expectedVMIDs: []string{"vm-id-1"},
		},
		{
			name: "instance without properties is not filtered",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: "sub-1",
				},
				Instances: []*azure.VirtualMachine{
					{
						ID: "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1",
					},
				},
			},
			existingNodes: []types.Server{
				makeAzureNode(t, "node-1", "sub-1", "vm-id-1"),
			},
			expectedVMIDs: []string{""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.instances.FilterExistingNodes(tc.existingNodes)

			var gotVMIDs []string
			for _, vm := range tc.instances.Instances {
				gotVMIDs = append(gotVMIDs, vm.VMID)
			}

			require.ElementsMatch(t, tc.expectedVMIDs, gotVMIDs)
		})
	}
}

func makeAzureNode(t *testing.T, name, subscriptionID, vmID string) types.Server {
	t.Helper()

	labels := map[string]string{
		types.SubscriptionIDLabelInternal: subscriptionID,
	}
	if vmID != "" {
		labels[types.VMIDLabelInternal] = vmID
	}

	node, err := types.NewServerWithLabels(name, types.KindNode, types.ServerSpecV2{}, labels)
	require.NoError(t, err)
	return node
}

func makeAzureVMID(subscription, resourceGroup, name string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s",
		subscription, resourceGroup, name,
	)
}

func TestMakeRunEvent(t *testing.T) {
	t.Parallel()

	const (
		subscriptionID = "sub-1"
		resourceGroup  = "rg1"
		region         = "eastus"
		vmID           = "vm-id-1"
		vmName         = "vm1"
		resourceID     = "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1"
	)

	vm := &azure.VirtualMachine{
		ID:   resourceID,
		Name: vmName,
		VMID: vmID,
	}

	tests := []struct {
		name   string
		result AzureInstallResult
		want   *apievents.AzureRun
	}{
		{
			name: "success",
			result: AzureInstallResult{
				Instance: vm,
				CommandResult: &azure.RunCommandResult{
					ExecutionState: "Succeeded",
					ExitCode:       0,
					StdOut:         "ok",
				},
			},
			want: &apievents.AzureRun{
				Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Code: libevents.AzureRunSuccessCode},
				AzureMetadata: apievents.AzureMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
					ResourceID:     resourceID,
				},
				AzureVMMetadata: apievents.AzureVMMetadata{
					VMID:   vmID,
					VMName: vmName,
				},
				Status:         "Installation completed successfully.",
				ExecutionState: "Succeeded",
				StandardOutput: "ok",
			},
		},
		{
			name: "command failure",
			result: AzureInstallResult{
				Instance: vm,
				CommandResult: &azure.RunCommandResult{
					ExecutionState: "Failed",
					ExitCode:       1,
					StdErr:         "something broke",
				},
			},
			want: &apievents.AzureRun{
				Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Code: libevents.AzureRunFailCode},
				AzureMetadata: apievents.AzureMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
					ResourceID:     resourceID,
				},
				AzureVMMetadata: apievents.AzureVMMetadata{
					VMID:   vmID,
					VMName: vmName,
				},
				Status:         "Installation failed with exit code 1. Please check stdout and stderr and try again.",
				ExitCode:       1,
				ExecutionState: "Failed",
				StandardError:  "something broke",
			},
		},
		{
			name: "API error",
			result: AzureInstallResult{
				Instance: vm,
				APIError: trace.AccessDenied("forbidden"),
			},
			want: &apievents.AzureRun{
				Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Code: libevents.AzureRunFailCode},
				AzureMetadata: apievents.AzureMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
					ResourceID:     resourceID,
				},
				AzureVMMetadata: apievents.AzureVMMetadata{
					VMID:   vmID,
					VMName: vmName,
				},
				Status:   "API call failed",
				APIError: "forbidden",
			},
		},
		{
			name: "known exit code",
			result: AzureInstallResult{
				Instance: vm,
				CommandResult: &azure.RunCommandResult{
					ExecutionState: "Failed",
					ExitCode:       102,
				},
			},
			want: &apievents.AzureRun{
				Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Code: libevents.AzureRunFailCode},
				AzureMetadata: apievents.AzureMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
					ResourceID:     resourceID,
				},
				AzureVMMetadata: apievents.AzureVMMetadata{
					VMID:   vmID,
					VMName: vmName,
				},
				Status:         "curl is not installed in the instance. Please install all required tools (bash, sudo, curl) and try again.",
				ExitCode:       102,
				ExecutionState: "Failed",
			},
		},
		{
			name: "nil instance",
			result: AzureInstallResult{
				Instance: nil,
				APIError: trace.AccessDenied("forbidden"),
			},
			want: &apievents.AzureRun{
				Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Code: libevents.AzureRunFailCode},
				AzureMetadata: apievents.AzureMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
				},
				Status:   "API call failed",
				APIError: "forbidden",
			},
		},
		{
			name: "instance without properties",
			result: AzureInstallResult{
				Instance: &azure.VirtualMachine{
					ID:   resourceID,
					Name: vmName,
				},
				APIError: trace.AccessDenied("forbidden"),
			},
			want: &apievents.AzureRun{
				Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Code: libevents.AzureRunFailCode},
				AzureMetadata: apievents.AzureMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
					ResourceID:     resourceID,
				},
				AzureVMMetadata: apievents.AzureVMMetadata{
					VMName: vmName,
				},
				Status:   "API call failed",
				APIError: "forbidden",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			instances := &AzureInstances{
				Metadata: AzureInstancesMetadata{
					SubscriptionID: subscriptionID,
					ResourceGroup:  resourceGroup,
					Region:         region,
				},
			}
			evt := instances.Metadata.MakeRunEvent(tc.result)
			require.Equal(t, tc.want, evt)
		})
	}
}

func TestMakeUsageEvent(t *testing.T) {
	t.Parallel()

	const (
		discoveryConfig = "my-config"
		resourceID      = "/subscriptions/sub-1/resourceGroups/rg1/providers/Microsoft.Compute/virtualMachines/vm1"
	)

	vm := &azure.VirtualMachine{
		ID: resourceID,
	}

	tests := []struct {
		name      string
		instances *AzureInstances
		wantKey   string
		want      *usageeventsv1.ResourceCreateEvent
	}{
		{
			name: "node",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					DiscoveryConfigName: discoveryConfig,
				},
			},
			wantKey: azureEventPrefix + resourceID,
			want: &usageeventsv1.ResourceCreateEvent{
				ResourceType:        types.DiscoveredResourceNode,
				ResourceOrigin:      types.OriginCloud,
				CloudProvider:       types.CloudAzure,
				DiscoveryConfigName: discoveryConfig,
			},
		},
		{
			name: "agentless node",
			instances: &AzureInstances{
				Metadata: AzureInstancesMetadata{
					DiscoveryConfigName: discoveryConfig,
					InstallerParams: &types.InstallerParams{
						ScriptName: installers.InstallerScriptNameAgentless,
					},
				},
			},
			wantKey: azureEventPrefix + resourceID,
			want: &usageeventsv1.ResourceCreateEvent{
				ResourceType:        types.DiscoveredResourceAgentlessNode,
				ResourceOrigin:      types.OriginCloud,
				CloudProvider:       types.CloudAzure,
				DiscoveryConfigName: discoveryConfig,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, evt := tc.instances.Metadata.MakeUsageEvent(vm)
			require.Equal(t, tc.wantKey, key)
			require.Equal(t, tc.want, evt)
		})
	}
}
