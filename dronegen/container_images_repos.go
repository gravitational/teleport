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
	"sort"
	"strings"

	"golang.org/x/exp/maps"
)

// Describes a registry and repo that images are to be published to.
type ContainerRepo struct {
	Name            string                            // Human readable name for the repo. Does not need to match remote value.
	IsImmutable     bool                              // True if the repo supports updating existing tags, false otherwise
	EnvironmentVars map[string]value                  // Steps that use the described repo should include these env vars
	RegistryDomain  string                            // The registry that hosts the container repo
	RegistryOrg     string                            // The organization name (usually "gravitational") that the repo is listed under
	SetupSteps      []step                            // Optional field that can be used to run setup code prior to first login
	LoginCommands   []string                          // Commands to authenticate the docker daemon with the repo
	TagBuilder      func(baseTag *ImageTag) *ImageTag // Postprocessor for tags that append CR-specific suffixes
}

func NewEcrContainerRepo(accessKeyIDSecret, secretAccessKeySecret, roleSecret, domain, name string,
	isPublic, isImmutable, guaranteeUnique bool) *ContainerRepo {
	ecrRegion := StagingEcrRegion
	loginSubcommand := "ecr"
	if isPublic {
		ecrRegion = PublicEcrRegion
		loginSubcommand = "ecr-public"
	}

	repoName := fmt.Sprintf("ECR - %s", name)
	profileName := fmt.Sprintf("ecr-%s", name)

	registryOrg := ProductionRegistryOrg
	if configureForPRTestingOnly {
		accessKeyIDSecret = testingSecretPrefix + accessKeyIDSecret
		secretAccessKeySecret = testingSecretPrefix + secretAccessKeySecret
		roleSecret = testingSecretPrefix + roleSecret
		registryOrg = testingECRRegistryOrg

		if !isPublic {
			domain = testingECRDomain
			ecrRegion = testingECRRegion
		}
	}

	loginCommands := []string{
		"apk add --no-cache aws-cli",
		fmt.Sprintf("aws %s get-login-password --region=%s | docker login -u=\"AWS\" --password-stdin %s", loginSubcommand, ecrRegion, domain),
		`printenv DOCKERHUB_PASSWORD | docker login -u="$DOCKERHUB_USERNAME" --password-stdin`,
	}

	if guaranteeUnique {
		loginCommands = append(loginCommands, "TIMESTAMP=$(date -d @\"$DRONE_BUILD_CREATED\" '+%Y%m%d%H%M')")
	}

	return &ContainerRepo{
		Name:        repoName,
		IsImmutable: isImmutable,
		EnvironmentVars: map[string]value{
			"AWS_PROFILE":        {raw: profileName},
			"DOCKERHUB_USERNAME": {fromSecret: "DOCKERHUB_USERNAME"},
			"DOCKERHUB_PASSWORD": {fromSecret: "DOCKERHUB_READONLY_TOKEN"},
		},
		RegistryDomain: domain,
		RegistryOrg:    registryOrg,
		SetupSteps: []step{
			kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
				awsRoleSettings: awsRoleSettings{
					awsAccessKeyID:     value{fromSecret: accessKeyIDSecret},
					awsSecretAccessKey: value{fromSecret: secretAccessKeySecret},
					role:               value{fromSecret: roleSecret},
				},
				configVolume: volumeRefAwsConfig,
				profile:      profileName,
				name:         fmt.Sprintf("Assume %s AWS Role", repoName),
				append:       true,
			}),
		},
		LoginCommands: loginCommands,
		TagBuilder: func(tag *ImageTag) *ImageTag {
			if guaranteeUnique {
				tag.AppendString("$TIMESTAMP")
			}

			return tag
		},
	}
}

func NewLocalContainerRepo() *ContainerRepo {
	return &ContainerRepo{
		Name:           "Local Registry",
		IsImmutable:    false,
		RegistryDomain: LocalRegistrySocket,
	}
}

func GetLocalContainerRepo() *ContainerRepo {
	return NewLocalContainerRepo()
}

func GetStagingContainerRepo(uniqueStagingTag bool) *ContainerRepo {
	return NewEcrContainerRepo("STAGING_TELEPORT_DRONE_USER_ECR_KEY", "STAGING_TELEPORT_DRONE_USER_ECR_SECRET",
		"STAGING_TELEPORT_DRONE_ECR_AWS_ROLE", StagingRegistry, "staging", false, true, uniqueStagingTag)
}

func GetProductionContainerRepos() []*ContainerRepo {
	return []*ContainerRepo{
		NewEcrContainerRepo("PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY", "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET",
			"PRODUCTION_TELEPORT_DRONE_ECR_AWS_ROLE", ProductionRegistry, "production", true, false, false),
	}
}

// This is a special case of "public.ecr.aws". This references a public ECR repo that may only ever be pulled from.
// The purpose of this is to authenticate with public ECR prior to `docker buildx build` so that the build command
// will pull from the repo as an authenticated user. Pulling as an authenticated user greatly increase the number
// of layers that can be pulled per second, which fixes certain issues with running build commands in parallel.
func GetPublicEcrPullRegistry() *ContainerRepo {
	// Note: these credentials currently allow for push and pull. I'd recommend either a separate role or set of
	// credentials for pull only access.
	return NewEcrContainerRepo("PRODUCTION_TELEPORT_DRONE_USER_ECR_KEY", "PRODUCTION_TELEPORT_DRONE_USER_ECR_SECRET",
		"PRODUCTION_TELEPORT_DRONE_ECR_AWS_ROLE", ProductionRegistry, "authenticated-pull", true, false, false)
}

func (cr *ContainerRepo) buildSteps(buildStepDetails []*buildStepOutput, flags *TriggerFlags) []step {
	// This is used to grab information that is (or at least should be by design) the same for all values in the slice
	if len(buildStepDetails) == 0 {
		return nil
	}
	sourceBuildStep := buildStepDetails[0]

	steps := make([]step, 0)

	// Tag and push, collecting the names of the tag/push steps and the images pushed.
	imageTags := cr.BuildImageTags(sourceBuildStep.Version, flags)
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

	// Create and push a manifest for each tag, referencing multiple architectures in the manifest
	for _, imageTag := range imageTags {
		multiarchImageTag := *imageTag
		multiarchImageTag.Arch = ""
		manifestImage := buildStepDetails[0].Product.ImageBuilder(cr, &multiarchImageTag)
		manifestStep := cr.createAndPushManifestStep(manifestImage, pushStepNames, pushedImages[imageTag])

		// Only create and push manifest for major and minor versions if the release version is not a prerelease
		if !flags.ShouldOnlyPublishFullSemver && !imageTag.IsForFullSemver {
			manifestStep.Commands = buildPrereleaseExclusionaryCommands(sourceBuildStep.Version, manifestStep.Commands)
		}

		steps = append(steps, manifestStep)
	}

	return steps
}

func (cr *ContainerRepo) logoutCommand() string {
	return fmt.Sprintf("docker logout %q", cr.RegistryDomain)
}

func (cr *ContainerRepo) buildCommandsWithLogin(wrappedCommands []string) []string {
	if cr.LoginCommands == nil || len(cr.LoginCommands) == 0 {
		return wrappedCommands
	}

	commands := make([]string, 0)
	commands = append(commands, cr.LoginCommands...)
	commands = append(commands, wrappedCommands...)
	commands = append(commands, cr.logoutCommand())

	return commands
}

func (cr *ContainerRepo) BuildImageRepo() string {
	return fmt.Sprintf("%s/%s/", cr.RegistryDomain, cr.RegistryOrg)
}

func (cr *ContainerRepo) BuildImageTags(version *ReleaseVersion, flags *TriggerFlags) []*ImageTag {
	tags := version.getTagsForVersion(flags.ShouldOnlyPublishFullSemver)

	if cr.TagBuilder != nil {
		for i, tag := range tags {
			tags[i] = cr.TagBuilder(tag)
		}
	}

	return tags
}

// Pulls an image with authentication pushes it to the local repo.
// Does not generate additional tags.
// Returns an *Image struct describing the locally pushed image.
func (cr *ContainerRepo) pullPushStep(image *Image, dependencySteps []string) (step, *Image) {
	localRepo := GetLocalContainerRepo()
	localRepoImage := *image
	localRepoImage.Repo = localRepo

	commands := image.Repo.buildCommandsWithLogin([]string{fmt.Sprintf("docker pull %s", image.GetShellName())})
	commands = append(commands,
		fmt.Sprintf("docker tag %s %s", image.GetShellName(), localRepoImage.GetShellName()),
		fmt.Sprintf("docker push %s", localRepoImage.GetShellName()),
	)

	return step{
		Name:        fmt.Sprintf("Pull %s and push it to %s", image.GetDisplayName(), localRepo.Name),
		Image:       "docker",
		Volumes:     []volumeRef{volumeRefAwsConfig, volumeRefDocker}, // no docker config volume, as this will race
		Environment: cr.EnvironmentVars,
		Commands:    commands,
		DependsOn:   dependencySteps,
	}, &localRepoImage
}

func (cr *ContainerRepo) tagAndPushStep(buildStepDetails *buildStepOutput, imageTags []*ImageTag) (step, map[*ImageTag]*Image) {
	archImageMap := make(map[*ImageTag]*Image, len(imageTags))
	for _, imageTag := range imageTags {
		archTag := *imageTag
		archTag.Arch = buildStepDetails.BuiltImage.Tag.Arch
		archImage := buildStepDetails.Product.ImageBuilder(cr, &archTag)
		archImageMap[imageTag] = archImage
	}

	// This is tracked separately as maps in golang have a non-deterministic order when iterated over.
	// As a result, .drone.yml will be updated every time `make dronegen` is ran regardless of if there
	// is a change to the map or not
	// The order/comparator does not matter here as long as it is deterministic between dronegen runs
	archImageKeys := maps.Keys(archImageMap)
	sort.SliceStable(archImageKeys, func(i, j int) bool { return archImageKeys[i].GetDisplayValue() < archImageKeys[j].GetDisplayValue() })

	platform := fmt.Sprintf("linux/%s", buildStepDetails.BuiltImage.Tag.Arch)
	pullCommands := []string{
		fmt.Sprintf("docker pull --platform %q %s", platform, buildStepDetails.BuiltImage.GetShellName()),
	}

	tagAndPushCommands := make([]string, 0)
	for _, archImageKey := range archImageKeys {
		archImage := archImageMap[archImageKey]

		// Skip pushing images if the tag or container registry is immutable
		tagAndPushCommands = append(tagAndPushCommands, buildImmutableSafeCommands(archImageKey.IsImmutable || cr.IsImmutable, archImage.GetShellName(), []string{
			fmt.Sprintf("docker tag %s %s", buildStepDetails.BuiltImage.GetShellName(), archImage.GetShellName()),
			fmt.Sprintf("docker push %s", archImage.GetShellName()),
		})...)
	}
	tagAndPushCommands = cr.buildCommandsWithLogin(tagAndPushCommands)

	commands := append(pullCommands, tagAndPushCommands...)

	dependencySteps := []string{}
	if buildStepDetails.StepName != "" {
		dependencySteps = append(dependencySteps, buildStepDetails.StepName)
	}

	step := step{
		Name:        fmt.Sprintf("Tag and push image %q to %s", buildStepDetails.BuiltImage.GetDisplayName(), cr.Name),
		Image:       "docker",
		Volumes:     []volumeRef{volumeRefAwsConfig, volumeRefDocker}, // no docker config volume, as this will race
		Environment: cr.EnvironmentVars,
		Commands:    commands,
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
		manifestCommandArgs = append(manifestCommandArgs, fmt.Sprintf("--amend %s", pushedImage.GetShellName()))
	}

	// Skip pushing manifest if the tag or container registry is immutable
	commands := buildImmutableSafeCommands(manifestImage.Tag.IsImmutable || cr.IsImmutable, manifestImage.GetShellName(), []string{
		fmt.Sprintf("docker manifest create %s %s", manifestImage.GetShellName(), strings.Join(manifestCommandArgs, " ")),
		fmt.Sprintf("docker manifest push %s", manifestImage.GetShellName()),
	})

	return step{
		Name:        fmt.Sprintf("Create manifest and push %q to %s", manifestImage.GetDisplayName(), cr.Name),
		Image:       "docker",
		Volumes:     []volumeRef{volumeRefAwsConfig, volumeRefDocker}, // no docker config volume, as this will race
		Environment: cr.EnvironmentVars,
		Commands:    cr.buildCommandsWithLogin(commands),
		DependsOn:   pushStepNames,
	}
}

func buildImmutableSafeCommands(isImmutable bool, imageToCheck string, commandsToRun []string) []string {
	if !isImmutable {
		return commandsToRun
	}

	conditionalCommand := fmt.Sprintf("docker manifest inspect %s > /dev/null 2>&1", imageToCheck)
	commandToRun := strings.Join(commandsToRun, " && ")
	return []string{fmt.Sprintf("%s && echo 'Found existing image, skipping' || (%s)", conditionalCommand, commandToRun)}
}

// Modifies a set of commands to only be ran if `version.ShellIsPrerelease` evaluates at runtiem to false.
func buildPrereleaseExclusionaryCommands(version *ReleaseVersion, commandsToRun []string) []string {
	// If no check is defined, just pass the commands through without a check
	if version.ShellIsPrerelease == "" {
		return commandsToRun
	}

	checkCommands := []string{
		fmt.Sprintf(`printf "Prerelease "; ! %s && printf "not "; printf "detected for version %s, "; %s && echo "skipping" || echo "continuing"`,
			version.ShellIsPrerelease, version.ShellVersion, version.ShellIsPrerelease),
		// This will cause the step to exit without error, allowing future steps to continue without killing the pipeline
		fmt.Sprintf("%s && exit 0", version.ShellIsPrerelease),
	}

	return append(checkCommands, commandsToRun...)
}
