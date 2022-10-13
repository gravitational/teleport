package main

import (
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
)

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
