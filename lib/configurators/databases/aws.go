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

package databases

import (
	"context"
	"fmt"
	"strings"

	awsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/secrets"
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws/arn"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
)

const (
	// DefaultPolicyName default policy name.
	DefaultPolicyName = "DatabaseAccess"
	// defaultPolicyDescription description used on the policy created.
	defaultPolicyDescription = "Used by Teleport database agents for discovering AWS-hosted databases."
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
	// rdsActions list of actions used when giving RDS permissions.
	rdsActions = []string{"rds:DescribeDBInstances", "rds:ModifyDBInstance"}
	// auroraActions list of actions used when giving RDS Aurora permissions.
	auroraActions = []string{"rds:DescribeDBClusters", "rds:ModifyDBCluster"}
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
	// boundaryRDSAuroraActions additional actions added to the policy boundary
	// when policy has RDS auto-discovery.
	boundaryRDSAuroraActions = []string{"rds-db:connect"}
	// boundaryRedshiftActions additional actions added to the policy boundary
	// when policy has Redshift auto-discovery.
	boundaryRedshiftActions = []string{"redshift:GetClusterCredentials"}
)

// awsConfigurator defines the AWS database configurator.
type awsConfigurator struct {
	// config AWS configurator list of configs.
	config AWSConfiguratorConfig
	// actions list of the configurator actions, those are populated on the
	// `build` function.
	actions []ConfiguratorAction
}

type AWSConfiguratorConfig struct {
	// Flags user-provided flags to configure/execute the configurator.
	Flags BootstrapFlags
	// FileConfig Teleport database agent config.
	FileConfig *config.FileConfig
	// AWSSession current AWS session.
	AWSSession *awssession.Session
	// AWSSTSClient AWS STS client.
	AWSSTSClient stsiface.STSAPI
	// Policies instance of the `Policies` that the actions use.
	Policies awslib.Policies
	// Identity is the current AWS credentials chain identity.
	Identity awslib.Identity
}

// CheckAndSetDefaults checks and set configuration default values.
func (c *AWSConfiguratorConfig) CheckAndSetDefaults() error {
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

		if c.Identity == nil {
			if c.AWSSTSClient == nil {
				c.AWSSTSClient = sts.New(c.AWSSession)
			}

			c.Identity, err = awslib.GetIdentityWithClient(context.Background(), c.AWSSTSClient)
			if err != nil {
				return trace.Wrap(err)
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
func NewAWSConfigurator(config AWSConfiguratorConfig) (Configurator, error) {
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
func (a *awsConfigurator) Actions() []ConfiguratorAction {
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
func (a *awsPolicyCreator) Execute(ctx context.Context, actionCtx *ConfiguratorActionContext) error {
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
func (a *awsPoliciesAttacher) Execute(ctx context.Context, actionCtx *ConfiguratorActionContext) error {
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

// buildActions generates the policy documents and configurator actions.
func buildActions(config AWSConfiguratorConfig) ([]ConfiguratorAction, error) {
	// Identity is going to be empty (`nil`) when running the command on
	// `Manual` mode, place a wildcard to keep the generated policies valid.
	accountID := "*"
	partitionID := "*"
	if config.Identity != nil {
		accountID = config.Identity.GetAccountID()
		partitionID = config.Identity.GetPartition()
	}

	// Define the target and target type.
	target, err := policiesTarget(config.Flags, accountID, partitionID, config.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Generate policies.
	policy, err := buildPolicyDocument(config.Flags, config.FileConfig, target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	policyBoundary, err := buildPolicyBoundaryDocument(config.Flags, config.FileConfig, target)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	formattedPolicy, err := policy.Document.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	formattedPolicyBoundary, err := policyBoundary.Document.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If the policy has no statements means that the agent doesn't require
	// any IAM permission. In this case, return without errors and with empty
	// actions.
	if len(policy.Document.Statements) == 0 {
		return []ConfiguratorAction{}, nil
	}

	return []ConfiguratorAction{
		// Create IAM Policy.
		&awsPolicyCreator{
			policies:        config.Policies,
			policy:          policy,
			formattedPolicy: formattedPolicy,
		},
		// Create IAM Policy boundary.
		&awsPolicyCreator{
			policies:        config.Policies,
			policy:          policyBoundary,
			formattedPolicy: formattedPolicyBoundary,
			isBoundary:      true,
		},
		// Attach both policies to the target.
		&awsPoliciesAttacher{policies: config.Policies, target: target},
	}, nil
}

// policiesTarget defines which target and its type the policies will be
// attached to.
func policiesTarget(flags BootstrapFlags, accountID string, partitionID string, identity awslib.Identity) (awslib.Identity, error) {
	if flags.AttachToUser != "" {
		userArn := flags.AttachToUser
		if !arn.IsARN(flags.AttachToUser) {
			userArn = fmt.Sprintf("arn:%s:iam::%s:user/%s", partitionID, accountID, flags.AttachToUser)
		}

		return awslib.IdentityFromArn(userArn)
	}

	if flags.AttachToRole != "" {
		roleArn := flags.AttachToRole
		if !arn.IsARN(flags.AttachToRole) {
			roleArn = fmt.Sprintf("arn:%s:iam::%s:role/%s", partitionID, accountID, flags.AttachToRole)
		}

		return awslib.IdentityFromArn(roleArn)
	}

	if identity == nil {
		return awslib.IdentityFromArn(fmt.Sprintf("arn:%s:iam::%s:user/%s", partitionID, accountID, defaultAttachUser))
	}

	return identity, nil
}

// buildPolicyBoundaryDocument builds the policy document.
func buildPolicyDocument(flags BootstrapFlags, fileConfig *config.FileConfig, target awslib.Identity) (*awslib.Policy, error) {
	var statements []*awslib.Statement
	rdsAutoDiscovery := isRDSAutoDiscoveryEnabled(flags, fileConfig)
	redshiftDatabases := hasRedshiftDatabases(flags, fileConfig)
	elastiCacheDatabases := hasElastiCacheDatabases(flags, fileConfig)
	memoryDBDatabases := hasMemoryDBDatabases(flags, fileConfig)
	requireSecretsManager := elastiCacheDatabases || memoryDBDatabases

	if rdsAutoDiscovery {
		statements = append(statements, buildRDSAutoDiscoveryStatements()...)
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

	// If RDS the auto discovery is enabled or there are Redshift databases, we
	// need permission to edit the target user/role.
	if rdsAutoDiscovery || redshiftDatabases {
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
		defaultPolicyDescription,
		defaultPolicyTags,
		document,
	), nil
}

// buildPolicyBoundaryDocument builds the policy boundary document.
func buildPolicyBoundaryDocument(flags BootstrapFlags, fileConfig *config.FileConfig, target awslib.Identity) (*awslib.Policy, error) {
	var statements []*awslib.Statement
	rdsAutoDiscovery := isRDSAutoDiscoveryEnabled(flags, fileConfig)
	redshiftDatabases := hasRedshiftDatabases(flags, fileConfig)
	elastiCacheDatabases := hasElastiCacheDatabases(flags, fileConfig)
	memoryDBDatabases := hasMemoryDBDatabases(flags, fileConfig)
	requireSecretsManager := elastiCacheDatabases || memoryDBDatabases

	if rdsAutoDiscovery {
		statements = append(statements, buildRDSAutoDiscoveryBoundaryStatements()...)
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

	// If RDS the auto discovery is enabled or there are Redshift databases, we
	// need permission to edit the target user/role.
	if rdsAutoDiscovery || redshiftDatabases {
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
		defaultPolicyDescription,
		defaultPolicyTags,
		document,
	), nil
}

// isRDSAutoDiscoveryEnabled checks if the agent needs permission for
// RDS/Aurora auto-discovery.
func isRDSAutoDiscoveryEnabled(flags BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceRDSPermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(fileConfig, services.AWSMatcherRDS)
}

// hasRedshiftDatabases checks if the agent needs permission for
// Redshift databases.
func hasRedshiftDatabases(flags BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceRedshiftPermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(fileConfig, services.AWSMatcherRedshift) ||
		findEndpointIs(fileConfig, awsutils.IsRedshiftEndpoint)
}

// hasElastiCacheDatabases checks if the agent needs permission for
// ElastiCache databases.
func hasElastiCacheDatabases(flags BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceElastiCachePermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(fileConfig, services.AWSMatcherElastiCache) ||
		findEndpointIs(fileConfig, awsutils.IsElastiCacheEndpoint)
}

// hasMemoryDBDatabases checks if the agent needs permission for
// ElastiCache databases.
func hasMemoryDBDatabases(flags BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceMemoryDBPermissions {
		return true
	}

	return isAutoDiscoveryEnabledForMatcher(fileConfig, services.AWSMatcherMemoryDB) ||
		findEndpointIs(fileConfig, awsutils.IsMemoryDBEndpoint)
}

// isAutoDiscoveryEnabledForMatcher returns true if provided AWS matcher type
// is found.
func isAutoDiscoveryEnabledForMatcher(fileConfig *config.FileConfig, matcherType string) bool {
	for _, matcher := range fileConfig.Databases.AWSMatchers {
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

// buildRDSAutoDiscoveryStatements returns IAM statements necessary for
// RDS/Aurora databases auto-discovery.
func buildRDSAutoDiscoveryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   append(rdsActions, auroraActions...),
			Resources: []string{"*"},
		},
	}
}

// buildRDSAutoDiscoveryBoundaryStatements returns IAM boundary statements
// necessary for RDS/Aurora databases auto-discovery.
func buildRDSAutoDiscoveryBoundaryStatements() []*awslib.Statement {
	return []*awslib.Statement{
		{
			Effect:    awslib.EffectAllow,
			Actions:   append(rdsActions, append(auroraActions, boundaryRDSAuroraActions...)...),
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
