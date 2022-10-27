// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import "fmt"

func promoteBuildPipelines() []pipeline {
	promotePipelines := make([]pipeline, 0)
	promotePipelines = append(promotePipelines, promoteBuildOsRepoPipelines()...)
	promotePipelines = append(promotePipelines, buildDockerPromotionPipelineECR(), buildDockerPromotionPipelineQuay())

	return promotePipelines
}

func buildDockerPromotionPipelineECR() pipeline {
	dockerPipeline := newKubePipeline("promote-docker-ecr")
	dockerPipeline.Trigger = triggerPromote
	dockerPipeline.Trigger.Target.Include = append(dockerPipeline.Trigger.Target.Include, "promote-docker", "promote-docker-ecr")
	dockerPipeline.Workspace = workspace{Path: "/go"}

	// Add docker service
	dockerPipeline.Services = []service{
		dockerService(),
	}
	dockerPipeline.Volumes = []volume{
		volumeDocker,
		volumeAwsConfig,
	}

	dockerPipeline.Steps = append(dockerPipeline.Steps, verifyTaggedStep())
	dockerPipeline.Steps = append(dockerPipeline.Steps, waitForDockerStep())

	// Pull/Push Steps
	dockerPipeline.Steps = append(dockerPipeline.Steps, kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
		awsRoleSettings: awsRoleSettings{
			awsAccessKeyID:     value{fromSecret: "PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY"},
			awsSecretAccessKey: value{fromSecret: "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET"},
			role:               value{fromSecret: "PRODUCTION_TELEPORT_DRONE_ECR_AWS_ROLE"},
		},
		configVolume: volumeRefAwsConfig,
	}))
	dockerPipeline.Steps = append(dockerPipeline.Steps, step{
		Name:  "Pull/retag Docker images",
		Image: "docker",
		Volumes: []volumeRef{
			volumeRefDocker,
			volumeRefAwsConfig,
		},
		Commands: []string{
			"apk add --no-cache aws-cli",
			"export VERSION=${DRONE_TAG##v}",
			// authenticate with staging credentials
			"aws ecr get-login-password --region=us-west-2 | docker login -u=\"AWS\" --password-stdin " + StagingRegistry,
			// pull staging images
			"echo \"---> Pulling images for $${VERSION}\"",
			fmt.Sprintf("docker pull %s/gravitational/teleport:$${VERSION}", StagingRegistry),
			fmt.Sprintf("docker pull %s/gravitational/teleport-ent:$${VERSION}", StagingRegistry),
			fmt.Sprintf("docker pull %s/gravitational/teleport-ent:$${VERSION}-fips", StagingRegistry),
			// retag images to production naming
			"echo \"---> Tagging images for $${VERSION}\"",
			fmt.Sprintf("docker tag %s/gravitational/teleport:$${VERSION} %s/gravitational/teleport:$${VERSION}", StagingRegistry, ProductionRegistry),
			fmt.Sprintf("docker tag %s/gravitational/teleport-ent:$${VERSION} %s/gravitational/teleport-ent:$${VERSION}", StagingRegistry, ProductionRegistry),
			fmt.Sprintf("docker tag %s/gravitational/teleport-ent:$${VERSION}-fips %s/gravitational/teleport-ent:$${VERSION}-fips", StagingRegistry, ProductionRegistry),
			// authenticate with production credentials
			"docker logout " + StagingRegistry,
			"aws ecr-public get-login-password --region=us-east-1 | docker login -u=\"AWS\" --password-stdin " + ProductionRegistry,
			// push production images
			"echo \"---> Pushing images for $${VERSION}\"",
			// push production images ECR
			fmt.Sprintf("docker push %s/gravitational/teleport:$${VERSION}", ProductionRegistry),
			fmt.Sprintf("docker push %s/gravitational/teleport-ent:$${VERSION}", ProductionRegistry),
			fmt.Sprintf("docker push %s/gravitational/teleport-ent:$${VERSION}-fips", ProductionRegistry),
		},
	})

	return dockerPipeline
}

func buildDockerPromotionPipelineQuay() pipeline {
	dockerPipeline := newKubePipeline("promote-docker-quay")
	dockerPipeline.Trigger = triggerPromote
	dockerPipeline.Trigger.Target.Include = append(dockerPipeline.Trigger.Target.Include, "promote-docker", "promote-docker-quay")
	dockerPipeline.Workspace = workspace{Path: "/go"}

	// Add docker service
	dockerPipeline.Services = []service{
		dockerService(),
	}
	dockerPipeline.Volumes = []volume{
		volumeDocker,
		volumeAwsConfig,
	}

	dockerPipeline.Steps = append(dockerPipeline.Steps, verifyTaggedStep())
	dockerPipeline.Steps = append(dockerPipeline.Steps, waitForDockerStep())

	// Pull/Push Steps
	dockerPipeline.Steps = append(dockerPipeline.Steps, kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
		awsRoleSettings: awsRoleSettings{
			awsAccessKeyID:     value{fromSecret: "PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY"},
			awsSecretAccessKey: value{fromSecret: "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET"},
			role:               value{fromSecret: "PRODUCTION_TELEPORT_DRONE_ECR_AWS_ROLE"},
		},
		configVolume: volumeRefAwsConfig,
	}))
	dockerPipeline.Steps = append(dockerPipeline.Steps, step{
		Name:  "Pull/retag Docker images",
		Image: "docker",
		Environment: map[string]value{
			"QUAY_USERNAME": {fromSecret: "PRODUCTION_QUAYIO_DOCKER_USERNAME"},
			"QUAY_PASSWORD": {fromSecret: "PRODUCTION_QUAYIO_DOCKER_PASSWORD"},
		},
		Volumes: []volumeRef{
			volumeRefDocker,
			volumeRefAwsConfig,
		},
		Commands: []string{
			"apk add --no-cache aws-cli",
			"export VERSION=${DRONE_TAG##v}",
			// authenticate with staging credentials
			"aws ecr get-login-password --region=us-west-2 | docker login -u=\"AWS\" --password-stdin " + StagingRegistry,
			// pull staging images
			"echo \"---> Pulling images for $${VERSION}\"",
			fmt.Sprintf("docker pull %s/gravitational/teleport:$${VERSION}", StagingRegistry),
			fmt.Sprintf("docker pull %s/gravitational/teleport-ent:$${VERSION}", StagingRegistry),
			fmt.Sprintf("docker pull %s/gravitational/teleport-ent:$${VERSION}-fips", StagingRegistry),
			// retag images to production naming
			"echo \"---> Tagging images for $${VERSION}\"",
			fmt.Sprintf("docker tag %s/gravitational/teleport:$${VERSION} %s/gravitational/teleport:$${VERSION}", StagingRegistry, ProductionRegistryQuay),
			fmt.Sprintf("docker tag %s/gravitational/teleport-ent:$${VERSION} %s/gravitational/teleport-ent:$${VERSION}", StagingRegistry, ProductionRegistryQuay),
			fmt.Sprintf("docker tag %s/gravitational/teleport-ent:$${VERSION}-fips %s/gravitational/teleport-ent:$${VERSION}-fips", StagingRegistry, ProductionRegistryQuay),
			// authenticate with production credentials
			"docker logout " + StagingRegistry,
			"docker login -u=\"$QUAY_USERNAME\" -p=\"$QUAY_PASSWORD\" " + ProductionRegistryQuay,
			// push production images
			"echo \"---> Pushing images for $${VERSION}\"",
			fmt.Sprintf("docker push %s/gravitational/teleport:$${VERSION}", ProductionRegistryQuay),
			fmt.Sprintf("docker push %s/gravitational/teleport-ent:$${VERSION}", ProductionRegistryQuay),
			fmt.Sprintf("docker push %s/gravitational/teleport-ent:$${VERSION}-fips", ProductionRegistryQuay),
		},
	})

	return dockerPipeline
}

func publishReleasePipeline() pipeline {
	return relcliPipeline(triggerPromote, "publish-rlz", "Publish in Release API", "relcli auto_publish -f -v 6")
}
