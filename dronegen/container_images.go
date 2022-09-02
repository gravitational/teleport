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
// # `drone exec` does not support `exec` or `kubernetes` pipelines
// sed -i '' 's/type\: kubernetes/type\: docker/' .drone.yml && sed -i '' 's/type\: exec/type\: docker/' .drone.yml
// # Drone requires certain variables to be set
// export DRONE_REMOTE_URL="https://github.com/gravitational/teleport"
// # `drone exec` does not properly map the workspace path. This creates a volume to be shared between steps
// #  at the correct path
// DOCKER_VOLUME_NAME="go"
// docker volume create "$DOCKER_VOLUME_NAME"
// drone exec --trusted --pipeline teleport-container-images-current-version-cron --clone=false --volume "${DOCKER_VOLUME_NAME}:/go"
// # Cleanup
// docker volume rm "$DOCKER_VOLUME_NAME"

import (
	"fmt"
	"path"
	"strings"
)

func buildContainerImagePipelines() []pipeline {
	// These need to be updated on each major release.
	latestMajorVersions := []string{"v10", "v9", "v8"}
	branchMajorVersion := "v10"

	if len(latestMajorVersions) == 0 {
		return []pipeline{}
	}

	triggers := []*TriggerInfo{
		NewPromoteTrigger(branchMajorVersion),
		NewCronTrigger(latestMajorVersions),
	}

	pipelines := make([]pipeline, 0, len(triggers))
	for _, trigger := range triggers {
		pipelines = append(pipelines, trigger.buildPipelines()...)
	}

	return pipelines
}

// TODO consider a fan-in step for all structs requiring setup steps to reduce
// dependency complexity

type TriggerInfo struct {
	Trigger           trigger
	Name              string
	SupportedVersions []*releaseVersion
	SetupSteps        []step
}

func NewPromoteTrigger(branchMajorVersion string) *TriggerInfo {
	promoteTrigger := triggerPromote
	promoteTrigger.Target.Include = append(promoteTrigger.Target.Include, "promote-docker")
	checkoutPath := "/go/src/github.com/gravitational/teleport"

	return &TriggerInfo{
		Trigger: promoteTrigger,
		Name:    "promote",
		SupportedVersions: []*releaseVersion{
			{
				MajorVersion:        branchMajorVersion,
				ShellVersion:        "$DRONE_TAG",
				RelativeVersionName: "drone-tag",
			},
		},
		SetupSteps: verifyValidPromoteRunSteps(checkoutPath, "$DRONE_TAG", true),
	}
}

func NewCronTrigger(latestMajorVersions []string) *TriggerInfo {
	if len(latestMajorVersions) == 0 {
		return nil
	}

	majorVersionVarDirectory := "/go/vars/full-version"

	supportedVersions := make([]*releaseVersion, 0, len(latestMajorVersions))
	if len(latestMajorVersions) > 0 {
		latestMajorVersion := latestMajorVersions[0]
		supportedVersions = append(supportedVersions, &releaseVersion{
			MajorVersion:        latestMajorVersion,
			ShellVersion:        readCronShellVersionCommand(majorVersionVarDirectory, latestMajorVersion),
			RelativeVersionName: "current-version",
			SetupSteps:          []step{getLatestSemverStep(latestMajorVersion, majorVersionVarDirectory)},
		})

		if len(latestMajorVersions) > 1 {
			for i, majorVersion := range latestMajorVersions[1:] {
				supportedVersions = append(supportedVersions, &releaseVersion{
					MajorVersion:        majorVersion,
					ShellVersion:        readCronShellVersionCommand(majorVersionVarDirectory, majorVersion),
					RelativeVersionName: fmt.Sprintf("previous-version-%d", i+1),
					SetupSteps:          []step{getLatestSemverStep(majorVersion, majorVersionVarDirectory)},
				})
			}
		}
	}

	return &TriggerInfo{
		Trigger:           cronTrigger([]string{"teleport-container-images-cron"}),
		Name:              "cron",
		SupportedVersions: supportedVersions,
	}
}

func getLatestSemverStep(majorVersion string, majorVersionVarDirectory string) step {
	// We don't use "/go/src/github.com/gravitational/teleport" here as a later stage
	// may need to clone a different version, and "/go" persists between steps
	cloneDirectory := "/tmp/teleport"
	majorVersionVarPath := path.Join(majorVersionVarDirectory, majorVersion)
	return step{
		Name:  fmt.Sprintf("Find the latest available semver for %s", majorVersion),
		Image: "golang:1.18",
		Commands: append(
			cloneRepoCommands(cloneDirectory, fmt.Sprintf("branch/%s", majorVersion)),
			fmt.Sprintf("mkdir -pv %q", majorVersionVarDirectory),
			fmt.Sprintf("cd %q", path.Join(cloneDirectory, "build.assets", "tooling", "cmd", "query-latest")),
			fmt.Sprintf("go run . %q > %q", majorVersion, majorVersionVarPath),
			fmt.Sprintf("echo Found full semver \"$(cat %q)\" for major version %q", majorVersionVarPath, majorVersion),
		),
	}
}

func readCronShellVersionCommand(majorVersionDirectory, majorVersion string) string {
	return fmt.Sprintf("$(cat '%s')", path.Join(majorVersionDirectory, majorVersion))
}

// Drone triggers must all evaluate to "true" for a pipeline to be executed.
// As a result these pipelines are duplicated for each trigger.
// See https://docs.drone.io/pipeline/triggers/ for details.
func (ti *TriggerInfo) buildPipelines() []pipeline {
	pipelines := make([]pipeline, 0, len(ti.SupportedVersions))
	for _, teleportVersion := range ti.SupportedVersions {
		pipeline := teleportVersion.buildVersionPipeline(ti.SetupSteps)
		pipeline.Name += "-" + ti.Name
		pipeline.Trigger = ti.Trigger

		pipelines = append(pipelines, pipeline)
	}

	return pipelines
}

type releaseVersion struct {
	MajorVersion        string // This is the major version of a given build. `SearchVersion` should match this when evaluated.
	ShellVersion        string // This value will be evaluated by the shell in the context of a Drone step
	RelativeVersionName string // The set of values for this should not change between major releases
	SetupSteps          []step // Version-specific steps that must be ran before executing build and push steps
}

func (rv *releaseVersion) buildVersionPipeline(triggerSetupSteps []step) pipeline {
	pipelineName := fmt.Sprintf("teleport-container-images-%s", rv.RelativeVersionName)

	setupSteps, dependentStepNames := rv.getSetupStepInformation(triggerSetupSteps)

	pipeline := newKubePipeline(pipelineName)
	pipeline.Workspace = workspace{Path: "/go"}
	pipeline.Services = []service{dockerService()}
	pipeline.Volumes = dockerVolumes()
	pipeline.Environment = map[string]value{
		"DEBIAN_FRONTEND": {
			raw: "noninteractive",
		},
	}
	pipeline.Steps = append(setupSteps, rv.buildSteps(dependentStepNames)...)

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

func (rv *releaseVersion) buildSteps(setupStepNames []string) []step {
	clonedRepoPath := "/go/src/github.com/gravitational/teleport"
	steps := make([]step, 0)

	setupSteps := []step{
		waitForDockerStep(),
		cloneRepoStep(clonedRepoPath, rv.ShellVersion),
	}
	for _, setupStep := range setupSteps {
		setupStep.DependsOn = append(setupStep.DependsOn, setupStepNames...)
		steps = append(steps, setupStep)
		setupStepNames = append(setupStepNames, setupStep.Name)
	}

	for _, product := range rv.getProducts(clonedRepoPath) {
		steps = append(steps, product.BuildSteps(rv, setupStepNames)...)
	}

	return steps
}

func (rv *releaseVersion) getProducts(clonedRepoPath string) []*product {
	ossTeleport := NewTeleportProduct(false, false, rv)
	teleportProducts := []*product{
		ossTeleport,                         // OSS
		NewTeleportProduct(true, false, rv), // Enterprise
		NewTeleportProduct(true, true, rv),  // Enterprise/FIPS
	}
	teleportLabProducts := []*product{
		NewTeleportLabProduct(clonedRepoPath, rv, ossTeleport),
	}
	teleportOperatorProduct := NewTeleportOperatorProduct(clonedRepoPath)

	products := make([]*product, 0, len(teleportProducts)+len(teleportLabProducts)+1)
	products = append(products, teleportProducts...)
	products = append(products, teleportLabProducts...)
	products = append(products, teleportOperatorProduct)

	return products
}

type product struct {
	Name                 string
	DockerfilePath       string
	WorkingDirectory     string
	DockerfileTarget     string
	SupportedArchs       []string
	SetupSteps           []step
	DockerfileArgBuilder func(arch string) []string
	ImageNameBuilder     func(repo, tag string) string
	GetRequiredStepNames func(arch string) []string
}

func NewTeleportProduct(isEnterprise, isFips bool, version *releaseVersion) *product {
	workingDirectory := "/go/build"
	dockerfile := path.Join(workingDirectory, "Dockerfile")
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

	setupStep, debPaths := teleportSetupStep(version.ShellVersion, name, dockerfile, downloadURL, supportedArches)

	return &product{
		Name:             name,
		DockerfilePath:   dockerfile,
		WorkingDirectory: workingDirectory,
		DockerfileTarget: dockerfileTarget,
		SupportedArchs:   supportedArches,
		SetupSteps:       []step{setupStep},
		DockerfileArgBuilder: func(arch string) []string {
			return []string{
				fmt.Sprintf("DEB_PATH=%s", debPaths[arch]),
			}
		},
		ImageNameBuilder: func(repo, tag string) string {
			imageProductName := "teleport"
			if isEnterprise {
				imageProductName += "-ent"
			}

			if isFips {
				tag += "-fips"
			}

			return defaultImageTagBuilder(repo, imageProductName, tag)
		},
	}
}

func NewTeleportLabProduct(cloneDirectory string, version *releaseVersion, teleport *product) *product {
	workingDirectory := "/tmp/build"
	dockerfile := path.Join(cloneDirectory, "docker", "sshd", "Dockerfile")
	name := "teleport-lab"

	return &product{
		Name:             name,
		DockerfilePath:   dockerfile,
		WorkingDirectory: workingDirectory,
		SupportedArchs:   teleport.SupportedArchs,
		DockerfileArgBuilder: func(arch string) []string {
			return []string{
				fmt.Sprintf("BASE_IMAGE=%s", teleport.BuildLocalImageName(arch, version)),
			}
		},
		ImageNameBuilder: func(repo, tag string) string { return defaultImageTagBuilder(repo, name, tag) },
		GetRequiredStepNames: func(arch string) []string {
			return []string{teleport.GetBuildStepName(arch, version)}
		},
	}
}

func NewTeleportOperatorProduct(cloneDirectory string) *product {
	name := "teleport-operator"
	return &product{
		Name:             name,
		DockerfilePath:   path.Join(cloneDirectory, "operator", "Dockerfile"),
		WorkingDirectory: cloneDirectory,
		SupportedArchs:   []string{"amd64", "arm", "arm64"},
		ImageNameBuilder: func(repo, tag string) string { return defaultImageTagBuilder(repo, name, tag) },
	}
}

func defaultImageTagBuilder(repo, name, tag string) string {
	return fmt.Sprintf("%s%s:%s", repo, name, tag)
}

func teleportSetupStep(shellVersion, packageName, dockerfilePath, downloadURL string, archs []string) (step, map[string]string) {
	keyPath := "/usr/share/keyrings/teleport-archive-keyring.asc"
	downloadDirectory := "/tmp/apt-download"
	timeout := 30 * 60 // 30 minutes in seconds
	sleepTime := 15    // 15 seconds

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
		archDir := path.Join("/go/artifacts/deb/", packageName, arch)
		// Example: `/go/artifacts/deb/teleport-ent/arm64/v10.1.4.deb`
		destPath := path.Join(archDir, fmt.Sprintf("%s.deb", shellVersion))

		archDestFileMap[arch] = destPath

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
			fmt.Sprintf("mv $${FILENAME} %q", destPath),
			fmt.Sprintf("echo \"Downloaded \\\"%s\\\" to \\\"%s\\\"\"", fullPackageName, destPath),
		}...)
	}

	return step{
		Name:     fmt.Sprintf("Download %q DEB artifact from APT", packageName),
		Image:    "ubuntu:22.04",
		Commands: commands,
	}, archDestFileMap
}

func (p *product) BuildLocalImageName(arch string, version *releaseVersion) string {
	return fmt.Sprintf("%s-%s-%s", p.Name, version.MajorVersion, arch)
}

func (p *product) BuildSteps(version *releaseVersion, setupStepNames []string) []step {
	containerRepos := GetContainerRepos()

	steps := make([]step, 0)

	for _, setupStep := range p.SetupSteps {
		setupStep.DependsOn = append(setupStep.DependsOn, setupStepNames...)
		steps = append(steps, setupStep)
		setupStepNames = append(setupStepNames, setupStep.Name)
	}

	archBuildStepDetails := make([]*buildStepOutput, 0, len(p.SupportedArchs))
	for _, supportedArch := range p.SupportedArchs {
		archBuildStep, archBuildStepDetail := p.createBuildStep(supportedArch, version)

		archBuildStep.DependsOn = append(archBuildStep.DependsOn, setupStepNames...)
		if p.GetRequiredStepNames != nil {
			archBuildStep.DependsOn = append(archBuildStep.DependsOn, p.GetRequiredStepNames(supportedArch)...)
		}

		steps = append(steps, archBuildStep)
		archBuildStepDetails = append(archBuildStepDetails, archBuildStepDetail)
	}

	for _, containerRepo := range containerRepos {
		steps = append(steps, containerRepo.buildSteps(archBuildStepDetails)...)
	}

	return steps
}

func (p *product) GetBuildStepName(arch string, version *releaseVersion) string {
	return fmt.Sprintf("Build %s image %q", p.Name, p.BuildLocalImageName(arch, version))
}

func (p *product) createBuildStep(arch string, version *releaseVersion) (step, *buildStepOutput) {
	imageName := p.BuildLocalImageName(arch, version)

	buildCommand := "docker build"
	if p.DockerfileTarget != "" {
		buildCommand += fmt.Sprintf(" --target %q", p.DockerfileTarget)
	}
	buildCommand += fmt.Sprintf(" --platform %q", "linux/"+arch)
	buildCommand += fmt.Sprintf(" --tag %q", imageName)
	buildCommand += fmt.Sprintf(" --file %q", p.DockerfilePath)
	if p.DockerfileArgBuilder != nil {
		for _, buildArg := range p.DockerfileArgBuilder(arch) {
			buildCommand += fmt.Sprintf(" --build-arg %q", buildArg)
		}
	}
	buildCommand += " " + p.WorkingDirectory

	step := step{
		Name:    p.GetBuildStepName(arch, version),
		Image:   "docker",
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			fmt.Sprintf("mkdir -pv %q && cd %q", p.WorkingDirectory, p.WorkingDirectory),
			buildCommand,
		},
	}

	return step, &buildStepOutput{
		StepName:       step.Name,
		BuiltImageName: imageName,
		BuiltImageArch: arch,
		Version:        version,
		Product:        p,
	}
}

// The `step` struct doesn't contain enough information to setup
// dependent steps so we add that via this struct
type buildStepOutput struct {
	StepName       string
	BuiltImageName string
	BuiltImageArch string
	Version        *releaseVersion
	Product        *product
}

type ContainerRepo struct {
	Name           string
	Environment    map[string]value
	RegistryDomain string
	LoginCommands  []string
	TagBuilder     func(baseTag string) string // Postprocessor for tags that append CR-specific suffixes
}

func NewEcrContainerRepo(accessKeyIDSecret, secretAccessKeySecret, domain string, isStaging bool) *ContainerRepo {
	nameSuffix := "staging"
	if !isStaging {
		nameSuffix = "production"
	}

	return &ContainerRepo{
		Name: fmt.Sprintf("ECR - %s", nameSuffix),
		Environment: map[string]value{
			"AWS_ACCESS_KEY_ID": {
				fromSecret: accessKeyIDSecret,
			},
			"AWS_SECRET_ACCESS_KEY": {
				fromSecret: secretAccessKeySecret,
			},
		},
		RegistryDomain: domain,
		LoginCommands: []string{
			"apk add --no-cache aws-cli",
			"TIMESTAMP=$(date -d @\"$DRONE_BUILD_CREATED\" '+%Y%m%d%H%M')",
			fmt.Sprintf("aws ecr get-login-password --region=us-west-2 | docker login -u=\"AWS\" --password-stdin %s", domain),
		},
		TagBuilder: func(baseTag string) string {
			if !isStaging {
				return baseTag
			}

			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
	}
}

func NewQuayContainerRepo(dockerUsername, dockerPassword string) *ContainerRepo {
	return &ContainerRepo{
		Name: "Quay",
		Environment: map[string]value{
			"QUAY_USERNAME": {
				fromSecret: dockerUsername,
			},
			"QUAY_PASSWORD": {
				fromSecret: dockerPassword,
			},
		},
		RegistryDomain: ProductionRegistryQuay,
		LoginCommands: []string{
			fmt.Sprintf("docker login -u=\"$QUAY_USERNAME\" -p=\"$QUAY_PASSWORD\" %q", ProductionRegistryQuay),
		},
	}
}

func GetContainerRepos() []*ContainerRepo {
	return []*ContainerRepo{
		NewQuayContainerRepo("PRODUCTION_QUAYIO_DOCKER_USERNAME", "PRODUCTION_QUAYIO_DOCKER_PASSWORD"),
		NewEcrContainerRepo("STAGING_TELEPORT_DRONE_USER_ECR_KEY", "STAGING_TELEPORT_DRONE_USER_ECR_SECRET", StagingRegistry, true),
		NewEcrContainerRepo("PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY", "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET", ProductionRegistry, false),
	}
}

func (cr *ContainerRepo) buildSteps(buildStepDetails []*buildStepOutput) []step {
	if len(buildStepDetails) == 0 {
		return nil
	}

	steps := make([]step, 0)

	pushStepDetails := make([]*pushStepOutput, 0, len(buildStepDetails))
	for _, buildStepDetail := range buildStepDetails {
		pushStep, pushStepDetail := cr.tagAndPushStep(buildStepDetail)
		pushStepDetails = append(pushStepDetails, pushStepDetail)
		steps = append(steps, pushStep)
	}

	manifestStepName := cr.createAndPushManifestStep(pushStepDetails)
	steps = append(steps, manifestStepName)

	return steps
}

func (cr *ContainerRepo) logoutCommand() string {
	return fmt.Sprintf("docker logout %q", cr.RegistryDomain)
}

func (cr *ContainerRepo) buildCommandsWithLogin(wrappedCommands []string) []string {
	commands := make([]string, 0)
	commands = append(commands, cr.LoginCommands...)
	commands = append(commands, wrappedCommands...)
	commands = append(commands, cr.logoutCommand())

	return commands
}

func (cr *ContainerRepo) BuildImageRepo() string {
	return cr.RegistryDomain + "/gravitational/"
}

func (cr *ContainerRepo) BuildImageTag(majorVersion string) string {
	baseTag := strings.TrimPrefix(majorVersion, "v")

	if cr.TagBuilder == nil {
		return baseTag
	}

	return cr.TagBuilder(baseTag)
}

type pushStepOutput struct {
	PushedImageName string
	BaseImageName   string
	StepName        string
}

func (cr *ContainerRepo) tagAndPushStep(buildStepDetails *buildStepOutput) (step, *pushStepOutput) {
	imageName := buildStepDetails.Product.ImageNameBuilder(cr.BuildImageRepo(), cr.BuildImageTag(buildStepDetails.Version.MajorVersion))
	archImageName := fmt.Sprintf("%s-%s", imageName, buildStepDetails.BuiltImageArch)

	step := step{
		Name:        fmt.Sprintf("Tag and push %q to %s", archImageName, cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands: cr.buildCommandsWithLogin([]string{
			fmt.Sprintf("docker tag %q %q", buildStepDetails.BuiltImageName, archImageName),
			fmt.Sprintf("docker push %q", archImageName),
		}),
		DependsOn: []string{
			buildStepDetails.StepName,
		},
	}

	return step, &pushStepOutput{
		PushedImageName: archImageName,
		BaseImageName:   imageName,
		StepName:        step.Name,
	}
}

func (cr *ContainerRepo) createAndPushManifestStep(pushStepDetails []*pushStepOutput) step {
	if len(pushStepDetails) == 0 {
		return step{}
	}

	manifestName := pushStepDetails[0].BaseImageName

	manifestCommandArgs := make([]string, 0, len(pushStepDetails))
	pushStepNames := make([]string, 0, len(pushStepDetails))
	for _, pushStepDetail := range pushStepDetails {
		manifestCommandArgs = append(manifestCommandArgs, fmt.Sprintf("--amend %q", pushStepDetail.PushedImageName))
		pushStepNames = append(pushStepNames, pushStepDetail.StepName)
	}

	return step{
		Name:        fmt.Sprintf("Create manifest and push %q to %s", manifestName, cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands: cr.buildCommandsWithLogin([]string{
			fmt.Sprintf("docker manifest create %q %s", manifestName, strings.Join(manifestCommandArgs, " ")),
			fmt.Sprintf("docker manifest push %q", manifestName),
		}),
		DependsOn: pushStepNames,
	}
}
