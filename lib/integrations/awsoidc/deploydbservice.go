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
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
)

var (
	// launcTypeFargateString is the FARGATE LaunchType converted into a string.
	launcTypeFargateString = string(ecsTypes.LaunchTypeFargate)
	// requiredCapacityProviders contains the FARGATE type which is required to deploy a DatabaseService.
	requiredCapacityProviders = []string{launcTypeFargateString}

	// Ensure Cpu and Memory use one of the allowed combinations:
	// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters.html
	taskCPU = "256"
	taskMem = "512"

	// taskAgentContainerName is the name of the container to run within the Task.
	// Each task supports multiple containers, but, currently, there's only one being used.
	// This is also going to be the NodeName of the teleport instance.
	taskAgentContainerName = "ecs-discovery-dbservice"

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

	// serviceStatusActive is the string representing an ACTIVE ECS Service.
	serviceStatusActive = "ACTIVE"
	// serviceStatusDraining is the string representing an DRAINING ECS Service.
	serviceStatusDraining = "DRAINING"
)

// DeployDBServiceRequest contains the required fields to deploy a Database and a Discovery Service.
type DeployDBServiceRequest struct {
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

	// TaskName is the ECS Task Definition's name.
	TaskName *string

	// TaskRoleARN is the AWS Role's ARN used within the Task execution.
	// Ensure the AWS Client has `iam:PassRole` for this Role's ARN.
	TaskRoleARN string

	// ProxyServerHostPort is the Teleport Proxy address as used for `teleport.yaml`
	// Eg proxy.example.com:443
	ProxyServerHostPort string

	// TeleportVersion is the Teleport version to be used by the container.
	// Eg 13.0.3
	TeleportVersion string

	// DiscoveryGroupName is the DiscoveryGroup to be used by the `discovery_service`.
	DiscoveryGroupName string

	// TeleportClusterName is the Teleport Cluster Name, used to create default names for Cluster, Service and Task.
	TeleportClusterName string
}

// CheckAndSetDefaults checks if the required fields are present.
func (r *DeployDBServiceRequest) CheckAndSetDefaults() error {
	if r.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name is required")
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
		clusterName := fmt.Sprintf("%s-teleport", r.TeleportClusterName)
		r.ClusterName = &clusterName
	}

	if r.ServiceName == nil || *r.ServiceName == "" {
		serviceName := fmt.Sprintf("%s-teleport-database-service", r.TeleportClusterName)
		r.ServiceName = &serviceName
	}

	if r.TaskName == nil || *r.TaskName == "" {
		taskName := fmt.Sprintf("%s-teleport-database-service", r.TeleportClusterName)
		r.TaskName = &taskName
	}

	if r.DiscoveryGroupName == "" {
		return trace.BadParameter("discovery group name is required")
	}

	if r.ProxyServerHostPort == "" {
		return trace.BadParameter("proxy server is required (format host:port)")
	}

	if r.TeleportVersion == "" {
		return trace.BadParameter("teleport version is required (eg, 13.0.2)")
	}

	return nil
}

// DeployDBServiceResponse contains the ARNs of the Amazon resources used to deploy the Database and Discovery Services.
type DeployDBServiceResponse struct {
	// ClusterARN is the Amazon ECS Cluster ARN where the task was started.
	ClusterARN string

	// ServiceARN is the Amazon ECS Cluster Service ARN created to run the task.
	ServiceARN string

	// TaskDefinitionARN is the Amazon ECS Task Definition ARN created to run the Database and Discovery services.
	TaskDefinitionARN string
}

// DeployDBServiceClient describes the required methods to Deploy a Database and Discovery Services.
type DeployDBServiceClient interface {
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

// NewDeployDBServiceClient creates a new ListDatabasesClient using a AWSClientRequest.
func NewDeployDBServiceClient(ctx context.Context, clientReq *AWSClientRequest) (DeployDBServiceClient, error) {
	return newECSClient(ctx, clientReq)
}

// DeployDBService calls Amazon ECS APIs to deploy two services:
// - Database Service
// - Discovery Service
//
// # Pre-requirement: Set up iam-token for auto join
//
// Both services connect via `iam-token`, so ensure your cluster has the following token:
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
// The req.TaskRoleARN Role must have the following policy:
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
// # Discovery and Database Service
//
// The Database Service only matches resources onboarded by the Discovery Service.
// This is achieved by using the `discovery_group` to link the two:
//   - when Discovery Service has a `discovery_group` X, it will create all the Databases with a label `teleport.internal/discovery-group-name: X`
//   - the Database Service resource matchers only watches for resources with the following label `teleport.internal/discovery-group-name: X`
func DeployDBService(ctx context.Context, clt DeployDBServiceClient, req DeployDBServiceRequest) (*DeployDBServiceResponse, error) {
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

	cluster, err := upsertCluster(ctx, clt, *req.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	service, err := upsertService(ctx, clt, req, taskDefinitionARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &DeployDBServiceResponse{
		ClusterARN:        *cluster.ClusterArn,
		ServiceARN:        *service.ServiceArn,
		TaskDefinitionARN: taskDefinitionARN,
	}, nil
}

// generateTeleportConfigString creates a teleport.yaml configuration
func generateTeleportConfigString(req DeployDBServiceRequest) (string, error) {
	serviceConfig, err := config.MakeSampleFileConfig(config.SampleFlags{
		Version:      defaults.TeleportConfigVersionV3,
		ProxyAddress: req.ProxyServerHostPort,
	})
	if err != nil {
		return "", err
	}

	// The implicit value is the current host (Proxy Host).
	// Setting this to the agent name adds some correlation between host name and service name.
	serviceConfig.NodeName = taskAgentContainerName

	// Use IAM Token join method to enroll into the Cluster.
	// iam-token must have the following TokenRule:
	/*
		types.TokenRule{
			AWSAccount: "<account-id>",
			AWSARN:     "arn:aws:sts::<account-id>:assumed-role/<taskRoleARN>/*",
		}
	*/
	serviceConfig.JoinParams = config.JoinParams{
		TokenName: string(types.JoinMethodIAM) + "-token",
		Method:    types.JoinMethodIAM,
	}

	// Disable default services
	serviceConfig.Auth.EnabledFlag = "no"
	serviceConfig.Proxy.EnabledFlag = "no"
	serviceConfig.SSH.EnabledFlag = "no"

	// Enable Discovery Service with a specific Discovery Group.
	serviceConfig.Discovery.EnabledFlag = "yes"
	serviceConfig.Discovery.DiscoveryGroup = req.DiscoveryGroupName
	// We need at least one matcher so the Discovery Service starts.
	// TODO(marco): remove this requirement to support only dynamic matchers (RFD 125)
	serviceConfig.Discovery.AWSMatchers = []config.AWSMatcher{{
		Types:   []string{"rds"},
		Regions: []string{req.Region},
	}}

	// Ensure the DatabaseService only proxies the Databases discovered by the Discovery Service.
	//
	// When the DiscoveryService has a DiscoveryGroup, it stamps all the added resources with the following label:
	// > teleport.internal/discovery-group-name: <discovery-group>
	// So, adding this as label matcher, ensures only those Databases are proxied.
	serviceConfig.Databases.Service.EnabledFlag = "yes"
	serviceConfig.Databases.ResourceMatchers = []config.ResourceMatcher{{
		Labels: map[string]utils.Strings{
			types.TeleportInternalDiscoveryGroupName: []string{req.DiscoveryGroupName},
		},
	}}

	teleportConfigYAMLBytes, err := yaml.Marshal(serviceConfig)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// This Config is meant to be passed as argument to `teleport start`
	// Eg, `teleport start --config-string <X>`
	teleportConfigString := base64.StdEncoding.EncodeToString(teleportConfigYAMLBytes)

	return teleportConfigString, nil
}

// upsertTask ensures a TaskDefinition with TaskName exists
func upsertTask(ctx context.Context, clt DeployDBServiceClient, req DeployDBServiceRequest, configB64 string) (*ecsTypes.TaskDefinition, error) {
	taskAgentContainerImage := fmt.Sprintf(teleportContainerImageFmt, req.TeleportVersion)

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
func upsertCluster(ctx context.Context, clt DeployDBServiceClient, clusterName string) (*ecsTypes.Cluster, error) {
	var cluster *ecsTypes.Cluster

	describeClustersResponse, err := clt.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{clusterName},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inactiveClusterExists := false
	if len(describeClustersResponse.Clusters) > 0 {
		cluster = &describeClustersResponse.Clusters[0]

		// If the cluster was recently removed, it might still be listed but with "INACTIVE" status.
		// Calling CreateCluster activates it.
		// From AWS Docs:
		// > INACTIVE The cluster has been deleted. Clusters with an INACTIVE status may remain discoverable in your account for a period of time.
		if *cluster.Status == clusterStatusInactive {
			inactiveClusterExists = true

		} else if !slices.Contains(cluster.CapacityProviders, launcTypeFargateString) {
			// Ensure the required capacity provider (Fargate) is available.
			putClusterCPResp, err := clt.PutClusterCapacityProviders(ctx, &ecs.PutClusterCapacityProvidersInput{
				Cluster:           &clusterName,
				CapacityProviders: requiredCapacityProviders,
				DefaultCapacityProviderStrategy: []ecsTypes.CapacityProviderStrategyItem{{
					CapacityProvider: &launcTypeFargateString,
				}},
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			cluster = putClusterCPResp.Cluster
		}
	}

	// No cluster exists or the cluster is inactive.
	if len(describeClustersResponse.Clusters) == 0 || inactiveClusterExists {
		createClusterResp, err := clt.CreateCluster(ctx, &ecs.CreateClusterInput{
			ClusterName:       &clusterName,
			CapacityProviders: requiredCapacityProviders,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cluster = createClusterResp.Cluster
	}

	// Anything other than ACTIVE, should throw an error (usually retryable)
	// Possible status: INACTIVE, PROVISIONING, DEPROVISIONING, FAILED
	if cluster.Status != nil && *cluster.Status != clusterStatusActive {
		return nil, trace.Errorf("cluster %q has an invalid status (%s), try again", clusterName, *cluster.Status)
	}

	return cluster, nil
}

// upsertService creates or updates the service.
// If the service exists but its LaunchType is not Fargate, then it gets re-created.
func upsertService(ctx context.Context, clt DeployDBServiceClient, req DeployDBServiceRequest, taskARN string) (*ecsTypes.Service, error) {
	describeServiceOut, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Services: []string{*req.ServiceName},
		Cluster:  req.ClusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Service already exists.
	if len(describeServiceOut.Services) > 0 {
		// The service already exists.
		service := &describeServiceOut.Services[0]

		if service.Status == nil {
			return nil, trace.Errorf("unknown status for ECS Service %q", *req.ServiceName)
		}

		if *service.Status == serviceStatusDraining {
			return nil, trace.Errorf("ECS Service is shutting down, please retry in a couple of minutes")
		}

		// Updating the service to use the new image if:
		// - launch type is FARGATE
		// - status is ACTIVE
		// Otherwise, the service is deleted and created again
		if service.LaunchType == ecsTypes.LaunchTypeFargate && *service.Status == serviceStatusActive {
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
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return updateServiceResp.Service, nil
		}

		// We can't update the LaunchType or the Status is INACTIVE, so the only solution is to delete and create the Service again.

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
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createServiceOut.Service, nil
}
