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

	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/server"
)

func TestInstallerBackoffRecordAttempt(t *testing.T) {
	now := time.Now()
	baseDelay := time.Minute
	backoff, err := newInstallerBackoff(baseDelay, nil)
	require.NoError(t, err)
	_, vm := backoffTestVM("vm-1")

	entry := backoff.recordFailedAttempt(vm, "first-issue", now)
	require.Equal(t, int32(1), entry.attempts)
	require.Equal(t, now, entry.lastAttemptAt)
	require.Equal(t, "first-issue", entry.issueType)
	require.Same(t, vm, entry.vm)
	require.Equal(t, now.Add(baseDelay), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	now = now.Add(time.Hour)
	entry = backoff.recordFailedAttempt(vm, "second-issue", now)
	require.Equal(t, int32(2), entry.attempts)
	require.Equal(t, now, entry.lastAttemptAt)
	require.Equal(t, "second-issue", entry.issueType)
	require.Same(t, vm, entry.vm)
	require.Equal(t, now.Add(2*baseDelay), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	now = now.Add(time.Hour)
	entry = backoff.recordSuccessfulAttempt(vm, now)
	require.Equal(t, int32(3), entry.attempts)
	require.Equal(t, now, entry.lastAttemptAt)
	require.Empty(t, entry.issueType)
	require.Same(t, vm, entry.vm)
	require.Equal(t, now.Add(4*baseDelay), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	backoff.reset()
	require.Empty(t, backoff.entries)
}

func TestInstallerBackoffDelayBounds(t *testing.T) {
	tests := []struct {
		name      string
		baseDelay time.Duration
		attempts  int32
		wantDelay time.Duration
	}{
		{
			name:      "uses minimum base delay",
			baseDelay: time.Second,
			attempts:  1,
			wantDelay: 1 * time.Minute,
		},
		{
			name:      "uses bounded poll interval based delay",
			baseDelay: 7 * time.Minute,
			attempts:  2,
			wantDelay: 14 * time.Minute,
		},
		{
			name:      "caps delay at maximum",
			baseDelay: time.Minute,
			attempts:  10000,
			wantDelay: maxInstallBackoff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff, err := newInstallerBackoff(tt.baseDelay, nil)
			require.NoError(t, err)
			_, vm := backoffTestVM("backed-off")
			var entry *installerBackoffEntry
			for range tt.attempts {
				entry = backoff.recordAttemptLocked(vm, time.Now())
			}
			require.Equal(t, tt.attempts, entry.attempts)
			require.Equal(t, tt.wantDelay, entry.retry.Duration())
		})
	}
}

func TestInstallerBackoffFilter(t *testing.T) {
	now := time.Now()
	baseDelay := time.Minute
	backoff, err := newInstallerBackoff(baseDelay, nil)
	require.NoError(t, err)

	vm1Key, vm1 := backoffTestVM("backed-off")
	vm2Key, vm2 := backoffTestVM("retryable")
	vm3Key, vm3 := backoffTestVM("never-failed")

	backoff.recordFailedAttempt(vm2, "retryable-issue", now)
	backedOffEntry := backoff.recordFailedAttempt(vm1, "backed-off-issue", now.Add(baseDelay))

	instances := &server.AzureInstances{
		Instances: []*azure.VirtualMachine{
			vm1,
			vm2,
			vm3,
		},
	}

	skipped := backoff.filter(instances, now.Add(2*baseDelay))
	require.Contains(t, backoff.entries, vm1Key)
	require.Contains(t, backoff.entries, vm2Key)
	require.Contains(t, backoff.entries, vm3Key)

	notSkipped := make([]string, 0, len(instances.Instances))
	for _, vm := range instances.Instances {
		notSkipped = append(notSkipped, vm.ID)
	}
	require.ElementsMatch(t, []string{vm2.ID, vm3.ID}, notSkipped)
	require.Len(t, skipped, 1)
	require.Equal(t, backedOffEntry, skipped[0])
	require.False(t, backoff.entries[vm3Key].isFailedAttempt())
}

func TestInstallerBackoffExpireEntries(t *testing.T) {
	now := time.Now()
	baseDelay := time.Minute
	backoff, err := newInstallerBackoff(baseDelay, nil)
	require.NoError(t, err)

	vm1Key, vm1 := backoffTestVM("vm1")
	vm2Key, vm2 := backoffTestVM("vm2")
	vm3Key, vm3 := backoffTestVM("vm3")

	backoff.recordFailedAttempt(vm1, "issue", now)
	backoff.recordFailedAttempt(vm2, "issue", now)
	backoff.recordFailedAttempt(vm3, "issue", now.Add(-2*baseDelay))
	backoff.entries[vm3Key].seenInLastScan = false
	require.Contains(t, backoff.entries, vm1Key)
	require.Contains(t, backoff.entries, vm2Key)
	require.Contains(t, backoff.entries, vm3Key)
	require.True(t, backoff.entries[vm3Key].retryable(now))
	backoff.expireEntries(now)
	require.Contains(t, backoff.entries, vm1Key)
	require.Contains(t, backoff.entries, vm2Key)
	require.NotContains(t, backoff.entries, vm3Key, "vm3 was not seen in last scan and expired, so it should have been removed")
	require.False(t, backoff.entries[vm1Key].seenInLastScan)
	require.False(t, backoff.entries[vm2Key].seenInLastScan)

	now = now.Add(baseDelay + time.Second)
	instances := &server.AzureInstances{
		Instances: []*azure.VirtualMachine{vm1},
	}
	_ = backoff.filter(instances, now)
	require.Contains(t, backoff.entries, vm1Key)
	require.Contains(t, backoff.entries, vm2Key)
	require.True(t, backoff.entries[vm1Key].seenInLastScan)
	require.False(t, backoff.entries[vm2Key].seenInLastScan)
	require.True(t, backoff.entries[vm1Key].retryable(now))
	require.True(t, backoff.entries[vm2Key].retryable(now))

	backoff.expireEntries(now)
	require.Contains(t, backoff.entries, vm1Key)
	require.NotContains(t, backoff.entries, vm2Key, "vm2 was not seen in last scan and expired, so it should have been removed")
}

func backoffTestVM(name string) (installerBackoffKey, *azure.VirtualMachine) {
	vm := &azure.VirtualMachine{
		ID:   "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/virtualMachines/" + name,
		Name: name,
		VMID: name + "-vmid",
	}
	key := newInstallerBackoffKey(vm)
	return key, vm
}
