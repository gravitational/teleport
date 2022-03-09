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

import "fmt"

func buildboxPipelineSteps() []step {
	steps := []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Commands: []string{
				`git clone --depth 1 --single-branch --branch ${DRONE_SOURCE_BRANCH:-master} https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
				`git checkout ${DRONE_COMMIT}`,
			},
		},
		waitForDockerStep(),
	}

	for _, name := range []string{"buildbox", "buildbox-arm"} {
		for _, os := range []string{"", "centos7"} {
			for _, fips := range []bool{false, true} {
				// FIPS is unsupported on ARM/ARM64
				if name == "buildbox-arm" && fips {
					continue
				}
				steps = append(steps, buildboxPipelineStep(name, os, fips))
			}
		}
	}
	return steps
}

func buildboxPipelineStep(buildboxName string, os string, fips bool) step {
	if os != "" {
		buildboxName += fmt.Sprintf("-%s", os)
	}
	if fips {
		buildboxName += "-fips"
	}
	return step{
		Name:  buildboxName,
		Image: "docker",
		Environment: map[string]value{
			"QUAYIO_DOCKER_USERNAME": {fromSecret: "QUAYIO_DOCKER_USERNAME"},
			"QUAYIO_DOCKER_PASSWORD": {fromSecret: "QUAYIO_DOCKER_PASSWORD"},
		},
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			`apk add --no-cache make`,
			`chown -R $UID:$GID /go`,
			`docker login -u="$$QUAYIO_DOCKER_USERNAME" -p="$$QUAYIO_DOCKER_PASSWORD" quay.io`,
			fmt.Sprintf(`make -C build.assets %s`, buildboxName),
			fmt.Sprintf(`docker push quay.io/gravitational/teleport-%s:$BUILDBOX_VERSION`, buildboxName),
		},
	}
}

func buildboxPipeline() pipeline {
	p := newKubePipeline("build-buildboxes")
	p.Environment = map[string]value{
		"BUILDBOX_VERSION": buildboxVersion,
		"UID":              {raw: "1000"},
		"GID":              {raw: "1000"},
	}
	p.Trigger = triggerTag
	p.Workspace = workspace{Path: "/go/src/github.com/gravitational/teleport"}
	p.Volumes = dockerVolumes()
	p.Services = []service{
		dockerService(),
	}
	p.Steps = buildboxPipelineSteps()
	return p
}
