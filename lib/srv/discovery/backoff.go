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

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
)

const (
	maxInstallBackoff = 6 * time.Hour
	minInstallBackoff = 5 * time.Minute
)

type installerBackoffEntry struct {
	vm *azure.VirtualMachine
	// issueType is the latest installation issue for this entry.
	issueType string
	// failures is the count of failed installation attempts for this entry.
	failures int32
	// lastFailureAt is the time of the last failure.
	lastFailureAt time.Time
	// retryAfter is the time after which the installation can be retried.
	retryAfter time.Time
}

// retryable returns true if the entry can be retried.
func (e *installerBackoffEntry) retryable(t time.Time) bool {
	return e == nil || t.After(e.retryAfter)
}

// installerBackoff tracks VMs that failed to install and backs the
// installer off to avoid excessive installation attempts.
type installerBackoff struct {
	baseDelay time.Duration
	maxDelay  time.Duration
	jitter    retryutils.Jitter

	mu sync.Mutex
	// entries is a map of failed installation attempts, by VM ID.
	entries map[string]*installerBackoffEntry
	// lastSeen is the set of VM IDs that the backoff has seen in the last discovery scan.
	// These are the VMs that were discovered and not already enrolled.
	lastSeen map[string]struct{}
}

// newInstallerBackoff creates a new [*installerBackoff].
func newInstallerBackoff(baseDelay time.Duration, jitter retryutils.Jitter) *installerBackoff {
	return &installerBackoff{
		entries:  make(map[string]*installerBackoffEntry),
		lastSeen: make(map[string]struct{}),
		// bound the base delay to [minInstallBackoff, maxInstallBackoff/4]
		baseDelay: min(
			max(baseDelay, minInstallBackoff),
			maxInstallBackoff/4,
		),
		maxDelay: maxInstallBackoff,
		jitter:   jitter,
	}
}

func (b *installerBackoff) delay(failures int32) time.Duration {
	delay := b.baseDelay
	for range failures - 1 {
		if delay >= b.maxDelay {
			break
		}
		delay *= 2
	}
	delay = min(delay, b.maxDelay)
	if b.jitter != nil {
		delay = b.jitter(delay)
	}
	return delay
}

// filter filters out instances that are should be backed off and returns the
// list of entries that were removed.
func (b *installerBackoff) filter(instances *server.AzureInstances, t time.Time) []installerBackoffEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, vm := range instances.Instances {
		b.lastSeen[vm.ID] = struct{}{}
	}

	if len(b.entries) == 0 {
		return nil
	}

	var removed []installerBackoffEntry
	instances.Instances = slices.DeleteFunc(instances.Instances, func(vm *azure.VirtualMachine) bool {
		if entry := b.entries[vm.ID]; !entry.retryable(t) {
			removed = append(removed, *entry)
			return true
		}
		return false
	})
	return removed
}

// resetLastSeen resets the set of VMs that need to be installed.
func (b *installerBackoff) resetLastSeen() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lastSeen = make(map[string]struct{})
}

// recordFailedAttempt records an entry in the backoff for a failed VM
// installation attempt and returns its backoff entry.
func (b *installerBackoff) recordFailedAttempt(vm *azure.VirtualMachine, issueType string, t time.Time) installerBackoffEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry := b.entries[vm.ID]
	if entry == nil {
		entry = &installerBackoffEntry{}
		b.entries[vm.ID] = entry
	}
	entry.vm = vm
	entry.issueType = issueType
	entry.failures++
	entry.lastFailureAt = t
	entry.retryAfter = t.Add(b.delay(entry.failures))
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
func (b *installerBackoff) expireEntries(t time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for key, entry := range b.entries {
		if _, seen := b.lastSeen[key]; !seen && entry.retryable(t) {
			delete(b.entries, key)
		}
	}
}
