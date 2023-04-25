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

func promoteBuildPipelines() []pipeline {
	promotePipelines := make([]pipeline, 0)
	promotePipelines = append(promotePipelines, promoteBuildOsRepoPipeline())

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
