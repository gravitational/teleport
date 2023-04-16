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

func buildOsRepoPipelines() []pipeline {
	pipelines := promoteBuildOsRepoPipelines()

	return pipelines
}

func promoteBuildOsRepoPipelines() []pipeline {
	pipelines := []pipeline{}

	// Normal release pipelines
	pipelines = append(pipelines, buildPromoteOsPackagePipelines("${DRONE_TAG}", `$($DRONE_REPO_PRIVATE && echo "*ent*" || echo "")`, "normal")...)

	// teleport-ent-updater to stable/cloud only pipelines
	pipelines = append(pipelines, buildPromoteOsPackagePipelines("cloud", "teleport-ent-updater*", "teleport-ent-updater")...)

	return pipelines
}

func buildPromoteOsPackagePipelines(versionChannel, packageNameFilter, pipelineNameSuffix string) []pipeline {
	return []pipeline{
		buildPromoteOsPackagePipeline("apt", versionChannel, packageNameFilter, pipelineNameSuffix),
		buildPromoteOsPackagePipeline("yum", versionChannel, packageNameFilter, pipelineNameSuffix),
	}
}

func buildPromoteOsPackagePipeline(repoType, versionChannel, packageNameFilter, pipelineNameSuffix string) pipeline {
	releaseEnvironmentFilePath := "/go/vars/release-environment.txt"
	clonePath := "/go/src/github.com/gravitational/teleport"

	pipeline := ghaBuildPipeline(ghaBuildType{
		trigger:           triggerPromote,
		pipelineName:      fmt.Sprintf("publish-%s-%s-new-repos", pipelineNameSuffix, repoType),
		ghaWorkflow:       "deploy-packages.yaml",
		timeout:           12 * time.Hour, // DR takes a long time
		workflowRef:       "refs/heads/master",
		shouldTagWorkflow: false,
		inputs: map[string]string{
			"repo-type":           repoType,
			"environment":         fmt.Sprintf("$(cat %q)", releaseEnvironmentFilePath),
			"artifact-tag":        "${DRONE_TAG}",
			"release-channel":     "stable",
			"version-channel":     versionChannel,
			"package-name-filter": packageNameFilter,
		},
	})

	pipeline.Steps = []step{
		pipeline.Steps[0],
		{
			Name:  "Determine if release should go to development or production",
			Image: fmt.Sprintf("golang:%s-alpine", GoVersion),
			Commands: []string{
				fmt.Sprintf("cd %q", path.Join(clonePath, "build.assets", "tooling")),
				fmt.Sprintf("mkdir -pv %q", path.Dir(releaseEnvironmentFilePath)),
				fmt.Sprintf(`(go run ./cmd/check -tag ${DRONE_TAG} -check prerelease && echo "promote" || echo "build") > %q`, releaseEnvironmentFilePath),
			},
		},
		pipeline.Steps[1],
	}

	return pipeline
}
