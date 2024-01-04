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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestGenerateServiceWithTaskDefinition(t *testing.T) {
	service := &ecsTypes.Service{
		ServiceName:    aws.String("service"),
		ClusterArn:     aws.String("cluster"),
		TaskDefinition: aws.String("task-definition-v1"),
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				AssignPublicIp: ecsTypes.AssignPublicIpEnabled,
				Subnets:        []string{"subnet"},
			},
		},
		PropagateTags: ecsTypes.PropagateTagsService,
	}

	expected := &ecs.UpdateServiceInput{
		Service:        aws.String("service"),
		Cluster:        aws.String("cluster"),
		TaskDefinition: aws.String("task-definition-v2"),
		NetworkConfiguration: &ecsTypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecsTypes.AwsVpcConfiguration{
				AssignPublicIp: ecsTypes.AssignPublicIpEnabled,
				Subnets:        []string{"subnet"},
			},
		},
		PropagateTags: ecsTypes.PropagateTagsService,
	}

	require.Equal(t, expected, generateServiceWithTaskDefinition(service, "task-definition-v2"))
}

func TestGenerateTaskDefinitionWithImage(t *testing.T) {
	taskDefinition := &ecsTypes.TaskDefinition{
		Family: aws.String("example-task"),
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:    &taskCPU,
		Memory: &taskMem,

		NetworkMode:      ecsTypes.NetworkModeAwsvpc,
		TaskRoleArn:      aws.String("task-role-arn"),
		ExecutionRoleArn: aws.String("task-role-arn"),
		ContainerDefinitions: []ecsTypes.ContainerDefinition{{
			Environment: []ecsTypes.KeyValuePair{{
				Name:  aws.String(types.InstallMethodAWSOIDCDeployServiceEnvVar),
				Value: aws.String("true"),
			}},
			Command: []string{
				"start",
				"--config-string",
				"config-bytes",
			},
			EntryPoint: []string{"teleport"},
			Image:      aws.String("image-v1"),
			Name:       &taskAgentContainerName,
			LogConfiguration: &ecsTypes.LogConfiguration{
				LogDriver: ecsTypes.LogDriverAwslogs,
				Options: map[string]string{
					"awslogs-group":         "ecs-cluster",
					"awslogs-region":        "us-west-2",
					"awslogs-create-group":  "true",
					"awslogs-stream-prefix": "service/example-task",
				},
			},
		}},
	}
	tags := []ecsTypes.Tag{
		{Key: aws.String("teleport.dev/origin"), Value: aws.String("integration_awsoidc")},
	}

	expected := &ecs.RegisterTaskDefinitionInput{
		Family: aws.String("example-task"),
		RequiresCompatibilities: []ecsTypes.Compatibility{
			ecsTypes.CompatibilityFargate,
		},
		Cpu:    &taskCPU,
		Memory: &taskMem,

		NetworkMode:      ecsTypes.NetworkModeAwsvpc,
		TaskRoleArn:      aws.String("task-role-arn"),
		ExecutionRoleArn: aws.String("task-role-arn"),
		ContainerDefinitions: []ecsTypes.ContainerDefinition{{
			Environment: []ecsTypes.KeyValuePair{{
				Name:  aws.String(types.InstallMethodAWSOIDCDeployServiceEnvVar),
				Value: aws.String("true"),
			}},
			Command: []string{
				"start",
				"--config-string",
				"config-bytes",
			},
			EntryPoint: []string{"teleport"},
			Image:      aws.String("image-v2"),
			Name:       &taskAgentContainerName,
			LogConfiguration: &ecsTypes.LogConfiguration{
				LogDriver: ecsTypes.LogDriverAwslogs,
				Options: map[string]string{
					"awslogs-group":         "ecs-cluster",
					"awslogs-region":        "us-west-2",
					"awslogs-create-group":  "true",
					"awslogs-stream-prefix": "service/example-task",
				},
			},
		}},
		Tags: tags,
	}

	input, err := generateTaskDefinitionWithImage(taskDefinition, "image-v2", tags)
	require.NoError(t, err)
	require.Equal(t, expected, input)
}
