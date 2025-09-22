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
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	apiaws "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/utils/teleportassets"
)

var (
	// launchTypeFargateString is the FARGATE LaunchType converted into a string.
	launchTypeFargateString = string(ecsTypes.LaunchTypeFargate)
	// requiredCapacityProviders contains the FARGATE type which is required to deploy a Teleport Service.
	requiredCapacityProviders = []string{launchTypeFargateString}

	// twoAgents is used to define the desired agent count when creating a service.
	// Deploying two agents in a FARGATE LaunchType Service, will most likely deploy
	// each one in a different AZ, as long as the Subnets include mustiple AZs.
	// From AWS Docs:
	// > Task placement strategies and constraints aren't supported for tasks using the Fargate launch type.
	// > Fargate will try its best to spread tasks across accessible Availability Zones.
	// > https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-placement.html#fargate-launch-type
	twoAgents = int32(2)
)

const (
	// clusterStatusActive is the string representing an ACTIVE ECS Cluster.
	clusterStatusActive = "ACTIVE"
	// clusterStatusInactive is the string representing an INACTIVE ECS Cluster.
	clusterStatusInactive = "INACTIVE"
	// clusterStatusProvisioning is the string representing an PROVISIONING ECS Cluster.
	clusterStatusProvisioning = "PROVISIONING"
	// clusterStatusProvisioningWaitTime defines for how long should the client wait for the Cluster to become available.
	clusterStatusProvisioningWaitTime = 30 * time.Second
	// clusterStatusProvisioningWaitTimeTick defines the interval between checks on Cluster status while it is Provisioning.
	clusterStatusProvisioningWaitTimeTick = 1 * time.Second

	// serviceStatusActive is the string representing an ACTIVE ECS Service.
	serviceStatusActive = "ACTIVE"
	// serviceStatusDraining is the string representing an DRAINING ECS Service.
	serviceStatusDraining = "DRAINING"

	// Ensure Cpu and Memory use one of the allowed combinations:
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html
	taskCPU = "2048"
	taskMem = "4096"

	// taskAgentContainerName is the name of the container to run within the Task.
	// Each task supports multiple containers, but, currently, there's only one being used.
	taskAgentContainerName = "teleport-service"

	// DatabaseServiceDeploymentMode is a deployment configuration for Deploying a Database Service.
	// This mode starts a Database with the specificied Resource Matchers.
	DatabaseServiceDeploymentMode = "database-service"
)

var (
	// DeploymentModes has all the available deployment modes.
	DeploymentModes = []string{
		DatabaseServiceDeploymentMode,
	}
)

// DeployServiceRequest contains the required fields to deploy a Teleport Service.
type DeployServiceRequest struct {
	// Region is the AWS Region
	Region string

	// AccountID is the AWS Account ID.
	// Optional. sts.GetCallerIdentity is used if the value is not provided.
	AccountID string

	// SubnetIDs are the subnets associated with the service.
	SubnetIDs []string

	// SecurityGroups to apply to the service's network configuration.
	// If empty, the default security group for the VPC is going to be used.
	SecurityGroups []string

	// ClusterName is the ECS Cluster to be used.
	// It will be created if it doesn't exist.
	// It will be updated if it doesn't include the FARGATE capacity provider using PutClusterCapacityProviders.
	ClusterName *string

	// ServiceName is the ECS Service to be used.
	// It will be created if it doesn't exist.
	// It will be updated if it doesn't match the required properties.
	ServiceName *string

	// TaskName is the ECS Task Definition's Family Name.
	TaskName *string

	// TaskRoleARN is the AWS Role's ARN used within the Task execution.
	// Ensure the AWS Client has `iam:PassRole` for this Role's ARN.
	TaskRoleARN string

	// TeleportClusterName is the Teleport Cluster Name, used to create default names for Cluster, Service and Task.
	TeleportClusterName string

	// DeploymentJoinTokenName is the Teleport IAM Token to use in the deployed Service.
	DeploymentJoinTokenName string

	// IntegrationName is the integration name.
	// Used for resource tagging when creating resources in AWS.
	IntegrationName string

	// ResourceCreationTags is used to add tags when creating resources in AWS.
	ResourceCreationTags tags.AWSTags

	// DeploymentMode is the identifier of a deployment mode - which Teleport Services to enable and their configuration.
	DeploymentMode string

	// TeleportVersionTag is the version of teleport to install.
	// Ensure the tag exists in:
	// public.ecr.aws/gravitational/teleport-distroless:<TeleportVersionTag>
	// Eg, 13.2.0
	// Optional. Defaults to the current version.
	TeleportVersionTag string

	// TeleportConfigString is the `teleport.yaml` configuration for the service to be deployed.
	// It should be base64 encoded as is expected by the `--config-string` param of `teleport start`.
	TeleportConfigString string
}

// normalizeECSResourceName converts a name into a valid ECS Resource Name.
// TeleportCluster name must match the following:
// > regexp.MustCompile(`^[0-9A-Za-z_\-@:./+]+$`)
//
// ECS Resources name must match the following:
// > Up to 255 letters (uppercase and lowercase), numbers, underscores, and hyphens are allowed.
// > regexp.MustCompile(`^[0-9A-Za-z_\-]+$`)
// The following resources should be normalized
// - ECS Cluster Name (r.ClusterName)
// - ECS Service Name (r.ServiceName)
// - ECS TaskDefinition Family (r.TaskName)
func normalizeECSResourceName(name string) string {
	replacer := strings.NewReplacer(
		"@", "_",
		":", "_",
		".", "_",
		"/", "_",
		"+", "_",
	)

	return replacer.Replace(name)
}

// normalizeECSClusterName returns the normalized ECS Cluster Name
func normalizeECSClusterName(teleportClusterName string) string {
	return normalizeECSResourceName(fmt.Sprintf("%s-teleport", teleportClusterName))
}

// normalizeECSServiceName returns the normalized ECS Service Name
func normalizeECSServiceName(teleportClusterName, deploymentMode string) string {
	return normalizeECSResourceName(fmt.Sprintf("%s-teleport-%s", teleportClusterName, deploymentMode))
}

// normalizeECSTaskName returns the normalized ECS TaskDefinition Family
func normalizeECSTaskName(teleportClusterName, deploymentMode string) string {
	return normalizeECSResourceName(fmt.Sprintf("%s-teleport-%s", teleportClusterName, deploymentMode))
}

// CheckAndSetDefaults checks if the required fields are present.
func (r *DeployServiceRequest) CheckAndSetDefaults() error {
	if r.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name is required")
	}

	if r.TeleportVersionTag == "" {
		r.TeleportVersionTag = teleport.Version
	}

	if r.DeploymentJoinTokenName == "" {
		return trace.BadParameter("deployment join token name is required")
	}

	if r.DeploymentMode == "" {
		return trace.BadParameter("deployment mode is required, please use one of the following: %v", DeploymentModes)
	}

	if !slices.Contains(DeploymentModes, r.DeploymentMode) {
		return trace.BadParameter("invalid deployment mode, please use one of the following: %v", DeploymentModes)
	}

	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if len(r.SubnetIDs) == 0 {
		return trace.BadParameter("at least one subnet id is required")
	}

	if r.TaskRoleARN == "" {
		return trace.BadParameter("task role arn is required")
	}

	if r.ClusterName == nil || *r.ClusterName == "" {
		clusterName := normalizeECSClusterName(r.TeleportClusterName)
		r.ClusterName = &clusterName
	}

	if r.ServiceName == nil || *r.ServiceName == "" {
		serviceName := normalizeECSServiceName(r.TeleportClusterName, r.DeploymentMode)
		r.ServiceName = &serviceName
	}

	if r.TaskName == nil || *r.TaskName == "" {
		taskName := normalizeECSTaskName(r.TeleportClusterName, r.DeploymentMode)
		r.TaskName = &taskName
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.ResourceCreationTags == nil {
		r.ResourceCreationTags = defaultResourceCreationTags(r.TeleportClusterName, r.IntegrationName)
	}

	if r.TeleportConfigString == "" {
		return trace.BadParameter("teleport config string is required")
	}

	return nil
}

// DeployServiceResponse contains the ARNs of the Amazon resources used to deploy the Teleport Service.
type DeployServiceResponse struct {
	// ClusterARN is the Amazon ECS Cluster ARN where the task was started.
	ClusterARN string

	// ServiceARN is the Amazon ECS Cluster Service ARN created to run the task.
	ServiceARN string

	// TaskDefinitionARN is the Amazon ECS Task Definition ARN created to run the  Teleport Service.
	TaskDefinitionARN string

	// ServiceDashboardURL is a link to the service's Dashboard URL in Amazon Console.
	ServiceDashboardURL string
}

// DeployServiceClient describes the required methods to Deploy a  Teleport Service.
type DeployServiceClient interface {
	// DescribeClusters lists ECS Clusters.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DescribeClusters
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)

	// CreateCluster creates a new cluster.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.CreateCluster
	CreateCluster(ctx context.Context, params *ecs.CreateClusterInput, optFns ...func(*ecs.Options)) (*ecs.CreateClusterOutput, error)

	// PutClusterCapacityProviders sets the Capacity Providers available for services in a given cluster.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.PutClusterCapacityProviders
	PutClusterCapacityProviders(ctx context.Context, params *ecs.PutClusterCapacityProvidersInput, optFns ...func(*ecs.Options)) (*ecs.PutClusterCapacityProvidersOutput, error)

	// DescribeServices lists the matching Services of a given Cluster.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DescribeServices
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)

	// ListServices returns a list of services. You can filter the results by cluster, launch type,
	// and scheduling strategy.
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)

	// UpdateService updates the service.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.UpdateService
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)

	// CreateService starts a task within a cluster.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.CreateService
	CreateService(ctx context.Context, params *ecs.CreateServiceInput, optFns ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error)

	// DescribeTaskDefinition describes the task definition.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DescribeTaskDefinition
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)

	// RegisterTaskDefinition registers a new task definition from the supplied family and containerDefinitions.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.RegisterTaskDefinition
	RegisterTaskDefinition(ctx context.Context, params *ecs.RegisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error)

	// DeregisterTaskDefinition deregisters the task definition.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DeregisterTaskDefinition
	DeregisterTaskDefinition(ctx context.Context, params *ecs.DeregisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DeregisterTaskDefinitionOutput, error)

	// TokenService are the required methods to manage the IAM Join Token.
	// When the deployed service connects to the cluster, it will use the IAM Join method.
	// Before deploying the service, it must ensure that the token exists and has the appropriate token rul.
	TokenService

	CallerIdentityGetter
}

type defaultDeployServiceClient struct {
	CallerIdentityGetter
	*ecs.Client
	tokenServiceClient TokenService
}

// GetToken returns a provision token by name.
func (d *defaultDeployServiceClient) GetToken(ctx context.Context, name string) (types.ProvisionToken, error) {
	return d.tokenServiceClient.GetToken(ctx, name)
}

// UpsertToken creates or updates a provision token.
func (d *defaultDeployServiceClient) UpsertToken(ctx context.Context, token types.ProvisionToken) error {
	return d.tokenServiceClient.UpsertToken(ctx, token)
}

// NewDeployServiceClient creates a new DeployServiceClient using a AWSClientRequest.
func NewDeployServiceClient(ctx context.Context, clientReq *AWSClientRequest, tokenServiceClient TokenService) (DeployServiceClient, error) {
	ecsClient, err := newECSClient(ctx, clientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stsClient, err := newSTSClient(ctx, clientReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultDeployServiceClient{
		Client:               ecsClient,
		CallerIdentityGetter: stsClient,
		tokenServiceClient:   tokenServiceClient,
	}, nil
}

// DeployService calls Amazon ECS APIs to deploy a Teleport Service.
//
// # Pre-requirement: Set up discover-aws-oidc-iam-token for auto join
//
// The Teleport Service connects via `discover-aws-oidc-iam-token`, so ensure your cluster has the following token:
//
//	kind: token
//	metadata:
//	  name: discover-aws-oidc-iam-token
//	spec:
//	  allow:
//	  - aws_account: "<account_id>"
//	  join_method: iam
//	  roles:
//	  - Db
//	version: v2
//
// You can also use the role received as parameter (req.TaskRoleARN) to have an even stricter matching.
// Eg of the identity ARN: "arn:aws:sts::0123456789012:assumed-role/<req.TaskRoleARN>/<abcd>"
//
// # Pre-requirement: TaskRole and Integration Role
// The required IAM Roles and Policies are described in [awsoidc.ConfigureDeployServiceIAM].

// # Resource tagging
//
// Created resources have the following set of tags:
// - teleport.dev/cluster: <clusterName>
// - teleport.dev/origin: aws-oidc-integration
// - teleport.dev/integration: <integrationName>
//
// If resources already exist, only resources with those tags will be updated.
func DeployService(ctx context.Context, clt DeployServiceClient, req DeployServiceRequest) (*DeployServiceResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.AccountID == "" {
		callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		req.AccountID = aws.ToString(callerIdentity.Account)
	}

	upsertTokenReq := upsertIAMJoinTokenRequest{
		tokenName:      req.DeploymentJoinTokenName,
		accountID:      req.AccountID,
		region:         req.Region,
		iamRole:        req.TaskRoleARN,
		deploymentMode: req.DeploymentMode,
	}
	if err := upsertIAMJoinToken(ctx, upsertTokenReq, clt); err != nil {
		return nil, trace.Wrap(err)
	}

	upsertTaskReq := upsertTaskRequest{
		TaskName:             aws.ToString(req.TaskName),
		TaskRoleARN:          req.TaskRoleARN,
		ClusterName:          aws.ToString(req.ClusterName),
		ServiceName:          aws.ToString(req.ServiceName),
		TeleportVersionTag:   req.TeleportVersionTag,
		ResourceCreationTags: req.ResourceCreationTags,
		Region:               req.Region,
		TeleportConfigB64:    req.TeleportConfigString,
	}
	taskDefinition, err := upsertTask(ctx, clt, upsertTaskReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	taskDefinitionARN := aws.ToString(taskDefinition.TaskDefinitionArn)

	cluster, err := upsertCluster(ctx, clt, aws.ToString(req.ClusterName), req.ResourceCreationTags)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upsertServiceReq := upsertServiceRequest{
		ServiceName:          aws.ToString(req.ServiceName),
		ClusterName:          aws.ToString(req.ClusterName),
		ResourceCreationTags: req.ResourceCreationTags,
		SubnetIDs:            req.SubnetIDs,
		SecurityGroups:       req.SecurityGroups,
	}
	service, err := upsertService(ctx, clt, upsertServiceReq, taskDefinitionARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &DeployServiceResponse{
		ClusterARN:          aws.ToString(cluster.ClusterArn),
		ServiceARN:          aws.ToString(service.ServiceArn),
		TaskDefinitionARN:   taskDefinitionARN,
		ServiceDashboardURL: serviceDashboardURL(req.Region, aws.ToString(req.ClusterName), aws.ToString(service.ServiceName)),
	}, nil
}

// serviceDashboardURL builds the ECS Service dashboard URL using the AWS Region, the ECS Cluster and Service Names.
// It returns an empty string if region is not valid.
func serviceDashboardURL(region, clusterName, serviceName string) string {
	if err := apiaws.IsValidRegion(region); err != nil {
		return ""
	}

	return fmt.Sprintf("https://%s.console.aws.amazon.com/ecs/v2/clusters/%s/services/%s", region, clusterName, serviceName)
}

type upsertTaskRequest struct {
	TaskName             string
	TaskRoleARN          string
	ClusterName          string
	ServiceName          string
	TeleportVersionTag   string
	ResourceCreationTags tags.AWSTags
	Region               string
	TeleportConfigB64    string
}

// upsertTask ensures a TaskDefinition with TaskName exists
func upsertTask(ctx context.Context, clt DeployServiceClient, req upsertTaskRequest) (*ecsTypes.TaskDefinition, error) {
	taskAgentContainerImage, err := getDistrolessTeleportImage(req.TeleportVersionTag)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	taskDefIn := &ecs.RegisterTaskDefinitionInput{
		Family: aws.String(req.TaskName),
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:    aws.String(taskCPU),
		Memory: aws.String(taskMem),

		NetworkMode:      ecsTypes.NetworkModeAwsvpc,
		TaskRoleArn:      &req.TaskRoleARN,
		ExecutionRoleArn: &req.TaskRoleARN,
		ContainerDefinitions: []ecsTypes.ContainerDefinition{{
			Environment: []ecsTypes.KeyValuePair{
				{
					Name:  aws.String(types.InstallMethodAWSOIDCDeployServiceEnvVar),
					Value: aws.String("true"),
				},
			},
			Command: []string{
				// --rewrite 15:3 rewrites SIGTERM -> SIGQUIT. This enables graceful shutdown of teleport
				"--rewrite",
				"15:3",
				"--",
				"teleport",
				"start",
				"--config-string",
				req.TeleportConfigB64,
			},
			EntryPoint: []string{"/usr/bin/dumb-init"},
			Image:      aws.String(taskAgentContainerImage),
			Name:       aws.String(taskAgentContainerName),
			LogConfiguration: &ecsTypes.LogConfiguration{
				LogDriver: ecsTypes.LogDriverAwslogs,
				Options: map[string]string{
					"awslogs-group":         "ecs-" + req.ClusterName,
					"awslogs-region":        req.Region,
					"awslogs-create-group":  "true",
					"awslogs-stream-prefix": req.ServiceName + "/" + req.TaskName,
				},
			},
		}},
		Tags: req.ResourceCreationTags.ToECSTags(),
	}

	// Ensure that the upgrader environment variables are set.
	// These will ensure that the instance reports Teleport upgrader metrics.
	if err := ensureUpgraderEnvironmentVariables(taskDefIn); err != nil {
		return nil, trace.Wrap(err)
	}

	taskDefOut, err := clt.RegisterTaskDefinition(ctx, taskDefIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return taskDefOut.TaskDefinition, nil
}

// upsertCluster creates the cluster if it doesn't exist.
// It will update the cluster if it doesn't have the required capacity provider (FARGATE)
// It will re-create if its status is INACTIVE.
// If the cluster status is not ACTIVE, an error is returned.
// The cluster is returned.
func upsertCluster(ctx context.Context, clt DeployServiceClient, clusterName string, resourceCreationTags tags.AWSTags) (*ecsTypes.Cluster, error) {
	describeClustersResponse, err := clt.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
		Include: []ecsTypes.ClusterField{
			ecsTypes.ClusterFieldTags,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if clusterMustBeCreated(describeClustersResponse.Clusters) {
		createClusterResp, err := clt.CreateCluster(ctx, &ecs.CreateClusterInput{
			ClusterName:       aws.String(clusterName),
			CapacityProviders: requiredCapacityProviders,
			Tags:              resourceCreationTags.ToECSTags(),
		}, func(o *ecs.Options) {
			o.Retryer = retry.NewStandard(func(so *retry.StandardOptions) {
				so.MaxAttempts = 10
				so.MaxBackoff = time.Minute
				// Retry if an error is a missing ECS service-linked role.
				// This is a retryable error because the ECS service-linked role
				// will be created automatically when the caller has
				// iam:CreateServiceLinkedRole permission (we should).
				// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using-service-linked-roles.html#create-slr
				so.Retryables = append(so.Retryables, retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
					if err != nil && strings.Contains(err.Error(), "verify that the ECS service linked role exists") {
						return aws.TrueTernary
					}
					return aws.FalseTernary
				}))
			})
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := waitForActiveCluster(ctx, clt, clusterName, createClusterResp.Cluster); err != nil {
			return nil, trace.Wrap(err)
		}

		return createClusterResp.Cluster, nil
	}

	// There's a cluster and it is not INACTIVE.
	cluster := &describeClustersResponse.Clusters[0]

	ownershipTags := resourceCreationTags
	if !ownershipTags.MatchesECSTags(cluster.Tags) {
		return nil, trace.Errorf("ECS Cluster %q already exists but is not managed by Teleport. "+
			"Add the following tags to allow Teleport to manage this cluster: %s", clusterName, resourceCreationTags)
	}

	if slices.Contains(cluster.CapacityProviders, launchTypeFargateString) {
		return cluster, nil
	}

	// Ensure the required capacity provider (Fargate) is available.
	putClusterCPResp, err := clt.PutClusterCapacityProviders(ctx, &ecs.PutClusterCapacityProvidersInput{
		Cluster:           aws.String(clusterName),
		CapacityProviders: requiredCapacityProviders,
		DefaultCapacityProviderStrategy: []ecsTypes.CapacityProviderStrategyItem{{
			CapacityProvider: &launchTypeFargateString,
		}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := waitForActiveCluster(ctx, clt, clusterName, cluster); err != nil {
		return nil, trace.Wrap(err)
	}

	return putClusterCPResp.Cluster, nil
}

// clusterMustBeCreated returns true if there's no cluster or the existing one has an Inactive (deleted) status.
func clusterMustBeCreated(clusters []ecsTypes.Cluster) bool {
	if len(clusters) == 0 {
		return true
	}

	cluster := clusters[0]

	return aws.ToString(cluster.Status) == clusterStatusInactive
}

// waitForActiveCluster waits until the Cluster is Active.
// If the Cluster is Provisioning, then it waits at most clusterStatusProvisioningWaitTime (30 seconds) for it to become ready.
func waitForActiveCluster(ctx context.Context, clt DeployServiceClient, clusterName string, cluster *ecsTypes.Cluster) error {
	if cluster.Status != nil && aws.ToString(cluster.Status) == clusterStatusActive {
		return nil
	}

	retry, err := retryutils.NewConstant(clusterStatusProvisioningWaitTimeTick)
	if err != nil {
		return trace.Wrap(err)
	}
	retryCtx, cancel := context.WithTimeout(ctx, clusterStatusProvisioningWaitTime)
	defer cancel()

	err = retry.For(retryCtx, func() error {
		describeClustersResponse, err := clt.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: []string{clusterName},
		})
		if err != nil {
			return retryutils.PermanentRetryError(trace.Wrap(err))
		}

		if len(describeClustersResponse.Clusters) == 0 {
			return retryutils.PermanentRetryError(trace.NotFound("cluster %q does not exist", aws.ToString(cluster.ClusterName)))
		}

		cluster := describeClustersResponse.Clusters[0]
		if cluster.Status == nil {
			return retryutils.PermanentRetryError(trace.Errorf("cluster %q has an unknown (nil) status", aws.ToString(cluster.ClusterName)))
		}

		if aws.ToString(cluster.Status) == clusterStatusActive {
			return nil
		}

		if aws.ToString(cluster.Status) == clusterStatusProvisioning {
			return trace.Errorf("cluster %q is provisioning...", aws.ToString(cluster.ClusterName))
		}

		return retryutils.PermanentRetryError(trace.Errorf("unexpected status %s for ECS Cluster %q", aws.ToString(cluster.ClusterName), aws.ToString(cluster.Status)))
	})

	return trace.Wrap(err)
}

func deployServiceNetworkConfiguration(subnetIDs, securityGroups []string) *ecsTypes.NetworkConfiguration {
	return &ecsTypes.NetworkConfiguration{
		AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
			AssignPublicIp: ecsTypes.AssignPublicIpEnabled, // no internet connection otherwise
			Subnets:        subnetIDs,
			SecurityGroups: securityGroups,
		},
	}
}

type upsertServiceRequest struct {
	ServiceName          string
	ClusterName          string
	ResourceCreationTags tags.AWSTags
	SubnetIDs            []string
	SecurityGroups       []string
}

// upsertService creates or updates the service.
// If the service exists but its LaunchType is not Fargate, then it gets re-created.
func upsertService(ctx context.Context, clt DeployServiceClient, req upsertServiceRequest, taskARN string) (*ecsTypes.Service, error) {
	describeServiceOut, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Services: []string{req.ServiceName},
		Cluster:  aws.String(req.ClusterName),
		Include: []ecsTypes.ServiceField{
			ecsTypes.ServiceFieldTags,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Service already exists.
	if len(describeServiceOut.Services) > 0 {
		service := &describeServiceOut.Services[0]

		if service.Status == nil {
			return nil, trace.Errorf("unknown status for ECS Service %q", req.ServiceName)
		}

		if aws.ToString(service.Status) == serviceStatusDraining {
			return nil, trace.Errorf("ECS Service is draining, please retry in a couple of minutes")
		}

		if aws.ToString(service.Status) == serviceStatusActive {
			ownershipTags := req.ResourceCreationTags
			if !ownershipTags.MatchesECSTags(service.Tags) {
				return nil, trace.Errorf("ECS Service %q already exists but is not managed by Teleport. "+
					"Add the following tags to allow Teleport to manage this service: %s", req.ServiceName, req.ResourceCreationTags)
			}

			// If the LaunchType is the required one, than we can update the current Service.
			// Otherwise we have to delete it.
			if service.LaunchType != ecsTypes.LaunchTypeFargate {
				return nil, trace.Errorf("ECS Service %q already exists but has an invalid LaunchType %q. Delete the Service and try again.", req.ServiceName, service.LaunchType)
			}

			updateServiceResp, err := clt.UpdateService(ctx, &ecs.UpdateServiceInput{
				Service:              aws.String(req.ServiceName),
				DesiredCount:         &twoAgents,
				TaskDefinition:       &taskARN,
				Cluster:              aws.String(req.ClusterName),
				NetworkConfiguration: deployServiceNetworkConfiguration(req.SubnetIDs, req.SecurityGroups),
				ForceNewDeployment:   true,
				PropagateTags:        ecsTypes.PropagateTagsService,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return updateServiceResp.Service, nil
		}
	}

	createServiceOut, err := clt.CreateService(ctx, &ecs.CreateServiceInput{
		ServiceName:          aws.String(req.ServiceName),
		DesiredCount:         &twoAgents,
		LaunchType:           ecsTypes.LaunchTypeFargate,
		TaskDefinition:       &taskARN,
		Cluster:              aws.String(req.ClusterName),
		NetworkConfiguration: deployServiceNetworkConfiguration(req.SubnetIDs, req.SecurityGroups),
		Tags:                 req.ResourceCreationTags.ToECSTags(),
		PropagateTags:        ecsTypes.PropagateTagsService,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createServiceOut.Service, nil
}

// getDistrolessTeleportImage returns the distroless teleport image string
func getDistrolessTeleportImage(version string) (string, error) {
	semVer, err := semver.NewVersion(version)
	if err != nil {
		return "", trace.BadParameter("invalid version tag %s", version)
	}

	return teleportassets.DistrolessImage(*semVer), nil
}
