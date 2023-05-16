// Copyright 2022 Gravitational, Inc
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

const relcliImage = "146628656107.dkr.ecr.us-west-2.amazonaws.com/gravitational/relcli:master-57a5d42-20230412T1204687"

func relcliPipeline(trigger trigger, name string, stepName string, command string) pipeline {
	p := newKubePipeline(name)
	p.Environment = map[string]value{
		"RELCLI_IMAGE": {raw: relcliImage},
	}
	p.Trigger = trigger
	p.Steps = []step{
		{
			Name:  "Check if commit is tagged",
			Image: "alpine",
			Commands: []string{
				`[ -n ${DRONE_TAG} ] || (echo 'DRONE_TAG is not set. Is the commit tagged?' && exit 1)`,
			},
		},
		waitForDockerStep(),
		kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
			awsRoleSettings: awsRoleSettings{
				awsAccessKeyID:     value{fromSecret: "TELEPORT_BUILD_USER_READ_ONLY_KEY"},
				awsSecretAccessKey: value{fromSecret: "TELEPORT_BUILD_USER_READ_ONLY_SECRET"},
				role:               value{fromSecret: "TELEPORT_BUILD_READ_ONLY_AWS_ROLE"},
			},
			configVolume: volumeRefAwsConfig,
		}),
		pullRelcliStep(volumeRefAwsConfig),
		executeRelcliStep(stepName, command),
	}

	p.Services = []service{dockerService(volumeRefTmpfs)}
	p.Volumes = []volume{volumeTmpfs, volumeAwsConfig, volumeDocker, volumeDockerConfig}

	return p
}

func pullRelcliStep(awsConfigVolumeRef volumeRef) step {
	return step{
		Name:  "Pull relcli",
		Image: "docker:cli",
		Environment: map[string]value{
			"AWS_DEFAULT_REGION": {raw: "us-west-2"},
		},
		Volumes: []volumeRef{volumeRefDocker, volumeRefAwsConfig},
		Commands: []string{
			`apk add --no-cache aws-cli`,
			`aws ecr get-login-password | docker login -u="AWS" --password-stdin 146628656107.dkr.ecr.us-west-2.amazonaws.com`,
			`docker pull $RELCLI_IMAGE`,
		},
	}
}

func executeRelcliStep(name string, command string) step {
	return step{
		Name:  name,
		Image: "docker:git",
		Environment: map[string]value{
			"RELCLI_BASE_URL": {raw: releasesHost},
			"RELEASES_CERT":   {fromSecret: "RELEASES_CERT"},
			"RELEASES_KEY":    {fromSecret: "RELEASES_KEY"},
			"RELCLI_CERT":     {raw: "/tmpfs/creds/releases.crt"},
			"RELCLI_KEY":      {raw: "/tmpfs/creds/releases.key"},
		},
		Volumes: []volumeRef{volumeRefDocker, volumeRefTmpfs, volumeRefAwsConfig},
		Commands: []string{
			`mkdir -p /tmpfs/creds`,
			`echo "$RELEASES_CERT" | base64 -d > "$RELCLI_CERT"`,
			`echo "$RELEASES_KEY" | base64 -d > "$RELCLI_KEY"`,
			`trap "rm -rf /tmpfs/creds" EXIT`,
			`docker run -i -v /tmpfs/creds:/tmpfs/creds \
  -e DRONE_REPO -e DRONE_TAG -e RELCLI_BASE_URL -e RELCLI_CERT -e RELCLI_KEY \
  $RELCLI_IMAGE ` + command,
		},
	}
}
