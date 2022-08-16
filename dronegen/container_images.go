package main

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
	SupportedVersions []*teleportVersion
	SetupSteps        []step
}

func NewPromoteTrigger(branchMajorVersion string) *TriggerInfo {
	promoteTrigger := triggerPromote
	promoteTrigger.Target.Include = append(promoteTrigger.Target.Include, "promote-docker")
	checkoutPath := "/go/src/github.com/gravitational/teleport"

	return &TriggerInfo{
		Trigger: promoteTrigger,
		Name:    "promote",
		SupportedVersions: []*teleportVersion{
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

	supportedVersions := make([]*teleportVersion, 0, len(latestMajorVersions))
	if len(latestMajorVersions) > 0 {
		latestMajorVersion := latestMajorVersions[0]
		supportedVersions = append(supportedVersions, &teleportVersion{
			MajorVersion:        latestMajorVersion,
			ShellVersion:        readCronShellVersionCommand(majorVersionVarDirectory, latestMajorVersion),
			RelativeVersionName: "current-version",
			SetupSteps:          []step{getLatestSemverStep(latestMajorVersion, majorVersionVarDirectory)},
		})

		if len(latestMajorVersions) > 1 {
			for i, majorVersion := range latestMajorVersions[1:] {
				supportedVersions = append(supportedVersions, &teleportVersion{
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
	return step{
		Name:  fmt.Sprintf("Find the latest available semver for %s", majorVersion),
		Image: "golang:1.18-alpine",
		Commands: append(
			cloneRepoCommands(cloneDirectory, fmt.Sprintf("branch/%s", majorVersion)),
			fmt.Sprintf("cd %q", path.Join(cloneDirectory, "build.assets", "tooling", "cmd", "query-latest")),
			fmt.Sprintf("go run . %q > %q", majorVersion, path.Join(majorVersionVarDirectory, majorVersion)),
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

type teleportVersion struct {
	MajorVersion        string // This is the major version of a given build. `SearchVersion` should match this when evaluated.
	ShellVersion        string // This value will be evaluated by the shell in the context of a Drone step
	RelativeVersionName string // The set of values for this should not change between major releases
	SetupSteps          []step // Version-specific steps that must be ran before executing build and push steps
}

func (tv *teleportVersion) buildVersionPipeline(triggerSetupSteps []step) pipeline {
	pipelineName := fmt.Sprintf("teleport-container-images-%s", tv.RelativeVersionName)

	setupSteps, dependentStepNames := tv.getSetupStepInformation(triggerSetupSteps)

	pipeline := newKubePipeline(pipelineName)
	pipeline.Workspace = workspace{Path: "/go"}
	pipeline.Services = []service{dockerService()}
	pipeline.Volumes = dockerVolumes()
	pipeline.Steps = append(setupSteps, tv.buildSteps(dependentStepNames)...)

	return pipeline
}

func (tv *teleportVersion) getSetupStepInformation(triggerSetupSteps []step) ([]step, []string) {
	triggerSetupStepNames := make([]string, 0, len(triggerSetupSteps))
	for _, triggerSetupStep := range triggerSetupSteps {
		triggerSetupStepNames = append(triggerSetupStepNames, triggerSetupStep.Name)
	}

	nextStageSetupStepNames := triggerSetupStepNames
	if len(tv.SetupSteps) > 0 {
		versionSetupStepNames := make([]string, 0, len(tv.SetupSteps))
		for _, versionSetupStep := range tv.SetupSteps {
			versionSetupStep.DependsOn = append(versionSetupStep.DependsOn, triggerSetupStepNames...)
			versionSetupStepNames = append(versionSetupStepNames, versionSetupStep.Name)
		}

		nextStageSetupStepNames = versionSetupStepNames
	}

	setupSteps := make([]step, 0, len(triggerSetupSteps)+len(tv.SetupSteps))
	setupSteps = append(setupSteps, triggerSetupSteps...)
	setupSteps = append(setupSteps, tv.SetupSteps...)

	return setupSteps, nextStageSetupStepNames
}

func (tv *teleportVersion) buildSteps(setupStepNames []string) []step {
	teleportPackages := []teleportPackage{
		{IsEnterprise: false, IsFIPS: false}, // OSS
		{IsEnterprise: true, IsFIPS: false},  // Enterprise
		{IsEnterprise: true, IsFIPS: true},   // Enterprise/FIPS
	}
	steps := make([]step, 0)

	dockerStep := waitForDockerStep()
	steps = append(steps, dockerStep)
	setupStepNames = append(setupStepNames, dockerStep.Name)

	for _, teleportPackage := range teleportPackages {
		steps = append(steps, teleportPackage.buildSteps(tv, setupStepNames)...)
	}

	return steps
}

type teleportPackage struct {
	IsEnterprise bool
	IsFIPS       bool
}

func (tp *teleportPackage) GetName() string {
	baseName := "teleport"
	if !tp.IsEnterprise {
		return baseName
	}

	baseName = fmt.Sprintf("%s-ent", baseName)
	if !tp.IsFIPS {
		return baseName
	}

	return fmt.Sprintf("%s-fips", baseName)
}

func (tp *teleportPackage) buildSteps(version *teleportVersion, versionSetupSteps []string) []step {
	// The base image (ubuntu:20.04) does not offer i386 images so we don't either
	supportedArchs := []string{
		"amd64",
		"arm64",
		"arm",
	}
	clonedRepoPath := "/go/src/github.com/gravitational/teleport"
	containerRepos := GetContainerRepos()

	steps := make([]step, 0)

	packageSetupSteps := []step{cloneRepoStep(clonedRepoPath, version.ShellVersion)}
	packageSetupStepNames := make([]string, 0, len(packageSetupSteps))
	for _, packageSetupStep := range packageSetupSteps {
		packageSetupStep.DependsOn = append(packageSetupStep.DependsOn, versionSetupSteps...)
		packageSetupStepNames = append(packageSetupStepNames, packageSetupStep.Name)
	}

	steps = append(steps, packageSetupSteps...)

	teleportBuildStepDetails := make([]*buildStepOutput, 0, len(supportedArchs))
	labBuildStepDetails := make([]*buildStepOutput, 0, len(supportedArchs))
	operatorBuildStepDetails := make([]*buildStepOutput, 0, len(supportedArchs))
	for _, supportedArch := range supportedArchs {
		// FIPS is only supported on AMD64 currently
		if tp.IsFIPS && supportedArch != "amd64" {
			continue
		}

		// Setup Teleport build steps
		teleportBuildArchStep, teleportBuildArchStepDetails := tp.buildTeleportArchStep(version, supportedArch)
		teleportBuildArchStep.DependsOn = packageSetupStepNames
		steps = append(steps, teleportBuildArchStep)
		teleportBuildStepDetails = append(teleportBuildStepDetails, teleportBuildArchStepDetails)

		// Setup Teleport lab build steps
		// Only use OSS for now as that's what we currently support
		if tp.IsEnterprise || tp.IsFIPS {
			continue
		}

		labBuildArchStep, labBuildArchStepDetails := tp.buildTeleportLabArchStep(teleportBuildArchStepDetails, clonedRepoPath)
		steps = append(steps, labBuildArchStep)
		labBuildStepDetails = append(labBuildStepDetails, labBuildArchStepDetails)

		operatorBuildArchStep, operatorBuildArchStepDetails := tp.buildTeleportOperatorArchStep(version, supportedArch, clonedRepoPath)
		steps = append(steps, operatorBuildArchStep)
		labBuildStepDetails = append(operatorBuildStepDetails, operatorBuildArchStepDetails)
	}

	for _, containerRepo := range containerRepos {
		steps = append(steps, containerRepo.buildSteps(teleportBuildStepDetails)...)
		steps = append(steps, containerRepo.buildSteps(labBuildStepDetails)...)
		steps = append(steps, containerRepo.buildSteps(operatorBuildStepDetails)...)
	}

	return steps
}

type product struct {
	Name string
}

func (p *product) BuildStep() {

}

func (tp *teleportPackage) buildTeleportLabArchStep(teleportBuildStepDetail *buildStepOutput, cloneDirectory string) (step, *buildStepOutput) {
	workingDirectory := "/tmp/build"
	dockerfile := path.Join(cloneDirectory, "docker", "sshd", "Dockerfile")

	step, stepDetail := tp.createBuildStep("teleport-lab", teleportBuildStepDetail.BuiltImageArch, dockerfile,
		workingDirectory, "", []string{fmt.Sprintf("BASE_IMAGE=%q", teleportBuildStepDetail.BuiltImageName)}, teleportBuildStepDetail.Version)
	step.DependsOn = []string{teleportBuildStepDetail.StepName}

	return step, stepDetail
}

func (tp *teleportPackage) buildTeleportOperatorArchStep(version *teleportVersion, arch, cloneDirectory string) (step, *buildStepOutput) {
	dockerfile := path.Join(cloneDirectory, "operator", "Dockerfile")

	step, stepDetail := tp.createBuildStep("teleport-operator", arch, dockerfile, cloneDirectory, "", []string{}, version)

	return step, stepDetail
}

func (tp *teleportPackage) buildTeleportArchStep(version *teleportVersion, arch string) (step, *buildStepOutput) {
	workingDirectory := "/go/build"
	dockerfile := path.Join(workingDirectory, "Dockerfile-cron")
	// Other dockerfiles can be added/configured here if needed in the future
	downloadURL := "https://raw.githubusercontent.com/gravitational/teleport/${DRONE_SOURCE_BRANCH:-master}/build.assets/Dockerfile-cron"

	target := "teleport"
	if tp.IsFIPS {
		target += "fips"
	}

	step, stepDetail := tp.createBuildStep("teleport", arch, dockerfile, workingDirectory, target,
		[]string{"DEB_SOURCE=apt", fmt.Sprintf("PACKAGE_VERSION=%q", version.ShellVersion), fmt.Sprintf("PACKAGE_NAME=%q", tp.GetName())},
		version)

	// Add setup commands to download the dockerfile
	step.Commands = append(
		[]string{
			"apk --update --no-cache add curl",
			fmt.Sprintf("curl -Ls -o %q %q", dockerfile, downloadURL),
		},
		step.Commands...,
	)

	return step, stepDetail
}

func (tp *teleportPackage) createBuildStep(buildName, arch, dockerfile, workingDirectory, target string,
	buildArgs []string, version *teleportVersion) (step, *buildStepOutput) {
	packageName := tp.GetName()
	// This makes the image name a little more intuitive
	imageNamePackageSection := ""
	if strings.HasPrefix(packageName, buildName) {
		imageNamePackageSection = strings.TrimPrefix(packageName, buildName)
	}
	imageName := fmt.Sprintf("%s-%s%s-%s", buildName, version.MajorVersion, imageNamePackageSection, arch)
	// workingDirectory := path.Join("/", "go", "build")

	if target == "" {
		target = "''" // Set target to an empty shell string rather than nil
	}

	buildCommand := "docker build"
	buildCommand += " --target " + target
	buildCommand += " --platform linux/" + arch
	buildCommand += " --tag " + imageName
	buildCommand += " --file " + dockerfile
	for _, buildArg := range buildArgs {
		buildCommand += " --build-arg " + buildArg
	}
	buildCommand += " " + workingDirectory

	step := step{
		Name:    fmt.Sprintf("Build %s image %q", buildName, imageName),
		Image:   "docker",
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			fmt.Sprintf("mkdir -p %q && cd %q", workingDirectory, workingDirectory),
			buildCommand,
		},
	}

	return step, &buildStepOutput{
		StepName:        step.Name,
		BuildName:       buildName,
		BuiltImageName:  imageName,
		BuiltImageArch:  arch,
		Version:         version,
		TeleportPackage: tp,
	}
}

// The `step` struct doesn't contain enough information to setup
// dependent steps so we add that via this struct
type buildStepOutput struct {
	StepName        string
	BuildName       string
	BuiltImageName  string
	BuiltImageArch  string
	Version         *teleportVersion
	TeleportPackage *teleportPackage
}

type ContainerRepo struct {
	Name             string
	Environment      map[string]value
	RegistryDomain   string
	LoginCommands    []string
	OssImageName     func(buildName, version string) string
	EntImageName     func(buildName, version string) string
	FipsEntImageName func(buildName, version string) string
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
		OssImageName: func(buildName, majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/%s:%s", domain, buildName, trimV(majorVersion))

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
		EntImageName: func(buildName, majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/%s-ent:%s", domain, buildName, trimV(majorVersion))

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
		FipsEntImageName: func(buildName, majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/%s-ent:%s-fips", domain, buildName, trimV(majorVersion))

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
		OssImageName: func(buildName, majorVersion string) string {
			return fmt.Sprintf("%s/gravitational/%s:%s", ProductionRegistryQuay, buildName, trimV(majorVersion))
		},
		EntImageName: func(buildName, majorVersion string) string {
			return fmt.Sprintf("%s/gravitational/%s-ent:%s", ProductionRegistryQuay, buildName, trimV(majorVersion))
		},
		FipsEntImageName: func(buildName, majorVersion string) string {
			return fmt.Sprintf("%s/gravitational/%s-ent:%s-fips", ProductionRegistryQuay, buildName, trimV(majorVersion))
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

func (cr *ContainerRepo) BuildImageName(buildName, majorVersion string, teleportPackage *teleportPackage) string {
	if !teleportPackage.IsEnterprise {
		return cr.OssImageName(buildName, majorVersion)
	}

	if !teleportPackage.IsFIPS {
		return cr.EntImageName(buildName, majorVersion)
	}

	return cr.FipsEntImageName(buildName, majorVersion)
}

type pushStepOutput struct {
	BuildName       string
	Version         *teleportVersion
	TeleportPackage *teleportPackage
	PushedImageName string
	StepName        string
}

func (cr *ContainerRepo) tagAndPushStep(buildStepDetails *buildStepOutput) (step, *pushStepOutput) {
	repoImageName := fmt.Sprintf("%s-%s", cr.BuildImageName(buildStepDetails.BuildName, buildStepDetails.Version.MajorVersion,
		buildStepDetails.TeleportPackage), buildStepDetails.BuiltImageArch)
	step := step{
		Name:        fmt.Sprintf("Tag and push %q to %s", repoImageName, cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands: cr.buildCommandsWithLogin([]string{
			fmt.Sprintf("docker tag %q %q", buildStepDetails.BuiltImageName, repoImageName),
			fmt.Sprintf("docker push %q", repoImageName),
		}),
		DependsOn: []string{
			buildStepDetails.StepName,
		},
	}

	return step, &pushStepOutput{
		BuildName:       buildStepDetails.BuildName,
		Version:         buildStepDetails.Version,
		TeleportPackage: buildStepDetails.TeleportPackage,
		PushedImageName: repoImageTag,
		StepName:        step.Name,
	}
}

func (cr *ContainerRepo) createAndPushManifestStep(pushStepDetails []*pushStepOutput) step {
	stepDetail := pushStepDetails[0]
	repoImageName := cr.BuildImageName(stepDetail.BuildName, stepDetail.Version.MajorVersion, stepDetail.TeleportPackage)

	manifestCommandArgs := make([]string, 0, len(pushStepDetails))
	pushStepNames := make([]string, 0, len(pushStepDetails))
	for _, pushStepDetail := range pushStepDetails {
		manifestCommandArgs = append(manifestCommandArgs, fmt.Sprintf("--amend %q", pushStepDetail.PushedImageName))
		pushStepNames = append(pushStepNames, pushStepDetail.StepName)
	}

	return step{
		Name:        fmt.Sprintf("Create manifest and push %q to %s", repoImageName, cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands: cr.buildCommandsWithLogin([]string{
			fmt.Sprintf("docker manifest create %q %s", repoImageName, strings.Join(manifestCommandArgs, " ")),
			fmt.Sprintf("docker manifest push %q", repoImageName),
		}),
		DependsOn: pushStepNames,
	}
}

func trimV(semver string) string {
	return strings.TrimPrefix(semver, "v")
}
