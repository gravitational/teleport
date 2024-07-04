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
	"sync"
	"time"

	"github.com/gravitational/trace"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
)

// updateDiscoveryConfigStatus updates the DiscoveryConfig Status field with the current in-memory status.
// The status will be updated with the following matchers:
// - AWS Sync (TAG) status
func (s *Server) updateDiscoveryConfigStatus(discoveryConfigName string) {
	discoveryConfigStatus := discoveryconfig.Status{
		State:        discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String(),
		LastSyncTime: s.clock.Now(),
	}

	// Merge AWS Sync (TAG) status
	discoveryConfigStatus = s.awsSyncStatus.mergeIntoGlobalStatus(discoveryConfigName, discoveryConfigStatus)

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	_, err := s.AccessPoint.UpdateDiscoveryConfigStatus(ctx, discoveryConfigName, discoveryConfigStatus)
	switch {
	case trace.IsNotImplemented(err):
		s.Log.Warn("UpdateDiscoveryConfigStatus method is not implemented in Auth Server. Please upgrade it to a recent version.")
	case err != nil:
		s.Log.WithError(err).Infof("Error updating discovery config %q status", discoveryConfigName)
	}
}

func newAWSSyncStatus() awsSyncStatus {
	return awsSyncStatus{
		awsSyncStatus: make(map[string]awsSyncResult),
	}
}

type awsSyncStatus struct {
	mu            sync.RWMutex
	awsSyncStatus map[string]awsSyncResult
}

type awsSyncResult struct {
	state               string
	errorMessage        *string
	lastSyncTime        time.Time
	discoveredResources uint64
}

func (d *awsSyncStatus) upsertStatus(discoveryConfig string, result awsSyncResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.awsSyncStatus == nil {
		d.awsSyncStatus = make(map[string]awsSyncResult)
	}
	d.awsSyncStatus[discoveryConfig] = result
}

func (d *awsSyncStatus) mergeIntoGlobalStatus(discoveryConfig string, existingStatus discoveryconfig.Status) discoveryconfig.Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	awsStatus, found := d.awsSyncStatus[discoveryConfig]
	if !found {
		return existingStatus
	}

	existingStatus.DiscoveredResources = existingStatus.DiscoveredResources + awsStatus.discoveredResources
	existingStatus.ErrorMessage = awsStatus.errorMessage
	existingStatus.State = awsStatus.state
	existingStatus.LastSyncTime = awsStatus.lastSyncTime

	return existingStatus
}

// updateAWSSyncDiscoveryConfigStatus updates the status for each DiscoveryConfig that originated the Fetchers.
// It updates the internal state and updates the Status in the cluster.
func (s *Server) updateAWSSyncDiscoveryConfigStatus(fetchers []aws_sync.AWSSync, pushErr error, preRun bool) error {
	lastUpdate := s.clock.Now()
	for _, fetcher := range fetchers {
		// Only update the status for fetchers that are from the discovery config.
		if !fetcher.IsFromDiscoveryConfig() {
			continue
		}

		status := buildAWSSyncFetcherStatus(fetcher, pushErr, lastUpdate)
		if preRun {
			status.state = discoveryconfigv1.DiscoveryConfigState_DISCOVERY_CONFIG_STATE_SYNCING.String()
		}
		s.awsSyncStatus.upsertStatus(fetcher.DiscoveryConfigName(), status)
		s.updateDiscoveryConfigStatus(fetcher.DiscoveryConfigName())
	}
	return nil
}
