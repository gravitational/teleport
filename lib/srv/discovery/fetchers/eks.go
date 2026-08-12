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
	"github.com/gravitational/teleport/lib/utils/aws/organizations"
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
	// OrganizationsClientGetter lists the accounts of an AWS Organization.
	// Required when the matcher has an organization matcher.
	OrganizationsClientGetter organizations.ClientGetter
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

// MatchersToEKSFetchersParams are the inputs shared by every fetcher built from a
// set of matchers.
type MatchersToEKSFetchersParams struct {
	// Matchers are the AWS matchers to build EKS fetchers from. Matchers that do
	// not include the eks type are ignored.
	Matchers []types.AWSMatcher
	// ClientGetter retrieves an EKS client and an STS client.
	ClientGetter AWSClientGetter
	// RegionsListerGetter lists AWS regions enabled for an account.
	RegionsListerGetter awsregions.ListerGetter
	// OrganizationsClientGetter lists the accounts of an AWS Organization.
	OrganizationsClientGetter organizations.ClientGetter
	// DiscoveryConfigName is the name of the discovery config which originated the matchers.
	DiscoveryConfigName string
	// Logger is the logger.
	Logger *slog.Logger
}

// MakeEKSFetchersFromAWSMatchers creates fetchers from the provided matchers.
// Emits one fetcher per matcher. Wildcard regions are expanded at fetch time.
func MakeEKSFetchersFromAWSMatchers(params MatchersToEKSFetchersParams) ([]common.Fetcher, error) {
	var kubeFetchers []common.Fetcher
	for _, matcher := range params.Matchers {
		if !slices.Contains(matcher.Types, types.AWSMatcherEKS) {
			continue
		}
		fetcher, err := NewEKSFetcher(EKSFetcherConfig{
			Matcher:                   matcher,
			ClientGetter:              params.ClientGetter,
			RegionsListerGetter:       params.RegionsListerGetter,
			OrganizationsClientGetter: params.OrganizationsClientGetter,
			DiscoveryConfigName:       params.DiscoveryConfigName,
			Logger:                    params.Logger,
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
	assumeRoles, err := f.accountAssumeRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var resources types.ResourcesWithLabels
	// A name override label set in more than one account collapses two clusters
	// onto one Teleport name, and the reconciler would drop one without a trace.
	discoveredNames := make(map[string]types.KubeAWS)

	for _, assumeRole := range assumeRoles {
		var roleARN string
		if assumeRole != nil {
			roleARN = assumeRole.RoleARN
		}
		awsOpts := credentialOpts(assumeRole, f.Matcher.Integration)

		regions, err := f.regions(ctx, awsOpts)
		if err != nil {
			if !f.Matcher.HasOrganizationMatcher() {
				return nil, trace.Wrap(err)
			}
			f.Logger.WarnContext(ctx, "Failed to resolve regions for account, skipping",
				"assume_role_arn", roleARN, "error", err)
			continue
		}

		for _, region := range regions {
			eksClient, err := f.regionClient(ctx, region, awsOpts)
			if err != nil {
				f.Logger.WarnContext(ctx, "Failed to initialize EKS client for region, skipping",
					"region", region, "assume_role_arn", roleARN, "error", err)
				continue
			}
			clusters, err := f.findClustersInRegion(ctx, eksClient, assumeRole)
			if err != nil {
				f.Logger.WarnContext(ctx, "Failed to discover EKS clusters in region, skipping",
					"region", region, "assume_role_arn", roleARN, "error", err)
				continue
			}
			for _, cluster := range clusters {
				meta := cluster.GetAWSConfig()
				if kept, ok := discoveredNames[cluster.GetName()]; ok {
					f.Logger.WarnContext(ctx, "Skipping EKS cluster whose Teleport name is already taken by another discovered cluster",
						"name", cluster.GetName(),
						"skipped_account_id", meta.AccountID, "skipped_region", meta.Region,
						"kept_account_id", kept.AccountID, "kept_region", kept.Region)
					continue
				}
				discoveredNames[cluster.GetName()] = meta
				resources = append(resources, cluster)
			}
		}
	}
	return resources, nil
}

// accountAssumeRoles returns one role to assume per account to search. Outside
// organization mode the only entry is the matcher's own assume role, which is nil
// when the matcher uses ambient or integration credentials directly.
func (f *eksFetcher) accountAssumeRoles(ctx context.Context) ([]*types.AssumeRole, error) {
	if !f.Matcher.HasOrganizationMatcher() {
		return []*types.AssumeRole{f.Matcher.AssumeRole}, nil
	}

	if f.OrganizationsClientGetter == nil {
		return nil, trace.BadParameter("missing OrganizationsClientGetter field, which is required to discover accounts under an AWS organization")
	}
	if f.Matcher.AssumeRole == nil || f.Matcher.AssumeRole.RoleName == "" {
		return nil, trace.BadParameter("assume role name is required when using AWS organization discovery")
	}

	orgsClient, err := f.OrganizationsClientGetter(ctx,
		awsconfig.WithCredentialsMaybeIntegration(awsconfig.IntegrationMetadata{Name: f.Matcher.Integration}),
	)
	if err != nil {
		return nil, trace.Wrap(awslib.ConvertRequestFailureError(err))
	}

	var includeOUs, excludeOUs []string
	if f.Matcher.Organization.OrganizationalUnits != nil {
		includeOUs = f.Matcher.Organization.OrganizationalUnits.Include
		excludeOUs = f.Matcher.Organization.OrganizationalUnits.Exclude
	}

	accounts, err := organizations.MatchingAccounts(ctx, f.Logger, orgsClient, organizations.MatchingAccountsFilter{
		OrganizationID: f.Matcher.Organization.OrganizationID,
		IncludeOUs:     includeOUs,
		ExcludeOUs:     excludeOUs,
	})
	if err != nil {
		return nil, trace.Wrap(awslib.ConvertRequestFailureError(err))
	}

	roleARNs, err := accounts.AssumeRoleARNs(f.Matcher.AssumeRole.RoleName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	assumeRoles := make([]*types.AssumeRole, 0, len(roleARNs))
	for _, roleARN := range roleARNs {
		assumeRoles = append(assumeRoles, &types.AssumeRole{
			RoleARN:    roleARN,
			ExternalID: f.Matcher.AssumeRole.ExternalID,
		})
	}
	return assumeRoles, nil
}

// regions returns the regions to search in one account, expanding the wildcard
// region with that account's credentials.
func (f *eksFetcher) regions(ctx context.Context, awsOpts []awsconfig.OptionsFn) ([]string, error) {
	if !f.Matcher.IsRegionWildcard() {
		return f.Matcher.Regions, nil
	}

	enabled, err := awsregions.ListEnabledRegions(ctx, f.RegionsListerGetter, awsOpts...)
	if err != nil {
		if trace.IsAccessDenied(err) {
			return nil, trace.BadParameter("Missing account:ListRegions permission in IAM Role, which is required to iterate over all regions. " +
				"Add this permission to the IAM Role, or enumerate the regions explicitly.")
		}
		return nil, trace.Wrap(err)
	}
	if len(enabled) == 0 {
		return nil, trace.Errorf("account:ListRegions returned no enabled regions")
	}
	return enabled, nil
}

// regionClient builds an EKS client scoped to one region with one account's
// read-side credentials.
func (f *eksFetcher) regionClient(ctx context.Context, region string, awsOpts []awsconfig.OptionsFn) (EKSClient, error) {
	cfg, err := f.ClientGetter.GetConfig(ctx, region, awsOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f.ClientGetter.GetAWSEKSClient(cfg), nil
}

// credentialOpts builds AWS config options that assume assumeRole on top of the
// integration's credentials. Callers pass an empty integration to provision access
// with the discovery service's own credentials.
func credentialOpts(assumeRole *types.AssumeRole, integration string) []awsconfig.OptionsFn {
	role := types.AssumeRole{}
	if assumeRole != nil {
		role = *assumeRole
	}
	return getAWSOpts(role, integration)
}

// findClustersInRegion lists EKS clusters reachable through eksClient and returns
// the ones matching the matcher. Per-cluster errors are logged and swallowed so one
// bad cluster cannot abort the region.
func (f *eksFetcher) findClustersInRegion(ctx context.Context, eksClient EKSClient, assumeRole *types.AssumeRole) ([]*DiscoveredEKSCluster, error) {
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
				cluster, err := f.getMatchingKubeCluster(groupCtx, eksClient, clusterName, assumeRole)
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
func (f *eksFetcher) getMatchingKubeCluster(ctx context.Context, eksClient EKSClient, clusterName string, assumeRole *types.AssumeRole) (*DiscoveredEKSCluster, error) {
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
		AssumeRole:             assumeRole,
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
