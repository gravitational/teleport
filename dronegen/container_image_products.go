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
	"regexp"
)

// Describes a Gravitational "product", where a "product" is a piece of software
// that we provide to our customers via container repositories.
type Product struct {
	Name                 string
	DockerfilePath       string
	WorkingDirectory     string                                          // Working directory to use for "docker build".
	DockerfileTarget     string                                          // Optional. Defines a dockerfile target to stop at on build.
	SupportedArchs       []string                                        // ISAs that the builder should produce
	SetupSteps           []step                                          // Product-specific steps that must be ran before building an image.
	DockerfileArgBuilder func(arch string) []string                      // Generator that returns "docker build --arg" strings
	ImageBuilder         func(repo *ContainerRepo, tag *ImageTag) *Image // Generator that returns an Image struct that defines what "docker build" should produce
	GetRequiredStepNames func(arch string) []string                      // Generator that returns the name of the steps that "docker build" should wait for
}

func NewTeleportOperatorProduct(cloneDirectory string) *Product {
	name := "teleport-operator"
	return &Product{
		Name:             name,
		DockerfilePath:   path.Join(cloneDirectory, "operator", "Dockerfile"),
		WorkingDirectory: cloneDirectory,
		SupportedArchs:   []string{"amd64", "arm", "arm64"},
		ImageBuilder: func(repo *ContainerRepo, tag *ImageTag) *Image {
			return &Image{
				Repo: repo,
				Name: name,
				Tag:  tag,
			}
		},
		DockerfileArgBuilder: func(arch string) []string {
			buildboxName := fmt.Sprintf("%s/gravitational/teleport-buildbox", ProductionRegistry)
			compilerName := ""
			switch arch {
			case "x86_64", "amd64":
				compilerName = "x86_64-linux-gnu-gcc"
			case "i686", "i386":
				compilerName = "i686-linux-gnu-gcc"
			case "arm64", "aarch64":
				buildboxName += "-arm"
				compilerName = "aarch64-linux-gnu-gcc"
			// We may want to add additional arm ISAs in the future to support devices without hardware FPUs
			case "armhf":
			case "arm":
				buildboxName += "-arm"
				compilerName = "arm-linux-gnueabihf-gcc"
			}

			buildboxName += ":teleport11"

			return []string{
				fmt.Sprintf("BUILDBOX=%s", buildboxName),
				fmt.Sprintf("COMPILER_NAME=%s", compilerName),
			}
		},
	}
}

func (p *Product) getBaseImage(arch string, version *ReleaseVersion) *Image {
	return &Image{
		Name: p.Name,
		Tag: &ImageTag{
			ShellBaseValue:   version.GetFullSemver().GetSemverValue(),
			DisplayBaseValue: version.MajorVersion,
			Arch:             arch,
		},
	}
}

func (p *Product) GetLocalRegistryImage(arch string, version *ReleaseVersion) *Image {
	image := p.getBaseImage(arch, version)
	image.Repo = NewLocalContainerRepo()

	return image
}

func (p *Product) GetStagingRegistryImage(arch string, version *ReleaseVersion, stagingRepo *ContainerRepo) *Image {
	image := p.getBaseImage(arch, version)
	image.Repo = stagingRepo

	return image
}

func (p *Product) buildSteps(version *ReleaseVersion, setupStepNames []string, flags *TriggerFlags) []step {
	steps := make([]step, 0)

	stagingRepo := GetStagingContainerRepo(flags.UseUniqueStagingTag)
	productionRepos := GetProductionContainerRepos()

	for _, setupStep := range p.SetupSteps {
		setupStep.DependsOn = append(setupStep.DependsOn, setupStepNames...)
		steps = append(steps, setupStep)
		setupStepNames = append(setupStepNames, setupStep.Name)
	}

	archBuildStepDetails := make([]*buildStepOutput, 0, len(p.SupportedArchs))

	for i, supportedArch := range p.SupportedArchs {
		// Include steps for building images from scratch
		if flags.ShouldBuildNewImages {
			archBuildStep, archBuildStepDetail := p.createBuildStep(supportedArch, version, i)

			archBuildStep.DependsOn = append(archBuildStep.DependsOn, setupStepNames...)
			if p.GetRequiredStepNames != nil {
				archBuildStep.DependsOn = append(archBuildStep.DependsOn, p.GetRequiredStepNames(supportedArch)...)
			}

			steps = append(steps, archBuildStep)
			archBuildStepDetails = append(archBuildStepDetails, archBuildStepDetail)
		} else {
			stagingImage := p.GetStagingRegistryImage(supportedArch, version, stagingRepo)
			pullStagingImageStep, locallyPushedImage := stagingRepo.pullPushStep(stagingImage, setupStepNames)
			steps = append(steps, pullStagingImageStep)

			// Generate build details that point to the pulled staging images
			archBuildStepDetails = append(archBuildStepDetails, &buildStepOutput{
				StepName:   pullStagingImageStep.Name,
				BuiltImage: locallyPushedImage,
				Version:    version,
				Product:    p,
			})
		}
	}

	for _, containerRepo := range getReposToPublishTo(productionRepos, stagingRepo, flags) {
		steps = append(steps, containerRepo.buildSteps(archBuildStepDetails, flags)...)
	}

	return steps
}

func getReposToPublishTo(productionRepos []*ContainerRepo, stagingRepo *ContainerRepo, flags *TriggerFlags) []*ContainerRepo {
	stagingRepos := []*ContainerRepo{stagingRepo}

	if flags.ShouldAffectProductionImages {
		if !flags.ShouldBuildNewImages {
			// In this case the images will be pulled from staging and therefor should not be re-published
			// to staging
			return productionRepos
		}

		return append(stagingRepos, productionRepos...)
	}

	return stagingRepos
}

func (p *Product) GetBuildStepName(arch string, version *ReleaseVersion) string {
	localImageName := p.GetLocalRegistryImage(arch, version)
	return fmt.Sprintf("Build %s image %q", p.Name, localImageName.GetDisplayName())
}

func cleanBuilderName(builderName string) string {
	var invalidBuildxCharExpression = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	return invalidBuildxCharExpression.ReplaceAllString(builderName, "-")
}

func (p *Product) createBuildStep(arch string, version *ReleaseVersion, delay int) (step, *buildStepOutput) {
	localRegistryImage := p.GetLocalRegistryImage(arch, version)
	builderName := cleanBuilderName(fmt.Sprintf("%s-builder", localRegistryImage.GetDisplayName()))

	buildxConfigFileDir := path.Join("/tmp", builderName)
	buildxConfigFilePath := path.Join(buildxConfigFileDir, "buildkitd.toml")

	buildxCreateCommand := "docker buildx create"
	buildxCreateCommand += fmt.Sprintf(" --driver %q", "docker-container")
	// This is set so that buildx can reach the local registry
	buildxCreateCommand += fmt.Sprintf(" --driver-opt %q", "network=host")
	buildxCreateCommand += fmt.Sprintf(" --name %q", builderName)
	buildxCreateCommand += fmt.Sprintf(" --config %q", buildxConfigFilePath)

	buildCommand := "docker buildx build"
	buildCommand += " --push"
	buildCommand += fmt.Sprintf(" --builder %q", builderName)
	if p.DockerfileTarget != "" {
		buildCommand += fmt.Sprintf(" --target %q", p.DockerfileTarget)
	}
	buildCommand += fmt.Sprintf(" --platform %q", "linux/"+arch)
	buildCommand += fmt.Sprintf(" --tag %s", localRegistryImage.GetShellName())
	buildCommand += fmt.Sprintf(" --file %q", p.DockerfilePath)
	if p.DockerfileArgBuilder != nil {
		for _, buildArg := range p.DockerfileArgBuilder(arch) {
			buildCommand += fmt.Sprintf(" --build-arg %q", buildArg)
		}
	}
	buildCommand += " " + p.WorkingDirectory

	delayTime := delay * 5

	step := step{
		Name:    p.GetBuildStepName(arch, version),
		Image:   "docker",
		Volumes: dockerVolumeRefs(),
		Environment: map[string]value{
			"DOCKER_BUILDKIT": {
				raw: "1",
			},
		},
		Commands: []string{
			// Without a delay buildx can occasionally try to pull base images faster than container registries will allow,
			// triggering a rate limit.
			fmt.Sprintf("echo 'Sleeping %ds to avoid registry pull rate limits' && sleep %d", delayTime, delayTime),
			"docker run --privileged --rm tonistiigi/binfmt --install all",
			fmt.Sprintf("mkdir -pv %q && cd %q", p.WorkingDirectory, p.WorkingDirectory),
			fmt.Sprintf("mkdir -pv %q", buildxConfigFileDir),
			fmt.Sprintf("echo '[registry.%q]' > %q", LocalRegistrySocket, buildxConfigFilePath),
			fmt.Sprintf("echo '  http = true' >> %q", buildxConfigFilePath),
			buildxCreateCommand,
			buildCommand,
			fmt.Sprintf("docker buildx rm %q", builderName),
			fmt.Sprintf("rm -rf %q", buildxConfigFileDir),
		},
	}

	return step, &buildStepOutput{
		StepName:   step.Name,
		BuiltImage: localRegistryImage,
		Version:    version,
		Product:    p,
	}
}
