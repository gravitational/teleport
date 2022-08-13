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

	triggers := []*triggerInfo{
		NewPromoteTrigger(branchMajorVersion),
		NewCronTrigger(latestMajorVersions),
	}

	pipelines := make([]pipeline, 0, len(triggers))
	for _, trigger := range triggers {
		pipelines = append(pipelines, trigger.BuildPipelines()...)
	}

	return pipelines
}

type triggerInfo struct {
	Trigger           trigger
	Name              string
	SupportedVersions []*teleportVersion
}

func NewPromoteTrigger(branchMajorVersion string) *triggerInfo {
	promoteTrigger := triggerPromote
	promoteTrigger.Target.Include = append(promoteTrigger.Target.Include, "promote-docker")

	return &triggerInfo{
		Trigger: promoteTrigger,
		Name:    "promote",
		SupportedVersions: []*teleportVersion{
			{
				MajorVersion:        branchMajorVersion,
				SearchVersion:       "$DRONE_TAG",
				RelativeVersionName: "drone-tag",
			},
		},
	}
}

func NewCronTrigger(latestMajorVersions []string) *triggerInfo {
	if len(latestMajorVersions) == 0 {
		return nil
	}

	supportedVersions := make([]*teleportVersion, 0, len(latestMajorVersions))
	if len(latestMajorVersions) > 0 {
		supportedVersions = append(supportedVersions, &teleportVersion{
			MajorVersion:        latestMajorVersions[0],
			SearchVersion:       latestMajorVersions[0],
			RelativeVersionName: "current-version",
		})

		if len(latestMajorVersions) > 1 {
			for i, latestMajorVersion := range latestMajorVersions[1:] {
				supportedVersions = append(supportedVersions, &teleportVersion{
					MajorVersion:        latestMajorVersion,
					SearchVersion:       latestMajorVersion,
					RelativeVersionName: fmt.Sprintf("previous-version-%d", i+1),
				})
			}
		}
	}

	return &triggerInfo{
		Trigger:           cronTrigger([]string{"teleport-container-images-cron"}),
		Name:              "cron",
		SupportedVersions: supportedVersions,
	}
}

// Drone triggers must all evaluate to "true" for a pipeline to be executed.
// As a result these pipelines are duplicated for each trigger.
// See https://docs.drone.io/pipeline/triggers/ for details.
func (ti *triggerInfo) BuildPipelines() []pipeline {
	pipelines := make([]pipeline, 0, len(ti.SupportedVersions))
	for _, teleportVersion := range ti.SupportedVersions {
		pipeline := teleportVersion.BuildVersionPipeline()
		pipeline.Name += "-" + ti.Name
		pipeline.Trigger = ti.Trigger

		pipelines = append(pipelines, pipeline)
	}

	return pipelines
}

type teleportVersion struct {
	MajorVersion        string // This is the major version of a given build. `SearchVersion` should match this when evaluated.
	SearchVersion       string // This value will be evaluated by the shell in the context of a Drone step
	RelativeVersionName string // The set of values for this should not change between major releases
}

func (tv *teleportVersion) BuildVersionPipeline() pipeline {
	pipelineName := fmt.Sprintf("teleport-container-images-%s", tv.RelativeVersionName)

	trigger := cronTrigger([]string{pipelineName})
	promoteTrigger := triggerPromote
	trigger.Event = promoteTrigger.Event
	trigger.Target = promoteTrigger.Target

	pipeline := newKubePipeline(pipelineName)
	pipeline.Trigger = trigger
	pipeline.Workspace = workspace{Path: "/go"}
	pipeline.Services = []service{dockerService()}
	pipeline.Volumes = dockerVolumes()
	pipeline.Steps = tv.buildSteps()

	return pipeline
}

func (tv *teleportVersion) buildSteps() []step {
	teleportPackages := []teleportPackage{
		{IsEnterprise: false, IsFIPS: false}, // OSS
		{IsEnterprise: true, IsFIPS: false},  // Enterprise
		{IsEnterprise: true, IsFIPS: true},   // Enterprise/FIPS
	}
	steps := make([]step, 0)

	setupStep := waitForDockerStep()
	steps = append(steps, setupStep)

	for _, teleportPackage := range teleportPackages {
		steps = append(steps, teleportPackage.BuildSteps(tv, setupStep.Name)...)
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

func (tp *teleportPackage) BuildSteps(version *teleportVersion, setupStep string) []step {
	// The base image (ubuntu:20.04) does not offer i386 images so we don't either
	supportedArchs := []string{
		"amd64",
		"arm64",
		"arm",
	}
	containerRepos := GetContainerRepos()

	steps := make([]step, 0)

	teleportBuildStepDetails := make([]*buildStepOutput, 0, len(supportedArchs))
	labBuildStepDetails := make([]*buildStepOutput, 0, len(supportedArchs))
	for _, supportedArch := range supportedArchs {
		// FIPS is only supported on AMD64 currently
		if tp.IsFIPS && supportedArch != "amd64" {
			continue
		}

		// Setup Teleport build steps
		teleportBuildArchStep, teleportBuildArchStepDetails := tp.buildTeleportArchStep(version, supportedArch)
		teleportBuildArchStep.DependsOn = []string{setupStep}
		steps = append(steps, teleportBuildArchStep)
		teleportBuildStepDetails = append(teleportBuildStepDetails, teleportBuildArchStepDetails)

		// Setup Teleport lab build steps
		// Only use OSS for now as that's what we currently support
		if tp.IsEnterprise || tp.IsFIPS {
			continue
		}

		labBuildArchStep, labBuildArchStepDetails := tp.buildTeleportLabArchStep(teleportBuildArchStepDetails)
		steps = append(steps, labBuildArchStep)
		labBuildStepDetails = append(labBuildStepDetails, labBuildArchStepDetails)
	}

	for _, containerRepo := range containerRepos {
		steps = append(steps, containerRepo.BuildSteps(teleportBuildStepDetails)...)
		steps = append(steps, containerRepo.BuildSteps(labBuildStepDetails)...)
	}

	return steps
}

func (tp *teleportPackage) buildTeleportLabArchStep(teleportBuildStepDetail *buildStepOutput) (step, *buildStepOutput) {
	dockerfile := "/go/src/github.com/gravitational/teleport/docker/sshd/Dockerfile"

	step, stepDetail := tp.createBuildStep("teleport-lab", teleportBuildStepDetail.BuiltImageArch,
		dockerfile, "", []string{fmt.Sprintf("BASE_IMAGE=%q", teleportBuildStepDetail.BuiltImageName)}, teleportBuildStepDetail.Version)
	step.Commands = append(
		cloneRepoCommands(),
		step.Commands...,
	)
	step.DependsOn = []string{teleportBuildStepDetail.StepName}

	return step, stepDetail
}

func (tp *teleportPackage) buildTeleportArchStep(version *teleportVersion, arch string) (step, *buildStepOutput) {
	workingDirectory := path.Join("/", "go", "build")
	dockerfile := path.Join(workingDirectory, "Dockerfile-cron")
	// Other dockerfiles can be added/configured here if needed in the future
	downloadUrl := "https://raw.githubusercontent.com/gravitational/teleport/${DRONE_SOURCE_BRANCH:-master}/build.assets/Dockerfile-cron"

	target := "teleport"
	if tp.IsFIPS {
		target += "fips"
	}

	step, stepDetail := tp.createBuildStep("teleport", arch, dockerfile, target,
		[]string{"DEB_SOURCE=apt", fmt.Sprintf("PACKAGE_VERSION=%q", version.SearchVersion), fmt.Sprintf("PACKAGE_NAME=%q", tp.GetName())},
		version)

	// Add setup commands to download the dockerfile
	step.Commands = append(
		[]string{
			"apk --update --no-cache add curl",
			fmt.Sprintf("curl -Ls -o %q %q", dockerfile, downloadUrl),
		},
		step.Commands...,
	)

	return step, stepDetail
}

func (tp *teleportPackage) createBuildStep(buildName, arch, dockerfile, target string, buildArgs []string, version *teleportVersion) (step, *buildStepOutput) {
	packageName := tp.GetName()
	// This makes the image name a little more intuitive
	imageNamePackageSection := ""
	if strings.HasPrefix(packageName, buildName) {
		imageNamePackageSection = strings.TrimPrefix(packageName, buildName)
	}
	imageName := fmt.Sprintf("%s-%s%s-%s", buildName, version.MajorVersion, imageNamePackageSection, arch)
	workingDirectory := path.Join("/", "go", "build")

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

type containerRepo struct {
	Name             string
	Environment      map[string]value
	RegistryDomain   string
	LoginCommands    []string
	OssImageName     func(buildName, version string) string
	EntImageName     func(buildName, version string) string
	FipsEntImageName func(buildName, version string) string
}

func NewEcrContainerRepo(name, accessKeyIdSecret, secretAccessKeySecret, domain string, isStaging bool) *containerRepo {
	return &containerRepo{
		Name: fmt.Sprintf("ECR - %s", name),
		Environment: map[string]value{
			"AWS_ACCESS_KEY_ID": {
				fromSecret: accessKeyIdSecret,
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
			baseTag := fmt.Sprintf("%s/gravitational/%s:%s", buildName, domain, trimV(majorVersion))

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
		EntImageName: func(buildName, majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/%s-ent:%s", buildName, domain, trimV(majorVersion))

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
		FipsEntImageName: func(buildName, majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/%s-ent:%s-fips", buildName, domain, trimV(majorVersion))

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
	}
}

func NewQuayContainerRepo(dockerUsername, dockerPassword string) *containerRepo {
	return &containerRepo{
		Name: "Quay",
		Environment: map[string]value{
			"QUAY_USERNAME": {
				fromSecret: dockerUsername,
			},
			"QUAY_PASSWORD": {
				fromSecret: dockerPassword,
			},
		},
		RegistryDomain: "quay.io",
		LoginCommands: []string{
			"docker login -u=\"$QUAY_USERNAME\" -p=\"$QUAY_PASSWORD\" \"quay.io\"",
		},
		OssImageName: func(buildName, majorVersion string) string {
			return fmt.Sprintf("quay.io/gravitational/%s:%s", buildName, trimV(majorVersion))
		},
		EntImageName: func(buildName, majorVersion string) string {
			return fmt.Sprintf(buildName, "quay.io/gravitational/%s-ent:%s", buildName, trimV(majorVersion))
		},
		FipsEntImageName: func(buildName, majorVersion string) string {
			return fmt.Sprintf("quay.io/gravitational/%s-ent:%s-fips", buildName, trimV(majorVersion))
		},
	}
}

func GetContainerRepos() []*containerRepo {
	return []*containerRepo{
		NewQuayContainerRepo("PRODUCTION_QUAYIO_DOCKER_USERNAME", "PRODUCTION_QUAYIO_DOCKER_PASSWORD"),
		NewEcrContainerRepo("staging", "STAGING_TELEPORT_DRONE_USER_ECR_KEY", "STAGING_TELEPORT_DRONE_USER_ECR_SECRET", "146628656107.dkr.ecr.us-west-2.amazonaws.com", true),
		NewEcrContainerRepo("production", "PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY", "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET", "public.ecr.aws", false),
	}
}

func (cr *containerRepo) BuildSteps(buildStepDetails []*buildStepOutput) []step {
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

func (cr *containerRepo) logoutCommand() string {
	return fmt.Sprintf("docker logout %q", cr.RegistryDomain)
}

func (cr *containerRepo) buildCommandsWithLogin(wrappedCommands []string) []string {
	commands := make([]string, 0)
	commands = append(commands, cr.LoginCommands...)
	commands = append(commands, wrappedCommands...)
	commands = append(commands, cr.logoutCommand())

	return commands
}

func (cr *containerRepo) BuildImageName(buildName, majorVersion string, teleportPackage *teleportPackage) string {
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

func (cr *containerRepo) tagAndPushStep(buildStepDetails *buildStepOutput) (step, *pushStepOutput) {
	repoImageTag := fmt.Sprintf("%s-%s", cr.BuildImageName(buildStepDetails.BuildName, buildStepDetails.Version.MajorVersion,
		buildStepDetails.TeleportPackage), buildStepDetails.BuiltImageArch)
	step := step{
		Name:        fmt.Sprintf("Tag and push %q to %s", repoImageTag, cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands: cr.buildCommandsWithLogin([]string{
			fmt.Sprintf("docker tag %q %q", buildStepDetails.BuiltImageName, repoImageTag),
			fmt.Sprintf("docker push %q", repoImageTag),
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

func (cr *containerRepo) createAndPushManifestStep(pushStepDetails []*pushStepOutput) step {
	stepDetail := pushStepDetails[0]
	repoImageTag := cr.BuildImageName(stepDetail.BuildName, stepDetail.Version.MajorVersion, stepDetail.TeleportPackage)

	manifestCommandArgs := make([]string, 0, len(pushStepDetails))
	pushStepNames := make([]string, 0, len(pushStepDetails))
	for _, pushStepDetail := range pushStepDetails {
		manifestCommandArgs = append(manifestCommandArgs, fmt.Sprintf("--amend %q", pushStepDetail.PushedImageName))
		pushStepNames = append(pushStepNames, pushStepDetail.StepName)
	}

	return step{
		Name:        fmt.Sprintf("Create manifest and push %q to %s", repoImageTag, cr.Name),
		Image:       "docker",
		Volumes:     dockerVolumeRefs(),
		Environment: cr.Environment,
		Commands: cr.buildCommandsWithLogin([]string{
			fmt.Sprintf("docker manifest create %q %s", repoImageTag, strings.Join(manifestCommandArgs, " ")),
			fmt.Sprintf("docker manifest push %q", repoImageTag),
		}),
		DependsOn: pushStepNames,
	}
}

func trimV(semver string) string {
	return strings.TrimPrefix(semver, "v")
}
