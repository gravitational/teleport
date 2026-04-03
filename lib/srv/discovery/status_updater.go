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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	pollInterval           time.Duration
	serverID               string
	clock                  clockwork.Clock
	semaphoreService       semaphoreAccessPoint
	discoveryConfigService discoveryConfigService
}

func newDiscoveryConfigStatusUpdater(cfg *Config) *discoveryConfigStatusUpdater {
	return &discoveryConfigStatusUpdater{
		log:                    cfg.Log,
		pollInterval:           cfg.PollInterval,
		serverID:               cfg.ServerID,
		clock:                  cfg.clock,
		semaphoreService:       cfg.AccessPoint,
		discoveryConfigService: cfg.AccessPoint,
	}
}

// update updates a DiscoveryConfig's status in the backend.
// A semaphore lock is acquired to perform a read-modify-write, ensuring concurrent Discovery Services don't overwrite each other's server iteration history.
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
		// Creating a new context here ensures we can cancel the semaphore lease even if the parent's context is already expired.
		// Re-using the parent context might cause the CancelSemaphoreLease call to fail immediately, leaving the lock present until it expires.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), operationTimeout)
		defer cancel()

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

	discoveryConfigStatus = s.updateServerStatus(discoveryConfigStatus, existingDiscoveryConfig)

	if _, err := s.discoveryConfigService.UpdateDiscoveryConfigStatus(ctx, discoveryConfigName, discoveryConfigStatus); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// updateServerStatus merges this server's integration summaries into the DiscoveryConfig status, preserving other servers' iteration history.
func (s *discoveryConfigStatusUpdater) updateServerStatus(status discoveryconfig.Status, existingDiscoveryConfig *discoveryconfig.DiscoveryConfig) discoveryconfig.Status {
	existingServerStatus := existingServerStatus(existingDiscoveryConfig)

	// Build this server's iteration history by merging each integration's current summary with its previous state.
	iterationHistory := make(map[string]*discoveryconfigv1.DiscoverSummary, len(status.IntegrationDiscoveredResources))
	for integration, currentDiscoverSummary := range status.IntegrationDiscoveredResources {
		previousSummary := existingDiscoverSummary(s.serverID, integration, existingServerStatus)

		iterationHistory[integration] = &discoveryconfigv1.DiscoverSummary{
			AwsEc2:   mergeResourceSummary(currentDiscoverSummary.GetAwsEc2(), previousSummary.GetAwsEc2()),
			AwsRds:   mergeResourceSummary(currentDiscoverSummary.GetAwsRds(), previousSummary.GetAwsRds()),
			AwsEks:   mergeResourceSummary(currentDiscoverSummary.GetAwsEks(), previousSummary.GetAwsEks()),
			AzureVms: mergeResourceSummary(currentDiscoverSummary.GetAzureVms(), previousSummary.GetAzureVms()),
		}
	}

	status.ServerStatus = map[string]*discoveryconfig.DiscoveryStatusServer{
		s.serverID: {
			DiscoveryStatusServer: &discoveryconfigv1.DiscoveryStatusServer{
				IntegrationSummaries: iterationHistory,
				LastUpdate:           timestamppb.New(s.clock.Now()),
				PollInterval:         durationpb.New(s.pollInterval),
			},
		},
	}

	// If multiple servers are discovering resources for this DiscoveryConfig, don't change their status report.
	// This call is protected by a semaphore lock, so statuses are not overridden by other services.
	for existingServerID, existingIterationHistory := range existingServerStatus {
		if s.serverID != existingServerID {
			status.ServerStatus[existingServerID] = &discoveryconfig.DiscoveryStatusServer{
				DiscoveryStatusServer: existingIterationHistory.DiscoveryStatusServer,
			}
		}
	}
	return status
}

func mergeResourceSummary(summary *discoveryconfigv1.ResourcesDiscoveredSummary, existingSummary *discoveryconfigv1.ResourceSummary) *discoveryconfigv1.ResourceSummary {
	if summary == nil {
		return nil
	}

	if summary.GetSyncEnd() != nil {
		return &discoveryconfigv1.ResourceSummary{
			Previous: summary,
			Current:  nil,
		}
	}

	return &discoveryconfigv1.ResourceSummary{
		Previous: existingSummary.GetPrevious(),
		Current:  summary,
	}
}

func existingServerStatus(existingDiscoveryConfig *discoveryconfig.DiscoveryConfig) map[string]*discoveryconfig.DiscoveryStatusServer {
	if existingDiscoveryConfig == nil {
		return nil
	}
	return existingDiscoveryConfig.Status.ServerStatus
}

func existingDiscoverSummary(serverID, integration string, existingServerIterations map[string]*discoveryconfig.DiscoveryStatusServer) *discoveryconfigv1.DiscoverSummary {
	if existingServerIterations == nil {
		return nil
	}
	existingServerStatus, ok := existingServerIterations[serverID]
	if !ok {
		return nil
	}

	if existingServerStatus.DiscoveryStatusServer == nil {
		return nil
	}
	return existingServerStatus.DiscoveryStatusServer.IntegrationSummaries[integration]
}
