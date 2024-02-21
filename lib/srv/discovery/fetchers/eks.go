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
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

const (
	concurrencyLimit = 5
)

type eksFetcher struct {
	EKSFetcherConfig

	mu     sync.Mutex
	client eksiface.EKSAPI
}

// EKSClientGetter is an interface for getting an EKS client.
type EKSClientGetter interface {
	// GetAWSEKSClient returns AWS EKS client for the specified region.
	GetAWSEKSClient(ctx context.Context, region string, opts ...cloud.AWSAssumeRoleOptionFn) (eksiface.EKSAPI, error)
}

// EKSFetcherConfig configures the EKS fetcher.
type EKSFetcherConfig struct {
	// EKSClientGetter retrieves an EKS client.
	EKSClientGetter EKSClientGetter
	// AssumeRole provides a role ARN and ExternalID to assume an AWS role
	// when fetching clusters.
	AssumeRole types.AssumeRole
	// Region is the region where the clusters should be located.
	Region string
	// FilterLabels are the filter criteria.
	FilterLabels types.Labels
	// Log is the logger.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *EKSFetcherConfig) CheckAndSetDefaults() error {
	if c.EKSClientGetter == nil {
		return trace.BadParameter("missing EKSClientGetter field")
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
func NewEKSFetcher(cfg EKSFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &eksFetcher{EKSFetcherConfig: cfg}, nil
}

func (a *eksFetcher) getClient(ctx context.Context) (eksiface.EKSAPI, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		return a.client, nil
	}

	client, err := a.EKSClientGetter.GetAWSEKSClient(
		ctx,
		a.Region,
		cloud.WithAssumeRole(
			a.AssumeRole.RoleARN,
			a.AssumeRole.ExternalID,
		),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a.client = client

	return a.client, nil
}

func (a *eksFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := a.getEKSClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	a.rewriteKubeClusters(clusters)
	return clusters.AsResources(), nil
}

// rewriteKubeClusters rewrites the discovered kube clusters.
func (a *eksFetcher) rewriteKubeClusters(clusters types.KubeClusters) {
	for _, c := range clusters {
		common.ApplyEKSNameSuffix(c)
	}
}

func (a *eksFetcher) getEKSClusters(ctx context.Context) (types.KubeClusters, error) {
	var (
		clusters        types.KubeClusters
		mu              sync.Mutex
		group, groupCtx = errgroup.WithContext(ctx)
	)
	group.SetLimit(concurrencyLimit)

	client, err := a.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting AWS EKS client")
	}

	err = client.ListClustersPagesWithContext(ctx,
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

func (a *eksFetcher) FetcherType() string {
	return types.AWSMatcherEKS
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
	client, err := a.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed getting AWS EKS client")
	}

	rsp, err := client.DescribeClusterWithContext(
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

	cluster, err := services.NewKubeClusterFromAWSEKS(aws.StringValue(rsp.Cluster.Name), aws.StringValue(rsp.Cluster.Arn), rsp.Cluster.Tags)
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
