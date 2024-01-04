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
	"path"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"
)

type ghaWorkflow struct {
	name              string
	stepName          string
	srcRefVar         string
	ref               string
	timeout           time.Duration
	slackOnError      bool
	shouldTagWorkflow bool
	seriesRun         bool
	seriesRunFilter   string
	inputs            map[string]string
}

type ghaBuildType struct {
	buildType
	trigger
	pipelineName               string
	checkoutPath               string
	dependsOn                  []string
	workflows                  []ghaWorkflow
	enableParallelWorkflowRuns bool
}

func ghaBuildPipeline(ghaBuild ghaBuildType) pipeline {
	return ghaMultiBuildPipeline(nil, ghaBuild)
}

// ghaMultiBuildPipeline returns a pipeline with multiple supported workflow call steps
func ghaMultiBuildPipeline(setupSteps []step, ghaBuild ghaBuildType) pipeline {
	p := newKubePipeline(ghaBuild.pipelineName)
	p.Trigger = ghaBuild.trigger
	p.Workspace = workspace{Path: "/go"}
	p.DependsOn = append(p.DependsOn, ghaBuild.dependsOn...)

	checkoutPath := ghaBuild.checkoutPath
	if checkoutPath == "" {
		checkoutPath = "/go/src/github.com/gravitational/teleport"
	}

	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Pull:  "if-not-exists",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: pushCheckoutCommandsWithPath(ghaBuild.buildType, checkoutPath),
		},
	}

	setupStepNames := getStepNames(p.Steps)
	if ghaBuild.enableParallelWorkflowRuns && len(setupSteps) >= 1 {
		for index := range setupSteps {
			setupSteps[index].DependsOn = append(setupSteps[index].DependsOn, setupStepNames...)
		}
		setupStepNames = getStepNames(setupSteps)
	}

	p.Steps = append(p.Steps, setupSteps...)

	for counter, workflow := range ghaBuild.workflows {
		// This wait time is to avoid a GHA pending/queuing issue where multiple workflows fired off at the same time
		// (or somewhat close to the same time) will cause only one workflow to be queued, with the others being canceled.
		// For details, see
		// https://docs.github.com/en/actions/using-jobs/using-concurrency#example-only-cancel-in-progress-jobs-or-runs-for-the-current-workflow:~:text=When%20a%20concurrent,progress%3A%20true.
		sleepTime := time.Duration(counter*10) * time.Second // 10 seconds for each workflow
		p.Steps = append(p.Steps, buildWorkflowSteps(workflow, checkoutPath, ghaBuild.enableParallelWorkflowRuns, sleepTime, setupStepNames)...)
	}

	return p
}

func buildWorkflowSteps(workflow ghaWorkflow, checkoutPath string, enableParallelWorkflowRuns bool, sleepTime time.Duration, setupStepNames []string) []step {
	var steps []step
	workflowStep := buildGHAWorkflowCallStep(workflow, checkoutPath)

	if enableParallelWorkflowRuns {
		if sleepTime > 0 {
			sleepStep := sleepStep(sleepTime, setupStepNames, workflow.stepName)
			steps = append(steps, sleepStep)
			workflowStep.DependsOn = append(workflowStep.DependsOn, sleepStep.Name)
		} else {
			workflowStep.DependsOn = append(workflowStep.DependsOn, setupStepNames...)
		}
	}

	steps = append(steps, workflowStep)

	if workflow.slackOnError {
		slackStep := sendErrorToSlackStep()
		if enableParallelWorkflowRuns {
			slackStep.DependsOn = append(slackStep.DependsOn, workflowStep.Name)
		}

		steps = append(steps, slackStep)
	}

	return steps
}

func sleepStep(sleepTime time.Duration, setupStepNames []string, stepNameSuffix string) step {
	return step{
		Name:  fmt.Sprintf("Wait - %s", stepNameSuffix),
		Image: "alpine:latest",
		Commands: []string{
			fmt.Sprintf("sleep %v", sleepTime.Round(time.Second).Seconds()),
		},
		DependsOn: setupStepNames,
	}
}

func buildGHAWorkflowCallStep(workflow ghaWorkflow, checkoutPath string) step {
	var cmd strings.Builder
	cmd.WriteString(`go run ./cmd/gh-trigger-workflow `)
	cmd.WriteString(`-owner ${DRONE_REPO_OWNER} `)
	cmd.WriteString(`-repo teleport.e `)

	if workflow.shouldTagWorkflow {
		cmd.WriteString(`-tag-workflow `)
	}

	if workflow.seriesRun {
		cmd.WriteString(`-series-run `)

		if workflow.seriesRunFilter != "" {
			fmt.Fprintf(&cmd, `-series-run-filter %s `, workflow.seriesRunFilter)
		}
	}

	fmt.Fprintf(&cmd, `-timeout %s `, workflow.timeout.String())
	fmt.Fprintf(&cmd, `-workflow %s `, workflow.name)
	fmt.Fprintf(&cmd, `-workflow-ref=%s `, workflow.ref)

	// If we don't need to build teleport...
	if workflow.srcRefVar != "" {
		cmd.WriteString(`-input oss-teleport-repo=${DRONE_REPO} `)
		fmt.Fprintf(&cmd, `-input oss-teleport-ref=${%s} `, workflow.srcRefVar)
	}

	// Sort inputs so the are output in a consistent order to avoid
	// spurious changes in the generated drone config.
	keys := maps.Keys(workflow.inputs)
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&cmd, `-input "%s=%s" `, k, workflow.inputs[k])
	}

	stepName := workflow.stepName
	if stepName == "" {
		stepName = "Delegate build to GitHub"
	}

	return step{
		Name:  stepName,
		Image: fmt.Sprintf("golang:%s-alpine", GoVersion),
		Pull:  "if-not-exists",
		Environment: map[string]value{
			"GHA_APP_KEY": {fromSecret: "GITHUB_WORKFLOW_APP_PRIVATE_KEY"},
		},
		Commands: []string{
			fmt.Sprintf(`cd %q`, path.Join(checkoutPath, "build.assets", "tooling")),
			cmd.String(),
		},
	}
}
