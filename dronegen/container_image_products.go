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

	"golang.org/x/exp/maps"
)

// Describes a Gravitational "product", where a "product" is a piece of software
// that we provide to our customers via container repositories.
type Product struct {
	Name                         string
	DockerfilePath               string
	WorkingDirectory             string                                          // Working directory to use for "docker build".
	DockerfileTarget             string                                          // Optional. Defines a dockerfile target to stop at on build.
	SupportedArchs               []string                                        // ISAs that the builder should produce
	SetupSteps                   []step                                          // Product-specific, arch agnostic steps that must be ran before building an image.
	ArchSetupSteps               map[string][]step                               // Product and arch specific steps that must be ran before building an image. If commands are empty then they are treated as dependent steps.
	DockerfileArgBuilder         func(arch string) []string                      // Generator that returns "docker build --arg" strings
	ImageBuilder                 func(repo *ContainerRepo, tag *ImageTag) *Image // Generator that returns an Image struct that defines what "docker build" should produce
	MinimumSupportedMajorVersion string                                          // Semver of the minimum major version that the product can be built for. For example, for Teleport Lab, this would be "v10".
}

func NewTeleportProduct(isEnterprise, isFips bool, version *ReleaseVersion) *Product {
	workingDirectory := "/go/build"
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

	setupSteps, dockerfilePath, downloadProfileName := getTeleportSetupSteps(name, workingDirectory, version.ShellVersion)
	archSetupSteps, debPaths := getTeleportArchsSetupSteps(supportedArches, workingDirectory, downloadProfileName, version, isEnterprise, isFips)

	return &Product{
		Name:             name,
		DockerfilePath:   dockerfilePath,
		WorkingDirectory: workingDirectory,
		DockerfileTarget: dockerfileTarget,
		SupportedArchs:   supportedArches,
		SetupSteps:       setupSteps,
		ArchSetupSteps:   archSetupSteps,
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
		// While technically this goes back much further, this is as far back as changes will be backported.
		MinimumSupportedMajorVersion: "v9",
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
		MinimumSupportedMajorVersion: "v10",
	}
}

// Builds all the steps required to prepare the pipeline for building Teleport images.
// Returns the setup steps, the path to the downloaded Teleport dockerfile, and the name of the
// AWS profile that can be used to download artifacts from S3.
func getTeleportSetupSteps(productName, workingPath, checkoutRef string) ([]step, string, string) {
	assumeS3DownloadRoleStep, profileName := assumeS3DownloadRoleStep(productName)
	downloadDockerfileStep, dockerfilePath := downloadTeleportDockerfileStep(productName, workingPath, checkoutRef)
	// Additional setup steps in the future should go here

	return []step{assumeS3DownloadRoleStep, downloadDockerfileStep}, dockerfilePath, profileName
}

// Generates steps that download a deb for each supported arch to the working directory.
// Returns maps keyed by the supported arches, with the generated setup steps and deb paths.
func getTeleportArchsSetupSteps(supportedArchs []string, workingDirectory, profile string, version *ReleaseVersion,
	isEnterprise, isFips bool) (map[string][]step, map[string]string) {

	archSetupSteps := make(map[string][]step, len(supportedArchs))
	debPaths := make(map[string]string, len(supportedArchs))

	for _, supportedArch := range supportedArchs {
		archSetupStep, debPath := getTeleportArchSetupStep(supportedArch, workingDirectory, profile, version, isEnterprise, isFips)
		archSetupSteps[supportedArch] = []step{archSetupStep}
		debPaths[supportedArch] = debPath
	}

	return archSetupSteps, debPaths
}

// Generates steps that download a deb for each supported arch to the working directory.
// Returns the generated step, and the path to the downloaded deb.
func getTeleportArchSetupStep(arch, workingDirectory, profile string, version *ReleaseVersion, isEnterprise, isFips bool) (step, string) {
	shellDebName := buildTeleportDebName(version, arch, isEnterprise, isFips, false)
	humanDebName := buildTeleportDebName(version, arch, isEnterprise, isFips, true)
	commands := generateDownloadCommandsForArch(shellDebName, version.GetFullSemver().GetSemverValue(), workingDirectory, profile)

	downloadStep := step{
		Name:  fmt.Sprintf("Download %q artifacts from S3", humanDebName),
		Image: "amazon/aws-cli",
		Environment: map[string]value{
			"AWS_REGION":    {raw: "us-west-2"},
			"AWS_S3_BUCKET": {fromSecret: "AWS_S3_BUCKET"},
			"AWS_PROFILE":   {raw: profile},
		},
		Commands: commands,
		Volumes:  []volumeRef{volumeRefAwsConfig},
	}

	return downloadStep, shellDebName
}

// Generates the commands to download `debName` from s3 to `workingDirectory`.
// Returns the commands as well as the path where the deb will be downloaded to.
func generateDownloadCommandsForArch(debName, trimmedTag, workingDirectory, profile string) []string {
	bucketPath := fmt.Sprintf("s3://$AWS_S3_BUCKET/teleport/tag/%s/", trimmedTag)
	checkCommands := []string{
		"SUCCESS=true",
		fmt.Sprintf("aws s3 ls %s | tr -s ' ' | cut -d' ' -f 4 | grep -x %s || SUCCESS=false", bucketPath, debName),
	}
	successCommand := "[ \"$SUCCESS\" = \"true\" ]"

	remotePath := fmt.Sprintf("%s%s", bucketPath, debName)
	downloadPath := path.Join(workingDirectory, debName)

	commands := make([]string, 0)
	// Wait up to an hour for debs to be build and published to s3 by other pipelines
	commands = append(commands, wrapCommandsInTimeout(checkCommands, successCommand, 60*60, 60)...)
	commands = append(commands, fmt.Sprintf("mkdir -pv %q", workingDirectory))
	commands = append(commands, fmt.Sprintf("aws s3 cp %s %s", remotePath, downloadPath))

	return commands
}

// Returns either a human-readable or shell-evaluable Teleport deb name.
func buildTeleportDebName(version *ReleaseVersion, arch string, isEnterprise, isFips, humanReadable bool) string {
	var versionString string
	if humanReadable {
		versionString = fmt.Sprintf("%s-tag", version.MajorVersion)
	} else {
		versionString = version.GetFullSemver().GetSemverValue()
	}

	debName := "teleport"
	if isEnterprise {
		debName = fmt.Sprintf("%s-ent", debName)
	}
	debName = fmt.Sprintf("%s_%s", debName, versionString)
	if isFips {
		debName = fmt.Sprintf("%s-fips", debName)
	}
	debName = fmt.Sprintf("%s_%s.deb", debName, arch)

	return debName
}

// Creates a shell loop with a timeout
// commands: commands to run in a loop
// successCommand: should evaluate to shell true (i.e. `[ true ]`) when the loop has succeeded
// timeoutSeconds: how long in seconds to wait before the loop fails
// sleepTimeSeconds: how long to wait after every iteration before running again
func wrapCommandsInTimeout(commands []string, successCommand string, timeoutSeconds int, sleepTimeSeconds int) []string {
	setupCommands := []string{
		fmt.Sprintf("END_TIME=$(( $(date +%%s) + %d ))", timeoutSeconds),
		"TIMED_OUT=true",
		"while [ $(date +%s) -lt $${END_TIME?} ]; do",
	}

	finalizeCommands := []string{
		// Evaluate the condition
		fmt.Sprintf("%s && TIMED_OUT=false && break;", successCommand),
		// Sleep if not met
		fmt.Sprintf("echo 'Condition not met yet, waiting another %d seconds...'", sleepTimeSeconds),
		fmt.Sprintf("sleep %d", sleepTimeSeconds),
		"done",
		// Conditionally log timeout failure and exit
		fmt.Sprintf("[ $${TIMED_OUT?} = true ] && echo 'Timed out while waiting for condition: %s' && exit 1", successCommand),
	}

	loopCommands := make([]string, 0)
	loopCommands = append(loopCommands, setupCommands...)
	loopCommands = append(loopCommands, commands...)
	loopCommands = append(loopCommands, finalizeCommands...)

	return loopCommands
}

// Generates a step that downloads the Teleport Dockerfile
// Returns the generated step and the path to the downloaded Dockerfile
func downloadTeleportDockerfileStep(productName, workingPath, checkoutRef string) (step, string) {
	clonePath := "/tmp/repo"
	// Enterprise and fips specific dockerfiles should be configured here in the future if needed
	repoRelativeDockerfilePath := path.Join("build.assets", "charts", "Dockerfile")
	destinationDockerfilePath := path.Join(workingPath, fmt.Sprintf("Dockerfile-%s", productName))

	// Note that this specific method of retrieving the Dockerfile is used because Drone has
	// access to a SSH private key for GitHub, but not an API token.
	// If a simple `curl https://raw.githubusercontent.com/...` is used here it will fail on private
	// repos that require authentication. The existence of the private key handles all of this for us.

	return step{
		Name:  fmt.Sprintf("Download Teleport Dockerfile to %q for %s", destinationDockerfilePath, productName),
		Image: "alpine/git:latest",
		Commands: append(
			cloneRepoCommands(clonePath, checkoutRef),
			[]string{
				fmt.Sprintf("mkdir -pv $(dirname %q)", destinationDockerfilePath),
				fmt.Sprintf("cp %q %q", path.Join(clonePath, repoRelativeDockerfilePath), destinationDockerfilePath),
			}...),
	}, destinationDockerfilePath
}

func assumeS3DownloadRoleStep(productName string) (step, string) {
	profileName := fmt.Sprintf("s3-download-%s", productName)
	return kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
		awsRoleSettings: awsRoleSettings{
			awsAccessKeyID:     value{fromSecret: "AWS_ACCESS_KEY_ID"},
			awsSecretAccessKey: value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
			role:               value{fromSecret: "AWS_ROLE"},
		},
		configVolume: volumeRefAwsConfig,
		profile:      profileName,
		name:         fmt.Sprintf("Assume S3 Download AWS Role for %s", productName),
		append:       true,
	}), profileName
}

func (p *Product) getBaseImage(arch string, version *ReleaseVersion, containerRepo *ContainerRepo) *Image {
	return p.ImageBuilder(
		containerRepo,
		&ImageTag{
			ShellBaseValue:   version.GetFullSemver().GetSemverValue(),
			DisplayBaseValue: version.MajorVersion,
			Arch:             arch,
			IsForFullSemver:  true,
		},
	)
}

func (p *Product) GetLocalRegistryImage(arch string, version *ReleaseVersion) *Image {
	return p.getBaseImage(arch, version, GetLocalContainerRepo())
}

func (p *Product) GetStagingRegistryImage(arch string, version *ReleaseVersion, stagingRepo *ContainerRepo) *Image {
	return p.getBaseImage(arch, version, stagingRepo)
}

func (p *Product) buildSteps(version *ReleaseVersion, parentStepNames []string, flags *TriggerFlags) []step {
	steps := make([]step, 0)

	// Get the container repos images will be pushed to
	stagingRepo := GetStagingContainerRepo(flags.UseUniqueStagingTag)
	publicEcrPullRegistry := GetPublicEcrPullRegistry()
	productionRepos := GetProductionContainerRepos()

	// Collect the name of the steps that are required before build/retrieval
	productSetupStepNames := make([]string, 0)
	if flags.ShouldBuildNewImages {
		for _, setupStep := range p.SetupSteps {
			// Wait for the parent steps before starting on the product setup steps
			setupStep.DependsOn = append(setupStep.DependsOn, parentStepNames...)
			steps = append(steps, setupStep)
			productSetupStepNames = append(productSetupStepNames, setupStep.Name)
		}
	}
	if len(productSetupStepNames) == 0 {
		// Cover the case where there are no product setup steps
		productSetupStepNames = parentStepNames
	}

	archBuildStepDetails := make([]*buildStepOutput, 0, len(p.SupportedArchs))

	// Add image build/retrieval steps
	for _, supportedArch := range p.SupportedArchs {
		// Include steps for building images from scratch
		if flags.ShouldBuildNewImages {
			archBuildStep, archBuildStepDetail := p.createBuildStep(supportedArch, version, publicEcrPullRegistry)

			// Collect the name of steps that are required before build, taking into account arch-specific steps
			setupStepNames := make([]string, 0)
			for _, archSetupStep := range p.ArchSetupSteps[supportedArch] {
				archSetupStep.DependsOn = append(archSetupStep.DependsOn, productSetupStepNames...)
				steps = append(steps, archSetupStep)
				setupStepNames = append(setupStepNames, archSetupStep.Name)
			}
			if len(setupStepNames) == 0 {
				// Cover the case where there are no arch specific steps
				setupStepNames = productSetupStepNames
			}

			archBuildStep.DependsOn = append(archBuildStep.DependsOn, setupStepNames...)

			steps = append(steps, archBuildStep)
			archBuildStepDetails = append(archBuildStepDetails, archBuildStepDetail)
		} else {
			stagingImage := p.GetStagingRegistryImage(supportedArch, version, stagingRepo)
			pullStagingImageStep, locallyPushedImage := stagingRepo.pullPushStep(stagingImage, productSetupStepNames)
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

	// Add publish steps
	for _, containerRepo := range getReposToPublishTo(productionRepos, stagingRepo, flags) {
		buildSteps := containerRepo.buildSteps(archBuildStepDetails, flags)

		// Add repo setup step dependency to the build steps
		setupStepNames := getStepNames(containerRepo.SetupSteps)
		for _, buildStep := range buildSteps {
			buildStep.DependsOn = append(buildStep.DependsOn, setupStepNames...)
		}

		steps = append(steps, buildSteps...)
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

func (p *Product) createBuildStep(arch string, version *ReleaseVersion, publicEcrPullRegistry *ContainerRepo) (step, *buildStepOutput) {
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
			buildCommand += fmt.Sprintf(" --build-arg %s", buildArg)
		}
	}
	buildCommand += " " + p.WorkingDirectory

	// This is important to prevent pull rate limiting. See `GetPublicEcrPullRegistry` doc comment
	// for details.
	authenticatedBuildCommands := publicEcrPullRegistry.buildCommandsWithLogin([]string{buildCommand})

	commands := []string{
		"docker run --privileged --rm tonistiigi/binfmt --install all",
		fmt.Sprintf("mkdir -pv %q && cd %q", p.WorkingDirectory, p.WorkingDirectory),
		fmt.Sprintf("mkdir -pv %q", buildxConfigFileDir),
		fmt.Sprintf("echo '[registry.%q]' > %q", LocalRegistrySocket, buildxConfigFilePath),
		fmt.Sprintf("echo '  http = true' >> %q", buildxConfigFilePath),
		buildxCreateCommand,
	}
	commands = append(commands, authenticatedBuildCommands...)
	commands = append(commands,
		fmt.Sprintf("docker buildx rm %q", builderName),
		fmt.Sprintf("rm -rf %q", buildxConfigFileDir),
	)

	envVars := maps.Clone(publicEcrPullRegistry.EnvironmentVars)
	envVars["DOCKER_BUILDKIT"] = value{
		raw: "1",
	}

	step := step{
		Name:        p.GetBuildStepName(arch, version),
		Image:       "docker",
		Volumes:     []volumeRef{volumeRefAwsConfig, volumeRefDocker}, // no docker config volume, as this will race
		Environment: envVars,
		Commands:    commands,
		DependsOn:   getStepNames(publicEcrPullRegistry.SetupSteps),
	}

	return step, &buildStepOutput{
		StepName:   step.Name,
		BuiltImage: localRegistryImage,
		Version:    version,
		Product:    p,
	}
}
