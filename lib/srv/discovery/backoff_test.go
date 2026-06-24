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

func TestInstallerBackoffRecordFailedAttempt(t *testing.T) {
	now := time.Now()
	backoff := newInstallerBackoff(time.Minute, nil)
	vm := backoffTestVM("vm-1")

	entry := backoff.recordFailedAttempt(vm, "first-issue", now)
	require.Equal(t, int32(1), entry.failures)
	require.Equal(t, now, entry.lastFailureAt)
	require.Equal(t, "first-issue", entry.issueType)
	require.Same(t, vm, entry.vm)
	require.Equal(t, now.Add(5*time.Minute), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	now = now.Add(time.Minute)
	entry = backoff.recordFailedAttempt(vm, "second-issue", now)
	require.Equal(t, int32(2), entry.failures)
	require.Equal(t, now, entry.lastFailureAt)
	require.Equal(t, "second-issue", entry.issueType)
	require.Same(t, vm, entry.vm)
	require.Equal(t, now.Add(10*time.Minute), entry.retryAfter)
	require.True(t, entry.seenInLastScan)
}

func TestInstallerBackoffDelayBounds(t *testing.T) {
	tests := []struct {
		name      string
		baseDelay time.Duration
		failures  int32
		wantDelay time.Duration
	}{
		{
			name:      "uses minimum base delay",
			baseDelay: time.Minute,
			failures:  1,
			wantDelay: 5 * time.Minute,
		},
		{
			name:      "uses bounded poll interval based delay",
			baseDelay: 7 * time.Minute,
			failures:  2,
			wantDelay: 14 * time.Minute,
		},
		{
			name:      "caps delay at maximum",
			baseDelay: time.Minute,
			failures:  10000,
			wantDelay: maxInstallBackoff,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := newInstallerBackoff(tt.baseDelay, nil)
			require.Equal(t, tt.wantDelay, backoff.delay(tt.failures))
		})
	}
}

func TestInstallerBackoffFilter(t *testing.T) {
	now := time.Now()
	backoff := newInstallerBackoff(time.Minute, nil)

	backedOffVM := backoffTestVM("backed-off")
	retryableVM := backoffTestVM("retryable")
	neverFailedVM := backoffTestVM("never-failed")

	backoff.recordFailedAttempt(retryableVM, "retryable-issue", now)
	backedOffEntry := backoff.recordFailedAttempt(backedOffVM, "backed-off-issue", now.Add(backoff.baseDelay))

	instances := &server.AzureInstances{
		Instances: []*azure.VirtualMachine{
			backedOffVM,
			retryableVM,
			neverFailedVM,
		},
	}

	skipped := backoff.filter(instances, now.Add(backoff.baseDelay+time.Minute))
	notSkipped := make([]string, 0, len(instances.Instances))
	for _, vm := range instances.Instances {
		notSkipped = append(notSkipped, vm.ID)
	}
	require.ElementsMatch(t, []string{retryableVM.ID, neverFailedVM.ID}, notSkipped)
	require.Len(t, skipped, 1)
	require.Equal(t, backedOffEntry, skipped[0])
	require.NotContains(t, backoff.entries, neverFailedVM.ID)
	require.Contains(t, backoff.entries, backedOffVM.ID)
	require.Contains(t, backoff.entries, retryableVM.ID)
	require.True(t, backoff.entries[backedOffEntry.vm.ID].seenInLastScan)
	require.True(t, backoff.entries[retryableVM.ID].seenInLastScan)
}

func TestInstallerBackoffExpireEntries(t *testing.T) {
	now := time.Now()
	backoff := newInstallerBackoff(time.Minute, nil)

	vm1 := backoffTestVM("vm1")
	vm2 := backoffTestVM("vm2")
	vm3 := backoffTestVM("vm3")

	backoff.recordFailedAttempt(vm1, "issue", now)
	backoff.recordFailedAttempt(vm2, "issue", now)
	backoff.recordFailedAttempt(vm3, "issue", now.Add(-2*backoff.baseDelay))
	backoff.entries[vm3.ID].seenInLastScan = false
	require.Contains(t, backoff.entries, vm1.ID)
	require.Contains(t, backoff.entries, vm2.ID)
	require.Contains(t, backoff.entries, vm3.ID)
	require.True(t, backoff.entries[vm3.ID].retryable(now))
	backoff.expireEntries(now)
	require.Contains(t, backoff.entries, vm1.ID)
	require.Contains(t, backoff.entries, vm2.ID)
	require.NotContains(t, backoff.entries, vm3.ID, "vm3 was not seen in last scan and expired, so it should have been removed")
	require.False(t, backoff.entries[vm1.ID].seenInLastScan)
	require.False(t, backoff.entries[vm2.ID].seenInLastScan)

	now = now.Add(backoff.baseDelay + time.Second)
	instances := &server.AzureInstances{
		Instances: []*azure.VirtualMachine{vm1},
	}
	_ = backoff.filter(instances, now)
	require.Contains(t, backoff.entries, vm1.ID)
	require.Contains(t, backoff.entries, vm2.ID)
	require.True(t, backoff.entries[vm1.ID].seenInLastScan)
	require.False(t, backoff.entries[vm2.ID].seenInLastScan)
	require.True(t, backoff.entries[vm1.ID].retryable(now))
	require.True(t, backoff.entries[vm2.ID].retryable(now))

	backoff.expireEntries(now)
	require.Contains(t, backoff.entries, vm1.ID)
	require.NotContains(t, backoff.entries, vm2.ID, "vm2 was not seen in last scan and expired, so it should have been removed")
}

func backoffTestVM(name string) *azure.VirtualMachine {
	return &azure.VirtualMachine{
		ID:   "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/virtualMachines/" + name,
		Name: name,
		VMID: name + "-vmid",
	}
}
