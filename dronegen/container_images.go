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
	clonedRepoPath := "/go/src/github.com/gravitational/teleport"

	teleportProducts := []*product{
		NewTeleportProduct(false, false), // OSS
		NewTeleportProduct(true, false),  // Enterprise
		NewTeleportProduct(true, true),   // Enterprise/FIPS
	}
	teleportLabProducts := make([]*product, 0, len(teleportProducts))
	for _, teleportProduct := range teleportProducts {
		teleportLabProducts = append(teleportLabProducts, NewTeleportLabProduct(clonedRepoPath, teleportProduct))
	}
	teleportOperatorProduct := NewTeleportOperatorProduct(clonedRepoPath)

	products := make([]*product, 0, len(teleportProducts)+len(teleportLabProducts)+1)
	products = append(products, teleportProducts...)
	products = append(products, teleportLabProducts...)
	products = append(products, teleportOperatorProduct)

	steps := make([]step, 0)

	setupSteps := []step{
		waitForDockerStep(),
		cloneRepoStep(clonedRepoPath, tv.ShellVersion),
	}
	for _, setupStep := range setupSteps {
		steps = append(steps, setupStep)
		setupStepNames = append(setupStepNames, setupStep.Name)
	}

	for _, product := range products {
		steps = append(steps, product.BuildSteps(tv, setupStepNames)...)
	}

	return steps
}

type product struct {
	Name                 string
	DockerfilePath       string
	WorkingDirectory     string
	DockerfileTarget     string
	BuildSetupCommands   []string
	SupportedArchs       []string
	DockerfileArgBuilder func(arch string, version *teleportVersion) []string
	ImageNameBuilder     func(repo, tag string) string
	GetRequiredStepNames func(arch string, version *teleportVersion) []string
}

func NewTeleportProduct(isEnterprise, isFips bool) *product {
	workingDirectory := "/go/build"
	dockerfile := path.Join(workingDirectory, "Dockerfile")
	downloadURL := "https://raw.githubusercontent.com/gravitational/teleport/${DRONE_SOURCE_BRANCH:-master}/build.assets/charts/Dockerfile"
	target := "teleport"
	if isFips {
		target += "fips"
	}
	name := "teleport"
	packageName := "teleport"
	if isEnterprise {
		packageName += "-ent"
		name += "-ent"
	}
	if isFips {
		name += "-fips"
	}
	supportedArches := []string{"amd64"}
	if !isFips {
		supportedArches = append(supportedArches, "arm", "arm64")
	}

	return &product{
		Name:             name,
		DockerfilePath:   dockerfile,
		WorkingDirectory: workingDirectory,
		DockerfileTarget: target,
		BuildSetupCommands: []string{
			"apk --update --no-cache add curl",
			fmt.Sprintf("curl -Ls -o %q %q", dockerfile, downloadURL),
		},
		SupportedArchs: supportedArches,
		DockerfileArgBuilder: func(arch string, version *teleportVersion) []string {
			return []string{
				"DEB_SOURCE=apt",
				fmt.Sprintf("PACKAGE_VERSION=%s", version.ShellVersion),
				fmt.Sprintf("PACKAGE_NAME=%s", packageName),
			}
		},
		ImageNameBuilder: func(repo, tag string) string {
			if isEnterprise {
				repo += "-ent"
			}

			if isFips {
				tag += "-fips"
			}

			return defaultImageTagBuilder(repo, tag)
		},
	}
}

func NewTeleportLabProduct(cloneDirectory string, teleport *product) *product {
	workingDirectory := "/tmp/build"
	dockerfile := path.Join(cloneDirectory, "docker", "sshd", "Dockerfile")

	return &product{
		Name:             "teleport-lab",
		DockerfilePath:   dockerfile,
		WorkingDirectory: workingDirectory,
		SupportedArchs:   teleport.SupportedArchs,
		DockerfileArgBuilder: func(arch string, version *teleportVersion) []string {
			return []string{
				fmt.Sprintf("BASE_IMAGE=%s", teleport.BuildLocalImageName(arch, version)),
			}
		},
		ImageNameBuilder: defaultImageTagBuilder,
		GetRequiredStepNames: func(arch string, version *teleportVersion) []string {
			return []string{teleport.GetBuildStepName(arch, version)}
		},
	}
}

func NewTeleportOperatorProduct(cloneDirectory string) *product {
	return &product{
		Name:             "teleport-operator",
		DockerfilePath:   path.Join(cloneDirectory, "operator", "Dockerfile"),
		WorkingDirectory: cloneDirectory,
		SupportedArchs:   []string{"amd64", "arm", "arm64"},
		ImageNameBuilder: defaultImageTagBuilder,
	}
}

func defaultImageTagBuilder(repo, tag string) string {
	return fmt.Sprintf("%s:%s", repo, tag)
}

func (p *product) BuildLocalImageName(arch string, version *teleportVersion) string {
	return fmt.Sprintf("%s-%s-%s", p.Name, version.MajorVersion, arch)
}

func (p *product) BuildSteps(version *teleportVersion, setupStepNames []string) []step {
	containerRepos := GetContainerRepos()

	steps := make([]step, 0)

	archBuildStepDetails := make([]*buildStepOutput, 0, len(p.SupportedArchs))
	for _, supportedArch := range p.SupportedArchs {
		archBuildStep, archBuildStepDetail := p.createBuildStep(supportedArch, version)

		archBuildStep.DependsOn = append(archBuildStep.DependsOn, setupStepNames...)
		if p.GetRequiredStepNames != nil {
			archBuildStep.DependsOn = append(archBuildStep.DependsOn, p.GetRequiredStepNames(supportedArch, version)...)
		}

		steps = append(steps, archBuildStep)
		archBuildStepDetails = append(archBuildStepDetails, archBuildStepDetail)
	}

	for _, containerRepo := range containerRepos {
		steps = append(steps, containerRepo.buildSteps(archBuildStepDetails)...)
	}

	return steps
}

func (p *product) GetBuildStepName(arch string, version *teleportVersion) string {
	return fmt.Sprintf("Build %s image %q", p.Name, p.BuildLocalImageName(arch, version))
}

func (p *product) createBuildStep(arch string, version *teleportVersion) (step, *buildStepOutput) {
	imageName := p.BuildLocalImageName(arch, version)

	if p.DockerfileTarget == "" {
		p.DockerfileTarget = "''" // Set target to an empty shell string rather than shell nil
	}

	buildCommand := "docker build"
	buildCommand += fmt.Sprintf(" --target %q", p.DockerfileTarget)
	buildCommand += fmt.Sprintf(" --platform %q", "linux/"+arch)
	buildCommand += fmt.Sprintf(" --tag %q", imageName)
	buildCommand += fmt.Sprintf(" --file %q", p.DockerfilePath)
	if p.DockerfileArgBuilder != nil {
		for _, buildArg := range p.DockerfileArgBuilder(arch, version) {
			buildCommand += fmt.Sprintf(" --build-arg %q", buildArg)
		}
		buildCommand += " " + p.WorkingDirectory
	}

	step := step{
		Name:    p.GetBuildStepName(arch, version),
		Image:   "docker",
		Volumes: dockerVolumeRefs(),
		Commands: append(p.BuildSetupCommands,
			[]string{
				fmt.Sprintf("mkdir -p %q && cd %q", p.WorkingDirectory, p.WorkingDirectory),
				buildCommand,
			}...,
		),
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
	Version        *teleportVersion
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
