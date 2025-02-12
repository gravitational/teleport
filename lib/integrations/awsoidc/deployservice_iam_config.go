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

package awsoidc

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awsapiutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/cloud/provisioning/awsactions"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
	awslibutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

var taskRoleDescription = "Used by Teleport Database Service deployed in Amazon ECS."

// DeployServiceIAMConfigureRequest is a request to configure the DeployService action required Roles.
type DeployServiceIAMConfigureRequest struct {
	// Cluster is the Teleport Cluster.
	// Used for tagging the created Roles/Policies.
	Cluster string

	// IntegrationName is the Integration Name.
	// Used for tagging the created Roles/Policies.
	IntegrationName string

	// Region is the AWS Region.
	// Used to set up the AWS SDK Client.
	Region string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleDeployServicePolicy is the Policy Name that is created to allow the DeployService to call AWS APIs (ecs, logs).
	// Defaults to DeployService.
	IntegrationRoleDeployServicePolicy string

	// TaskRole is the AWS Role used by the deployed service.
	TaskRole string

	// AccountID is the AWS Account ID.
	AccountID string

	// ResourceCreationTags is used to add tags when creating resources in AWS.
	// Defaults to:
	// - teleport.dev/cluster: <cluster>
	// - teleport.dev/origin: aws-oidc-integration
	// - teleport.dev/integration: <integrationName>
	ResourceCreationTags tags.AWSTags

	// AutoConfirm skips user confirmation of the operation plan if true.
	AutoConfirm bool

	// partitionID is the AWS Partition ID.
	// Eg, aws, aws-cn, aws-us-gov
	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html
	partitionID string

	// stdout is used to override stdout output in tests.
	stdout io.Writer
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *DeployServiceIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.Cluster == "" {
		return trace.BadParameter("cluster is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.IntegrationRoleDeployServicePolicy == "" {
		r.IntegrationRoleDeployServicePolicy = "DeployService"
	}

	if r.TaskRole == "" {
		return trace.BadParameter("task role is required")
	}

	if len(r.ResourceCreationTags) == 0 {
		r.ResourceCreationTags = tags.DefaultResourceCreationTags(r.Cluster, r.IntegrationName)
	}

	r.partitionID = awsapiutils.GetPartitionFromRegion(r.Region)

	return nil
}

// DeployServiceIAMConfigureClient describes the required methods to create the IAM Roles/Policies required for the DeployService action.
type DeployServiceIAMConfigureClient interface {
	CallerIdentityGetter
	awsactions.AssumeRolePolicyUpdater
	awsactions.RoleCreator
	awsactions.RoleGetter
	awsactions.RolePolicyPutter
	awsactions.RoleTagger
}

type defaultDeployServiceIAMConfigureClient struct {
	*iam.Client
	CallerIdentityGetter
}

// NewDeployServiceIAMConfigureClient creates a new DeployServiceIAMConfigureClient.
func NewDeployServiceIAMConfigureClient(ctx context.Context, region string) (DeployServiceIAMConfigureClient, error) {
	if region == "" {
		return nil, trace.BadParameter("region is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultDeployServiceIAMConfigureClient{
		Client:               iam.NewFromConfig(cfg),
		CallerIdentityGetter: stsutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureDeployServiceIAM set ups the roles required for calling the DeployService action.
// It creates the following:
//
// A) Role to be used by the deployed service, also known as _TaskRole_.
// The Role is able to manage policies and create logs.
//
// B) Create a Policy in the Integration Role - the role used when setting up the integration.
// This policy allows for the required API Calls to set up the Amazon ECS TaskDefinition, Cluster and Service.
// It also allows to 'iam:PassRole' only for the _TaskRole_.
//
// The following actions must be allowed by the IAM Role assigned in the Client.
// - iam:CreateRole
// - iam:PutRolePolicy
// - iam:TagRole
func ConfigureDeployServiceIAM(ctx context.Context, clt DeployServiceIAMConfigureClient, req DeployServiceIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if req.AccountID == "" {
		callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		req.AccountID = aws.ToString(callerIdentity.Account)
	} else if err := CheckAccountID(ctx, clt, req.AccountID); err != nil {
		return trace.Wrap(err)
	}

	createTaskRole, err := createTaskRoleAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	addPolicyToTaskRole, err := addPolicyToTaskRoleAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	addPolicyToIntegrationRole, err := addPolicyToIntegrationRoleAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name: "deployservice-iam",
		Actions: []provisioning.Action{
			*createTaskRole,
			*addPolicyToTaskRole,
			*addPolicyToIntegrationRole,
		},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}))
}

// createTaskRoleAction returns an action that creates the TaskRole and sets up
// its permissions and trust relationship.
func createTaskRoleAction(clt DeployServiceIAMConfigureClient, req DeployServiceIAMConfigureRequest) (*provisioning.Action, error) {
	trustPolicy := awslib.NewPolicyDocument(
		awslib.StatementForECSTaskRoleTrustRelationships(),
	)
	return awsactions.CreateRole(clt, req.TaskRole, taskRoleDescription, trustPolicy, req.ResourceCreationTags)
}

// addPolicyToTaskRoleAction returns an action that updates the TaskRole to
// allow the service to:
// - manage Policies of the TaskRole
// - write logs to CloudWatch
func addPolicyToTaskRoleAction(clt DeployServiceIAMConfigureClient, req DeployServiceIAMConfigureRequest) (*provisioning.Action, error) {
	policy := awslib.NewPolicyDocument(
		awslib.StatementForRDSDBConnect(),
		awslib.StatementForRDSMetadata(),
		awslib.StatementForWritingLogs(),
	)
	policyName := req.TaskRole // we name the inline policy with the role's name
	return awsactions.PutRolePolicy(clt, policyName, req.TaskRole, policy)
}

// addPolicyToIntegrationRoleAction returns an action that creates or updates
// the DeployService Policy in IntegrationRole.
// It allows the Proxy to call ECS APIs and to pass the TaskRole when deploying
// a service.
func addPolicyToIntegrationRoleAction(clt DeployServiceIAMConfigureClient, req DeployServiceIAMConfigureRequest) (*provisioning.Action, error) {
	taskRoleARN := awslibutils.RoleARN(req.partitionID, req.AccountID, req.TaskRole)

	policy := awslib.NewPolicyDocument(
		awslib.StatementForIAMPassRole(taskRoleARN),
		awslib.StatementForECSManageService(),
	)
	policyName := req.IntegrationRoleDeployServicePolicy
	return awsactions.PutRolePolicy(clt, policyName, req.IntegrationRole, policy)
}
