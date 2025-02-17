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
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/server"
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// pageSize is the default page size to use when fetching AWS resources
// from the AWS API for endpoints that support pagination.
const pageSize int32 = 500

// Config is the configuration for the AWS fetcher.
type Config struct {
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider
	// GetEKSClient gets an AWS EKS client for the given region.
	GetEKSClient EKSClientGetter
	// GetEC2Client gets an AWS EC2 client for the given region.
	GetEC2Client server.EC2ClientGetter
	// AccountID is the AWS account ID to use when fetching resources.
	AccountID string
	// Regions is the list of AWS regions to fetch resources from.
	Regions []string
	// AssumeRole is the configuration for assuming an AWS role.
	AssumeRole *AssumeRole
	// Integration is the name of the AWS integration to use when fetching resources.
	Integration string
	// DiscoveryConfigName if set, will be used to report the Discovery Config Status to the Auth Server.
	DiscoveryConfigName string

	// awsClients provides AWS SDK clients.
	awsClients awsClientProvider
}

func (c *Config) CheckAndSetDefaults() error {
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}

	if c.awsClients == nil {
		c.awsClients = defaultAWSClients{}
	}
	return nil
}

// iamClient defines a subset of the AWS IAM client API.
type iamClient interface {
	iam.ListAttachedGroupPoliciesAPIClient
	iam.ListAttachedRolePoliciesAPIClient
	iam.ListAttachedUserPoliciesAPIClient
	iam.ListGroupPoliciesAPIClient
	iam.ListGroupsAPIClient
	iam.ListGroupsForUserAPIClient
	iam.ListInstanceProfilesAPIClient
	iam.ListPoliciesAPIClient
	iam.ListRolePoliciesAPIClient
	iam.ListRolesAPIClient
	iam.ListUserPoliciesAPIClient
	iam.ListUsersAPIClient

	GetGroupPolicy(context.Context, *iam.GetGroupPolicyInput, ...func(*iam.Options)) (*iam.GetGroupPolicyOutput, error)
	GetOpenIDConnectProvider(context.Context, *iam.GetOpenIDConnectProviderInput, ...func(*iam.Options)) (*iam.GetOpenIDConnectProviderOutput, error)
	GetPolicyVersion(context.Context, *iam.GetPolicyVersionInput, ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error)
	GetRolePolicy(context.Context, *iam.GetRolePolicyInput, ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error)
	GetSAMLProvider(context.Context, *iam.GetSAMLProviderInput, ...func(*iam.Options)) (*iam.GetSAMLProviderOutput, error)
	GetUserPolicy(context.Context, *iam.GetUserPolicyInput, ...func(*iam.Options)) (*iam.GetUserPolicyOutput, error)
	ListOpenIDConnectProviders(context.Context, *iam.ListOpenIDConnectProvidersInput, ...func(*iam.Options)) (*iam.ListOpenIDConnectProvidersOutput, error)
	ListSAMLProviders(context.Context, *iam.ListSAMLProvidersInput, ...func(*iam.Options)) (*iam.ListSAMLProvidersOutput, error)
}

// stsClient defines a subset of the AWS STS client API.
type stsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// awsClientProvider provides AWS service API clients.
type awsClientProvider interface {
	// getIAMClient provides an [iamClient].
	getIAMClient(cfg aws.Config, optFns ...func(*iam.Options)) iamClient
	// getRDSClient provides an [rdsClient].
	getRDSClient(cfg aws.Config, optFns ...func(*rds.Options)) rdsClient
	// getS3Client provides an [s3Client].
	getS3Client(cfg aws.Config, optFns ...func(*s3.Options)) s3Client
	// getSTSClient provides an [stsClient].
	getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient
}

type defaultAWSClients struct{}

func (defaultAWSClients) getIAMClient(cfg aws.Config, optFns ...func(*iam.Options)) iamClient {
	return iamutils.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getRDSClient(cfg aws.Config, optFns ...func(*rds.Options)) rdsClient {
	return rds.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getS3Client(cfg aws.Config, optFns ...func(*s3.Options)) s3Client {
	return s3.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient {
	return stsutils.NewFromConfig(cfg, optFns...)
}

// AssumeRole is the configuration for assuming an AWS role.
type AssumeRole struct {
	// RoleARN is the ARN of the role to assume.
	RoleARN string
	// ExternalID is the external ID to use when assuming the role.
	ExternalID string
}

// Fetcher is a fetcher that fetches AWS resources.
type Fetcher struct {
	Config
	lastError               error
	lastDiscoveredResources uint64
	lastResult              *Resources
}

// Resources is a collection of polled AWS resources.
type Resources struct {
	// Users is the list of AWS users.
	Users []*accessgraphv1alpha.AWSUserV1
	// UserInlinePolicies is the list of inline policies configured for AWS users.
	UserInlinePolicies []*accessgraphv1alpha.AWSUserInlinePolicyV1
	// UserAttachedPolicies is the list of attached policies configured for AWS users.
	// This is a User ARN -> Policy ARN mapping and the policy document is included
	// in Policies.
	UserAttachedPolicies []*accessgraphv1alpha.AWSUserAttachedPolicies
	// UserGroups is the list of groups that AWS users are members of.
	UserGroups []*accessgraphv1alpha.AWSUserGroupsV1
	// Groups is the list of AWS groups.
	Groups []*accessgraphv1alpha.AWSGroupV1
	// GroupInlinePolicies is the list of inline policies configured for AWS groups.
	GroupInlinePolicies []*accessgraphv1alpha.AWSGroupInlinePolicyV1
	// GroupAttachedPolicies is the list of attached policies configured for AWS groups.
	// This is a Group ARN -> Policy ARN mapping and the policy document is included
	GroupAttachedPolicies []*accessgraphv1alpha.AWSGroupAttachedPolicies
	// Instances is the list of AWS EC2 instances.
	Instances []*accessgraphv1alpha.AWSInstanceV1
	// Policies is the list of AWS IAM policies and their policy documents.
	Policies []*accessgraphv1alpha.AWSPolicyV1
	// S3Buckets is the list of AWS S3 buckets.
	S3Buckets []*accessgraphv1alpha.AWSS3BucketV1
	// Roles is the list of AWS IAM roles.
	Roles []*accessgraphv1alpha.AWSRoleV1
	// RoleInlinePolicies is the list of inline policies configured for AWS roles.
	RoleInlinePolicies []*accessgraphv1alpha.AWSRoleInlinePolicyV1
	// RoleAttachedPolicies is the list of attached policies configured for AWS roles.
	// This is a Role ARN -> Policy ARN mapping and the policy document is included
	RoleAttachedPolicies []*accessgraphv1alpha.AWSRoleAttachedPolicies
	// InstanceProfiles is the list of AWS IAM instance profiles.
	InstanceProfiles []*accessgraphv1alpha.AWSInstanceProfileV1
	// EKSClusters is the list of EKS clusters
	EKSClusters []*accessgraphv1alpha.AWSEKSClusterV1
	// AssociatedAccessPolicies is the list of Associated Access policies
	AssociatedAccessPolicies []*accessgraphv1alpha.AWSEKSAssociatedAccessPolicyV1
	// AccessEntries is the list of Access Entries.
	AccessEntries []*accessgraphv1alpha.AWSEKSClusterAccessEntryV1
	// RDSDatabases is a list of RDS instances and clusters.
	RDSDatabases []*accessgraphv1alpha.AWSRDSDatabaseV1
	// SAMLProviders is a list of SAML providers.
	SAMLProviders []*accessgraphv1alpha.AWSSAMLProviderV1
	// OIDCProviders is a list of OIDC providers.
	OIDCProviders []*accessgraphv1alpha.AWSOIDCProviderV1
}

func (r *Resources) count() int {
	if r == nil {
		return 0
	}

	elem := reflect.ValueOf(r).Elem()
	sum := 0
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		if field.IsValid() {
			switch field.Kind() {
			case reflect.Slice:
				sum += field.Len()
			}
		}
	}
	return sum
}

// UsageReport returns a usage report based on the resources.
func (r *Resources) UsageReport(numberAccounts int) *usageeventsv1.AccessGraphAWSScanEvent {
	if r == nil {
		return &usageeventsv1.AccessGraphAWSScanEvent{
			TotalAccounts: uint64(numberAccounts),
		}
	}
	return &usageeventsv1.AccessGraphAWSScanEvent{
		TotalEc2Instances:  uint64(len(r.Instances)),
		TotalUsers:         uint64(len(r.Users)),
		TotalGroups:        uint64(len(r.Groups)),
		TotalRoles:         uint64(len(r.Roles)),
		TotalPolicies:      uint64(len(r.Policies)),
		TotalEksClusters:   uint64(len(r.EKSClusters)),
		TotalRdsInstances:  uint64(len(r.RDSDatabases)),
		TotalS3Buckets:     uint64(len(r.S3Buckets)),
		TotalSamlProviders: uint64(len(r.SAMLProviders)),
		TotalOidcProviders: uint64(len(r.OIDCProviders)),
		TotalAccounts:      uint64(numberAccounts),
	}
}

// NewFetcher creates a new AWS fetcher.
func NewFetcher(ctx context.Context, cfg Config) (*Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	a := &Fetcher{
		Config:     cfg,
		lastResult: &Resources{},
	}
	accountID, err := a.getAccountId(context.Background())
	if err != nil {
		return nil, trace.Wrap(err, "failed to get AWS account ID")
	}
	a.AccountID = accountID
	return a, nil
}

// Poll retrieves all AWS resources and returns the result.
// Poll is a blocking call and will return when all resources have been fetched.
// It's possible that the call returns Resources and an error at the same time
// if some resources were fetched successfully and some were not.
func (a *Fetcher) Poll(ctx context.Context, features Features) (*Resources, error) {
	result, err := a.poll(ctx, features)
	deduplicateResources(result)
	a.storeReport(result, err)
	return result, trace.Wrap(err)
}

func (a *Fetcher) storeReport(rec *Resources, err error) {
	a.lastError = err
	if rec == nil {
		return
	}
	a.lastResult = rec
	a.lastDiscoveredResources = uint64(rec.count())
}

func (a *Fetcher) GetAccountID() string {
	return a.AccountID
}

func (a *Fetcher) poll(ctx context.Context, features Features) (*Resources, error) {
	eGroup, ctx := errgroup.WithContext(ctx)
	// Set the limit for the number of concurrent pollers running in parallel.
	// This is to prevent the number of concurrent pollers from growing too large
	// and causing the AWS API to throttle requests.
	eGroup.SetLimit(5)
	var (
		errs   []error
		errMu  sync.Mutex
		result = &Resources{}
	)
	// collectErr collects an error and adds it to the list of errors.
	// errors are collected in parallel and are not returned until all
	// resources have been fetched.
	collectErr := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		errs = append(errs, err)
	}

	// fetch AWS users and their associated resources.
	// - inline policies
	// - attached policies
	// - user groups they are members of
	if features.Users {
		eGroup.Go(a.pollAWSUsers(ctx, result, a.lastResult, collectErr))
	}

	// fetch AWS groups and their associated resources.
	// - inline policies
	// - attached policies
	if features.Roles {
		eGroup.Go(a.pollAWSRoles(ctx, result, collectErr))
	}

	// fetch AWS groups and their associated resources.
	// - inline policies
	// - attached policies
	if features.Groups {
		eGroup.Go(a.pollAWSGroups(ctx, result, collectErr))
	}

	// fetch AWS EC2 instances and their associated resources.
	// - instance profiles
	if features.EC2 {
		eGroup.Go(a.pollAWSEC2Instances(ctx, result, collectErr))
	}

	// fetch AWS IAM policies and their policy documents.
	if features.Users || features.Roles {
		eGroup.Go(a.pollAWSPolicies(ctx, result, collectErr))
	}

	// fetch AWS S3 buckets.
	if features.S3 {
		eGroup.Go(a.pollAWSS3Buckets(ctx, result, collectErr))
	}

	// fetch AWS EKS clusters
	if features.EKS {
		eGroup.Go(a.pollAWSEKSClusters(ctx, result, collectErr))
	}

	// fetch AWS RDS instances and clusters
	if features.RDS {
		eGroup.Go(a.pollAWSRDSDatabases(ctx, result, collectErr))
	}

	if features.IDP {
		eGroup.Go(a.pollAWSSAMLProviders(ctx, result, collectErr))
		eGroup.Go(a.pollAWSOIDCProviders(ctx, result, collectErr))
	}

	if err := eGroup.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	return result, trace.NewAggregate(errs...)
}

// getAWSOptions returns a list of options to be used when creating AWS clients
// with the v2 sdk.
func (a *Fetcher) getAWSOptions() []awsconfig.OptionsFn {
	opts := []awsconfig.OptionsFn{
		awsconfig.WithCredentialsMaybeIntegration(a.Config.Integration),
	}

	if a.Config.AssumeRole != nil {
		opts = append(opts, awsconfig.WithAssumeRole(a.Config.AssumeRole.RoleARN, a.Config.AssumeRole.ExternalID))
	}
	const maxRetries = 10
	opts = append(opts, awsconfig.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(so *retry.StandardOptions) {
			so.MaxAttempts = maxRetries
			so.Backoff = retry.NewExponentialJitterBackoff(300 * time.Second)
		})
	}))

	return opts
}

func (a *Fetcher) getAccountId(ctx context.Context) (string, error) {
	awsCfg, err := a.AWSConfigProvider.GetConfig(
		ctx,
		"", /* region is empty because groups are global */
		a.getAWSOptions()...,
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	stsClient := a.awsClients.getSTSClient(awsCfg)

	input := &sts.GetCallerIdentityInput{}
	req, err := stsClient.GetCallerIdentity(ctx, input)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return aws.ToString(req.Account), nil
}

func (a *Fetcher) DiscoveryConfigName() string {
	return a.Config.DiscoveryConfigName
}

func (a *Fetcher) IsFromDiscoveryConfig() bool {
	return a.Config.DiscoveryConfigName != ""
}

func (a *Fetcher) Status() (uint64, error) {
	return a.lastDiscoveredResources, a.lastError
}
