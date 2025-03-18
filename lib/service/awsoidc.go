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

package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	// updateAWSOIDCDeployServiceInterval specifies how frequently to check for available updates.
	updateAWSOIDCDeployServiceInterval = time.Minute * 30

	// maxConcurrentUpdates specifies the maximum number of concurrent updates
	maxConcurrentUpdates = 3
)

func (process *TeleportProcess) initAWSOIDCDeployServiceUpdater(channels automaticupgrades.Channels) error {
	// start process only after teleport process has started
	if _, err := process.WaitForEvent(process.GracefulExitContext(), TeleportReadyEvent); err != nil {
		return trace.Wrap(err)
	}

	authClient := process.getInstanceClient()
	if authClient == nil {
		return trace.Errorf("instance client not yet initialized")
	}

	resp, err := authClient.Ping(process.GracefulExitContext())
	if err != nil {
		return trace.Wrap(err)
	}

	if !resp.ServerFeatures.AutomaticUpgrades {
		return nil
	}

	upgradeChannel, err := channels.DefaultChannel()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterNameConfig, err := authClient.GetClusterName(process.GracefulExitContext())
	if err != nil {
		return trace.Wrap(err)
	}

	updater, err := NewDeployServiceUpdater(AWSOIDCDeployServiceUpdaterConfig{
		Log:                    process.logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentProxy, "aws_oidc_deploy_service_updater")),
		AuthClient:             authClient,
		Clock:                  process.Clock,
		TeleportClusterName:    clusterNameConfig.GetClusterName(),
		TeleportClusterVersion: resp.GetServerVersion(),
		UpgradeChannel:         upgradeChannel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	process.logger.InfoContext(process.ExitContext(), "The new service has started successfully.", "update_interval", updateAWSOIDCDeployServiceInterval)
	return trace.Wrap(updater.Run(process.GracefulExitContext()))
}

// AWSOIDCDeployServiceUpdaterConfig specifies updater configs
type AWSOIDCDeployServiceUpdaterConfig struct {
	// Log is the logger
	Log *slog.Logger
	// AuthClient is the auth api client
	AuthClient *authclient.Client
	// Clock is the local clock
	Clock clockwork.Clock
	// TeleportClusterName specifies the teleport cluster name
	TeleportClusterName string
	// TeleportClusterVersion specifies the teleport cluster version
	TeleportClusterVersion string
	// UpgradeChannel is the channel that serves the version used by the updater.
	UpgradeChannel *automaticupgrades.Channel
}

// CheckAndSetDefaults checks and sets default config values.
func (cfg *AWSOIDCDeployServiceUpdaterConfig) CheckAndSetDefaults() error {
	if cfg.AuthClient == nil {
		return trace.BadParameter("auth client required")
	}

	if cfg.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name required")
	}

	if cfg.TeleportClusterVersion == "" {
		return trace.BadParameter("teleport cluster version required")
	}

	if cfg.UpgradeChannel == nil {
		return trace.BadParameter("automatic upgrades channel required")
	}

	if cfg.Log == nil {
		cfg.Log = slog.Default().With(teleport.ComponentKey, teleport.Component(teleport.ComponentProxy, "aws_oidc_deploy_service_updater"))
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return nil
}

// AWSOIDCDeployServiceUpdater periodically updates AWS OIDC deploy service
type AWSOIDCDeployServiceUpdater struct {
	AWSOIDCDeployServiceUpdaterConfig
}

// NewAWSOIDCDeployServiceUpdater returns a new AWSOIDCDeployServiceUpdater
func NewDeployServiceUpdater(config AWSOIDCDeployServiceUpdaterConfig) (*AWSOIDCDeployServiceUpdater, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &AWSOIDCDeployServiceUpdater{
		AWSOIDCDeployServiceUpdaterConfig: config,
	}, nil
}

// Run periodically updates the AWS OIDC deploy service
func (updater *AWSOIDCDeployServiceUpdater) Run(ctx context.Context) error {
	periodic := interval.New(interval.Config{
		Duration: updateAWSOIDCDeployServiceInterval,
		Jitter:   retryutils.SeventhJitter,
	})
	defer periodic.Stop()

	for {
		if err := updater.updateAWSOIDCDeployServices(ctx); err != nil {
			updater.Log.WarnContext(ctx, "Update failed. Retrying", "retry_interval", updateAWSOIDCDeployServiceInterval, "error", err)
		}

		select {
		case <-periodic.Next():
		case <-ctx.Done():
			return nil
		}
	}
}

func (updater *AWSOIDCDeployServiceUpdater) updateAWSOIDCDeployServices(ctx context.Context) error {
	cmc, err := updater.AuthClient.GetClusterMaintenanceConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	critical, err := updater.UpgradeChannel.GetCritical(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Upgrade should only be attempted if the current time is within the configured
	// upgrade window, or if a critical upgrade is available
	if !cmc.WithinUpgradeWindow(updater.Clock.Now()) && !critical {
		return nil
	}

	stableVersion, err := updater.UpgradeChannel.GetVersion(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// stableVersion has vX.Y.Z format, however the container image tag does not include the `v`.
	stableVersion = strings.TrimPrefix(stableVersion, "v")

	// minServerVersion specifies the minimum version of the cluster required for
	// updated AWS OIDC deploy service to remain compatible with the cluster.
	minServerVersion, err := utils.MajorSemver(stableVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	if !utils.MeetsMinVersion(updater.TeleportClusterVersion, minServerVersion) {
		updater.Log.DebugContext(ctx, "Skipping update. AWS OIDC Deploy Service will not be compatible with cluster", "cluster_version", updater.TeleportClusterVersion, "stable_version", stableVersion)
		return nil
	}

	databases, err := updater.AuthClient.GetDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// The updater needs to iterate over all integrations and aws regions to check
	// for AWS OIDC deploy services to update. In order to reduce the number of api
	// calls, the aws regions are first reduced to only the regions containing
	// an RDS database.
	awsRegions := make(map[string]interface{})
	for _, database := range databases {
		if database.IsAWSHosted() && database.IsRDS() {
			awsRegions[database.GetAWS().Region] = nil
		}
	}

	integrations, err := updater.AuthClient.ListAllIntegrations(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Perform updates in parallel across regions.
	sem := semaphore.NewWeighted(maxConcurrentUpdates)
	for _, ig := range integrations {
		for region := range awsRegions {
			if err := sem.Acquire(ctx, 1); err != nil {
				return trace.Wrap(err)
			}
			go func(ig types.Integration, region string) {
				defer sem.Release(1)
				if err := updater.updateAWSOIDCDeployService(ctx, ig, region, stableVersion); err != nil {
					updater.Log.WarnContext(ctx, "Failed to update AWS OIDC Deploy Service", "integration", ig.GetName(), "region", region, "error", err)
				}
			}(ig, region)
		}
	}

	// Wait for all updates to finish.
	return trace.Wrap(sem.Acquire(ctx, maxConcurrentUpdates))
}

func (updater *AWSOIDCDeployServiceUpdater) updateAWSOIDCDeployService(ctx context.Context, integration types.Integration, awsRegion, teleportVersion string) error {
	// Do not attempt update if integration is not an AWS OIDC integration.
	if integration.GetAWSOIDCIntegrationSpec() == nil {
		return nil
	}

	token, err := updater.AuthClient.GenerateAWSOIDCToken(ctx, integration.GetName())
	if err != nil {
		return trace.Wrap(err)
	}

	req := &awsoidc.AWSClientRequest{
		Token:   token,
		RoleARN: integration.GetAWSOIDCIntegrationSpec().RoleARN,
		Region:  awsRegion,
	}

	// The deploy service client is initialized using AWS OIDC integration.
	awsOIDCDeployServiceClient, err := awsoidc.NewDeployServiceClient(ctx, req, updater.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	// ownershipTags are used to identify if the ecs resources are managed by the
	// teleport integration.
	ownershipTags := map[string]string{
		types.ClusterLabel:     updater.TeleportClusterName,
		types.OriginLabel:      types.OriginIntegrationAWSOIDC,
		types.IntegrationLabel: integration.GetName(),
	}

	// Acquire a lease for the region + integration before attempting to update the deploy service.
	// If the lease cannot be acquired, the update is already being handled by another instance.
	semLock, err := updater.AuthClient.AcquireSemaphore(ctx, types.AcquireSemaphoreRequest{
		SemaphoreKind: types.SemaphoreKindConnection,
		SemaphoreName: fmt.Sprintf("update_aws_oidc_deploy_service_%s_%s", awsRegion, integration.GetName()),
		MaxLeases:     1,
		Expires:       updater.Clock.Now().Add(updateAWSOIDCDeployServiceInterval),
		Holder:        "update_aws_oidc_deploy_service",
	})
	if err != nil {
		if strings.Contains(err.Error(), teleport.MaxLeases) {
			updater.Log.DebugContext(ctx, "AWS OIDC Deploy Service update is already being processed", "error", err)
			return nil
		}
		return trace.Wrap(err)
	}
	defer func() {
		if err := updater.AuthClient.CancelSemaphoreLease(ctx, *semLock); err != nil {
			updater.Log.ErrorContext(ctx, "Failed to cancel semaphore lease", "error", err)
		}
	}()

	updater.Log.DebugContext(ctx, "Updating AWS OIDC Deploy Service",
		"integration", integration.GetName(),
		"region", awsRegion,
		"new_version", teleportVersion,
	)
	if err := awsoidc.UpdateDeployService(ctx, awsOIDCDeployServiceClient, updater.Log, awsoidc.UpdateServiceRequest{
		TeleportClusterName: updater.TeleportClusterName,
		TeleportVersionTag:  teleportVersion,
		OwnershipTags:       ownershipTags,
	}); err != nil {

		switch {
		case trace.IsNotFound(err):
			// The updater checks each integration/region combination, so
			// there will be regions where there is no ECS cluster deployed
			// for the integration.
			updater.Log.DebugContext(ctx, "Integration does not manage any services in given region", "integration", integration.GetName(), "region", awsRegion)
			return nil
		case trace.IsAccessDenied(awslib.ConvertIAMError(trace.Unwrap(err))):
			// The AWS OIDC role may lack permissions due to changes in teleport.
			// In this situation users should be notified that they will need to
			// re-run the deploy service iam configuration script and update the
			// permissions.
			updater.Log.DebugContext(ctx, "Update integration role and add missing permissions", "integration", integration.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}
