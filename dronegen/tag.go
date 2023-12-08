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
	"strconv"
	"time"
)

const (
	// tagCleanupPipelineName is the name of the pipeline that cleans up
	// artifacts from a previous partially-failed build
	tagCleanupPipelineName = "clean-up-previous-build"
)

const releasesHost = "https://releases-prod.platform.teleport.sh"

// tagPipelines builds all applicable tag pipeline combinations
func tagPipelines() []pipeline {
	var ps []pipeline

	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "amd64", fips: false, centos7: true, buildConnect: true, buildOSPkg: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "amd64", fips: true, centos7: true, buildConnect: false, buildOSPkg: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "386", buildOSPkg: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "arm64", buildOSPkg: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "arm", buildOSPkg: true}))

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-oci-distroless-images",
		dependsOn: []string{
			tagCleanupPipelineName,
			"build-linux-amd64",
			"build-linux-amd64-fips",
			"build-linux-arm64",
			"build-linux-arm",
		},
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-oci-distroless.yml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
			},
		},
	}))

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-hardened-amis",
		dependsOn: []string{
			tagCleanupPipelineName,
			"build-linux-amd64",
			"build-linux-amd64-fips",
		},
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-hardened-amis.yaml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
			},
		},
	}))

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-kube-agent-updater-oci-images",
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-kube-agent-updater-oci.yml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
			},
		},
	}))

	ps = append(ps, darwinTagPipelineGHA())
	ps = append(ps, windowsTagPipelineGHA())

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		pipelineName: "build-legacy-amis",
		trigger:      triggerTag,
		buildType:    buildType{fips: false},
		dependsOn: []string{
			"build-linux-amd64",
			"build-linux-amd64-fips",
		},
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-legacy-amis.yaml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
				slackOnError:      true,
			},
		},
	}))

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		pipelineName: "build-oci",
		trigger:      triggerTag,
		buildType:    buildType{fips: false},
		dependsOn: []string{
			"build-linux-amd64",
			"build-linux-amd64-fips",
			"build-linux-arm64",
			"build-linux-arm",
		},
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-oci.yaml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
				slackOnError:      true,
			},
		},
	}))

	ps = append(ps, tagCleanupPipeline())
	return ps
}

// ghaLinuxTagPipeline generates a tag pipeline for a given combination of
// os/arch/FIPS that calls a GitHub Actions workflow to perform the build on a
// Linux box. This dispatches to the release-linux.yaml workflow in the
// teleport.e repo, which is a little more generic than the
// release-linux-arm64.yml workflow used for the arm64 build. The two will be
// unified shortly.
func ghaLinuxTagPipeline(b buildType) pipeline {
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	pipelineName := fmt.Sprintf("build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName += "-fips"
	}
	wf := ghaWorkflow{
		name:              "release-linux.yaml",
		timeout:           150 * time.Minute,
		slackOnError:      true,
		srcRefVar:         "DRONE_TAG",
		ref:               "${DRONE_TAG}",
		shouldTagWorkflow: true,
		inputs: map[string]string{
			"release-artifacts": "true",
			"release-target":    releaseMakefileTarget(b),
			"build-connect":     strconv.FormatBool(b.buildConnect),
			"build-os-packages": strconv.FormatBool(b.buildOSPkg),
		},
	}
	bt := ghaBuildType{
		buildType:    buildType{os: b.os, arch: b.arch},
		trigger:      triggerTag,
		pipelineName: pipelineName,
		workflows:    []ghaWorkflow{wf},
	}
	return ghaBuildPipeline(bt)
}

func tagCleanupPipeline() pipeline {
	return relcliPipeline(triggerTag, tagCleanupPipelineName, "Clean up previously built artifacts", "auto_destroy -f -v 6")
}
