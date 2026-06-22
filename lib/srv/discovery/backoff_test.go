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

	now = now.Add(time.Minute)
	entry = backoff.recordFailedAttempt(vm, "second-issue", now)
	require.Equal(t, int32(2), entry.failures)
	require.Equal(t, now, entry.lastFailureAt)
	require.Equal(t, "second-issue", entry.issueType)
	require.Same(t, vm, entry.vm)
	require.Equal(t, now.Add(10*time.Minute), entry.retryAfter)
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
	require.Contains(t, backoff.lastSeen, backedOffVM.ID)
	require.Contains(t, backoff.lastSeen, retryableVM.ID)
	require.Contains(t, backoff.lastSeen, neverFailedVM.ID)
}

func TestInstallerBackoffExpireEntries(t *testing.T) {
	now := time.Now()
	backoff := newInstallerBackoff(time.Minute, nil)

	seenVM := backoffTestVM("attempted")
	unseenExpiredVM := backoffTestVM("expired")
	unseenUnexpiredVM := backoffTestVM("still-backed-off")

	backoff.recordFailedAttempt(seenVM, "attempted-issue", now)
	backoff.recordFailedAttempt(unseenExpiredVM, "expired-issue", now)
	backoff.recordFailedAttempt(unseenUnexpiredVM, "still-backed-off-issue", now.Add(backoff.baseDelay))

	instances := &server.AzureInstances{
		Instances: []*azure.VirtualMachine{seenVM},
	}
	_ = backoff.filter(instances, now.Add(time.Minute))

	backoff.expireEntries(now.Add(backoff.baseDelay * 2))
	require.Contains(t, backoff.entries, seenVM.ID)
	require.NotContains(t, backoff.entries, unseenExpiredVM.ID)
	require.Contains(t, backoff.entries, unseenUnexpiredVM.ID)

	backoff.resetLastSeen()
	require.Empty(t, backoff.lastSeen)
	require.NotEmpty(t, backoff.entries)
	backoff.expireEntries(now.Add(backoff.baseDelay * 3))
	require.Empty(t, backoff.entries)
	require.Empty(t, backoff.lastSeen)
}

func backoffTestVM(name string) *azure.VirtualMachine {
	return &azure.VirtualMachine{
		ID:   "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/virtualMachines/" + name,
		Name: name,
		VMID: name + "-vmid",
	}
}
