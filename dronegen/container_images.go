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
	"strings"
)

// *************************************************************
// ****** These need to be updated on each major release. ******
// ****** After updating, "make dronegen" must be reran.  ******
// *************************************************************
// These should match up when a feature branch is cut, but should be off by
// one on master
const branchMajorVersion int = 14
const latestReleaseVersion int = 13

func buildPipelineVersions() (string, []string) {
	branchMajorSemver := fmt.Sprintf("v%d", branchMajorVersion)
	// Note that this only matters in the context of the master branch
	updateVersionCount := 3
	imageUpdateSemvers := make([]string, updateVersionCount)
	for i := 0; i < updateVersionCount; i++ {
		imageUpdateSemvers[i] = fmt.Sprintf("v%d", latestReleaseVersion-i)
	}

	return branchMajorSemver, imageUpdateSemvers
}

func buildContainerImagePipelines() []pipeline {
	branchMajorSemver, imageUpdateSemvers := buildPipelineVersions()

	triggers := []*TriggerInfo{
		NewTagTrigger(branchMajorSemver),
		NewPromoteTrigger(branchMajorSemver),
		NewCronTrigger(imageUpdateSemvers),
	}

	if configureForPRTestingOnly {
		triggers = append(triggers, NewTestTrigger(prBranch, branchMajorSemver))
	}

	pipelines := make([]pipeline, 0, len(triggers))
	for _, trigger := range triggers {
		pipelines = append(pipelines, trigger.buildPipelines()...)
	}

	return pipelines
}

// Describes a container image. Used for both local and remove images.
type Image struct {
	Repo *ContainerRepo
	Name string
	Tag  *ImageTag
}

func (i *Image) GetShellName() string {
	repo := strings.TrimSuffix(i.Repo.RegistryDomain, "/")
	if i.Repo.RegistryOrg != "" {
		repo = fmt.Sprintf("%s/%s", repo, i.Repo.RegistryOrg)
	}
	return fmt.Sprintf("%s/%s:%s", repo, i.Name, i.Tag.GetShellValue())
}

func (i *Image) GetDisplayName() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag.GetDisplayValue())
}

// Contains information about the tag portion of an image.
type ImageTag struct {
	ShellBaseValue   string // Should evaluate in a shell context to the tag's value
	DisplayBaseValue string // Should be set to a human-readable version of ShellTag
	Arch             string
	IsImmutable      bool
	IsForFullSemver  bool // True if the image tag contains a full semver
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
// This is used internally to pass information around
type buildStepOutput struct {
	StepName   string
	BuiltImage *Image
	Version    *ReleaseVersion
	Product    *Product
}
