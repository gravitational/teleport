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

package main

import "fmt"

func buildboxPipelineSteps() []step {
	steps := []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Commands: []string{
				`git clone --depth 1 --single-branch --branch ${DRONE_SOURCE_BRANCH:-master} https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
				`git checkout ${DRONE_COMMIT}`,
			},
		},
		waitForDockerStep(),
		kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
			awsRoleSettings: awsRoleSettings{
				awsAccessKeyID:     value{fromSecret: "STAGING_BUILDBOX_DRONE_USER_ECR_KEY"},
				awsSecretAccessKey: value{fromSecret: "STAGING_BUILDBOX_DRONE_USER_ECR_SECRET"},
				role:               value{fromSecret: "STAGING_BUILDBOX_DRONE_ECR_AWS_ROLE"},
			},
			configVolume: volumeRefAwsConfig,
			name:         "Configure Staging AWS Profile",
			profile:      "staging",
		}),
		kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
			awsRoleSettings: awsRoleSettings{
				awsAccessKeyID:     value{fromSecret: "PRODUCTION_BUILDBOX_DRONE_USER_ECR_KEY"},
				awsSecretAccessKey: value{fromSecret: "PRODUCTION_BUILDBOX_DRONE_USER_ECR_SECRET"},
				role:               value{fromSecret: "PRODUCTION_BUILDBOX_DRONE_ECR_AWS_ROLE"},
			},
			configVolume: volumeRefAwsConfig,
			name:         "Configure Production AWS Profile",
			append:       true,
			profile:      "production",
		}),
	}

	for _, name := range []string{"buildbox", "buildbox-arm", "buildbox-centos7"} {
		for _, fips := range []bool{false, true} {
			// FIPS is only supported on centos7
			if fips && name != "buildbox-centos7" {
				continue
			}
			steps = append(steps, buildboxPipelineStep(name, fips))
		}
	}
	return steps
}

func buildboxPipelineStep(buildboxName string, fips bool) step {
	var buildboxTagSuffix string
	if buildboxName == "buildbox-centos7" {
		// Drone-managed buildboxes are only amd64
		buildboxTagSuffix = "-amd64"
	}
	if fips {
		buildboxName += "-fips"
	}
	return step{
		Name:    "Build and push " + buildboxName,
		Image:   "docker",
		Pull:    "if-not-exists",
		Volumes: []volumeRef{volumeRefAwsConfig, volumeRefDocker, volumeRefDockerConfig},
		Commands: []string{
			`apk add --no-cache make aws-cli go`,
			`chown -R $UID:$GID /go`,
			// Authenticate to staging registry
			`aws ecr get-login-password --profile staging --region=us-west-2 | docker login -u="AWS" --password-stdin ` + StagingRegistry,
			// Build buildbox image
			fmt.Sprintf(`make -C build.assets %s`, buildboxName),
			// Retag for staging registry
			fmt.Sprintf(`docker tag %s/gravitational/teleport-%s:$BUILDBOX_VERSION%s %s/gravitational/teleport-%s:$BUILDBOX_VERSION-$DRONE_COMMIT_SHA`, GitHubRegistry, buildboxName, buildboxTagSuffix, StagingRegistry, buildboxName),
			// Push to staging registry
			fmt.Sprintf(`docker push %s/gravitational/teleport-%s:$BUILDBOX_VERSION-$DRONE_COMMIT_SHA`, StagingRegistry, buildboxName),
			// Authenticate to production registry
			`docker logout ` + StagingRegistry,
			`aws ecr-public get-login-password --profile production --region=us-east-1 | docker login -u="AWS" --password-stdin ` + ProductionRegistry,
			// Retag for production registry
			fmt.Sprintf(`docker tag %s/gravitational/teleport-%s:$BUILDBOX_VERSION%s %s/gravitational/teleport-%s:$BUILDBOX_VERSION`, GitHubRegistry, buildboxName, buildboxTagSuffix, ProductionRegistry, buildboxName),
			// Push to production registry
			fmt.Sprintf(`docker push %s/gravitational/teleport-%s:$BUILDBOX_VERSION`, ProductionRegistry, buildboxName),
		},
	}
}

func buildboxPipeline() pipeline {
	p := newKubePipeline("build-buildboxes")
	p.Environment = map[string]value{
		"BUILDBOX_VERSION": buildboxVersion,
		"UID":              {raw: "1000"},
		"GID":              {raw: "1000"},
	}

	// only on master for now; add the release branch name when forking a new release series.
	p.Trigger = pushTriggerForBranch("master", "branch/*")
	p.Workspace = workspace{Path: "/go/src/github.com/gravitational/teleport"}
	p.Volumes = []volume{volumeAwsConfig, volumeDocker, volumeDockerConfig}
	p.Services = []service{
		dockerService(),
	}
	p.Steps = buildboxPipelineSteps()
	return p
}
