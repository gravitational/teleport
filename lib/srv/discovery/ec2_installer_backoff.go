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
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/srv/server"
)

// ec2InstallerBackoffTarget combines an instance with the context needed to
// derive its backoff identity while retaining current metadata for reporting.
type ec2InstallerBackoffTarget struct {
	accountID       string
	region          string
	enrollmentScope ec2InstallerBackoffScope
	instance        server.EC2Instance
}

// ec2InstallerBackoffScope identifies the effective enrollment configuration.
// DiscoveryConfig name and matcher position are excluded so equivalent
// matchers share backoff state.
type ec2InstallerBackoffScope struct {
	documentName  string
	integration   string
	assumeRoleARN string
	externalID    string
	enrollMode    string
	parametersKey string
}

// ec2InstallerBackoffKey identifies a physical EC2 instance under one
// effective enrollment configuration.
type ec2InstallerBackoffKey struct {
	accountID       string
	region          string
	instanceID      string
	enrollmentScope ec2InstallerBackoffScope
}

// ec2InstallerBackoff tracks completed EC2 installation attempts.
type ec2InstallerBackoff struct {
	*installerBackoff[ec2InstallerBackoffKey, ec2InstallerBackoffTarget]
}

// newEC2InstallerBackoffKey derives the backoff identity for target.
func newEC2InstallerBackoffKey(target ec2InstallerBackoffTarget) ec2InstallerBackoffKey {
	return ec2InstallerBackoffKey{
		accountID:       target.accountID,
		region:          target.region,
		instanceID:      target.instance.InstanceID,
		enrollmentScope: target.enrollmentScope,
	}
}

// newEC2InstallerBackoffTarget derives a target from an installation group and
// one of its instances.
func newEC2InstallerBackoffTarget(group *server.EC2Instances, instance server.EC2Instance) ec2InstallerBackoffTarget {
	return newEC2InstallerBackoffTargetWithScope(group, instance, newEC2InstallerBackoffScope(group))
}

// newEC2InstallerBackoffTargetWithScope reuses a precomputed scope when
// constructing multiple targets from the same group.
func newEC2InstallerBackoffTargetWithScope(group *server.EC2Instances, instance server.EC2Instance, scope ec2InstallerBackoffScope) ec2InstallerBackoffTarget {
	return ec2InstallerBackoffTarget{
		accountID:       group.AccountID,
		region:          group.Region,
		enrollmentScope: scope,
		instance:        instance,
	}
}

// newEC2InstallerBackoffTargetFromResult combines result identity with the
// originating group's enrollment configuration.
func newEC2InstallerBackoffTargetFromResult(group *server.EC2Instances, result *server.SSMInstallationResult) ec2InstallerBackoffTarget {
	return newEC2InstallerBackoffTarget(group, server.EC2Instance{
		InstanceID:   result.SSMRunEvent.InstanceID,
		InstanceName: result.InstanceName,
	})
}

// newEC2InstallerBackoffScope returns the effective enrollment configuration.
// Equivalent matchers share a scope even when their DiscoveryConfig names or
// parameter map insertion order differ.
func newEC2InstallerBackoffScope(group *server.EC2Instances) ec2InstallerBackoffScope {
	return ec2InstallerBackoffScope{
		documentName:  group.DocumentName,
		integration:   group.Integration,
		assumeRoleARN: group.AssumeRoleARN,
		externalID:    group.ExternalID,
		enrollMode:    group.EnrollMode.String(),
		parametersKey: canonicalEC2InstallerParams(group.Parameters),
	}
}

// canonicalEC2InstallerParams returns a stable, collision-safe representation
// of params independent of map iteration order.
func canonicalEC2InstallerParams(params map[string]string) string {
	var canonical strings.Builder
	for _, key := range slices.Sorted(maps.Keys(params)) {
		writeLengthPrefixedString(&canonical, key)
		writeLengthPrefixedString(&canonical, params[key])
	}
	return canonical.String()
}

func writeLengthPrefixedString(builder *strings.Builder, value string) {
	builder.WriteString(strconv.Itoa(len(value)))
	builder.WriteByte(':')
	builder.WriteString(value)
}

// newEC2InstallerBackoff creates an EC2 installer backoff keyed by AWS
// account, region, instance ID, and effective enrollment configuration.
func newEC2InstallerBackoff(baseDelay time.Duration, jitter retryutils.Jitter) (*ec2InstallerBackoff, error) {
	backoff, err := newInstallerBackoff(baseDelay, jitter, newEC2InstallerBackoffKey)
	if err != nil {
		return nil, err
	}
	return &ec2InstallerBackoff{
		installerBackoff: backoff,
	}, nil
}

// filter removes instances with active completed-attempt cooldowns from group
// and returns the corresponding entries.
func (b *ec2InstallerBackoff) filter(group *server.EC2Instances, t time.Time) []installerBackoffEntry[ec2InstallerBackoffTarget] {
	scope := newEC2InstallerBackoffScope(group)
	var targets []ec2InstallerBackoffTarget
	for _, instance := range group.Instances {
		targets = append(targets, newEC2InstallerBackoffTargetWithScope(group, instance, scope))
	}

	retained, skipped := b.installerBackoff.filter(targets, t)
	group.Instances = group.Instances[:0]
	for _, target := range retained {
		group.Instances = append(group.Instances, target.instance)
	}
	return skipped
}
