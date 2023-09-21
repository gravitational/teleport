/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/integration/integrationv1"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	// updateDeployAgentsInterval specifies how frequently to check for available updates.
	updateDeployAgentsInterval = time.Minute * 30

	// updateDeployAgentsRateLimit specifies the time between updates across AWS regions.
	updateDeployAgentsRateLimit = time.Second * 30
)

func (process *TeleportProcess) periodUpdateDeployServiceAgents() error {
	if !process.Config.Auth.Enabled {
		return nil
	}

	// start process only after teleport process has started
	if _, err := process.WaitForEvent(process.GracefulExitContext(), TeleportReadyEvent); err != nil {
		return nil
	}
	process.log.Infof("The new service has started successfully. Checking for deploy service updates every %v.", updateDeployAgentsInterval)

	// Acquire the semaphore before attempting to update the deploy service agents.
	// This task should only run on a single instance at a time.
	lock, err := services.AcquireSemaphoreWithRetry(process.GracefulExitContext(),
		services.AcquireSemaphoreWithRetryConfig{
			Service: process.GetAuthServer(),
			Request: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.SemaphoreKindConnection,
				SemaphoreName: "update_deploy_service_agents",
				MaxLeases:     1,
				Expires:       process.Clock.Now().Add(updateDeployAgentsInterval),
			},
			Retry: retryutils.LinearConfig{
				Step: time.Minute,
				Max:  updateDeployAgentsInterval,
			},
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err := process.GetAuthServer().CancelSemaphoreLease(process.GracefulExitContext(), *lock); err != nil {
			process.log.WithError(err).Errorf("Failed to cancel lease: %v.", lock)
		}
	}()

	periodic := interval.New(interval.Config{
		Duration: updateDeployAgentsInterval,
		Jitter:   retryutils.NewSeventhJitter(),
	})
	defer periodic.Stop()

	for {
		if err := process.updateDeployServiceAgents(process.GracefulExitContext(), process.GetAuthServer()); err != nil {
			process.log.Warningf("Update failed: %v. Retrying in ~%v", err, updateDeployAgentsInterval)
		}

		select {
		case <-periodic.Next():
		case <-process.GracefulExitContext().Done():
			return nil
		}
	}
}

func (process *TeleportProcess) updateDeployServiceAgents(ctx context.Context, authServer *auth.Server) error {
	if !process.shouldUpdateDeployAgents() {
		return nil
	}

	teleportVersion, err := process.getStableTeleportVersion()
	if err != nil {
		return trace.Wrap(err)
	}

	issuer, err := awsoidc.IssuerForCluster(ctx, authServer)
	if err != nil {
		return trace.Wrap(err)
	}

	token, err := integrationv1.GenerateAWSOIDCToken(ctx, integrationv1.AWSOIDCTokenConfig{
		CAGetter: authServer,
		Clock:    process.Clock,
		TTL:      updateDeployAgentsInterval,
		Issuer:   issuer,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clusterNameConfig, err := authServer.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	var resources []types.Integration
	var nextKey string
	for {
		igs, nextKey, err := authServer.ListIntegrations(ctx, 0, nextKey)
		if err != nil {
			return trace.Wrap(err)
		}
		resources = append(resources, igs...)
		if nextKey == "" {
			break
		}
	}

	awsRegions, err := process.listAWSDatabaseRegions()
	if err != nil {
		return trace.Wrap(err)
	}

	limit := rate.NewLimiter(rate.Every(updateDeployAgentsRateLimit), 1)
	for _, ig := range resources {
		spec := ig.GetAWSOIDCIntegrationSpec()
		if spec == nil {
			continue
		}

		for _, region := range awsRegions {
			if err := limit.Wait(ctx); err != nil {
				return trace.Wrap(err)
			}

			req := &awsoidc.AWSClientRequest{
				IntegrationName: ig.GetName(),
				Token:           token,
				RoleARN:         spec.RoleARN,
				Region:          region,
			}

			deployServiceClient, err := awsoidc.NewDeployServiceClient(ctx, req, authServer)
			if err != nil {
				process.log.Warningf("Failed to update deploy service agents: %v", err)
				continue
			}

			ownershipTags := map[string]string{
				types.ClusterLabel:     clusterNameConfig.GetClusterName(),
				types.OriginLabel:      types.OriginIntegrationAWSOIDC,
				types.IntegrationLabel: ig.GetName(),
			}

			err = awsoidc.UpdateDeployServiceAgents(ctx, deployServiceClient, clusterNameConfig.GetClusterName(), teleportVersion, ownershipTags)
			invalidTokenError := new(ststypes.InvalidIdentityTokenException)
			if errors.As(err, &invalidTokenError) {
				process.log.Debugf("Invalid identity token for region %v: %v", region, err)
				continue
			}
			if err != nil {
				process.log.Warningf("Failed to update deploy service agents: %v", err)
				continue
			}
		}
	}
	return nil
}

// listAWSDatabaseRegions returns the list of AWS regions containing a connected database.
func (process *TeleportProcess) listAWSDatabaseRegions() ([]string, error) {
	databases, err := process.GetAuthServer().GetDatabases(process.GracefulExitContext())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	regions := make(map[string]interface{})
	for _, database := range databases {
		if database.IsAWSHosted() && database.IsRDS() {
			regions[database.GetAWS().Region] = nil
		}
	}

	var result []string
	for region := range regions {
		result = append(result, region)
	}

	return result, nil
}

// shouldUpdateDeployAgents returns true if deploy agents should be updated.
func (process *TeleportProcess) shouldUpdateDeployAgents() bool {
	cmc, err := process.GetAuthServer().GetClusterMaintenanceConfig(process.GracefulExitContext())
	if err != nil {
		process.log.Debugf("Failed to get cluster maintenance config: %v", err)
		return false
	}

	var criticalEndpoint string
	if automaticupgrades.GetChannel() != "" {
		criticalEndpoint, err = url.JoinPath(automaticupgrades.GetChannel(), "critical")
		if err != nil {
			process.log.Debugf("Failed to get critical upgrade endpoint: %v", err)
			return false
		}
	}

	critical, err := automaticupgrades.Critical(process.GracefulExitContext(), criticalEndpoint)
	if err != nil {
		process.log.Debugf("Failed to get critical upgrade value: %v", err)
		return false
	}

	if withinUpgradeWindow(cmc, process.Clock) || critical {
		return true
	}

	return false
}

func (process *TeleportProcess) getStableTeleportVersion() (string, error) {
	var versionEndpoint string
	var err error
	if automaticupgrades.GetChannel() != "" {
		versionEndpoint, err = url.JoinPath(automaticupgrades.GetChannel(), "version")
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	stableVersion, err := automaticupgrades.Version(process.GracefulExitContext(), versionEndpoint)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// cloudStableVersion has vX.Y.Z format, however the container image tag does not include the `v`.
	return strings.TrimPrefix(stableVersion, "v"), nil
}

// withinUpgradeWindow returns true if the current time is within the configured
// upgrade window.
func withinUpgradeWindow(cmc types.ClusterMaintenanceConfig, clock clockwork.Clock) bool {
	upgradeWindow, ok := cmc.GetAgentUpgradeWindow()
	if !ok {
		return false
	}

	now := clock.Now()
	if len(upgradeWindow.Weekdays) == 0 {
		if int(upgradeWindow.UTCStartHour) == now.Hour() {
			return true
		}
	}

	weekday := now.Weekday().String()
	for _, upgradeWeekday := range upgradeWindow.Weekdays {
		if weekday == upgradeWeekday {
			if int(upgradeWindow.UTCStartHour) == now.Hour() {
				return true
			}
		}
	}
	return false
}
