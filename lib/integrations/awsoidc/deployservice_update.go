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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// waitDuration specifies the amount of time to wait for a service to become healthy after an update.
const waitDuration = time.Minute * 5

// UpdateServiceRequest contains the required fields to update a Teleport Service.
type UpdateServiceRequest struct {
	// TeleportClusterName specifies the teleport cluster name
	TeleportClusterName string
	// TeleportVersionTag specifies the desired teleport version in the format "13.4.0"
	TeleportVersionTag string
	// OwnershipTags specifies ownership tags
	OwnershipTags AWSTags
}

// CheckAndSetDefaults checks and sets default config values.
func (req *UpdateServiceRequest) CheckAndSetDefaults() error {
	if req.TeleportClusterName == "" {
		return trace.BadParameter("teleport cluster name required")
	}

	if req.TeleportVersionTag == "" {
		return trace.BadParameter("teleport version tag required")
	}

	if req.OwnershipTags == nil {
		return trace.BadParameter("ownership tags required")
	}

	return nil
}

// UpdateDeployService updates the AWS OIDC deploy service with the specified version tag.
func UpdateDeployService(ctx context.Context, clt DeployServiceClient, req UpdateServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	teleportImage := getDistrolessTeleportImage(req.TeleportVersionTag)
	service, err := getManagedService(ctx, clt, req.TeleportClusterName, req.OwnershipTags)
	if err != nil {
		return trace.Wrap(err)
	}

	taskDefinition, err := getManagedTaskDefinition(ctx, clt, aws.ToString(service.TaskDefinition), req.OwnershipTags)
	if err != nil {
		return trace.Wrap(err)
	}

	currentTeleportImage, err := getTaskDefinitionTeleportImage(taskDefinition)
	if err != nil {
		return trace.Wrap(err)
	}

	// There is no need to update the ecs service if the ecs service is already
	// running the latest stable version of teleport.
	if currentTeleportImage == teleportImage {
		return nil
	}

	registerTaskDefinitionIn, err := generateTaskDefinitionWithImage(taskDefinition, teleportImage, req.OwnershipTags.ToECSTags())
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
		// If update failed, then rollback task definition.
		// The update will be re-attempted during the next interval if it is still
		// within the upgrade window or the critical upgrade flag is still enabled.
		_, rollbackErr := clt.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
			TaskDefinition: registerTaskDefinitionOut.TaskDefinition.TaskDefinitionArn,
		})
		if rollbackErr != nil {
			return trace.NewAggregate(err, trace.Wrap(rollbackErr, "failed to rollback task definition"))
		}
		return trace.Wrap(err)
	}

	// Attempt to deregister previous task definition but ignore error on failure
	_, err = clt.DeregisterTaskDefinition(ctx, &ecs.DeregisterTaskDefinitionInput{
		TaskDefinition: taskDefinition.TaskDefinitionArn,
	})
	if err != nil {
		logrus.WithError(err).Warning("Failed to deregister task definition.")
	}

	return nil
}

func getManagedService(ctx context.Context, clt DeployServiceClient, teleportClusterName string, ownershipTags AWSTags) (*ecsTypes.Service, error) {
	var ecsServiceNames []string
	for _, deploymentMode := range DeploymentModes {
		ecsServiceNames = append(ecsServiceNames, normalizeECSServiceName(teleportClusterName, deploymentMode))
	}

	describeServicesOut, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(normalizeECSClusterName(teleportClusterName)),
		Services: ecsServiceNames,
		Include:  []ecsTypes.ServiceField{ecsTypes.ServiceFieldTags},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(describeServicesOut.Services) == 0 {
		return nil, trace.NotFound("services %v not found", ecsServiceNames)
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
		return nil, trace.NewAggregate(err, trace.Wrap(rollbackErr, "failed to rollback service"))
	}

	rollbackErr = serviceStableWaiter.Wait(ctx, &ecs.DescribeServicesInput{
		Services: []string{aws.ToString(rollbackServiceOut.Service.ServiceName)},
		Cluster:  updateServiceOut.Service.ClusterArn,
	}, waitDuration)
	if rollbackErr != nil {
		return nil, trace.NewAggregate(err, trace.Wrap(rollbackErr, "failed to rollback service"))
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
