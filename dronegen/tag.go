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
	"strings"
	"time"
)

const (
	// rpmPackage is the RPM package type
	rpmPackage = "rpm"
	// debPackage is the DEB package type
	debPackage = "deb"

	// tagCleanupPipelineName is the name of the pipeline that cleans up
	// artifacts from a previous partially-failed build
	tagCleanupPipelineName = "clean-up-previous-build"
)

const releasesHost = "https://releases-prod.platform.teleport.sh"

// tagCheckoutCommands builds a list of commands for Drone to check out a git commit on a tag build
func tagCheckoutCommands(b buildType) []string {
	return []string{
		`mkdir -p /go/src/github.com/gravitational/teleport`,
		`cd /go/src/github.com/gravitational/teleport`,
		`git clone https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
		`git checkout ${DRONE_TAG:-$DRONE_COMMIT}`,
		// fetch enterprise submodules
		`mkdir -m 0700 /root/.ssh && echo -n "$GITHUB_PRIVATE_KEY" > /root/.ssh/id_rsa && chmod 600 /root/.ssh/id_rsa`,
		`ssh-keyscan -H github.com > /root/.ssh/known_hosts 2>/dev/null && chmod 600 /root/.ssh/known_hosts`,
		`git submodule update --init e`,
		`rm -f /root/.ssh/id_rsa`,
		// create necessary directories
		`mkdir -p /go/cache /go/artifacts`,
		// set version
		`VERSION=$(egrep ^VERSION Makefile | cut -d= -f2)
if [ "$$VERSION" != "${DRONE_TAG##v}" ]; then
  echo "Mismatch between Makefile version: $$VERSION and git tag: $DRONE_TAG"
  exit 1
fi
echo "$$VERSION" > /go/.version.txt`,
	}
}

// tagBuildCommands generates a list of commands for Drone to build an artifact as part of a tag build
func tagBuildCommands(b buildType) []string {
	commands := []string{
		`apk add --no-cache make`,
		`chown -R $UID:$GID /go`,
		`cd /go/src/github.com/gravitational/teleport`,
	}

	if b.fips || b.hasTeleportConnect() {
		commands = append(commands,
			"export VERSION=$(cat /go/.version.txt)",
		)
	}

	// For Windows builds, configure code signing.
	if b.os == "windows" {
		commands = append(commands,
			`echo -n "$WINDOWS_SIGNING_CERT" | base64 -d > windows-signing-cert.pfx`,
		)
	}

	commands = append(commands,
		fmt.Sprintf(
			`make -C build.assets %s`, releaseMakefileTarget(b),
		),
	)

	// Build Teleport Connect on suported OS/arch
	if b.hasTeleportConnect() {
		switch b.os {
		case "linux":
			commands = append(commands, `make -C build.assets teleterm`)
		}
	}

	if b.os == "windows" {
		commands = append(commands,
			`rm -f windows-signing-cert.pfx`,
		)
	}

	return commands
}

// tagCopyArtifactCommands generates a set of commands to find and copy built tarball artifacts as part of a tag build
func tagCopyArtifactCommands(b buildType) []string {
	extension := ".tar.gz"
	if b.os == "windows" {
		extension = ".zip"
	}

	commands := []string{
		`cd /go/src/github.com/gravitational/teleport`,
	}

	// don't copy OSS artifacts for any FIPS build
	if !b.fips {
		commands = append(commands,
			fmt.Sprintf(`find . -maxdepth 1 -iname "teleport*%s" -print -exec cp {} /go/artifacts \;`, extension),
		)
	}

	// copy enterprise artifacts
	if b.os == "windows" {
		commands = append(commands,
			`export VERSION=$(cat /go/.version.txt)`,
			`cp /go/artifacts/teleport-v$${VERSION}-windows-amd64-bin.zip /go/artifacts/teleport-ent-v$${VERSION}-windows-amd64-bin.zip`,
		)
	} else {
		commands = append(commands,
			`find e/ -maxdepth 1 -iname "teleport*.tar.gz" -print -exec cp {} /go/artifacts \;`,
		)
	}

	// we need to specifically rename artifacts which are created for CentOS
	// these is the only special case where renaming is not handled inside the Makefile
	if b.centos7 {
		// for CentOS 7, we support OSS, Enterprise, and FIPS (Enterprise only)
		commands = append(commands, `export VERSION=$(cat /go/.version.txt)`)
		if !b.fips {
			commands = append(commands,
				`mv /go/artifacts/teleport-v$${VERSION}-linux-amd64-bin.tar.gz /go/artifacts/teleport-v$${VERSION}-linux-amd64-centos7-bin.tar.gz`,
				`mv /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-bin.tar.gz /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-centos7-bin.tar.gz`,
			)
		} else {
			commands = append(commands,
				`mv /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-fips-bin.tar.gz /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-centos7-fips-bin.tar.gz`,
			)
		}
	}

	if b.hasTeleportConnect() {
		commands = append(commands,
			`find /go/src/github.com/gravitational/teleport/web/packages/teleterm/build/release -maxdepth 1 \( -iname "teleport-connect*.tar.gz" -o -iname "teleport-connect*.rpm" -o -iname "teleport-connect*.deb" \) -print -exec cp {} /go/artifacts/ \;`,
		)
	}

	// generate checksums
	commands = append(commands, fmt.Sprintf(`cd /go/artifacts && for FILE in teleport*%s; do sha256sum $FILE > $FILE.sha256; done && ls -l`, extension))

	if b.os == "linux" && b.hasTeleportConnect() {
		commands = append(commands,
			`cd /go/artifacts && for FILE in teleport-connect*.deb teleport-connect*.rpm; do
  sha256sum $FILE > $FILE.sha256;
done && ls -l`)
	}
	return commands
}

// tagPipelines builds all applicable tag pipeline combinations
func tagPipelines() []pipeline {
	var ps []pipeline
	// regular tarball builds
	for _, arch := range []string{"amd64", "386", "arm"} {
		for _, fips := range []bool{false, true} {
			if arch != "amd64" && fips {
				// FIPS mode only supported on linux/amd64
				continue
			}
			ps = append(ps, tagPipeline(buildType{os: "linux", arch: arch, fips: fips}))

			// RPM/DEB package builds
			for _, packageType := range []string{rpmPackage, debPackage} {
				bt := buildType{os: "linux", arch: arch, fips: fips}
				if packageType == "rpm" && arch == "amd64" {
					bt.centos7 = true
				}
				ps = append(ps, tagPackagePipeline(packageType, bt))
			}
		}
	}

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", arch: "arm64", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-linux-arm64",
		dependsOn:    []string{tagCleanupPipelineName},
		workflows: []ghaWorkflow{
			{
				name:              "release-linux-arm64.yml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
				inputs:            map[string]string{"upload-artifacts": "true"},
			},
		},
	}))

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-oci-distroless-images",
		dependsOn: []string{
			tagCleanupPipelineName,
			"build-linux-amd64-deb",
			"build-linux-arm64-deb",
		},
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-oci-distroless.yml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
			},
		},
	}))

	// Only amd64 Windows is supported for now.
	ps = append(ps, tagPipeline(buildType{os: "windows", arch: "amd64"}))

	// Also add CentOS artifacts
	// CentOS 6 FIPS builds have been removed in Teleport 7.0. See https://github.com/gravitational/teleport/issues/7207
	ps = append(ps, tagPipeline(buildType{os: "linux", arch: "amd64", centos7: true}))
	ps = append(ps, tagPipeline(buildType{os: "linux", arch: "amd64", centos7: true, fips: true}))

	ps = append(ps, darwinTagPipelineGHA())
	ps = append(ps, windowsTagPipeline())

	ps = append(ps, tagCleanupPipeline())
	return ps
}

// tagPipeline generates a tag pipeline for a given combination of os/arch/FIPS
func tagPipeline(b buildType) pipeline {
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	pipelineName := fmt.Sprintf("build-%s-%s", b.os, b.arch)
	if b.centos7 {
		pipelineName += "-centos7"
	}
	tagEnvironment := map[string]value{
		"UID":     {raw: "1000"},
		"GID":     {raw: "1000"},
		"GOCACHE": {raw: "/go/cache"},
		"GOPATH":  {raw: "/go"},
		"OS":      {raw: b.os},
		"ARCH":    {raw: b.arch},
	}
	if b.fips {
		pipelineName += "-fips"
		tagEnvironment["FIPS"] = value{raw: "yes"}
	}

	if b.os == "windows" {
		tagEnvironment["WINDOWS_SIGNING_CERT"] = value{fromSecret: "WINDOWS_SIGNING_CERT"}
	}

	var extraQualifications []string
	if b.os == "windows" {
		extraQualifications = []string{"tsh client only"}
	}

	p := newKubePipeline(pipelineName)
	p.Environment = map[string]value{
		"BUILDBOX_VERSION": buildboxVersion,
		"RUNTIME":          goRuntime,
	}
	p.Trigger = triggerTag
	p.DependsOn = []string{tagCleanupPipelineName}
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = []volume{volumeAwsConfig, volumeDocker, volumeDockerConfig}
	p.Services = []service{
		dockerService(),
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Pull:  "if-not-exists",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: tagCheckoutCommands(b),
		},
		waitForDockerStep(),
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Pull:        "if-not-exists",
			Environment: tagEnvironment,
			Volumes:     []volumeRef{volumeRefDocker, volumeRefDockerConfig},
			Commands:    tagBuildCommands(b),
		},
		{
			Name:     "Copy artifacts",
			Image:    "docker",
			Pull:     "if-not-exists",
			Commands: tagCopyArtifactCommands(b),
		},
		kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
			awsRoleSettings: awsRoleSettings{
				awsAccessKeyID:     value{fromSecret: "AWS_ACCESS_KEY_ID"},
				awsSecretAccessKey: value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
				role:               value{fromSecret: "AWS_ROLE"},
			},
			configVolume: volumeRefAwsConfig,
		}),
		kubernetesUploadToS3Step(kubernetesS3Settings{
			region:       "us-west-2",
			source:       "/go/artifacts/",
			target:       "teleport/tag/${DRONE_TAG##v}",
			configVolume: volumeRefAwsConfig,
		}),
		{
			Name:     "Register artifacts",
			Image:    "docker",
			Pull:     "if-not-exists",
			Commands: tagCreateReleaseAssetCommands(b, "", extraQualifications),
			Environment: map[string]value{
				"RELEASES_CERT": {fromSecret: "RELEASES_CERT"},
				"RELEASES_KEY":  {fromSecret: "RELEASES_KEY"},
			},
		},
	}
	return p
}

// tagDownloadArtifactCommands generates a set of commands to download appropriate artifacts for creating a package as part of a tag build
func tagDownloadArtifactCommands(b buildType) []string {
	commands := []string{
		`export VERSION=$(cat /go/.version.txt)`,
		`if [[ "${DRONE_TAG}" != "" ]]; then export S3_PATH="tag/$${DRONE_TAG##v}/"; else export S3_PATH="tag/"; fi`,
	}
	artifactOSS := true
	artifactType := fmt.Sprintf("%s-%s", b.os, b.arch)

	if b.centos7 {
		artifactType += "-centos7"
	}

	if b.fips {
		artifactType += "-fips"
		artifactOSS = false
	}

	if artifactOSS {
		commands = append(commands,
			fmt.Sprintf(`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}teleport-v$${VERSION}-%s-bin.tar.gz /go/artifacts/`, artifactType),
		)
	}
	commands = append(commands,
		fmt.Sprintf(`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}teleport-ent-v$${VERSION}-%s-bin.tar.gz /go/artifacts/`, artifactType),
	)
	return commands
}

// tagCopyPackageArtifactCommands generates a set of commands to find and copy built package artifacts as part of a tag build
func tagCopyPackageArtifactCommands(b buildType, packageType string) []string {
	commands := []string{
		`cd /go/src/github.com/gravitational/teleport`,
	}
	if !b.fips {
		commands = append(commands, fmt.Sprintf(`find build -maxdepth 1 -iname "teleport*.%s*" -print -exec cp {} /go/artifacts \;`, packageType))
	}
	commands = append(commands, fmt.Sprintf(`find e/build -maxdepth 1 -iname "teleport*.%s*" -print -exec cp {} /go/artifacts \;`, packageType))
	return commands
}

// createReleaseAssetCommands generates a set of commands to create release & asset in release management service
func tagCreateReleaseAssetCommands(b buildType, packageType string, extraQualifications []string) []string {
	commands := []string{
		`WORKSPACE_DIR=$${WORKSPACE_DIR:-/}`,
		`VERSION=$(cat "$WORKSPACE_DIR/go/.version.txt")`,
		fmt.Sprintf(`RELEASES_HOST='%v'`, releasesHost),
		`echo "$RELEASES_CERT" | base64 -d > "$WORKSPACE_DIR/releases.crt"`,
		`echo "$RELEASES_KEY" | base64 -d > "$WORKSPACE_DIR/releases.key"`,
		`trap "rm -f '$WORKSPACE_DIR/releases.crt' '$WORKSPACE_DIR/releases.key'" EXIT`,
		`CREDENTIALS="--cert $WORKSPACE_DIR/releases.crt --key $WORKSPACE_DIR/releases.key"`,
		`which curl || apk add --no-cache curl`,
		fmt.Sprintf(`cd "$WORKSPACE_DIR/go/artifacts"
find . -type f ! -iname '*.sha256' ! -iname '*-unsigned.zip*' | while read -r file; do
  # Skip files that are not results of this build
  # (e.g. tarballs from which OS packages are made)
  [ -f "$file.sha256" ] || continue

  name="$(basename "$file" | sed -E 's/(-|_)v?[0-9].*$//')" # extract part before -vX.Y.Z
  description="%[1]s"
  products="$name"
  if [ "$name" = "tsh" ]; then
    products="teleport teleport-ent"
  elif [ "$name" = "Teleport Connect" -o "$name" = "teleport-connect" ]; then
    description="Teleport Connect"
    products="teleport teleport-ent"
  fi
  shasum="$(cat "$file.sha256" | cut -d ' ' -f 1)"

  release_params="" # List of "-F releaseId=XXX" parameters to curl

  for product in $products; do
    status_code=$(curl $CREDENTIALS -o "$WORKSPACE_DIR/curl_out.txt" -w "%%{http_code}" -F "product=$product" -F "version=$VERSION" -F notesMd="# Teleport $VERSION" -F status=draft "$RELEASES_HOST/releases")
    if [ $status_code -ne 200 ] && [ $status_code -ne 409 ]; then
      echo "curl HTTP status: $status_code"
      cat $WORKSPACE_DIR/curl_out.txt
      exit 1
    fi

    release_params="$release_params -F releaseId=$product@$VERSION"
  done

  curl $CREDENTIALS --fail -o /dev/null -F description="$description" -F os="%[2]s" -F arch="%[3]s" -F "file=@$file" -F "sha256=$shasum" $release_params "$RELEASES_HOST/assets";
done`,
			b.Description(packageType, extraQualifications...), b.os, b.arch),
	}
	return commands
}

// tagPackagePipeline generates a tag package pipeline for a given combination of os/arch/FIPS
func tagPackagePipeline(packageType string, b buildType) pipeline {
	if packageType == "" {
		panic("packageType must be set")
	}
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	environment := map[string]value{
		"ARCH":             {raw: b.arch},
		"TMPDIR":           {raw: "/go"},
		"ENT_TARBALL_PATH": {raw: "/go/artifacts"},
	}

	dependentPipeline := fmt.Sprintf("build-%s-%s", b.os, b.arch)

	if b.centos7 {
		dependentPipeline += "-centos7"
	}

	apkPackages := []string{"bash", "curl", "gzip", "make", "tar"}
	if packageType == rpmPackage {
		// Required by `make rpm`
		apkPackages = append(apkPackages, "go")
	}

	packageBuildCommands := []string{
		fmt.Sprintf("apk add --no-cache %s", strings.Join(apkPackages, " ")),
		`apk add --no-cache aws-cli`,
		`cd /go/src/github.com/gravitational/teleport`,
		`export VERSION=$(cat /go/.version.txt)`,
		// Login to Amazon ECR Public
		`aws ecr-public get-login-password --region us-east-1 | docker login -u="AWS" --password-stdin public.ecr.aws`,
	}

	makeCommand := fmt.Sprintf("make %s", packageType)
	if b.fips {
		dependentPipeline += "-fips"
		environment["FIPS"] = value{raw: "yes"}
		environment["RUNTIME"] = value{raw: "fips"}
		makeCommand = fmt.Sprintf("make -C e %s", packageType)
	} else {
		environment["OSS_TARBALL_PATH"] = value{raw: "/go/artifacts"}
	}

	packageDockerVolumes := []volume{volumeAwsConfig, volumeDocker, volumeDockerConfig}
	packageDockerVolumeRefs := []volumeRef{
		volumeRefDocker,
		volumeRefDockerConfig,
		volumeRefAwsConfig,
	}
	packageDockerService := dockerService()

	switch packageType {
	case rpmPackage:
		environment["GNUPG_DIR"] = value{raw: "/tmpfs/gnupg"}
		environment["GPG_RPM_SIGNING_ARCHIVE"] = value{fromSecret: "GPG_RPM_SIGNING_ARCHIVE"}
		packageBuildCommands = append(packageBuildCommands,
			`mkdir -m0700 $GNUPG_DIR`,
			`echo "$GPG_RPM_SIGNING_ARCHIVE" | base64 -d | tar -xzf - -C $GNUPG_DIR`,
			`chown -R root:root $GNUPG_DIR`,
			makeCommand,
			`rm -rf $GNUPG_DIR`,
		)
		// RPM builds require tmpfs to hold the key material in memory.
		packageDockerVolumes = append(packageDockerVolumes, volumeTmpfs)
		packageDockerVolumeRefs = append(packageDockerVolumeRefs, volumeRefTmpfs)
		packageDockerService = dockerService(volumeRefTmpfs)
	case debPackage:
		packageBuildCommands = append(packageBuildCommands,
			makeCommand,
		)
	default:
		panic("packageType is not set")
	}

	assumeDownloadRoleStep := kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
		awsRoleSettings: awsRoleSettings{
			awsAccessKeyID:     value{fromSecret: "AWS_ACCESS_KEY_ID"},
			awsSecretAccessKey: value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
			role:               value{fromSecret: "AWS_ROLE"},
		},
		configVolume: volumeRefAwsConfig,
		name:         "Assume Download AWS Role",
	})
	assumeBuildRoleStep := kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
		awsRoleSettings: awsRoleSettings{
			awsAccessKeyID:     value{fromSecret: "TELEPORT_BUILD_USER_READ_ONLY_KEY"},
			awsSecretAccessKey: value{fromSecret: "TELEPORT_BUILD_USER_READ_ONLY_SECRET"},
			role:               value{fromSecret: "TELEPORT_BUILD_READ_ONLY_AWS_ROLE"},
		},
		configVolume: volumeRefAwsConfig,
		name:         "Assume Build AWS Role",
	})
	assumeUploadRoleStep := kubernetesAssumeAwsRoleStep(kubernetesRoleSettings{
		awsRoleSettings: awsRoleSettings{
			awsAccessKeyID:     value{fromSecret: "AWS_ACCESS_KEY_ID"},
			awsSecretAccessKey: value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
			role:               value{fromSecret: "AWS_ROLE"},
		},
		configVolume: volumeRefAwsConfig,
		name:         "Assume Upload AWS Role",
	})

	pipelineName := fmt.Sprintf("%s-%s", dependentPipeline, packageType)

	p := newKubePipeline(pipelineName)
	p.Trigger = triggerTag
	p.DependsOn = []string{dependentPipeline, tagCleanupPipelineName}
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = packageDockerVolumes
	p.Services = []service{
		packageDockerService,
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: tagCheckoutCommands(b),
		},
		waitForDockerStep(),
		assumeDownloadRoleStep,
		{
			Name:  "Download artifacts from S3",
			Image: "amazon/aws-cli",
			Environment: map[string]value{
				"AWS_REGION":    {raw: "us-west-2"},
				"AWS_S3_BUCKET": {fromSecret: "AWS_S3_BUCKET"},
			},
			Commands: tagDownloadArtifactCommands(b),
			Volumes:  []volumeRef{volumeRefAwsConfig},
		},
		assumeBuildRoleStep,
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Environment: environment,
			Volumes:     packageDockerVolumeRefs,
			Commands:    packageBuildCommands,
		},
		{
			Name:     "Copy artifacts",
			Image:    "docker",
			Commands: tagCopyPackageArtifactCommands(b, packageType),
		},
		assumeUploadRoleStep,
		kubernetesUploadToS3Step(kubernetesS3Settings{
			region:       "us-west-2",
			source:       "/go/artifacts/",
			target:       "teleport/tag/${DRONE_TAG##v}",
			configVolume: volumeRefAwsConfig,
		}),
		{
			Name:     "Register artifacts",
			Image:    "docker",
			Commands: tagCreateReleaseAssetCommands(b, strings.ToUpper(packageType), nil),
			Environment: map[string]value{
				"RELEASES_CERT": {fromSecret: "RELEASES_CERT"},
				"RELEASES_KEY":  {fromSecret: "RELEASES_KEY"},
			},
		},
	}
	return p
}

func tagCleanupPipeline() pipeline {
	return relcliPipeline(triggerTag, tagCleanupPipelineName, "Clean up previously built artifacts", "auto_destroy -f -v 6")
}
