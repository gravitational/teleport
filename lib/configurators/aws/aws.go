// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	apiutils "github.com/gravitational/teleport/api/utils"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/configurators"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/secrets"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// DefaultPolicyName default policy name.
	DefaultPolicyName = "DatabaseAccess"
	// databasePolicyDescription description used on the policy created.
	databasePolicyDescription = "Used by Teleport database agents for discovering AWS-hosted databases."
	// discoveryServicePolicyDescription description used on the policy created.
	discoveryServicePolicyDescription = "Used by Teleport the discovery service to discover AWS-hosted services."
	// boundarySuffix boundary policies will have this suffix.
	boundarySuffix = "Boundary"
	// policyTeleportTagKey key of the tag added to the policies created.
	policyTeleportTagKey = "teleport"
	// policyTeleportTagValue value of the tag added to the policies created.
	policyTeleportTagValue = ""
	// defaultAttachUser default user that the policy will be attached to.
	defaultAttachUser = "username"
)

var (
	// defaultPolicyTags default list of tags present at the managed policies.
	defaultPolicyTags = map[string]string{policyTeleportTagKey: policyTeleportTagValue}
	// userBaseActions list of actions used when target is an user.
	userBaseActions = []string{"iam:GetUserPolicy", "iam:PutUserPolicy", "iam:DeleteUserPolicy"}
	// roleBaseActions list of actions used when target is a role.
	roleBaseActions = []string{"iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy"}
	// rdsInstancesActions list of actions used when giving RDS instances permissions.
	rdsInstancesActions = []string{"rds:DescribeDBInstances", "rds:ModifyDBInstance"}
	// auroraActions list of actions used when giving RDS Aurora permissions.
	auroraActions = []string{"rds:DescribeDBClusters", "rds:ModifyDBCluster"}
	// rdsProxyActions list of actions used when giving RDS Proxy permissions.
	rdsProxyActions = []string{"rds:DescribeDBProxies", "rds:DescribeDBProxyEndpoints", "rds:DescribeDBProxyTargets", "rds:ListTagsForResource"}
	// redshiftActions list of actions used when giving Redshift auto-discovery
	// permissions.
	redshiftActions = []string{"redshift:DescribeClusters"}
	// elastiCacheActions is a list of actions used for ElastiCache
	// auto-discovery and metadata update.
	elastiCacheActions = []string{
		"elasticache:ListTagsForResource",
		"elasticache:DescribeReplicationGroups",
		"elasticache:DescribeCacheClusters",
		"elasticache:DescribeCacheSubnetGroups",
		"elasticache:DescribeUsers",
		"elasticache:ModifyUser",
	}
	// memoryDBActions is a list of actions used for MemoryDB auto-discovery
	// and metadata update.
	memoryDBActions = []string{
		"memorydb:ListTags",
		"memorydb:DescribeClusters",
		"memorydb:DescribeSubnetGroups",
		"memorydb:DescribeUsers",
		"memorydb:UpdateUser",
	}
	// secretsManagerActions is a list of actions used for SecretsManager.
	secretsManagerActions = []string{
		"secretsmanager:DescribeSecret",
		"secretsmanager:CreateSecret",
		"secretsmanager:UpdateSecret",
		"secretsmanager:DeleteSecret",
		"secretsmanager:GetSecretValue",
		"secretsmanager:PutSecretValue",
		"secretsmanager:TagResource",
	}
	// kmsActions is a list of actions used for custom KMS keys.
	kmsActions = []string{
		"kms:GenerateDataKey",
		"kms:Decrypt",
	}
	// ec2Actions is a list of actions used for EC2 auto-discovery
	ec2Actions = []string{
		"ec2:DescribeInstances",
		"ssm:GetCommandInvocation",
		"ssm:SendCommand",
	}
	// boundaryRDSConnectActions additional actions added to the policy boundary
	// when policy has RDS auto-discovery.
	boundaryRDSConnectActions = []string{"rds-db:connect"}
	// boundaryRedshiftActions additional actions added to the policy boundary
	// when policy has Redshift auto-discovery.
	boundaryRedshiftActions = []string{"redshift:GetClusterCredentials"}
)

// awsConfigurator defines the AWS database configurator.
type awsConfigurator struct {
	// config AWS configurator list of configs.
	config ConfiguratorConfig
	// actions list of the configurator actions, those are populated on the
	// `build` function.
	actions []configurators.ConfiguratorAction
}

type ConfiguratorConfig struct {
	// Flags user-provided flags to configure/execute the configurator.
	Flags configurators.BootstrapFlags
	// FileConfig Teleport database agent config.
	FileConfig *config.FileConfig
	// AWSSession current AWS session.
	AWSSession *awssession.Session
	// AWSSTSClient AWS STS client.
	AWSSTSClient stsiface.STSAPI
	// AWSIAMClient AWS IAM client.
	AWSIAMClient iamiface.IAMAPI
	// AWSSSMClient AWS SSM Client
	AWSSSMClient ssmiface.SSMAPI
	// Policies instance of the `Policies` that the actions use.
	Policies awslib.Policies
	// Identity is the current AWS credentials chain identity.
	Identity awslib.Identity
}

// CheckAndSetDefaults checks and set configuration default values.
func (c *ConfiguratorConfig) CheckAndSetDefaults() error {
	if c.FileConfig == nil {
		return trace.BadParameter("config file is required")
	}

	// When running the command in manual mode, we want to have zero dependency
	// with AWS configurations (like awscli or environment variables), so that
	// the user can run this command and generate the instructions without any
	// pre-requisite.
	if !c.Flags.Manual {
		var err error

		if c.AWSSession == nil {
			c.AWSSession, err = awssession.NewSessionWithOptions(awssession.Options{
				SharedConfigState: awssession.SharedConfigEnable,
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}

		if c.AWSSTSClient == nil {
			c.AWSSTSClient = sts.New(c.AWSSession)
		}
		if c.AWSIAMClient == nil {
			c.AWSIAMClient = iam.New(c.AWSSession)
		}
		if c.Identity == nil {
			c.Identity, err = awslib.GetIdentityWithClient(context.Background(), c.AWSSTSClient)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		if c.AWSSSMClient == nil {
			c.AWSSSMClient = ssm.New(c.AWSSession)
		}

		if c.Policies == nil {
			c.Policies = awslib.NewPolicies(c.Identity.GetPartition(), c.Identity.GetAccountID(), iam.New(c.AWSSession))
		}
	}

	return nil
}

// NewAWSConfigurator creates an instance of awsConfigurator and builds its
// actions.
func NewAWSConfigurator(config ConfiguratorConfig) (configurators.Configurator, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions, err := buildActions(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &awsConfigurator{config, actions}, nil
}

// IsEmpty checks if the configurator has no actions.
func (a *awsConfigurator) IsEmpty() bool {
	return len(a.actions) == 0
}

// Name returns human-readable configurator name.
func (a *awsConfigurator) Name() string {
	return "AWS"
}

// Actions list of configurator actions.
func (a *awsConfigurator) Actions() []configurators.ConfiguratorAction {
	return a.actions
}

// awsPolicyCreator creates a `PolicyDocument` on AWS, it also stores the policy
// ARN in the context.
type awsPolicyCreator struct {
	// policies `Policies` used to upsert the policy document.
	policies awslib.Policies
	// isBoundary indicates if the policy created is a boundary or not.
	isBoundary bool
	// policy document that will be created on AWS.
	policy *awslib.Policy
	// formattedPolicy human-readable representation of the policy document.
	formattedPolicy string
}

// Description returns what the action will perform.
func (a *awsPolicyCreator) Description() string {
	return fmt.Sprintf("Create IAM Policy %q", a.policy.Name)
}

// Details returns the policy document that will be created.
func (a *awsPolicyCreator) Details() string {
	return a.formattedPolicy
}

// Execute upserts the policy and store its ARN in the action context.
func (a *awsPolicyCreator) Execute(ctx context.Context, actionCtx *configurators.ConfiguratorActionContext) error {
	if a.policies == nil {
		return trace.BadParameter("policy helper not initialized")
	}
	arn, err := a.policies.Upsert(ctx, a.policy)
	if err != nil {
		return trace.Wrap(err)
	}

	if a.isBoundary {
		actionCtx.AWSPolicyBoundaryArn = arn
	} else {
		actionCtx.AWSPolicyArn = arn
	}

	return nil
}

// awsPoliciesAttacher attach policy and policy boundary to a target. Both
// policies ARN are retrieved from the `Execute` `context.Context`.
type awsPoliciesAttacher struct {
	// policies `Policies` used to attach policy and policy boundary.
	policies awslib.Policies
	// target identity where the policy will be attached to.
	target awslib.Identity
}

// Description returns what the action will perform.
func (a *awsPoliciesAttacher) Description() string {
	return fmt.Sprintf("Attach IAM policies to %q", a.target.GetName())
}

// Details attacher doesn't have any extra detail, this function returns an
// empty string.
func (a *awsPoliciesAttacher) Details() string {
	return ""
}

// Execute retrieves policy and policy boundary ARNs from
// `ConfiguratorActionContext` and attach them to the `target`.
func (a *awsPoliciesAttacher) Execute(ctx context.Context, actionCtx *configurators.ConfiguratorActionContext) error {
	if a.policies == nil {
		return trace.BadParameter("policy helper not initialized")
	}

	if actionCtx.AWSPolicyArn == "" {
		return trace.BadParameter("policy ARN not present")
	}

	if actionCtx.AWSPolicyBoundaryArn == "" {
		return trace.BadParameter("policy boundary ARN not present")
	}

	err := a.policies.Attach(ctx, actionCtx.AWSPolicyArn, a.target)
	if err != nil {
		return trace.Wrap(err)
	}

	err = a.policies.AttachBoundary(ctx, actionCtx.AWSPolicyBoundaryArn, a.target)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func buildDiscoveryActions(config ConfiguratorConfig, target awslib.Identity) ([]configurators.ConfiguratorAction, error) {
	actions, err := buildCommonActions(config, target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ssmDocumentCreators, err := buildSSMDocuments(config.AWSSSMClient, config.Flags, config.FileConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	actions = append(actions, ssmDocumentCreators...)
	return actions, nil
}

func buildCommonActions(config ConfiguratorConfig, target awslib.Identity) ([]configurators.ConfiguratorAction, error) {
	// Generate policies.
	policy, err := buildPolicyDocument(config.Flags, config.FileConfig, target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policyBoundary, err := buildPolicyBoundaryDocument(config.Flags, config.FileConfig, target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the policy has no statements means that the agent doesn't require
	// any IAM permission. In this case, return without errors and with empty
	// actions.
	if len(policy.Document.Statements) == 0 {
		return []configurators.ConfiguratorAction{}, nil
	}

	formattedPolicy, err := policy.Document.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var actions []configurators.ConfiguratorAction

	// Create IAM Policy.
	actions = append(actions, &awsPolicyCreator{
		policies:        config.Policies,
		policy:          policy,
		formattedPolicy: formattedPolicy,
	})

	formattedPolicyBoundary, err := policyBoundary.Document.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Create IAM Policy boundary.
	actions = append(actions, &awsPolicyCreator{
		policies:        config.Policies,
		policy:          policyBoundary,
		formattedPolicy: formattedPolicyBoundary,
		isBoundary:      true,
	})

	// Attach both policies to the target.
	actions = append(actions, &awsPoliciesAttacher{policies: config.Policies, target: target})
	return actions, nil
}

// buildActions generates the policy documents and configurator actions.
func buildActions(config ConfiguratorConfig) ([]configurators.ConfiguratorAction, error) {
	// Identity is going to be empty (`nil`) when running the command on
	// `Manual` mode, place a wildcard to keep the generated policies valid.
	accountID := "*"
	partitionID := "*"
	if config.Identity != nil {
		accountID = config.Identity.GetAccountID()
		partitionID = config.Identity.GetPartition()
	}

	// Define the target and target type.
	target, err := policiesTarget(config.Flags, accountID, partitionID, config.Identity, config.AWSIAMClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if config.Flags.DiscoveryService {
		return buildDiscoveryActions(config, target)
	}
	return buildCommonActions(config, target)
}

// policiesTarget defines which target and its type the policies will be
// attached to.
func policiesTarget(flags configurators.BootstrapFlags, accountID string, partitionID string, identity awslib.Identity, iamClient iamiface.IAMAPI) (awslib.Identity, error) {
	if flags.AttachToUser != "" {
		userArn := flags.AttachToUser
		if !arn.IsARN(flags.AttachToUser) {
			userArn = buildIAMARN(partitionID, accountID, "user", flags.AttachToUser)
		}

		return awslib.IdentityFromArn(userArn)
	}

	if flags.AttachToRole != "" {
		roleArn := flags.AttachToRole
		if !arn.IsARN(flags.AttachToRole) {
			roleArn = buildIAMARN(partitionID, accountID, "role", flags.AttachToRole)
		}

		return awslib.IdentityFromArn(roleArn)
	}

	if identity == nil {
		return awslib.IdentityFromArn(buildIAMARN(partitionID, accountID, "user", defaultAttachUser))
	}

	if identity.GetType() == awslib.ResourceTypeAssumedRole {
		roleIdentity, err := getRoleARNForAssumedRole(iamClient, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return roleIdentity, nil
	}

	return identity, nil
}

// buildIAMARN constructs an AWS IAM ARN string from the given partition,
// account, resource type, and resource.
// If the resource starts with the "/" prefix, this function takes care not to
// add an additional "/" prefix to the constructed ARN.
// This handles resource names that include a path correctly. Example:
// resource input: "/some/path/to/rolename"
// arn output: "arn:aws:iam::123456789012:role/some/path/to/rolename"
func buildIAMARN(partitionID, accountID, resourceType, resource string) string {
	if strings.HasPrefix(resource, "/") {
		return fmt.Sprintf("arn:%s:iam::%s:%s%s", partitionID, accountID, resourceType, resource)
	} else {
		return fmt.Sprintf("arn:%s:iam::%s:%s/%s", partitionID, accountID, resourceType, resource)
	}
}

// failedToResolveAssumeRoleARN is an error message returned when an
// assumed-role identity cannot be resolved to the role ARN that it assumes,
// which is necessary to attach policies to the identity.
// Rather than returning errors about why it failed, this message suggests a
// simple fix for the user to specify a role or user to attach policies to.
const failedToResolveAssumeRoleARN = "Running with assumed-role credentials. Policies cannot be attached to an assumed-role. Provide the name or ARN of the IAM user or role to attach policies to."

// getRoleARNForAssumedRole attempts to resolve assumed-role credentials to
// the underlying role ARN using IAM API.
// This is necessary since the assumed-role ARN does not include the role path,
// so we cannot reliably reconstruct the role ARN from the assumed-role ARN.
func getRoleARNForAssumedRole(iamClient iamiface.IAMAPI, identity awslib.Identity) (awslib.Identity, error) {
	roleOutput, err := iamClient.GetRole(&iam.GetRoleInput{RoleName: aws.String(identity.GetName())})
	if err != nil || roleOutput == nil || roleOutput.Role == nil || roleOutput.Role.Arn == nil {
		return nil, trace.BadParameter(failedToResolveAssumeRoleARN)
	}
	roleIdentity, err := awslib.IdentityFromArn(*roleOutput.Role.Arn)
	if err != nil {
		return nil, trace.BadParameter(failedToResolveAssumeRoleARN)
	}
	return roleIdentity, nil
}

// buildPolicyDocument builds the policy document.
func buildPolicyDocument(flags configurators.BootstrapFlags, fileConfig *config.FileConfig, target awslib.Identity) (*awslib.Policy, error) {
	var statements []*awslib.Statement
	if flags.DiscoveryService {
		if isEC2AutoDiscoveryEnabled(flags, fileConfig) {
			statements = append(statements, buildEC2AutoDiscoveryStatements()...)
		}
		document := awslib.NewPolicyDocument()
		document.Statements = statements
		return awslib.NewPolicy(
			flags.PolicyName,
			discoveryServicePolicyDescription,
			defaultPolicyTags,
			document,
		), nil
	}

	rdsDatabases := hasRDSDatabases(flags, fileConfig)
	rdsProxyDatabases := hasRDSProxyDatabases(flags, fileConfig)
	redshiftDatabases := hasRedshiftDatabases(flags, fileConfig)
	elastiCacheDatabases := hasElastiCacheDatabases(flags, fileConfig)
	memoryDBDatabases := hasMemoryDBDatabases(flags, fileConfig)
	requireSecretsManager := elastiCacheDatabases || memoryDBDatabases

	if rdsDatabases {
		statements = append(statements, buildRDSStatements()...)
	}

	if rdsProxyDatabases {
		statements = append(statements, buildRDSProxyStatements()...)
	}

	if redshiftDatabases {
		statements = append(statements, buildRedshiftStatements()...)
	}

	// ElastiCache does not require permissions to edit user/role IAM policy.
	if elastiCacheDatabases {
		statements = append(statements, buildElastiCacheStatements()...)
	}
	if memoryDBDatabases {
		statements = append(statements, buildMemoryDBStatements()...)
	}

	if requireSecretsManager {
		statements = append(statements, buildSecretsManagerStatements(fileConfig, target)...)
	}

	// If there are RDS, RDS Proxy, Redshift databases, we need permission to
	// edit the target user/role.
	if rdsDatabases || rdsProxyDatabases || redshiftDatabases {
		targetStatements, err := buildIAMEditStatements(target)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		statements = append(statements, targetStatements...)
	}

	document := awslib.NewPolicyDocument()
	document.Statements = statements
	return awslib.NewPolicy(
		flags.PolicyName,
		databasePolicyDescription,
		defaultPolicyTags,
		document,
	), nil
}

func getProxyAddrFromFileConfig(fc *config.FileConfig) (string, error) {
	addrs, err := utils.AddrsFromStrings(fc.Proxy.PublicAddr, defaults.HTTPListenPort)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return fmt.Sprintf("https://<proxy address>:%d", defaults.HTTPListenPort), nil
	}
	addr := addrs[0]

	return fmt.Sprintf("https://%s", addr.String()), nil
}

func buildSSMDocuments(ssm ssmiface.SSMAPI, flags configurators.BootstrapFlags, fileConfig *config.FileConfig) ([]configurators.ConfiguratorAction, error) {
	var creators []configurators.ConfiguratorAction
	proxyAddr, err := getProxyAddrFromFileConfig(fileConfig)
	if err != nil {
		return nil, err
	}
	for _, matcher := range fileConfig.Discovery.AWSMatchers {
		if !apiutils.SliceContainsStr(matcher.Types, constants.AWSServiceTypeEC2) {
			continue
		}
		ssmCreator := awsSSMDocumentCreator{
			ssm:      ssm,
			Name:     matcher.SSM.DocumentName,
			Contents: EC2DiscoverySSMDocument(proxyAddr),
		}
		creators = append(creators, &ssmCreator)
	}
	return creators, nil
}

// buildPolicyBoundaryDocument builds the policy boundary document.
func buildPolicyBoundaryDocument(flags configurators.BootstrapFlags, fileConfig *config.FileConfig, target awslib.Identity) (*awslib.Policy, error) {
	var statements []*awslib.Statement

	if isEC2AutoDiscoveryEnabled(flags, fileConfig) {

		statements = append(statements, buildEC2AutoDiscoveryBoundaryStatements()...)

		document := awslib.NewPolicyDocument()
		document.Statements = statements
		return awslib.NewPolicy(
			fmt.Sprintf("%s%s", flags.PolicyName, boundarySuffix),
			databasePolicyDescription,
			defaultPolicyTags,
			document,
		), nil
	}
	rdsDatabases := hasRDSDatabases(flags, fileConfig)
	rdsProxyDatabases := hasRDSProxyDatabases(flags, fileConfig)
	redshiftDatabases := hasRedshiftDatabases(flags, fileConfig)
	elastiCacheDatabases := hasElastiCacheDatabases(flags, fileConfig)
	memoryDBDatabases := hasMemoryDBDatabases(flags, fileConfig)
	requireSecretsManager := elastiCacheDatabases || memoryDBDatabases

	if rdsDatabases {
		statements = append(statements, buildRDSBoundaryStatements()...)
	}

	if rdsProxyDatabases {
		statements = append(statements, buildRDSProxyBoundaryStatements()...)
	}

	if redshiftDatabases {
		statements = append(statements, buildRedshiftBoundaryStatements()...)
	}
	if memoryDBDatabases {
		statements = append(statements, buildMemoryDBBoundaryStatements()...)
	}

	// ElastiCache does not require permissions to edit user/role IAM policy.
	if elastiCacheDatabases {
		statements = append(statements, buildElastiCacheBoundaryStatements()...)
	}

	if requireSecretsManager {
		statements = append(statements, buildSecretsManagerStatements(fileConfig, target)...)
	}

	// If there are RDS, RDS Proxy, Redshift databases, we need permission to
	// edit the target user/role.
	if rdsDatabases || rdsProxyDatabases || redshiftDatabases {
		targetStatements, err := buildIAMEditStatements(target)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		statements = append(statements, targetStatements...)
	}

	document := awslib.NewPolicyDocument()
	document.Statements = statements
	return awslib.NewPolicy(
		fmt.Sprintf("%s%s", flags.PolicyName, boundarySuffix),
		databasePolicyDescription,
		defaultPolicyTags,
		document,
	), nil
}

func isEC2AutoDiscoveryEnabled(flags configurators.BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceEC2Permissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(services.AWSMatcherEC2, fileConfig.Discovery.AWSMatchers)
}

// hasRDSDatabases checks if the agent needs permission for
// RDS/Aurora databases.
func hasRDSDatabases(flags configurators.BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceRDSPermissions {
		return true
	}
	// isRDSAutoDiscoveryEnabled checks if the agent needs permission for
	// RDS/Aurora auto-discovery.
	return isAutoDiscoveryEnabledForMatcher(services.AWSMatcherRDS, fileConfig.Databases.AWSMatchers) ||
		findEndpointIs(fileConfig, isRDSEndpoint)
}

// hasRDSProxyDatabases checks if the agent needs permission for
// RDS Proxy databases.
func hasRDSProxyDatabases(flags configurators.BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceRDSProxyPermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(services.AWSMatcherRDSProxy, fileConfig.Databases.AWSMatchers) ||
		findEndpointIs(fileConfig, isRDSProxyEndpoint)
}

// hasRedshiftDatabases checks if the agent needs permission for
// Redshift databases.
func hasRedshiftDatabases(flags configurators.BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceRedshiftPermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(services.AWSMatcherRedshift, fileConfig.Databases.AWSMatchers) ||
		findEndpointIs(fileConfig, awsutils.IsRedshiftEndpoint)
}

// hasElastiCacheDatabases checks if the agent needs permission for
// ElastiCache databases.
func hasElastiCacheDatabases(flags configurators.BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceElastiCachePermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(services.AWSMatcherElastiCache, fileConfig.Databases.AWSMatchers) ||
		findEndpointIs(fileConfig, awsutils.IsElastiCacheEndpoint)
}

// hasMemoryDBDatabases checks if the agent needs permission for
// ElastiCache databases.
func hasMemoryDBDatabases(flags configurators.BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceMemoryDBPermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(services.AWSMatcherMemoryDB, fileConfig.Databases.AWSMatchers) ||
		findEndpointIs(fileConfig, awsutils.IsMemoryDBEndpoint)
}

// isAutoDiscoveryEnabledForMatcher returns true if provided AWS matcher type
// is found.
func isAutoDiscoveryEnabledForMatcher(matcherType string, matchers []config.AWSMatcher) bool {
	for _, matcher := range matchers {
		for _, databaseType := range matcher.Types {
			if databaseType == matcherType {
				return true
			}
		}
	}
	return false
}

// findEndpointIs returns true if provided check returns true for any static
// endpoint.
func findEndpointIs(fileConfig *config.FileConfig, endpointIs func(string) bool) bool {
	for _, database := range fileConfig.Databases.Databases {
		if endpointIs(database.URI) {
			return true
		}
	}
	return false
}

// isRDSEndpoint returns true if the endpoint is an endpoint for RDS instance or Aurora cluster.
func isRDSEndpoint(uri string) bool {
	details, err := awsutils.ParseRDSEndpoint(uri)
	if err != nil {
		return false
	}
	return !details.IsProxy()
}

// isRDSProxyEndpoint returns true if the endpoint is an endpoint for RDS Proxy.
func isRDSProxyEndpoint(uri string) bool {
	details, err := awsutils.ParseRDSEndpoint(uri)
	if err != nil {
		return false
	}
	return details.IsProxy()
}

// buildIAMEditStatements returns IAM statements necessary for the Teleport
// agent to edit user/role permissions.
func buildIAMEditStatements(target awslib.Identity) ([]*awslib.Statement, error) {
	statement := &awslib.Statement{
		Effect:    awslib.EffectAllow,
		Resources: []string{target.String()},
	}

	switch target.(type) {
	case awslib.User, *awslib.User:
		statement.Actions = userBaseActions
	case awslib.Role, *awslib.Role:
		statement.Actions = roleBaseActions
	default:
		return nil, trace.BadParameter("policies target must be an user or role, received %T", target)
	}

	return []*awslib.Statement{statement}, nil
}

// buildEC2AutoDiscoveryStatements returns IAM statements necessary for
// EC2 instance auto-discovery.
func buildEC2AutoDiscoveryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   ec2Actions,
			Resources: []string{"*"},
		},
	}
}

func buildEC2AutoDiscoveryBoundaryStatements() []*awslib.Statement {
	return buildEC2AutoDiscoveryStatements()
}

// buildRDSAutoDiscoveryStatements returns IAM statements necessary for
// RDS/Aurora databases auto-discovery.
func buildRDSStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   append(rdsInstancesActions, auroraActions...),
			Resources: []string{"*"},
		},
	}
}

// buildRDSBoundaryStatements returns IAM boundary statements
// necessary for RDS/Aurora databases auto-discovery.
func buildRDSBoundaryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   append(rdsInstancesActions, append(auroraActions, boundaryRDSConnectActions...)...),
			Resources: []string{"*"},
		},
	}
}

// buildRDSProxyStatements returns IAM statements necessary for
// RDS Proxy databases auto-discovery.
func buildRDSProxyStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   rdsProxyActions,
			Resources: []string{"*"},
		},
	}
}

// buildRDSProxyBoundaryStatements returns IAM boundary statements
// necessary for RDS Proxy databases auto-discovery.
func buildRDSProxyBoundaryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   append(rdsProxyActions, boundaryRDSConnectActions...),
			Resources: []string{"*"},
		},
	}
}

// buildRedshiftStatements returns IAM statements necessary for Redshift
// databases.
func buildRedshiftStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   redshiftActions,
			Resources: []string{"*"},
		},
	}
}

// buildRedshiftBoundaryStatements returns IAM boundary statements necessary for
// Redshift databases.
func buildRedshiftBoundaryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   append(redshiftActions, boundaryRedshiftActions...),
			Resources: []string{"*"},
		},
	}
}

// buildElastiCacheStatements returns IAM statements necessary for ElastiCache
// databases.
func buildElastiCacheStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   elastiCacheActions,
			Resources: []string{"*"},
		},
	}
}

// buildElastiCacheBoundaryStatements returns IAM boundary statements necessary
// for ElastiCache databases.
func buildElastiCacheBoundaryStatements() []*awslib.Statement {
	return buildElastiCacheStatements()
}

func buildMemoryDBStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   memoryDBActions,
			Resources: []string{"*"},
		},
	}
}
func buildMemoryDBBoundaryStatements() []*awslib.Statement {
	return buildMemoryDBStatements()
}

// buildSecretsManagerStatements returns IAM statements necessary for using AWS
// Secrets Manager.
func buildSecretsManagerStatements(fileConfig *config.FileConfig, target awslib.Identity) []*awslib.Statement {
	// Populate resources with the default secrets prefix.
	secretsManagerStatement := &awslib.Statement{
		Effect:    awslib.EffectAllow,
		Actions:   secretsManagerActions,
		Resources: []string{buildSecretsManagerARN(target, secrets.DefaultKeyPrefix)},
	}
	// KMS statement is only required when using custom KMS keys.
	kmsStatement := &awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: kmsActions,
	}

	addedSecretPrefixes := map[string]bool{}
	addedKMSKeyIDs := map[string]bool{}
	for _, database := range fileConfig.Databases.Databases {
		if !awsutils.IsElastiCacheEndpoint(database.URI) &&
			!awsutils.IsMemoryDBEndpoint(database.URI) {
			continue
		}

		// Build Secrets Manager resources.
		prefix := database.AWS.SecretStore.KeyPrefix
		if prefix != "" && !addedSecretPrefixes[prefix] {
			addedSecretPrefixes[prefix] = true
			secretsManagerStatement.Resources = append(
				secretsManagerStatement.Resources,
				buildSecretsManagerARN(target, prefix),
			)
		}

		// Build KMS resources.
		kmsKeyID := database.AWS.SecretStore.KMSKeyID
		if kmsKeyID != "" && !addedKMSKeyIDs[kmsKeyID] {
			addedKMSKeyIDs[kmsKeyID] = true
			kmsStatement.Resources = append(
				kmsStatement.Resources,
				buildARN(target, kms.ServiceName, "key/"+kmsKeyID),
			)
		}
	}

	statements := []*awslib.Statement{
		secretsManagerStatement,
	}
	if len(kmsStatement.Resources) > 0 {
		statements = append(statements, kmsStatement)
	}
	return statements
}

// buildSecretsManagerARN builds an ARN of a secret used for Secrets Manager
// IAM policies.
func buildSecretsManagerARN(target awslib.Identity, keyPrefix string) string {
	return buildARN(
		target,
		secretsmanager.ServiceName,
		fmt.Sprintf("secret:%s/*", strings.TrimSuffix(keyPrefix, "/")),
	)
}

// buildARN builds an ARN string with provided identity, service and resource.
func buildARN(target awslib.Identity, service, resource string) string {
	arn := arn.ARN{
		Partition: target.GetPartition(),
		AccountID: target.GetAccountID(),
		Region:    "*",
		Service:   service,
		Resource:  resource,
	}
	return arn.String()
}

type awsSSMDocumentCreator struct {
	Contents string
	ssm      ssmiface.SSMAPI
	Name     string
}

// Description returns what the action will perform.
func (a *awsSSMDocumentCreator) Description() string {
	return fmt.Sprintf("Create SSM Document %q", a.Name)
}

// Details returns the policy document that will be created.
func (a *awsSSMDocumentCreator) Details() string {
	return a.Contents
}

// Execute upserts the policy and store its ARN in the action context.
func (a *awsSSMDocumentCreator) Execute(ctx context.Context, actionCtx *configurators.ConfiguratorActionContext) error {
	_, err := a.ssm.CreateDocumentWithContext(ctx, &ssm.CreateDocumentInput{
		Content:        aws.String(a.Contents),
		Name:           aws.String(a.Name),
		DocumentType:   aws.String(ssm.DocumentTypeCommand),
		DocumentFormat: aws.String("YAML"),
	})

	if err != nil {
		var aErr awserr.Error
		if errors.As(err, &aErr) && aErr.Code() == ssm.ErrCodeDocumentAlreadyExists {
			fmt.Printf("⚠️ Warning: SSM document %s already exists. Not overwriting.\n", a.Name)
			return nil
		}
		return trace.Wrap(err)
	}

	return nil
}
