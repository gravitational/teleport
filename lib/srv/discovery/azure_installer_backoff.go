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
	"time"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
)

// azureInstallerBackoffKey identifies an Azure virtual machine for installer
// backoff purposes.
type azureInstallerBackoffKey struct {
	// resourceID is the path based resource ID Azure, e.g., /subscriptions/<sub-id>/resourceGroups/<rg-id>/providers/Microsoft.Compute/virtualMachines/<vm-name>
	// It is not necessarily unique.
	resourceID string
	// vmID is a VM UUID. In practice this ID is not empty.
	// In case it is empty, we also include the resource ID.
	vmID string
}

// newAzureInstallerBackoffKey derives the installer backoff identity for vm.
func newAzureInstallerBackoffKey(vm *azure.VirtualMachine) azureInstallerBackoffKey {
	return azureInstallerBackoffKey{
		resourceID: vm.ID,
		vmID:       vm.VMID,
	}
}

// azureInstallerBackoff adapts the generic installer backoff to Azure virtual
// machines and Azure installation groups.
type azureInstallerBackoff struct {
	*installerBackoff[azureInstallerBackoffKey, *azure.VirtualMachine]
}

// newAzureInstallerBackoff creates an Azure installer backoff keyed by Azure
// resource ID and VM ID.
func newAzureInstallerBackoff(baseDelay time.Duration, jitter retryutils.Jitter) (*azureInstallerBackoff, error) {
	backoff, err := newInstallerBackoff(baseDelay, jitter, newAzureInstallerBackoffKey)
	if err != nil {
		return nil, err
	}
	return &azureInstallerBackoff{
		installerBackoff: backoff,
	}, nil
}

// filter removes backed-off virtual machines from instances and returns their
// completed backoff entries.
func (b *azureInstallerBackoff) filter(instances *server.AzureInstances, t time.Time) []installerBackoffEntry[*azure.VirtualMachine] {
	var removed []installerBackoffEntry[*azure.VirtualMachine]
	instances.Instances, removed = b.installerBackoff.filter(instances.Instances, t)
	return removed
}
