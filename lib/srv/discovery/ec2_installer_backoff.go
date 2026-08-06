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
	"github.com/gravitational/teleport/lib/srv/server"
)

type ec2InstallerBackoffTarget struct {
	accountID string
	region    string
	instance  server.EC2Instance
}

func newEC2InstallerBackoffTarget(group *server.EC2Instances, instance server.EC2Instance) ec2InstallerBackoffTarget {
	return ec2InstallerBackoffTarget{
		accountID: group.AccountID,
		region:    group.Region,
		instance:  instance,
	}
}

func newEC2InstallerBackoffTargetFromResult(result *server.SSMInstallationResult) ec2InstallerBackoffTarget {
	return ec2InstallerBackoffTarget{
		accountID: result.SSMRunEvent.AccountID,
		region:    result.SSMRunEvent.Region,
		instance: server.EC2Instance{
			InstanceID:   result.SSMRunEvent.InstanceID,
			InstanceName: result.InstanceName,
		},
	}
}

type ec2InstallerBackoffKey struct {
	accountID  string
	region     string
	instanceID string
}

func newEC2InstallerBackoffKey(target ec2InstallerBackoffTarget) ec2InstallerBackoffKey {
	return ec2InstallerBackoffKey{
		accountID:  target.accountID,
		region:     target.region,
		instanceID: target.instance.InstanceID,
	}
}

// ec2InstallerBackoff tracks EC2 installation attempts and backs the
// installer off to avoid excessive attempts.
type ec2InstallerBackoff struct {
	*installerBackoff[ec2InstallerBackoffKey, ec2InstallerBackoffTarget]
}

func newEC2InstallerBackoff(baseDelay time.Duration, jitter retryutils.Jitter) (*ec2InstallerBackoff, error) {
	backoff, err := newInstallerBackoff(baseDelay, jitter, newEC2InstallerBackoffKey)
	if err != nil {
		return nil, err
	}
	return &ec2InstallerBackoff{
		installerBackoff: backoff,
	}, nil
}

// filter removes instances with an active backoff from the group and returns
// their entries.
func (b *ec2InstallerBackoff) filter(group *server.EC2Instances, t time.Time) []installerBackoffEntry[ec2InstallerBackoffTarget] {
	targets := make([]ec2InstallerBackoffTarget, 0, len(group.Instances))
	for _, instance := range group.Instances {
		targets = append(targets, newEC2InstallerBackoffTarget(group, instance))
	}

	var removed []installerBackoffEntry[ec2InstallerBackoffTarget]
	targets, removed = b.installerBackoff.filter(targets, t)
	group.Instances = group.Instances[:0]
	for _, target := range targets {
		group.Instances = append(group.Instances, target.instance)
	}
	return removed
}
