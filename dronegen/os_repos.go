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
			versionChannel:    "${DRONE_TAG}",
			packageNameFilter: `$($DRONE_REPO_PRIVATE && echo "*ent*" || echo "")`,
			packageToTest:     "teleport-ent",
			displayName:       "Teleport",
		},
	}

	return buildPromoteOsPackagePipelines(packageDeployments)
}

func buildPromoteOsPackagePipelines(packageDeployments []osPackageDeployment) pipeline {
	releaseEnvironmentFilePath := "/go/vars/release-environment.txt"
	clonePath := "/go/src/github.com/gravitational/teleport"

	ghaBuild := ghaBuildType{
		trigger:      triggerPromote,
		pipelineName: "publish-os-package-repos",
		checkoutPath: clonePath,
		workflows:    buildWorkflows(releaseEnvironmentFilePath, packageDeployments),
	}
	setupSteps := []step{
		{
			Name:  "Determine if release should go to development or production",
			Image: fmt.Sprintf("golang:%s-alpine", GoVersion),
			Commands: []string{
				fmt.Sprintf("cd %q", path.Join(clonePath, "build.assets", "tooling")),
				fmt.Sprintf("mkdir -pv %q", path.Dir(releaseEnvironmentFilePath)),
				fmt.Sprintf(`(go run ./cmd/check -tag ${DRONE_TAG} -check prerelease && echo "promote" || echo "build") > %q`, releaseEnvironmentFilePath),
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
				"repo-type":           repoType,
				"environment":         fmt.Sprintf("$(cat %q)", releaseEnvironmentFilePath),
				"artifact-tag":        "${DRONE_TAG}",
				"release-channel":     "stable",
				"version-channel":     packageDeployment.versionChannel,
				"package-name-filter": packageDeployment.packageNameFilter,
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
				inputs:            inputs,
			})
		}
	}

	return workflows
}
