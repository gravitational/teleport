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

package aws

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/endpoints"
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

	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	apiawsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/configurators"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/secrets"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// DatabaseAccessPolicyName is the policy name for database access.
	DatabaseAccessPolicyName = "DatabaseAccess"
	// databasePolicyDescription description used on the policy created.
	databasePolicyDescription = "Used by Teleport database agents for accessing AWS-hosted databases."
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
	// targetIdentityARNSectionPlaceholder is the placeholder to use in a target
	// AWS IAM identity ARN when the full ARN is not given by the user and the
	// configurator is running in --manual mode.
	// e.g. arn:*:iam::*:user/username (placeholder for partition and account).
	targetIdentityARNSectionPlaceholder = "*"
)

type databaseActions struct {
	// discovery is a list of actions used for database discovery.
	discovery []string
	// iamAuth is a list of actions used for enabling IAM auth.
	iamAuth []string
	// metadata is a list of actions used for fetching database metadata
	metadata []string
	// managedUsers is a list of actions used for managing database users.
	managedUsers []string
	// authBoundary is a list of actions for authorization that need to added
	// to boundary policies.
	authBoundary []string

	requireIAMEdit        bool
	requireSecretsManager bool
}

func (a databaseActions) buildStatement(opt databaseActionsBuildOption) *awslib.Statement {
	var actions []string
	if opt.withDiscovery {
		actions = append(actions, a.discovery...)
	}
	if opt.withMetadata {
		actions = append(actions, a.metadata...)
	}
	if opt.withAuth {
		actions = append(actions, a.iamAuth...)
		actions = append(actions, a.managedUsers...)
	}
	if opt.withAuthBoundary {
		actions = append(actions, a.authBoundary...)
	}
	return &awslib.Statement{
		Effect:    awslib.EffectAllow,
		Actions:   apiutils.Deduplicate(actions),
		Resources: []string{"*"},
	}
}

type databaseActionsBuildOption struct {
	withDiscovery    bool
	withMetadata     bool
	withAuth         bool
	withAuthBoundary bool
}

func makeDatabaseActionsBuildOption(flags configurators.BootstrapFlags, targetCfg targetConfig, boundary bool) databaseActionsBuildOption {
	switch flags.Service {
	case configurators.DiscoveryService:
		return databaseActionsBuildOption{
			withDiscovery: true,
		}

	case configurators.DatabaseServiceByDiscoveryServiceConfig:
		return databaseActionsBuildOption{
			withDiscovery:    false,
			withAuth:         true,
			withAuthBoundary: boundary,
			// Discovered databases should be checked by URL validator which
			// requires same permissions as the metadata service.
			withMetadata: true,
		}

	case configurators.DatabaseService:
		return databaseActionsBuildOption{
			withDiscovery:    true,
			withMetadata:     true,
			withAuth:         true,
			withAuthBoundary: boundary,
		}

	default:
		return databaseActionsBuildOption{}
	}
}

var (
	// defaultPolicyTags default list of tags present at the managed policies.
	defaultPolicyTags = map[string]string{policyTeleportTagKey: policyTeleportTagValue}
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
	// stsActions is a list of actions used for assuming an AWS role.
	stsActions = []string{
		"sts:AssumeRole",
	}
	// rdsActions contains IAM actions for types.AWSMatcherRDS (RDS
	// instances and Aurora clusters).
	rdsActions = databaseActions{
		discovery:      []string{"rds:DescribeDBInstances", "rds:DescribeDBClusters"},
		metadata:       []string{"rds:DescribeDBInstances", "rds:DescribeDBClusters"},
		iamAuth:        []string{"rds:ModifyDBInstance", "rds:ModifyDBCluster"},
		authBoundary:   []string{"rds-db:connect"},
		requireIAMEdit: true,
	}
	// rdsProxyActions contains IAM actions for types.AWSMatcherRDSProxy.
	rdsProxyActions = databaseActions{
		discovery: []string{
			"rds:DescribeDBProxies",
			"rds:DescribeDBProxyEndpoints",
			"rds:ListTagsForResource",
		},
		metadata: []string{
			"rds:DescribeDBProxies",
			"rds:DescribeDBProxyEndpoints",
		},
		authBoundary:   []string{"rds-db:connect"},
		requireIAMEdit: true,
	}
	// redshiftActions contains IAM actions for types.AWSMatcherRedshift.
	redshiftActions = databaseActions{
		discovery: []string{"redshift:DescribeClusters"},
		metadata:  []string{"redshift:DescribeClusters"},
		authBoundary: append(
			[]string{"redshift:GetClusterCredentials"},
			stsActions..., // For IAM-auth-as-IAM-role.
		),
		requireIAMEdit: true,
	}
	// redshiftServerlessActions contains IAM actions for types.AWSMatcherRedshiftServerless.
	redshiftServerlessActions = databaseActions{
		discovery: []string{
			"redshift-serverless:ListWorkgroups",
			"redshift-serverless:ListEndpointAccess",
			"redshift-serverless:ListTagsForResource",
		},
		metadata: []string{
			"redshift-serverless:GetEndpointAccess",
			"redshift-serverless:GetWorkgroup",
		},
		authBoundary: stsActions,
	}
	// elastiCacheActions contains IAM actions for types.AWSMatcherElastiCache.
	elastiCacheActions = databaseActions{
		discovery: []string{
			"elasticache:ListTagsForResource",
			"elasticache:DescribeReplicationGroups",
			"elasticache:DescribeCacheClusters",
			"elasticache:DescribeCacheSubnetGroups",
		},
		metadata: []string{
			"elasticache:DescribeReplicationGroups",
		},
		managedUsers: []string{
			"elasticache:DescribeUsers",
			"elasticache:ModifyUser",
		},
		requireSecretsManager: true,
		authBoundary:          []string{"elasticache:Connect"},
		requireIAMEdit:        true,
	}
	// memoryDBActions contains IAM actions for types.AWSMatcherMemoryDB.
	memoryDBActions = databaseActions{
		discovery: []string{
			"memorydb:ListTags",
			"memorydb:DescribeClusters",
			"memorydb:DescribeSubnetGroups",
		},
		metadata: []string{
			"memorydb:DescribeClusters",
		},
		managedUsers: []string{
			"memorydb:DescribeUsers",
			"memorydb:UpdateUser",
		},
		requireSecretsManager: true,
		authBoundary:          []string{"memorydb:Connect"},
		requireIAMEdit:        true,
	}
	// awsKeyspacesActions contains IAM actions for static AWS Keyspaces databases.
	awsKeyspacesActions = databaseActions{
		authBoundary: stsActions,
	}
	// dynamodbActions contains IAM actions for static AWS DynamoDB databases.
	dynamodbActions = databaseActions{
		authBoundary: append(stsActions, "sts:TagSession"),
	}
	// opensearchActions contains IAM actions for types.AWSMatcherOpenSearch
	opensearchActions = databaseActions{
		discovery: []string{
			"es:ListDomainNames",
			"es:DescribeDomains",
			"es:ListTags",
		},
		metadata: []string{
			// Used for url validation.
			"es:DescribeDomains",
		},
		authBoundary: stsActions,
	}
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
	// ServiceConfig Teleport database service config.
	ServiceConfig *servicecfg.Config
	// AWSSession current AWS session.
	AWSSession *awssession.Session
	// AWSSTSClient AWS STS client.
	AWSSTSClient stsiface.STSAPI
	// AWSIAMClient AWS IAM client.
	AWSIAMClient iamiface.IAMAPI
	// AWSSSMClient is a mapping of region -> ssm client
	AWSSSMClients map[string]ssmiface.SSMAPI
	// Policies instance of the `Policies` that the actions use.
	Policies awslib.Policies
	// Identity is the current AWS credentials chain identity.
	Identity awslib.Identity
}

// CheckAndSetDefaults checks and set configuration default values.
func (c *ConfiguratorConfig) CheckAndSetDefaults() error {
	if c.ServiceConfig == nil {
		return trace.BadParameter("config file is required")
	}

	useFIPSEndpoint := endpoints.FIPSEndpointStateUnset
	if modules.GetModules().IsBoringBinary() {
		useFIPSEndpoint = endpoints.FIPSEndpointStateEnabled
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
				Config: aws.Config{
					UseFIPSEndpoint: useFIPSEndpoint,
				},
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
		if c.AWSSSMClients == nil {
			c.AWSSSMClients = make(map[string]ssmiface.SSMAPI)
			for _, matcher := range c.ServiceConfig.Discovery.AWSMatchers {
				if !slices.Contains(matcher.Types, types.AWSMatcherEC2) {
					continue
				}
				for _, region := range matcher.Regions {
					if _, ok := c.AWSSSMClients[region]; ok {
						continue
					}
					session, err := awssession.NewSessionWithOptions(awssession.Options{
						Config: aws.Config{
							Region:          &region,
							UseFIPSEndpoint: useFIPSEndpoint,
						},
						SharedConfigState: awssession.SharedConfigEnable,
					})
					if err != nil {
						return trace.Wrap(err)
					}
					c.AWSSSMClients[region] = ssm.New(session)
				}
			}

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

// Description returns a brief description of the configurator.
func (a *awsConfigurator) Description() string {
	return "Configure AWS for " + a.config.Flags.Service.Name()
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

func buildDiscoveryActions(config ConfiguratorConfig, targetCfg targetConfig) ([]configurators.ConfiguratorAction, error) {
	actions, err := buildCommonActions(config, targetCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyAddr, err := getProxyAddrFromConfig(config.ServiceConfig, config.Flags)
	if err != nil {
		return nil, err
	}

	actions = append(actions, buildSSMDocumentCreators(config.AWSSSMClients, targetCfg, proxyAddr)...)
	return actions, nil
}

func buildCommonActions(config ConfiguratorConfig, targetCfg targetConfig) ([]configurators.ConfiguratorAction, error) {
	// Generate policies.
	policy, err := buildPolicyDocument(config.Flags, targetCfg, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policyBoundary, err := buildPolicyDocument(config.Flags, targetCfg, true)
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
	actions = append(actions, &awsPoliciesAttacher{policies: config.Policies, target: targetCfg.identity})
	return actions, nil
}

// buildActions generates the policy documents and configurator actions.
func buildActions(config ConfiguratorConfig) ([]configurators.ConfiguratorAction, error) {
	// Identity is going to be empty (`nil`) when running the command on
	// `Manual` mode, place a wildcard to keep the generated policies valid.
	accountID := targetIdentityARNSectionPlaceholder
	partitionID := targetIdentityARNSectionPlaceholder
	if config.Identity != nil {
		accountID = config.Identity.GetAccountID()
		partitionID = config.Identity.GetPartition()
	}

	// Define the target and target type.
	target, err := policiesTarget(config.Flags, accountID, partitionID, config.Identity, config.AWSIAMClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetCfg, err := getTargetConfig(config.Flags, config.ServiceConfig, target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if config.Flags.Service.IsDiscovery() {
		return buildDiscoveryActions(config, targetCfg)
	}
	return buildCommonActions(config, targetCfg)
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
func buildPolicyDocument(flags configurators.BootstrapFlags, targetCfg targetConfig, boundary bool) (*awslib.Policy, error) {
	policyDoc := awslib.NewPolicyDocument()
	policyDescription := databasePolicyDescription
	policyName := flags.PolicyName

	if boundary {
		policyName += boundarySuffix
	}

	if flags.Service.IsDiscovery() {
		policyDescription = discoveryServicePolicyDescription

		if isEC2AutoDiscoveryEnabled(flags, targetCfg.awsMatchers) {
			policyDoc.EnsureStatements(buildEC2AutoDiscoveryStatements()...)
		}
	}

	// Build statements for databases.
	// TODO(greedy52) remove discovery permissions for static databases.
	var requireSecretsManager, requireIAMEdit bool
	var allActions []databaseActions
	if hasRDSDatabases(flags, targetCfg) {
		allActions = append(allActions, rdsActions)
	}
	if hasRDSProxyDatabases(flags, targetCfg) {
		allActions = append(allActions, rdsProxyActions)
	}
	if hasRedshiftDatabases(flags, targetCfg) {
		allActions = append(allActions, redshiftActions)
	}
	if hasRedshiftServerlessDatabases(flags, targetCfg) {
		allActions = append(allActions, redshiftServerlessActions)
	}
	if hasElastiCacheDatabases(flags, targetCfg) {
		allActions = append(allActions, elastiCacheActions)
	}
	if hasMemoryDBDatabases(flags, targetCfg) {
		allActions = append(allActions, memoryDBActions)
	}
	if hasAWSKeyspacesDatabases(flags, targetCfg) {
		allActions = append(allActions, awsKeyspacesActions)
	}
	if hasDynamoDBDatabases(flags, targetCfg) {
		allActions = append(allActions, dynamodbActions)
	}
	if hasOpenSearchDatabases(flags, targetCfg) {
		allActions = append(allActions, opensearchActions)
	}

	dbOption := makeDatabaseActionsBuildOption(flags, targetCfg, boundary)
	for _, dbActions := range allActions {
		policyDoc.EnsureStatements(dbActions.buildStatement(dbOption))
		if dbOption.withAuth {
			requireSecretsManager = requireSecretsManager || dbActions.requireSecretsManager
			requireIAMEdit = requireIAMEdit || dbActions.requireIAMEdit
		}
	}

	stsAssumeRoleStatements, err := buildSTSAssumeRoleStatements(flags, targetCfg, boundary)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	policyDoc.EnsureStatements(stsAssumeRoleStatements...)

	// For databases that need to access SecretsManager (and KMS).
	if requireSecretsManager {
		policyDoc.EnsureStatements(buildSecretsManagerStatements(targetCfg.databases, targetCfg.identity)...)
	}
	// For databases that need to edit IAM user/role policy.
	if requireIAMEdit {
		targetStatements, err := buildIAMEditStatements(targetCfg.identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		policyDoc.EnsureStatements(targetStatements...)
	}

	return awslib.NewPolicy(
		policyName,
		policyDescription,
		defaultPolicyTags,
		policyDoc,
	), nil
}

func getProxyAddrFromConfig(cfg *servicecfg.Config, flags configurators.BootstrapFlags) (string, error) {
	if flags.Proxy != "" {
		addr, err := utils.ParseHostPortAddr(flags.Proxy, defaults.HTTPListenPort)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return fmt.Sprintf("https://%s", addr.String()), nil
	}

	if len(cfg.Proxy.PublicAddrs) != 0 {
		return fmt.Sprintf("https://%s", cfg.Proxy.PublicAddrs[0].String()), nil
	}

	if !cfg.ProxyServer.IsEmpty() {
		return fmt.Sprintf("https://%s", cfg.ProxyServer.String()), nil
	}

	return "", trace.NotFound("proxy address not found, please provide --proxy, or set either teleport.proxy_server or proxy_service.public_addr in the teleport config")
}

func buildSSMDocumentCreators(ssm map[string]ssmiface.SSMAPI, targetCfg targetConfig, proxyAddr string) []configurators.ConfiguratorAction {
	var creators []configurators.ConfiguratorAction
	for _, matcher := range targetCfg.awsMatchers {
		if !slices.Contains(matcher.Types, types.AWSMatcherEC2) {
			continue
		}
		for _, region := range matcher.Regions {
			ssmCreator := awsSSMDocumentCreator{
				ssm:      ssm[region],
				Name:     matcher.SSM.DocumentName,
				Contents: awslib.EC2DiscoverySSMDocument(proxyAddr),
			}
			creators = append(creators, &ssmCreator)
		}
	}
	return creators
}

func isEC2AutoDiscoveryEnabled(flags configurators.BootstrapFlags, matchers []types.AWSMatcher) bool {
	if flags.ForceEC2Permissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherEC2, matchers)
}

// hasRDSDatabases checks if the agent needs permission for
// RDS/Aurora databases.
func hasRDSDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceRDSPermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherRDS, targetCfg.awsMatchers) ||
		findEndpointIs(targetCfg.databases, isRDSEndpoint)
}

// hasRDSProxyDatabases checks if the agent needs permission for
// RDS Proxy databases.
func hasRDSProxyDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceRDSProxyPermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherRDSProxy, targetCfg.awsMatchers) ||
		findEndpointIs(targetCfg.databases, isRDSProxyEndpoint)
}

// hasRedshiftDatabases checks if the agent needs permission for
// Redshift databases.
func hasRedshiftDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceRedshiftPermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherRedshift, targetCfg.awsMatchers) ||
		findEndpointIs(targetCfg.databases, apiawsutils.IsRedshiftEndpoint)
}

// hasRedshiftServerlessDatabases checks if the agent needs permission for
// Redshift Serverless databases.
func hasRedshiftServerlessDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceRedshiftServerlessPermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherRedshiftServerless, targetCfg.awsMatchers) ||
		findEndpointIs(targetCfg.databases, apiawsutils.IsRedshiftServerlessEndpoint)
}

// hasElastiCacheDatabases checks if the agent needs permission for
// ElastiCache databases.
func hasElastiCacheDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceElastiCachePermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherElastiCache, targetCfg.awsMatchers) ||
		findEndpointIs(targetCfg.databases, apiawsutils.IsElastiCacheEndpoint)
}

// hasMemoryDBDatabases checks if the agent needs permission for
// ElastiCache databases.
func hasMemoryDBDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceMemoryDBPermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherMemoryDB, targetCfg.awsMatchers) ||
		findEndpointIs(targetCfg.databases, apiawsutils.IsMemoryDBEndpoint)
}

// hasOpenSearchDatabases checks if the agent needs permission for OpenSearch
// databases.
func hasOpenSearchDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceOpenSearchPermissions {
		return true
	}
	return isAutoDiscoveryEnabledForMatcher(types.AWSMatcherOpenSearch, targetCfg.awsMatchers) ||
		findDatabaseIs(targetCfg.databases, func(db *servicecfg.Database) bool {
			return db.Protocol == defaults.ProtocolOpenSearch
		})
}

// hasAWSKeyspacesDatabases checks if the agent needs permission for AWS Keyspaces.
func hasAWSKeyspacesDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceAWSKeyspacesPermissions {
		return true
	}
	// There is no auto discovery for AWS Keyspaces.
	if flags.Service.IsDiscovery() {
		return false
	}
	return findDatabaseIs(targetCfg.databases, func(database *servicecfg.Database) bool {
		return database.Protocol == defaults.ProtocolCassandra && database.AWS.AccountID != ""
	})
}

// hasDynamoDBDatabases checks if the agent needs permission for AWS DynamoDB.
func hasDynamoDBDatabases(flags configurators.BootstrapFlags, targetCfg targetConfig) bool {
	if flags.ForceDynamoDBPermissions {
		return true
	}
	// There is no auto discovery for AWS DynamoDB.
	if flags.Service.IsDiscovery() {
		return false
	}
	return findDatabaseIs(targetCfg.databases, func(database *servicecfg.Database) bool {
		return database.Protocol == defaults.ProtocolDynamoDB
	})
}

// isAutoDiscoveryEnabledForMatcher returns true if provided AWS matcher type
// is found.
func isAutoDiscoveryEnabledForMatcher(matcherType string, matchers []types.AWSMatcher) bool {
	return findAWSMatcherIs(matchers, func(matcher *types.AWSMatcher) bool {
		for _, databaseType := range matcher.Types {
			if databaseType == matcherType {
				return true
			}
		}
		return false
	})
}

// findEndpointIs returns true if provided check returns true for any static
// endpoint.
func findEndpointIs(databases []*servicecfg.Database, endpointIs func(string) bool) bool {
	return findDatabaseIs(databases, func(database *servicecfg.Database) bool {
		return endpointIs(database.URI)
	})
}

// findDatabaseIs returns true if provided check returns true for any static
// database config.
func findDatabaseIs(databases []*servicecfg.Database, is func(*servicecfg.Database) bool) bool {
	for _, database := range databases {
		if is(database) {
			return true
		}
	}
	return false
}

// findAWSMatcherIs returns true if the provided check returns true for any
// AWS matcher.
func findAWSMatcherIs(matchers []types.AWSMatcher, is func(*types.AWSMatcher) bool) bool {
	for i := range matchers {
		if is(&matchers[i]) {
			return true
		}
	}
	return false
}

// supportsAWSAssumeRole returns true if the given matcher supports assuming
// AWS roles. Currently limited to just the AWS database matchers.
func supportsAWSAssumeRole(matcher types.AWSMatcher) bool {
	for _, matcherType := range matcher.Types {
		if slices.Contains(types.SupportedAWSDatabaseMatchers, matcherType) {
			return true
		}
	}
	return false
}

// isRDSEndpoint returns true if the endpoint is an endpoint for RDS instance or Aurora cluster.
func isRDSEndpoint(uri string) bool {
	details, err := apiawsutils.ParseRDSEndpoint(uri)
	if err != nil {
		return false
	}
	return !details.IsProxy()
}

// isRDSProxyEndpoint returns true if the endpoint is an endpoint for RDS Proxy.
func isRDSProxyEndpoint(uri string) bool {
	details, err := apiawsutils.ParseRDSEndpoint(uri)
	if err != nil {
		return false
	}
	return details.IsProxy()
}

// buildIAMEditStatements returns IAM statements necessary for the Teleport
// agent to edit user/role permissions.
func buildIAMEditStatements(target awslib.Identity) ([]*awslib.Statement, error) {
	switch target.(type) {
	case awslib.User, *awslib.User:
		return []*awslib.Statement{
			awslib.StatementForIAMEditUserPolicy(target.String()),
		}, nil

	case awslib.Role, *awslib.Role:
		return []*awslib.Statement{
			awslib.StatementForIAMEditRolePolicy(target.String()),
		}, nil

	default:
		return nil, trace.BadParameter("policies target must be an user or role, received %T", target)
	}
}

// buildEC2AutoDiscoveryStatements returns IAM statements necessary for
// EC2 instance auto-discovery.
func buildEC2AutoDiscoveryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		awslib.StatementForEC2SSMAutoDiscover(),
	}
}

// buildSecretsManagerStatements returns IAM statements necessary for using AWS
// Secrets Manager.
func buildSecretsManagerStatements(databases []*servicecfg.Database, target awslib.Identity) []*awslib.Statement {
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
	for _, database := range databases {
		if !apiawsutils.IsElastiCacheEndpoint(database.URI) &&
			!apiawsutils.IsMemoryDBEndpoint(database.URI) {
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

// buildSTSAssumeRoleStatements returns AWS IAM statements necessary for
// assuming AWS IAM roles.
func buildSTSAssumeRoleStatements(flags configurators.BootstrapFlags, targetCfg targetConfig, boundary bool) ([]*awslib.Statement, error) {
	if len(targetCfg.assumesAWSRoles) == 0 {
		return nil, nil
	}
	if boundary {
		return []*awslib.Statement{{
			Effect:    awslib.EffectAllow,
			Actions:   stsActions,
			Resources: []string{"*"},
		}}, nil
	}
	return []*awslib.Statement{{
		Effect:    awslib.EffectAllow,
		Actions:   stsActions,
		Resources: targetCfg.assumesAWSRoles,
	}}, nil
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

// targetConfig contains the target agent identity, and all associated databases,
// AWS matchers, and AWS role ARNs for that target identity.
// These are the resources that require AWS permissions for the identity to access.
type targetConfig struct {
	// identity is the target identity.
	identity awslib.Identity
	// awsMatchers are the AWS matchers associated with the target identity.
	awsMatchers []types.AWSMatcher
	// databases are the databases associated with the target identity.
	databases []*servicecfg.Database
	// assumesAWSRoles are the AWS IAM roles that the target identity needs to
	// be able to assume.
	assumesAWSRoles []string
}

// getTargetConfig gets the resources that are relevant to the target identity
// from cli flags and file configuration.
func getTargetConfig(flags configurators.BootstrapFlags, cfg *servicecfg.Config, target awslib.Identity) (targetConfig, error) {
	forcedRoles, err := parseForcedAWSRoles(flags, target)
	if err != nil {
		return targetConfig{}, trace.Wrap(err)
	}
	awsMatchers := awsMatchersFromConfig(flags, cfg)
	databases := databasesFromConfig(flags, cfg)
	resourceMatchers := resourceMatchersFromConfig(flags, cfg)
	targetIsAssumeRole := isTargetAWSAssumeRole(flags, awsMatchers, databases, resourceMatchers, target)
	targetAssumesRoles := rolesForTarget(forcedRoles, awsMatchers, databases, resourceMatchers, targetIsAssumeRole)
	err = checkStubRoleAssumingRolesFromConfig(forcedRoles, targetAssumesRoles, target)
	if err != nil {
		return targetConfig{}, trace.Wrap(err)
	}
	return targetConfig{
		identity:        target,
		awsMatchers:     matchersForTarget(awsMatchers, target, targetIsAssumeRole),
		databases:       databasesForTarget(databases, target, targetIsAssumeRole),
		assumesAWSRoles: targetAssumesRoles,
	}, nil
}

// checkStubRoleAssumingRolesFromConfig returns an error if a policy attachment
// target is a stub AWS IAM role target (contains placeholders in its ARN)
// that assumes at least one role from config not given in --assumes-roles.
//
// The configurator can be given a role name as the policy attachment target
// instead of a full ARN, but in --manual mode, the configurator constructs a
// stub ARN using "*" as a placeholder for the AWS account and partition.
// The stub role ARN will not match any `assume_role_arn` in config, so the
// configurator will not have enough information to correctly determine the
// required permissions policies for the target.
// We check for this scenario to avoid printing the wrong permissions in
// --manual mode, and advise users to specify a full role ARN instead of just
// the role's name.
func checkStubRoleAssumingRolesFromConfig(forcedRoles []string, targetAssumesRoles []string, target awslib.Identity) error {
	isRole := target.GetType() == "role"
	isStub := target.GetAccountID() == targetIdentityARNSectionPlaceholder ||
		target.GetPartition() == targetIdentityARNSectionPlaceholder
	// forcedRoles come from the cli flag `--assumes-roles`.
	// targetAssumesRoles is a superset of forcedRoles - it is the union
	// of forcedRoles and the `assume_role_arn` settings from config.
	// When targetAssumesRoles is bigger than the forced roles, it indicates
	// that there is at least one role in config that does not match any
	// forced role.
	// This also handles the case where forcedRoles are given as short names
	// instead of full ARNs in manual mode - if there are any roles in the
	// config, then this error will trigger when the policy attachment target is
	// a short role name.
	isTargetAssumingRolesInConfig := len(targetAssumesRoles) > len(forcedRoles)
	if isRole && isStub && isTargetAssumingRolesInConfig {
		return trace.BadParameter(
			"unable to determine required permissions for policy attachment "+
				"target %q in manual mode, please specify the full role ARN",
			target.GetName())
	}
	return nil
}

// awsMatchersFromConfig is a helper function that extracts database AWS matchers
// from the service configuration based on cli flags.
func awsMatchersFromConfig(flags configurators.BootstrapFlags, cfg *servicecfg.Config) []types.AWSMatcher {
	if flags.Service.UseDiscoveryServiceConfig() {
		return cfg.Discovery.AWSMatchers
	}
	return cfg.Databases.AWSMatchers
}

// databasesFromConfig is a helper function that extracts databases
// from the service configuration based on cli flags.
func databasesFromConfig(flags configurators.BootstrapFlags, cfg *servicecfg.Config) []*servicecfg.Database {
	if flags.Service.UseDiscoveryServiceConfig() {
		return nil
	}
	databases := make([]*servicecfg.Database, 0, len(cfg.Databases.Databases))
	for i := range cfg.Databases.Databases {
		databases = append(databases, &cfg.Databases.Databases[i])
	}
	return databases
}

func resourceMatchersFromConfig(flags configurators.BootstrapFlags, cfg *servicecfg.Config) []services.ResourceMatcher {
	if flags.Service.UseDiscoveryServiceConfig() {
		return nil
	}
	return cfg.Databases.ResourceMatchers
}

// isTargetAWSAssumeRole determines if the target identity exists in config or cli
// flags as an AWS IAM role arn that will be assumed by the database agent.
func isTargetAWSAssumeRole(flags configurators.BootstrapFlags, matchers []types.AWSMatcher, databases []*servicecfg.Database, resourceMatchers []services.ResourceMatcher, target awslib.Identity) bool {
	switch target.(type) {
	case awslib.Role, *awslib.Role:
	default:
		return false
	}

	targetARN := target.String()
	return isTargetAWSAssumeRoleForMatchers(matchers, targetARN) ||
		isTargetAWSAssumeRoleForDatabases(databases, targetARN) ||
		isTargetAWSAssumeRoleForResourceMatchers(resourceMatchers, targetARN)
}

// isTargetAWSAssumeRoleForMatchers checks if the target identity is the same as
// an AWS matcher's assume_role_arn.
func isTargetAWSAssumeRoleForMatchers(matchers []types.AWSMatcher, target string) bool {
	return findAWSMatcherIs(matchers, func(m *types.AWSMatcher) bool {
		assumeRoleARN := ""
		if m.AssumeRole != nil {
			assumeRoleARN = m.AssumeRole.RoleARN
		}
		return assumeRoleARN == target
	})
}

// isTargetAWSAssumeRoleForDatabases checks if the target identity is the same as
// an AWS database's assume_role_arn.
func isTargetAWSAssumeRoleForDatabases(databases []*servicecfg.Database, targetARN string) bool {
	return findDatabaseIs(databases, func(db *servicecfg.Database) bool {
		return db.AWS.AssumeRoleARN == targetARN
	})
}

func isTargetAWSAssumeRoleForResourceMatchers(resourceMatchers []services.ResourceMatcher, targetARN string) bool {
	for _, resourceMatcher := range resourceMatchers {
		if resourceMatcher.AWS.AssumeRoleARN == targetARN {
			return true
		}
	}
	return false
}

// predicate is a generic predicate function type.
type predicate[Elem any] func(t Elem) bool

// filter is a generic filtering function that returns all resources in a slice
// that the provided predicate function returns true for.
func filter[Elem any](elems []Elem, keepFn predicate[Elem]) []Elem {
	out := make([]Elem, 0, len(elems))
	for _, elem := range elems {
		if keepFn(elem) {
			out = append(out, elem)
		}
	}
	return out
}

// matchersForTarget returns all AWS matchers that are associated with the target identity.
func matchersForTarget(matchers []types.AWSMatcher, target awslib.Identity, targetIsAssumeRole bool) []types.AWSMatcher {
	if targetIsAssumeRole {
		targetARN := target.String()
		return filter(matchers, func(matcher types.AWSMatcher) bool {
			assumeRoleARN := ""
			if matcher.AssumeRole != nil {
				assumeRoleARN = matcher.AssumeRole.RoleARN
			}
			return assumeRoleARN == targetARN
		})
	}
	return filter(matchers, func(matcher types.AWSMatcher) bool {
		assumeRoleARN := ""
		if matcher.AssumeRole != nil {
			assumeRoleARN = matcher.AssumeRole.RoleARN
		}
		return assumeRoleARN == ""
	})
}

// databasesForTarget returns all databases that are associated with the target identity.
func databasesForTarget(databases []*servicecfg.Database, target awslib.Identity, targetIsAssumeRole bool) []*servicecfg.Database {
	if targetIsAssumeRole {
		targetARN := target.String()
		return filter(databases, func(database *servicecfg.Database) bool {
			return database.AWS.AssumeRoleARN == targetARN
		})
	}
	return filter(databases, func(database *servicecfg.Database) bool {
		return database.AWS.AssumeRoleARN == ""
	})
}

// parseForcedAWSRoles parses the bootstrap --assumes-roles flag as a
// comma-separated list of either complete IAM role ARNs, or as names of roles
// in the same account as the target identity, in which case it constructs the
// full role ARN using the target's partition and account ID.
func parseForcedAWSRoles(flags configurators.BootstrapFlags, target awslib.Identity) ([]string, error) {
	if flags.ForceAssumesRoles == "" {
		return nil, nil
	}
	var out []string
	for _, role := range strings.Split(flags.ForceAssumesRoles, ",") {
		if role == "" {
			continue
		}
		if !arn.IsARN(role) {
			role = buildIAMARN(target.GetPartition(), target.GetAccountID(), "role", role)
		}
		_, err := awsutils.ParseRoleARN(role)
		if err != nil && !isStubAccountIDError(target, err) {
			return nil, trace.BadParameter("--assumes-roles %q: %v", flags.ForceAssumesRoles, err)
		}
		out = append(out, role)
	}
	return out, nil
}

// isStubAccountIDError returns true if the given AWS IAM role parse error is
// from an invalid account ID due to a stub account ID "*" in the target identity.
func isStubAccountIDError(target awslib.Identity, err error) bool {
	return target.GetAccountID() == "*" && strings.Contains(err.Error(), "invalid account ID")
}

// rolesForTarget returns all AWS roles from cli flags, AWS matchers, and
// databases that the target identity will need to be able to assume.
func rolesForTarget(forcedRoles []string, matchers []types.AWSMatcher, databases []*servicecfg.Database, resourceMatchers []services.ResourceMatcher, targetIsAssumeRole bool) []string {
	roleSet := make(map[string]struct{})
	for _, roleARN := range forcedRoles {
		roleSet[roleARN] = struct{}{}
	}
	if targetIsAssumeRole {
		// if target is the same as some assume_role_arn in matchers/databases
		// config, then it shouldn't assume other roles from config.
		return utils.StringsSliceFromSet(roleSet)
	}
	for _, matcher := range matchers {
		assumeRoleARN := ""
		if matcher.AssumeRole != nil {
			assumeRoleARN = matcher.AssumeRole.RoleARN
		}

		if assumeRoleARN == "" || !supportsAWSAssumeRole(matcher) {
			continue
		}
		roleSet[assumeRoleARN] = struct{}{}
	}
	for _, db := range databases {
		if db.AWS.AssumeRoleARN == "" {
			continue
		}
		roleSet[db.AWS.AssumeRoleARN] = struct{}{}
	}
	for _, resourceMatcher := range resourceMatchers {
		if resourceMatcher.AWS.AssumeRoleARN == "" {
			continue
		}
		roleSet[resourceMatcher.AWS.AssumeRoleARN] = struct{}{}
	}
	return utils.StringsSliceFromSet(roleSet)
}
