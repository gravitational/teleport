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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/srv/server"
)

func TestEC2InstallerBackoffRecordAttempt(t *testing.T) {
	now := time.Now()
	baseDelay := time.Minute
	backoff, err := newEC2InstallerBackoff(baseDelay, nil)
	require.NoError(t, err)
	target := ec2BackoffTestTarget("account-1", "region-1", "instance-1")

	entry := backoff.recordFailedAttempt(target, "first-issue", now)
	require.Equal(t, int32(1), entry.attempts)
	require.Equal(t, now, entry.lastAttemptAt)
	require.Equal(t, "first-issue", entry.issueType)
	require.Equal(t, target, entry.target)
	require.Equal(t, now.Add(baseDelay), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	now = now.Add(time.Hour)
	entry = backoff.recordFailedAttempt(target, "second-issue", now)
	require.Equal(t, int32(2), entry.attempts)
	require.Equal(t, now, entry.lastAttemptAt)
	require.Equal(t, "second-issue", entry.issueType)
	require.Equal(t, target, entry.target)
	require.Equal(t, now.Add(2*baseDelay), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	now = now.Add(time.Hour)
	entry = backoff.recordSuccessfulAttempt(target, now)
	require.Equal(t, int32(3), entry.attempts)
	require.Equal(t, now, entry.lastAttemptAt)
	require.Empty(t, entry.issueType)
	require.Equal(t, target, entry.target)
	require.Equal(t, now.Add(4*baseDelay), entry.retryAfter)
	require.True(t, entry.seenInLastScan)

	backoff.reset()
	require.Empty(t, backoff.entries)
}

func TestEC2InstallerBackoffKeyScope(t *testing.T) {
	backoff, err := newEC2InstallerBackoff(time.Minute, nil)
	require.NoError(t, err)

	targets := []ec2InstallerBackoffTarget{
		ec2BackoffTestTarget("account-1", "region-1", "instance-1"),
		ec2BackoffTestTarget("account-2", "region-1", "instance-1"),
		ec2BackoffTestTarget("account-1", "region-2", "instance-1"),
	}
	for _, target := range targets {
		backoff.recordFailedAttempt(target, "issue", time.Now())
	}

	require.Len(t, backoff.entries, len(targets))
	for _, target := range targets {
		require.Contains(t, backoff.entries, newEC2InstallerBackoffKey(target))
	}
}

func TestEC2InstallerBackoffFilter(t *testing.T) {
	now := time.Now()
	baseDelay := time.Minute
	backoff, err := newEC2InstallerBackoff(baseDelay, nil)
	require.NoError(t, err)

	group := &server.EC2Instances{
		AccountID: "account-1",
		Region:    "region-1",
		Instances: []server.EC2Instance{
			{InstanceID: "backed-off", InstanceName: "Backed off"},
			{InstanceID: "retryable", InstanceName: "Retryable"},
			{InstanceID: "never-attempted", InstanceName: "Never attempted"},
		},
	}
	retryableTarget := newEC2InstallerBackoffTarget(group, group.Instances[1])
	backedOffTarget := newEC2InstallerBackoffTarget(group, group.Instances[0])
	backoff.recordFailedAttempt(retryableTarget, "retryable-issue", now)
	backedOffEntry := backoff.recordFailedAttempt(backedOffTarget, "backed-off-issue", now.Add(baseDelay))

	skipped := backoff.filter(group, now.Add(2*baseDelay))

	remainingIDs := make([]string, 0, len(group.Instances))
	for _, instance := range group.Instances {
		remainingIDs = append(remainingIDs, instance.InstanceID)
	}
	require.ElementsMatch(t, []string{"retryable", "never-attempted"}, remainingIDs)
	require.Equal(t, []installerBackoffEntry[ec2InstallerBackoffTarget]{backedOffEntry}, skipped)
	require.Len(t, backoff.entries, 3)
	require.False(t, backoff.entries[ec2InstallerBackoffKey{
		accountID:  group.AccountID,
		region:     group.Region,
		instanceID: "never-attempted",
	}].isFailedAttempt())
}

func TestEC2InstallerBackoffExpireEntries(t *testing.T) {
	now := time.Now()
	baseDelay := time.Minute
	backoff, err := newEC2InstallerBackoff(baseDelay, nil)
	require.NoError(t, err)

	target1 := ec2BackoffTestTarget("account-1", "region-1", "instance-1")
	target2 := ec2BackoffTestTarget("account-1", "region-1", "instance-2")
	target3 := ec2BackoffTestTarget("account-1", "region-1", "instance-3")
	key1 := newEC2InstallerBackoffKey(target1)
	key2 := newEC2InstallerBackoffKey(target2)
	key3 := newEC2InstallerBackoffKey(target3)

	backoff.recordFailedAttempt(target1, "issue", now)
	backoff.recordFailedAttempt(target2, "issue", now)
	backoff.recordFailedAttempt(target3, "issue", now.Add(-2*baseDelay))
	backoff.entries[key3].seenInLastScan = false

	backoff.expireEntries(now)
	require.Contains(t, backoff.entries, key1)
	require.Contains(t, backoff.entries, key2)
	require.NotContains(t, backoff.entries, key3)
	require.False(t, backoff.entries[key1].seenInLastScan)
	require.False(t, backoff.entries[key2].seenInLastScan)

	now = now.Add(baseDelay + time.Second)
	group := &server.EC2Instances{
		AccountID: target1.accountID,
		Region:    target1.region,
		Instances: []server.EC2Instance{target1.instance},
	}
	_ = backoff.filter(group, now)
	backoff.expireEntries(now)
	require.Contains(t, backoff.entries, key1)
	require.NotContains(t, backoff.entries, key2)
}

func TestEC2InstallerBackoffConcurrentResults(t *testing.T) {
	backoff, err := newEC2InstallerBackoff(time.Minute, nil)
	require.NoError(t, err)
	target := ec2BackoffTestTarget("account-1", "region-1", "instance-1")

	const attempts = 50
	var wg sync.WaitGroup
	for range attempts {
		wg.Go(func() {
			backoff.recordFailedAttempt(target, "issue", time.Now())
		})
	}
	wg.Wait()

	entry := backoff.entries[newEC2InstallerBackoffKey(target)]
	require.NotNil(t, entry)
	require.Equal(t, int32(attempts), entry.attempts)
}

func ec2BackoffTestTarget(accountID, region, instanceID string) ec2InstallerBackoffTarget {
	return ec2InstallerBackoffTarget{
		accountID: accountID,
		region:    region,
		instance: server.EC2Instance{
			InstanceID:   instanceID,
			InstanceName: instanceID + "-name",
		},
	}
}
