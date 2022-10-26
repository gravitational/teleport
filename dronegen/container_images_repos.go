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
	Name             string
	IsProductionRepo bool
	IsImmutable      bool
	EnvironmentVars  map[string]value
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
		EnvironmentVars: map[string]value{
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
		EnvironmentVars: map[string]value{
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

func NewLocalContainerRepo() *ContainerRepo {
	return &ContainerRepo{
		Name:             "Local Registry",
		IsProductionRepo: false,
		IsImmutable:      false,
		RegistryDomain:   LocalRegistrySocket,
	}
}

func GetLocalContainerRepo() *ContainerRepo {
	return NewLocalContainerRepo()
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

func (cr *ContainerRepo) buildSteps(buildStepDetails []*buildStepOutput, flags *TriggerFlags) []step {
	if len(buildStepDetails) == 0 {
		return nil
	}

	steps := make([]step, 0)

	// Tag and push, collecting the names of the tag/push steps and the images pushed.
	imageTags := cr.BuildImageTags(buildStepDetails[0].Version, flags)
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
		manifestStepName := cr.createAndPushManifestStep(manifestImage, pushStepNames, pushedImages[imageTag])
		steps = append(steps, manifestStepName)
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
		Volumes:     dockerVolumeRefs(),
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

	pullCommands := []string{
		fmt.Sprintf("docker pull %s", buildStepDetails.BuiltImage.GetShellName()),
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
		Volumes:     dockerVolumeRefs(),
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
		Volumes:     dockerVolumeRefs(),
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
