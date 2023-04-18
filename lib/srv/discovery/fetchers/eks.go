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
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

const (
	concurrencyLimit = 5
)

type eksFetcher struct {
	EKSFetcherConfig
}

// EKSFetcherConfig configures the EKS fetcher.
type EKSFetcherConfig struct {
	// Client is the AWS eKS client.
	Client eksiface.EKSAPI
	// Region is the region where the clusters should be located.
	Region string
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Log is the logger.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *EKSFetcherConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing Client field")
	}
	if len(c.Region) == 0 {
		return trace.BadParameter("missing Region field")
	}

	if len(c.FilterLabels) == 0 {
		return trace.BadParameter("missing FilterLabels field")
	}

	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "fetcher:eks")
	}
	return nil
}

// NewEKSFetcher creates a new EKS fetcher configuration.
func NewEKSFetcher(cfg EKSFetcherConfig) (Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &eksFetcher{cfg}, nil
}

func (a *eksFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := a.getEKSClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clusters.AsResources(), nil
}

func (a *eksFetcher) getEKSClusters(ctx context.Context) (types.KubeClusters, error) {
	var (
		clusters        types.KubeClusters
		mu              sync.Mutex
		group, groupCtx = errgroup.WithContext(ctx)
	)
	group.SetLimit(concurrencyLimit)

	err := a.Client.ListClustersPagesWithContext(ctx,
		&eks.ListClustersInput{
			Include: nil, // For now we should only list EKS clusters
		},
		func(clustersList *eks.ListClustersOutput, _ bool) bool {
			for i := 0; i < len(clustersList.Clusters); i++ {
				clusterName := aws.StringValue(clustersList.Clusters[i])
				// group.Go will block if the concurrency limit is reached.
				// It will resume once any running function finishes.
				group.Go(func() error {
					cluster, err := a.getMatchingKubeCluster(groupCtx, clusterName)
					// trace.CompareFailed is returned if the cluster did not match the matcher filtering labels
					// or if the cluster is not yet active.
					if trace.IsCompareFailed(err) {
						a.Log.WithError(err).Debugf("Cluster %q did not match the filtering criteria.", clusterName)
						// never return an error otherwise we will impact discovery process
						return nil
					} else if err != nil {
						a.Log.WithError(err).Warnf("Failed to discover EKS cluster %q.", clusterName)
						// never return an error otherwise we will impact discovery process
						return nil
					}

					mu.Lock()
					defer mu.Unlock()
					clusters = append(clusters, cluster)
					return nil
				})
			}
			return true
		},
	)
	// error can be discarded since we do not return any error from group.Go closure.
	_ = group.Wait()
	return clusters, trace.Wrap(err)
}

func (a *eksFetcher) ResourceType() string {
	return types.KindKubernetesCluster
}

func (a *eksFetcher) Cloud() string {
	return types.CloudAWS
}

func (a *eksFetcher) String() string {
	return fmt.Sprintf("eksFetcher(Region=%v, FilterLabels=%v)",
		a.Region, a.FilterLabels)
}

// getMatchingKubeCluster extracts EKS cluster Tags and cluster status from EKS and checks if the cluster matches
// the AWS matcher filtering labels. It also excludes EKS clusters that are not ready.
// If any cluster does not match the filtering criteria, this function returns a “trace.CompareFailed“ error
// to distinguish filtering and operational errors.
func (a *eksFetcher) getMatchingKubeCluster(ctx context.Context, clusterName string) (types.KubeCluster, error) {
	rsp, err := a.Client.DescribeClusterWithContext(
		ctx,
		&eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		},
	)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to describe EKS cluster %q", clusterName)
	}

	switch st := aws.StringValue(rsp.Cluster.Status); st {
	case eks.ClusterStatusUpdating, eks.ClusterStatusActive:
		a.Log.WithField("cluster_name", clusterName).Debugf("EKS cluster status is valid: %s", st)
	default:
		return nil, trace.CompareFailed("EKS cluster %q not enrolled due to its current status: %s", clusterName, st)
	}

	cluster, err := services.NewKubeClusterFromAWSEKS(rsp.Cluster)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to convert eks.Cluster cluster into types.KubernetesClusterV3.")
	}

	if match, reason, err := services.MatchLabels(a.FilterLabels, cluster.GetAllLabels()); err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to match EKS cluster labels against match labels.")
	} else if !match {
		return nil, trace.CompareFailed("EKS cluster %q labels does not match the selector: %s", clusterName, reason)
	}

	return cluster, nil
}
