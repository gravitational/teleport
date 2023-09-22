// Copyright 2023 Gravitational, Inc
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

import "time"

// darwinTagPipelineGHA returns a pipeline that kicks off a tagged build of
// the Mac (darwin) release assets on GitHub Actions. The action builds:
// * a tarball of signed teleport binaries (teleport, tsh, tctl, tbot).
// * a package with the Teleport binaries (teleport, tsh, tctl, tbot).
// * a package with the tsh binary.
// * a disk image (dmg) of Teleport Connect containing the signed tsh package.
// These build assets are signed and notarized.
func darwinTagPipelineGHA() pipeline {
	bt := ghaBuildType{
		buildType:    buildType{os: "darwin", arch: "amd64"},
		trigger:      triggerTag,
		pipelineName: "build-darwin-amd64",
		workflows: []ghaWorkflow{
			{
				name:              "release-mac-amd64.yaml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				slackOnError:      true,
				shouldTagWorkflow: true,
				inputs: map[string]string{
					"release-artifacts": "true",
					"build-packages":    "true",
				},
			},
		},
	}
	return ghaBuildPipeline(bt)
}

// darwinPushPipelineGHA returns a pipeline that kicks off a push build of the
// teleport binaries and the teleport connect dmg. The binaries are signed and
// notarized even though we do not release these assets. This tests that the
// signing and notarization process continues to work so we don't wait until
// release time to discover breakage.
func darwinPushPipelineGHA() pipeline {
	bt := ghaBuildType{
		buildType:    buildType{os: "darwin", arch: "amd64"},
		trigger:      triggerPush,
		pipelineName: "push-build-darwin-amd64",
		workflows: []ghaWorkflow{
			{
				name:              "release-mac-amd64.yaml",
				srcRefVar:         "DRONE_COMMIT",
				ref:               "${DRONE_BRANCH}",
				timeout:           150 * time.Minute,
				slackOnError:      true,
				shouldTagWorkflow: true,
				inputs: map[string]string{
					"release-artifacts": "false",
					"build-packages":    "false",
				},
			},
		},
	}
	return ghaBuildPipeline(bt)
}
