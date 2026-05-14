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
	"cmp"
	"maps"
	"slices"

	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

const cloudAzure = "Azure"

// azureInfo holds Azure-specific instance metadata.
type azureInfo struct {
	VMID           string `json:"vm_id"`
	VMName         string `json:"vm_name,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
	ResourceGroup  string `json:"resource_group,omitempty"`
	ResourceID     string `json:"resource_id,omitempty"`
}

func (a *azureInfo) cloudName() string      { return cloudAzure }
func (a *azureInfo) cloudAccountID() string { return a.SubscriptionID }

// instanceText renders "<resource-group>/<vm-id>" when the resource group is
// known, since bare VM IDs are 36-char UUIDs that aren't useful on their own.
// Falls back to just the VM ID when the resource group is missing.
func (a *azureInfo) instanceText() string {
	if a.ResourceGroup != "" {
		return a.ResourceGroup + "/" + a.VMID
	}
	return a.VMID
}

// correlateAzureRunEvents builds an instance map from Azure run events.
// Events must be in descending order (most recent first); only the first
// event per VM ID is kept.
func correlateAzureRunEvents(events []*apievents.AzureRun) map[string]instanceInfo {
	instances := make(map[string]instanceInfo)
	for _, run := range events {
		if run.VMID == "" {
			continue
		}
		if _, ok := instances[run.VMID]; ok {
			continue // events are most-recent-first; keep only the latest
		}
		instances[run.VMID] = instanceInfo{
			Region: run.Region,
			Azure: &azureInfo{
				VMID:           run.VMID,
				VMName:         run.VMName,
				SubscriptionID: run.SubscriptionID,
				ResourceGroup:  run.ResourceGroup,
				ResourceID:     run.ResourceID,
			},
			RunResult: &runResult{
				Time:      run.Time,
				ExitCode:  int64(run.ExitCode),
				Output:    combineOutput(run.StandardOutput, run.StandardError),
				APIError:  run.APIError,
				IsFailure: run.Code == libevents.AzureRunFailCode,
			},
		}
	}
	return instances
}

// correlateAzureNodes builds an instance map from online Teleport Azure VM nodes.
func correlateAzureNodes(nodes []types.Server) map[string]instanceInfo {
	instances := make(map[string]instanceInfo)
	for _, node := range nodes {
		labels := node.GetAllLabels()
		vmID := cmp.Or(labels[types.VMIDLabel], labels[types.VMIDLabelInternal])
		if vmID == "" {
			continue
		}
		info := instanceInfo{
			IsOnline: true,
			Region:   cmp.Or(labels[types.RegionLabel], labels[types.RegionLabelInternal]),
			Azure: &azureInfo{
				VMID:           vmID,
				SubscriptionID: cmp.Or(labels[types.SubscriptionIDLabel], labels[types.SubscriptionIDLabelInternal]),
				ResourceGroup:  cmp.Or(labels[types.ResourceGroupLabel], labels[types.ResourceGroupLabelInternal]),
			},
		}
		if !node.Expiry().IsZero() {
			info.Expiry = node.Expiry()
		}
		instances[vmID] = info
	}
	return instances
}

func azureTaskInstanceKeys(task *usertasksv1.UserTask) []string {
	if instanceGroup := task.GetSpec().GetDiscoverAzureVm(); instanceGroup != nil {
		return slices.Collect(maps.Keys(instanceGroup.GetInstances()))
	}
	return nil
}
