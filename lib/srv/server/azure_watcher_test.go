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
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/lib/cloud/azure"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// mockClients exposes Azure clients needed by the watcher tests.
type mockClients struct {
	azure.Clients

	rgClient  azure.ResourceGraphClient
	vmClient  azure.VirtualMachinesClient
	vmClients map[string]azure.VirtualMachinesClient
}

func (c *mockClients) GetResourceGraphClient(_ context.Context) (azure.ResourceGraphClient, error) {
	if c.rgClient == nil {
		return nil, trace.NotFound("resource graph client not configured")
	}
	return c.rgClient, nil
}

func (c *mockClients) GetVirtualMachinesClient(_ context.Context, subscription string) (azure.VirtualMachinesClient, error) {
	if c.vmClients != nil {
		client, ok := c.vmClients[subscription]
		if !ok {
			return nil, trace.NotFound("virtual machines client not configured")
		}
		return client, nil
	}
	if c.vmClient == nil {
		return nil, trace.NotFound("virtual machines client not configured")
	}
	return c.vmClient, nil
}

func makeAzureVMID(subscription, resourceGroup, name string) string {
	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s",
		subscription, resourceGroup, name,
	)
}

func TestAzureScopeLogFieldsBounded(t *testing.T) {
	t.Parallel()

	subscriptions := make([]string, 12)
	for i := range subscriptions {
		subscriptions[i] = fmt.Sprintf("sub-%d", i)
	}
	resourceGroups := []string{"rg-1", "rg-2"}

	attrs := azureScopeLogFields(subscriptions, resourceGroups)
	got := make(map[string]any, len(attrs)/2)
	for i := 0; i < len(attrs); i += 2 {
		got[attrs[i].(string)] = attrs[i+1]
	}

	assert.Equal(t, 12, got["subscription_count"])
	assert.Equal(t, 2, got["subscription_omitted"])
	assert.Equal(t, subscriptions[:azureScopeLogSampleSize], got["subscription_sample"])
	assert.Equal(t, 2, got["resource_group_count"])
	assert.Equal(t, 0, got["resource_group_omitted"])
	assert.Equal(t, resourceGroups, got["resource_group_sample"])

	subscriptions[0] = "mutated"
	assert.Equal(t, "sub-0", got["subscription_sample"].([]string)[0],
		"log samples must not alias caller-owned slices")
}

func TestAzureLabelsMatchAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels types.Labels
		want   bool
	}{
		{
			name: "empty labels match all in Azure watcher",
			want: true,
		},
		{
			name:   "explicit wildcard labels match all",
			labels: types.Labels{types.Wildcard: []string{types.Wildcard}},
			want:   true,
		},
		{
			name:   "concrete labels do not match all",
			labels: types.Labels{"team": []string{"platform"}},
		},
		{
			name:   "wildcard key with concrete values does not match all",
			labels: types.Labels{types.Wildcard: []string{"platform"}},
		},
		{
			name: "wildcard plus concrete labels does not match all",
			labels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
				"team":         []string{"platform"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, azureLabelsMatchAll(tt.labels))
		})
	}
}

type testAzureVMClient struct {
	vmsByResourceGroup  map[string][]*armcompute.VirtualMachine
	errsByResourceGroup map[string]error
}

func (c *testAzureVMClient) Get(context.Context, string) (*azure.VirtualMachine, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (c *testAzureVMClient) GetByVMID(context.Context, string) (*azure.VirtualMachine, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (c *testAzureVMClient) ListVirtualMachines(_ context.Context, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
	if err := c.errsByResourceGroup[resourceGroup]; err != nil {
		return nil, err
	}
	return c.vmsByResourceGroup[resourceGroup], nil
}

func makeAzureNode(t *testing.T, name, subscriptionID, vmID string) types.Server {
	t.Helper()
	srv, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)
	labels := map[string]string{
		types.SubscriptionIDLabelInternal: subscriptionID,
		types.VMIDLabelInternal:           vmID,
	}
	srv.SetStaticLabels(labels)
	return srv
}

func TestAzureWatcher(t *testing.T) {
	t.Parallel()

	const (
		sub1 = "00000000-0000-0000-0000-000000000000"
		sub2 = "11111111-1111-1111-1111-111111111111"
	)

	mkVM := func(sub, rg, name, location string, tags map[string]string) azure.DiscoveredVM {
		if tags == nil {
			tags = map[string]string{}
		}
		return azure.DiscoveredVM{
			ID:             makeAzureVMID(sub, rg, name),
			SubscriptionID: sub,
			Name:           name,
			VMID:           name + "-vm-id",
			Location:       location,
			ResourceGroup:  rg,
			Tags:           tags,
		}
	}

	sub1VMs := []azure.DiscoveredVM{
		mkVM(sub1, "rg1", "vm1", "location1", nil),
		mkVM(sub1, "rg1", "vm2", "location1", map[string]string{"teleport": "yes"}),
		mkVM(sub1, "rg1", "vm5", "location2", nil),
		mkVM(sub1, "rg2", "vm3", "location1", nil),
		mkVM(sub1, "rg2", "vm4", "location1", map[string]string{"teleport": "yes"}),
		mkVM(sub1, "rg2", "vm6", "location2", nil),
	}
	sub2VMs := []azure.DiscoveredVM{
		mkVM(sub2, "rg3", "vm7", "location1", nil),
		mkVM(sub2, "rg3", "vm8", "location1", map[string]string{"teleport": "yes"}),
		mkVM(sub2, "rg3", "vm9", "location2", nil),
		mkVM(sub2, "rg4", "vm10", "location1", nil),
		mkVM(sub2, "rg4", "vm11", "location1", map[string]string{"teleport": "yes"}),
		mkVM(sub2, "rg4", "vm12", "location2", nil),
	}

	// newClients returns a fresh mockClients per subtest so call metadata stays
	// local to each parallel case.
	newClients := func() *mockClients {
		return &mockClients{
			rgClient: &azure.ARMResourceGraphMock{
				VMsBySubscription: map[string][]azure.DiscoveredVM{
					sub1: sub1VMs,
					sub2: sub2VMs,
				},
			},
		}
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
				ResourceGroups: []string{types.Wildcard},
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
				Subscriptions:  []string{types.Wildcard},
			},
			wantVMs: []string{"vm1", "vm2", "vm5", "vm10", "vm11", "vm12"},
		},
		{
			name: "subscription wildcard with resource group wildcard",
			matcher: types.AzureMatcher{
				ResourceGroups: []string{types.Wildcard},
				Regions:        []string{types.Wildcard},
				ResourceTags:   types.Labels{"*": []string{"*"}},
				Subscriptions:  []string{types.Wildcard},
			},
			wantVMs: []string{
				"vm1", "vm2", "vm3", "vm4", "vm5", "vm6",
				"vm7", "vm8", "vm9", "vm10", "vm11", "vm12",
			},
		},
	}

	logger := logtest.NewLogger()
	for _, tc := range tests {
		tc.matcher.Types = []string{"vm"}

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)
			watcher := NewWatcher[*AzureInstances](ctx, logger)

			clients := newClients()

			const noDiscoveryConfig = ""
			watcher.SetFetchers(noDiscoveryConfig,
				MatchersToAzureInstanceFetchers(
					t.Context(),
					logger,
					[]types.AzureMatcher{tc.matcher},
					func(ctx context.Context, integration string) (azure.Clients, error) {
						return clients, nil
					},
					noDiscoveryConfig,
					func(ctx context.Context, integration string) (subscriptions []string, err error) {
						return []string{sub1, sub2}, nil
					},
				),
			)

			go watcher.Run()
			t.Cleanup(watcher.Stop)

			var vmNames []string
			for len(vmNames) < len(tc.wantVMs) {
				select {
				case results := <-watcher.InstancesC:
					for _, vm := range results.Instances {
						vmNames = append(vmNames, vm.Name)
					}
					assert.NotEqual(t, types.Wildcard, results.ResourceGroup,
						"discovered VM's ResourceGroup should never be the wildcard")
					assert.NotEqual(t, types.Wildcard, results.SubscriptionID,
						"discovered VM's SubscriptionID should never be the wildcard")
				case <-ctx.Done():
					require.ElementsMatch(t, tc.wantVMs, vmNames, "timed out while waiting for expected VMs")
				}
			}
			require.ElementsMatch(t, tc.wantVMs, vmNames)
		})
	}
}

func TestAzureInstances_FilterExistingNodes(t *testing.T) {
	t.Parallel()

	const (
		subA = "sub-a"
		subB = "sub-b"
	)

	mkVM := func(name, vmID string) azure.DiscoveredVM {
		return azure.DiscoveredVM{
			ID:   makeAzureVMID(subA, "rg", name),
			Name: name,
			VMID: vmID,
		}
	}

	tests := []struct {
		name      string
		instances *AzureInstances
		existing  []types.Server
		wantNames []string
	}{
		{
			name: "removes VMs already enrolled in this subscription",
			instances: &AzureInstances{
				SubscriptionID: subA,
				Instances: []azure.DiscoveredVM{
					mkVM("vm1", "vmid-1"),
					mkVM("vm2", "vmid-2"),
					mkVM("vm3", "vmid-3"),
				},
			},
			existing: []types.Server{
				makeAzureNode(t, "node-1", subA, "vmid-1"),
				makeAzureNode(t, "node-3", subA, "vmid-3"),
			},
			wantNames: []string{"vm2"},
		},
		{
			// Same VMID in a different subscription must not dedup.
			name: "nodes from a different subscription do not match",
			instances: &AzureInstances{
				SubscriptionID: subA,
				Instances: []azure.DiscoveredVM{
					mkVM("vm1", "vmid-1"),
				},
			},
			existing:  []types.Server{makeAzureNode(t, "node-1", subB, "vmid-1")},
			wantNames: []string{"vm1"},
		},
		{
			// VM with empty VMID could match an existing node missing
			// the label, so verify that does not collapse to dedup.
			name: "nodes without VMID are ignored",
			instances: &AzureInstances{
				SubscriptionID: subA,
				Instances: []azure.DiscoveredVM{
					mkVM("vm1", "vmid-1"),
					{ID: makeAzureVMID(subA, "rg", "vm2"), Name: "vm2"},
				},
			},
			existing:  []types.Server{makeAzureNode(t, "node-1", subA, "")},
			wantNames: []string{"vm1", "vm2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.instances.FilterExistingNodes(tt.existing)
			names := make([]string, 0, len(tt.instances.Instances))
			for _, vm := range tt.instances.Instances {
				names = append(names, vm.Name)
			}
			assert.Equal(t, tt.wantNames, names)
		})
	}
}

func TestAzureWatcher_GetInstances_PassesARGParams(t *testing.T) {
	t.Parallel()

	rgMock := &azure.ARMResourceGraphMock{}
	clients := mockClients{rgClient: rgMock}

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher: types.AzureMatcher{
			Regions: []string{"eastus", "westus"},
		},
		Subscriptions:       []string{"sub-x", "sub-y"},
		ResourceGroups:      []string{"rg-x", "rg-y"},
		AzureClientGetter:   func(_ context.Context, _ string) (azure.Clients, error) { return &clients, nil },
		DiscoveryConfigName: "",
		Logger:              logtest.NewLogger(),
	})

	_, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)

	require.Equal(t, 1, rgMock.Calls(),
		"one fetcher per matcher must yield one ARG query, regardless of subscription or RG count")
	lastParams := rgMock.LastParams()
	assert.Equal(t, []string{"sub-x", "sub-y"}, lastParams.SubscriptionIDs)
	assert.Equal(t, []string{"eastus", "westus"}, lastParams.Regions)
	assert.Equal(t, []string{"rg-x", "rg-y"}, lastParams.ResourceGroups)
}

func TestAzureWatcher_GetInstances_LabelMatchClientSide(t *testing.T) {
	t.Parallel()

	const sub = "sub"
	rgMock := &azure.ARMResourceGraphMock{
		VMs: []azure.DiscoveredVM{
			{ID: makeAzureVMID(sub, "rg", "match"), SubscriptionID: sub, Name: "match", VMID: "match-vmid", Location: "eastus", ResourceGroup: "rg", Tags: map[string]string{"team": "platform"}},
			{ID: makeAzureVMID(sub, "rg", "miss"), SubscriptionID: sub, Name: "miss", VMID: "miss-vmid", Location: "eastus", ResourceGroup: "rg", Tags: map[string]string{"team": "other"}},
		},
	}

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher: types.AzureMatcher{
			Regions:      []string{"eastus"},
			ResourceTags: types.Labels{"team": []string{"platform"}},
		},
		Subscriptions:     []string{sub},
		ResourceGroups:    []string{"rg"},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) { return &mockClients{rgClient: rgMock}, nil },
		Logger:            logtest.NewLogger(),
	})

	groups, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Instances, 1)
	assert.Equal(t, "match", groups[0].Instances[0].Name)
}

func TestAzureWatcher_GetInstances_BucketsByRGAndRegion(t *testing.T) {
	t.Parallel()

	const subA = "sub-a"
	const subB = "sub-b"
	rgMock := &azure.ARMResourceGraphMock{
		VMs: []azure.DiscoveredVM{
			{ID: makeAzureVMID(subA, "rg-a", "vm-a1"), SubscriptionID: subA, Name: "vm-a1", VMID: "vm-a1-vmid", Location: "eastus", ResourceGroup: "rg-a"},
			{ID: makeAzureVMID(subA, "rg-a", "vm-a2"), SubscriptionID: subA, Name: "vm-a2", VMID: "vm-a2-vmid", Location: "westus", ResourceGroup: "rg-a"},
			{ID: makeAzureVMID(subB, "rg-b", "vm-b1"), SubscriptionID: subB, Name: "vm-b1", VMID: "vm-b1-vmid", Location: "eastus", ResourceGroup: "rg-b"},
		},
	}

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher: types.AzureMatcher{
			Regions:      []string{types.Wildcard},
			ResourceTags: types.Labels{"*": []string{"*"}},
		},
		Subscriptions:     []string{subA, subB},
		ResourceGroups:    []string{types.Wildcard},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) { return &mockClients{rgClient: rgMock}, nil },
		Logger:            logtest.NewLogger(),
	})

	groups, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)

	type bucket struct {
		sub, rg, region string
		count           int
	}
	got := make([]bucket, 0, len(groups))
	for _, g := range groups {
		got = append(got, bucket{sub: g.SubscriptionID, rg: g.ResourceGroup, region: g.Region, count: len(g.Instances)})
	}
	sort.Slice(got, func(i, j int) bool {
		if got[i].sub != got[j].sub {
			return got[i].sub < got[j].sub
		}
		if got[i].rg != got[j].rg {
			return got[i].rg < got[j].rg
		}
		return got[i].region < got[j].region
	})
	assert.Equal(t, []bucket{
		{sub: subA, rg: "rg-a", region: "eastus", count: 1},
		{sub: subA, rg: "rg-a", region: "westus", count: 1},
		{sub: subB, rg: "rg-b", region: "eastus", count: 1},
	}, got)
}

func TestAzureWatcher_GetInstances_EmptyDoesNotEmit(t *testing.T) {
	t.Parallel()

	rgMock := &azure.ARMResourceGraphMock{}
	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher:           types.AzureMatcher{Regions: []string{"eastus"}},
		Subscriptions:     []string{"sub"},
		ResourceGroups:    []string{"rg"},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) { return &mockClients{rgClient: rgMock}, nil },
		Logger:            logtest.NewLogger(),
	})
	groups, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestAzureWatcher_GetInstances_FallsBackToARMOnARGError(t *testing.T) {
	t.Parallel()

	const sub = "sub"
	vm := &armcompute.VirtualMachine{
		ID:       to.Ptr(makeAzureVMID(sub, "rg", "fallback")),
		Name:     to.Ptr("fallback"),
		Location: to.Ptr("eastus"),
		Tags:     map[string]*string{"team": to.Ptr("platform")},
		Properties: &armcompute.VirtualMachineProperties{
			VMID: to.Ptr("fallback-vmid"),
			StorageProfile: &armcompute.StorageProfile{
				OSDisk: &armcompute.OSDisk{OSType: to.Ptr(armcompute.OperatingSystemTypesLinux)},
			},
			InstanceView: &armcompute.VirtualMachineInstanceView{
				Statuses: []*armcompute.InstanceViewStatus{{Code: to.Ptr("PowerState/running")}},
			},
		},
	}
	rgMock := &azure.ARMResourceGraphMock{Err: errors.New("ARG access denied")}
	vmClient := azure.NewVirtualMachinesClientByAPI(&azure.ARMComputeMock{
		VirtualMachines: map[string][]*armcompute.VirtualMachine{"rg": {vm}},
	}, nil)

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher:        types.AzureMatcher{Regions: []string{"eastus"}, ResourceTags: types.Labels{"*": []string{"*"}}},
		Subscriptions:  []string{"sub"},
		ResourceGroups: []string{"rg"},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) {
			return &mockClients{rgClient: rgMock, vmClient: vmClient}, nil
		},
		Logger: logtest.NewLogger(),
	})
	groups, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Instances, 1)
	assert.Equal(t, "fallback", groups[0].Instances[0].Name)
	assert.Equal(t, "fallback-vmid", groups[0].Instances[0].VMID)
}

func TestAzureWatcher_GetInstances_DistinguishesARGFallbackFailureMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		clients   *mockClients
		wantStage string
		wantError string
	}{
		{
			name:      "resource graph client initialization failed",
			clients:   &mockClients{},
			wantStage: azureARGStageClientInit,
			wantError: "Azure Resource Graph client initialization failed",
		},
		{
			name: "resource graph query failed",
			clients: &mockClients{
				rgClient: &azure.ARMResourceGraphMock{Err: errors.New("ARG access denied")},
			},
			wantStage: azureARGStageQuery,
			wantError: "Azure Resource Graph VM query failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fetcher := newAzureInstanceFetcher(azureFetcherConfig{
				Matcher:        types.AzureMatcher{Regions: []string{"eastus"}},
				Subscriptions:  []string{"sub"},
				ResourceGroups: []string{"rg"},
				AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) {
					return tt.clients, nil
				},
				Logger: logtest.NewLogger(),
			})

			_, err := fetcher.GetInstances(t.Context(), false)
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantStage+": "+tt.wantError)
			require.ErrorContains(t, err, tt.wantError)
			require.ErrorContains(t, err, "ARM VM listing fallback failed")
		})
	}
}

func TestAzureWatcher_GetInstances_SkipsARMFallbackWhenContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher:        types.AzureMatcher{Regions: []string{"eastus"}},
		Subscriptions:  []string{"sub"},
		ResourceGroups: []string{"rg"},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) {
			return &mockClients{
				rgClient: &azure.ARMResourceGraphMock{Err: context.Canceled},
			}, nil
		},
		Logger: logtest.NewLogger(),
	})

	_, err := fetcher.GetInstances(ctx, false)
	require.ErrorIs(t, err, context.Canceled)
	require.NotContains(t, err.Error(), "ARM VM listing fallback failed",
		"context cancellation should return directly instead of attempting ARM fallback per scope")
}

func TestAzureWatcher_GetInstances_ARMFallbackIsBestEffortPerScope(t *testing.T) {
	t.Parallel()

	const (
		badSub  = "sub-bad"
		goodSub = "sub-good"
		rg      = "rg"
		region  = "eastus"
	)
	vm := &armcompute.VirtualMachine{
		ID:       to.Ptr(makeAzureVMID(goodSub, rg, "good-vm")),
		Name:     to.Ptr("good-vm"),
		Location: to.Ptr(region),
		Tags:     map[string]*string{"team": to.Ptr("platform")},
		Properties: &armcompute.VirtualMachineProperties{
			VMID: to.Ptr("good-vmid"),
		},
	}

	clients := &mockClients{
		rgClient: &azure.ARMResourceGraphMock{Err: errors.New("ARG failed")},
		vmClients: map[string]azure.VirtualMachinesClient{
			badSub: &testAzureVMClient{
				errsByResourceGroup: map[string]error{rg: trace.AccessDenied("denied")},
			},
			goodSub: &testAzureVMClient{
				vmsByResourceGroup: map[string][]*armcompute.VirtualMachine{rg: {vm}},
			},
		},
	}

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher:        types.AzureMatcher{Regions: []string{region}, ResourceTags: types.Labels{"*": []string{"*"}}},
		Subscriptions:  []string{badSub, goodSub},
		ResourceGroups: []string{rg},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) {
			return clients, nil
		},
		Logger: logtest.NewLogger(),
	})

	groups, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, goodSub, groups[0].SubscriptionID)
	require.Len(t, groups[0].Instances, 1)
	assert.Equal(t, "good-vm", groups[0].Instances[0].Name)
}

func TestAzureWatcher_GetInstances_ARMFallbackSummarizesIdenticalClientInitFailures(t *testing.T) {
	t.Parallel()

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher:        types.AzureMatcher{Regions: []string{"eastus"}},
		Subscriptions:  []string{"sub-1", "sub-2"},
		ResourceGroups: []string{"rg"},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) {
			return &mockClients{
				rgClient: &azure.ARMResourceGraphMock{Err: errors.New("ARG failed")},
			}, nil
		},
		Logger: logtest.NewLogger(),
	})

	_, err := fetcher.GetInstances(t.Context(), false)
	require.Error(t, err)
	require.ErrorContains(t, err, "ARM VM listing fallback failed")
	require.ErrorContains(t, err, "getting Azure VM clients failed for all 2 subscriptions")
	require.ErrorContains(t, err, `first subscription "sub-1"`)
	require.ErrorContains(t, err, "virtual machines client not configured")
	require.NotContains(t, err.Error(), `subscription "sub-2"`,
		"identical client initialization failures should be summarized instead of repeated once per subscription")
}

// TestAzureWatcher_GetInstances_FallbackMatchesMaster verifies the ARM fallback
// applies region and label filters only.
func TestAzureWatcher_GetInstances_FallbackMatchesMaster(t *testing.T) {
	t.Parallel()

	const sub = "sub"
	const rg = "rg1"
	const region = "eastus"

	mkARMVM := func(name string, osType armcompute.OperatingSystemTypes, powerStateCode string) *armcompute.VirtualMachine {
		return &armcompute.VirtualMachine{
			ID:       to.Ptr(makeAzureVMID(sub, rg, name)),
			Name:     to.Ptr(name),
			Location: to.Ptr(region),
			Properties: &armcompute.VirtualMachineProperties{
				VMID: to.Ptr(name + "-vmid"),
				StorageProfile: &armcompute.StorageProfile{
					OSDisk: &armcompute.OSDisk{OSType: to.Ptr(osType)},
				},
				InstanceView: &armcompute.VirtualMachineInstanceView{
					Statuses: []*armcompute.InstanceViewStatus{{Code: to.Ptr(powerStateCode)}},
				},
			},
		}
	}

	armVMs := []*armcompute.VirtualMachine{
		mkARMVM("vm-linux-running", armcompute.OperatingSystemTypesLinux, "PowerState/running"),
		mkARMVM("vm-windows", armcompute.OperatingSystemTypesWindows, "PowerState/running"),
		mkARMVM("vm-deallocated", armcompute.OperatingSystemTypesLinux, "PowerState/deallocated"),
	}

	clients := &mockClients{
		rgClient: &azure.ARMResourceGraphMock{Err: errors.New("ARG failed")},
		vmClient: azure.NewVirtualMachinesClientByAPI(&azure.ARMComputeMock{
			VirtualMachines: map[string][]*armcompute.VirtualMachine{rg: armVMs},
		}, nil),
	}

	fetcher := newAzureInstanceFetcher(azureFetcherConfig{
		Matcher:        types.AzureMatcher{Regions: []string{region}, ResourceTags: types.Labels{"*": []string{"*"}}},
		Subscriptions:  []string{sub},
		ResourceGroups: []string{rg},
		AzureClientGetter: func(_ context.Context, _ string) (azure.Clients, error) {
			return clients, nil
		},
		Logger: logtest.NewLogger(),
	})

	groups, err := fetcher.GetInstances(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, groups, 1)

	gotNames := make([]string, 0, len(groups[0].Instances))
	for _, vm := range groups[0].Instances {
		gotNames = append(gotNames, vm.Name)
	}
	assert.ElementsMatch(t,
		[]string{"vm-linux-running", "vm-windows", "vm-deallocated"},
		gotNames,
		"fallback must match master: no OS or power-state filtering applied",
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

	vm := azure.DiscoveredVM{
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
			name: "zero-value instance",
			result: AzureInstallResult{
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
			name: "instance without VMID",
			result: AzureInstallResult{
				Instance: azure.DiscoveredVM{
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
			t.Parallel()
			instances := &AzureInstances{
				SubscriptionID: subscriptionID,
				ResourceGroup:  resourceGroup,
				Region:         region,
			}
			evt := instances.MakeRunEvent(tc.result)
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

	vm := azure.DiscoveredVM{ID: resourceID}

	tests := []struct {
		name      string
		instances *AzureInstances
		wantKey   string
		want      *usageeventsv1.ResourceCreateEvent
	}{
		{
			name: "node",
			instances: &AzureInstances{
				DiscoveryConfigName: discoveryConfig,
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
				DiscoveryConfigName: discoveryConfig,
				InstallerParams: &types.InstallerParams{
					ScriptName: installers.InstallerScriptNameAgentless,
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
			t.Parallel()
			key, evt := tc.instances.MakeUsageEvent(vm)
			require.Equal(t, tc.wantKey, key)
			require.Equal(t, tc.want, evt)
		})
	}
}
