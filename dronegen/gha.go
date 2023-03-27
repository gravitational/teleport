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

import (
	"fmt"
	"strings"
	"time"
)

type ghaBuildType struct {
	buildType
	trigger
	pipelineName string
	ghaWorkflow  string
	srcRefVar    string
	workflowRef  string
	timeout      time.Duration
	slackOnError bool
	dependsOn    []string
	inputs       map[string]string
}

func ghaBuildPipeline(b ghaBuildType) pipeline {
	p := newKubePipeline(b.pipelineName)
	p.Trigger = b.trigger
	p.Workspace = workspace{Path: "/go"}
	p.DependsOn = append(p.DependsOn, b.dependsOn...)

	var cmd strings.Builder
	cmd.WriteString(`go run ./cmd/gh-trigger-workflow `)
	cmd.WriteString(`-owner ${DRONE_REPO_OWNER} `)
	cmd.WriteString(`-repo teleport.e `)
	cmd.WriteString(`-tag-workflow `)
	fmt.Fprintf(&cmd, `-timeout %s `, b.timeout.String())
	fmt.Fprintf(&cmd, `-workflow %s `, b.ghaWorkflow)
	fmt.Fprintf(&cmd, `-workflow-ref=%s `, b.workflowRef)

	// If we don't need to build teleport...
	if b.srcRefVar != "" {
		cmd.WriteString(`-input oss-teleport-repo=${DRONE_REPO} `)
		fmt.Fprintf(&cmd, `-input oss-teleport-ref=${%s} `, b.srcRefVar)
	}

	for k, v := range b.inputs {
		fmt.Fprintf(&cmd, `-input "%s=%s" `, k, v)
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
				cmd.String(),
			},
		},
	}

	if b.slackOnError {
		p.Steps = append(p.Steps, sendErrorToSlackStep())
	}

	return p
}
