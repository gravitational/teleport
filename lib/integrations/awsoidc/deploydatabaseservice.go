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
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// DeployDatabaseServiceRequest contains the required fields to deploy multiple Teleport Databases Services.
// Each Service will proxy a specific set of Databases, based on their "account-id", "region" and "vpc-id" labels.
type DeployDatabaseServiceRequest struct {
	// Region is the AWS Region
	Region string

	// Deployments contains a list of services to be deployed.
	Deployments []DeployDatabaseServiceRequestDeployment

	// TaskRoleARN is the AWS Role's ARN used within the Task execution.
	// Ensure the AWS Client has `iam:PassRole` for this Role's ARN.
	TaskRoleARN string

	// TeleportClusterName is the Teleport Cluster Name.
	// Used to create names for Cluster and TaskDefinitions, and AWS resource tags.
	TeleportClusterName string

	// IntegrationName is the integration name.
	// Used for resource tagging when creating resources in AWS.
	IntegrationName string

	// TeleportVersionTag is the version of teleport to install.
	// Ensure the tag exists in:
	// public.ecr.aws/gravitational/teleport-distroless:<TeleportVersionTag>
	// Eg, 13.2.0
	// Optional. Defaults to the current version.
	TeleportVersionTag string

	// ResourceCreationTags is used to add tags when creating resources in AWS.
	ResourceCreationTags AWSTags

	// DeploymentJoinTokenName is the Teleport IAM Join Token name that the deployed service must use to join the cluster.
	DeploymentJoinTokenName string

	// ecsClusterName is the ECS Cluster Name to be used.
	// It is based on the Teleport Cluster's Name.
	ecsClusterName string

	// accountID is the AWS Account ID.
	// sts.GetCallerIdentity is used to obtain its value.
	accountID string
}

// CheckAndSetDefaults checks if the required fields are present.
func (r *DeployDatabaseServiceRequest) CheckAndSetDefaults() error {
	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if len(r.Deployments) == 0 {
		return trace.BadParameter("at least one deployment is required")
	}
	for _, deployment := range r.Deployments {
		if deployment.VPCID == "" {
			return trace.BadParameter("vpcid is required in every deployment")
		}
		if len(deployment.SubnetIDs) == 0 {
			return trace.BadParameter("at least one subnet is required in every deployment")
		}

		if deployment.DeployServiceConfig == "" {
			return trace.BadParameter("deploy service config is required")
		}
	}

	if r.TaskRoleARN == "" {
		return trace.BadParameter("task role arn is required")
	}

	if r.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.DeploymentJoinTokenName == "" {
		return trace.BadParameter("invalid deployment join token name")
	}

	if r.TeleportVersionTag == "" {
		r.TeleportVersionTag = teleport.Version
	}

	if r.ResourceCreationTags == nil {
		r.ResourceCreationTags = defaultResourceCreationTags(r.TeleportClusterName, r.IntegrationName)
	}

	r.ecsClusterName = normalizeECSClusterName(r.TeleportClusterName)

	return nil
}

// DeployDatabaseServiceRequestDeployment identifies the required fields to deploy a DatabaseService.
type DeployDatabaseServiceRequestDeployment struct {
	// VPCID is the VPCID where the service is going to be deployed.
	VPCID string

	// SubnetIDs are the subnets for the network configuration.
	// They must belong to the VPCID above.
	SubnetIDs []string

	// SecurityGroupIDs are the SecurityGroups that should be applied to the ECS Service.
	// Optional. If empty, uses the VPC's default SecurityGroup.
	SecurityGroupIDs []string

	// DeployServiceConfig is the `teleport.yaml` configuration for the service to be deployed.
	// It should be base64 encoded as is expected by the `--config-string` param of `teleport start`.
	DeployServiceConfig string
}

// DeployDatabaseServiceResponse contains the ARNs of the Amazon resources used to deploy the Teleport Service.
type DeployDatabaseServiceResponse struct {
	// ClusterARN is the Amazon ECS Cluster ARN where the task was started.
	ClusterARN string

	// ClusterDashboardURL is a link to the Cluster's Dashboard URL in Amazon Console.
	ClusterDashboardURL string
}

// DeployDatabaseService calls Amazon ECS APIs to deploy multiple Teleport DatabaseService.
// Each DatabaseService is created per Deployment and proxies the DBs that match a specific set of labels.
//
// Those DatabaseServices join the cluster using an IAM Join Token, which is created if it doesn't exist yet.
//
// The following AWS ECS resources are created: Cluster, Services and Task Definition.
//
// A single ECS Cluster is created, named <teleport-cluster-name>-teleport
// Eg, tenant_teleport_sh-teleport
//
// An ECS TaskDefinition is created per deployment.
// It uses Teleport Image and is configured to start a DatabaseService proxying the
// databases that match on `account-id`, `region` and `vpc-id`.
// A new revision is created if it already exists.
// Example of an ECS TaskDefinition name: tenant_teleport_sh-teleport-database-service-vpc-123:1
//
// An ECS Service is created per deployment/ECS TaskDefinition.
// It will be configured to run on the VPC and have the Subnets and SecurityGroups defined in each deployment.
// If no SecurityGroup is provided, it will be assigned the VPC's default SecurityGroup.
// Its IAM access are the ones defined in the TaskRoleARN AWS IAM Role.
// For the required permissions, see the method ConfigureDeployServiceIAM.
// Example of an ECS Service name: database-service-vpc-123
//
// Each of those AWS ECS Resources will have the following set of tags:
//
// - teleport.dev/cluster: <clusterName>
//
// - teleport.dev/origin: aws-oidc-integration
//
// - teleport.dev/integration: <integrationName>
//
// If resources already exist, only resources with those tags will be updated.
func DeployDatabaseService(ctx context.Context, clt DeployServiceClient, req DeployDatabaseServiceRequest) (*DeployDatabaseServiceResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.accountID = aws.ToString(callerIdentity.Account)

	upsertTokenReq := upsertIAMJoinTokenRequest{
		accountID:      req.accountID,
		region:         req.Region,
		iamRole:        req.TaskRoleARN,
		deploymentMode: DatabaseServiceDeploymentMode,
		tokenName:      req.DeploymentJoinTokenName,
	}
	if err := upsertIAMJoinToken(ctx, upsertTokenReq, clt); err != nil {
		return nil, trace.Wrap(err)
	}

	log := slog.Default().With(
		"region", req.Region,
		"account_id", req.accountID,
		"integration", req.IntegrationName,
		"deployservice", DatabaseServiceDeploymentMode,
		"ecs_cluster", req.ecsClusterName,
	)

	log.DebugContext(ctx, "Upsert ECS Cluster")
	cluster, err := upsertCluster(ctx, clt, req.ecsClusterName, req.ResourceCreationTags)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, deployment := range req.Deployments {
		taskName := ecsTaskName(req.TeleportClusterName, DatabaseServiceDeploymentMode, deployment.VPCID)
		serviceName := ecsServiceName(DatabaseServiceDeploymentMode, deployment.VPCID)

		logDeployment := log.With(
			"vpc_id", deployment.VPCID,
			"ecs_task_name", taskName,
			"ecs_service_name", serviceName,
		)

		upsertTaskReq := upsertTaskRequest{
			TaskName:             taskName,
			TaskRoleARN:          req.TaskRoleARN,
			ClusterName:          req.ecsClusterName,
			ServiceName:          serviceName,
			TeleportVersionTag:   req.TeleportVersionTag,
			ResourceCreationTags: req.ResourceCreationTags,
			Region:               req.Region,
			TeleportConfigB64:    deployment.DeployServiceConfig,
		}
		logDeployment.DebugContext(ctx, "Upsert ECS TaskDefinition.")
		taskDefinition, err := upsertTask(ctx, clt, upsertTaskReq)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		taskDefinitionARN := aws.ToString(taskDefinition.TaskDefinitionArn)

		upsertServiceReq := upsertServiceRequest{
			ServiceName:          serviceName,
			ClusterName:          req.ecsClusterName,
			ResourceCreationTags: req.ResourceCreationTags,
			SubnetIDs:            deployment.SubnetIDs,
			SecurityGroups:       deployment.SecurityGroupIDs,
		}
		logDeployment.DebugContext(ctx, "Upsert ECS Service.")
		if _, err := upsertService(ctx, clt, upsertServiceReq, taskDefinitionARN); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clusterDashboardURL := fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s/services", req.Region, aws.ToString(cluster.ClusterName))

	return &DeployDatabaseServiceResponse{
		ClusterARN:          aws.ToString(cluster.ClusterArn),
		ClusterDashboardURL: clusterDashboardURL,
	}, nil
}

// ecsTaskName returns the normalized ECS TaskDefinition Family
func ecsTaskName(teleportClusterName, deploymentMode, vpcid string) string {
	return normalizeECSResourceName(fmt.Sprintf("%s-teleport-%s-%s", teleportClusterName, deploymentMode, vpcid))
}

// ecsServiceName returns the normalized ECS Service Family
func ecsServiceName(deploymentMode, vpcid string) string {
	return normalizeECSResourceName(fmt.Sprintf("%s-%s", deploymentMode, vpcid))
}
