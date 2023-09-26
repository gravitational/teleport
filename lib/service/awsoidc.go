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
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	// updateDeployAgentsInterval specifies how frequently to check for available updates.
	updateDeployAgentsInterval = time.Minute * 30

	// updateDeployAgentsRateLimit specifies the time between updates across AWS regions.
	updateDeployAgentsRateLimit = time.Second * 30
)

func (process *TeleportProcess) periodicUpdateDeployServiceAgents() error {
	if !process.Config.Proxy.Enabled {
		return nil
	}

	// start process only after teleport process has started
	if _, err := process.WaitForEvent(process.GracefulExitContext(), TeleportReadyEvent); err != nil {
		return trace.Wrap(err)
	}
	process.log.Infof("The new service has started successfully. Checking for deploy service updates every %v.", updateDeployAgentsInterval)

	resp, err := process.getInstanceClient().Ping(process.GracefulExitContext())
	if err != nil {
		return trace.Wrap(err)
	}

	if !resp.ServerFeatures.AutomaticUpgrades {
		return nil
	}

	periodic := interval.New(interval.Config{
		Duration: updateDeployAgentsInterval,
		Jitter:   retryutils.NewSeventhJitter(),
	})
	defer periodic.Stop()

	for {
		if err := process.updateDeployServiceAgents(process.GracefulExitContext(), process.getInstanceClient()); err != nil {
			process.log.Warningf("Update failed: %v. Retrying in ~%v", err, updateDeployAgentsInterval)
		}

		select {
		case <-periodic.Next():
		case <-process.GracefulExitContext().Done():
			return nil
		}
	}
}

func (process *TeleportProcess) updateDeployServiceAgents(ctx context.Context, authClient *auth.Client) error {
	cmc, err := authClient.GetClusterMaintenanceConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// If criticalEndpoint or versionEndpoint are empty, the default stable/cloud endpoint will be used
	var criticalEndpoint string
	var versionEndpoint string
	if automaticupgrades.GetChannel() != "" {
		criticalEndpoint, err = url.JoinPath(automaticupgrades.GetChannel(), "critical")
		if err != nil {
			return trace.Wrap(err)
		}
		versionEndpoint, err = url.JoinPath(automaticupgrades.GetChannel(), "version")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	critical, err := automaticupgrades.Critical(ctx, criticalEndpoint)
	if err != nil {
		return trace.Wrap(err)
	}

	// Upgrade should only be attempted if the current time is within the configured
	// upgrade window, or if a critical upgrade is available
	if !cmc.WithinUpgradeWindow(process.Clock.Now()) && !critical {
		return nil
	}

	stableVersion, err := automaticupgrades.Version(ctx, versionEndpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	// cloudStableVersion has vX.Y.Z format, however the container image tag does not include the `v`.
	cloudStableVersion := strings.TrimPrefix(stableVersion, "v")

	issuer, err := awsoidc.IssuerFromPublicAddress(process.proxyPublicAddr().Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterNameConfig, err := authClient.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterName := clusterNameConfig.GetClusterName()

	databases, err := authClient.GetDatabases(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	awsRegions := make(map[string]interface{})
	for _, database := range databases {
		if database.IsAWSHosted() && database.IsRDS() {
			awsRegions[database.GetAWS().Region] = nil
		}
	}

	integrations, err := authClient.ListAllIntegrations(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	limit := rate.NewLimiter(rate.Every(updateDeployAgentsRateLimit), 1)
	for _, ig := range integrations {
		spec := ig.GetAWSOIDCIntegrationSpec()
		if spec == nil {
			continue
		}
		integrationName := ig.GetName()

		for region := range awsRegions {
			if err := limit.Wait(ctx); err != nil {
				return trace.Wrap(err)
			}

			token, err := authClient.GenerateAWSOIDCToken(ctx, types.GenerateAWSOIDCTokenRequest{
				Issuer: issuer,
			})
			if err != nil {
				return trace.Wrap(err)
			}

			req := &awsoidc.AWSClientRequest{
				IntegrationName: ig.GetName(),
				Token:           token,
				RoleARN:         spec.RoleARN,
				Region:          region,
			}

			deployServiceClient, err := awsoidc.NewDeployServiceClient(ctx, req, authClient)
			if err != nil {
				process.log.Warningf("Failed to update deploy service agents: %v", err)
				continue
			}

			ownershipTags := map[string]string{
				types.ClusterLabel:     clusterName,
				types.OriginLabel:      types.OriginIntegrationAWSOIDC,
				types.IntegrationLabel: integrationName,
			}

			// Acquire a lease for the region + integration before attempting to update the deploy service agent.
			// If the lease cannot be acquired, the update is already being handled by another instance.
			semLock, err := authClient.AcquireSemaphore(ctx, types.AcquireSemaphoreRequest{
				SemaphoreKind: types.SemaphoreKindConnection,
				SemaphoreName: fmt.Sprintf("update_deploy_service_agents_%s_%s", region, integrationName),
				MaxLeases:     1,
				Expires:       process.Clock.Now().Add(updateDeployAgentsInterval),
				Holder:        "update_deploy_service_agents",
			})

			if err != nil {
				if strings.Contains(err.Error(), teleport.MaxLeases) {
					process.log.Debug("Deploy service agent update is already being processed")
					continue
				}
				return trace.Wrap(err)
			}

			process.log.Debugf("Updating Deploy Service Agents in AWS region: %s", region)
			if err := awsoidc.UpdateDeployServiceAgent(ctx, deployServiceClient, clusterNameConfig.GetClusterName(), cloudStableVersion, ownershipTags); err != nil {
				process.log.Warningf("Failed to update deploy service agents: %v", err)

				// Release the semaphore lease on failure so that another instance may attempt the update
				if err := authClient.CancelSemaphoreLease(ctx, *semLock); err != nil {
					process.log.WithError(err).Error("Failed to cancel semaphore lease")
				}
			}
		}
	}
	return nil
}
