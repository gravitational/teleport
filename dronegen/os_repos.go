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
	"time"
)

type osPackageDeployment struct {
	versionChannel    string
	packageNameFilter string
	packageToTest     string
	displayName       string
}

func promoteBuildOsRepoPipeline() pipeline {
	packageDeployments := []osPackageDeployment{
		// Normal release pipelines
		{
			versionChannel: "${DRONE_TAG}",
			packageToTest:  "teleport-ent",
			displayName:    "Teleport",
		},
		// teleport-ent-updater to stable/cloud only pipelines
		{
			versionChannel:    "cloud",
			packageNameFilter: `teleport-ent-updater*`,
			displayName:       "teleport-ent-updater",
		},
		// Rolling release pipelines
		{
			versionChannel: "rolling",
			packageToTest:  "teleport-ent",
			displayName:    "Teleport",
		},
	}

	return buildPromoteOsPackagePipelines(packageDeployments)
}

func buildPromoteOsPackagePipelines(packageDeployments []osPackageDeployment) pipeline {
	releaseEnvironmentFilePath := "/go/vars/release-environment.txt"
	clonePath := "/go/src/github.com/gravitational/teleport"

	ghaBuild := ghaBuildType{
		trigger:                    triggerPromote,
		pipelineName:               "publish-os-package-repos",
		checkoutPath:               clonePath,
		workflows:                  buildWorkflows(releaseEnvironmentFilePath, packageDeployments),
		enableParallelWorkflowRuns: true,
	}
	setupSteps := []step{
		{
			Name:  "Determine if release should go to development or production",
			Image: fmt.Sprintf("golang:%s-alpine", GoVersion),
			Commands: []string{
				fmt.Sprintf("cd %q", path.Join(clonePath, "build.assets", "tooling")),
				fmt.Sprintf("mkdir -pv %q", path.Dir(releaseEnvironmentFilePath)),
				fmt.Sprintf(`(CGO_ENABLED=0 go run ./cmd/check -tag ${DRONE_TAG} -check prerelease && echo "promote" || echo "build") > %q`, releaseEnvironmentFilePath),
			},
		},
	}

	return ghaMultiBuildPipeline(setupSteps, ghaBuild)
}

func buildWorkflows(releaseEnvironmentFilePath string, packageDeployments []osPackageDeployment) []ghaWorkflow {
	repoTypes := []string{"apt", "yum"}
	workflows := make([]ghaWorkflow, 0, len(repoTypes)*len(packageDeployments))
	for _, packageDeployment := range packageDeployments {
		for _, repoType := range repoTypes {
			inputs := map[string]string{
				"repo-type":       repoType,
				"environment":     fmt.Sprintf("$(cat %q)", releaseEnvironmentFilePath),
				"artifact-tag":    "${DRONE_TAG}",
				"release-channel": "stable",
				"version-channel": packageDeployment.versionChannel,
			}

			if packageDeployment.packageNameFilter != "" {
				inputs["package-name-filter"] = packageDeployment.packageNameFilter
			}

			if packageDeployment.packageToTest != "" {
				inputs["package-to-test"] = packageDeployment.packageToTest
			}

			workflows = append(workflows, ghaWorkflow{
				stepName:          fmt.Sprintf("Publish %s to stable/%s %s repo", packageDeployment.displayName, packageDeployment.versionChannel, repoType),
				name:              "deploy-packages.yaml",
				ref:               "refs/heads/master",
				timeout:           12 * time.Hour, // DR takes a long time
				shouldTagWorkflow: true,
				seriesRun:         true,
				seriesRunFilter:   fmt.Sprintf(".*%s.*", repoType),
				inputs:            inputs,
			})
		}
	}

	return workflows
}
