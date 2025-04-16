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

	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// GKEFetcherConfig configures the GKE fetcher.
type GKEFetcherConfig struct {
	// GKEClient is the GCP GKE client.
	GKEClient gcp.GKEClient
	// ProjectClient is the GCP project client.
	ProjectClient gcp.ProjectsClient
	// ProjectID is the projectID the cluster should belong to.
	ProjectID string
	// Location is the GCP's location where the clusters should be located.
	// Wildcard "*" is supported.
	Location string
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Log is the logger.
	Logger *slog.Logger
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	DiscoveryConfigName string
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *GKEFetcherConfig) CheckAndSetDefaults() error {
	if c.GKEClient == nil {
		return trace.BadParameter("missing Client field")
	}
	if c.ProjectClient == nil {
		return trace.BadParameter("missing ProjectClient field")
	}
	if len(c.Location) == 0 {
		return trace.BadParameter("missing Location field")
	}

	if len(c.FilterLabels) == 0 {
		return trace.BadParameter("missing FilterLabels field")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "fetcher:gke")
	}
	return nil
}

// gkeFetcher is a GKE fetcher.
type gkeFetcher struct {
	GKEFetcherConfig
}

// NewGKEFetcher creates a new GKE fetcher configuration.
func NewGKEFetcher(ctx context.Context, cfg GKEFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &gkeFetcher{cfg}, nil
}

func (a *gkeFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {

	// Get the project IDs that this fetcher is configured to query.
	projectIDs, err := a.getProjectIDs(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.Logger.DebugContext(ctx, "Fetching GKE clusters for configured projects", "project_ids", projectIDs)
	var clusters types.KubeClusters
	for _, projectID := range projectIDs {
		lClusters, err := a.getGKEClusters(ctx, projectID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters = append(clusters, lClusters...)
	}

	a.rewriteKubeClusters(clusters)
	return clusters.AsResources(), nil
}

func (a *gkeFetcher) getGKEClusters(ctx context.Context, projectID string) (types.KubeClusters, error) {
	var clusters types.KubeClusters

	gkeClusters, err := a.GKEClient.ListClusters(ctx, projectID, a.Location)
	if trace.IsAccessDenied(err) {
		a.Logger.WarnContext(ctx, "Access denied to list GKE clusters", "project_id", projectID, "location", a.Location)
		return nil, nil
	} else if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, gkeCluster := range gkeClusters {
		cluster, err := a.getMatchingKubeCluster(gkeCluster)
		// trace.CompareFailed is returned if the cluster did not match the matcher filtering labels
		// or if the cluster is not yet active.
		if trace.IsCompareFailed(err) {
			a.Logger.DebugContext(ctx, "Cluster did not match the filtering criteria", "error", err, "cluster", gkeCluster.Name)
			continue
		} else if err != nil {
			a.Logger.WarnContext(ctx, "Failed to discover GKE cluster", "error", err, "cluster", gkeCluster.Name)
			continue
		}
		clusters = append(clusters, cluster)
	}

	return clusters, trace.Wrap(err)
}

// rewriteKubeClusters rewrites the discovered kube clusters.
func (a *gkeFetcher) rewriteKubeClusters(clusters types.KubeClusters) {
	for _, c := range clusters {
		common.ApplyGKENameSuffix(c)
	}
}

func (a *gkeFetcher) ResourceType() string {
	return types.KindKubernetesCluster
}

func (a *gkeFetcher) FetcherType() string {
	return types.GCPMatcherKubernetes
}

func (a *gkeFetcher) Cloud() string {
	return types.CloudGCP
}

func (a *gkeFetcher) IntegrationName() string {
	// There is currently no integration that supports Auto Discover for GCP resources.
	return ""
}
func (a *gkeFetcher) GetDiscoveryConfigName() string {
	return a.DiscoveryConfigName
}

func (a *gkeFetcher) String() string {
	return fmt.Sprintf("gkeFetcher(ProjectID=%v, Location=%v, FilterLabels=%v)",
		a.ProjectID, a.Location, a.FilterLabels)
}

// getMatchingKubeCluster checks if the GKE cluster tags matches the GCP matcher
// filtering labels. It also excludes GKE clusters that are not Running/Degraded/Reconciling.
// If any cluster does not match the filtering criteria, this function returns
// a “trace.CompareFailed“ error to distinguish filtering and operational errors.
func (a *gkeFetcher) getMatchingKubeCluster(gkeCluster gcp.GKECluster) (types.KubeCluster, error) {
	cluster, err := common.NewKubeClusterFromGCPGKE(gkeCluster)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to create types.KubernetesClusterV3 cluster from gcp.GKECluster.")
	}

	if match, reason, err := services.MatchLabels(a.FilterLabels, cluster.GetAllLabels()); err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to match GKE cluster labels against match labels.")
	} else if !match {
		return nil, trace.CompareFailed("GKE cluster %q labels does not match the selector: %s", gkeCluster.Name, reason)
	}

	switch st := gkeCluster.Status; st {
	case containerpb.Cluster_RUNNING, containerpb.Cluster_RECONCILING, containerpb.Cluster_DEGRADED:
	default:
		return nil, trace.CompareFailed("GKE cluster %q not enrolled due to its current status: %s", gkeCluster.Name, st)
	}

	return cluster, nil
}

// getProjectIDs returns the project ids that this fetcher is configured to query.
// This will make an API call to list project IDs when the fetcher is configured to match "*" projectID,
// in order to discover and query new projectID.
// Otherwise, a list containing the fetcher's non-wildcard project is returned.
func (a *gkeFetcher) getProjectIDs(ctx context.Context) ([]string, error) {
	if a.ProjectID != types.Wildcard {
		return []string{a.ProjectID}, nil
	}

	gcpProjects, err := a.ProjectClient.ListProjects(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var projectIDs []string
	for _, prj := range gcpProjects {
		projectIDs = append(projectIDs, prj.ID)
	}
	return projectIDs, nil
}
