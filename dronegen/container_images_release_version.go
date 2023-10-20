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
	"path"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

const (
	varDirectory = "/go/var"
)

// Describes a Teleport/repo release version. All product releases are tied to Teleport's release cycle
// via this struct.
type ReleaseVersion struct {
	MajorVersion        string // This is the major version of a given build. `SearchVersion` should match this when evaluated.
	ShellVersion        string // This value will be evaluated by the shell in the context of a Drone step
	ShellIsPrerelease   string // This value will be evaluated in a shell context to determine if a release version is a prerelease. Must be POSIX compliant and not rely on other external utilities.
	RelativeVersionName string // The set of values for this should not change between major releases
	SetupSteps          []step // Version-specific steps that must be ran before executing build and push steps
}

func (rv *ReleaseVersion) buildVersionPipeline(triggerSetupSteps []step, flags *TriggerFlags) pipeline {
	pipelineName := fmt.Sprintf("teleport-container-images-%s", rv.RelativeVersionName)

	setupSteps, dependentStepNames := rv.getSetupStepInformation(triggerSetupSteps)

	pipeline := newKubePipeline(pipelineName)
	pipeline.Workspace = workspace{Path: "/go"}
	pipeline.Services = []service{
		dockerService(),
		dockerRegistryService(),
	}
	pipeline.Volumes = []volume{volumeAwsConfig, volumeDocker, volumeDockerConfig}
	pipeline.Environment = map[string]value{
		"DEBIAN_FRONTEND": {
			raw: "noninteractive",
		},
	}
	pipeline.Steps = append(setupSteps, rv.buildSteps(dependentStepNames, flags)...)

	return pipeline
}

func (rv *ReleaseVersion) getSetupStepInformation(triggerSetupSteps []step) ([]step, []string) {
	triggerSetupStepNames := make([]string, 0, len(triggerSetupSteps))
	for _, triggerSetupStep := range triggerSetupSteps {
		triggerSetupStepNames = append(triggerSetupStepNames, triggerSetupStep.Name)
	}

	nextStageSetupStepNames := triggerSetupStepNames
	if len(rv.SetupSteps) > 0 {
		versionSetupStepNames := make([]string, 0, len(rv.SetupSteps))
		for _, versionSetupStep := range rv.SetupSteps {
			versionSetupStep.DependsOn = append(versionSetupStep.DependsOn, triggerSetupStepNames...)
			versionSetupStepNames = append(versionSetupStepNames, versionSetupStep.Name)
		}

		nextStageSetupStepNames = versionSetupStepNames
	}

	setupSteps := append(triggerSetupSteps, rv.SetupSteps...)

	return setupSteps, nextStageSetupStepNames
}

func (rv *ReleaseVersion) buildSteps(parentSetupStepNames []string, flags *TriggerFlags) []step {
	clonedRepoPath := "/go/src/github.com/gravitational/teleport"
	steps := make([]step, 0)

	setupSteps := []step{
		waitForDockerStep(),
		waitForDockerRegistryStep(),
		cloneRepoStep(clonedRepoPath, rv.ShellVersion),
		rv.buildSplitSemverSteps(flags.ShouldOnlyPublishFullSemver),
	}

	// These are sequential to prevent read/write contention by mounting volumes on
	// multiple containeres at once
	repos := getReposUsedByPipeline(flags)
	var previousSetupRepo *ContainerRepo
	for _, containerRepo := range repos {
		repoSetupSteps := containerRepo.SetupSteps
		if previousSetupRepo != nil {
			previousRepoStepNames := getStepNames(previousSetupRepo.SetupSteps)
			for i, repoSetupStep := range repoSetupSteps {
				repoSetupSteps[i].DependsOn = append(repoSetupStep.DependsOn, previousRepoStepNames...)
			}
		}
		setupSteps = append(setupSteps, repoSetupSteps...)

		if len(repoSetupSteps) > 0 {
			previousSetupRepo = containerRepo
		}
	}

	for _, setupStep := range setupSteps {
		setupStep.DependsOn = append(setupStep.DependsOn, parentSetupStepNames...)
		steps = append(steps, setupStep)
	}

	setupStepNames := append(parentSetupStepNames, getStepNames(setupSteps)...)

	for _, product := range rv.getProducts(clonedRepoPath) {
		if semver.Compare(rv.MajorVersion, product.MinimumSupportedMajorVersion) < 0 {
			// If the release version doesn't support the product
			continue
		}

		steps = append(steps, product.buildSteps(rv, setupStepNames, flags)...)
	}

	return steps
}

func getReposUsedByPipeline(flags *TriggerFlags) []*ContainerRepo {
	repos := []*ContainerRepo{GetStagingContainerRepo(flags.UseUniqueStagingTag)}

	if flags.ShouldBuildNewImages {
		repos = append(repos, GetPublicEcrPullRegistry())
	}

	if flags.ShouldAffectProductionImages {
		repos = append(repos, GetProductionContainerRepos()...)
	}

	return repos
}

type Semver struct {
	Name        string // Human-readable name for the information contained in the semver, i.e. "major"
	FilePath    string // The path under the working dir where the information can be read from
	FieldCount  int    // The number of significant version fields available in the semver i.e. "v11" -> 1
	IsImmutable bool
	IsFull      bool
}

func (rv *ReleaseVersion) GetSemvers() []*Semver {
	return []*Semver{
		{
			Name:        "major",
			FilePath:    path.Join(varDirectory, "major-version"),
			FieldCount:  1,
			IsImmutable: false,
		},
		{
			Name:        "minor",
			FilePath:    path.Join(varDirectory, "minor-version"),
			FieldCount:  2,
			IsImmutable: false,
		},
		rv.GetFullSemver(),
	}
}

func (rv *ReleaseVersion) GetFullSemver() *Semver {
	return &Semver{
		// For releases this is the "canonical" semver.
		// For prereleases this is canonical + metadata.
		// This is done to keep prereleases pushed to staging
		//  from overwriting release versions.
		Name:        "full",
		FilePath:    path.Join(varDirectory, "full-version"),
		IsImmutable: true,
		IsFull:      true,
	}
}

func (s *Semver) GetSemverValue() string {
	return fmt.Sprintf("$(cat %q)", s.FilePath)
}

func (rv *ReleaseVersion) buildSplitSemverSteps(onlyBuildFullSemver bool) step {
	semvers := rv.GetSemvers()

	// Build the commands that generate the semvers
	commands := make([]string, 0, len(semvers))
	stepNameVersions := make([]string, 0, len(semvers))
	for _, semver := range semvers {
		if onlyBuildFullSemver && !semver.IsFull {
			continue
		}

		commands = append(commands, fmt.Sprintf("mkdir -pv $(dirname %q)", semver.FilePath))
		if semver.IsFull {
			// Special case for full semver where only the "v" should be trimmed
			commands = append(commands, fmt.Sprintf("echo %s | sed 's/v//' > %q", rv.ShellVersion, semver.FilePath))
		} else {
			// Trim the semver metadata and some digits
			// Ex: semver.FieldCount = 3, cutFieldString = "1,2,3"
			cutFieldStrings := make([]string, 0, semver.FieldCount)
			for i := 1; i <= semver.FieldCount; i++ {
				cutFieldStrings = append(cutFieldStrings, strconv.Itoa(i))
			}
			cutFieldString := strings.Join(cutFieldStrings, ",")

			commands = append(commands, fmt.Sprintf("echo %s | sed 's/v//' | cut -d'.' -f %q > %q",
				rv.ShellVersion, cutFieldString, semver.FilePath))
		}
		// For debugging
		commands = append(commands, fmt.Sprintf("echo %s", semver.GetSemverValue()))

		stepNameVersions = append(stepNameVersions, semver.Name)
	}

	// Build the formatted, human-readable step name
	concatStepNameVersions := "Build"
	for i, stepNameVersion := range stepNameVersions {
		if i+1 < len(stepNameVersions) {
			// If not the last version name
			concatStepNameVersions = fmt.Sprintf("%s %s,", concatStepNameVersions, stepNameVersion)
		} else {
			if len(stepNameVersions) > 1 {
				concatStepNameVersions = fmt.Sprintf("%s and", concatStepNameVersions)
			}

			concatStepNameVersions = fmt.Sprintf("%s %s semver", concatStepNameVersions, stepNameVersion)
			if len(stepNameVersions) > 1 {
				concatStepNameVersions = fmt.Sprintf("%ss", concatStepNameVersions)
			}
		}
	}

	return step{
		Name:     concatStepNameVersions,
		Image:    "alpine",
		Commands: commands,
	}
}

func (rv *ReleaseVersion) getProducts(clonedRepoPath string) []*Product {
	teleportProducts := []*Product{
		NewTeleportProduct(false, false, rv), // OSS
		NewTeleportProduct(true, false, rv),  // Enterprise
		NewTeleportProduct(true, true, rv),   // Enterprise/FIPS
	}

	teleportOperatorProduct := NewTeleportOperatorProduct(clonedRepoPath)

	products := make([]*Product, 0, len(teleportProducts)+1)
	products = append(products, teleportProducts...)
	products = append(products, teleportOperatorProduct)

	return products
}

func (rv *ReleaseVersion) getTagsForVersion(onlyBuildFullSemver bool) []*ImageTag {
	semvers := rv.GetSemvers()
	imageTags := make([]*ImageTag, 0, len(semvers))
	for _, semver := range semvers {
		if onlyBuildFullSemver && !semver.IsFull {
			continue
		}

		imageTags = append(imageTags, &ImageTag{
			ShellBaseValue:   semver.GetSemverValue(),
			DisplayBaseValue: semver.Name,
			IsImmutable:      semver.IsImmutable,
			IsForFullSemver:  semver.IsFull,
		})
	}

	return imageTags
}
