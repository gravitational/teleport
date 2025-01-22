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
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
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
	OwnershipTags tags.AWSTags
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

// UpdateDeployService updates all the AWS OIDC deployed services with the specified version tag.
func UpdateDeployService(ctx context.Context, clt DeployServiceClient, log *slog.Logger, req UpdateServiceRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	teleportImage, err := getDistrolessTeleportImage(req.TeleportVersionTag)
	if err != nil {
		return trace.Wrap(err)
	}
	services, err := getManagedServices(ctx, clt, log, req.TeleportClusterName, req.OwnershipTags)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, ecsService := range services {
		logService := log.With(
			"ecs_service_arn", aws.ToString(ecsService.ServiceArn),
			"teleport_image", teleportImage,
		)
		if err := updateServiceContainerImage(ctx, clt, logService, &ecsService, teleportImage, req.OwnershipTags); err != nil {
			logService.WarnContext(ctx, "Failed to upgrade ECS Service.", "error", err)
			continue
		}
	}

	return nil
}

func updateServiceContainerImage(ctx context.Context, clt DeployServiceClient, log *slog.Logger, service *ecsTypes.Service, teleportImage string, ownershipTags tags.AWSTags) error {
	taskDefinition, err := getManagedTaskDefinition(ctx, clt, aws.ToString(service.TaskDefinition), ownershipTags)
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
		log.InfoContext(ctx, "ECS service version already matches, not updating")
		return nil
	}

	registerTaskDefinitionIn, err := generateTaskDefinitionWithImage(taskDefinition, teleportImage, ownershipTags.ToECSTags())
	if err != nil {
		return trace.Wrap(err)
	}

	// Ensure that the upgrader variables are set.
	// These will ensure that the instance reports Teleport upgrader metrics.
	if err := ensureUpgraderEnvironmentVariables(registerTaskDefinitionIn); err != nil {
		return trace.Wrap(err)
	}

	registerTaskDefinitionOut, err := clt.RegisterTaskDefinition(ctx, registerTaskDefinitionIn)
	if err != nil {
		return trace.Wrap(err)
	}
	newTaskDefinitionARN := registerTaskDefinitionOut.TaskDefinition.TaskDefinitionArn
	oldTaskDefinitionARN := aws.ToString(service.TaskDefinition)

	// Update service with new task definition
	targetNewServiceVersion, err := generateServiceWithTaskDefinition(service, aws.ToString(newTaskDefinitionARN))
	if err != nil {
		return trace.Wrap(err)
	}
	serviceNewVersion, err := clt.UpdateService(ctx, targetNewServiceVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	// Wait for Service to become stable, or rollback to the previous TaskDefinition.
	go waitServiceStableOrRollback(ctx, clt, log, serviceNewVersion.Service, oldTaskDefinitionARN)

	log.InfoContext(ctx, "Successfully upgraded ECS Service.")

	return nil
}

func getAllServiceNamesForCluster(ctx context.Context, clt DeployServiceClient, clusterName *string) ([]string, error) {
	ret := make([]string, 0)

	nextToken := ""
	for {
		resp, err := clt.ListServices(ctx, &ecs.ListServicesInput{
			Cluster:   clusterName,
			NextToken: aws.String(nextToken),
		})
		if err != nil {
			return nil, awslib.ConvertIAMError(err)
		}

		ret = append(ret, resp.ServiceArns...)

		nextToken = aws.ToString(resp.NextToken)
		if nextToken == "" {
			break
		}
	}
	return ret, nil
}

func getManagedServices(ctx context.Context, clt DeployServiceClient, log *slog.Logger, teleportClusterName string, ownershipTags tags.AWSTags) ([]ecsTypes.Service, error) {
	// The Cluster name is created using the Teleport Cluster Name.
	// Check the DeployDatabaseServiceRequest.CheckAndSetDefaults
	// and DeployServiceRequest.CheckAndSetDefaults.
	wellKnownClusterName := aws.String(normalizeECSClusterName(teleportClusterName))

	ecsServiceNames, err := getAllServiceNamesForCluster(ctx, clt, wellKnownClusterName)
	if err != nil {
		if !trace.IsAccessDenied(err) {
			return nil, trace.Wrap(err)
		}

		// Previous versions of the DeployService only deployed a single ECS Service, based on the DatabaseServiceDeploymentMode.
		// During the Discover Wizard flow, users were asked to run a script that added the required permissions, but ecs:ListServices was not initially included.
		// For those situations, fallback to using the only ECS Service that was deployed.
		ecsServiceNameLegacy := normalizeECSServiceName(teleportClusterName, DatabaseServiceDeploymentMode)
		ecsServiceNames = []string{ecsServiceNameLegacy}
	}

	ecsServices := make([]ecsTypes.Service, 0, len(ecsServiceNames))

	// According to AWS API docs, a maximum of 10 Services can be queried at the same time when using the ecs:DescribeServices operation.
	batchSize := 10
	for batchStart := 0; batchStart < len(ecsServiceNames); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(ecsServiceNames) {
			batchEnd = len(ecsServiceNames)
		}

		describeServicesOut, err := clt.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  wellKnownClusterName,
			Services: ecsServiceNames[batchStart:batchEnd],
			Include:  []ecsTypes.ServiceField{ecsTypes.ServiceFieldTags},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Filter out Services without Ownership tags or an invalid LaunchType.
		for _, s := range describeServicesOut.Services {
			log := log.With("ecs_service", aws.ToString(s.ServiceArn))
			if !ownershipTags.MatchesECSTags(s.Tags) {
				log.WarnContext(ctx, "ECS Service exists but is not managed by Teleport. Add the tags to allow Teleport to manage this service", "tags", ownershipTags)
				continue
			}
			// If the LaunchType is the required one, than we can update the current Service.
			// Otherwise we have to delete it.
			if s.LaunchType != ecsTypes.LaunchTypeFargate {
				log.WarnContext(ctx, "ECS Service already exists but has an invalid LaunchType. Delete the Service and try again.", "launch_type", s.LaunchType)
				continue
			}
			ecsServices = append(ecsServices, s)
		}
	}

	return ecsServices, nil
}

func getManagedTaskDefinition(ctx context.Context, clt DeployServiceClient, taskDefinitionName string, ownershipTags tags.AWSTags) (*ecsTypes.TaskDefinition, error) {
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

// waitServiceStableOrRollback waits for the ECS Service to be stable, and if it takes longer than 5 minutes, it restarts it with its old task definition.
func waitServiceStableOrRollback(ctx context.Context, clt DeployServiceClient, log *slog.Logger, service *ecsTypes.Service, oldTaskDefinitionARN string) {
	log = log.With(
		"ecs_service", aws.ToString(service.ServiceArn),
		"task_definition", aws.ToString(service.TaskDefinition),
		"old_task_definition", oldTaskDefinitionARN,
	)

	log.DebugContext(ctx, "Waiting for ECS Service to become stable")
	serviceStableWaiter := ecs.NewServicesStableWaiter(clt)
	waitErr := serviceStableWaiter.Wait(ctx, &ecs.DescribeServicesInput{
		Services: []string{aws.ToString(service.ServiceName)},
		Cluster:  service.ClusterArn,
	}, waitDuration)
	if waitErr == nil {
		log.DebugContext(ctx, "ECS Service is stable")
		return
	}

	log.WarnContext(ctx, "ECS Service is not stable, restarting the service with its previous TaskDefinition", "error", waitErr)

	rollbackServiceInput, err := generateServiceWithTaskDefinition(service, oldTaskDefinitionARN)
	if err != nil {
		log.WarnContext(ctx, "Failed to generate UpdateServiceInput targeting its previous version", "error", err)
		return
	}

	_, rollbackErr := clt.UpdateService(ctx, rollbackServiceInput)
	if rollbackErr != nil {
		log.WarnContext(ctx, "Failed to update ECS Service with its previous version", "error", rollbackErr)
	}
}

// generateTaskDefinitionWithImage returns new register task definition input with the desired teleport image
func generateTaskDefinitionWithImage(taskDefinition *ecsTypes.TaskDefinition, teleportImage string, tags []ecsTypes.Tag) (*ecs.RegisterTaskDefinitionInput, error) {
	if len(taskDefinition.ContainerDefinitions) != 1 {
		return nil, trace.BadParameter("expected 1 task container definition, but got %d", len(taskDefinition.ContainerDefinitions))
	}

	// Copy container definition and replace the teleport image with desired version
	newContainerDefinition := &taskDefinition.ContainerDefinitions[0]
	newContainerDefinition.Image = aws.String(teleportImage)

	// Copy task definition and replace container definitions
	//
	// AWS SDK Go v1 had a `awsutil.Copy` which we could use to copy all the values from one struct into another one.
	// AWS SDK Go v2 does not expose such method: https://github.com/aws/aws-sdk-go-v2/blob/9005edfbb1194c8f340b9bb7288698b58fc7274a/internal/awsutil/copy.go#L15
	// To solve this, the TaskDefinition is json marshaled and unmarshalled into a RegisterTaskDefinitionInput.
	registerTaskDefinitionIn := &ecs.RegisterTaskDefinitionInput{}
	taskDefinitionJSON, err := json.Marshal(taskDefinition)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := json.Unmarshal(taskDefinitionJSON, registerTaskDefinitionIn); err != nil {
		return nil, trace.Wrap(err)
	}
	registerTaskDefinitionIn.ContainerDefinitions = []ecsTypes.ContainerDefinition{*newContainerDefinition}
	registerTaskDefinitionIn.Tags = tags

	return registerTaskDefinitionIn, nil
}

// ensureUpgraderEnvironmentVariables modifies the taskDefinition and ensures that
// the upgrader specific environment variables are set.
func ensureUpgraderEnvironmentVariables(taskDefinition *ecs.RegisterTaskDefinitionInput) error {
	containerDefinitions := []ecsTypes.ContainerDefinition{}
	for _, containerDefinition := range taskDefinition.ContainerDefinitions {
		environment := []ecsTypes.KeyValuePair{}

		// Copy non-upgrader specific environemt variables as is
		for _, env := range containerDefinition.Environment {
			if aws.ToString(env.Name) == automaticupgrades.EnvUpgrader ||
				aws.ToString(env.Name) == automaticupgrades.EnvUpgraderVersion {
				continue
			}
			environment = append(environment, env)
		}

		// Ensure ugprader specific environment variables are set
		environment = append(environment,
			ecsTypes.KeyValuePair{
				Name:  aws.String(automaticupgrades.EnvUpgraderVersion),
				Value: aws.String(teleport.Version),
			},
		)
		containerDefinition.Environment = environment
		containerDefinitions = append(containerDefinitions, containerDefinition)
	}
	taskDefinition.ContainerDefinitions = containerDefinitions
	return nil
}

// generateServiceWithTaskDefinition returns new update service input with the desired task definition
func generateServiceWithTaskDefinition(service *ecsTypes.Service, taskDefinitionName string) (*ecs.UpdateServiceInput, error) {
	// AWS SDK Go v1 had a `awsutil.Copy` which we could use to copy all the values from one struct into another one.
	// AWS SDK Go v2 does not expose such method: https://github.com/aws/aws-sdk-go-v2/blob/9005edfbb1194c8f340b9bb7288698b58fc7274a/internal/awsutil/copy.go#L15
	// To solve this, the Service is json marshaled and unmarshalled into a UpdateServiceInput.
	updateServiceIn := &ecs.UpdateServiceInput{}
	serviceJSON, err := json.Marshal(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := json.Unmarshal(serviceJSON, updateServiceIn); err != nil {
		return nil, trace.Wrap(err)
	}
	updateServiceIn.Service = service.ServiceName
	updateServiceIn.Cluster = service.ClusterArn
	updateServiceIn.TaskDefinition = aws.String(taskDefinitionName)
	return updateServiceIn, nil
}
