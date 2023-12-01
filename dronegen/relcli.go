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

import (
	"strings"
)

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
	commands := []string{
		`mkdir -p /tmpfs/creds`,
		`echo "$RELEASES_CERT" | base64 -d > "$RELCLI_CERT"`,
		`echo "$RELEASES_KEY" | base64 -d > "$RELCLI_KEY"`,
		`trap "rm -rf /tmpfs/creds" EXIT`,
	}

	runReleaseServerCLICommand := "docker run -i -v /tmpfs/creds:/tmpfs/creds " +
		"-e DRONE_REPO -e DRONE_TAG -e RELCLI_BASE_URL -e RELCLI_CERT -e RELCLI_KEY " +
		"$RELCLI_IMAGE " + command

	// This is a workaround for a release server issue, and should be removed after the issue is fixed.
	// The release server publish step does not fail on or after the third step, consistently.
	if strings.HasPrefix(command, "auto_publish") {
		// Retry the command up to 10 times until success, and fail if none succeed.
		runReleaseServerCLICommand = `for i in $(seq 10); do ` + runReleaseServerCLICommand + ` && break; done || false`
	}
	commands = append(commands, runReleaseServerCLICommand)

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
		Volumes:  []volumeRef{volumeRefDocker, volumeRefTmpfs, volumeRefAwsConfig},
		Commands: commands,
	}
}
