// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

const (
	azureExecStateSucceeded = "Succeeded"
	azureExecStateFailed    = "Failed"
)

// makeAzureRun creates an *apievents.AzureRun suitable for use in tests.
func makeAzureRun(vmID, subscriptionID, resourceGroup, region, execState string, exitCode int32, output, apiError string, ts time.Time) *apievents.AzureRun {
	code := libevents.AzureRunFailCode
	if execState == azureExecStateSucceeded {
		code = libevents.AzureRunSuccessCode
	}
	return &apievents.AzureRun{
		Metadata: apievents.Metadata{
			Type: libevents.AzureRunEvent,
			Time: ts,
			Code: code,
		},
		AzureMetadata: apievents.AzureMetadata{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Region:         region,
		},
		AzureVMMetadata: apievents.AzureVMMetadata{
			VMID: vmID,
		},
		ExitCode:       exitCode,
		ExecutionState: execState,
		StandardOutput: output,
		APIError:       apiError,
	}
}

// makeAzureNode creates a types.Server with Azure VM labels set.
func makeAzureNode(name, vmID, subscriptionID, resourceGroup, region string, expiry time.Time) *types.ServerV2 {
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: name,
			Labels: map[string]string{
				types.VMIDLabel:           vmID,
				types.SubscriptionIDLabel: subscriptionID,
				types.ResourceGroupLabel:  resourceGroup,
				types.RegionLabel:         region,
			},
		},
	}
	if !expiry.IsZero() {
		node.Metadata.SetExpiry(expiry)
	}
	return node
}

func TestCorrelateAzureRunEvents(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		desc   string
		events []*apievents.AzureRun
		want   map[string]instanceInfo
	}{
		{
			desc: "failures and successes both included",
			events: []*apievents.AzureRun{
				makeAzureRun("vm-aaa", "sub-1", "rg-1", "eastus", azureExecStateFailed, 1, "install failed", "", now),
				makeAzureRun("vm-bbb", "sub-2", "rg-2", "westus", azureExecStateSucceeded, 0, "", "", now),
			},
			want: map[string]instanceInfo{
				"vm-aaa": {
					Azure:     &azureInfo{VMID: "vm-aaa", SubscriptionID: "sub-1", ResourceGroup: "rg-1"},
					Region:    "eastus",
					RunResult: &runResult{ExitCode: 1, Output: "install failed", Time: now, IsFailure: true},
				},
				"vm-bbb": {
					Azure:     &azureInfo{VMID: "vm-bbb", SubscriptionID: "sub-2", ResourceGroup: "rg-2"},
					Region:    "westus",
					RunResult: &runResult{ExitCode: 0, Time: now},
				},
			},
		},
		{
			desc: "empty VM ID is skipped",
			events: []*apievents.AzureRun{
				makeAzureRun("", "", "", "", azureExecStateFailed, 1, "", "", now),
			},
			want: map[string]instanceInfo{},
		},
		{
			desc: "dedup keeps most recent",
			events: []*apievents.AzureRun{
				makeAzureRun("vm-aaa", "sub-1", "rg-1", "eastus", azureExecStateFailed, 1, "newest error", "", now),
				makeAzureRun("vm-aaa", "sub-1", "rg-1", "eastus", azureExecStateFailed, 2, "older error", "", now.Add(-10*time.Minute)),
			},
			want: map[string]instanceInfo{
				"vm-aaa": {
					Azure:     &azureInfo{VMID: "vm-aaa", SubscriptionID: "sub-1", ResourceGroup: "rg-1"},
					Region:    "eastus",
					RunResult: &runResult{ExitCode: 1, Output: "newest error", Time: now, IsFailure: true},
				},
			},
		},
		{
			desc: "API error surfaces",
			events: []*apievents.AzureRun{
				makeAzureRun("vm-aaa", "sub-1", "rg-1", "eastus", azureExecStateFailed, 0, "", "forbidden", now),
			},
			want: map[string]instanceInfo{
				"vm-aaa": {
					Azure:     &azureInfo{VMID: "vm-aaa", SubscriptionID: "sub-1", ResourceGroup: "rg-1"},
					Region:    "eastus",
					RunResult: &runResult{ExitCode: 0, APIError: "forbidden", Time: now, IsFailure: true},
				},
			},
		},
		{
			desc: "VMName and ResourceID propagate from event metadata",
			events: []*apievents.AzureRun{
				{
					Metadata: apievents.Metadata{Type: libevents.AzureRunEvent, Time: now, Code: libevents.AzureRunSuccessCode},
					AzureMetadata: apievents.AzureMetadata{
						SubscriptionID: "sub-1",
						ResourceGroup:  "rg-1",
						Region:         "eastus",
						ResourceID:     "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/web-prod-01",
					},
					AzureVMMetadata: apievents.AzureVMMetadata{VMID: "vm-aaa", VMName: "web-prod-01"},
				},
			},
			want: map[string]instanceInfo{
				"vm-aaa": {
					Azure: &azureInfo{
						VMID:           "vm-aaa",
						VMName:         "web-prod-01",
						SubscriptionID: "sub-1",
						ResourceGroup:  "rg-1",
						ResourceID:     "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/virtualMachines/web-prod-01",
					},
					Region:    "eastus",
					RunResult: &runResult{ExitCode: 0, Time: now},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := correlateAzureRunEvents(tt.events)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCorrelateAzureNodes(t *testing.T) {
	t.Parallel()

	expiry := time.Now().UTC().Truncate(time.Second).Add(time.Hour)

	tests := []struct {
		desc  string
		nodes []types.Server
		want  map[string]instanceInfo
	}{
		{
			desc:  "node with visible labels",
			nodes: []types.Server{makeAzureNode("node-1", "vm-aaa", "sub-1", "rg-1", "eastus", expiry)},
			want: map[string]instanceInfo{
				"vm-aaa": {
					IsOnline: true,
					Region:   "eastus",
					Expiry:   expiry,
					Azure:    &azureInfo{VMID: "vm-aaa", SubscriptionID: "sub-1", ResourceGroup: "rg-1"},
				},
			},
		},
		{
			desc: "node with internal labels falls back",
			nodes: []types.Server{
				&types.ServerV2{
					Kind:    types.KindNode,
					Version: types.V2,
					Metadata: types.Metadata{
						Name: "node-1",
						Labels: map[string]string{
							types.VMIDLabelInternal:           "vm-aaa",
							types.SubscriptionIDLabelInternal: "sub-1",
							types.ResourceGroupLabelInternal:  "rg-1",
							types.RegionLabelInternal:         "eastus",
						},
					},
				},
			},
			want: map[string]instanceInfo{
				"vm-aaa": {
					IsOnline: true,
					Region:   "eastus",
					Azure:    &azureInfo{VMID: "vm-aaa", SubscriptionID: "sub-1", ResourceGroup: "rg-1"},
				},
			},
		},
		{
			desc:  "node without VM ID is skipped",
			nodes: []types.Server{makeNode("node-1", "i-aws", "111", "us-east-1", time.Time{})},
			want:  map[string]instanceInfo{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := correlateAzureNodes(tt.nodes)
			require.Equal(t, tt.want, got)
		})
	}
}
