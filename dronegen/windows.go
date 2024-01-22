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
	"time"
)

func ghaWindowsPushPipeline() pipeline {
	return getWindowsPipeline(triggerPush, "push", "DRONE_BRANCH")
}

func windowsTagPipelineGHA() pipeline {
	return getWindowsPipeline(triggerTag, "tag", "DRONE_TAG")
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
					srcRefVar:         reference,
					ref:               fmt.Sprintf("${%s}", reference),
					shouldTagWorkflow: true,
				},
			},
		},
	)
}
