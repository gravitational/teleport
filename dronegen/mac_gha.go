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

// darwinTagPipelineGHA returns a pipeline that kicks off a tagged build of
// the Mac (darwin) release assets on GitHub Actions. The action builds:
// * a tarball of signed teleport binaries (teleport, tsh, tctl, tbot).
// * a package with the Teleport binaries (teleport, tsh, tctl, tbot).
// * a package with the tsh binary.
// * a disk image (dmg) of Teleport Connect containing the signed tsh package.
// These build assets are signed and notarized.
// The tarballs are build for amd64, arm64 and universal. The packages and
// disk image are build for universal only.
func darwinTagPipelineGHA() pipeline {
	bt := ghaBuildType{
		buildType:    buildType{os: "darwin", arch: "amd64"},
		trigger:      triggerTag,
		pipelineName: "build-darwin-amd64",
		workflows: []ghaWorkflow{
			{
				name:              "release-mac.yaml",
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
