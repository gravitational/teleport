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
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/eks/eksiface"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
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
		c.Log = logrus.New()
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
		clusters types.KubeClusters
		mu       sync.Mutex
	)
	err := a.Client.ListClustersPagesWithContext(ctx,
		&eks.ListClustersInput{
			Include: nil, // For now we should only list EKS clusters
		},
		func(clustersList *eks.ListClustersOutput, _ bool) bool {
			wg := &sync.WaitGroup{}
			wg.Add(len(clustersList.Clusters))
			for i := 0; i < len(clustersList.Clusters); i++ {
				go func(clusterName string) {
					defer wg.Done()

					logger := a.Log.WithField("cluster_name", clusterName)

					rsp, err := a.Client.DescribeClusterWithContext(
						ctx,
						&eks.DescribeClusterInput{
							Name: aws.String(clusterName),
						},
					)
					if err != nil {
						a.Log.WithError(err).Warnf("Unable to describe EKS cluster %q", clusterName)
						return
					}

					if match, reason, err := services.MatchLabels(a.FilterLabels, a.awsEKSTagsToLabels(rsp.Cluster.Tags)); err != nil {
						logger.WithError(err).Warnf("Unable to match EKS cluster labels against match labels")
						return
					} else if !match {
						a.Log.Debugf("EKS cluster labels does not match the selector: %s", reason)
						return
					}

					cluster, err := services.NewKubeClusterFromAWSEKS(rsp.Cluster)
					if err != nil {
						logger.WithError(err).Warnf("unable to convert eks.Cluster into types.KubernetesClusterV3")
						return
					}

					mu.Lock()
					defer mu.Unlock()
					clusters = append(clusters, cluster)
				}(aws.StringValue(clustersList.Clusters[i]))
			}
			wg.Wait()
			return true
		},
	)
	return clusters, trace.Wrap(err)
}

func (a *eksFetcher) ResourceType() string {
	return types.KindKubernetesCluster
}
func (a *eksFetcher) Cloud() string {
	return AWS
}

// awsEKSTagsToLabels converts EKS tags to a labels map.
func (a *eksFetcher) awsEKSTagsToLabels(tags map[string]*string) map[string]string {
	labels := make(map[string]string)
	for key, val := range tags {
		if types.IsValidLabelKey(key) {
			labels[key] = aws.StringValue(val)
		} else {
			a.Log.Debugf("Skipping EKS tag %q, not a valid label key.", key)
		}
	}
	return labels
}
