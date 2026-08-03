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
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
)

const (
	maxInstallBackoff = 3 * time.Hour
	minInstallBackoff = time.Minute
)

type azureInstallerBackoffEntry struct {
	vm *azure.VirtualMachine
	// issueType is the latest installation issue for this entry.
	issueType string
	// attempts is the count of installation attempts for this entry.
	attempts int32
	// lastAttemptAt is the time of the last attempt.
	lastAttemptAt time.Time
	// retryAfter is the time after which the installation can be retried.
	retryAfter time.Time
	// seenInLastScan indicates that the VM was seen in the last discovery scan.
	// These are the VMs that were discovered and not already enrolled.
	seenInLastScan bool
	// retry tracks the attempts and calculates the retry backoff duration.
	retry retryutils.Retry
}

// retryable returns true if the entry can be retried.
func (e *azureInstallerBackoffEntry) retryable(t time.Time) bool {
	return t.After(e.retryAfter)
}

func (e *azureInstallerBackoffEntry) isFailedAttempt() bool {
	return e.issueType != ""
}

type azureInstallerBackoffKey struct {
	// resourceID is the path based resource ID Azure, e.g., /subscriptions/<sub-id>/resourceGroups/<rg-id>/providers/Microsoft.Compute/virtualMachines/<vm-name>
	// It is not necessarily unique.
	resourceID string
	// vmID is a VM UUID. In practice this ID is not empty.
	// In case it is empty, we also include the resource ID.
	vmID string
}

func newAzureInstallerBackoffKey(vm *azure.VirtualMachine) azureInstallerBackoffKey {
	return azureInstallerBackoffKey{
		resourceID: vm.ID,
		vmID:       vm.VMID,
	}
}

// azureInstallerBackoff tracks VM installation attempts and backs the installer off to
// avoid excessive attempts.
type azureInstallerBackoff struct {
	retry *retryutils.RetryV2

	mu sync.Mutex
	// entries is a map of installation attempts, by VM ID.
	entries map[azureInstallerBackoffKey]*azureInstallerBackoffEntry
}

// newAzureInstallerBackoff creates a new [*azureInstallerBackoff].
func newAzureInstallerBackoff(baseDelay time.Duration, jitter retryutils.Jitter) (*azureInstallerBackoff, error) {
	// bound the base delay to [minInstallBackoff, maxInstallBackoff/4]
	baseDelay = min(
		max(baseDelay, minInstallBackoff),
		maxInstallBackoff/4,
	)
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(baseDelay),
		Max:    maxInstallBackoff,
		Jitter: jitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &azureInstallerBackoff{
		retry:   retry,
		entries: make(map[azureInstallerBackoffKey]*azureInstallerBackoffEntry),
	}, nil
}

// filter filters out instances that are should be backed off and returns the
// list of entries that were removed.
func (b *azureInstallerBackoff) filter(instances *server.AzureInstances, t time.Time) []azureInstallerBackoffEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	var removed []azureInstallerBackoffEntry
	instances.Instances = slices.DeleteFunc(instances.Instances, func(vm *azure.VirtualMachine) bool {
		entry := b.addLocked(vm)
		entry.seenInLastScan = true
		if entry.retryable(t) {
			return false
		}
		removed = append(removed, *entry)
		return true
	})
	return removed
}

func (b *azureInstallerBackoff) addLocked(vm *azure.VirtualMachine) *azureInstallerBackoffEntry {
	key := newAzureInstallerBackoffKey(vm)
	entry := b.entries[key]
	if entry == nil {
		entry = &azureInstallerBackoffEntry{
			retry: b.retry.Clone(),
		}
		b.entries[key] = entry
	}
	entry.vm = vm
	return entry
}

func (b *azureInstallerBackoff) recordAttemptLocked(vm *azure.VirtualMachine, t time.Time) *azureInstallerBackoffEntry {
	entry := b.addLocked(vm)
	entry.retry.Inc()
	entry.attempts++
	entry.lastAttemptAt = t
	entry.retryAfter = t.Add(entry.retry.Duration())
	entry.seenInLastScan = true
	return entry
}

func (b *azureInstallerBackoff) recordSuccessfulAttempt(vm *azure.VirtualMachine, t time.Time) azureInstallerBackoffEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.recordAttemptLocked(vm, t)
	entry.issueType = ""
	return *entry
}

// recordFailedAttempt records an entry in the backoff for a failed VM
// installation attempt and returns its backoff entry.
func (b *azureInstallerBackoff) recordFailedAttempt(vm *azure.VirtualMachine, issueType string, t time.Time) azureInstallerBackoffEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.recordAttemptLocked(vm, t)
	entry.issueType = issueType
	return *entry
}

// expireEntries removes all entries that were not attempted in the last
// discovery scan for which the backoff period has elapsed.
// If discovery no longer matches a VM or if the VM is enrolled and will not be
// attempted again, then that VM must eventually be removed from the backoff to
// bound memory growth.
// Undiscovered entries that have not elapsed the retry period are kept around
// to handle the case where discovery config is updated to match a VM that was
// failing and should still be backed off.
func (b *azureInstallerBackoff) expireEntries(t time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for key, entry := range b.entries {
		if !entry.seenInLastScan && entry.retryable(t) {
			delete(b.entries, key)
		} else {
			entry.seenInLastScan = false
		}
	}
}

// reset clears all backoff entries.
func (b *azureInstallerBackoff) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make(map[azureInstallerBackoffKey]*azureInstallerBackoffEntry)
}
