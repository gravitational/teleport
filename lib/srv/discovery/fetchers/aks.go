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

package fetchers

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

type aksFetcher struct {
	AKSFetcherConfig
}

// AKSFetcherConfig configures the AKS fetcher.
type AKSFetcherConfig struct {
	// Client is the Azure AKS client.
	Client azure.AKSClient
	// Regions are the regions where the clusters should be located.
	Regions []string
	// ResourceGroups are the Azure resource groups the clusters must belong to.
	ResourceGroups []string
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Log is the logger.
	Logger *slog.Logger
	// DiscoveryConfigName is the name of the DiscoveryConfig that created this Fetcher.
	DiscoveryConfigName string
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *AKSFetcherConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing Client field")
	}
	if len(c.Regions) == 0 {
		return trace.BadParameter("missing Regions field")
	}

	if len(c.FilterLabels) == 0 {
		return trace.BadParameter("missing FilterLabels field")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "fetcher:aks")
	}
	return nil
}

// NewAKSFetcher creates a new AKS fetcher configuration.
func NewAKSFetcher(cfg AKSFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &aksFetcher{cfg}, nil
}

func (a *aksFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := a.getAKSClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var kubeClusters types.KubeClusters
	for _, cluster := range clusters {
		if !a.isRegionSupported(cluster.Location) {
			a.Logger.DebugContext(ctx, "Cluster region does not match with allowed values", "region", cluster.Location)
			continue
		}
		kubeCluster, err := common.NewKubeClusterFromAzureAKS(cluster)
		if err != nil {
			a.Logger.WarnContext(ctx, "Unable to create Kubernetes cluster from azure.AKSCluster", "error", err)
			continue
		}
		if match, reason, err := services.MatchLabels(a.FilterLabels, kubeCluster.GetAllLabels()); err != nil {
			a.Logger.WarnContext(ctx, "Unable to match AKS cluster labels against match labels", "error", err)
			continue
		} else if !match {
			a.Logger.DebugContext(ctx, "AKS cluster labels does not match the selector", "reason", reason)
			continue
		}

		kubeClusters = append(kubeClusters, kubeCluster)
	}

	a.rewriteKubeClusters(kubeClusters)
	return kubeClusters.AsResources(), nil
}

// rewriteKubeClusters rewrites the discovered kube clusters.
func (a *aksFetcher) rewriteKubeClusters(clusters types.KubeClusters) {
	for _, c := range clusters {
		common.ApplyAKSNameSuffix(c)
	}
}

func (a *aksFetcher) getAKSClusters(ctx context.Context) ([]*azure.AKSCluster, error) {
	var (
		clusters []*azure.AKSCluster
		err      error
	)
	if len(a.ResourceGroups) == 1 && a.ResourceGroups[0] == types.Wildcard {
		clusters, err = a.Client.ListAll(ctx)
	} else {
		var errs []error
		for _, resourceGroup := range a.ResourceGroups {
			lClusters, lerr := a.Client.ListWithinGroup(ctx, resourceGroup)
			if lerr != nil {
				errs = append(errs, trace.Wrap(lerr))
				continue
			}
			clusters = append(clusters, lClusters...)
		}
		err = trace.NewAggregate(errs...)
	}
	return clusters, trace.Wrap(err)
}

func (a *aksFetcher) isRegionSupported(region string) bool {
	return slices.Contains(a.Regions, types.Wildcard) || slices.Contains(a.Regions, region)
}

func (a *aksFetcher) ResourceType() string {
	return types.KindKubernetesCluster
}

func (a *aksFetcher) Cloud() string {
	return types.CloudAzure
}

func (a *aksFetcher) IntegrationName() string {
	return ""
}

func (a *aksFetcher) GetDiscoveryConfigName() string {
	return a.DiscoveryConfigName
}

func (a *aksFetcher) FetcherType() string {
	return types.AzureMatcherKubernetes
}

func (a *aksFetcher) String() string {
	return fmt.Sprintf("aksFetcher(ResourceGroups=%v, Regions=%v, FilterLabels=%v)",
		a.ResourceGroups, a.Regions, a.FilterLabels)
}
