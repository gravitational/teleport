/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsoidc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

var (
	// launchTypeFargateString is the FARGATE LaunchType converted into a string.
	launchTypeFargateString = string(ecsTypes.LaunchTypeFargate)
	// requiredCapacityProviders contains the FARGATE type which is required to deploy a Teleport Service.
	requiredCapacityProviders = []string{launchTypeFargateString}

	// Ensure Cpu and Memory use one of the allowed combinations:
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html
	taskCPU = "512"
	taskMem = "1024"

	// taskAgentContainerName is the name of the container to run within the Task.
	// Each task supports multiple containers, but, currently, there's only one being used.
	taskAgentContainerName = "teleport-service"

	// oneAgent is used to define the desired agent count when creating a service.
	oneAgent = int32(1)

	// serviceForceDeletion indicates that the service must be deleted even if it has running tasks.
	serviceForceDeletion = true
)

const (
	// teleportContainerImageFmt is the Teleport Container Image to be used
	teleportContainerImageFmt = "public.ecr.aws/gravitational/teleport-distroless:%s"

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
)

var (
	// DatabaseServiceDeploymentMode is a deployment configuration for Deploying a Database Service.
	// This mode starts a Database with the specificied Resource Matchers.
	DatabaseServiceDeploymentMode = "database-service"

	// DeploymentModes has all the available deployment modes.
	DeploymentModes = []string{
		DatabaseServiceDeploymentMode,
	}
)

// DeployServiceRequest contains the required fields to deploy a Teleport Service.
type DeployServiceRequest struct {
	// Region is the AWS Region
	Region string

	// SubnetIDs are the subnets associated with the service.
	SubnetIDs []string

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

	// ProxyServerHostPort is the Teleport Proxy's Public.
	ProxyServerHostPort string

	// IntegrationName is the integration name.
	// Used for resource tagging when creating resources in AWS.
	IntegrationName string

	// ResourceCreationTags is used to add tags when creating resources in AWS.
	ResourceCreationTags awsTags

	// DeploymentMode is the identifier of a deployment mode - which Teleport Services to enable and their configuration.
	DeploymentMode string

	// DatabaseResourceMatcherLabels contains the set of labels to be used by the DatabaseService.
	// This is used when the deployment mode creates a Database Service.
	DatabaseResourceMatcherLabels types.Labels
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

// CheckAndSetDefaults checks if the required fields are present.
func (r *DeployServiceRequest) CheckAndSetDefaults() error {
	if r.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name is required")
	}
	baseResourceName := normalizeECSResourceName(r.TeleportClusterName)

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
		clusterName := fmt.Sprintf("%s-teleport", baseResourceName)
		r.ClusterName = &clusterName
	}

	if r.ServiceName == nil || *r.ServiceName == "" {
		serviceName := fmt.Sprintf("%s-teleport-%s", baseResourceName, r.DeploymentMode)
		r.ServiceName = &serviceName
	}

	if r.TaskName == nil || *r.TaskName == "" {
		taskName := fmt.Sprintf("%s-teleport-%s", baseResourceName, r.DeploymentMode)
		r.TaskName = &taskName
	}

	if r.ProxyServerHostPort == "" {
		return trace.BadParameter("proxy address is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.ResourceCreationTags == nil {
		r.ResourceCreationTags = DefaultResourceCreationTags(r.TeleportClusterName, r.IntegrationName)
	}

	if len(r.DatabaseResourceMatcherLabels) == 0 {
		return trace.BadParameter("at least one agent matcher label is required")
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

	// UpdateService updates the service.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.UpdateService
	UpdateService(ctx context.Context, params *ecs.UpdateServiceInput, optFns ...func(*ecs.Options)) (*ecs.UpdateServiceOutput, error)

	// DeleteService deletes a service.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.DeleteService
	DeleteService(ctx context.Context, params *ecs.DeleteServiceInput, optFns ...func(*ecs.Options)) (*ecs.DeleteServiceOutput, error)

	// CreateService starts a task within a cluster.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.CreateService
	CreateService(ctx context.Context, params *ecs.CreateServiceInput, optFns ...func(*ecs.Options)) (*ecs.CreateServiceOutput, error)

	// RegisterTaskDefinition registers a new task definition from the supplied family and containerDefinitions.
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/ecs@v1.27.1#Client.RegisterTaskDefinition
	RegisterTaskDefinition(ctx context.Context, params *ecs.RegisterTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.RegisterTaskDefinitionOutput, error)
}

// NewDeployServiceClient creates a new DeployServiceClient using a AWSClientRequest.
func NewDeployServiceClient(ctx context.Context, clientReq *AWSClientRequest) (DeployServiceClient, error) {
	return newECSClient(ctx, clientReq)
}

// DeployService calls Amazon ECS APIs to deploy a Teleport Service.
//
// # Pre-requirement: Set up iam-token for auto join
//
// The Teleport Service connects via `iam-token`, so ensure your cluster has the following token:
//
//	kind: token
//	metadata:
//	  name: iam-token
//	spec:
//	  allow:
//	  - aws_account: "<account_id>"
//	  join_method: iam
//	  roles:
//	  - Discovery
//	  - Db
//	version: v2
//
// You can also use the role received as parameter (req.TaskRoleARN) to have an even stricter matching.
// Eg of the identity ARN: "arn:aws:sts::0123456789012:assumed-role/<req.TaskRoleARN>/<abcd>"
//
// # Pre-requirement: TaskRole creation
//
// The req.TaskRoleARN Role must have permissions according to the Teleport Services being deployed.
// Example for a DatabaseService:
//
//		{
//		    "Version": "2012-10-17",
//		    "Statement": [
//		        {
//		            "Effect": "Allow",
//		            "Action": [
//		                "iam:DeleteRolePolicy",
//		                "iam:PutRolePolicy",
//		                "iam:GetRolePolicy"
//		            ],
//		            "Resource": "arn:aws:iam::123456789012:role/<req.TaskRoleARN>"
//		        },
//	         {
//	             "Effect": "Allow",
//	             "Action": [
//	                  "rds:DescribeDBInstances",
//	                  "rds:ModifyDBInstance"
//	              ],
//	              "Resource": "*"
//	         },
//		        {
//		            "Effect": "Allow",
//		            "Action": "logs:*",
//		            "Resource": "*"
//		        }
//		    ]
//		}
//
// And the following Trust Policy
//
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Effect": "Allow",
//	            "Principal": {
//	                "Service": "ecs-tasks.amazonaws.com"
//	            },
//	            "Action": "sts:AssumeRole"
//	        }
//	    ]
//	}
//
// # Pre-requirement: AWS OIDC Integration Role
//
// To deploy those services the AWS OIDC Integration Role requires the following policy:
//
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Effect": "Allow",
//	            "Action": [
//	                "ecs:CreateCluster",
//	                "ecs:PutClusterCapacityProviders",
//	                "ecs:DescribeClusters",
//	                "ecs:RegisterTaskDefinition",
//	                "ecs:CreateService",
//	                "ecs:DescribeServices",
//	                "ecs:DeleteService",
//	                "ecs:UpdateService"
//	            ],
//	            "Resource": "*"
//	        },
//	        {
//	            "Effect": "Allow",
//	            "Action": [
//	                "iam:PassRole"
//	            ],
//	            "Resource": "arn:aws:iam::123456789012:role/<req.TaskRoleARN>"
//	        }
//	    ]
//	}
//
// # Resource tagging
//
// Created resources have the following set of tags:
// - teleport.dev/origin: aws-oidc-integration
// - teleport.dev/cluster: <clusterName>
// - teleport.dev/integration: <integrationName>
//
// If resources already exist, only resources with those tags will be updated.
func DeployService(ctx context.Context, clt DeployServiceClient, req DeployServiceRequest) (*DeployServiceResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	teleportConfigString, err := generateTeleportConfigString(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	taskDefinition, err := upsertTask(ctx, clt, req, teleportConfigString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	taskDefinitionARN := *taskDefinition.TaskDefinitionArn

	cluster, err := upsertCluster(ctx, clt, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	service, err := upsertService(ctx, clt, req, taskDefinitionARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &DeployServiceResponse{
		ClusterARN:        *cluster.ClusterArn,
		ServiceARN:        *service.ServiceArn,
		TaskDefinitionARN: taskDefinitionARN,
	}, nil
}

// upsertTask ensures a TaskDefinition with TaskName exists
func upsertTask(ctx context.Context, clt DeployServiceClient, req DeployServiceRequest, configB64 string) (*ecsTypes.TaskDefinition, error) {
	taskAgentContainerImage := fmt.Sprintf(teleportContainerImageFmt, teleport.Version)

	taskDefOut, err := clt.RegisterTaskDefinition(ctx, &ecs.RegisterTaskDefinitionInput{
		Family: req.TaskName,
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:    &taskCPU,
		Memory: &taskMem,

		NetworkMode:      ecsTypes.NetworkModeAwsvpc,
		TaskRoleArn:      &req.TaskRoleARN,
		ExecutionRoleArn: &req.TaskRoleARN,
		ContainerDefinitions: []ecsTypes.ContainerDefinition{{
			Command: []string{
				"start",
				"--config-string",
				configB64,
			},
			EntryPoint: []string{"teleport"},
			Image:      &taskAgentContainerImage,
			Name:       &taskAgentContainerName,
			LogConfiguration: &ecsTypes.LogConfiguration{
				LogDriver: ecsTypes.LogDriverAwslogs,
				Options: map[string]string{
					"awslogs-group":         "ecs-" + *req.ClusterName,
					"awslogs-region":        req.Region,
					"awslogs-create-group":  "true",
					"awslogs-stream-prefix": *req.ServiceName + "/" + *req.TaskName,
				},
			},
		}},
		Tags: req.ResourceCreationTags.ForECS(),
	})
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
func upsertCluster(ctx context.Context, clt DeployServiceClient, req DeployServiceRequest) (*ecsTypes.Cluster, error) {
	describeClustersResponse, err := clt.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{*req.ClusterName},
		Include: []ecsTypes.ClusterField{
			ecsTypes.ClusterFieldTags,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if clusterMustBeCreated(describeClustersResponse.Clusters) {
		createClusterResp, err := clt.CreateCluster(ctx, &ecs.CreateClusterInput{
			ClusterName:       req.ClusterName,
			CapacityProviders: requiredCapacityProviders,
			Tags:              req.ResourceCreationTags.ForECS(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := waitForActiveCluster(ctx, clt, req, createClusterResp.Cluster); err != nil {
			return nil, trace.Wrap(err)
		}

		return createClusterResp.Cluster, nil
	}

	// There's a cluster and it is not INACTIVE.
	cluster := &describeClustersResponse.Clusters[0]

	ownershipTags := req.ResourceCreationTags
	if !ownershipTags.MatchesECSTags(cluster.Tags) {
		return nil, trace.Errorf("ECS Cluster %q already exists but is not managed by Teleport. "+
			"Add the following tags to allow Teleport to manage this cluster: %s", *req.ClusterName, req.ResourceCreationTags)
	}

	if slices.Contains(cluster.CapacityProviders, launchTypeFargateString) {
		return cluster, nil
	}

	// Ensure the required capacity provider (Fargate) is available.
	putClusterCPResp, err := clt.PutClusterCapacityProviders(ctx, &ecs.PutClusterCapacityProvidersInput{
		Cluster:           req.ClusterName,
		CapacityProviders: requiredCapacityProviders,
		DefaultCapacityProviderStrategy: []ecsTypes.CapacityProviderStrategyItem{{
			CapacityProvider: &launchTypeFargateString,
		}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := waitForActiveCluster(ctx, clt, req, cluster); err != nil {
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

	return *cluster.Status == clusterStatusInactive
}

// waitForActiveCluster waits until the Cluster is Active.
// If the Cluster is Provisioning, then it waits at most clusterStatusProvisioningWaitTime (30 seconds) for it to become ready.
func waitForActiveCluster(ctx context.Context, clt DeployServiceClient, req DeployServiceRequest, cluster *ecsTypes.Cluster) error {
	if cluster.Status != nil && *cluster.Status == clusterStatusActive {
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
			Clusters: []string{*req.ClusterName},
		})
		if err != nil {
			return retryutils.PermanentRetryError(trace.Wrap(err))
		}

		if len(describeClustersResponse.Clusters) == 0 {
			return retryutils.PermanentRetryError(trace.NotFound("cluster %q does not exist", *cluster.ClusterName))
		}

		cluster := describeClustersResponse.Clusters[0]
		if cluster.Status == nil {
			return retryutils.PermanentRetryError(trace.Errorf("cluster %q has an unknown (nil) status", *cluster.ClusterName))
		}

		if *cluster.Status == clusterStatusActive {
			return nil
		}

		if *cluster.Status == clusterStatusProvisioning {
			return trace.Errorf("cluster %q is provisioning...", *cluster.ClusterName)
		}

		return retryutils.PermanentRetryError(trace.Errorf("unexpected status %s for ECS Cluster %q", *cluster.ClusterName, *cluster.Status))
	})

	return trace.Wrap(err)
}

// upsertService creates or updates the service.
// If the service exists but its LaunchType is not Fargate, then it gets re-created.
func upsertService(ctx context.Context, clt DeployServiceClient, req DeployServiceRequest, taskARN string) (*ecsTypes.Service, error) {
	describeServiceOut, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Services: []string{*req.ServiceName},
		Cluster:  req.ClusterName,
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
			return nil, trace.Errorf("unknown status for ECS Service %q", *req.ServiceName)
		}

		if *service.Status == serviceStatusDraining {
			return nil, trace.Errorf("ECS Service is draining, please retry in a couple of minutes")
		}

		if *service.Status == serviceStatusActive {
			ownershipTags := req.ResourceCreationTags
			if !ownershipTags.MatchesECSTags(service.Tags) {
				return nil, trace.Errorf("ECS Service %q already exists but is not managed by Teleport. "+
					"Add the following tags to allow Teleport to manage this service: %s", *req.ServiceName, req.ResourceCreationTags)
			}

			// If the LaunchType is the required one, than we can update the current Service.
			// Otherwise we have to delete it.
			if service.LaunchType == ecsTypes.LaunchTypeFargate {
				updateServiceResp, err := clt.UpdateService(ctx, &ecs.UpdateServiceInput{
					Service:        req.ServiceName,
					DesiredCount:   &oneAgent,
					TaskDefinition: &taskARN,
					Cluster:        req.ClusterName,
					NetworkConfiguration: &ecsTypes.NetworkConfiguration{
						AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
							AssignPublicIp: ecsTypes.AssignPublicIpEnabled, // no internet connection otherwise
							Subnets:        req.SubnetIDs,
						},
					},
					ForceNewDeployment: true,
					PropagateTags:      ecsTypes.PropagateTagsService,
				})
				if err != nil {
					return nil, trace.Wrap(err)
				}

				return updateServiceResp.Service, nil
			}
		}

		// The service is deleted if the its status is INACTIVE (aka deleted) or if it's owned by Teleport and has the wrong LaunchType.
		_, err := clt.DeleteService(ctx, &ecs.DeleteServiceInput{
			Service: req.ServiceName,
			Cluster: req.ClusterName,
			Force:   &serviceForceDeletion,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	createServiceOut, err := clt.CreateService(ctx, &ecs.CreateServiceInput{
		ServiceName:    req.ServiceName,
		DesiredCount:   &oneAgent,
		LaunchType:     ecsTypes.LaunchTypeFargate,
		TaskDefinition: &taskARN,
		Cluster:        req.ClusterName,
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				AssignPublicIp: ecsTypes.AssignPublicIpEnabled, // no internet connection otherwise
				Subnets:        req.SubnetIDs,
			},
		},
		Tags:          req.ResourceCreationTags.ForECS(),
		PropagateTags: ecsTypes.PropagateTagsService,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createServiceOut.Service, nil
}
