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
)

const (
	// rpmPackage is the RPM package type
	rpmPackage = "rpm"
	// debPackage is the DEB package type
	debPackage = "deb"
)

const releasesHost = "https://releases-staging.platform.teleport.sh"

// tagCheckoutCommands builds a list of commands for Drone to check out a git commit on a tag build
func tagCheckoutCommands(fips bool) []string {
	commands := []string{
		`mkdir -p /go/src/github.com/gravitational/teleport`,
		`cd /go/src/github.com/gravitational/teleport`,
		`git clone https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
		`git checkout ${DRONE_TAG:-$DRONE_COMMIT}`,
		// fetch enterprise submodules
		`mkdir -m 0700 /root/.ssh && echo -n "$GITHUB_PRIVATE_KEY" > /root/.ssh/id_rsa && chmod 600 /root/.ssh/id_rsa`,
		`ssh-keyscan -H github.com > /root/.ssh/known_hosts 2>/dev/null && chmod 600 /root/.ssh/known_hosts`,
		`git submodule update --init e`,
		// this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
		`git submodule update --init --recursive webassets || true`,
		`rm -f /root/.ssh/id_rsa`,
		// create necessary directories
		`mkdir -p /go/cache /go/artifacts`,
		// set version
		`if [[ "${DRONE_TAG}" != "" ]]; then echo "${DRONE_TAG##v}" > /go/.version.txt; else egrep ^VERSION Makefile | cut -d= -f2 > /go/.version.txt; fi; cat /go/.version.txt`,
	}
	return commands
}

// tagBuildCommands generates a list of commands for Drone to build an artifact as part of a tag build
func tagBuildCommands(b buildType) []string {
	commands := []string{
		`apk add --no-cache make`,
		`chown -R $UID:$GID /go`,
		`cd /go/src/github.com/gravitational/teleport`,
	}

	if b.fips {
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
	if b.centos6 {
		// for CentOS 6 we don't support FIPS, just OSS and enterprise
		commands = append(commands,
			`export VERSION=$(cat /go/.version.txt)`,
			`mv /go/artifacts/teleport-v$${VERSION}-linux-amd64-bin.tar.gz /go/artifacts/teleport-v$${VERSION}-linux-amd64-centos6-bin.tar.gz`,
			`mv /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-bin.tar.gz /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-centos6-bin.tar.gz`,
		)
	} else if b.centos7 {
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

	// generate checksums
	commands = append(commands, fmt.Sprintf(`cd /go/artifacts && for FILE in teleport*%s; do sha256sum $FILE > $FILE.sha256; done && ls -l`, extension))
	return commands
}

type s3Settings struct {
	region      string
	source      string
	target      string
	stripPrefix string
}

// uploadToS3Step generates an S3 upload step
func uploadToS3Step(s s3Settings) step {
	return step{
		Name:  "Upload to S3",
		Image: "plugins/s3",
		Settings: map[string]value{
			"bucket":       {fromSecret: "AWS_S3_BUCKET"},
			"access_key":   {fromSecret: "AWS_ACCESS_KEY_ID"},
			"secret_key":   {fromSecret: "AWS_SECRET_ACCESS_KEY"},
			"region":       {raw: s.region},
			"source":       {raw: s.source},
			"target":       {raw: s.target},
			"strip_prefix": {raw: s.stripPrefix},
		},
	}
}

// tagPipelines builds all applicable tag pipeline combinations
func tagPipelines() []pipeline {
	var ps []pipeline
	// regular tarball builds
	for _, arch := range []string{"amd64", "386", "arm", "arm64"} {
		for _, fips := range []bool{false, true} {
			if arch != "amd64" && fips {
				// FIPS mode only supported on linux/amd64
				continue
			}
			ps = append(ps, tagPipeline(buildType{os: "linux", arch: arch, fips: fips}))

			// RPM/DEB package builds
			for _, packageType := range []string{rpmPackage, debPackage} {
				ps = append(ps, tagPackagePipeline(packageType, buildType{os: "linux", arch: arch, fips: fips}))
			}
		}
	}

	// Only amd64 Windows is supported for now.
	ps = append(ps, tagPipeline(buildType{os: "windows", arch: "amd64"}))

	// Also add CentOS artifacts
	// CentOS 6 FIPS builds have been removed in Teleport 7.0. See https://github.com/gravitational/teleport/issues/7207
	ps = append(ps, tagPipeline(buildType{os: "linux", arch: "amd64", centos6: true}))
	ps = append(ps, tagPipeline(buildType{os: "linux", arch: "amd64", centos7: true}))
	ps = append(ps, tagPipeline(buildType{os: "linux", arch: "amd64", centos7: true, fips: true}))

	ps = append(ps, darwinTagPipeline(), darwinTeleportPkgPipeline(), darwinTshPkgPipeline())
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
	if b.centos6 {
		pipelineName += "-centos6"
	} else if b.centos7 {
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

	p := newKubePipeline(pipelineName)
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
	}
	p.Trigger = triggerTag
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = dockerVolumes()
	p.Services = []service{
		dockerService(),
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: tagCheckoutCommands(b.fips),
		},
		waitForDockerStep(),
		{
			Name:        "Build artifacts",
			Image:       "docker",
			Environment: tagEnvironment,
			Volumes:     dockerVolumeRefs(),
			Commands:    tagBuildCommands(b),
		},
		{
			Name:     "Copy artifacts",
			Image:    "docker",
			Commands: tagCopyArtifactCommands(b),
		},
		uploadToS3Step(s3Settings{
			region:      "us-west-2",
			source:      "/go/artifacts/*",
			target:      "teleport/tag/${DRONE_TAG##v}",
			stripPrefix: "/go/artifacts/",
		}),
		{
			Name:     "Register artifacts",
			Image:    "docker",
			Failure:  "ignore",
			Commands: tagCreateReleaseAssetCommands(b),
			Environment: map[string]value{
				"RELEASES_CERT": value{fromSecret: "RELEASES_CERT_STAGING"},
				"RELEASES_KEY":  value{fromSecret: "RELEASES_KEY_STAGING"},
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
func tagCreateReleaseAssetCommands(b buildType) []string {
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
for file in $(find . -type f ! -iname '*.sha256'); do
  # Skip files that are not results of this build
  # (e.g. tarballs from which OS packages are made)
  [ -f "$file.sha256" ] || continue

  product="$(basename "$file" | sed -E 's/(-|_)v?[0-9].*$//')" # extract part before -vX.Y.Z
  shasum="$(cat "$file.sha256" | cut -d ' ' -f 1)"
  status_code=$(curl $CREDENTIALS -o "$WORKSPACE_DIR/curl_out.txt" -w "%%{http_code}" -F "product=$product" -F "version=$VERSION" -F notesMd="# Teleport $VERSION" -F status=draft "$RELEASES_HOST/releases")
  if [ $status_code -ne 200 ] && [ $status_code -ne 409 ]; then
    echo "curl HTTP status: $status_code"
    cat $WORKSPACE_DIR/curl_out.txt
    exit 1
  fi
  curl $CREDENTIALS --fail -o /dev/null -F description="TODO" -F os="%s" -F arch="%s" -F "file=@$file" -F "sha256=$shasum" -F "releaseId=$product@$VERSION" "$RELEASES_HOST/assets";
done`,
			b.os, b.arch /* TODO: fips */),
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
	packageBuildCommands := []string{
		`apk add --no-cache bash curl gzip make tar`,
		`cd /go/src/github.com/gravitational/teleport`,
		`export VERSION=$(cat /go/.version.txt)`,
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

	packageDockerVolumes := dockerVolumes()
	packageDockerVolumeRefs := dockerVolumeRefs()
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
		packageDockerVolumes = dockerVolumes(volumeTmpfs)
		packageDockerVolumeRefs = dockerVolumeRefs(volumeRefTmpfs)
		packageDockerService = dockerService(volumeRefTmpfs)
	case debPackage:
		packageBuildCommands = append(packageBuildCommands,
			makeCommand,
		)
	default:
		panic("packageType is not set")
	}

	pipelineName := fmt.Sprintf("%s-%s", dependentPipeline, packageType)

	p := newKubePipeline(pipelineName)
	p.Trigger = triggerTag
	p.DependsOn = []string{dependentPipeline}
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
			Commands: tagCheckoutCommands(b.fips),
		},
		waitForDockerStep(),
		{
			Name:  "Download artifacts from S3",
			Image: "amazon/aws-cli",
			Environment: map[string]value{
				"AWS_REGION":            {raw: "us-west-2"},
				"AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
			},
			Commands: tagDownloadArtifactCommands(b),
		},
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
		uploadToS3Step(s3Settings{
			region:      "us-west-2",
			source:      "/go/artifacts/*",
			target:      "teleport/tag/${DRONE_TAG##v}",
			stripPrefix: "/go/artifacts/",
		}),
		{
			Name:     "Register artifacts",
			Image:    "docker",
			Commands: tagCreateReleaseAssetCommands(b),
			Failure:  "ignore",
			Environment: map[string]value{
				"RELEASES_CERT": value{fromSecret: "RELEASES_CERT_STAGING"},
				"RELEASES_KEY":  value{fromSecret: "RELEASES_KEY_STAGING"},
			},
		},
	}
	return p
}
