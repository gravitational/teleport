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
	"strconv"
	"time"
)

const (
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

	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "amd64", fips: false, centos7: true, buildConnect: true, buildDeb: true, buildRPM: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "amd64", fips: true, centos7: true, buildConnect: false, buildDeb: true, buildRPM: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "386", buildDeb: true, buildRPM: true}))
	ps = append(ps, ghaLinuxTagPipeline(buildType{os: "linux", arch: "arm", buildDeb: true, buildRPM: true}))

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
			"build-linux-amd64",
			"build-linux-amd64-fips",
			"build-linux-arm64-deb",
			"build-linux-arm",
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

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-hardened-amis",
		dependsOn: []string{
			tagCleanupPipelineName,
			"build-linux-amd64",
			"build-linux-amd64-fips",
		},
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-hardened-amis.yaml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
			},
		},
	}))

	ps = append(ps, ghaBuildPipeline(ghaBuildType{
		buildType:    buildType{os: "linux", fips: false},
		trigger:      triggerTag,
		pipelineName: "build-teleport-kube-agent-updater-oci-images",
		workflows: []ghaWorkflow{
			{
				name:              "release-teleport-kube-agent-updater-oci.yml",
				srcRefVar:         "DRONE_TAG",
				ref:               "${DRONE_TAG}",
				timeout:           150 * time.Minute,
				shouldTagWorkflow: true,
			},
		},
	}))

	// Only amd64 Windows is supported for now.
	ps = append(ps, tagPipeline(buildType{os: "windows", arch: "amd64"}))

	ps = append(ps, darwinTagPipelineGHA())
	ps = append(ps, windowsTagPipeline())

	ps = append(ps, tagCleanupPipeline())
	return ps
}

// ghaLinuxTagPipeline generates a tag pipeline for a given combination of
// os/arch/FIPS that calls a GitHub Actions workflow to perform the build on a
// Linux box. This dispatches to the release-linux.yaml workflow in the
// teleport.e repo, which is a little more generic than the
// release-linux-arm64.yml workflow used for the arm64 build. The two will be
// unified shortly.
func ghaLinuxTagPipeline(b buildType) pipeline {
	if b.os == "" {
		panic("b.os must be set")
	}
	if b.arch == "" {
		panic("b.arch must be set")
	}

	pipelineName := fmt.Sprintf("build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName += "-fips"
	}
	wf := ghaWorkflow{
		name:              "release-linux.yaml",
		timeout:           150 * time.Minute,
		slackOnError:      true,
		srcRefVar:         "DRONE_TAG",
		ref:               "${DRONE_TAG}",
		shouldTagWorkflow: true,
		inputs: map[string]string{
			"release-artifacts": "true",
			"release-target":    releaseMakefileTarget(b),
			"build-connect":     strconv.FormatBool(b.buildConnect),
			"build-deb":         strconv.FormatBool(b.buildDeb),
			"build-rpm":         strconv.FormatBool(b.buildRPM),
		},
	}
	bt := ghaBuildType{
		buildType:    buildType{os: b.os, arch: b.arch},
		trigger:      triggerTag,
		pipelineName: pipelineName,
		workflows:    []ghaWorkflow{wf},
	}
	return ghaBuildPipeline(bt)
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

func tagCleanupPipeline() pipeline {
	return relcliPipeline(triggerTag, tagCleanupPipelineName, "Clean up previously built artifacts", "auto_destroy -f -v 6")
}
