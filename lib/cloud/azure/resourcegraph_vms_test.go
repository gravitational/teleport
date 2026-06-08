/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package azure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestQueryVirtualMachinesSanitize(t *testing.T) {
	t.Parallel()

	uuid1 := uuid.NewString()
	uuid2 := strings.ToUpper(uuid.NewString())

	for _, tc := range []struct {
		name     string
		params   QueryLinuxVMsParams
		expected *validatedQueryLinuxVMsParams
		errCheck require.ErrorAssertionFunc
	}{
		{
			name: "valid parameters: no change",
			params: QueryLinuxVMsParams{
				SubscriptionID: uuid1,
				ResourceGroup:  "rg1",
				Locations:      []string{"westus2", "westeurope"},
			},
			expected: &validatedQueryLinuxVMsParams{
				SubscriptionID: uuid1,
				ResourceGroup:  "rg1",
				Locations:      []string{"westus2", "westeurope"},
			},
			errCheck: require.NoError,
		},
		{
			name: "valid parameters: lowercase",
			params: QueryLinuxVMsParams{
				SubscriptionID: uuid1,
				ResourceGroup:  "RG1",
				Locations:      []string{"WestUS2", "westeurope"},
			},
			expected: &validatedQueryLinuxVMsParams{
				SubscriptionID: uuid1,
				ResourceGroup:  "rg1",
				Locations:      []string{"westus2", "westeurope"},
			},
			errCheck: require.NoError,
		},
		{
			name: "valid parameters: subscription ID in all caps is valid",
			params: QueryLinuxVMsParams{
				SubscriptionID: uuid2,
			},
			expected: &validatedQueryLinuxVMsParams{
				SubscriptionID: uuid2,
			},
			errCheck: require.NoError,
		},
		{
			name:     "invalid parameters: empty subscription ID",
			params:   QueryLinuxVMsParams{},
			errCheck: require.Error,
		},
		{
			name: "invalid parameters: malformed subscription ID",
			params: QueryLinuxVMsParams{
				SubscriptionID: "not-a-uuid",
			},
			errCheck: require.Error,
		},
		{
			name: "invalid resource group",
			params: QueryLinuxVMsParams{
				SubscriptionID: uuid1,
				ResourceGroup:  "invalid resource group",
			},
			errCheck: require.Error,
		},
		{
			name: "invalid location",
			params: QueryLinuxVMsParams{
				SubscriptionID: uuid1,
				Locations:      []string{"US East 1"},
			},
			errCheck: require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.params.sanitize()
			tc.errCheck(t, err)
			if tc.expected != nil {
				require.Equal(t, tc.expected.SubscriptionID, result.SubscriptionID)
				require.Equal(t, tc.expected.ResourceGroup, result.ResourceGroup)
				require.ElementsMatch(t, tc.expected.Locations, result.Locations)
			} else {
				require.Nil(t, result)
			}
		})
	}
}

func TestKQLQueryLinuxVMs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		params   *validatedQueryLinuxVMsParams
		expected string
	}{
		{
			name:   "no filters",
			params: &validatedQueryLinuxVMsParams{},
			expected: `
(
  resources |
  where type == 'microsoft.compute/virtualmachines'
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    // no filter by location
    // no filter by resourceGroup
  | project id, name, location, tags, vmId = properties.vmId
) | union (
  computeresources |
  where type == "microsoft.compute/virtualmachinescalesets/virtualmachines"
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    // no filter by location
    // no filter by resourceGroup
  | project id, name, location, tags, vmId = properties.vmId
)
| order by id desc`,
		},
		{
			name: "filter by 1 location and 1 resource group",
			params: &validatedQueryLinuxVMsParams{
				Locations:     []string{"westus"},
				ResourceGroup: "rg1",
			},
			expected: `
(
  resources |
  where type == 'microsoft.compute/virtualmachines'
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    and location in ('westus')
    and resourceGroup == 'rg1'
  | project id, name, location, tags, vmId = properties.vmId
) | union (
  computeresources |
  where type == "microsoft.compute/virtualmachinescalesets/virtualmachines"
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    and location in ('westus')
    and resourceGroup == 'rg1'
  | project id, name, location, tags, vmId = properties.vmId
)
| order by id desc`,
		},
		{
			name: "filter by multiple locations and resource groups",
			params: &validatedQueryLinuxVMsParams{
				Locations:     []string{"westus", "eastus"},
				ResourceGroup: "rg1",
			},
			expected: `
(
  resources |
  where type == 'microsoft.compute/virtualmachines'
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    and location in ('westus', 'eastus')
    and resourceGroup == 'rg1'
  | project id, name, location, tags, vmId = properties.vmId
) | union (
  computeresources |
  where type == "microsoft.compute/virtualmachinescalesets/virtualmachines"
    and properties.extended.instanceView.powerState.code == 'PowerState/running'
    and properties.storageProfile.osDisk.osType == 'Linux'
    and location in ('westus', 'eastus')
    and resourceGroup == 'rg1'
  | project id, name, location, tags, vmId = properties.vmId
)
| order by id desc`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := kqlQueryLinuxVMs(tc.params)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestParseVirtualMachines(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	makeRow := func(name string) map[string]any {
		return map[string]any{
			"id":       "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/" + name,
			"name":     name,
			"location": "eastus",
			"tags":     map[string]any{},
			"vmId":     name + "-vmid",
		}
	}
	makeRowScaleSetVM := func(name string, idx string) map[string]any {
		return map[string]any{
			"id":       "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/" + name + "/virtualMachines/" + idx,
			"name":     name + "_" + idx,
			"location": "eastus",
			"tags":     map[string]any{},
			"vmId":     name + "-vmid",
		}
	}

	for _, tc := range []struct {
		name     string
		data     []any
		errCheck require.ErrorAssertionFunc
		verify   func(t *testing.T, got []*VirtualMachine)
	}{
		{
			name:     "nil data returns nil slice without error",
			data:     nil,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Nil(t, got)
			},
		},
		{
			name:     "empty data returns empty slice without error",
			data:     []any{},
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.NotNil(t, got)
				require.Empty(t, got)
			},
		},
		{
			name: "all good rows are parsed and returned in order",
			data: []any{
				makeRow("vm1"),
				makeRow("vm2"),
			},
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Len(t, got, 2)
				require.Equal(t, "vm1", got[0].Name)
				require.Equal(t, "vm2", got[1].Name)
			},
		},
		{
			name: "happy path projects all fields including tags",
			data: []any{
				func() map[string]any {
					row := makeRow("vm1")
					row["tags"] = map[string]any{"env": "prod", "owner": "alice"}
					return row
				}(),
			},
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Equal(t, []*VirtualMachine{{
					ID:            "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1",
					Subscription:  "00000000-0000-0000-0000-000000000000",
					Name:          "vm1",
					VMID:          "vm1-vmid",
					Location:      "eastus",
					ResourceGroup: "rg",
					Tags:          map[string]string{"env": "prod", "owner": "alice"},
				}}, got)
			},
		},
		{
			name: "non-map row is skipped, good sibling is kept",
			data: []any{
				makeRow("good-vm"),
				"not a map",
			},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Len(t, got, 1)
				require.Equal(t, "good-vm", got[0].Name)
			},
		},
		{
			name: "row missing required field is skipped, good sibling is kept",
			data: []any{
				makeRow("good-vm"),
				func() map[string]any {
					row := makeRow("bad-vm")
					delete(row, "id")
					return row
				}(),
			},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Len(t, got, 1)
				require.Equal(t, "good-vm", got[0].Name)
			},
		},
		{
			name: "all rows malformed returns error and empty slice",
			data: []any{
				"not a map",
				func() map[string]any {
					row := makeRow("bad-vm")
					delete(row, "id")
					return row
				}(),
			},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Empty(t, got)
			},
		},
		{
			name:     "single malformed row returns error and empty slice",
			data:     []any{123},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Empty(t, got)
			},
		},
		{
			name: "row missing required id field returns error and empty slice",
			data: []any{
				func() map[string]any {
					row := makeRow("bad-vm")
					delete(row, "id")
					return row
				}(),
			},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Empty(t, got)
			},
		},
		{
			name: "row missing required location field returns error and empty slice",
			data: []any{
				func() map[string]any {
					row := makeRow("bad-vm")
					delete(row, "location")
					return row
				}(),
			},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.Empty(t, got)
			},
		},
		{
			name: "uniform vmss: two vms",
			data: []any{
				makeRowScaleSetVM("vmss1", "0"),
				makeRowScaleSetVM("vmss1", "1"),
			},
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine) {
				require.ElementsMatch(t, []*VirtualMachine{
					{
						ID:                          "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss1/virtualMachines/0",
						Subscription:                "00000000-0000-0000-0000-000000000000",
						Name:                        "vmss1_0",
						VMID:                        "vmss1-vmid",
						Location:                    "eastus",
						ResourceGroup:               "rg",
						UniformScaleSetName:         "vmss1",
						UniformScaleSetVMInstanceID: "0",
						Tags:                        map[string]string{},
					},
					{
						ID:                          "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss1/virtualMachines/1",
						Subscription:                "00000000-0000-0000-0000-000000000000",
						Name:                        "vmss1_1",
						VMID:                        "vmss1-vmid",
						Location:                    "eastus",
						ResourceGroup:               "rg",
						UniformScaleSetName:         "vmss1",
						UniformScaleSetVMInstanceID: "1",
						Tags:                        map[string]string{},
					},
				}, got)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseVirtualMachines(t.Context(), logger, tc.data)
			tc.errCheck(t, got.lastParseError)
			tc.verify(t, got.vms)
		})
	}
}

// fakeARGAPI is an in-memory implementation of argResourcesAPI that lets each
// test pin the exact response sequence (or error) for the Resources call.
type fakeARGAPI struct {
	respond  func(callIdx int) (armresourcegraph.ClientResourcesResponse, error)
	calls    int
	requests []armresourcegraph.QueryRequest
}

func (f *fakeARGAPI) Resources(_ context.Context, req armresourcegraph.QueryRequest, _ *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	f.requests = append(f.requests, req)
	resp, err := f.respond(f.calls)
	f.calls++
	return resp, err
}

// argPage builds a ClientResourcesResponse from a small set of fields callers care about.
func argPage(data any, skipToken *string, truncated *armresourcegraph.ResultTruncated) armresourcegraph.ClientResourcesResponse {
	return armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			Data:            data,
			SkipToken:       skipToken,
			ResultTruncated: truncated,
		},
	}
}

// staticPages returns a responder that yields the given responses in order; calling past
// the end is a test bug, surfaced via the wrapping subtest's require.Less assertion.
func staticPages(t *testing.T, pages ...armresourcegraph.ClientResourcesResponse) func(int) (armresourcegraph.ClientResourcesResponse, error) {
	return func(idx int) (armresourcegraph.ClientResourcesResponse, error) {
		require.Less(t, idx, len(pages), "fakeARGAPI called more times than configured")
		return pages[idx], nil
	}
}

// staticError returns a responder that always returns the same error.
func staticError(err error) func(int) (armresourcegraph.ClientResourcesResponse, error) {
	return func(int) (armresourcegraph.ClientResourcesResponse, error) {
		return armresourcegraph.ClientResourcesResponse{}, err
	}
}

func TestQueryLinuxVMs(t *testing.T) {
	t.Parallel()

	subID := uuid.NewString()
	validParams := QueryLinuxVMsParams{SubscriptionID: subID}

	goodVMRow := func(name string) map[string]any {
		return map[string]any{
			"id":             "/subscriptions/" + subID + "/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/" + name,
			"subscriptionId": subID,
			"name":           name,
			"vmId":           name + "-vmid",
			"location":       "eastus",
			"resourceGroup":  "rg",
		}
	}

	truncated := to.Ptr(armresourcegraph.ResultTruncatedTrue)

	for _, tc := range []struct {
		name     string
		newAPI   func(t *testing.T) *fakeARGAPI
		params   QueryLinuxVMsParams
		errCheck require.ErrorAssertionFunc
		verify   func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, err error)
	}{
		{
			name: "invalid subscription ID is rejected before any API call",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{
					respond: func(int) (armresourcegraph.ClientResourcesResponse, error) {
						t.Fatal("Resources must not be called when sanitize fails")
						return armresourcegraph.ClientResourcesResponse{}, nil
					},
				}
			},
			params:   QueryLinuxVMsParams{SubscriptionID: "not-a-uuid"},
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, _ error) {
				require.Nil(t, got)
				require.Equal(t, 0, api.calls)
			},
		},
		{
			name: "generic SDK error is propagated",
			newAPI: func(*testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticError(errors.New("boom"))}
			},
			params:   validParams,
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, _ error) {
				require.Nil(t, got)
				require.Equal(t, 1, api.calls)
			},
		},
		{
			name: "403 ResponseError surfaces as AccessDenied",
			newAPI: func(*testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticError(&azcore.ResponseError{StatusCode: http.StatusForbidden})}
			},
			params:   validParams,
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, err error) {
				require.Nil(t, got)
				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %T", err)
			},
		},
		{
			name: "429 ResponseError surfaces as LimitExceeded",
			newAPI: func(*testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticError(&azcore.ResponseError{StatusCode: http.StatusTooManyRequests})}
			},
			params:   validParams,
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, err error) {
				require.Nil(t, got)
				require.True(t, trace.IsLimitExceeded(err), "expected LimitExceeded, got %T", err)
			},
		},
		{
			name: "nil Data field returns 0 VMs",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t, armresourcegraph.ClientResourcesResponse{})}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, err error) {
				require.Empty(t, got)
			},
		},
		{
			name: "wrong-type Data field returns BadParameter",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t, argPage("not a slice", nil, nil))}
			},
			params:   validParams,
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, err error) {
				require.Nil(t, got)
				require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %T", err)
			},
		},
		{
			name: "only one-malformed row does not surface an error",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t, argPage([]any{"not a map", goodVMRow("vm1")}, nil, nil))}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, _ error) {
				require.Len(t, got, 1)
				require.Equal(t, "vm1", got[0].Name)
			},
		},
		{
			name: "all-malformed rows in a single page surface parseVirtualMachines error",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t, argPage([]any{"not a map", 42}, nil, nil))}
			},
			params:   validParams,
			errCheck: require.Error,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, _ error) {
				require.Nil(t, got)
			},
		},
		{
			name: "single page returns parsed VMs and forwards request fields",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t,
					argPage([]any{goodVMRow("vm1"), goodVMRow("vm2")}, nil, nil),
				)}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, _ error) {
				require.Len(t, got, 2)
				require.Equal(t, "vm1", got[0].Name)
				require.Equal(t, "vm2", got[1].Name)
				require.Equal(t, 1, api.calls)

				require.Len(t, api.requests, 1)
				req := api.requests[0]
				require.NotNil(t, req.Query)
				require.Len(t, req.Subscriptions, 1)
				require.NotNil(t, req.Subscriptions[0])
				require.Equal(t, subID, *req.Subscriptions[0])
				require.NotNil(t, req.Options)
				require.NotNil(t, req.Options.ResultFormat)
				require.Equal(t, armresourcegraph.ResultFormatObjectArray, *req.Options.ResultFormat)
				require.NotNil(t, req.Options.Top)
				require.Equal(t, resourceGraphPageSize, int(*req.Options.Top))
				require.Nil(t, req.Options.SkipToken, "first page must start with nil SkipToken")
			},
		},
		{
			name: "paginates across SkipToken and forwards the token on the follow-up request",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t,
					argPage([]any{goodVMRow("vm1")}, to.Ptr("page-2-token"), nil),
					argPage([]any{goodVMRow("vm2")}, nil, nil),
				)}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, _ error) {
				require.Len(t, got, 2)
				require.Equal(t, "vm1", got[0].Name)
				require.Equal(t, "vm2", got[1].Name)
				require.Equal(t, 2, api.calls)
				require.NotNil(t, api.requests[1].Options.SkipToken)
				require.Equal(t, "page-2-token", *api.requests[1].Options.SkipToken)
			},
		},
		{
			name: "truncated result without skip token returns current VMs",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t,
					argPage([]any{goodVMRow("vm1")}, nil, truncated),
				)}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, _ *fakeARGAPI, err error) {
				require.Len(t, got, 1)
				require.Equal(t, "vm1", got[0].Name)
			},
		},
		{
			name: "exhausting the page cap returns the current VMs",
			newAPI: func(*testing.T) *fakeARGAPI {
				return &fakeARGAPI{
					respond: func(idx int) (armresourcegraph.ClientResourcesResponse, error) {
						if idx >= resourceGraphMaxPages {
							return armresourcegraph.ClientResourcesResponse{}, fmt.Errorf("unexpected call past page cap at idx %d", idx)
						}
						if idx == resourceGraphMaxPages-1 {
							return argPage([]any{}, nil, truncated), nil
						}
						return argPage([]any{}, to.Ptr(fmt.Sprintf("page-%d", idx+1)), truncated), nil
					},
				}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, err error) {
				require.Empty(t, got)
				require.Equal(t, resourceGraphMaxPages, api.calls)
				require.NoError(t, err)
			},
		},
		{
			name: "empty page returns no VMs without error",
			newAPI: func(t *testing.T) *fakeARGAPI {
				return &fakeARGAPI{respond: staticPages(t, argPage([]any{}, nil, nil))}
			},
			params:   validParams,
			errCheck: require.NoError,
			verify: func(t *testing.T, got []*VirtualMachine, api *fakeARGAPI, _ error) {
				require.Empty(t, got)
				require.Equal(t, 1, api.calls)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := tc.newAPI(t)
			c := &resourceGraphClient{
				logger:       slog.Default(),
				resourcesAPI: api,
			}
			got, err := c.QueryLinuxVMs(t.Context(), tc.params)
			tc.errCheck(t, err)
			tc.verify(t, got, api, err)
		})
	}
}
