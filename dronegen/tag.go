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
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "arm64", fips: false, buildOSPkg: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "arm64", fips: true, buildOSPkg: true}))
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
			"build-linux-arm64-fips",
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
			"build-linux-arm64",
			"build-linux-arm64-fips",
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

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-spacelift-runner-oci-images",
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-spacelift-runner-oci.yml",
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
		pipelineName: "build-oci",
		trigger:      triggerTag,
		buildType:    buildType{fips: false},
		dependsOn: []string{
			"build-linux-amd64",
			"build-linux-amd64-fips",
			"build-linux-arm64",
			"build-linux-arm64-fips",
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
