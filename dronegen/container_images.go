package main

import (
	"fmt"
	"path"
	"strings"
)

func cronBuildContainerImagePipelines() []pipeline {
	// This needs to be updated on each major release.
	teleportVersions := []teleportVersion{
		{MajorVersion: "v11", RelativeVersionName: "current-version"},
		{MajorVersion: "v10", RelativeVersionName: "previous-version-one"},
		{MajorVersion: "v9", RelativeVersionName: "previous-version-two"},
	}

	pipelines := make([]pipeline, 0, len(teleportVersions))
	for _, teleportVersion := range teleportVersions {
		pipelines = append(pipelines, teleportVersion.BuildVersionPipeline())
	}

	return pipelines
}

type teleportVersion struct {
	MajorVersion        string
	RelativeVersionName string // The set of values for this should not change between major releases
}

func (tv *teleportVersion) BuildVersionPipeline() pipeline {
	pipelineName := fmt.Sprintf("teleport-docker-cron-%s", tv.RelativeVersionName)

	pipeline := newKubePipeline(pipelineName)
	pipeline.Type = "docker"
	pipeline.Trigger = cronTrigger([]string{pipelineName})
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
		steps = append(steps, teleportPackage.BuildSteps(tv.MajorVersion, setupStep.Name)...)
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

func (tp *teleportPackage) BuildSteps(majorVersion, setupStep string) []step {
	supportedArchs := []string{"amd64", "i386", "arm64", "arm"}
	containerRepos := GetContainerRepos()

	steps := make([]step, 0)

	buildStepDetails := make([]*buildStepOutput, 0, len(supportedArchs))
	for _, supportedArch := range supportedArchs {
		// FIPS is only supported on AMD64 currently
		if tp.IsFIPS && supportedArch != "amd64" {
			continue
		}

		buildArchStep, buildArchStepDetails := tp.buildArchStep(majorVersion, supportedArch)
		buildArchStep.DependsOn = []string{setupStep}
		steps = append(steps, buildArchStep)

		buildStepDetails = append(buildStepDetails, buildArchStepDetails)
	}

	for _, containerRepo := range containerRepos {
		steps = append(steps, containerRepo.BuildSteps(buildStepDetails)...)
	}

	return steps
}

func (tp *teleportPackage) buildArchStep(majorVersion, arch string) (step, *buildStepOutput) {
	packageName := tp.GetName()
	imageName := fmt.Sprintf("teleport-%s-%s-%s", majorVersion, packageName, arch)
	workingDirectory := path.Join("/", "go", "build")
	dockerfile := path.Join(workingDirectory, "Dockerfile-cron")
	// Other dockerfiles can be added/configured here if needed in the future
	downloadUrl := "https://raw.githubusercontent.com/gravitational/teleport/${DRONE_SOURCE_BRANCH:-master}/build.assets/Dockerfile-cron"

	step := step{
		Name:    fmt.Sprintf("Build Teleport image %q", imageName),
		Image:   "docker",
		Volumes: dockerVolumeRefs(),
		Commands: []string{
			"apk --update --no-cache add curl",
			fmt.Sprintf("mkdir -p %q && cd %q", workingDirectory, workingDirectory),
			fmt.Sprintf("curl -Ls -o %q %q", dockerfile, downloadUrl),
			strings.Join([]string{
				"docker build",
				"--target teleport",
				fmt.Sprintf("--platform linux/%s", arch),
				fmt.Sprintf("--build-arg MAJOR_VERSION=%q", majorVersion),
				fmt.Sprintf("--build-arg PACKAGE_NAME=%q", packageName),
				fmt.Sprintf("--tag %q", imageName),
				fmt.Sprintf("--file %q", dockerfile),
				workingDirectory,
			}, " "),
		},
	}

	return step, &buildStepOutput{
		StepName:        step.Name,
		BuiltImageArch:  arch,
		BuiltImageName:  imageName,
		MajorVersion:    majorVersion,
		TeleportPackage: tp,
	}
}

// The `step` struct doesn't contain enough information to setup
// dependent steps so we add that via this struct
type buildStepOutput struct {
	StepName        string
	BuiltImageName  string
	BuiltImageArch  string
	MajorVersion    string
	TeleportPackage *teleportPackage
}

type containerRepo struct {
	Name             string
	Environment      map[string]value
	RegistryDomain   string
	LoginCommands    []string
	OssImageName     func(majorVersion string) string
	EntImageName     func(majorVersion string) string
	FipsEntImageName func(majorVersion string) string
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
			"TIMESTAMP=$(date '+%Y%m%d%H%M')",
			fmt.Sprintf("aws ecr get-login-password --region=us-west-2 | docker login -u=\"AWS\" --password-stdin %s", domain),
		},
		OssImageName: func(majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/teleport:%s", domain, majorVersion)

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
		EntImageName: func(majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/teleport-ent:%s", domain, majorVersion)

			if !isStaging {
				return baseTag
			}
			return fmt.Sprintf("%s-%s", baseTag, "$TIMESTAMP")
		},
		FipsEntImageName: func(majorVersion string) string {
			baseTag := fmt.Sprintf("%s/gravitational/teleport-ent:%s-fips", domain, majorVersion)

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
		OssImageName: func(majorVersion string) string {
			return fmt.Sprintf("quay.io/gravitational/teleport:%s", majorVersion)
		},
		EntImageName: func(majorVersion string) string {
			return fmt.Sprintf("quay.io/gravitational/teleport-ent:%s", majorVersion)
		},
		FipsEntImageName: func(majorVersion string) string {
			return fmt.Sprintf("quay.io/gravitational/teleport-ent:%s-fips", majorVersion)
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

func (cr *containerRepo) BuildImageName(majorVersion string, teleportPackage *teleportPackage) string {
	if !teleportPackage.IsEnterprise {
		return cr.OssImageName(majorVersion)
	}

	if !teleportPackage.IsFIPS {
		return cr.EntImageName(majorVersion)
	}

	return cr.FipsEntImageName(majorVersion)
}

type pushStepOutput struct {
	MajorVersion    string
	TeleportPackage *teleportPackage
	PushedImageName string
	StepName        string
}

func (cr *containerRepo) tagAndPushStep(buildStepDetails *buildStepOutput) (step, *pushStepOutput) {
	repoImageTag := fmt.Sprintf("%s-%s", cr.BuildImageName(buildStepDetails.MajorVersion, buildStepDetails.TeleportPackage), buildStepDetails.BuiltImageArch)
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
		MajorVersion:    buildStepDetails.MajorVersion,
		TeleportPackage: buildStepDetails.TeleportPackage,
		PushedImageName: repoImageTag,
		StepName:        step.Name,
	}
}

func (cr *containerRepo) createAndPushManifestStep(pushStepDetails []*pushStepOutput) step {
	repoImageTag := cr.BuildImageName(pushStepDetails[0].MajorVersion, pushStepDetails[0].TeleportPackage)

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

// func teleportLabSteps() []step {
// 	containerRepos := GetContainerRepos()
// 	steps := make([]step, 0, len(containerRepos))

// 	for _, containerRepo := range containerRepos {
// 		steps = append(steps, step{
// 			Name: fmt.Sprintf("Build teleport lab for "),
// 		})
// 	}
// }
