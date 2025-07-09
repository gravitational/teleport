/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// EKSClientGetter returns an EKS client for aws-sync.
type EKSClientGetter func(ctx context.Context, region string, opts ...awsconfig.OptionsFn) (EKSClient, error)

// EKSClient is the subset of the EKS interface we use in aws-sync.
type EKSClient interface {
	eks.ListClustersAPIClient
	eks.DescribeClusterAPIClient

	eks.ListAccessEntriesAPIClient
	DescribeAccessEntry(ctx context.Context, params *eks.DescribeAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DescribeAccessEntryOutput, error)

	eks.ListAssociatedAccessPoliciesAPIClient
}

// pollAWSEKSClusters is a function that returns a function that fetches
// eks clusters and their access scope levels.
func (a *Fetcher) pollAWSEKSClusters(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		output, err := a.fetchAWSSEKSClusters(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch eks clusters"))
		}
		result.EKSClusters = output.clusters
		result.AssociatedAccessPolicies = output.associatedPolicies
		result.AccessEntries = output.accessEntry
		return nil
	}
}

// fetchAWSEKSClustersOutput is the output of the fetchAWSSEKSClusters function.
type fetchAWSEKSClustersOutput struct {
	clusters           []*accessgraphv1alpha.AWSEKSClusterV1
	associatedPolicies []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1
	accessEntry        []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1
}

// fetchAWSSEKSClusters fetches eks instances from all regions.
func (a *Fetcher) fetchAWSSEKSClusters(ctx context.Context) (fetchAWSEKSClustersOutput, error) {
	var (
		output   fetchAWSEKSClustersOutput
		hostsMu  sync.Mutex
		errs     []error
		existing = a.lastResult
	)
	eG, ctx := errgroup.WithContext(ctx)
	// Set the limit to 5 to avoid too many concurrent requests.
	// This is a temporary solution until we have a better way to limit the
	// number of concurrent requests.
	eG.SetLimit(5)
	collectClusters := func(cluster *accessgraphv1alpha.AWSEKSClusterV1,
		clusterAssociatedPolicies []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1,
		clusterAccessEntries []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1,
		err error,
	) {
		hostsMu.Lock()
		defer hostsMu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if cluster != nil {
			output.clusters = append(output.clusters, cluster)
		}
		output.associatedPolicies = append(output.associatedPolicies, clusterAssociatedPolicies...)
		output.accessEntry = append(output.accessEntry, clusterAccessEntries...)
	}

	for _, region := range a.Regions {
		eG.Go(func() error {
			eksClient, err := a.GetEKSClient(ctx, region, a.getAWSOptions()...)
			if err != nil {
				collectClusters(nil, nil, nil, trace.Wrap(err))
				return nil
			}

			var eksClusterNames []string

			for p := eks.NewListClustersPaginator(eksClient, nil); p.HasMorePages(); {
				out, err := p.NextPage(ctx)
				if err != nil {
					oldEKSClusters := sliceFilter(existing.EKSClusters, func(cluster *accessgraphv1alpha.AWSEKSClusterV1) bool {
						return cluster.Region == region && cluster.AccountId == a.AccountID
					})
					oldAccessEntries := sliceFilter(existing.AccessEntries, func(ae *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) bool {
						return ae.Cluster.Region == region && ae.AccountId == a.AccountID
					})
					oldAssociatedPolicies := sliceFilter(existing.AssociatedAccessPolicies, func(ap *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) bool {
						return ap.Cluster.Region == region && ap.AccountId == a.AccountID
					})
					hostsMu.Lock()
					output.clusters = append(output.clusters, oldEKSClusters...)
					output.associatedPolicies = append(output.associatedPolicies, oldAssociatedPolicies...)
					output.accessEntry = append(output.accessEntry, oldAccessEntries...)
					hostsMu.Unlock()
					break
				}
				eksClusterNames = append(eksClusterNames, out.Clusters...)
			}

			for _, cluster := range eksClusterNames {
				oldCluster := sliceFilterPickFirst(existing.EKSClusters, func(c *accessgraphv1alpha.AWSEKSClusterV1) bool {
					return c.Name == cluster && c.AccountId == a.AccountID && c.Region == region
				})
				oldAccessEntries := sliceFilter(existing.AccessEntries, func(ae *accessgraphv1alpha.AWSEKSClusterAccessEntryV1) bool {
					return ae.Cluster.Name == cluster && ae.AccountId == a.AccountID && ae.Cluster.Region == region
				})
				oldAssociatedPolicies := sliceFilter(existing.AssociatedAccessPolicies, func(ap *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1) bool {
					return ap.Cluster.Name == cluster && ap.AccountId == a.AccountID && ap.Cluster.Region == region
				})
				// DescribeClusterWithContext retrieves the cluster details.
				cluster, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{
					Name: aws.String(cluster),
				},
				)
				if err != nil {
					collectClusters(oldCluster, oldAssociatedPolicies, oldAccessEntries, trace.Wrap(err))
					return nil
				}
				protoCluster := awsEKSClusterToProtoCluster(cluster.Cluster, region, a.AccountID)

				// if eks cluster only allows CONFIGMAP auth, skip polling of access entries and
				// associated policies.
				if cluster.Cluster != nil && cluster.Cluster.AccessConfig != nil &&
					cluster.Cluster.AccessConfig.AuthenticationMode == ekstypes.AuthenticationModeConfigMap {
					collectClusters(protoCluster, nil, nil, nil)
					continue
				}
				// fetchAccessEntries retries the list of configured access entries
				accessEntries, err := a.fetchAccessEntries(ctx, eksClient, protoCluster)
				if err != nil {
					collectClusters(protoCluster, oldAssociatedPolicies, oldAccessEntries, trace.Wrap(err))
				}

				accessEntryARNs := make([]string, 0, len(accessEntries))
				for _, accessEntry := range accessEntries {
					accessEntryARNs = append(
						accessEntryARNs,
						accessEntry.PrincipalArn,
					)
				}

				associatedPolicies, err := a.fetchAssociatedPolicies(ctx, eksClient, protoCluster, accessEntryARNs)
				if err != nil {
					collectClusters(protoCluster, oldAssociatedPolicies, accessEntries, trace.Wrap(err))
				}
				collectClusters(protoCluster, associatedPolicies, accessEntries, nil)
			}
			return nil
		})
	}

	err := eG.Wait()
	return output, trace.NewAggregate(append(errs, err)...)
}

// awsEKSClusterToProtoCluster converts an eks.Cluster to accessgraphv1alpha.AWSEKSClusterV1
// representation.
func awsEKSClusterToProtoCluster(cluster *ekstypes.Cluster, region, accountID string) *accessgraphv1alpha.AWSEKSClusterV1 {
	var tags []*accessgraphv1alpha.AWSTag
	for k, v := range cluster.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   k,
			Value: wrapperspb.String(v),
		})
	}

	return &accessgraphv1alpha.AWSEKSClusterV1{
		Name:         aws.ToString(cluster.Name),
		Arn:          aws.ToString(cluster.Arn),
		CreatedAt:    awsTimeToProtoTime(cluster.CreatedAt),
		Status:       string(cluster.Status),
		Region:       region,
		AccountId:    accountID,
		Tags:         tags,
		LastSyncTime: timestamppb.Now(),
	}
}

// fetchAccessEntries fetches the access entries for the given cluster.
func (a *Fetcher) fetchAccessEntries(ctx context.Context, eksClient EKSClient, cluster *accessgraphv1alpha.AWSEKSClusterV1) ([]*accessgraphv1alpha.AWSEKSClusterAccessEntryV1, error) {
	var accessEntries []string

	for p := eks.NewListAccessEntriesPaginator(eksClient,
		&eks.ListAccessEntriesInput{ClusterName: aws.String(cluster.Name)},
	); p.HasMorePages(); {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		accessEntries = append(accessEntries, out.AccessEntries...)
	}

	var errs []error
	var protoAccessEntries []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1
	for _, accessEntry := range accessEntries {
		rsp, err := eksClient.DescribeAccessEntry(
			ctx,
			&eks.DescribeAccessEntryInput{
				PrincipalArn: aws.String(accessEntry),
				ClusterName:  aws.String(cluster.Name),
			},
		)
		if err != nil {
			errs = append(errs, trace.Wrap(err))
			continue
		}
		protoAccessEntry := awsAccessEntryToProtoAccessEntry(
			rsp.AccessEntry,
			cluster,
			a.AccountID,
		)
		protoAccessEntries = append(protoAccessEntries, protoAccessEntry)
	}

	return protoAccessEntries, trace.NewAggregate(errs...)
}

// awsAccessEntryToProtoAccessEntry converts an eks.AccessEntry to accessgraphv1alpha.AWSEKSClusterV1
func awsAccessEntryToProtoAccessEntry(accessEntry *ekstypes.AccessEntry, cluster *accessgraphv1alpha.AWSEKSClusterV1, accountID string) *accessgraphv1alpha.AWSEKSClusterAccessEntryV1 {
	tags := make([]*accessgraphv1alpha.AWSTag, 0, len(accessEntry.Tags))
	for k, v := range accessEntry.Tags {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   k,
			Value: wrapperspb.String(v),
		})
	}

	return &accessgraphv1alpha.AWSEKSClusterAccessEntryV1{
		Cluster:          cluster,
		AccessEntryArn:   aws.ToString(accessEntry.AccessEntryArn),
		CreatedAt:        awsTimeToProtoTime(accessEntry.CreatedAt),
		KubernetesGroups: accessEntry.KubernetesGroups,
		Username:         aws.ToString(accessEntry.Username),
		ModifiedAt:       awsTimeToProtoTime(accessEntry.ModifiedAt),
		PrincipalArn:     aws.ToString(accessEntry.PrincipalArn),
		Type:             aws.ToString(accessEntry.Type),
		Tags:             tags,
		AccountId:        accountID,
		LastSyncTime:     timestamppb.Now(),
	}
}

// fetchAccessEntries fetches the access entries for the given cluster.
func (a *Fetcher) fetchAssociatedPolicies(ctx context.Context, eksClient EKSClient, cluster *accessgraphv1alpha.AWSEKSClusterV1, arns []string) ([]*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1, error) {
	var associatedPolicies []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1
	var errs []error

	for _, arn := range arns {
		for p := eks.NewListAssociatedAccessPoliciesPaginator(eksClient,
			&eks.ListAssociatedAccessPoliciesInput{
				ClusterName:  aws.String(cluster.Name),
				PrincipalArn: aws.String(arn),
			},
		); p.HasMorePages(); {
			out, err := p.NextPage(ctx)
			if err != nil {
				errs = append(errs, err)
				break
			}
			for _, policy := range out.AssociatedAccessPolicies {
				associatedPolicies = append(associatedPolicies,
					awsAssociatedAccessPolicy(policy, cluster, arn, a.AccountID),
				)
			}
		}
	}

	return associatedPolicies, trace.NewAggregate(errs...)
}

// awsAssociatedAccessPolicy converts an eks.AssociatedAccessPolicy to accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1
func awsAssociatedAccessPolicy(policy ekstypes.AssociatedAccessPolicy, cluster *accessgraphv1alpha.AWSEKSClusterV1, principalARN, accountID string) *accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1 {
	var accessScope *accessgraphv1alpha.AWSEKSAccessScopeV1
	if policy.AccessScope != nil {
		accessScope = &accessgraphv1alpha.AWSEKSAccessScopeV1{
			Namespaces: policy.AccessScope.Namespaces,
			Type:       string(policy.AccessScope.Type),
		}
	}

	return &accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1{
		Cluster:      cluster,
		AssociatedAt: awsTimeToProtoTime(policy.AssociatedAt),
		ModifiedAt:   awsTimeToProtoTime(policy.ModifiedAt),
		PrincipalArn: principalARN,
		PolicyArn:    aws.ToString(policy.PolicyArn),
		Scope:        accessScope,
		AccountId:    accountID,
		LastSyncTime: timestamppb.Now(),
	}
}
