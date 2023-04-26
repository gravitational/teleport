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

import (
	"fmt"
	"time"
)

// pushCheckoutCommands builds a list of commands for Drone to check out a git commit on a push build
func pushCheckoutCommands(b buildType) []string {
	return pushCheckoutCommandsWithPath(b, "/go/src/github.com/gravitational/teleport")
}

func pushCheckoutCommandsWithPath(b buildType, checkoutPath string) []string {
	var commands []string
	commands = append(commands, cloneRepoCommands(checkoutPath, "${DRONE_COMMIT_SHA}")...)
	commands = append(commands,
		`mkdir -m 0700 /root/.ssh && echo "$GITHUB_PRIVATE_KEY" > /root/.ssh/id_rsa && chmod 600 /root/.ssh/id_rsa`,
		`ssh-keyscan -H github.com > /root/.ssh/known_hosts 2>/dev/null && chmod 600 /root/.ssh/known_hosts`,
		`git submodule update --init e`,
		`mkdir -pv /go/cache`,
		`rm -f /root/.ssh/id_rsa`,
	)

	if b.fips {
		commands = append(commands, `if [[ "${DRONE_TAG}" != "" ]]; then echo "${DRONE_TAG##v}" > /go/.version.txt; else egrep ^VERSION Makefile | cut -d= -f2 > /go/.version.txt; fi; cat /go/.version.txt`)
	}
	return commands
}

// pushBuildCommands generates a list of commands for Drone to build an artifact as part of a push build
func pushBuildCommands(b buildType) []string {
	commands := []string{
		`apk add --no-cache make`,
		`chown -R $UID:$GID /go`,
		`cd /go/src/github.com/gravitational/teleport`,
	}
	if b.fips || b.hasTeleportConnect() {
		commands = append(commands,
			`export VERSION=$(cat /go/.version.txt)`,
		)
	}
	commands = append(commands,
		fmt.Sprintf(`make -C build.assets %s`, releaseMakefileTarget(b)),
	)

	if b.hasTeleportConnect() {
		commands = append(commands, `make -C build.assets teleterm`)
	}
	return commands
}

// pushPipelines builds all applicable push pipeline combinations
func pushPipelines() []pipeline {
	var ps []pipeline
	for _, arch := range []string{"amd64", "386", "arm"} {
		for _, fips := range []bool{false, true} {
			if arch != "amd64" && fips {
				// FIPS mode only supported on linux/amd64
				continue
			}
			ps = append(ps, pushPipeline(buildType{os: "linux", arch: arch, fips: fips}))
		}
	}

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", arch: "arm64"},
		trigger:      triggerPush,
		pipelineName: "push-build-linux-arm64",
		workflows: []ghaWorkflow{
			{
				name:              "release-linux-arm64.yml",
				timeout:           150 * time.Minute,
				slackOnError:      true,
				srcRefVar:         "DRONE_COMMIT",
				ref:               "${DRONE_BRANCH}",
				shouldTagWorkflow: true,
				inputs:            map[string]string{"upload-artifacts": "false"},
			},
		},
	}))

	// Only amd64 Windows is supported for now.
	ps = append(ps, pushPipeline(buildType{os: "windows", arch: "amd64", windowsUnsigned: true}))

	ps = append(ps, darwinPushPipelineGHA())
	ps = append(ps, windowsPushPipeline())
	return ps
}

// pushPipeline generates a push pipeline for a given combination of os/arch/FIPS
func pushPipeline(b buildType) pipeline {
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	pipelineName := fmt.Sprintf("push-build-%s-%s", b.os, b.arch)
	pushEnvironment := map[string]value{
		"UID":     {raw: "1000"},
		"GID":     {raw: "1000"},
		"GOCACHE": {raw: "/go/cache"},
		"GOPATH":  {raw: "/go"},
		"OS":      {raw: b.os},
		"ARCH":    {raw: b.arch},
	}
	if b.fips {
		pipelineName += "-fips"
		pushEnvironment["FIPS"] = value{raw: "yes"}
	}

	p := newKubePipeline(pipelineName)
	p.Environment = map[string]value{
		"BUILDBOX_VERSION": buildboxVersion,
		"RUNTIME":          goRuntime,
		"UID":              {raw: "1000"},
		"GID":              {raw: "1000"},
	}
	p.Trigger = triggerPush
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = []volume{volumeDocker, volumeDockerConfig}
	p.Services = []service{
		dockerService(),
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Pull:  "if-not-exists",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: pushCheckoutCommands(b),
		},
		waitForDockerStep(),
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Pull:        "if-not-exists",
			Environment: pushEnvironment,
			Volumes:     []volumeRef{volumeRefDocker, volumeRefDockerConfig},
			Commands:    pushBuildCommands(b),
		},
		sendErrorToSlackStep(),
	}
	return p
}

func sendErrorToSlackStep() step {
	return step{
		Name:  "Send Slack notification",
		Image: "plugins/slack:1.4.1",
		Settings: map[string]value{
			"webhook": {fromSecret: "SLACK_WEBHOOK_DEV_TELEPORT"},
			"template": {
				raw: "*✘ Failed:* `{{ build.event }}` / `${DRONE_STAGE_NAME}` / <{{ build.link }}|Build: #{{ build.number }}>\n" +
					"Author: <https://github.com/{{ build.author }}|{{ build.author }}> " +
					"Repo: <https://github.com/{{ repo.owner }}/{{ repo.name }}/|{{ repo.owner }}/{{ repo.name }}> " +
					"Branch: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commits/{{ build.branch }}|{{ build.branch }}> " +
					"Commit: <https://github.com/{{ repo.owner }}/{{ repo.name }}/commit/{{ build.commit }}|{{ truncate build.commit 8 }}>",
			},
		},
		When: &condition{Status: []string{"failure"}},
	}
}
