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
	"fmt"
	"strconv"
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

	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "amd64", fips: false, buildConnect: true}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "amd64", fips: true}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "386", fips: false}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "arm64", fips: false}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "arm64", fips: true}))
	ps = append(ps, ghaLinuxPushPipeline(buildType{os: "linux", arch: "arm", fips: false}))
	ps = append(ps, ghaWindowsPushPipeline())

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
		inputs: map[string]string{
			"release-target": releaseMakefileTarget(b),
			"build-connect":  strconv.FormatBool(b.buildConnect),
		},
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
