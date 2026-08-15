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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	awsregions "github.com/gravitational/teleport/lib/cloud/aws/regions"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

const (
	concurrencyLimit = 5
)

type eksFetcher struct {
	EKSFetcherConfig
}

// EKSClient is the subset of the EKS interface we use in fetchers.
type EKSClient interface {
	eks.DescribeClusterAPIClient
	eks.ListClustersAPIClient

	AssociateAccessPolicy(ctx context.Context, params *eks.AssociateAccessPolicyInput, optFns ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error)
	CreateAccessEntry(ctx context.Context, params *eks.CreateAccessEntryInput, optFns ...func(*eks.Options)) (*eks.CreateAccessEntryOutput, error)
	DeleteAccessEntry(ctx context.Context, params *eks.DeleteAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DeleteAccessEntryOutput, error)
	DescribeAccessEntry(ctx context.Context, params *eks.DescribeAccessEntryInput, optFns ...func(*eks.Options)) (*eks.DescribeAccessEntryOutput, error)
	UpdateAccessEntry(ctx context.Context, params *eks.UpdateAccessEntryInput, optFns ...func(*eks.Options)) (*eks.UpdateAccessEntryOutput, error)
}

// STSClient is the subset of the STS interface we use in fetchers.
type STSClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
	stscreds.AssumeRoleAPIClient
}

// STSPresignClient is the subset of the STS presign interface we use in fetchers.
type STSPresignClient = kubeutils.STSPresignClient

// IAMClient is the subset of the IAM interface we use in fetchers.
type IAMClient interface {
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
}

// AWSClientGetter is an interface for getting an EKS client and an STS client.
type AWSClientGetter interface {
	awsconfig.Provider
	// GetAWSEKSClient returns AWS EKS client for the specified config.
	GetAWSEKSClient(aws.Config) EKSClient
	// GetAWSSTSClient returns AWS STS client for the specified config.
	GetAWSSTSClient(aws.Config) STSClient
	// GetAWSSTSPresignClient returns AWS STS presign client for the specified config.
	GetAWSSTSPresignClient(aws.Config) STSPresignClient
	// GetAWSIAMClient returns AWS IAM client for the specified config.
	GetAWSIAMClient(aws.Config) IAMClient
}

// EKSFetcherConfig configures the EKS fetcher.
type EKSFetcherConfig struct {
	// ClientGetter retrieves an EKS client and an STS client.
	ClientGetter AWSClientGetter
	// Matcher is the AWS matcher with discovery rules: regions, tags,
	// integration, assume role, access setup.
	Matcher types.AWSMatcher
	// RegionsListerGetter lists AWS regions enabled for the caller's account.
	// Required to expand the wildcard region.
	RegionsListerGetter awsregions.ListerGetter
	// DiscoveryConfigName is the name of the discovery config which originated the resource.
	// Might be empty when the fetcher is using static matchers:
	// ie teleport.yaml/discovery_service.<cloud>.<matcher>
	DiscoveryConfigName string
	// Logger is the logger.
	Logger *slog.Logger
}

// CheckAndSetDefaults validates and sets the defaults values.
func (c *EKSFetcherConfig) CheckAndSetDefaults() error {
	if c.ClientGetter == nil {
		return trace.BadParameter("missing ClientGetter field")
	}
	if len(c.Matcher.Regions) == 0 {
		return trace.BadParameter("missing Matcher.Regions field")
	}
	if c.Matcher.IsRegionWildcard() && c.RegionsListerGetter == nil {
		return trace.BadParameter("missing RegionsListerGetter field for wildcard region matcher")
	}
	if len(c.Matcher.Tags) == 0 {
		return trace.BadParameter("missing Matcher.Tags field")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "fetcher:eks")
	}

	return nil
}

// MakeEKSFetchersFromAWSMatchers creates fetchers from the provided matchers.
// Emits one fetcher per matcher. Wildcard regions are expanded at fetch time.
func MakeEKSFetchersFromAWSMatchers(
	logger *slog.Logger,
	clients AWSClientGetter,
	regionsListerGetter awsregions.ListerGetter,
	matchers []types.AWSMatcher,
	discoveryConfigName string,
) ([]common.Fetcher, error) {
	var kubeFetchers []common.Fetcher
	for _, matcher := range matchers {
		if !slices.Contains(matcher.Types, types.AWSMatcherEKS) {
			continue
		}
		fetcher, err := NewEKSFetcher(EKSFetcherConfig{
			ClientGetter:        clients,
			Matcher:             matcher,
			RegionsListerGetter: regionsListerGetter,
			Logger:              logger,
			DiscoveryConfigName: discoveryConfigName,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		kubeFetchers = append(kubeFetchers, fetcher)
	}
	return kubeFetchers, nil
}

// NewEKSFetcher creates a new EKS fetcher.
func NewEKSFetcher(cfg EKSFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &eksFetcher{EKSFetcherConfig: cfg}, nil
}

// GetIntegration returns the integration name that is used for getting credentials of the fetcher.
func (f *eksFetcher) GetIntegration() string {
	return f.Matcher.Integration
}

type DiscoveredEKSCluster struct {
	types.KubeCluster
	awsCluster *ekstypes.Cluster

	Integration            string
	EnableKubeAppDiscovery bool

	AssumeRole        *types.AssumeRole
	SetupAccessForARN string
}

func (d *DiscoveredEKSCluster) GetIntegration() string {
	return d.Integration
}

func (d *DiscoveredEKSCluster) GetKubeAppDiscovery() bool {
	return d.EnableKubeAppDiscovery
}

func (d *DiscoveredEKSCluster) GetKubeCluster() types.KubeCluster {
	return d.KubeCluster
}

func (d *DiscoveredEKSCluster) GetAssumeRole() *types.AssumeRole {
	return d.AssumeRole
}

func (d *DiscoveredEKSCluster) GetAssumeRoleARN() string {
	if d.AssumeRole == nil {
		return ""
	}
	return d.AssumeRole.RoleARN
}

func (d *DiscoveredEKSCluster) GetSetupAccessForARN() string {
	return d.SetupAccessForARN
}

func (f *eksFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	regions := f.Matcher.Regions
	if f.Matcher.IsRegionWildcard() {
		enabled, err := awsregions.ListEnabledRegions(ctx, f.RegionsListerGetter, matcherCredentialOpts(f.Matcher)...)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return nil, trace.BadParameter("Missing account:ListRegions permission in IAM Role, which is required to iterate over all regions. " +
					"Add this permission to the IAM Role, or enumerate the regions explicitly.")
			}
			return nil, trace.Wrap(err)
		}
		regions = enabled
	}
	if len(regions) == 0 {
		return nil, trace.Errorf("account:ListRegions returned no enabled regions")
	}

	var resources types.ResourcesWithLabels
	for _, region := range regions {
		eksClient, err := f.regionClient(ctx, region)
		if err != nil {
			f.Logger.WarnContext(ctx, "Failed to initialize EKS client for region, skipping",
				"region", region, "error", err)
			continue
		}
		clusters, err := f.findClustersInRegion(ctx, eksClient)
		if err != nil {
			f.Logger.WarnContext(ctx, "Failed to discover EKS clusters in region, skipping",
				"region", region, "error", err)
			continue
		}
		for _, cluster := range clusters {
			resources = append(resources, cluster)
		}
	}
	return resources, nil
}

// regionClient builds an EKS client scoped to one region with the matcher's
// read-side credentials.
func (f *eksFetcher) regionClient(ctx context.Context, region string) (EKSClient, error) {
	cfg, err := f.ClientGetter.GetConfig(ctx, region, matcherCredentialOpts(f.Matcher)...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f.ClientGetter.GetAWSEKSClient(cfg), nil
}

// matcherCredentialOpts builds the AWS config options for discovery's read side:
// listing and describing clusters with the matcher's credentials, including its
// integration. Access provisioning uses accessCredentialOpts, which omits the
// integration.
func matcherCredentialOpts(m types.AWSMatcher) []awsconfig.OptionsFn {
	var assumeRole types.AssumeRole
	if m.AssumeRole != nil {
		assumeRole = *m.AssumeRole
	}
	return getAWSOpts(assumeRole, m.Integration)
}

// findClustersInRegion lists EKS clusters reachable through eksClient and returns
// the ones matching the matcher. Per-cluster errors are logged and swallowed so one
// bad cluster cannot abort the region.
func (f *eksFetcher) findClustersInRegion(ctx context.Context, eksClient EKSClient) ([]*DiscoveredEKSCluster, error) {
	var (
		clusters        []*DiscoveredEKSCluster
		mu              sync.Mutex
		group, groupCtx = errgroup.WithContext(ctx)
	)
	group.SetLimit(concurrencyLimit)

	for p := eks.NewListClustersPaginator(eksClient, nil); p.HasMorePages(); {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, clusterName := range out.Clusters {
			group.Go(func() error {
				cluster, err := f.getMatchingKubeCluster(groupCtx, eksClient, clusterName)
				if trace.IsCompareFailed(err) {
					f.Logger.DebugContext(groupCtx, "Cluster did not match the filtering criteria", "error", err, "cluster", clusterName)
					return nil
				} else if err != nil {
					f.Logger.WarnContext(groupCtx, "Failed to discover EKS cluster", "error", err, "cluster", clusterName)
					return nil
				}

				mu.Lock()
				defer mu.Unlock()
				clusters = append(clusters, cluster)
				return nil
			})
		}
	}

	// The closures always return nil, so the group error is too.
	_ = group.Wait()
	return clusters, nil
}

func (f *eksFetcher) ResourceType() string {
	return types.KindKubernetesCluster
}

func (f *eksFetcher) FetcherType() string {
	return types.AWSMatcherEKS
}

func (f *eksFetcher) Cloud() string {
	return types.CloudAWS
}

func (f *eksFetcher) IntegrationName() string {
	return f.Matcher.Integration
}

func (f *eksFetcher) GetDiscoveryConfigName() string {
	return f.DiscoveryConfigName
}

func (f *eksFetcher) String() string {
	return fmt.Sprintf("eksFetcher(Regions=%v, FilterLabels=%v)",
		f.Matcher.Regions, f.Matcher.Tags)
}

// getMatchingKubeCluster describes clusterName, excludes clusters that are not ready,
// and matches the result against the matcher's labels. It returns trace.CompareFailed
// for a clean non-match to distinguish filtering from operational errors.
func (f *eksFetcher) getMatchingKubeCluster(ctx context.Context, eksClient EKSClient, clusterName string) (*DiscoveredEKSCluster, error) {
	rsp, err := eksClient.DescribeCluster(
		ctx,
		&eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		},
	)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to describe EKS cluster %q", clusterName)
	}

	switch st := rsp.Cluster.Status; st {
	case ekstypes.ClusterStatusUpdating, ekstypes.ClusterStatusActive:
		f.Logger.DebugContext(ctx, "EKS cluster status is valid", "status", st, "cluster", clusterName)
	default:
		return nil, trace.CompareFailed("EKS cluster %q not enrolled due to its current status: %s", clusterName, st)
	}

	kube, err := common.NewKubeClusterFromAWSEKS(aws.ToString(rsp.Cluster.Name), aws.ToString(rsp.Cluster.Arn), rsp.Cluster.Tags)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to convert EKS cluster %q into a Teleport kube cluster", clusterName)
	}

	if match, reason, err := services.MatchLabels(f.Matcher.Tags, kube.GetAllLabels()); err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to match EKS cluster labels against match labels.")
	} else if !match {
		return nil, trace.CompareFailed("EKS cluster %q labels does not match the selector: %s", clusterName, reason)
	}

	common.ApplyEKSNameSuffix(kube)
	return &DiscoveredEKSCluster{
		KubeCluster:            kube,
		awsCluster:             rsp.Cluster,
		Integration:            f.Matcher.Integration,
		EnableKubeAppDiscovery: f.Matcher.KubeAppDiscovery,
		AssumeRole:             f.Matcher.AssumeRole,
		SetupAccessForARN:      f.Matcher.SetupAccessForARN,
	}, nil
}

func getAWSOpts(assumeRole types.AssumeRole, integration string) []awsconfig.OptionsFn {
	return []awsconfig.OptionsFn{
		awsconfig.WithAssumeRole(
			assumeRole.RoleARN,
			assumeRole.ExternalID,
		),
		awsconfig.WithCredentialsMaybeIntegration(awsconfig.IntegrationMetadata{Name: integration}),
	}
}

func convertAWSError[T any](rsp T, err error) (T, error) {
	err = awslib.ConvertRequestFailureError(err)
	return rsp, trace.Wrap(err)
}
