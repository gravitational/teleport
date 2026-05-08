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

package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestBuildVMDiscoveryKQL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		regions         []string
		resourceGroups  []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:           "no filters",
			regions:        nil,
			resourceGroups: nil,
			wantContains: []string{
				"Resources",
				"Microsoft.Compute/virtualMachines",
				"osType) =~ 'Linux'",
				"powerState.code) =~ 'PowerState/running'",
				"| project id, name, subscriptionId, resourceGroup",
			},
			wantNotContains: []string{
				"| where location in~",
				"| where resourceGroup in~",
			},
		},
		{
			name:           "wildcard region and rg",
			regions:        []string{types.Wildcard},
			resourceGroups: []string{types.Wildcard},
			wantNotContains: []string{
				"| where location in~",
				"| where resourceGroup in~",
			},
		},
		{
			name:           "wildcard mixed with concrete filters matches all",
			regions:        []string{types.Wildcard, "eastus"},
			resourceGroups: []string{types.Wildcard, "rg1"},
			wantNotContains: []string{
				"| where location in~",
				"| where resourceGroup in~",
			},
		},
		{
			name:    "single region uses case-insensitive set membership",
			regions: []string{"eastus"},
			wantContains: []string{
				"| where location in~ ('eastus')",
			},
		},
		{
			name:           "single resource group uses case-insensitive set membership",
			resourceGroups: []string{"discover-rg"},
			wantContains: []string{
				"| where resourceGroup in~ ('discover-rg')",
			},
		},
		{
			name:           "single quote in resource group is escaped",
			resourceGroups: []string{"rg'name"},
			wantContains: []string{
				"| where resourceGroup in~ ('rg''name')",
			},
		},
		{
			name:    "single quote in region is escaped",
			regions: []string{"east'us"},
			wantContains: []string{
				"| where location in~ ('east''us')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildVMDiscoveryKQL(tt.regions, tt.resourceGroups)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "missing expected fragment %q in:\n%s", want, got)
			}
			for _, unwanted := range tt.wantNotContains {
				assert.NotContains(t, got, unwanted, "unexpected fragment %q in:\n%s", unwanted, got)
			}
		})
	}
}

func TestQueryVMsParamsFieldsStayInSyncWithARGMock(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(QueryVMsParams{})
	fields := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		fields = append(fields, typ.Field(i).Name)
	}

	assert.ElementsMatch(t, []string{"SubscriptionIDs", "Regions", "ResourceGroups"}, fields,
		"QueryVMsParams fields define the caller-controllable ARG filters. "+
			"If a field is added, update buildVMDiscoveryKQL and mockARGServerFilter together, "+
			"or document why the new parameter is not part of ARG filtering.")
}

func TestEscapeKQL(t *testing.T) {
	t.Parallel()
	assert.Empty(t, escapeKQL(""))
	assert.Equal(t, "plain", escapeKQL("plain"))
	assert.Equal(t, "it''s", escapeKQL("it's"))
	assert.Equal(t, "''both''", escapeKQL("'both'"))
}

func makeARGVMRow(id, name string) map[string]any {
	return map[string]any{
		"id":             id,
		"subscriptionId": "sub",
		"name":           name,
		"vmId":           name + "-vmid",
		"location":       "eastus",
		"resourceGroup":  "rg",
	}
}

func TestParseDiscoveredVMs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		data    any
		wantErr bool
		verify  func(t *testing.T, got []DiscoveredVM)
	}{
		{
			name: "nil data returns nil slice",
			data: nil,
			verify: func(t *testing.T, got []DiscoveredVM) {
				assert.Nil(t, got)
			},
		},
		{
			name:    "unexpected top-level type returns error",
			data:    "not a slice",
			wantErr: true,
		},
		{
			name:    "unexpected row type returns error",
			data:    []any{"not a map"},
			wantErr: true,
		},
		{
			name: "happy path projects all fields",
			data: []any{map[string]any{
				"id":             "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1",
				"subscriptionId": "sub",
				"name":           "vm1",
				"vmId":           "abc-123",
				"location":       "eastus",
				"resourceGroup":  "rg",
				"tags": map[string]any{
					"env":   "prod",
					"owner": "alice",
				},
			}},
			verify: func(t *testing.T, got []DiscoveredVM) {
				want := []DiscoveredVM{{
					ID:             "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1",
					SubscriptionID: "sub",
					Name:           "vm1",
					VMID:           "abc-123",
					Location:       "eastus",
					ResourceGroup:  "rg",
					Tags:           map[string]string{"env": "prod", "owner": "alice"},
				}}
				assert.Empty(t, cmp.Diff(want, got))
			},
		},
		{
			// Empty required fields are row contract drift.
			name: "empty id field returns error",
			data: []any{map[string]any{
				"id":             "",
				"subscriptionId": "sub",
				"name":           "drop-empty-id",
				"vmId":           "vm-1",
				"location":       "eastus",
				"resourceGroup":  "rg",
			}},
			wantErr: true,
		},
		{
			// Missing required fields are row contract drift.
			name: "missing id field returns error",
			data: []any{map[string]any{
				"subscriptionId": "sub",
				"name":           "drop-missing-id",
				"vmId":           "vm-2",
				"location":       "eastus",
				"resourceGroup":  "rg",
			}},
			wantErr: true,
		},
		{
			// Nil required fields are treated like missing required fields.
			name: "nil required string field returns error",
			data: []any{map[string]any{
				"id":             "/subscriptions/sub/.../vm",
				"subscriptionId": "sub",
				"name":           nil,
				"vmId":           "vm-vmid",
				"location":       "westeurope",
				"resourceGroup":  "rg",
			}},
			wantErr: true,
		},
		{
			name: "missing routing field returns error",
			data: []any{map[string]any{
				"id":            "/subscriptions/sub/.../vm",
				"name":          "vm",
				"vmId":          "vm-vmid",
				"location":      "eastus",
				"resourceGroup": "rg",
			}},
			wantErr: true,
		},
		{
			// Type drift in required fields must surface as an error.
			name: "non-string scalar field returns error",
			data: []any{map[string]any{
				"id":             "/subscriptions/sub/.../vm",
				"subscriptionId": "sub",
				"name":           "vm",
				"vmId":           42, // wrong type
				"location":       "westeurope",
				"resourceGroup":  "rg",
			}},
			wantErr: true,
		},
		{
			name: "non-string tag values stringified, nil dropped",
			data: []any{func() map[string]any {
				row := makeARGVMRow("/subscriptions/sub/.../vm", "vm")
				row["tags"] = map[string]any{
					"strKey": "value",
					"intKey": 42,
					"nilKey": nil,
				}
				return row
			}()},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "value", got[0].Tags["strKey"])
				assert.Equal(t, "42", got[0].Tags["intKey"])
				_, hasNil := got[0].Tags["nilKey"]
				assert.False(t, hasNil, "nil-valued tags are dropped; see utils.Fields.GetStringMap")
			},
		},
		{
			// Type drift in tags must surface as an error.
			name: "tags wrong type returns error",
			data: []any{map[string]any{
				"id":             "/subscriptions/sub/.../vm",
				"subscriptionId": "sub",
				"name":           "vm",
				"vmId":           "vm-vmid",
				"location":       "eastus",
				"resourceGroup":  "rg",
				"tags":           "not a map",
			}},
			wantErr: true,
		},
		{
			// Missing/nil tags must still produce a non-nil empty map.
			name: "missing tags yields non-nil empty map",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../no-tags", "no-tags"),
				func() map[string]any {
					row := makeARGVMRow("/subscriptions/sub/.../nil-tags", "nil-tags")
					row["tags"] = nil
					return row
				}(),
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 2)
				for _, vm := range got {
					assert.NotNil(t, vm.Tags, "tags must be non-nil for %q", vm.ID)
					assert.Empty(t, vm.Tags)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDiscoveredVMs(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, got)
			}
		})
	}
}

// fakeARGAPI records Resource Graph calls and replays pre-built pages.
type fakeARGAPI struct {
	pages       []argPage
	calls       int
	gotRequests []armresourcegraph.QueryRequest
	err         error
}

type argPage struct {
	data            any
	skipToken       *string
	count           *int64
	totalRecords    *int64
	resultTruncated *armresourcegraph.ResultTruncated
}

func makeRunawayARGPages(n int) []argPage {
	pages := make([]argPage, n)
	for i := range n {
		pages[i] = argPage{
			data:            []any{},
			count:           to.Ptr[int64](0),
			totalRecords:    to.Ptr(int64(n)),
			resultTruncated: to.Ptr(armresourcegraph.ResultTruncatedTrue),
			skipToken:       to.Ptr(fmt.Sprintf("page-%d", i+1)),
		}
	}
	return pages
}

func (f *fakeARGAPI) Resources(_ context.Context, query armresourcegraph.QueryRequest, _ *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error) {
	f.calls++
	f.gotRequests = append(f.gotRequests, query)
	if f.err != nil {
		return armresourcegraph.ClientResourcesResponse{}, f.err
	}
	if f.calls > len(f.pages) {
		return armresourcegraph.ClientResourcesResponse{}, errors.New("fakeARGAPI: more pages requested than configured")
	}
	page := f.pages[f.calls-1]
	return armresourcegraph.ClientResourcesResponse{
		QueryResponse: armresourcegraph.QueryResponse{
			Count:           page.count,
			Data:            page.data,
			ResultTruncated: page.resultTruncated,
			TotalRecords:    page.totalRecords,
			SkipToken:       page.skipToken,
		},
	}, nil
}

func TestQueryVMs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		api    *fakeARGAPI
		params QueryVMsParams
		verify func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error)
	}{
		{
			name:   "missing subscriptions is rejected",
			api:    &fakeARGAPI{},
			params: QueryVMsParams{},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				assert.Error(t, err)
				assert.True(t, trace.IsBadParameter(err),
					"empty subscription list must be BadParameter, got %T: %v", err, err)
				assert.Equal(t, 0, api.calls,
					"validation must run before any ARG round trip")
			},
		},
		{
			name: "single page returns parsed VMs and forwards multi-sub / multi-rg params",
			api: &fakeARGAPI{
				pages: []argPage{{
					data: []any{
						makeARGVMRow("/sub/.../vm1", "vm1"),
						makeARGVMRow("/sub/.../vm2", "vm2"),
					},
				}},
			},
			params: QueryVMsParams{
				SubscriptionIDs: []string{"sub-1", "sub-2"},
				Regions:         []string{"eastus"},
				ResourceGroups:  []string{"rg-1", "rg-2"},
			},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.NoError(t, err)
				require.Len(t, got, 2)
				assert.Equal(t, 1, api.calls)
				req := api.gotRequests[0]
				require.NotNil(t, req.Query)
				assert.Contains(t, *req.Query, "| where location in~ ('eastus')")
				assert.Contains(t, *req.Query, "| where resourceGroup in~ ('rg-1', 'rg-2')")
				require.Len(t, req.Subscriptions, 2)
				require.NotNil(t, req.Subscriptions[0])
				assert.Equal(t, "sub-1", *req.Subscriptions[0])
				assert.Equal(t, "sub-2", *req.Subscriptions[1])
				require.NotNil(t, req.Options)
				require.NotNil(t, req.Options.ResultFormat)
				assert.Equal(t, armresourcegraph.ResultFormatObjectArray, *req.Options.ResultFormat)
				require.NotNil(t, req.Options.Top)
				assert.Equal(t, int32(1000), *req.Options.Top)
				assert.Nil(t, req.Options.SkipToken)
			},
		},
		{
			name: "paginates across SkipToken",
			api: &fakeARGAPI{
				pages: []argPage{
					{
						data: []any{
							makeARGVMRow("/sub/.../vm1", "vm1"),
						},
						skipToken: to.Ptr("page-2-token"),
					},
					{
						data: []any{
							makeARGVMRow("/sub/.../vm2", "vm2"),
						},
						skipToken: to.Ptr(""), // empty string also terminates
					},
				},
			},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.NoError(t, err)
				require.Len(t, got, 2)
				assert.Equal(t, 2, api.calls)
				require.NotNil(t, api.gotRequests[1].Options)
				require.NotNil(t, api.gotRequests[1].Options.SkipToken)
				assert.Equal(t, "page-2-token", *api.gotRequests[1].Options.SkipToken)
			},
		},
		{
			name:   "propagates SDK errors",
			api:    &fakeARGAPI{err: errors.New("boom")},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.True(t, strings.Contains(err.Error(), "boom"), "expected wrapped error, got %v", err)
			},
		},
		{
			name: "403 surfaces AccessDenied with remediation message",
			// Build an *azcore.ResponseError so ConvertResponseError maps it to AccessDenied.
			api:    &fakeARGAPI{err: &azcore.ResponseError{StatusCode: http.StatusForbidden}},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %T: %v", err, err)
				// Surface the RBAC action commonly missing when ARG returns 403.
				assert.Contains(t, err.Error(), "Microsoft.Compute/virtualMachines/read",
					"error must name the missing permission so operators know which RBAC action to grant")
			},
		},
		{
			// Duplicate SkipToken values must abort pagination.
			name: "non-advancing SkipToken aborts pagination",
			api: &fakeARGAPI{
				pages: []argPage{
					{
						data:      []any{makeARGVMRow("/sub/.../vm1", "vm1")},
						skipToken: to.Ptr("stuck-token"),
					},
					{
						data:            []any{makeARGVMRow("/sub/.../vm2", "vm2")},
						count:           to.Ptr[int64](1),
						totalRecords:    to.Ptr[int64](2),
						resultTruncated: to.Ptr(armresourcegraph.ResultTruncatedFalse),
						skipToken:       to.Ptr("stuck-token"), // same as above
					},
				},
			},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "non-advancing SkipToken",
					"error must name the non-advancing-token failure mode "+
						"so operators can distinguish it from a generic ARG error")
				assert.Contains(t, err.Error(), `skip_token="stuck-token"`)
				assert.Contains(t, err.Error(), "count=1")
				assert.Contains(t, err.Error(), "total_records=2")
				assert.Contains(t, err.Error(), "result_truncated=false")
				assert.Equal(t, 2, api.calls,
					"second call must run before the guard trips; "+
						"the guard fires after, not before, the duplicate token")
			},
		},
		{
			// Runaway pagination must hit the explicit page cap.
			name: "pagination safety cap aborts runaway paging",
			api: &fakeARGAPI{
				pages: makeRunawayARGPages(argMaxPagesPerChunk),
			},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "pagination exceeded")
				assert.Contains(t, err.Error(), fmt.Sprintf("%d-page safety cap", argMaxPagesPerChunk),
					"error must include the configured cap so operators can identify this boundary")
				assert.Contains(t, err.Error(), fmt.Sprintf(`skip_token="page-%d"`, argMaxPagesPerChunk))
				assert.Contains(t, err.Error(), "count=0")
				assert.Contains(t, err.Error(), fmt.Sprintf("total_records=%d", argMaxPagesPerChunk))
				assert.Contains(t, err.Error(), "result_truncated=true")
				assert.Equal(t, argMaxPagesPerChunk, api.calls,
					"the safety cap should allow exactly argMaxPagesPerChunk page attempts before aborting")
			},
		},
		{
			name: "subscription list at argMaxSubscriptionsPerQuery uses one chunk",
			api: &fakeARGAPI{
				pages: []argPage{{data: []any{makeARGVMRow("/sub/.../chunk-vm", "chunk-vm")}}},
			},
			params: QueryVMsParams{
				SubscriptionIDs: makeSubscriptionIDs(argMaxSubscriptionsPerQuery),
			},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.NoError(t, err)
				require.Len(t, got, 1)
				assert.Equal(t, 1, api.calls,
					"exactly argMaxSubscriptionsPerQuery subscriptions must fit in one ARG request")
				require.Len(t, api.gotRequests, 1)
				assert.Len(t, api.gotRequests[0].Subscriptions, argMaxSubscriptionsPerQuery)
			},
		},
		{
			// 2N+1 subscription IDs split into chunks of N, N, and 1.
			name: "chunks subscription list when over argMaxSubscriptionsPerQuery",
			api: &fakeARGAPI{
				pages: []argPage{
					{data: []any{makeARGVMRow("/sub/.../chunk1-vm", "chunk1-vm")}},
					{data: []any{makeARGVMRow("/sub/.../chunk2-vm", "chunk2-vm")}},
					{data: []any{makeARGVMRow("/sub/.../chunk3-vm", "chunk3-vm")}},
				},
			},
			params: QueryVMsParams{
				SubscriptionIDs: makeSubscriptionIDs(2*argMaxSubscriptionsPerQuery + 1),
			},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.NoError(t, err)
				assert.Equal(t, 3, api.calls,
					"expected one ARG call per chunk for 2N+1 subscriptions")
				require.Len(t, got, 3, "result must be the union of per-chunk responses")
				ids := []string{got[0].ID, got[1].ID, got[2].ID}
				assert.ElementsMatch(t, []string{
					"/sub/.../chunk1-vm",
					"/sub/.../chunk2-vm",
					"/sub/.../chunk3-vm",
				}, ids)
				require.Len(t, api.gotRequests, 3)
				assert.Len(t, api.gotRequests[0].Subscriptions, argMaxSubscriptionsPerQuery)
				assert.Len(t, api.gotRequests[1].Subscriptions, argMaxSubscriptionsPerQuery)
				assert.Len(t, api.gotRequests[2].Subscriptions, 1,
					"final chunk holds the remainder")
			},
		},
		{
			// SkipToken state must not carry from one subscription chunk to the next.
			name: "pagination state does not leak across chunks",
			api: &fakeARGAPI{
				pages: []argPage{
					{
						data:      []any{makeARGVMRow("/sub/.../chunk1-page1", "chunk1-page1")},
						skipToken: to.Ptr("chunk1-page2"),
					},
					{
						data: []any{makeARGVMRow("/sub/.../chunk1-page2", "chunk1-page2")},
					},
					{
						data: []any{makeARGVMRow("/sub/.../chunk2-page1", "chunk2-page1")},
					},
				},
			},
			params: QueryVMsParams{
				SubscriptionIDs: makeSubscriptionIDs(argMaxSubscriptionsPerQuery + 1),
			},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.NoError(t, err)
				require.Len(t, got, 3, "every page across both chunks must be returned")
				require.Len(t, api.gotRequests, 3)
				require.NotNil(t, api.gotRequests[0].Options)
				assert.Nil(t, api.gotRequests[0].Options.SkipToken,
					"chunk 1 page 1 must start with a nil SkipToken")
				require.NotNil(t, api.gotRequests[1].Options)
				require.NotNil(t, api.gotRequests[1].Options.SkipToken,
					"chunk 1 page 2 must carry the page-1 token")
				assert.Equal(t, "chunk1-page2", *api.gotRequests[1].Options.SkipToken)
				require.NotNil(t, api.gotRequests[2].Options)
				assert.Nil(t, api.gotRequests[2].Options.SkipToken,
					"chunk 2 page 1 must start fresh; a non-nil token here means "+
						"chunk-1 pagination state leaked into chunk 2")
			},
		},
		{
			// Parse errors on later pages must surface instead of returning partial results.
			name: "parse error on a paginated page surfaces the error",
			api: &fakeARGAPI{
				pages: []argPage{
					{
						data:      []any{makeARGVMRow("/sub/.../vm1", "vm1")},
						skipToken: to.Ptr("page-2"),
					},
					{
						// Malformed: rows must be []any of map[string]any.
						data: []any{"not a map"},
					},
				},
			},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.True(t, trace.IsBadParameter(err),
					"parseDiscoveredVMs surfaces row-shape drift as BadParameter; "+
						"got %T: %v", err, err)
				assert.Contains(t, err.Error(), "resource graph response row",
					"the row-shape error from parseDiscoveredVMs must surface, not be swallowed")
				assert.Equal(t, 2, api.calls,
					"the parse error must come from page 2; page 1 had to succeed first")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &resourceGraphClient{api: tt.api}
			got, err := c.QueryVMs(t.Context(), tt.params)
			tt.verify(t, got, tt.api, err)
		})
	}
}

// makeSubscriptionIDs returns n synthetic subscription IDs for chunking tests.
func makeSubscriptionIDs(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = fmt.Sprintf("sub-%d", i)
	}

	return out
}

func makeMockARGVM(sub, rg, location, name string) DiscoveredVM {
	return DiscoveredVM{
		ID:             fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", sub, rg, name),
		SubscriptionID: sub,
		Name:           name,
		VMID:           name + "-vmid",
		Location:       location,
		ResourceGroup:  rg,
		Tags:           map[string]string{},
	}
}

// TestARMResourceGraphMock_caseInsensitiveFilters keeps mock filtering aligned
// with KQL `in~` semantics.
func TestARMResourceGraphMock_caseInsensitiveFilters(t *testing.T) {
	t.Parallel()
	mock := &ARMResourceGraphMock{
		VMs: []DiscoveredVM{
			makeMockARGVM("sub-1", "rg-a", "eastus", "vm1"), // ARG canonical lowercase; RG names are case-insensitive in ARM.
			makeMockARGVM("sub-1", "rg-b", "westus2", "vm2"),
		},
	}

	got, err := mock.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: []string{"sub-1"},
		Regions:         []string{"EastUS"}, // display-cased
		ResourceGroups:  []string{"RG-A"},   // mixed-case
	})

	require.NoError(t, err)
	require.Len(t, got, 1,
		"mock must apply case-insensitive matching to mirror KQL `in~`; "+
			"otherwise display-cased operator config silently returns zero VMs")
	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/rg-a/providers/Microsoft.Compute/virtualMachines/vm1", got[0].ID)
}

func TestARMResourceGraphMock_wildcardMixedWithConcreteFiltersMatchesAll(t *testing.T) {
	t.Parallel()
	mock := &ARMResourceGraphMock{
		VMs: []DiscoveredVM{
			makeMockARGVM("sub-1", "rg-a", "eastus", "vm1"),
			makeMockARGVM("sub-1", "rg-b", "westus2", "vm2"),
		},
	}

	got, err := mock.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: []string{"sub-1"},
		Regions:         []string{types.Wildcard, "eastus"},
		ResourceGroups:  []string{types.Wildcard, "rg-a"},
	})

	require.NoError(t, err)
	require.Len(t, got, 2,
		"wildcard must be absorbing even when mixed with concrete filters; "+
			"otherwise unsimplified input silently narrows discovery")
}

func TestARMResourceGraphMock_rejectsInvalidFixtures(t *testing.T) {
	t.Parallel()
	mock := &ARMResourceGraphMock{
		VMs: []DiscoveredVM{
			{
				ID:             "/subscriptions/sub-1/resourceGroups/rg-a/providers/Microsoft.Compute/virtualMachines/vm1",
				SubscriptionID: "sub-1",
				Name:           "vm1",
				Location:       "eastus",
				ResourceGroup:  "rg-a",
				// VMID intentionally omitted.
			},
		},
	}

	_, err := mock.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub-1"}})
	require.Error(t, err)
	assert.True(t, trace.IsBadParameter(err), "invalid fixtures should fail like production row parsing, got %T: %v", err, err)
	assert.Contains(t, err.Error(), "ARMResourceGraphMock fixture missing or empty required field")
}

// TestARMResourceGraphMock_LastParamsClones verifies LastParams cloning.
func TestARMResourceGraphMock_LastParamsClones(t *testing.T) {
	t.Parallel()
	mock := &ARMResourceGraphMock{}

	subs := []string{"sub-1"}
	regions := []string{"eastus"}
	rgs := []string{"rg-a"}
	_, err := mock.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: subs,
		Regions:         regions,
		ResourceGroups:  rgs,
	})
	require.NoError(t, err)

	// Mutate the caller's slices in place after the call.
	subs[0] = "MUTATED"
	regions[0] = "MUTATED"
	rgs[0] = "MUTATED"

	got := mock.LastParams()
	assert.Equal(t, []string{"sub-1"}, got.SubscriptionIDs,
		"LastParams must snapshot SubscriptionIDs, not alias the caller's slice")
	assert.Equal(t, []string{"eastus"}, got.Regions,
		"LastParams must snapshot Regions, not alias the caller's slice")
	assert.Equal(t, []string{"rg-a"}, got.ResourceGroups,
		"LastParams must snapshot ResourceGroups, not alias the caller's slice")

	first := mock.LastParams()
	require.Len(t, first.SubscriptionIDs, 1)
	require.Len(t, first.Regions, 1)
	require.Len(t, first.ResourceGroups, 1)
	first.SubscriptionIDs[0] = "MUTATED"
	first.Regions[0] = "MUTATED"
	first.ResourceGroups[0] = "MUTATED"

	second := mock.LastParams()
	assert.Equal(t, []string{"sub-1"}, second.SubscriptionIDs,
		"LastParams must clone on read so prior callers cannot mutate the snapshot")
	assert.Equal(t, []string{"eastus"}, second.Regions)
	assert.Equal(t, []string{"rg-a"}, second.ResourceGroups)
}

// TestARMResourceGraphMock_AccessorsAreRaceFree exercises concurrent accessors
// under -race.
func TestARMResourceGraphMock_AccessorsAreRaceFree(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	mock := &ARMResourceGraphMock{
		VMsBySubscription: map[string][]DiscoveredVM{
			"sub-0": {makeMockARGVM("sub-0", "rg-a", "eastus", "vm-0")},
			"sub-1": {makeMockARGVM("sub-1", "rg-a", "eastus", "vm-1")},
			"sub-2": {makeMockARGVM("sub-2", "rg-a", "eastus", "vm-2")},
			"sub-3": {makeMockARGVM("sub-3", "rg-a", "eastus", "vm-3")},
		},
	}

	const writers = 4
	const readers = 4
	const callsPerWriter = 50

	var wg sync.WaitGroup
	wg.Add(writers + readers)
	errCh := make(chan error, writers*callsPerWriter)

	for i := range writers {
		go func(i int) {
			defer wg.Done()
			params := QueryVMsParams{
				SubscriptionIDs: []string{fmt.Sprintf("sub-%d", i)},
				Regions:         []string{"eastus"},
				ResourceGroups:  []string{"rg-a"},
			}
			for range callsPerWriter {
				_, err := mock.QueryVMs(ctx, params)
				if err != nil {
					errCh <- err
				}
			}
		}(i)
	}
	for range readers {
		go func() {
			defer wg.Done()
			for range callsPerWriter {
				_ = mock.Calls()
				_ = mock.LastParams()
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	assert.Equal(t, writers*callsPerWriter, mock.Calls(),
		"every QueryVMs invocation across all writers must be counted exactly once")
}
