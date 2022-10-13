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
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"
)

// If you are working on a PR/testing changes to this file you should configure the following for Drone testing:
// 1. Publish the branch you're working on
// 2. Set `prBranch` to the name of the branch in (1)
// 3. Set `configureForPRTestingOnly` to true
// 4. Create a public and private ECR, Quay repos for "teleport", "teleport-ent", "teleport-operator", "teleport-lab"
// 5. Set `testingQuayRegistryOrg` and `testingECRRegistryOrg` to the org name(s) used in (4)
// 6. Set the `ECRTestingDomain` to the domain used for the private ECR repos
// 7. Create two separate IAM users, each with full access to either the public ECR repo OR the private ECR repo
// 8. Create a Quay "robot account" with write permissions for the created Quay repos
// 9. Set the Drone secrets for the secret names listed in "GetContainerRepos" to the credentials in (7, 8), prefixed by the value of `testingSecretPrefix`
//
// On each commit, after running `make dronegen``, run the following commands and resign the file:
// # Pull the current branch instead of v11 so the appropriate dockerfile gets loaded
// sed -i '' "s~git checkout -qf \"\$(cat '/go/vars/full-version/v11')\"~git checkout -qf \"${DRONE_SOURCE_BRANCH}\"~" .drone.yml
//
// When finishing up your PR check the following:
// * The testing secrets added to Drone have been removed
// * `configureForPRTestingOnly` has been set to false, and `make dronegen` has been reran afterwords

const (
	configureForPRTestingOnly bool   = false
	testingSecretPrefix       string = "TEST_"
	testingQuayRegistryOrg    string = "fred_heinecke"
	testingECRRegistryOrg     string = "u8j2q1d9"
	testingECRRegion          string = "us-east-2"
	prBranch                  string = "fred/multiarch-teleport-container-images"
	testingECRDomain          string = "278576220453.dkr.ecr.us-east-2.amazonaws.com"
)

const (
	ProductionRegistryOrg string = "gravitational"
	PublicEcrRegion       string = "us-east-1"
	StagingEcrRegion      string = "us-west-2"

	localRegistry string = "drone-docker-registry:5000"
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

type TriggerInfo struct {
	Trigger           trigger
	Name              string
	Flags             *TriggerFlags
	SupportedVersions []*releaseVersion
	SetupSteps        []step
}

// This is mainly used to make passing these vars around cleaner
type TriggerFlags struct {
	ShouldAffectProductionImages bool
	ShouldBuildNewImages         bool
	UseUniqueStagingTag          bool
}

func NewTestTrigger(triggerBranch, testMajorVersion string) *TriggerInfo {
	baseTrigger := NewCronTrigger([]string{testMajorVersion})
	baseTrigger.Name = "Test trigger on push"
	baseTrigger.Trigger = trigger{
		Repo:   triggerRef{Include: []string{"gravitational/teleport"}},
		Event:  triggerRef{Include: []string{"push"}},
		Branch: triggerRef{Include: []string{triggerBranch}},
	}

	return baseTrigger
}

func NewTagTrigger(branchMajorVersion string) *TriggerInfo {
	tagTrigger := triggerTag

	return &TriggerInfo{
		Trigger: tagTrigger,
		Name:    "tag",
		Flags: &TriggerFlags{
			ShouldAffectProductionImages: false,
			ShouldBuildNewImages:         true,
			UseUniqueStagingTag:          false,
		},
		SupportedVersions: []*releaseVersion{
			{
				MajorVersion:        branchMajorVersion,
				ShellVersion:        "$DRONE_TAG",
				RelativeVersionName: "branch",
			},
		},
	}
}

func NewPromoteTrigger(branchMajorVersion string) *TriggerInfo {
	promoteTrigger := triggerPromote
	promoteTrigger.Target.Include = append(promoteTrigger.Target.Include, "promote-docker")

	return &TriggerInfo{
		Trigger: promoteTrigger,
		Name:    "promote",
		Flags: &TriggerFlags{
			ShouldAffectProductionImages: true,
			ShouldBuildNewImages:         false,
			UseUniqueStagingTag:          false,
		},
		SupportedVersions: []*releaseVersion{
			{
				MajorVersion:        branchMajorVersion,
				ShellVersion:        "$DRONE_TAG",
				RelativeVersionName: "branch",
			},
		},
		SetupSteps: verifyValidPromoteRunSteps(),
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
		Trigger: cronTrigger([]string{"teleport-container-images-cron"}),
		Name:    "cron",
		Flags: &TriggerFlags{
			ShouldAffectProductionImages: true,
			ShouldBuildNewImages:         true,
			UseUniqueStagingTag:          true,
		},
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
		Image: fmt.Sprintf("golang:%s", GoVersion),
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
		pipeline := teleportVersion.buildVersionPipeline(ti.SetupSteps, ti.Flags)
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

type Product struct {
	Name                 string
	DockerfilePath       string
	WorkingDirectory     string
	DockerfileTarget     string
	SupportedArchs       []string
	SetupSteps           []step
	DockerfileArgBuilder func(arch string) []string
	ImageBuilder         func(repo string, tag *ImageTag) *Image
	GetRequiredStepNames func(arch string) []string
}

func NewTeleportOperatorProduct(cloneDirectory string) *Product {
	name := "teleport-operator"
	return &Product{
		Name:             name,
		DockerfilePath:   path.Join(cloneDirectory, "operator", "Dockerfile"),
		WorkingDirectory: cloneDirectory,
		SupportedArchs:   []string{"amd64", "arm", "arm64"},
		ImageBuilder: func(repo string, tag *ImageTag) *Image {
			return &Image{
				Repo: repo,
				Name: name,
				Tag:  tag,
			}
		},
		DockerfileArgBuilder: func(arch string) []string {
			gccPackage := ""
			compilerName := ""
			switch arch {
			case "x86_64", "amd64":
				gccPackage = "gcc-x86-64-linux-gnu"
				compilerName = "x86_64-linux-gnu-gcc"
			case "i686", "i386":
				gccPackage = "gcc-multilib-i686-linux-gnu"
				compilerName = "i686-linux-gnu-gcc"
			case "arm64", "aarch64":
				gccPackage = "gcc-aarch64-linux-gnu"
				compilerName = "aarch64-linux-gnu-gcc"
			// We may want to add additional arm ISAs in the future to support devices without hardware FPUs
			case "armhf":
			case "arm":
				gccPackage = "gcc-arm-linux-gnueabihf"
				compilerName = "arm-linux-gnueabihf-gcc"
			}

			return []string{
				fmt.Sprintf("COMPILER_PACKAGE=%s", gccPackage),
				fmt.Sprintf("COMPILER_NAME=%s", compilerName),
			}
		},
	}
}

func (p *Product) getBaseImage(arch string, version *releaseVersion) *Image {
	return &Image{
		Name: p.Name,
		Tag: &ImageTag{
			ShellBaseValue:   version.ShellVersion,
			DisplayBaseValue: version.MajorVersion,
			Arch:             arch,
		},
	}
}

func (p *Product) GetLocalRegistryImage(arch string, version *releaseVersion) *Image {
	image := p.getBaseImage(arch, version)
	image.Repo = localRegistry

	return image
}

func (p *Product) GetStagingRegistryImage(arch string, version *releaseVersion, stagingRepo *ContainerRepo) *Image {
	image := p.getBaseImage(arch, version)
	image.Repo = stagingRepo.RegistryDomain

	return image
}

func (p *Product) buildSteps(version *releaseVersion, setupStepNames []string, flags *TriggerFlags) []step {
	steps := make([]step, 0)

	stagingRepo := GetStagingContainerRepo(flags.UseUniqueStagingTag)
	productionRepos := GetProductionContainerRepos()

	for _, setupStep := range p.SetupSteps {
		setupStep.DependsOn = append(setupStep.DependsOn, setupStepNames...)
		steps = append(steps, setupStep)
		setupStepNames = append(setupStepNames, setupStep.Name)
	}

	archBuildStepDetails := make([]*buildStepOutput, 0, len(p.SupportedArchs))

	for _, supportedArch := range p.SupportedArchs {
		// Include steps for building images from scratch
		if flags.ShouldBuildNewImages {
			archBuildStep, archBuildStepDetail := p.createBuildStep(supportedArch, version)

			archBuildStep.DependsOn = append(archBuildStep.DependsOn, setupStepNames...)
			if p.GetRequiredStepNames != nil {
				archBuildStep.DependsOn = append(archBuildStep.DependsOn, p.GetRequiredStepNames(supportedArch)...)
			}

			steps = append(steps, archBuildStep)
			archBuildStepDetails = append(archBuildStepDetails, archBuildStepDetail)
		} else {
			// Generate build details that point to staging images
			archBuildStepDetails = append(archBuildStepDetails, &buildStepOutput{
				StepName:   "",
				BuiltImage: p.GetStagingRegistryImage(supportedArch, version, stagingRepo),
				Version:    version,
				Product:    p,
			})
		}
	}

	for _, containerRepo := range getReposToPublishTo(productionRepos, stagingRepo, flags) {
		steps = append(steps, containerRepo.buildSteps(archBuildStepDetails)...)
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

func (p *Product) GetBuildStepName(arch string, version *releaseVersion) string {
	telportImageName := p.GetLocalRegistryImage(arch, version)
	return fmt.Sprintf("Build %s image %q", p.Name, telportImageName.GetDisplayName())
}

func cleanBuilderName(builderName string) string {
	var invalidBuildxCharExpression = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
	return invalidBuildxCharExpression.ReplaceAllString(builderName, "-")
}

func (p *Product) createBuildStep(arch string, version *releaseVersion) (step, *buildStepOutput) {
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
	buildCommand += fmt.Sprintf(" --tag %q", localRegistryImage.GetShellName())
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
		Environment: map[string]value{
			"DOCKER_BUILDKIT": {
				raw: "1",
			},
		},
		Commands: []string{
			"docker run --privileged --rm tonistiigi/binfmt --install all",
			fmt.Sprintf("mkdir -pv %q && cd %q", p.WorkingDirectory, p.WorkingDirectory),
			fmt.Sprintf("mkdir -pv %q", buildxConfigFileDir),
			fmt.Sprintf("echo '[registry.%q]' > %q", localRegistry, buildxConfigFilePath),
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

// The `step` struct doesn't contain enough information to setup
// dependent steps so we add that via this struct
type buildStepOutput struct {
	StepName   string
	BuiltImage *Image
	Version    *releaseVersion
	Product    *Product
}

type ContainerRepo struct {
	Name             string
	IsProductionRepo bool
	IsImmutable      bool
	Environment      map[string]value
	RegistryDomain   string
	RegistryOrg      string
	LoginCommands    []string
	TagBuilder       func(baseTag *ImageTag) *ImageTag // Postprocessor for tags that append CR-specific suffixes
}

func NewEcrContainerRepo(accessKeyIDSecret, secretAccessKeySecret, domain string, isProduction, isImmutable, guaranteeUnique bool) *ContainerRepo {
	nameSuffix := "staging"
	ecrRegion := StagingEcrRegion
	loginSubcommand := "ecr"
	if isProduction {
		nameSuffix = "production"
		ecrRegion = PublicEcrRegion
		loginSubcommand = "ecr-public"
	}

	registryOrg := ProductionRegistryOrg
	if configureForPRTestingOnly {
		accessKeyIDSecret = testingSecretPrefix + accessKeyIDSecret
		secretAccessKeySecret = testingSecretPrefix + secretAccessKeySecret
		registryOrg = testingECRRegistryOrg

		if !isProduction {
			domain = testingECRDomain
			ecrRegion = testingECRRegion
		}
	}

	loginCommands := []string{
		"apk add --no-cache aws-cli",
		fmt.Sprintf("aws %s get-login-password --region=%s | docker login -u=\"AWS\" --password-stdin %s", loginSubcommand, ecrRegion, domain),
	}

	if guaranteeUnique {
		loginCommands = append(loginCommands, "TIMESTAMP=$(date -d @\"$DRONE_BUILD_CREATED\" '+%Y%m%d%H%M')")
	}

	return &ContainerRepo{
		Name:             fmt.Sprintf("ECR - %s", nameSuffix),
		IsProductionRepo: isProduction,
		IsImmutable:      isImmutable,
		Environment: map[string]value{
			"AWS_ACCESS_KEY_ID": {
				fromSecret: accessKeyIDSecret,
			},
			"AWS_SECRET_ACCESS_KEY": {
				fromSecret: secretAccessKeySecret,
			},
		},
		RegistryDomain: domain,
		RegistryOrg:    registryOrg,
		LoginCommands:  loginCommands,
		TagBuilder: func(tag *ImageTag) *ImageTag {
			if guaranteeUnique {
				tag.AppendString("$TIMESTAMP")
			}

			return tag
		},
	}
}

func NewQuayContainerRepo(dockerUsername, dockerPassword string) *ContainerRepo {
	registryOrg := ProductionRegistryOrg
	if configureForPRTestingOnly {
		dockerUsername = testingSecretPrefix + dockerUsername
		dockerPassword = testingSecretPrefix + dockerPassword
		registryOrg = testingQuayRegistryOrg
	}

	return &ContainerRepo{
		Name:             "Quay",
		IsProductionRepo: true,
		IsImmutable:      false,
		Environment: map[string]value{
			"QUAY_USERNAME": {
				fromSecret: dockerUsername,
			},
			"QUAY_PASSWORD": {
				fromSecret: dockerPassword,
			},
		},
		RegistryDomain: ProductionRegistryQuay,
		RegistryOrg:    registryOrg,
		LoginCommands: []string{
			fmt.Sprintf("docker login -u=\"$QUAY_USERNAME\" -p=\"$QUAY_PASSWORD\" %q", ProductionRegistryQuay),
		},
	}
}

func GetStagingContainerRepo(uniqueStagingTag bool) *ContainerRepo {
	return NewEcrContainerRepo("STAGING_TELEPORT_DRONE_USER_ECR_KEY", "STAGING_TELEPORT_DRONE_USER_ECR_SECRET", StagingRegistry, false, true, uniqueStagingTag)
}

func GetProductionContainerRepos() []*ContainerRepo {
	return []*ContainerRepo{
		NewQuayContainerRepo("PRODUCTION_QUAYIO_DOCKER_USERNAME", "PRODUCTION_QUAYIO_DOCKER_PASSWORD"),
		NewEcrContainerRepo("PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY", "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET", ProductionRegistry, true, false, false),
	}
}

func (cr *ContainerRepo) buildSteps(buildStepDetails []*buildStepOutput) []step {
	if len(buildStepDetails) == 0 {
		return nil
	}

	steps := make([]step, 0)

	imageTags := cr.BuildImageTags(buildStepDetails[0].Version)
	pushedImages := make(map[*ImageTag][]*Image, len(imageTags))
	pushStepNames := make([]string, 0, len(buildStepDetails))
	for _, buildStepDetail := range buildStepDetails {
		pushStep, pushedArchImages := cr.tagAndPushStep(buildStepDetail, imageTags)
		pushStepNames = append(pushStepNames, pushStep.Name)
		for _, imageTag := range imageTags {
			pushedImages[imageTag] = append(pushedImages[imageTag], pushedArchImages[imageTag])
		}

		steps = append(steps, pushStep)
	}

	imageRepo := cr.BuildImageRepo()
	for _, imageTag := range imageTags {
		multiarchImageTag := *imageTag
		multiarchImageTag.Arch = ""
		manifestImage := buildStepDetails[0].Product.ImageBuilder(imageRepo, &multiarchImageTag)
		manifestStepName := cr.createAndPushManifestStep(manifestImage, pushStepNames, pushedImages[imageTag])
		steps = append(steps, manifestStepName)
	}

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
	return fmt.Sprintf("%s/%s/", cr.RegistryDomain, cr.RegistryOrg)
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

func (cr *ContainerRepo) BuildImageTags(version *releaseVersion) []*ImageTag {
	tags := version.getTagsForVersion()

	if cr.TagBuilder != nil {
		for i, tag := range tags {
			tags[i] = cr.TagBuilder(tag)
		}
	}

	return tags
}

func (cr *ContainerRepo) tagAndPushStep(buildStepDetails *buildStepOutput, imageTags []*ImageTag) (step, map[*ImageTag]*Image) {
	imageRepo := cr.BuildImageRepo()

	archImageMap := make(map[*ImageTag]*Image, len(imageTags))
	for _, imageTag := range imageTags {
		archTag := *imageTag
		archTag.Arch = buildStepDetails.BuiltImage.Tag.Arch
		archImage := buildStepDetails.Product.ImageBuilder(imageRepo, &archTag)
		archImageMap[imageTag] = archImage
	}

	// This is tracked separately as maps in golang have a non-deterministic order when iterated over.
	// As a result, .drone.yml will be updated every time `make dronegen` is ran regardless of if there
	// is a change to the map or not
	// The order/comparator does not matter here as long as it is deterministic between dronegen runs
	archImageKeys := maps.Keys(archImageMap)
	sort.SliceStable(archImageKeys, func(i, j int) bool { return archImageKeys[i].GetDisplayValue() < archImageKeys[j].GetDisplayValue() })

	commands := []string{
		fmt.Sprintf("docker pull %q", buildStepDetails.BuiltImage.GetShellName()), // This will pull from the local registry
	}
	for _, archImageKey := range archImageKeys {
		archImage := archImageMap[archImageKey]

		// Skip pushing images if the tag or container registry is immutable
		commands = append(commands, buildImmutableSafeCommands(archImageKey.IsImmutable || cr.IsImmutable, archImage.GetShellName(), []string{
			fmt.Sprintf("docker tag %q %q", buildStepDetails.BuiltImage.GetShellName(), archImage.GetShellName()),
			fmt.Sprintf("docker push %q", archImage.GetShellName()),
		})...)
	}

	dependencySteps := []string{}
	if buildStepDetails.StepName != "" {
		dependencySteps = append(dependencySteps, buildStepDetails.StepName)
	}

	step := step{
		Name:        fmt.Sprintf("Tag and push image %q to %s", buildStepDetails.BuiltImage.GetDisplayName(), cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands:    cr.buildCommandsWithLogin(commands),
		DependsOn:   dependencySteps,
	}

	return step, archImageMap
}

func (cr *ContainerRepo) createAndPushManifestStep(manifestImage *Image, pushStepNames []string, pushedImages []*Image) step {
	if len(pushStepNames) == 0 {
		return step{}
	}

	manifestCommandArgs := make([]string, 0, len(pushedImages))
	for _, pushedImage := range pushedImages {
		manifestCommandArgs = append(manifestCommandArgs, fmt.Sprintf("--amend %q", pushedImage.GetShellName()))
	}

	// Skip pushing manifest if the tag or container registry is immutable
	commands := buildImmutableSafeCommands(manifestImage.Tag.IsImmutable || cr.IsImmutable, manifestImage.GetShellName(), []string{
		fmt.Sprintf("docker manifest create %q %s", manifestImage.GetShellName(), strings.Join(manifestCommandArgs, " ")),
		fmt.Sprintf("docker manifest push %q", manifestImage.GetShellName()),
	})

	return step{
		Name:        fmt.Sprintf("Create manifest and push %q to %s", manifestImage.GetDisplayName(), cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands:    cr.buildCommandsWithLogin(commands),
		DependsOn:   pushStepNames,
	}
}

func buildImmutableSafeCommands(isImmutable bool, imageToCheck string, commandsToRun []string) []string {
	if !isImmutable {
		return commandsToRun
	}

	conditionalCommand := fmt.Sprintf("docker manifest inspect %q > /dev/null 2>&1", imageToCheck)
	commandToRun := strings.Join(commandsToRun, " && ")
	return []string{fmt.Sprintf("%s && echo 'Found existing image, skipping' || (%s)", conditionalCommand, commandToRun)}
}
