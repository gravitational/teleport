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

// To run one of these pipelines locally:
// # Drone requires certain variables to be set
// export DRONE_REMOTE_URL="https://github.com/gravitational/teleport"
// export DRONE_SOURCE_BRANCH="$(git branch --show-current)"
// # `drone exec` does not support `exec` or `kubernetes` pipelines
// sed -i '' 's/type\: kubernetes/type\: docker/' .drone.yml && sed -i '' 's/type\: exec/type\: docker/' .drone.yml
// # Drone has a bug where "workspace" is appended to "/drone/src". This fixes that by updating references
// sed -i '' 's~/go/~/drone/src/go/~g' .drone.yml
// # Pull the current branch instead of v11
// sed -i '' "s~git checkout -qf \"\$(cat '/go/vars/full-version/v11')\"~git checkout -qf \"${DRONE_SOURCE_BRANCH}\"~" .drone.yml
// # `drone exec` does not properly map the workspace path. This creates a volume to be shared between steps
// #  at the correct path
// DOCKER_VOLUME_NAME="go"
// docker volume create "$DOCKER_VOLUME_NAME"
// drone exec --trusted --pipeline teleport-container-images-current-version-cron --clone=false --volume "${DOCKER_VOLUME_NAME}:/go"
// # Cleanup
// docker volume rm "$DOCKER_VOLUME_NAME"

import (
	"fmt"
	"strings"
)

func buildContainerImagePipelines() []pipeline {
	// *************************************************************
	// ****** These need to be updated on each major release. ******
	// *************************************************************
	latestMajorVersions := []string{"v11", "v10", "v9"}
	branchMajorVersion := "v11"

	triggers := []*TriggerInfo{
		NewTagTrigger(branchMajorVersion),
		NewPromoteTrigger(branchMajorVersion),
		NewCronTrigger(latestMajorVersions),
	}

	if configureForPRTestingOnly {
		triggers = append(triggers, NewTestTrigger(prBranch, branchMajorVersion))
	}

	pipelines := make([]pipeline, 0, len(triggers))
	for _, trigger := range triggers {
		pipelines = append(pipelines, trigger.buildPipelines()...)
	}

	return pipelines
}

type Image struct {
	Repo string
	Name string
	Tag  *ImageTag
}

func (i *Image) GetShellName() string {
	repo := ""
	if !i.IsLocalImage() {
		// Ensure one and only one "/"
		repo = strings.TrimSuffix(i.Repo, "/")
		repo += "/"
	}

	return fmt.Sprintf("%s%s:%s", repo, i.Name, i.Tag.GetShellValue())
}

func (i *Image) GetDisplayName() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag.GetDisplayValue())
}

func (i *Image) IsLocalImage() bool {
	return i.Repo == ""
}

type ImageTag struct {
	ShellBaseValue   string // Should evaluate in a shell context to the tag's value
	DisplayBaseValue string // Should be set to a human-readable version of ShellTag
	Arch             string
	IsImmutable      bool
}

func NewLatestTag() *ImageTag {
	return &ImageTag{
		ShellBaseValue:   "latest",
		DisplayBaseValue: "latest",
	}
}

func (it *ImageTag) AppendString(s string) {
	it.ShellBaseValue += fmt.Sprintf("-%s", s)
	it.DisplayBaseValue += fmt.Sprintf("-%s", s)
}

func (it *ImageTag) IsMultArch() bool {
	return it.Arch != ""
}

func (it *ImageTag) GetShellValue() string {
	return it.getValue(it.ShellBaseValue)
}

func (it *ImageTag) GetDisplayValue() string {
	return it.getValue(it.DisplayBaseValue)
}

func (it *ImageTag) getValue(baseValue string) string {
	if it.Arch == "" {
		return baseValue
	}

	return fmt.Sprintf("%s-%s", baseValue, it.Arch)
}

// The `step` struct doesn't contain enough information to setup
// dependent steps so we add that via this struct
type buildStepOutput struct {
	StepName   string
	BuiltImage *Image
	Version    *releaseVersion
	Product    *Product
}
