// Copyright 2022 Gravitational, Inc
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
	"time"
)

func ghaWindowsPushPipeline() pipeline {
	return getWindowsPipeline(triggerPush, "push", "${DRONE_BRANCH}")
}

func windowsTagPipelineGHA() pipeline {
	return getWindowsPipeline(triggerTag, "tag", "${DRONE_TAG}")
}

func getWindowsPipeline(pipelineTrigger trigger, triggerName, reference string) pipeline {
	return ghaBuildPipeline(
		ghaBuildType{
			trigger:      pipelineTrigger,
			pipelineName: fmt.Sprintf("%s-build-windows-amd64", triggerName),
			workflows: []ghaWorkflow{
				{
					name:              "release-windows.yaml",
					timeout:           30 * time.Minute,
					slackOnError:      true,
					srcRefVar:         "DRONE_COMMIT",
					ref:               reference,
					shouldTagWorkflow: true,
				},
			},
		},
	)
}
