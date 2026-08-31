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
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// MakeAKSFetchersFromAzureMatchers creates Azure AKS fetchers from the provided matchers.
func MakeAKSFetchersFromAzureMatchers(
	ctx context.Context,
	logger *slog.Logger,
	getAzureClients func(context.Context, string) (azure.Clients, error),
	matchers []types.AzureMatcher,
	discoveryConfigName string,
) ([]common.Fetcher, error) {
	var kubeFetchers []common.Fetcher
	for _, matcher := range services.SimplifyAzureMatchers(matchers) {
		if !slices.Contains(matcher.Types, types.AzureMatcherKubernetes) {
			continue
		}

		azureClients, err := getAzureClients(ctx, matcher.Integration)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		fetcher, err := newAKSFetcher(aksFetcherConfig{
			Logger:              logger,
			AzureClients:        azureClients,
			Subscriptions:       matcher.Subscriptions,
			Regions:             matcher.Regions,
			ResourceGroups:      matcher.ResourceGroups,
			FilterLabels:        matcher.ResourceTags,
			Integration:         matcher.Integration,
			DiscoveryConfigName: discoveryConfigName,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeFetchers = append(kubeFetchers, fetcher)
	}

	return kubeFetchers, nil
}

type aksFetcher struct {
	aksFetcherConfig
}

// aksFetcherConfig configures the AKS fetcher.
type aksFetcherConfig struct {
	// AzureClients is the Azure clients used to fetch Azure Subscriptions (when using wildcard) and AKS clusters.
	AzureClients azure.Clients
	// Subscriptions are the Azure subscriptions to fetch AKS clusters from.
	Subscriptions []string
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
	// Integration is the name of Azure integration used for auth.
	Integration string
}

// checkAndSetDefaults validates and sets the defaults values.
func (c *aksFetcherConfig) checkAndSetDefaults() error {
	if c.AzureClients == nil {
		return trace.BadParameter("missing AzureClients field")
	}
	if len(c.Regions) == 0 {
		return trace.BadParameter("missing Regions field")
	}

	if len(c.FilterLabels) == 0 {
		return trace.BadParameter("missing FilterLabels field")
	}

	if len(c.ResourceGroups) == 0 {
		return trace.BadParameter("missing ResourceGroups field")
	}

	if len(c.Subscriptions) == 0 {
		return trace.BadParameter("missing Subscriptions field")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "fetcher:aks")
	}
	return nil
}

// newAKSFetcher creates a new AKS fetcher configuration.
func newAKSFetcher(cfg aksFetcherConfig) (common.Fetcher, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
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

// getAKSClusters fetches all AKS clusters from the configured subscriptions and resource groups.
// If there's an error fetching clusters from a subscription, it will continue to the next subscription and log the errors at the end.
func (a *aksFetcher) getAKSClusters(ctx context.Context) ([]*azure.AKSCluster, error) {
	subscriptions, err := azure.ExpandSubscriptionIDs(ctx, a.AzureClients, a.Subscriptions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If there are no subscriptions to fetch, return an empty list of clusters.
	// This will remove any previously discovered clusters but might be a valid state if the user has removed access to all subscriptions to the client (integration or ambient credentials).
	if len(subscriptions) == 0 {
		return nil, nil
	}

	var (
		clusters []*azure.AKSCluster
		errs     []error
		mu       sync.Mutex
	)

	group, groupCtx := errgroup.WithContext(ctx)
	// Work in parallel up to a limit of 5 subscriptions.
	group.SetLimit(concurrencyLimit)

	for _, subscription := range subscriptions {
		group.Go(func() error {
			subscriptionClusters, err := a.getAKSClusterInSubscription(groupCtx, subscription)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, trace.Wrap(err, "subscription %q", subscription))
			} else {
				clusters = append(clusters, subscriptionClusters...)
			}

			return nil
		})
	}

	_ = group.Wait()

	switch {
	// All subscriptions failed, return an aggregated error.
	case len(subscriptions) == len(errs):
		return nil, trace.NewAggregate(errs...)
	// Some subscriptions failed, log the errors and return the clusters from the successful subscriptions.
	case len(errs) > 0:
		a.Logger.WarnContext(ctx, "Failed to fetch AKS clusters in some subscriptions", "error", trace.NewAggregate(errs...))
	}

	return clusters, nil
}

func (a *aksFetcher) getAKSClusterInSubscription(ctx context.Context, subscriptionID string) ([]*azure.AKSCluster, error) {
	kubeClient, err := a.AzureClients.GetKubernetesClient(ctx, subscriptionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if a.matchAllResourceGroups() {
		clusters, err := kubeClient.ListAll(ctx)
		return clusters, trace.Wrap(err)
	}

	var clusters []*azure.AKSCluster
	for _, resourceGroup := range a.ResourceGroups {
		clustersInResourceGroup, err := kubeClient.ListWithinGroup(ctx, resourceGroup)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusters = append(clusters, clustersInResourceGroup...)
	}

	return clusters, nil
}

func (a *aksFetcher) matchAllResourceGroups() bool {
	return len(a.ResourceGroups) == 1 && a.ResourceGroups[0] == types.Wildcard
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
	return a.Integration
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
