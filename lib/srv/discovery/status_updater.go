/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

type semaphoreAccessPoint interface {
	// AcquireSemaphore acquires lease with requested resources from semaphore.
	AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error)
	// CancelSemaphoreLease cancels semaphore lease early.
	CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error
}

type discoveryConfigService interface {
	// GetDiscoveryConfig returns the DiscoveryConfig resource with the given name.
	GetDiscoveryConfig(ctx context.Context, name string) (*discoveryconfig.DiscoveryConfig, error)
	// UpdateDiscoveryConfigStatus updates the Status field of the DiscoveryConfig resource with the given name.
	UpdateDiscoveryConfigStatus(ctx context.Context, name string, status discoveryconfig.Status) (*discoveryconfig.DiscoveryConfig, error)
}

type discoveryConfigStatusUpdater struct {
	log                    *slog.Logger
	serverID               string
	clock                  clockwork.Clock
	semaphoreService       semaphoreAccessPoint
	discoveryConfigService discoveryConfigService
}

func newDiscoveryConfigStatusUpdater(cfg *Config) *discoveryConfigStatusUpdater {
	return &discoveryConfigStatusUpdater{
		log:                    cfg.Log,
		serverID:               cfg.ServerID,
		clock:                  cfg.clock,
		semaphoreService:       cfg.AccessPoint,
		discoveryConfigService: cfg.AccessPoint,
	}
}

// update receives a new status for a DiscoveryConfig and updates it in the backend.
// The DiscoveryConfig Status keeps the last 5 fetch summaries (ie. how many resources were discovered/enrolled).
// We might have concurrent Discovery Services working in the same Discovery Config, so a semaphore lock is acquired to ensure that the history is kept consistent and not overwritten by concurrent updates.
func (s *discoveryConfigStatusUpdater) update(ctx context.Context, discoveryConfigName string, discoveryConfigStatus discoveryconfig.Status) error {
	const operationTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	lease, err := s.semaphoreService.AcquireSemaphore(ctx, types.AcquireSemaphoreRequest{
		SemaphoreKind: types.KindDiscoveryConfig,
		SemaphoreName: discoveryConfigName,
		MaxLeases:     1,
		Expires:       s.clock.Now().Add(operationTimeout),
		Holder:        s.serverID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := s.semaphoreService.CancelSemaphoreLease(ctx, *lease); err != nil {
			s.log.WarnContext(ctx, "Failed to release DiscoveryConfig status update lock, it will expire automatically",
				"discovery_config_name", discoveryConfigName,
				"expiration", lease.Expires,
				"error", err,
			)
		}
	}()

	existingDiscoveryConfig, err := s.discoveryConfigService.GetDiscoveryConfig(ctx, discoveryConfigName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	discoveryConfigStatus = mergeSummaries(discoveryConfigStatus, existingDiscoveryConfig)

	if _, err := s.discoveryConfigService.UpdateDiscoveryConfigStatus(ctx, discoveryConfigName, discoveryConfigStatus); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DiscoveryConfigStatus has a HistoryIntegrationDiscoveredResources field which keeps a history of previous iteration summaries.
// This function merges the existing summaries into the new status while ensuring the maximum number of summaries is not exceeded.
func mergeSummaries(discoveryConfigStatus discoveryconfig.Status, existingDiscoveryConfig *discoveryconfig.DiscoveryConfig) discoveryconfig.Status {
	const maxSummariesInHistory = 5

	if existingDiscoveryConfig == nil {
		return discoveryConfigStatus
	}
	existingStatus := existingDiscoveryConfig.Status

	discoveryConfigStatus.IntegrationDiscoveredResourcesHistory = make(map[string]*discoveryconfigv1.IntegrationDiscoveredSummaryHistory)

	for integration := range discoveryConfigStatus.IntegrationDiscoveredResources {
		var summaries []*discoveryconfigv1.IntegrationDiscoveredSummary

		if summary, ok := existingStatus.IntegrationDiscoveredResources[integration]; ok {
			summaries = append(summaries, summary)
		}

		if history, ok := existingStatus.IntegrationDiscoveredResourcesHistory[integration]; ok {
			for _, s := range history.GetSummaries() {
				if len(summaries) >= maxSummariesInHistory {
					break
				}
				summaries = append(summaries, s)
			}
		}
		discoveryConfigStatus.IntegrationDiscoveredResourcesHistory[integration] = &discoveryconfigv1.IntegrationDiscoveredSummaryHistory{
			Summaries: summaries,
		}
	}

	return discoveryConfigStatus
}
