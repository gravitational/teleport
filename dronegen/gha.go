// Copyright 2023 Gravitational, Inc
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

type ghaBuildType struct {
	buildType
	trigger
	namePrefix      string
	uploadArtifacts bool
	srcRefVar       string
	workflowRefVar  string
	slackOnError    bool
}

func ghaBuildPipeline(b ghaBuildType) pipeline {
	p := newKubePipeline(fmt.Sprintf("%sbuild-%s-%s", b.namePrefix, b.os, b.arch))
	p.Trigger = b.trigger
	p.Workspace = workspace{Path: "/go"}
	p.Environment = map[string]value{
		"BUILDBOX_VERSION": buildboxVersion,
		"RUNTIME":          goRuntime,
		"UID":              {raw: "1000"},
		"GID":              {raw: "1000"},
	}

	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: pushCheckoutCommands(b.buildType),
		},
		{
			Name:  "Delegate build to GitHub",
			Image: fmt.Sprintf("golang:%s-alpine", GoVersion),
			Environment: map[string]value{
				"GHA_APP_KEY": {fromSecret: "GITHUB_WORKFLOW_APP_PRIVATE_KEY"},
			},
			Commands: []string{
				`cd "/go/src/github.com/gravitational/teleport/build.assets/tooling"`,
				`go run ./cmd/gh-trigger-workflow -owner ${DRONE_REPO_OWNER} -repo teleport.e -workflow release-linux-arm64.yml ` +
					fmt.Sprintf(`-workflow-ref=${%s} `, b.workflowRefVar) +
					fmt.Sprintf(`-input oss-teleport-ref=${%s} `, b.srcRefVar) +
					fmt.Sprintf(`-input upload-artifacts=%t`, b.uploadArtifacts),
			},
		},
	}

	if b.slackOnError {
		p.Steps = append(p.Steps, sendErrorToSlackStep())
	}

	return p
}
