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

	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws/arn"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
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
	// auroraActions list of acions used when giving RDS Aurora permissions.
	auroraActions = []string{"rds:DescribeDBClusters", "rds:ModifyDBCluster"}
	// redshiftActions list of actions used when giving Redshift auto-discovery
	// permissions.
	redshiftActions = []string{"redshift:DescribeClusters"}
	// boundaryRDSAuroraActions aditional actions added to the policy boundary
	// when policy has RDS auto-discovery.
	boundaryRDSAuroraActions = []string{"rds-db:connect"}
	// boundaryRedshiftActions aditional actions added to the policy boundary
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
			c.Policies = awslib.NewPolicies(c.Identity.GetAccountID(), iam.New(c.AWSSession))
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

// Name returns humam-readable configurator name.
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

// Details returnst the policy document that will be created.
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
	if config.Identity != nil {
		accountID = config.Identity.GetAccountID()
	}

	// Define the target and target type.
	target, err := policiesTarget(config.Flags, accountID, config.Identity)
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
func policiesTarget(flags BootstrapFlags, accountID string, identity awslib.Identity) (awslib.Identity, error) {
	if flags.AttachToUser != "" {
		userArn := flags.AttachToUser
		if !arn.IsARN(flags.AttachToUser) {
			userArn = fmt.Sprintf("arn:aws:iam::%s:user/%s", accountID, flags.AttachToUser)
		}

		return awslib.IdentityFromArn(userArn)
	}

	if flags.AttachToRole != "" {
		roleArn := flags.AttachToRole
		if !arn.IsARN(flags.AttachToRole) {
			roleArn = fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, flags.AttachToRole)
		}

		return awslib.IdentityFromArn(roleArn)
	}

	if identity == nil {
		return awslib.IdentityFromArn(fmt.Sprintf("arn:aws:iam::%s:user/%s", accountID, defaultAttachUser))
	}

	return identity, nil
}

// buildPolicyBoundaryDocument builds the policy document.
func buildPolicyDocument(flags BootstrapFlags, fileConfig *config.FileConfig, target awslib.Identity) (*awslib.Policy, error) {
	var statements []*awslib.Statement
	rdsAutoDiscovery := isRDSAutoDiscoveryEnabled(flags, fileConfig)
	redshiftDatabases := hasRedshiftDatabases(flags, fileConfig)

	if rdsAutoDiscovery {
		statements = append(statements, buildRDSAutoDiscoveryStatements()...)
	}

	if redshiftDatabases {
		statements = append(statements, buildRedshiftStatements()...)
	}

	// If RDS the auto discovery is enabled or there are Redshift databases, we
	// need permission to edit the target user/role.
	if rdsAutoDiscovery || redshiftDatabases {
		targetStaments, err := buildIAMEditStatements(target)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		statements = append(statements, targetStaments...)
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

	if rdsAutoDiscovery {
		statements = append(statements, buildRDSAutoDiscoveryBoundaryStatements()...)
	}

	if redshiftDatabases {
		statements = append(statements, buildRedshiftBoundaryStatements()...)
	}

	// If RDS the auto discovery is enabled or there are Redshift databases, we
	// need permission to edit the target user/role.
	if rdsAutoDiscovery || redshiftDatabases {
		targetStaments, err := buildIAMEditStatements(target)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		statements = append(statements, targetStaments...)
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

	for _, matcher := range fileConfig.Databases.AWSMatchers {
		for _, databaseType := range matcher.Types {
			if databaseType == types.DatabaseTypeRDS {
				return true
			}
		}
	}

	return false
}

// hasRedshiftDatabases checks if the agent needs permission for
// Redshift databases.
func hasRedshiftDatabases(flags BootstrapFlags, fileConfig *config.FileConfig) bool {
	if flags.ForceRedshiftPermissions {
		return true
	}

	// Check if Redshift auto-discovery is enabled.
	for _, matcher := range fileConfig.Databases.AWSMatchers {
		for _, databaseType := range matcher.Types {
			if databaseType == types.DatabaseTypeRedshift {
				return true
			}
		}
	}

	// Check if there is any static Redshift database configured.
	for _, database := range fileConfig.Databases.Databases {
		if awsutils.IsRedshiftEndpoint(database.URI) {
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
