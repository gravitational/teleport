package main

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

type releaseVersion struct {
	MajorVersion        string // This is the major version of a given build. `SearchVersion` should match this when evaluated.
	ShellVersion        string // This value will be evaluated by the shell in the context of a Drone step
	RelativeVersionName string // The set of values for this should not change between major releases
	SetupSteps          []step // Version-specific steps that must be ran before executing build and push steps
}

func (rv *releaseVersion) buildVersionPipeline(triggerSetupSteps []step, flags *TriggerFlags) pipeline {
	pipelineName := fmt.Sprintf("teleport-container-images-%s", rv.RelativeVersionName)

	setupSteps, dependentStepNames := rv.getSetupStepInformation(triggerSetupSteps)

	pipeline := newKubePipeline(pipelineName)
	pipeline.Workspace = workspace{Path: "/go"}
	pipeline.Services = []service{
		dockerService(),
		dockerRegistryService(),
	}
	pipeline.Volumes = dockerVolumes()
	pipeline.Environment = map[string]value{
		"DEBIAN_FRONTEND": {
			raw: "noninteractive",
		},
	}
	pipeline.Steps = append(setupSteps, rv.buildSteps(dependentStepNames, flags)...)

	return pipeline
}

func (rv *releaseVersion) getSetupStepInformation(triggerSetupSteps []step) ([]step, []string) {
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

func (rv *releaseVersion) buildSteps(setupStepNames []string, flags *TriggerFlags) []step {
	clonedRepoPath := "/go/src/github.com/gravitational/teleport"
	steps := make([]step, 0)

	setupSteps := []step{
		waitForDockerStep(),
		waitForDockerRegistryStep(),
		cloneRepoStep(clonedRepoPath, rv.ShellVersion),
		rv.buildSplitSemverSteps(),
	}
	for _, setupStep := range setupSteps {
		setupStep.DependsOn = append(setupStep.DependsOn, setupStepNames...)
		steps = append(steps, setupStep)
		setupStepNames = append(setupStepNames, setupStep.Name)
	}

	for _, product := range rv.getProducts(clonedRepoPath) {
		steps = append(steps, product.buildSteps(rv, setupStepNames, flags)...)
	}

	return steps
}

type semver struct {
	Name        string // Human-readable name for the information contained in the semver, i.e. "major"
	FilePath    string // The path under the working dir where the information can be read from
	FieldCount  int    // The number of significant version fields available in the semver i.e. "v11" -> 1
	IsImmutable bool
}

func (rv *releaseVersion) getSemvers() []*semver {
	varDirectory := "/go/var"
	return []*semver{
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
		{
			Name:        "canonical",
			FilePath:    path.Join(varDirectory, "canonical-version"),
			FieldCount:  3,
			IsImmutable: true,
		},
	}
}

func (rv *releaseVersion) buildSplitSemverSteps() step {
	semvers := rv.getSemvers()

	commands := make([]string, 0, len(semvers))
	for _, semver := range semvers {
		// Ex: semver.FieldCount = 3, cutFieldString = "1,2,3"
		cutFieldStrings := make([]string, 0, semver.FieldCount)
		for i := 1; i <= semver.FieldCount; i++ {
			cutFieldStrings = append(cutFieldStrings, strconv.Itoa(i))
		}
		cutFieldString := strings.Join(cutFieldStrings, ",")

		commands = append(commands,
			fmt.Sprintf("echo %s | sed 's/v//' | cut -d'.' -f %q > %q", rv.ShellVersion, cutFieldString, semver.FilePath),
		)
	}

	return step{
		Name:     "Build major, minor, and canonical semver",
		Image:    "alpine",
		Commands: commands,
	}
}

func (rv *releaseVersion) getProducts(clonedRepoPath string) []*Product {
	teleportOperatorProduct := NewTeleportOperatorProduct(clonedRepoPath)

	products := make([]*Product, 0, 1)
	products = append(products, teleportOperatorProduct)

	return products
}

func (rv *releaseVersion) getTagsForVersion() []*ImageTag {
	semvers := rv.getSemvers()
	imageTags := make([]*ImageTag, 0, len(semvers))
	for _, semver := range semvers {
		imageTags = append(imageTags, &ImageTag{
			ShellBaseValue:   fmt.Sprintf("$(cat %s)", semver.FilePath),
			DisplayBaseValue: semver.Name,
			IsImmutable:      semver.IsImmutable,
		})
	}

	return imageTags
}
