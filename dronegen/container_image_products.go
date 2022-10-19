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
	"strings"
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

func NewTeleportProduct(isEnterprise, isFips bool, version *ReleaseVersion) *Product {
	workingDirectory := "/go/build"
	downloadURL := "https://raw.githubusercontent.com/gravitational/teleport/${DRONE_SOURCE_BRANCH:-master}/build.assets/charts/Dockerfile"
	name := "teleport"
	dockerfileTarget := "teleport"
	supportedArches := []string{"amd64"}

	if isEnterprise {
		name += "-ent"
	}
	if isFips {
		dockerfileTarget += "-fips"
		name += "-fips"
	} else {
		supportedArches = append(supportedArches, "arm", "arm64")
	}

	setupStep, debPaths, dockerfilePath := teleportSetupStep(version.ShellVersion, name, workingDirectory, downloadURL, supportedArches)

	return &Product{
		Name:             name,
		DockerfilePath:   dockerfilePath,
		WorkingDirectory: workingDirectory,
		DockerfileTarget: dockerfileTarget,
		SupportedArchs:   supportedArches,
		SetupSteps:       []step{setupStep},
		DockerfileArgBuilder: func(arch string) []string {
			return []string{
				fmt.Sprintf("DEB_PATH=%s", debPaths[arch]),
			}
		},
		ImageBuilder: func(repo *ContainerRepo, tag *ImageTag) *Image {
			imageProductName := "teleport"
			if isEnterprise {
				imageProductName += "-ent"
			}

			if isFips {
				tag.AppendString("fips")
			}

			return &Image{
				Repo: repo,
				Name: imageProductName,
				Tag:  tag,
			}
		},
	}
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

func teleportSetupStep(shellVersion, packageName, workingPath, downloadURL string, archs []string) (step, map[string]string, string) {
	keyPath := "/usr/share/keyrings/teleport-archive-keyring.asc"
	downloadDirectory := "/tmp/apt-download"
	timeout := 30 * 60 // 30 minutes in seconds
	sleepTime := 15    // 15 seconds
	dockerfilePath := path.Join(workingPath, "Dockerfile")

	commands := []string{
		// Setup the environment
		fmt.Sprintf("PACKAGE_NAME=%q", packageName),
		fmt.Sprintf("PACKAGE_VERSION=%q", shellVersion),
		"apt update",
		"apt install --no-install-recommends -y ca-certificates curl",
		"update-ca-certificates",
		// Download the dockerfile
		fmt.Sprintf("mkdir -pv $(dirname %q)", dockerfilePath),
		fmt.Sprintf("curl -Ls -o %q %q", dockerfilePath, downloadURL),
		// Add the Teleport APT repo
		fmt.Sprintf("curl https://apt.releases.teleport.dev/gpg -o %q", keyPath),
		". /etc/os-release",
		// Per https://docs.drone.io/pipeline/environment/syntax/#common-problems I'm using '$$' here to ensure
		// That the shell variable is not expanded until runtime, preventing drone from erroring on the
		// drone-unsupported '?'
		"MAJOR_VERSION=$(echo $${PACKAGE_VERSION?} | cut -d'.' -f 1)",
		fmt.Sprintf("echo \"deb [signed-by=%s] https://apt.releases.teleport.dev/$${ID?} $${VERSION_CODENAME?} stable/$${MAJOR_VERSION?}\""+
			" > /etc/apt/sources.list.d/teleport.list", keyPath),
		fmt.Sprintf("END_TIME=$(( $(date +%%s) + %d ))", timeout),
		"TRIMMED_VERSION=$(echo $${PACKAGE_VERSION} | cut -d'v' -f 2)",
		"TIMED_OUT=true",
		// Poll APT until the timeout is reached or the package becomes available
		"while [ $(date +%s) -lt $${END_TIME?} ]; do",
		"echo 'Running apt update...'",
		// This will error on new major versions where the "stable/$${MAJOR_VERSION}" component doesn't exist yet, so we ignore it here.
		"apt update > /dev/null || true",
		"[ $(apt-cache madison $${PACKAGE_NAME} | grep $${TRIMMED_VERSION?} | wc -l) -ge 1 ] && TIMED_OUT=false && break;",
		fmt.Sprintf("echo 'Package not found yet, waiting another %d seconds...'", sleepTime),
		fmt.Sprintf("sleep %d", sleepTime),
		"done",
		// Log success or failure and record full version string
		"[ $${TIMED_OUT?} = true ] && echo \"Timed out while looking for APT package \\\"$${PACKAGE_NAME}\\\" matching \\\"$${TRIMMED_VERSION}\\\"\" && exit 1",
		"FULL_VERSION=$(apt-cache madison $${PACKAGE_NAME} | grep $${TRIMMED_VERSION} | cut -d'|' -f 2 | tr -d ' ' | head -n 1)",
		fmt.Sprintf("echo \"Found APT package, downloading \\\"$${PACKAGE_NAME}=$${FULL_VERSION}\\\" for %q...\"", strings.Join(archs, "\", \"")),
		fmt.Sprintf("mkdir -pv %q", downloadDirectory),
		fmt.Sprintf("cd %q", downloadDirectory),
	}

	for _, arch := range archs {
		// Our built debs are listed as ISA "armhf" not "arm", so we account for that here
		if arch == "arm" {
			arch = "armhf"
		}

		commands = append(commands, []string{
			// This will allow APT to download other architectures
			fmt.Sprintf("dpkg --add-architecture %q", arch),
		}...)
	}

	// This will error due to Ubuntu's APT repo structure but it doesn't matter here
	commands = append(commands, "apt update &> /dev/null || true")

	archDestFileMap := make(map[string]string, len(archs))
	for _, arch := range archs {
		relArchDir := path.Join(".", "/artifacts/deb/", packageName, arch)
		archDir := path.Join(workingPath, relArchDir)
		// Example: `./artifacts/deb/teleport-ent/arm64/v10.1.4.deb`
		relDestPath := path.Join(relArchDir, fmt.Sprintf("%s.deb", shellVersion))
		// Example: `/go/./artifacts/deb/teleport-ent/arm64/v10.1.4.deb`
		destPath := path.Join(workingPath, relDestPath)

		archDestFileMap[arch] = relDestPath

		// Our built debs are listed as ISA "armhf" not "arm", so we account for that here
		if arch == "arm" {
			arch = "armhf"
		}

		// This could probably be parallelized to slightly reduce runtime
		fullPackageName := fmt.Sprintf("%s:%s=$${FULL_VERSION}", packageName, arch)
		commands = append(commands, []string{
			fmt.Sprintf("mkdir -pv %q", archDir),
			fmt.Sprintf("apt download %q", fullPackageName),
			"FILENAME=$(ls)", // This will only return the download file as it is the only file in that directory
			"echo \"Downloaded file \\\"$${FILENAME}\\\"\"",
			fmt.Sprintf("mv \"$${FILENAME}\" %q", path.Join(archDir, "$${PACKAGE_VERSION}.deb")),
			fmt.Sprintf("echo Downloaded %q to %q", fullPackageName, destPath),
		}...)
	}

	return step{
		Name:     fmt.Sprintf("Download %q Dockerfile and DEB artifacts from APT", packageName),
		Image:    "ubuntu:22.04",
		Commands: commands,
	}, archDestFileMap, dockerfilePath
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
