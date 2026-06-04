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

	// accessManager owns the write side of discovery: access entries and the
	// AWS discovery status. The fetcher resolves identity and matches clusters,
	// then delegates access provisioning to it.
	accessManager *EKSAccessManager
}

// regionalFetcher discovers EKS clusters in a single AWS region.
type regionalFetcher struct {
	tags          types.Labels
	logger        *slog.Logger
	eks           EKSClient
	accessManager *EKSAccessManager
	access        resolvedAccess
}

// EKSClient is the subset of the EKS interface we use in fetchers.
type EKSClient interface {
	eks.DescribeClusterAPIClient
	eks.ListClustersAPIClient

	AssociateAccessPolicy(ctx context.Context, params *eks.AssociateAccessPolicyInput, optFns ...func(*eks.Options)) (*eks.AssociateAccessPolicyOutput, error)
	DisassociateAccessPolicy(ctx context.Context, params *eks.DisassociateAccessPolicyInput, optFns ...func(*eks.Options)) (*eks.DisassociateAccessPolicyOutput, error)
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

// NewEKSFetcher creates a new EKS fetcher configuration.
func NewEKSFetcher(cfg EKSFetcherConfig) (common.Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	accessManager, err := NewEKSAccessManager(cfg.ClientGetter, cfg.Logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &eksFetcher{EKSFetcherConfig: cfg, accessManager: accessManager}, nil
}

// GetIntegration returns the integration name that is used for getting credentials of the fetcher.
func (a *eksFetcher) GetIntegration() string {
	return a.Matcher.Integration
}

type DiscoveredEKSCluster struct {
	types.KubeCluster

	Integration            string
	EnableKubeAppDiscovery bool
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

func (a *eksFetcher) Get(ctx context.Context) (types.ResourcesWithLabels, error) {
	clusters, err := a.getEKSClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resources := make(types.ResourcesWithLabels, 0, len(clusters))
	for _, cluster := range clusters {
		common.ApplyEKSNameSuffix(cluster)
		resources = append(resources, &DiscoveredEKSCluster{
			KubeCluster:            cluster,
			Integration:            a.Matcher.Integration,
			EnableKubeAppDiscovery: a.Matcher.KubeAppDiscovery,
		})
	}
	return resources, nil
}

func (a *eksFetcher) getEKSClusters(ctx context.Context) (types.KubeClusters, error) {
	regions := a.Matcher.Regions
	if a.Matcher.IsRegionWildcard() {
		enabled, err := awsregions.ListEnabledRegions(ctx, a.RegionsListerGetter, matcherCredentialOpts(a.Matcher)...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		regions = enabled
	}
	if len(regions) == 0 {
		return nil, trace.Errorf("account:ListRegions returned no enabled regions")
	}

	// Integration-enrolled clusters are provisioned by their integration, so the
	// access manager runs only for non-integration matchers.
	var resolved resolvedAccess
	if a.Matcher.Integration == "" {
		resolved = a.accessManager.resolveAccess(ctx, accessConfigFromMatcher(a.Matcher), regions)
	}

	var clusters types.KubeClusters
	for _, region := range regions {
		rf, err := a.newRegionalFetcher(ctx, region, resolved)
		if err != nil {
			a.Logger.WarnContext(ctx, "Failed to initialize regional EKS fetcher, skipping",
				"region", region, "error", err)
			continue
		}
		regionClusters, err := rf.FindClusters(ctx)
		if err != nil {
			a.Logger.WarnContext(ctx, "Failed to discover EKS clusters in region, skipping",
				"region", region, "error", err)
			continue
		}
		clusters = append(clusters, regionClusters...)
	}
	return clusters, nil
}

func (a *eksFetcher) newRegionalFetcher(ctx context.Context, region string, access resolvedAccess) (*regionalFetcher, error) {
	cfg, err := a.ClientGetter.GetConfig(ctx, region, matcherCredentialOpts(a.Matcher)...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &regionalFetcher{
		tags:          a.Matcher.Tags,
		logger:        a.Logger,
		eks:           a.ClientGetter.GetAWSEKSClient(cfg),
		accessManager: a.accessManager,
		access:        access,
	}, nil
}

// matcherCredentialOpts builds the AWS config options for discovery's read side:
// listing and describing clusters with the matcher's credentials, including its
// integration. Access provisioning uses the integration-free EKSAccessConfig.
func matcherCredentialOpts(m types.AWSMatcher) []awsconfig.OptionsFn {
	var assumeRole types.AssumeRole
	if m.AssumeRole != nil {
		assumeRole = *m.AssumeRole
	}
	return getAWSOpts(assumeRole, m.Integration)
}

// accessConfigFromMatcher extracts the access-relevant fields a matcher carries,
// translating discovery configuration into the access intent the manager needs.
func accessConfigFromMatcher(m types.AWSMatcher) EKSAccessConfig {
	return EKSAccessConfig{
		AssumeRole:        m.AssumeRole,
		SetupAccessForARN: m.SetupAccessForARN,
	}
}

// FindClusters lists EKS clusters in this region, filters them against the
// matcher, and sets up access entries where required. Per-cluster errors are
// logged and swallowed so one bad cluster cannot abort the region.
func (r *regionalFetcher) FindClusters(ctx context.Context) (types.KubeClusters, error) {
	var (
		clusters        types.KubeClusters
		mu              sync.Mutex
		group, groupCtx = errgroup.WithContext(ctx)
	)
	group.SetLimit(concurrencyLimit)

	// For now we should only list EKS clusters so we use nil (default) input param.
	for p := eks.NewListClustersPaginator(r.eks, nil); p.HasMorePages(); {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, clusterName := range out.Clusters {
			// group.Go will block if the concurrency limit is reached.
			// It will resume once any running function finishes.
			group.Go(func() error {
				cluster, err := r.getMatchingKubeCluster(groupCtx, clusterName)
				// trace.CompareFailed is returned if the cluster did not match the matcher filtering labels
				// or if the cluster is not yet active.
				if trace.IsCompareFailed(err) {
					r.logger.DebugContext(groupCtx, "Cluster did not match the filtering criteria", "error", err, "cluster", clusterName)
					return nil
				} else if err != nil {
					r.logger.WarnContext(groupCtx, "Failed to discover EKS cluster", "error", err, "cluster", clusterName)
					return nil
				}

				mu.Lock()
				defer mu.Unlock()
				clusters = append(clusters, cluster)
				return nil
			})
		}
	}

	// The error can be discarded since we do not return any error from group.Go closure.
	_ = group.Wait()
	return clusters, nil
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

func (a *eksFetcher) IntegrationName() string {
	return a.Matcher.Integration
}

func (a *eksFetcher) GetDiscoveryConfigName() string {
	return a.DiscoveryConfigName
}

func (a *eksFetcher) String() string {
	return fmt.Sprintf("eksFetcher(Regions=%v, FilterLabels=%v)",
		a.Matcher.Regions, a.Matcher.Tags)
}

// getMatchingKubeCluster extracts EKS cluster Tags and cluster status from EKS and checks if the cluster matches
// the AWS matcher filtering labels. It also excludes EKS clusters that are not ready.
// If any cluster does not match the filtering criteria, this function returns a “trace.CompareFailed“ error
// to distinguish filtering and operational errors.
func (r *regionalFetcher) getMatchingKubeCluster(ctx context.Context, clusterName string) (types.KubeCluster, error) {
	rsp, err := r.eks.DescribeCluster(
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
		r.logger.DebugContext(ctx, "EKS cluster status is valid", "status", st, "cluster", clusterName)
	default:
		return nil, trace.CompareFailed("EKS cluster %q not enrolled due to its current status: %s", clusterName, st)
	}

	cluster, err := common.NewKubeClusterFromAWSEKS(aws.ToString(rsp.Cluster.Name), aws.ToString(rsp.Cluster.Arn), rsp.Cluster.Tags)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to convert eks.Cluster cluster into types.KubernetesClusterV3.")
	}

	if match, reason, err := services.MatchLabels(r.tags, cluster.GetAllLabels()); err != nil {
		return nil, trace.WrapWithMessage(err, "Unable to match EKS cluster labels against match labels.")
	} else if !match {
		return nil, trace.CompareFailed("EKS cluster %q labels does not match the selector: %s", clusterName, reason)
	}

	// On an access error the cluster keeps its recorded status and stays in
	// discovery. A later deletion can then clean up any access entry that was
	// provisioned, and the access is retried next cycle.
	status, err := r.accessManager.Ensure(ctx, r.access, rsp.Cluster)
	if status != nil {
		cluster.SetStatus(&types.KubernetesClusterStatus{
			Discovery: &types.KubernetesClusterDiscoveryStatus{
				Aws: status,
			},
		})
	}
	if err != nil {
		r.logger.WarnContext(ctx, "Failed to provision EKS access; keeping cluster with recorded status for later cleanup",
			"error", err,
			"cluster", clusterName,
		)
	}
	return cluster, nil
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
