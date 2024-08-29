/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package discovery

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	libevents "github.com/gravitational/teleport/lib/events"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
	"github.com/gravitational/teleport/lib/srv/server"
)

// updateDiscoveryConfigStatus updates the DiscoveryConfig Status field with the current in-memory status.
// The status will be updated with the following matchers:
// - AWS Sync (TAG) status
// - AWS EC2 Auto Discover status
func (s *Server) updateDiscoveryConfigStatus(discoveryConfigName string) {
	discoveryConfigStatus := discoveryconfig.Status{
		State:                          discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
		LastSyncTime:                   s.clock.Now(),
		IntegrationDiscoveredResources: make(map[string]*discoveryconfigv1.IntegrationDiscoveredSummary),
	}

	// Merge AWS Sync (TAG) status
	discoveryConfigStatus = s.awsSyncStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

	// Merge AWS EC2 Instances (auto discovery) status
	discoveryConfigStatus = s.awsEC2ResourcesStatus.mergeEC2IntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	_, err := s.AccessPoint.UpdateDiscoveryConfigStatus(ctx, discoveryConfigName, discoveryConfigStatus)
	switch {
	case trace.IsNotImplemented(err):
		s.Log.Warn("UpdateDiscoveryConfigStatus method is not implemented in Auth Server. Please upgrade it to a recent version.")
	case err != nil:
		s.Log.WithError(err).WithField("discovery_config_name", discoveryConfigName).Info("Error updating discovery config status")
	}
}

// awsSyncStatus contains all the status for aws_sync Fetchers grouped by DiscoveryConfig.
type awsSyncStatus struct {
	mu sync.RWMutex
	// awsSyncResults maps the DiscoveryConfig name to a aws_sync result.
	// Each DiscoveryConfig might have multiple `aws_sync` matchers.
	awsSyncResults map[string][]awsSyncResult
}

// awsSyncResult stores the result of the aws_sync Matchers for a given DiscoveryConfig.
type awsSyncResult struct {
	// state is the State for the DiscoveryConfigStatus.
	// Allowed values are:
	// - DISCOVERY_CONFIG_STATE_SYNCING
	// - DISCOVERY_CONFIG_STATE_ERROR
	// - DISCOVERY_CONFIG_STATE_RUNNING
	state               string
	errorMessage        *string
	lastSyncTime        time.Time
	discoveredResources uint64
}

func (d *awsSyncStatus) iterationFinished(fetchers []aws_sync.AWSSync, pushErr error, lastUpdate time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.awsSyncResults = make(map[string][]awsSyncResult)
	for _, fetcher := range fetchers {
		// Only update the status for fetchers that are from the discovery config.
		if !fetcher.IsFromDiscoveryConfig() {
			continue
		}

		count, statusErr := fetcher.Status()
		statusAndPushErr := trace.NewAggregate(statusErr, pushErr)

		fetcherResult := awsSyncResult{
			state:               discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_RUNNING.String(),
			lastSyncTime:        lastUpdate,
			discoveredResources: count,
		}

		if statusAndPushErr != nil {
			errorMessage := statusAndPushErr.Error()
			fetcherResult.errorMessage = &errorMessage
			fetcherResult.state = discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String()
		}

		d.awsSyncResults[fetcher.DiscoveryConfigName()] = append(d.awsSyncResults[fetcher.DiscoveryConfigName()], fetcherResult)
	}
}

func (d *awsSyncStatus) discoveryConfigs() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ret := make([]string, 0, len(d.awsSyncResults))
	for k := range d.awsSyncResults {
		ret = append(ret, k)
	}
	return ret
}

func (d *awsSyncStatus) iterationStarted(fetchers []aws_sync.AWSSync, lastUpdate time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.awsSyncResults = make(map[string][]awsSyncResult)
	for _, fetcher := range fetchers {
		// Only update the status for fetchers that are from the discovery config.
		if !fetcher.IsFromDiscoveryConfig() {
			continue
		}

		fetcherResult := awsSyncResult{
			state:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
			lastSyncTime: lastUpdate,
		}

		d.awsSyncResults[fetcher.DiscoveryConfigName()] = append(d.awsSyncResults[fetcher.DiscoveryConfigName()], fetcherResult)
	}
}

func (d *awsSyncStatus) mergeIntoGlobalStatus(discoveryConfigName string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	awsStatusFetchers, found := d.awsSyncResults[discoveryConfigName]
	if !found {
		return existingStatus
	}

	var statusErrorMessages []string
	if existingStatus.ErrorMessage != nil {
		statusErrorMessages = append(statusErrorMessages, *existingStatus.ErrorMessage)
	}
	for _, fetcher := range awsStatusFetchers {
		existingStatus.DiscoveredResources = existingStatus.DiscoveredResources + fetcher.discoveredResources

		// Each DiscoveryConfigStatus has a global State and Error Message, but those are produced per Fetcher.
		// We choose to keep the most informative states by favoring error states/messages.
		if fetcher.errorMessage != nil {
			statusErrorMessages = append(statusErrorMessages, *fetcher.errorMessage)
		}

		if existingStatus.State != discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_ERROR.String() {
			existingStatus.State = fetcher.state
		}

		// Keep the earliest sync time.
		if existingStatus.LastSyncTime.After(fetcher.lastSyncTime) {
			existingStatus.LastSyncTime = fetcher.lastSyncTime
		}
	}

	if len(statusErrorMessages) > 0 {
		newErrorMessage := strings.Join(statusErrorMessages, "\n")
		existingStatus.ErrorMessage = &newErrorMessage
	}

	return existingStatus
}

// awsResourcesStatus contains all the status for AWS Matchers grouped by DiscoveryConfig for a specific matcher type.
type awsResourcesStatus struct {
	mu sync.RWMutex
	// awsResourcesResults maps the DiscoveryConfig name and integration to a summary of discovered/enrolled resources.
	awsResourcesResults map[awsResourceGroup]awsResourceGroupResult
}

// awsResourceGroup is the key for the summary
type awsResourceGroup struct {
	discoveryConfig string
	integration     string
}

// awsResourceGroupResult stores the result of the aws_sync Matchers for a given DiscoveryConfig.
type awsResourceGroupResult struct {
	found    int
	enrolled int
	failed   int
}

func (d *awsResourcesStatus) iterationStarted() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
}

func (ars *awsResourcesStatus) mergeEC2IntoGlobalStatus(discoveryConfigName string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	ars.mu.RLock()
	defer ars.mu.RUnlock()

	for group, groupResult := range ars.awsResourcesResults {
		if group.discoveryConfig != discoveryConfigName {
			continue
		}

		// Update global discovered resources count.
		existingStatus.DiscoveredResources = existingStatus.DiscoveredResources + uint64(groupResult.found)

		// Update counters specific to AWS EC2 resources discovered.
		existingIntegrationResources, ok := existingStatus.IntegrationDiscoveredResources[group.integration]
		if !ok {
			existingIntegrationResources = &discoveryconfigv1.IntegrationDiscoveredSummary{}
		}
		existingIntegrationResources.AwsEc2 = &discoveryconfigv1.ResourcesDiscoveredSummary{
			Found:    uint64(groupResult.found),
			Enrolled: uint64(groupResult.enrolled),
			Failed:   uint64(groupResult.failed),
		}
		existingStatus.IntegrationDiscoveredResources[group.integration] = existingIntegrationResources
	}

	return existingStatus
}

func (ars *awsResourcesStatus) incrementFailed(g awsResourceGroup, count int) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	groupStats := ars.awsResourcesResults[g]
	groupStats.failed = groupStats.failed + count
	ars.awsResourcesResults[g] = groupStats
}

func (ars *awsResourcesStatus) incrementFound(g awsResourceGroup, count int) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	groupStats := ars.awsResourcesResults[g]
	groupStats.found = groupStats.found + count
	ars.awsResourcesResults[g] = groupStats
}

func (ars *awsResourcesStatus) incrementEnrolled(g awsResourceGroup, count int) {
	ars.mu.Lock()
	defer ars.mu.Unlock()
	if ars.awsResourcesResults == nil {
		ars.awsResourcesResults = make(map[awsResourceGroup]awsResourceGroupResult)
	}
	groupStats := ars.awsResourcesResults[g]
	groupStats.enrolled = groupStats.enrolled + count
	ars.awsResourcesResults[g] = groupStats
}

// ReportEC2SSMInstallationResult is called when discovery gets the result of running the installation script in a EC2 instance.
// It will emit an audit event with the result and update the DiscoveryConfig status
func (s *Server) ReportEC2SSMInstallationResult(ctx context.Context, result *server.SSMInstallationResult) error {
	if err := s.Emitter.EmitAuditEvent(ctx, result.SSMRunEvent); err != nil {
		return trace.Wrap(err)
	}

	// Only failed runs are counted.
	// Successful ones only mean that the teleport was installed in the target host.
	// If they succeed in joining the cluster, during the next iteration, they will be countd as "enrolled"
	if result.SSMRunEvent.Metadata.Code == libevents.SSMRunSuccessCode {
		return nil
	}

	s.awsEC2ResourcesStatus.incrementFailed(awsResourceGroup{
		discoveryConfig: result.DiscoveryConfig,
		integration:     result.IntegrationName,
	}, 1)

	s.updateDiscoveryConfigStatus(result.DiscoveryConfig)

	return nil
}
