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
