/*
Copyright 2022 Gravitational, Inc.

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

package fetchers

import (
	"context"
	"fmt"

	containerpb "cloud.google.com/go/container/apiv1/containerpb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// GKEFetcherConfig configures the GKE fetcher.
type GKEFetcherConfig struct {
	// Client is the GCP GKE client.
	Client gcp.GKEClient
	// ProjectID is the projectID the cluster should belong to.
	ProjectID string
	// Location is the GCP's location where the clusters should be located.
	// Wildcard "*" is supported.
	Location string
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Log is the logger.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *GKEFetcherConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing Client field")
	}
	if len(c.Location) == 0 {
		return trace.BadParameter("missing Location field")
	}

	if len(c.FilterLabels) == 0 {
		return trace.BadParameter("missing FilterLabels field")
	}

	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "fetcher:gke")
	}
	return nil
}

// gkeFetcher is a GKE fetcher.
type gkeFetcher struct {
	GKEFetcherConfig
}

// NewGKEFetcher creates a new GKE fetcher configuration.
func NewGKEFetcher(cfg GKEFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &gkeFetcher{cfg}, nil
}

func (a *gkeFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := a.getGKEClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.rewriteKubeClusters(clusters)
	return clusters.AsResources(), nil
}

func (a *gkeFetcher) getGKEClusters(ctx context.Context) (types.KubeClusters, error) {
	var clusters types.KubeClusters

	gkeClusters, err := a.Client.ListClusters(ctx, a.ProjectID, a.Location)
	for _, gkeCluster := range gkeClusters {
		cluster, err := a.getMatchingKubeCluster(gkeCluster)
		// trace.CompareFailed is returned if the cluster did not match the matcher filtering labels
		// or if the cluster is not yet active.
		if trace.IsCompareFailed(err) {
			a.Log.WithError(err).Debugf("Cluster %q did not match the filtering criteria.", gkeCluster.Name)
			continue
		} else if err != nil {
			a.Log.WithError(err).Warnf("Failed to discover GKE cluster %q.", gkeCluster.Name)
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

func (a *gkeFetcher) Cloud() string {
	return types.CloudGCP
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
	cluster, err := services.NewKubeClusterFromGCPGKE(gkeCluster)
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
