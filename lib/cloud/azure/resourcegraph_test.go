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
	"unicode"
	"unicode/utf8"

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
		osTypes         []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:           "no filters",
			regions:        nil,
			resourceGroups: nil,
			osTypes:        nil,
			wantContains: []string{
				"Resources",
				"Microsoft.Compute/virtualMachines",
				"powerState.code) =~ 'PowerState/running'",
				"| project id, name, subscriptionId, resourceGroup",
				"vmId = tostring(properties.vmId)",
				"osType = tostring(properties.storageProfile.osDisk.osType)",
			},
			wantNotContains: []string{
				"| where location in~",
				"| where resourceGroup in~",
				"| where tostring(properties.storageProfile.osDisk.osType)",
			},
		},
		{
			name:           "wildcard region, rg, and os types",
			regions:        []string{types.Wildcard},
			resourceGroups: []string{types.Wildcard},
			osTypes:        []string{types.Wildcard},
			wantNotContains: []string{
				"| where location in~",
				"| where resourceGroup in~",
				"| where tostring(properties.storageProfile.osDisk.osType)",
			},
		},
		{
			name:           "wildcard mixed with concrete filters matches all",
			regions:        []string{types.Wildcard, "eastus"},
			resourceGroups: []string{types.Wildcard, "rg1"},
			osTypes:        []string{types.Wildcard, OSTypeLinux},
			wantNotContains: []string{
				"| where location in~",
				"| where resourceGroup in~",
				"| where tostring(properties.storageProfile.osDisk.osType)",
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
			name:    "single os type uses case-insensitive set membership",
			osTypes: []string{OSTypeLinux},
			wantContains: []string{
				"| where tostring(properties.storageProfile.osDisk.osType) in~ ('Linux')",
			},
		},
		{
			name:    "multiple os types render as a set",
			osTypes: []string{OSTypeLinux, OSTypeWindows},
			wantContains: []string{
				"| where tostring(properties.storageProfile.osDisk.osType) in~ ('Linux', 'Windows')",
			},
		},
		// The next three cases exercise buildVMDiscoveryKQL directly with values
		// QueryVMs would reject at validation time. They verify the lower-level
		// quote-escape behavior as a defense-in-depth contract on the helper,
		// not as input the public API accepts.
		{
			name:           "defense-in-depth: single quote in resource group is escaped",
			resourceGroups: []string{"rg'name"},
			wantContains: []string{
				"| where resourceGroup in~ ('rg''name')",
			},
		},
		{
			name:    "defense-in-depth: single quote in region is escaped",
			regions: []string{"east'us"},
			wantContains: []string{
				"| where location in~ ('east''us')",
			},
		},
		{
			name:    "defense-in-depth: single quote in os type is escaped",
			osTypes: []string{"odd'os"},
			wantContains: []string{
				"| where tostring(properties.storageProfile.osDisk.osType) in~ ('odd''os')",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildVMDiscoveryKQL(tt.regions, tt.resourceGroups, tt.osTypes)
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

	assert.ElementsMatch(t, []string{"SubscriptionIDs", "Regions", "ResourceGroups", "OSTypes"}, fields,
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
		"osType":         OSTypeLinux,
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
			// Bad row alongside a good one: bad is skipped, good is kept.
			// Single-row all-bad would now trip the query-level drift guard; that
			// path is covered by TestQueryVMsContractDriftOnAllQueryRowsFail.
			name: "unexpected row type is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-row-type"),
				"not a map",
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-row-type", got[0].Name)
			},
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
				"osType":         OSTypeLinux,
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
					OSType:         OSTypeLinux,
					Tags:           map[string]string{"env": "prod", "owner": "alice"},
				}}
				assert.Empty(t, cmp.Diff(want, got))
			},
		},
		{
			// osType absent on input: parser tolerates it (empty string in output).
			// VMs without an osType in ARG can still flow through; downstream
			// matchers using OSTypes will not match this VM.
			name: "missing osType yields empty OSType, VM kept",
			data: []any{func() map[string]any {
				row := makeARGVMRow("/subscriptions/sub/.../no-os", "no-os-vm")
				delete(row, "osType")
				return row
			}()},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Empty(t, got[0].OSType,
					"missing osType in the ARG row must produce empty OSType, not drop the VM")
			},
		},
		{
			// osType is optional, but if ARG returns it with a shifted type the row
			// is malformed. Keep the sibling good row so this pins row-level skip
			// behavior without tripping the query-level all-rows-failed drift guard.
			name: "non-string osType is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-nonstring-os"),
				func() map[string]any {
					row := makeARGVMRow("/subscriptions/sub/.../bad-os", "bad-os-vm")
					row["osType"] = 42
					return row
				}(),
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-nonstring-os", got[0].Name)
			},
		},
		{
			// Empty required field on the bad row: that row is skipped, the
			// sibling good row is kept. (All-bad input would trip the query-level
			// drift guard; that's covered by TestQueryVMsContractDriftOnAllQueryRowsFail.)
			name: "empty id field is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-empty-id"),
				map[string]any{
					"id":             "",
					"subscriptionId": "sub",
					"name":           "drop-empty-id",
					"vmId":           "vm-1",
					"location":       "eastus",
					"resourceGroup":  "rg",
				},
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-empty-id", got[0].Name)
			},
		},
		{
			name: "missing id field is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-missing-id"),
				map[string]any{
					"subscriptionId": "sub",
					"name":           "drop-missing-id",
					"vmId":           "vm-2",
					"location":       "eastus",
					"resourceGroup":  "rg",
				},
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-missing-id", got[0].Name)
			},
		},
		{
			// Nil required fields are treated like missing required fields.
			name: "nil required string field is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-nil-name"),
				map[string]any{
					"id":             "/subscriptions/sub/.../vm",
					"subscriptionId": "sub",
					"name":           nil,
					"vmId":           "vm-vmid",
					"location":       "westeurope",
					"resourceGroup":  "rg",
				},
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-nil-name", got[0].Name)
			},
		},
		{
			name: "missing subscriptionId field is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-missing-sub"),
				map[string]any{
					"id":            "/subscriptions/sub/.../vm",
					"name":          "vm",
					"vmId":          "vm-vmid",
					"location":      "eastus",
					"resourceGroup": "rg",
				},
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-missing-sub", got[0].Name)
			},
		},
		{
			// Type drift in a required field: bad row dropped, good row kept.
			name: "non-string scalar field is skipped, sibling good row kept",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good", "good-nonstring-scalar"),
				map[string]any{
					"id":             "/subscriptions/sub/.../vm",
					"subscriptionId": "sub",
					"name":           "vm",
					"vmId":           42, // wrong type
					"location":       "westeurope",
					"resourceGroup":  "rg",
				},
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1)
				assert.Equal(t, "good-nonstring-scalar", got[0].Name)
			},
		},
		{
			// Tag values flow into services.MatchLabels in the discovery layer.
			// Non-string drift is dropped, not coerced via fmt.Sprint, so
			// selectors never match fabricated Go-formatted values.
			name: "non-string and nil tag values are dropped, string values kept",
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
				_, hasInt := got[0].Tags["intKey"]
				assert.False(t, hasInt,
					"non-string tag values must be dropped, not stringified; "+
						"tag values feed Teleport label matchers in discovery")
				_, hasNil := got[0].Tags["nilKey"]
				assert.False(t, hasNil, "nil tag values are dropped")
			},
		},
		{
			// Tags are metadata, not identity: shape drift in tags must not
			// lose the VM. The row is kept with an empty (but non-nil) Tags map.
			name: "tags wrong type yields empty tags, VM kept",
			data: []any{map[string]any{
				"id":             "/subscriptions/sub/.../vm",
				"subscriptionId": "sub",
				"name":           "vm",
				"vmId":           "vm-vmid",
				"location":       "eastus",
				"resourceGroup":  "rg",
				"tags":           "not a map",
			}},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1, "tag-shape drift must not drop the VM; tags are metadata")
				require.NotNil(t, got[0].Tags, "Tags must always be a non-nil map")
				assert.Empty(t, got[0].Tags)
			},
		},
		{
			// One bad row in a batch must not poison the rest.
			name: "mixed good and bad rows keeps the good one",
			data: []any{
				makeARGVMRow("/subscriptions/sub/.../good-vm", "good-vm"),
				map[string]any{
					"id":             "",
					"subscriptionId": "sub",
					"name":           "bad-vm",
					"vmId":           "vm-bad",
					"location":       "eastus",
					"resourceGroup":  "rg",
				},
				"not a map",
			},
			verify: func(t *testing.T, got []DiscoveredVM) {
				require.Len(t, got, 1, "the good row must be kept even when other rows are malformed")
				assert.Equal(t, "good-vm", got[0].Name)
			},
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
			got, err := parseDiscoveredVMs(t.Context(), tt.data)
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

// TestQueryVMsContractDriftOnAllQueryRowsFail pins the failure-loud behavior
// when every row returned by a whole query fails to parse. This is the failure
// mode where a renamed field or shifted type would otherwise produce
// ([]DiscoveredVM{}, nil) - indistinguishable from a healthy empty result
// and visible only as "discovery silently stopped finding VMs."
//
// The guard lives at the query level, not per page or per subscription chunk,
// so isolated malformed pages/chunks cannot drop valid VMs found elsewhere.
func TestQueryVMsContractDriftOnAllQueryRowsFail(t *testing.T) {
	t.Parallel()
	// Mix several per-row failure shapes; every one must fail to parse for
	// the query-level drift guard to fire.
	api := &fakeARGAPI{
		pages: []argPage{{
			data: []any{
				"not a map",
				map[string]any{
					// missing id
					"subscriptionId": "sub",
					"name":           "vm-a",
					"vmId":           "vm-a-id",
					"location":       "eastus",
					"resourceGroup":  "rg",
				},
				map[string]any{
					"id":             "/sub/.../vm-b",
					"subscriptionId": "sub",
					"name":           "vm-b",
					"vmId":           42, // wrong type
					"location":       "eastus",
					"resourceGroup":  "rg",
				},
			},
		}},
	}
	c := &resourceGraphClient{api: api}

	got, err := c.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub"}})
	require.Error(t, err,
		"all rows failing to parse must surface as drift, not silent empty result")
	assert.Empty(t, got, "drift error must not return partial results")
	assert.Contains(t, err.Error(), "contract drift",
		"error must name the failure mode so operators don't read this as an empty tenant")
	assert.Contains(t, err.Error(), "3 rows",
		"error must name how many rows ARG returned for context")
}

// TestQueryVMsContractDriftIsQueryScopedNotPageScoped pins the bug-fix:
// a single bad row on a small trailing page must NOT discard earlier pages'
// valid VMs. The drift guard fires only when the entire query produced zero
// parsed VMs, not when one page happens to be all-bad.
func TestQueryVMsContractDriftIsQueryScopedNotPageScoped(t *testing.T) {
	t.Parallel()
	api := &fakeARGAPI{
		pages: []argPage{
			{
				// Page 1: a valid row.
				data:      []any{makeARGVMRow("/sub/.../valid-vm", "valid-vm")},
				skipToken: to.Ptr("page-2"),
			},
			{
				// Page 2: a single row that fails to parse (missing id).
				// Old per-page guard would fire here and drop page 1's results.
				// Query-level guard sees rawRowsTotal=2, kept=1, so no drift.
				data: []any{
					map[string]any{
						"subscriptionId": "sub",
						"name":           "bad-trailing-vm",
						"vmId":           "bad-id",
						"location":       "eastus",
						"resourceGroup":  "rg",
					},
				},
			},
		},
	}
	c := &resourceGraphClient{api: api}

	got, err := c.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub"}})
	require.NoError(t, err,
		"a single bad row on a trailing page must not tank the chunk; "+
			"page-scoped drift detection would lose page 1's valid VMs")
	require.Len(t, got, 1,
		"page 1's valid VM must be returned; only the single bad row from page 2 is dropped")
	assert.Equal(t, "valid-vm", got[0].Name)
}

// TestQueryVMsContractDriftIsQueryScopedNotChunkScoped pins the same policy at subscription-chunk
// boundaries: an all-bad later chunk must not discard valid VMs found in an earlier chunk.
func TestQueryVMsContractDriftIsQueryScopedNotChunkScoped(t *testing.T) {
	t.Parallel()
	api := &fakeARGAPI{
		pages: []argPage{
			{
				// Chunk 1: a valid row.
				data: []any{makeARGVMRow("/sub/.../valid-vm", "valid-vm")},
			},
			{
				// Chunk 2: a single row that fails to parse (missing id).
				data: []any{
					map[string]any{
						"subscriptionId": "sub",
						"name":           "bad-chunk-vm",
						"vmId":           "bad-id",
						"location":       "eastus",
						"resourceGroup":  "rg",
					},
				},
			},
		},
	}
	c := &resourceGraphClient{api: api}

	got, err := c.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: makeSubscriptionIDs(argMaxSubscriptionsPerQuery + 1),
	})
	require.NoError(t, err,
		"an all-bad later chunk must not tank the query when earlier chunks produced valid VMs")
	require.Len(t, got, 1,
		"chunk 1's valid VM must be returned; only the bad row from chunk 2 is dropped")
	assert.Equal(t, "valid-vm", got[0].Name)
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
				assert.Equal(t, argPageSize, *req.Options.Top)
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
			// ARG throttles per-user quotas with 429 plus x-ms-user-quota-resets-after.
			// ConvertResponseError must classify this as LimitExceeded so callers can
			// detect throttling (vs auth failure, transient breakage) and back off.
			name:   "429 surfaces LimitExceeded so callers can back off",
			api:    &fakeARGAPI{err: &azcore.ResponseError{StatusCode: http.StatusTooManyRequests}},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.True(t, trace.IsLimitExceeded(err),
					"429 must classify as LimitExceeded so the discovery loop can distinguish "+
						"throttling from other errors and reduce poll frequency; got %T: %v", err, err)
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
			// Outer-shape drift on a later page must surface fatally; earlier pages' success
			// must not mask it.
			name: "outer-shape drift on a paginated page surfaces the error",
			api: &fakeARGAPI{
				pages: []argPage{
					{
						data:      []any{makeARGVMRow("/sub/.../vm1", "vm1")},
						skipToken: to.Ptr("page-2"),
					},
					{
						// Outer-shape drift: Data must be []any, not a string.
						data: "not a slice",
					},
				},
			},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.Error(t, err)
				assert.True(t, trace.IsBadParameter(err),
					"outer-shape drift must surface as BadParameter; got %T: %v", err, err)
				assert.Contains(t, err.Error(), "resource graph response Data",
					"the outer-shape error from parseDiscoveredVMs must surface, not be swallowed")
				assert.Equal(t, 2, api.calls,
					"the parse error must come from page 2; page 1 had to succeed first")
			},
		},
		{
			// Per-row malformed entries on a later page must be skipped, not fatal: the
			// good page 1 row plus the good entry on page 2 should be returned.
			name: "row-level drift on a paginated page is skipped, chunk still succeeds",
			api: &fakeARGAPI{
				pages: []argPage{
					{
						data:      []any{makeARGVMRow("/sub/.../vm1", "vm1")},
						skipToken: to.Ptr("page-2"),
					},
					{
						data: []any{
							"not a map",
							makeARGVMRow("/sub/.../vm2", "vm2"),
						},
					},
				},
			},
			params: QueryVMsParams{SubscriptionIDs: []string{"sub"}},
			verify: func(t *testing.T, got []DiscoveredVM, api *fakeARGAPI, err error) {
				require.NoError(t, err,
					"per-row drift must skip the row, not abort the chunk")
				require.Len(t, got, 2,
					"page 1's good row plus page 2's good row must both be kept; only the malformed row is dropped")
				assert.Equal(t, "vm1", got[0].Name)
				assert.Equal(t, "vm2", got[1].Name)
				assert.Equal(t, 2, api.calls)
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
		OSType:         OSTypeLinux,
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

// TestARMResourceGraphMock_OSTypesFiltering proves the mock enforces OSTypes
// against the fixture's OSType field. Without this, a regression that swaps
// the production KQL operator (e.g. in~ to ==) would still pass mock-based
// tests despite breaking against real ARG.
func TestARMResourceGraphMock_OSTypesFiltering(t *testing.T) {
	t.Parallel()
	linuxVM := makeMockARGVM("sub-1", "rg-a", "eastus", "linux-vm")
	windowsVM := makeMockARGVM("sub-1", "rg-a", "eastus", "windows-vm")
	windowsVM.OSType = OSTypeWindows
	mock := &ARMResourceGraphMock{
		VMs: []DiscoveredVM{linuxVM, windowsVM},
	}

	t.Run("Linux-only filter returns only Linux VMs", func(t *testing.T) {
		t.Parallel()
		got, err := mock.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub-1"},
			OSTypes:         []string{OSTypeLinux},
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "linux-vm", got[0].Name)
	})

	t.Run("Windows-only filter returns only Windows VMs", func(t *testing.T) {
		t.Parallel()
		got, err := mock.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub-1"},
			OSTypes:         []string{OSTypeWindows},
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "windows-vm", got[0].Name)
	})

	t.Run("wildcard OSTypes returns all OS types", func(t *testing.T) {
		t.Parallel()
		got, err := mock.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub-1"},
			OSTypes:         []string{types.Wildcard},
		})
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("OSTypes uses case-insensitive matching like KQL in~", func(t *testing.T) {
		t.Parallel()
		got, err := mock.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub-1"},
			OSTypes:         []string{"linux"}, // canonical lowercase; fixture has "Linux"
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "linux-vm", got[0].Name)
	})
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
	osTypes := []string{OSTypeLinux}
	_, err := mock.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: subs,
		Regions:         regions,
		ResourceGroups:  rgs,
		OSTypes:         osTypes,
	})
	require.NoError(t, err)

	// Mutate the caller's slices in place after the call.
	subs[0] = "MUTATED"
	regions[0] = "MUTATED"
	rgs[0] = "MUTATED"
	osTypes[0] = "MUTATED"

	got := mock.LastParams()
	assert.Equal(t, []string{"sub-1"}, got.SubscriptionIDs,
		"LastParams must snapshot SubscriptionIDs, not alias the caller's slice")
	assert.Equal(t, []string{"eastus"}, got.Regions,
		"LastParams must snapshot Regions, not alias the caller's slice")
	assert.Equal(t, []string{"rg-a"}, got.ResourceGroups,
		"LastParams must snapshot ResourceGroups, not alias the caller's slice")
	assert.Equal(t, []string{OSTypeLinux}, got.OSTypes,
		"LastParams must snapshot OSTypes, not alias the caller's slice")

	first := mock.LastParams()
	require.Len(t, first.SubscriptionIDs, 1)
	require.Len(t, first.Regions, 1)
	require.Len(t, first.ResourceGroups, 1)
	require.Len(t, first.OSTypes, 1)
	first.SubscriptionIDs[0] = "MUTATED"
	first.Regions[0] = "MUTATED"
	first.ResourceGroups[0] = "MUTATED"
	first.OSTypes[0] = "MUTATED"

	second := mock.LastParams()
	assert.Equal(t, []string{"sub-1"}, second.SubscriptionIDs,
		"LastParams must clone on read so prior callers cannot mutate the snapshot")
	assert.Equal(t, []string{"eastus"}, second.Regions)
	assert.Equal(t, []string{"rg-a"}, second.ResourceGroups)
	assert.Equal(t, []string{OSTypeLinux}, second.OSTypes)
}

// TestARMResourceGraphMock_ValidationFailureDoesNotBumpCalls pins the mock's
// agreement with production: production rejects invalid params before any SDK
// round trip, so the mock's calls counter must not increment on validation
// failure. Otherwise mock-based tests would over-count calls relative to the
// real client.
func TestARMResourceGraphMock_ValidationFailureDoesNotBumpCalls(t *testing.T) {
	t.Parallel()
	mock := &ARMResourceGraphMock{}

	_, err := mock.QueryVMs(t.Context(), QueryVMsParams{})
	require.Error(t, err)
	assert.True(t, trace.IsBadParameter(err))
	assert.Equal(t, 0, mock.Calls(),
		"validation failure must not register as a call; production never reaches the SDK either")

	_, err = mock.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: []string{"sub"},
		Regions:         []string{"east'us"}, // invalid char
	})
	require.Error(t, err)
	assert.Equal(t, 0, mock.Calls(),
		"filter validation failure must not register as a call either")

	// Sanity: a valid call increments.
	_, err = mock.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub"}})
	require.NoError(t, err)
	assert.Equal(t, 1, mock.Calls())
}

// TestARMResourceGraphMock_PreservesNonNilTags pins the production contract that
// DiscoveredVM.Tags is always non-nil (empty map for "no tags"). cloneDiscoveredVMs
// must not regress this when a fixture omits Tags.
func TestARMResourceGraphMock_PreservesNonNilTags(t *testing.T) {
	t.Parallel()
	mock := &ARMResourceGraphMock{
		VMs: []DiscoveredVM{
			{
				ID:             "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1",
				SubscriptionID: "sub",
				Name:           "vm1",
				VMID:           "vm1-vmid",
				Location:       "eastus",
				ResourceGroup:  "rg",
				// Tags intentionally nil.
			},
		},
	}

	got, err := mock.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub"}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[0].Tags,
		"Tags must be non-nil to match production's parseDiscoveredVMs contract; "+
			"callers should be able to index Tags without a nil check")
	assert.Empty(t, got[0].Tags)
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

func TestGetStringIfPresent(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"name": "vincent",
		"nil":  nil,
		"int":  42,
	}

	got, err := getStringIfPresent(m, "name")
	require.NoError(t, err)
	require.Equal(t, "vincent", got)

	got, err = getStringIfPresent(m, "missing")
	require.NoError(t, err)
	require.Empty(t, got)

	got, err = getStringIfPresent(m, "nil")
	require.NoError(t, err)
	require.Empty(t, got)

	_, err = getStringIfPresent(m, "int")
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err), "type drift must surface as BadParameter, got %T", err)
}

func TestGetStringMap(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"happy":  map[string]any{"strKey": "value", "intKey": 42, "nilKey": nil},
		"wrong":  "not a map",
		"nested": map[string]any{"deepKey": map[string]any{"k": "v"}},
	}

	t.Run("missing key yields empty non-nil map", func(t *testing.T) {
		got := getStringMap(t.Context(), m, "missing")
		require.NotNil(t, got)
		assert.Empty(t, got)
	})

	t.Run("non-string and nil values are dropped, string values kept", func(t *testing.T) {
		got := getStringMap(t.Context(), m, "happy")
		assert.Equal(t, map[string]string{"strKey": "value"}, got,
			"non-string tag values must be dropped, not coerced via fmt.Sprint, "+
				"since tag values feed services.MatchLabels in discovery enrollment decisions")
	})

	t.Run("outer shape drift yields empty map, not error", func(t *testing.T) {
		got := getStringMap(t.Context(), m, "wrong")
		require.NotNil(t, got)
		assert.Empty(t, got,
			"wrong outer type is logged at debug and degrades to empty tags; "+
				"the parent row is not lost")
	})

	t.Run("structured inner values are dropped, not Go-formatted", func(t *testing.T) {
		got := getStringMap(t.Context(), m, "nested")
		assert.Empty(t, got,
			"nested object tag values are schema drift; dropping them is safer "+
				"than fmt.Sprint producing Go-syntax strings selectors would never see")
	})
}

// TestQueryVMsValidatesInputs is the restriction half of the C73 defense-in-depth.
// It pairs with FuzzEscapeKQL: the regex rejects any input that contains KQL
// metacharacters, escapeKQL is the safety net if validation is ever bypassed.
func TestQueryVMsValidatesInputs(t *testing.T) {
	t.Parallel()
	// Dangerous characters and substrings that must never reach escapeKQL.
	// Each is tried against every list field (Regions, ResourceGroups, OSTypes)
	// so a future loosened regex on any one field is caught by these tests.
	dangerous := []struct {
		name  string
		value string
	}{
		{"single quote", "east'us"},
		{"double quote", `east"us`},
		{"backslash", `east\us`},
		{"newline", "east\nus"},
		{"carriage return", "east\rus"},
		{"tab", "east\tus"},
		{"null byte", "east\x00us"},
		{"pipe", "east|us"},
		{"semicolon", "east;us"},
		{"space", "east us"},
		{"unicode letter", "eastusé"},
		{"emoji", "eastus🎉"},
		{"comment fragment", "eastus // x"},
		{"slash star", "eastus/*"},
	}
	type field int
	const (
		fieldRegion field = iota
		fieldRG
		fieldOSType
	)
	fields := []struct {
		f    field
		kind string
	}{
		{fieldRegion, "region"},
		{fieldRG, "resource group"},
		{fieldOSType, "OS type"},
	}

	for _, fld := range fields {
		for _, d := range dangerous {
			t.Run(fld.kind+"/"+d.name, func(t *testing.T) {
				t.Parallel()
				params := QueryVMsParams{SubscriptionIDs: []string{"sub"}}
				switch fld.f {
				case fieldRegion:
					params.Regions = []string{d.value}
				case fieldRG:
					params.ResourceGroups = []string{d.value}
				case fieldOSType:
					params.OSTypes = []string{d.value}
				}

				c := &resourceGraphClient{api: &fakeARGAPI{}}
				_, err := c.QueryVMs(t.Context(), params)
				require.Error(t, err)
				assert.True(t, trace.IsBadParameter(err),
					"validation must surface as BadParameter, got %T: %v", err, err)
				assert.Contains(t, err.Error(), fld.kind,
					"error must name the offending field kind")
				assert.Contains(t, err.Error(), fmt.Sprintf("%q", d.value),
					"error must include the offending value so operators can find their config")
			})
		}
	}

	// emptyPage is a single empty Resource Graph page the fake replays when
	// validation must pass and the call must proceed to the SDK; the test
	// then asserts no error was returned, regardless of the (empty) result.
	emptyPage := func() *fakeARGAPI {
		return &fakeARGAPI{pages: []argPage{{data: []any{}}}}
	}

	// Wildcards must still pass for every field; the validator should not
	// treat "*" as an invalid character.
	t.Run("wildcard passes for all fields", func(t *testing.T) {
		t.Parallel()
		c := &resourceGraphClient{api: emptyPage()}
		_, err := c.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub"},
			Regions:         []string{types.Wildcard},
			ResourceGroups:  []string{types.Wildcard},
			OSTypes:         []string{types.Wildcard},
		})
		require.NoError(t, err)
	})

	// Wildcard mixed with concrete (still valid) entries also passes; the
	// validator must check each entry independently and skip wildcards.
	t.Run("wildcard mixed with valid concrete passes", func(t *testing.T) {
		t.Parallel()
		c := &resourceGraphClient{api: emptyPage()}
		_, err := c.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub"},
			Regions:         []string{types.Wildcard, "eastus"},
			ResourceGroups:  []string{types.Wildcard, "rg-1"},
			OSTypes:         []string{types.Wildcard, OSTypeLinux},
		})
		require.NoError(t, err)
	})

	// Empty filter values produce a clear "must not be empty" message, not
	// the generic "contains invalid characters" the regex would emit.
	t.Run("empty region rejected with clear message", func(t *testing.T) {
		t.Parallel()
		c := &resourceGraphClient{api: &fakeARGAPI{}}
		_, err := c.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"sub"},
			Regions:         []string{""},
		})
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "region must not be empty",
			"empty values get a dedicated error rather than a confusing 'invalid characters'")
	})

	// Untrimmed values produce a "must not have leading or trailing whitespace"
	// message, distinguishing input-hygiene errors from character-class errors.
	// Each field has its own kind label, so check all three.
	untrimmed := []struct {
		name   string
		params QueryVMsParams
		kind   string
		value  string
	}{
		{
			name: "untrimmed region rejected with clear message",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"sub"},
				Regions:         []string{" eastus "},
			},
			kind:  "region",
			value: " eastus ",
		},
		{
			name: "untrimmed resource group rejected with clear message",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"sub"},
				ResourceGroups:  []string{" rg-1 "},
			},
			kind:  "resource group",
			value: " rg-1 ",
		},
		{
			name: "untrimmed OS type rejected with clear message",
			params: QueryVMsParams{
				SubscriptionIDs: []string{"sub"},
				OSTypes:         []string{" Linux "},
			},
			kind:  "OS type",
			value: " Linux ",
		},
	}
	for _, tt := range untrimmed {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &resourceGraphClient{api: &fakeARGAPI{}}
			_, err := c.QueryVMs(t.Context(), tt.params)
			require.Error(t, err)
			assert.True(t, trace.IsBadParameter(err))
			assert.Contains(t, err.Error(), tt.kind,
				"error must name the offending field kind")
			assert.Contains(t, err.Error(), "leading or trailing whitespace")
			assert.Contains(t, err.Error(), fmt.Sprintf("%q", tt.value),
				"error must include the offending value")
		})
	}

	// Empty subscription IDs are caught as a query-scope error before any
	// filter validation runs.
	t.Run("empty subscription ID rejected", func(t *testing.T) {
		t.Parallel()
		c := &resourceGraphClient{api: &fakeARGAPI{}}
		_, err := c.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{""},
		})
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "subscription ID must not be empty")
	})

	t.Run("whitespace-only subscription ID rejected", func(t *testing.T) {
		t.Parallel()
		c := &resourceGraphClient{api: &fakeARGAPI{}}
		_, err := c.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{"   "},
		})
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "subscription ID must not be empty")
	})

	t.Run("untrimmed subscription ID rejected", func(t *testing.T) {
		t.Parallel()
		c := &resourceGraphClient{api: &fakeARGAPI{}}
		_, err := c.QueryVMs(t.Context(), QueryVMsParams{
			SubscriptionIDs: []string{" sub-1 "},
		})
		require.Error(t, err)
		assert.True(t, trace.IsBadParameter(err))
		assert.Contains(t, err.Error(), "must not have leading or trailing whitespace")
	})
}

// TestQueryVMsTruncatedWithoutSkipToken pins the behavior where ARG marks the
// response truncated but omits a SkipToken: results would otherwise be silently
// incomplete, so QueryVMs must surface this as an error instead.
func TestQueryVMsTruncatedWithoutSkipToken(t *testing.T) {
	t.Parallel()
	api := &fakeARGAPI{
		pages: []argPage{
			{
				data:            []any{makeARGVMRow("/sub/.../vm1", "vm1")},
				resultTruncated: to.Ptr(armresourcegraph.ResultTruncatedTrue),
				// No skipToken -- the pathology this test targets.
			},
		},
	}
	c := &resourceGraphClient{api: api}
	_, err := c.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "truncated",
		"truncation without a skip token must surface; silently returning partial results would be worse")
	assert.Contains(t, err.Error(), "result_truncated=true",
		"the response summary should name the failure mode")
}

// TestQueryVMsNotTruncatedWithoutSkipTokenReturnsResults pins the partner of
// TestQueryVMsTruncatedWithoutSkipToken: a clean final page (ResultTruncated=false,
// no SkipToken) must return its rows, not be misread as a truncation error.
func TestQueryVMsNotTruncatedWithoutSkipTokenReturnsResults(t *testing.T) {
	t.Parallel()
	api := &fakeARGAPI{
		pages: []argPage{
			{
				data:            []any{makeARGVMRow("/sub/.../vm1", "vm1")},
				resultTruncated: to.Ptr(armresourcegraph.ResultTruncatedFalse),
			},
		},
	}
	c := &resourceGraphClient{api: api}
	got, err := c.QueryVMs(t.Context(), QueryVMsParams{SubscriptionIDs: []string{"sub"}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "vm1", got[0].Name)
}

// TestQueryVMsForwardsDuplicateSubscriptions pins the documented no-dedup
// contract on SubscriptionIDs: duplicate entries flow through to ARG as-is,
// which may produce duplicate VMs. Deduplication is the caller's job.
func TestQueryVMsForwardsDuplicateSubscriptions(t *testing.T) {
	t.Parallel()
	api := &fakeARGAPI{
		pages: []argPage{{data: []any{}}},
	}
	c := &resourceGraphClient{api: api}
	_, err := c.QueryVMs(t.Context(), QueryVMsParams{
		SubscriptionIDs: []string{"sub-1", "sub-1", "sub-2"},
	})
	require.NoError(t, err)
	require.Len(t, api.gotRequests, 1, "all entries fit in one chunk")
	require.Len(t, api.gotRequests[0].Subscriptions, 3,
		"duplicates must pass through unchanged; deduplication is the caller's contract")
	for i, want := range []string{"sub-1", "sub-1", "sub-2"} {
		require.NotNil(t, api.gotRequests[0].Subscriptions[i])
		assert.Equal(t, want, *api.gotRequests[0].Subscriptions[i],
			"subscription order must be preserved across the dedup-skipping path")
	}
}

// TestDecodeKQLSingleQuoted exercises the quoteKQL inverse directly so a bug in
// the test decoder cannot silently make FuzzEscapeKQL pass for the wrong reasons.
func TestDecodeKQLSingleQuoted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", `''`, ""},
		{"plain", `'plain'`, "plain"},
		{"doubled quote", `'with''quote'`, "with'quote"},
		{"two doubled quotes", `'two''''quotes'`, "two''quotes"},
		{"leading quote", `'''leading'`, "'leading"},
		{"trailing quote", `'trailing'''`, "trailing'"},
		{"backslash literal", `'back\slash'`, `back\slash`},
		{"newline literal", "'line\nbreak'", "line\nbreak"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeKQLSingleQuoted(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{"unwrapped", `plain`},
		{"missing closing quote", `'plain`},
		{"missing opening quote", `plain'`},
		{"isolated quote in body", `'east'us'`},
		{"single byte", `'`},
	}
	for _, tt := range bad {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeKQLSingleQuoted(tt.in)
			require.Error(t, err)
		})
	}
}

// TestDecodeKQLDoubleQuoted exercises the double-quoted KQL decoder directly so
// a decoder bug cannot mask reference-encoder disagreement in FuzzQuoteKQL.
func TestDecodeKQLDoubleQuoted(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", `""`, ""},
		{"plain", `"plain"`, "plain"},
		{"escaped double quote", `"with\"quote"`, `with"quote`},
		{"escaped single quote", `"with\'quote"`, `with'quote`},
		{"escaped backslash", `"with\\slash"`, `with\slash`},
		{"null", `"with\0null"`, "with\x00null"},
		{"control chars", `"a\bb\fc\nd\re\tf\vg"`, "a\bb\fc\nd\re\tf\vg"},
		{"unicode bmp", `"é"`, "é"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeKQLDoubleQuoted(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	bad := []struct {
		name string
		in   string
	}{
		{"unwrapped", `plain`},
		{"missing closing quote", `"plain`},
		{"unknown escape", `"\q"`},
		{"truncated unicode", `"\u12"`},
		{"non-hex unicode", `"\uzzzz"`},
		{"single byte", `"`},
	}
	for _, tt := range bad {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := decodeKQLDoubleQuoted(tt.in)
			require.Error(t, err)
		})
	}
}

// FuzzEscapeKQL is the second half of the C73 defense-in-depth: even if a
// dangerous value ever bypasses validateKQLValues, escapeKQL must produce
// output that cannot break out of the surrounding single-quoted KQL literal.
// Two invariants:
//  1. Round-trip: undoing the quote-doubling recovers the original input.
//  2. No isolated single quote remains in the output.
func FuzzEscapeKQL(f *testing.F) {
	seeds := []string{
		"",
		"plain",
		"with'quote",
		"two''quotes",
		"'leading",
		"trailing'",
		`back\slash`,
		"new\nline",
		"null\x00byte",
		"tab\there",
		"unicode-é",
		"emoji-🎉",
		"' OR 1=1 --",
		"'; drop table; --",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		escaped := escapeKQL(s)

		// Invariant 1: round-trip recovers the input exactly.
		recovered := strings.ReplaceAll(escaped, "''", "'")
		if recovered != s {
			t.Fatalf("escapeKQL round-trip failed: input %q -> escaped %q -> recovered %q",
				s, escaped, recovered)
		}

		// Invariant 2: every single quote in the output is part of a
		// doubled pair. An isolated quote would close the surrounding
		// KQL literal and let the rest of the input be parsed as KQL.
		for i := 0; i < len(escaped); i++ {
			if escaped[i] != '\'' {
				continue
			}
			if i+1 >= len(escaped) || escaped[i+1] != '\'' {
				t.Fatalf("escapeKQL left an isolated single quote at index %d in %q (input %q)",
					i, escaped, s)
			}
			i++
		}
	})
}

// FuzzQuoteKQL checks quoteKQL against an independent local double-quoted KQL
// reference. The two encoders produce different forms by design:
//   - ours: single-quoted with ” doubling, e.g. 'east”us'
//   - reference: double-quoted with backslash escapes, e.g. "east\'us"
//
// String equality between the two outputs is meaningless; we compare by
// decoding both back to the original input and asserting they match.
func FuzzQuoteKQL(f *testing.F) {
	seeds := []string{
		"",
		"plain",
		"with'quote",
		"two''quotes",
		`back\slash`,
		`mixed'and\both`,
		"new\nline",
		"null\x00byte",
		"tab\there",
		"unicode-é",
		"emoji-🎉",
		"' OR 1=1 --",
		"'; drop table; --",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// The dependency-free reference below mirrors Azure Kusto's string quoting
		// behavior, which ranges over runes. Skip inputs where that behavior is
		// intentionally lossy or ambiguous compared to production's byte-preserving
		// single-quote escape.
		if !utf8.ValidString(s) {
			t.Skip()
		}
		for _, r := range s {
			// Azure Kusto's QuoteString emits fmt.Sprintf("\\u%04x", r), which
			// produces 5+ hex digits for astral-plane runes. KQL \u escapes are
			// four hex digits, so skip these rather than make the local decoder
			// accept ambiguous nonstandard output.
			if r > 0xFFFF {
				t.Skip()
			}
		}

		ours := quoteKQL(s)
		theirs := quoteKQLDoubleQuotedReference(s)

		oursDecoded, err := decodeKQLSingleQuoted(ours)
		if err != nil {
			t.Fatalf("quoteKQL produced an undecodable string: input=%q ours=%q err=%v", s, ours, err)
		}
		if oursDecoded != s {
			t.Fatalf("quoteKQL did not faithfully encode input: input=%q ours=%q decoded=%q",
				s, ours, oursDecoded)
		}

		theirsDecoded, err := decodeKQLDoubleQuoted(theirs)
		if err != nil {
			t.Fatalf("double-quoted reference produced an undecodable string: input=%q theirs=%q err=%v",
				s, theirs, err)
		}
		if theirsDecoded != s {
			t.Fatalf("double-quoted reference did not faithfully encode input: input=%q theirs=%q decoded=%q",
				s, theirs, theirsDecoded)
		}

		// Parity: both encoders agreed on the original value.
		if oursDecoded != theirsDecoded {
			t.Fatalf("parity failed: input=%q ours=%q→%q theirs=%q→%q",
				s, ours, oursDecoded, theirs, theirsDecoded)
		}
	})
}

// quoteKQLDoubleQuotedReference is a test-only, dependency-free port of Azure
// Kusto's QuoteString(value, false) string escaping behavior. It intentionally
// uses a different representation than production quoteKQL, which makes
// FuzzQuoteKQL compare semantics rather than output form.
func quoteKQLDoubleQuotedReference(s string) string {
	var out strings.Builder
	out.Grow(len(s) + 2)
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '\'':
			out.WriteString(`\'`)
		case '"':
			out.WriteString(`\"`)
		case '\x00':
			out.WriteString(`\0`)
		case '\a':
			out.WriteString(`\a`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\v':
			out.WriteString(`\v`)
		default:
			if !shouldEscapeKQLRune(r) {
				out.WriteRune(r)
			} else {
				out.WriteString(fmt.Sprintf(`\u%04x`, r))
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

func shouldEscapeKQLRune(r rune) bool {
	if r <= unicode.MaxLatin1 {
		return unicode.IsControl(r)
	}
	return true
}

// decodeKQLSingleQuoted is the inverse of quoteKQL: strip the surrounding
// single quotes and undo the ” doubling. Errors if the input is not a
// well-formed single-quoted KQL string literal.
func decodeKQLSingleQuoted(s string) (string, error) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", fmt.Errorf("not a single-quoted literal: %q", s)
	}
	inner := s[1 : len(s)-1]
	// Walk the body to ensure every ' is doubled (no isolated quote).
	var out strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\'' {
			out.WriteByte(inner[i])
			continue
		}
		if i+1 >= len(inner) || inner[i+1] != '\'' {
			return "", fmt.Errorf("isolated single quote at index %d in %q", i, inner)
		}
		out.WriteByte('\'')
		i++
	}
	return out.String(), nil
}

// decodeKQLDoubleQuoted is the inverse of quoteKQLDoubleQuotedReference: strip
// the surrounding double quotes and undo backslash escapes. Mirrors the escape
// set in quoteKQLDoubleQuotedReference plus \uNNNN so decoder tests can catch
// malformed unicode escapes.
func decodeKQLDoubleQuoted(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("not a double-quoted literal: %q", s)
	}
	inner := s[1 : len(s)-1]
	var out strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] != '\\' {
			out.WriteByte(inner[i])
			continue
		}
		if i+1 >= len(inner) {
			return "", fmt.Errorf("trailing backslash in %q", inner)
		}
		switch inner[i+1] {
		case '\\':
			out.WriteByte('\\')
		case '\'':
			out.WriteByte('\'')
		case '"':
			out.WriteByte('"')
		case '0':
			out.WriteByte('\x00')
		case 'a':
			out.WriteByte('\a')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'v':
			out.WriteByte('\v')
		case 'u':
			if i+5 >= len(inner) {
				return "", fmt.Errorf("truncated \\uNNNN at index %d in %q", i, inner)
			}
			var r rune
			for j := 2; j < 6; j++ {
				c := inner[i+j]
				var d rune
				switch {
				case '0' <= c && c <= '9':
					d = rune(c - '0')
				case 'a' <= c && c <= 'f':
					d = rune(c-'a') + 10
				case 'A' <= c && c <= 'F':
					d = rune(c-'A') + 10
				default:
					return "", fmt.Errorf("non-hex digit %q in \\uNNNN at index %d in %q", c, i, inner)
				}
				r = r*16 + d
			}
			out.WriteRune(r)
			i += 4
		default:
			return "", fmt.Errorf("unknown escape \\%c at index %d in %q", inner[i+1], i, inner)
		}
		i++
	}
	return out.String(), nil
}
