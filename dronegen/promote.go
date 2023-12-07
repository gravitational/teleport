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

import "time"

func promoteBuildPipelines() []pipeline {
	promotePipelines := make([]pipeline, 0)
	promotePipelines = append(promotePipelines, promoteBuildOsRepoPipeline())

	ociPipeline := ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerPromote,
		pipelineName: "promote-teleport-oci-distroless-images",
		workflows: []ghaWorkflow{
			{
				name:              "promote-teleport-oci-distroless.yml",
				timeout:           150 * time.Minute,
				ref:               "${DRONE_TAG}",
				shouldTagWorkflow: true,
				inputs: map[string]string{
					"release-source-tag": "${DRONE_TAG}",
				},
			},
		},
	})
	ociPipeline.Trigger.Target.Include = append(ociPipeline.Trigger.Target.Include, "promote-distroless")
	promotePipelines = append(promotePipelines, ociPipeline)

	amiPipeline := ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerPromote,
		pipelineName: "promote-teleport-hardened-amis",
		workflows: []ghaWorkflow{
			{
				name:              "promote-teleport-hardened-amis.yaml",
				timeout:           150 * time.Minute,
				ref:               "${DRONE_TAG}",
				srcRefVar:         "DRONE_TAG",
				shouldTagWorkflow: true,
				inputs: map[string]string{
					"release-source-tag": "${DRONE_TAG}",
				},
			},
		},
	})
	amiPipeline.Trigger.Target.Include = append(amiPipeline.Trigger.Target.Include, "promote-hardened-amis")
	promotePipelines = append(promotePipelines, amiPipeline)

	updaterPipeline := ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerPromote,
		pipelineName: "promote-teleport-kube-agent-updater-oci-images",
		workflows: []ghaWorkflow{
			{
				name:              "promote-teleport-kube-agent-updater-oci.yml",
				timeout:           150 * time.Minute,
				ref:               "${DRONE_TAG}",
				shouldTagWorkflow: true,
				inputs: map[string]string{
					"release-source-tag": "${DRONE_TAG}",
				},
			},
		},
	})
	updaterPipeline.Trigger.Target.Include = append(updaterPipeline.Trigger.Target.Include, "promote-updater")
	promotePipelines = append(promotePipelines, updaterPipeline)

	teleportSpaceliftRunnerPipeline := ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerPromote,
		pipelineName: "promote-teleport-spacelift-runner-oci-images",
		workflows: []ghaWorkflow{
			{
				name:              "promote-teleport-spacelift-runner-oci.yml",
				timeout:           150 * time.Minute,
				ref:               "${DRONE_TAG}",
				shouldTagWorkflow: true,
				inputs: map[string]string{
					"release-source-tag": "${DRONE_TAG}",
				},
			},
		},
	})
	teleportSpaceliftRunnerPipeline.Trigger.Target.Include = append(teleportSpaceliftRunnerPipeline.Trigger.Target.Include, "promote-teleport-spacelift-runner")
	promotePipelines = append(promotePipelines, teleportSpaceliftRunnerPipeline)

	return promotePipelines
}

func publishReleasePipeline() pipeline {
	p := relcliPipeline(triggerPromote, "publish-rlz", "Publish in Release API", "auto_publish -f -v 6")

	p.DependsOn = []string{"promote-build"} // Manually written pipeline

	for _, dep := range buildContainerImagePipelines() {
		for _, event := range dep.Trigger.Event.Include {
			if event == "promote" {
				p.DependsOn = append(p.DependsOn, dep.Name)
			}
		}
	}

	for _, dep := range promoteBuildPipelines() {
		p.DependsOn = append(p.DependsOn, dep.Name)
	}

	return p
}
