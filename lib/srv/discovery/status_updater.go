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
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

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
	semaphoreService       types.Semaphores
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

// acquireSemaphoreDiscoveryConfigStatusUpdate tries to acquire a semaphore lock for this discovery config.
// It returns a func which must be called to release the lock.
// It also returns a context which is tied to the lease and will be canceled if the lease ends.
func (s *discoveryConfigStatusUpdater) acquireSemaphoreDiscoveryConfigStatusUpdate(ctx context.Context, discoveryConfigName string) (releaseFn func(), contextWithLease context.Context, err error) {
	const semaphoreExpiration = 10 * time.Second

	// AcquireSemaphoreLock will retry until the semaphore is acquired.
	// This prevents multiple discovery services from writing DiscoveryConfig Status resources in parallel.
	// lease must be released to cleanup the resource in auth server.
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.semaphoreService,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindDiscoveryConfig,
					SemaphoreName: discoveryConfigName,
					MaxLeases:     1,
					Holder:        s.serverID,
				},
				Expiry: semaphoreExpiration,
				Clock:  s.clock,
			},
			Retry: retryutils.LinearConfig{
				Clock:  s.clock,
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	ctxWithLease, cancel := context.WithCancel(lease)
	releaseFn = func() {
		cancel()
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.log.WarnContext(ctx, "Error cleaning up DiscoveryConfig Status semaphore",
				"semaphore", discoveryConfigName,
				"error", err,
			)
		}
	}

	return releaseFn, ctxWithLease, nil
}

// update updates a DiscoveryConfig's status in the backend.
// A semaphore lock is acquired to perform a read-modify-write, ensuring concurrent Discovery Services don't overwrite each other's server iteration history.
func (s *discoveryConfigStatusUpdater) update(ctx context.Context, discoveryConfigName string, discoveryConfigStatus discoveryconfig.Status) error {
	releaseFn, leaseCtx, err := s.acquireSemaphoreDiscoveryConfigStatusUpdate(ctx, discoveryConfigName)
	if err != nil {
		return trace.Wrap(err)
	}
	defer releaseFn()

	existingDiscoveryConfig, err := s.discoveryConfigService.GetDiscoveryConfig(leaseCtx, discoveryConfigName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	discoveryConfigStatus = s.updateServerStatus(discoveryConfigStatus, existingDiscoveryConfig)

	if _, err := s.discoveryConfigService.UpdateDiscoveryConfigStatus(leaseCtx, discoveryConfigName, discoveryConfigStatus); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// updateServerStatus merges this server's integration summaries into the DiscoveryConfig status, preserving other servers' iteration history.
func (s *discoveryConfigStatusUpdater) updateServerStatus(status discoveryconfig.Status, existingDiscoveryConfig *discoveryconfig.DiscoveryConfig) discoveryconfig.Status {
	existingServerStatuses := existingServerStatuses(existingDiscoveryConfig)

	// Build this server's iteration history by merging each integration's current summary with its previous state.
	integrationSummaries := make(map[string]*discoveryconfigv1.DiscoverSummary, len(status.IntegrationDiscoveredResources))
	for integration, currentDiscoverSummary := range status.IntegrationDiscoveredResources {
		previousSummary := existingDiscoverSummary(s.serverID, integration, existingServerStatuses)

		integrationSummaries[integration] = &discoveryconfigv1.DiscoverSummary{
			AwsEc2:   mergeResourceSummary(currentDiscoverSummary.GetAwsEc2(), previousSummary.GetAwsEc2()),
			AwsRds:   mergeResourceSummary(currentDiscoverSummary.GetAwsRds(), previousSummary.GetAwsRds()),
			AwsEks:   mergeResourceSummary(currentDiscoverSummary.GetAwsEks(), previousSummary.GetAwsEks()),
			AzureVms: mergeResourceSummary(currentDiscoverSummary.GetAzureVms(), previousSummary.GetAzureVms()),
		}
	}

	status.ServerStatus = map[string]*discoveryconfig.DiscoveryStatusServer{
		s.serverID: {
			DiscoveryStatusServer: &discoveryconfigv1.DiscoveryStatusServer{
				IntegrationSummaries: integrationSummaries,
				LastUpdate:           timestamppb.New(s.clock.Now()),
				PollInterval:         durationpb.New(s.pollInterval),
			},
		},
	}

	// If multiple servers are discovering resources for this DiscoveryConfig, don't change their status report.
	// This call is protected by a semaphore lock, so statuses are not overridden by other services.
	for existingServerID, existingServerStatus := range existingServerStatuses {
		if s.serverID != existingServerID {
			if staleServerStatus(s.clock.Now(), existingServerStatus) {
				continue
			}

			status.ServerStatus[existingServerID] = &discoveryconfig.DiscoveryStatusServer{
				DiscoveryStatusServer: existingServerStatus.DiscoveryStatusServer,
			}
		}
	}
	return status
}

// staleServerStatus determines whether the existing summary for a server is stale.
// The summary may be discarded if the server has not updated its status for a long time, indicating that it may be down and its status is stale.
func staleServerStatus(now time.Time, existingServerStatus *discoveryconfig.DiscoveryStatusServer) bool {
	serverPollInterval := existingServerStatus.GetPollInterval().AsDuration()
	if serverPollInterval <= 0 {
		serverPollInterval = common.DefaultDiscoveryPollInterval
	}
	tooOldThreshold := now.Add(-10 * serverPollInterval)
	return existingServerStatus.GetLastUpdate().AsTime().Before(tooOldThreshold)
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

func existingServerStatuses(existingDiscoveryConfig *discoveryconfig.DiscoveryConfig) map[string]*discoveryconfig.DiscoveryStatusServer {
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
