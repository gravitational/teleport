/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"path"
)

// Describes a Drone trigger as it pertains to container image building.
type TriggerInfo struct {
	Trigger              trigger
	Name                 string
	Flags                *TriggerFlags
	SupportedVersions    []*ReleaseVersion
	SetupSteps           []step
	ParentePipelineNames []string
}

// This type is mainly used to make passing these vars around cleaner
type TriggerFlags struct {
	ShouldAffectProductionImages bool
	ShouldBuildNewImages         bool
	UseUniqueStagingTag          bool
	ShouldOnlyPublishFullSemver  bool
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
			ShouldOnlyPublishFullSemver:  true,
		},
		SupportedVersions: []*ReleaseVersion{
			{
				MajorVersion: branchMajorVersion,
				ShellVersion: "$DRONE_TAG",
				// Omitted because it doesn't matter here - only the full semver will only be published (see Flags)
				// ShellIsPrerelease:   // ,
				RelativeVersionName: "branch",
			},
		},
		ParentePipelineNames: []string{
			tagCleanupPipelineName,
		},
	}
}

func NewPromoteTrigger(branchMajorVersion string) *TriggerInfo {
	prereleaseFilePath := "/go/vars/release-is-prerelease"
	shellVersion := "$DRONE_TAG"

	promoteTrigger := triggerPromote
	promoteTrigger.Target.Include = append(promoteTrigger.Target.Include, "promote-docker")

	return &TriggerInfo{
		Trigger: promoteTrigger,
		Name:    "promote",
		Flags: &TriggerFlags{
			ShouldAffectProductionImages: true,
			ShouldBuildNewImages:         false,
			UseUniqueStagingTag:          false,
			ShouldOnlyPublishFullSemver:  false,
		},
		SupportedVersions: []*ReleaseVersion{
			{
				MajorVersion: branchMajorVersion,
				ShellVersion: shellVersion,
				// Truthy if the file exists, which indicates a prerelease. See `recordPrereleaseStatus`` for details.
				ShellIsPrerelease:   fmt.Sprintf("[ -f %s ]", prereleaseFilePath),
				RelativeVersionName: "branch",
			},
		},
		SetupSteps: []step{verifyTaggedStep(), recordPrereleaseStatus(shellVersion, prereleaseFilePath)},
	}
}

func NewCronTrigger(latestMajorVersions []string) *TriggerInfo {
	if len(latestMajorVersions) == 0 {
		return nil
	}

	majorVersionVarBasePath := "/go/vars/full-version"

	supportedVersions := make([]*ReleaseVersion, 0, len(latestMajorVersions))
	if len(latestMajorVersions) > 0 {
		latestMajorVersion := latestMajorVersions[0]
		supportedVersions = append(supportedVersions, &ReleaseVersion{
			MajorVersion:        latestMajorVersion,
			ShellVersion:        readCronShellVersionCommand(majorVersionVarBasePath, latestMajorVersion),
			RelativeVersionName: "current-version",
			SetupSteps:          []step{getLatestSemverStep(latestMajorVersion, majorVersionVarBasePath)},
		})

		if len(latestMajorVersions) > 1 {
			for i, majorVersion := range latestMajorVersions[1:] {
				supportedVersions = append(supportedVersions, &ReleaseVersion{
					MajorVersion: majorVersion,
					ShellVersion: readCronShellVersionCommand(majorVersionVarBasePath, majorVersion),
					// Omitted because it doesn't matter here - latest tags should always be built
					// ShellIsPrerelease:   // ,
					RelativeVersionName: fmt.Sprintf("previous-version-%d", i+1),
					SetupSteps:          []step{getLatestSemverStep(majorVersion, majorVersionVarBasePath)},
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
			ShouldOnlyPublishFullSemver:  false,
		},
		SupportedVersions: supportedVersions,
	}
}

func getLatestSemverStep(majorVersion string, majorVersionVarBasePath string) step {
	// We don't use "/go/src/github.com/gravitational/teleport" here as a later stage
	// may need to clone a different version, and "/go" persists between steps
	cloneDirectory := "/tmp/teleport"
	majorVersionVarPath := fmt.Sprintf("%s-%s", majorVersionVarBasePath, majorVersion)
	return step{
		Name:  fmt.Sprintf("Find the latest available semver for %s", majorVersion),
		Image: fmt.Sprintf("golang:%s", GoVersion),
		Commands: append(
			cloneRepoCommands(cloneDirectory, fmt.Sprintf("branch/%s", majorVersion)),
			fmt.Sprintf("mkdir -pv $(dirname %q)", majorVersionVarPath),
			fmt.Sprintf("cd %q", path.Join(cloneDirectory, "build.assets", "tooling", "cmd", "query-latest")),
			fmt.Sprintf("go run . %q | sed 's/v//' > %q", majorVersion, majorVersionVarPath),
			fmt.Sprintf("echo Found full semver \"$(cat %q)\" for major version %q", majorVersionVarPath, majorVersion),
		),
	}
}

func readCronShellVersionCommand(majorVersionDirectory, majorVersion string) string {
	return fmt.Sprintf("v$(cat '%s-%s')", majorVersionDirectory, majorVersion)
}

func recordPrereleaseStatus(shellVersion, recordFilePath string) step {
	clonePath := "/tmp/repo"
	commands := []string{
		"apk add git",
	}
	commands = append(commands, cloneRepoCommands(clonePath, "${DRONE_TAG}")...)
	commands = append(commands,
		fmt.Sprintf("mkdir -pv $(dirname %q)", recordFilePath),
		fmt.Sprintf("cd %q", path.Join(clonePath, "build.assets", "tooling")),
		// If the tag is a prerelease, create a file who's existence shows that it is one
		fmt.Sprintf("CGO_ENABLED=0 go run ./cmd/check -tag %s -check prerelease &> /dev/null || echo 'Version is a prerelease' > %q", shellVersion, recordFilePath),
		fmt.Sprintf("printf 'Version is '; [ ! -f \"%s\" ] && printf 'not '; echo 'a prerelease'", recordFilePath),
	)

	return step{
		// Note that Drone will evaluate certain variables (such as '${DRONE_TAG}') to their actual value in the step name
		Name:     fmt.Sprintf("Record if tag (%s) is prerelease", shellVersion),
		Image:    fmt.Sprintf("golang:%s-alpine", GoVersion),
		Commands: commands,
	}
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
		pipeline.DependsOn = append(pipeline.DependsOn, ti.ParentePipelineNames...)

		pipelines = append(pipelines, pipeline)
	}

	return pipelines
}
