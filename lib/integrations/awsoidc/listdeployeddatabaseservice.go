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

package awsoidc

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

// ListDeployedDatabaseServicesRequest contains the required fields to list the deployed database services in Amazon ECS.
type ListDeployedDatabaseServicesRequest struct {
	// Region is the AWS Region.
	Region string
	// Integration is the AWS OIDC Integration name
	Integration string
	// TeleportClusterName is the name of the Teleport Cluster.
	// Used to uniquely identify the ECS Cluster in Amazon.
	TeleportClusterName string
	// NextToken is the token to be used to fetch the next page.
	// If empty, the first page is fetched.
	NextToken string
}

func (req *ListDeployedDatabaseServicesRequest) checkAndSetDefaults() error {
	if req.Region == "" {
		return trace.BadParameter("region is required")
	}

	if req.Integration == "" {
		return trace.BadParameter("integration is required")
	}

	if req.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name is required")
	}

	return nil
}

// ListDeployedDatabaseServicesResponse contains a page of Deployed Database Services.
type ListDeployedDatabaseServicesResponse struct {
	// DeployedDatabaseServices contains the page of Deployed Database Services.
	DeployedDatabaseServices []DeployedDatabaseService `json:"deployedDatabaseServices"`

	// NextToken is used for pagination.
	// If non-empty, it can be used to request the next page.
	NextToken string `json:"nextToken"`
}

// DeployedDatabaseService contains a database service that was deployed to Amazon ECS.
type DeployedDatabaseService struct {
	// Name is the ECS Service name.
	Name string
	// ServiceDashboardURL is the Amazon Web Console URL for this ECS Service.
	ServiceDashboardURL string
	// ContainerEntryPoint is the entry point for the container 0 that is running in the ECS Task.
	ContainerEntryPoint []string
	// ContainerCommand is the list of arguments that are passed into the ContainerEntryPoint.
	ContainerCommand []string
}

// ListDeployedDatabaseServicesClient describes the required methods to list AWS VPCs.
type ListDeployedDatabaseServicesClient interface {
	// ListServices returns a list of services.
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	// DescribeServices returns ECS Services details.
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	// DescribeTaskDefinition returns an ECS Task Definition.
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
}

type defaultListDeployedDatabaseServicesClient struct {
	*ecs.Client
}

// NewListDeployedDatabaseServicesClient creates a new ListDeployedDatabaseServicesClient using an AWSClientRequest.
func NewListDeployedDatabaseServicesClient(ctx context.Context, req *AWSClientRequest) (ListDeployedDatabaseServicesClient, error) {
	ecsClient, err := newECSClient(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultListDeployedDatabaseServicesClient{
		Client: ecsClient,
	}, nil
}

// ListDeployedDatabaseServices calls the following AWS API:
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListServices.html
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTaskDefinition.html
// It returns a list of ECS Services running Teleport Database Service and an optional NextToken that can be used to fetch the next page.
func ListDeployedDatabaseServices(ctx context.Context, clt ListDeployedDatabaseServicesClient, req ListDeployedDatabaseServicesRequest) (*ListDeployedDatabaseServicesResponse, error) {
	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName := normalizeECSClusterName(req.TeleportClusterName)

	log := slog.With(
		"integration", req.Integration,
		"aws_region", req.Region,
		"ecs_cluster", clusterName,
	)

	// Do not increase this value because ecs.DescribeServices only allows up to 10 services per API call.
	maxServicesPerPage := aws.Int32(10)
	listServicesInput := &ecs.ListServicesInput{
		Cluster:    &clusterName,
		MaxResults: maxServicesPerPage,
		LaunchType: ecstypes.LaunchTypeFargate,
	}
	if req.NextToken != "" {
		listServicesInput.NextToken = &req.NextToken
	}

	listServicesOutput, err := clt.ListServices(ctx, listServicesInput)
	if err != nil {
		convertedError := awslib.ConvertRequestFailureError(err)
		if trace.IsNotFound(convertedError) {
			return &ListDeployedDatabaseServicesResponse{}, nil
		}

		return nil, trace.Wrap(convertedError)
	}

	describeServicesOutput, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Services: listServicesOutput.ServiceArns,
		Include:  []ecstypes.ServiceField{ecstypes.ServiceFieldTags},
		Cluster:  &clusterName,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ownershipTags := defaultResourceCreationTags(req.TeleportClusterName, req.Integration)

	deployedDatabaseServices := []DeployedDatabaseService{}
	for _, ecsService := range describeServicesOutput.Services {
		log := log.With("ecs_service", aws.ToString(ecsService.ServiceName))
		if !ownershipTags.MatchesECSTags(ecsService.Tags) {
			log.WarnContext(ctx, "Missing ownership tags in ECS Service, skipping")
			continue
		}

		taskDefinitionOut, err := clt.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: ecsService.TaskDefinition,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if len(taskDefinitionOut.TaskDefinition.ContainerDefinitions) == 0 {
			log.WarnContext(ctx, "Task has no containers defined, skipping",
				"ecs_task_family", aws.ToString(taskDefinitionOut.TaskDefinition.Family),
				"ecs_task_revision", taskDefinitionOut.TaskDefinition.Revision,
			)
			continue
		}

		entryPoint := taskDefinitionOut.TaskDefinition.ContainerDefinitions[0].EntryPoint
		command := taskDefinitionOut.TaskDefinition.ContainerDefinitions[0].Command

		deployedDatabaseServices = append(deployedDatabaseServices, DeployedDatabaseService{
			Name:                aws.ToString(ecsService.ServiceName),
			ServiceDashboardURL: serviceDashboardURL(req.Region, clusterName, aws.ToString(ecsService.ServiceName)),
			ContainerEntryPoint: entryPoint,
			ContainerCommand:    command,
		})
	}

	return &ListDeployedDatabaseServicesResponse{
		DeployedDatabaseServices: deployedDatabaseServices,
		NextToken:                aws.ToString(listServicesOutput.NextToken),
	}, nil
}
