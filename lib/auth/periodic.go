/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package auth

import (
	"slices"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	vc "github.com/gravitational/teleport/lib/versioncontrol"
)

const enrollmentMessage = "Some agents are outdated and would benefit from enrollment in automatic updates." +
	" See https://goteleport.com/docs/upgrading/ for more details about updating Teleport." +
	" Use 'tctl alerts' command to manage alerts."

// upgradeEnrollPeriodic is a periodic operation that aggregates per-version counts of instances
// by whether or not they are enrolled in automatic upgrades and generates a prompt to enroll
// instances
type upgradeEnrollPeriodic struct {
	// authVersion is used to identify out dated agents
	authVersion string
	// enrolled/unenrolled per-version counts
	enrolled, unenrolled map[string]int
}

func newUpgradeEnrollPeriodic() *upgradeEnrollPeriodic {
	return &upgradeEnrollPeriodic{
		authVersion: vc.Normalize(teleport.SemVersion.String()),
		enrolled:    make(map[string]int),
		unenrolled:  make(map[string]int),
	}
}

// VisitInstance adds an instance to ongoing aggregations.
func (u *upgradeEnrollPeriodic) VisitInstance(instance proto.UpstreamInventoryHello) {
	ver := vc.Normalize(instance.GetVersion())
	if !semver.IsValid(ver) {
		return
	}

	if instance.GetExternalUpgrader() == "" {
		u.unenrolled[ver]++
	} else {
		u.enrolled[ver]++
	}
}

// GenerateEnrollPrompt generates a prompt suggesting enrollment of unenrolled instances
func (u *upgradeEnrollPeriodic) GenerateEnrollPrompt() (msg string, prompt bool) {
	for version, count := range u.unenrolled {
		// If an instance is running on an older major version than the control plane
		// and it is not enrolled in automatic updates, then generate the enrollment
		// notice.
		if count > 0 && semver.Major(version) < semver.Major(u.authVersion) {
			return enrollmentMessage, true
		}
	}
	return "", false
}

// instanceMetricsPeriodic is an aggregator for general instance metrics.
type instanceMetricsPeriodic struct {
	metadata []instanceMetadata
}

// instanceMetadata contains instance metadata to be exported.
type instanceMetadata struct {
	// version specifies the version of the Teleport instance
	version string
	// installMethod specifies the Teleport agent installation method
	installMethod string
	// upgraderType specifies the upgrader type
	upgraderType string
	// upgraderVersion specifies the upgrader version
	upgraderVersion string
}

func newInstanceMetricsPeriodic() *instanceMetricsPeriodic {
	return &instanceMetricsPeriodic{
		metadata: []instanceMetadata{},
	}
}

func (i *instanceMetricsPeriodic) VisitInstance(instance proto.UpstreamInventoryHello, metadata proto.UpstreamInventoryAgentMetadata) {
	// Sort install methods if multiple methods are specified.
	installMethod := "unknown"
	installMethods := append([]string{}, metadata.GetInstallMethods()...)
	if len(installMethods) > 0 {
		slices.Sort(installMethods)
		installMethod = strings.Join(installMethods, ",")
	}

	iMetadata := instanceMetadata{
		version:         instance.GetVersion(),
		installMethod:   installMethod,
		upgraderType:    instance.GetExternalUpgrader(),
		upgraderVersion: instance.GetExternalUpgraderVersion(),
	}
	i.metadata = append(i.metadata, iMetadata)
}

type registeredAgent struct {
	version          string
	automaticUpdates string
}

// RegisteredAgentsCount returns the count registered agents count.
func (i *instanceMetricsPeriodic) RegisteredAgentsCount() map[registeredAgent]int {
	result := make(map[registeredAgent]int)
	for _, metadata := range i.metadata {
		automaticUpdates := "false"
		if metadata.upgraderType != "" {
			automaticUpdates = "true"
		}

		agent := registeredAgent{
			version:          metadata.version,
			automaticUpdates: automaticUpdates,
		}
		result[agent]++
	}
	return result
}

// InstallMethodCounts returns the count of each install method.
func (i *instanceMetricsPeriodic) InstallMethodCounts() map[string]int {
	installMethodCount := make(map[string]int)
	for _, metadata := range i.metadata {
		installMethodCount[metadata.installMethod]++
	}
	return installMethodCount
}

type upgrader struct {
	upgraderType string
	version      string
}

// UpgraderCounts returns the count for the different upgrader version and type combinations.
func (i *instanceMetricsPeriodic) UpgraderCounts() map[upgrader]int {
	result := make(map[upgrader]int)
	for _, metadata := range i.metadata {
		// Do not count the instance if a type is not specified
		if metadata.upgraderType == "" {
			continue
		}

		upgrader := upgrader{
			upgraderType: metadata.upgraderType,
			version:      metadata.upgraderVersion,
		}
		result[upgrader]++
	}
	return result
}

// TotalEnrolledInUpgrades gets the total number of instances that have some upgrader defined.
func (i *instanceMetricsPeriodic) TotalEnrolledInUpgrades() int {
	var total int
	for _, metadata := range i.metadata {
		if metadata.upgraderType != "" {
			total++
		}
	}
	return total
}

// TotalInstances gets the total number of known instances.
func (i *instanceMetricsPeriodic) TotalInstances() int {
	return len(i.metadata)
}
