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

// pushPipelines builds all applicable push pipeline combinations
func pushPipelines() []pipeline {
	var ps []pipeline

	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "amd64", fips: false}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "amd64", fips: true}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "386", fips: false}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "arm", fips: false}))

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
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "windows", arch: "amd64", windowsUnsigned: true}))

	ps = append(ps, windowsPushPipeline())
	return ps
}

// ghaLinuxPushPipeline generates a push pipeline for a given combination of
// os/arch/FIPS that calls a GitHub Actions workflow to perform the build on
// a Linux buildbox. This dispatches to the release-linux.yaml workflow in
// the teleport.e repo, which is a little more generic than the
// release-linux-arm64.yml workflow used for the arm64 build. The two will
// be unified shortly.
func ghaLinuxPushPipeline(b buildType) pipeline {
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	pipelineName := fmt.Sprintf("push-build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName += "-fips"
	}
	wf := ghaWorkflow{
		name:              "release-linux.yaml",
		timeout:           150 * time.Minute,
		slackOnError:      true,
		srcRefVar:         "DRONE_COMMIT",
		ref:               "${DRONE_BRANCH}",
		shouldTagWorkflow: true,
		inputs:            map[string]string{"release-target": releaseMakefileTarget(b)},
	}
	bt := ghaBuildType{
		buildType:    buildType{os: b.os, arch: b.arch},
		trigger:      triggerPush,
		pipelineName: pipelineName,
		workflows:    []ghaWorkflow{wf},
	}
	return ghaBuildPipeline(bt)
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
