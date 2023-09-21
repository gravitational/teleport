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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/modules"
)

// UpdateDeployServiceAgents updates the deploy service agents with the specified teleportVersionTag.
func UpdateDeployServiceAgents(ctx context.Context, clt DeployServiceClient, teleportClusterName, teleportVersionTag string, ownershipTags AWSTags) error {
	teleportFlavor := teleportOSS
	if modules.GetModules().BuildType() == modules.BuildEnterprise {
		teleportFlavor = teleportEnt
	}
	teleportImage := fmt.Sprintf("public.ecr.aws/gravitational/%s-distroless:%s", teleportFlavor, teleportVersionTag)

	if err := updateDeployServiceAgent(ctx, clt, teleportClusterName, teleportImage, ownershipTags); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func updateDeployServiceAgent(ctx context.Context, clt DeployServiceClient, teleportClusterName, teleportImage string, ownershipTags AWSTags) error {
	service, err := getManagedService(ctx, clt, teleportClusterName, ownershipTags)
	if err != nil {
		return trace.Wrap(err)
	}

	taskDefinition, err := getManagedTaskDefinition(ctx, clt, aws.ToString(service.TaskDefinition), ownershipTags)
	if err != nil {
		return trace.Wrap(err)
	}

	currentTeleportImage, err := getTaskDefinitionTeleportImage(taskDefinition)
	if err != nil {
		return trace.Wrap(err)
	}

	if currentTeleportImage == teleportImage {
		return nil
	}

	registerTaskDefinitionIn, err := generateTaskDefinitionWithImage(taskDefinition, teleportImage, ownershipTags.ToECSTags())
	if err != nil {
		return trace.Wrap(err)
	}

	registerTaskDefinitionOut, err := clt.RegisterTaskDefinition(ctx, registerTaskDefinitionIn)
	if err != nil {
		return trace.Wrap(err)
	}

	// Update service with new task definition
	_, err = updateServiceOrRollback(ctx, clt, service, registerTaskDefinitionOut.TaskDefinition)
	if err != nil {
		// If update failed, then rollback task definition
		_, rollbackErr := clt.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: registerTaskDefinitionOut.TaskDefinition.TaskDefinitionArn,
		})
		if rollbackErr != nil {
			return trace.Wrap(err, "failed to rollback task definition: %v", rollbackErr)
		}
		return trace.Wrap(err)
	}

	// Attempt to deregister previous task definition but ignore error on failure
	clt.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: taskDefinition.TaskDefinitionArn,
	})
	return nil
}

// waitDuration specifies the amount of time to wait for a service to become healthy after an update.
const waitDuration = time.Minute * 5

func getManagedService(ctx context.Context, clt DeployServiceClient, teleportClusterName string, ownershipTags AWSTags) (*ecsTypes.Service, error) {
	ecsClusterName := fmt.Sprintf("%s-teleport", normalizeECSResourceName(teleportClusterName))

	var ecsServiceNames []string
	for _, deploymentMode := range DeploymentModes {
		ecsServiceNames = append(ecsServiceNames, fmt.Sprintf("%s-%s", ecsClusterName, deploymentMode))
	}

	describeServicesOut, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(ecsClusterName),
		Services: ecsServiceNames,
		Include:  []ecsTypes.ServiceField{ecsTypes.ServiceFieldTags},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(describeServicesOut.Services) != 1 {
		return nil, trace.BadParameter("expected 1 service, but got %d", len(describeServicesOut.Services))
	}
	service := describeServicesOut.Services[0]

	if !ownershipTags.MatchesECSTags(service.Tags) {
		return nil, trace.Errorf("ECS Service %q already exists but is not managed by Teleport. "+
			"Add the following tags to allow Teleport to manage this service: %s", aws.ToString(service.ServiceName), ownershipTags)
	}
	// If the LaunchType is the required one, than we can update the current Service.
	// Otherwise we have to delete it.
	if service.LaunchType != ecsTypes.LaunchTypeFargate {
		return nil, trace.Errorf("ECS Service %q already exists but has an invalid LaunchType %q. Delete the Service and try again.", aws.ToString(service.ServiceName), service.LaunchType)
	}

	return &service, nil
}

func getManagedTaskDefinition(ctx context.Context, clt DeployServiceClient, taskDefinitionName string, ownershipTags AWSTags) (*ecsTypes.TaskDefinition, error) {
	describeTaskDefinitionOut, err := clt.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDefinitionName),
		Include:        []ecsTypes.TaskDefinitionField{ecsTypes.TaskDefinitionFieldTags},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !ownershipTags.MatchesECSTags(describeTaskDefinitionOut.Tags) {
		return nil, trace.Errorf("ECS Task Definition %q already exists but is not managed by Teleport. "+
			"Add the following tags to allow Teleport to manage this task definition: %s", taskDefinitionName, ownershipTags)
	}
	return describeTaskDefinitionOut.TaskDefinition, nil
}

func getTaskDefinitionTeleportImage(taskDefinition *ecsTypes.TaskDefinition) (string, error) {
	if len(taskDefinition.ContainerDefinitions) != 1 {
		return "", trace.BadParameter("expected 1 task container definition, but got %d", len(taskDefinition.ContainerDefinitions))
	}
	return aws.ToString(taskDefinition.ContainerDefinitions[0].Image), nil
}

// updateServiceOrRollback attempts to update the service with the specified task definition.
// The service will be rolled back if the service fails to become healthy.
func updateServiceOrRollback(ctx context.Context, clt DeployServiceClient, service *ecsTypes.Service, taskDefinition *ecsTypes.TaskDefinition) (*ecsTypes.Service, error) {
	// Update service with new task definition
	updateServiceOut, err := clt.UpdateService(ctx, generateServiceWithTaskDefinition(service, aws.ToString(taskDefinition.TaskDefinitionArn)))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceStableWaiter := ecs.NewServicesStableWaiter(clt)
	err = serviceStableWaiter.Wait(ctx, &ecs.DescribeServicesInput{
		Services: []string{aws.ToString(updateServiceOut.Service.ServiceName)},
		Cluster:  updateServiceOut.Service.ClusterArn,
	}, waitDuration)
	if err == nil {
		return updateServiceOut.Service, nil
	}

	// If the service fails to reach a stable state within the allowed wait time,
	// then rollback service with previous task definition
	rollbackServiceOut, rollbackErr := clt.UpdateService(ctx, generateServiceWithTaskDefinition(service, aws.ToString(service.TaskDefinition)))
	if rollbackErr != nil {
		return nil, trace.Wrap(err, "failed to rollback service: %v", err)
	}

	rollbackErr = serviceStableWaiter.Wait(ctx, &ecs.DescribeServicesInput{
		Services: []string{aws.ToString(rollbackServiceOut.Service.ServiceName)},
		Cluster:  updateServiceOut.Service.ClusterArn,
	}, waitDuration)
	if rollbackErr != nil {
		return nil, trace.Wrap(err, "failed to rollback service: %v", err)
	}

	return nil, trace.Wrap(err)
}

// generateTaskDefinitionWithImage returns new register task definition input with the desired teleport image
func generateTaskDefinitionWithImage(taskDefinition *ecsTypes.TaskDefinition, teleportImage string, tags []ecsTypes.Tag) (*ecs.RegisterTaskDefinitionInput, error) {
	if len(taskDefinition.ContainerDefinitions) != 1 {
		return nil, trace.BadParameter("expected 1 task container definition, but got %d", len(taskDefinition.ContainerDefinitions))
	}

	// Copy container definition and replace the teleport image with desired version
	newContainerDefinition := new(ecsTypes.ContainerDefinition)
	awsutil.Copy(newContainerDefinition, &taskDefinition.ContainerDefinitions[0])
	newContainerDefinition.Image = aws.String(teleportImage)

	// Copy task definition and replace container definitions
	registerTaskDefinitionIn := new(ecs.RegisterTaskDefinitionInput)
	awsutil.Copy(registerTaskDefinitionIn, taskDefinition)
	registerTaskDefinitionIn.ContainerDefinitions = []ecsTypes.ContainerDefinition{*newContainerDefinition}
	registerTaskDefinitionIn.Tags = tags

	return registerTaskDefinitionIn, nil
}

// generateServiceWithTaskDefinition returns new update service input with the desired task definition
func generateServiceWithTaskDefinition(service *ecsTypes.Service, taskDefinitionName string) *ecs.UpdateServiceInput {
	updateServiceIn := new(ecs.UpdateServiceInput)
	awsutil.Copy(updateServiceIn, service)
	updateServiceIn.Service = service.ServiceName
	updateServiceIn.Cluster = service.ClusterArn
	updateServiceIn.TaskDefinition = aws.String(taskDefinitionName)
	return updateServiceIn
}
