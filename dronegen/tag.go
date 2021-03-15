package main

import (
	"fmt"
	"strings"
)

const (
	// rpmPackage is the RPM package type
	rpmPackage = "rpm"
	// debPackage is the DEB package type
	debPackage = "deb"
)

type tagBuildType struct {
	os      string
	arch    string
	fips    bool
	centos6 bool
}

// tagMakefileTarget gets the correct tag pipeline Makefile target for a given arch/fips combo
func tagMakefileTarget(params tagBuildType) string {
	makefileTarget := fmt.Sprintf("release-%s", params.arch)
	if params.centos6 {
		makefileTarget += "-centos6"
	}
	if params.fips {
		makefileTarget += "-fips"
	}
	return makefileTarget
}

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
func tagBuildCommands(params tagBuildType) []string {
	commands := []string{
		`apk add --no-cache make`,
		`chown -R $UID:$GID /go`,
		`cd /go/src/github.com/gravitational/teleport`,
	}

	if params.fips {
		commands = append(commands,
			"export VERSION=$(cat /go/.version.txt)",
		)
	}

	commands = append(commands,
		fmt.Sprintf(
			`make -C build.assets %s`, tagMakefileTarget(params),
		),
	)

	return commands
}

// tagCopyArtifactCommands generates a set of commands to find and copy built tarball artifacts as part of a tag build
func tagCopyArtifactCommands(params tagBuildType) []string {
	extension := ".tar.gz"
	if params.os == "windows" {
		extension = ".zip"
	}

	commands := []string{
		`cd /go/src/github.com/gravitational/teleport`,
	}

	// don't copy OSS artifacts for any FIPS build
	if !params.fips {
		commands = append(commands,
			fmt.Sprintf(`find . -maxdepth 1 -iname "teleport*%s" -print -exec cp {} /go/artifacts \;`, extension),
		)
	}

	// copy enterprise artifacts
	if params.os == "windows" {
		commands = append(commands,
			`export VERSION=$(cat /go/.version.txt)`,
			`cp /go/artifacts/teleport-v$${VERSION}-windows-amd64-bin.zip /go/artifacts/teleport-ent-v$${VERSION}-windows-amd64-bin.zip`,
		)
	} else {
		commands = append(commands,
			`find e/ -maxdepth 1 -iname "teleport*.tar.gz" -print -exec cp {} /go/artifacts \;`,
		)
	}

	// we need to specifically rename artifacts which are created for CentOS 6
	// these is the only special case where renaming is not handled inside the Makefile
	if params.centos6 {
		commands = append(commands, `export VERSION=$(cat /go/.version.txt)`)
		if !params.fips {
			commands = append(commands,
				`mv /go/artifacts/teleport-v$${VERSION}-linux-amd64-bin.tar.gz /go/artifacts/teleport-v$${VERSION}-linux-amd64-centos6-bin.tar.gz`,
				`mv /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-bin.tar.gz /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-centos6-bin.tar.gz`,
			)
		} else {
			commands = append(commands,
				`mv /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-fips-bin.tar.gz /go/artifacts/teleport-ent-v$${VERSION}-linux-amd64-centos6-fips-bin.tar.gz`,
			)
		}
	}

	// generate checksums
	commands = append(commands, fmt.Sprintf(`cd /go/artifacts && for FILE in teleport*%s; do sha256sum $FILE > $FILE.sha256; done && ls -l`, extension))
	return commands
}

// tagPipelines builds all applicable tag pipeline combinations
func tagPipelines() []pipeline {
	var ps []pipeline
	// regular tarball builds
	for _, arch := range []string{"amd64", "386", "arm", "arm64"} {
		for _, fips := range []bool{false, true} {
			if (arch == "386" || arch == "arm") && fips {
				// FIPS mode not supported on i386 or ARM
				continue
			}
			ps = append(ps, tagPipeline(tagBuildType{os: "linux", arch: arch, fips: fips}))
			// TODO(gus): support needs to be added upstream for building ARM/ARM64 packages first
			// ps = append(ps, tagPackagePipeline(rpmPackage, tagBuildType{os: "linux", arch: arch, fips: fips}))
			// ps = append(ps, tagPackagePipeline(debPackage, tagBuildType{os: "linux", arch: arch, fips: fips}))
		}
	}

	// TODO(gus): needed until support is added upstream for building ARM/ARM64 packages
	// Remove this section and uncomment above once this is added.
	for _, arch := range []string{"amd64", "386"} {
		for _, fips := range []bool{false, true} {
			if (arch == "386" || arch == "arm") && fips {
				// FIPS mode not supported on i386 or ARM
				continue
			}
			ps = append(ps, tagPackagePipeline(rpmPackage, tagBuildType{os: "linux", arch: arch, fips: fips}))
			ps = append(ps, tagPackagePipeline(debPackage, tagBuildType{os: "linux", arch: arch, fips: fips}))
		}
	}

	// Only amd64 Windows is supported for now.
	ps = append(ps, tagPipeline(tagBuildType{os: "windows", arch: "amd64"}))
	// Also add the two CentOS 6 artifacts.
	ps = append(ps, tagPipeline(tagBuildType{os: "linux", arch: "amd64", centos6: true}))
	ps = append(ps, tagPipeline(tagBuildType{os: "linux", arch: "amd64", centos6: true, fips: true}))
	return ps
}

// tagPipeline generates a tag pipeline for a given combination of os/arch/FIPS
func tagPipeline(params tagBuildType) pipeline {
	if params.os == "" {
		panic("params.os must be set")
	}
	if params.arch == "" {
		panic("params.arch must be set")
	}

	pipelineName := fmt.Sprintf("build-%s-%s", params.os, params.arch)
	pipelineTitleSegment := "release artifacts"
	if params.centos6 {
		pipelineName += "-centos6"
		pipelineTitleSegment = "CentOS 6 release artifacts"
	}
	tagEnvironment := map[string]value{
		"UID":    value{raw: "1000"},
		"GID":    value{raw: "1000"},
		"GOPATH": value{raw: "/go"},
		"OS":     value{raw: params.os},
		"ARCH":   value{raw: params.arch},
	}
	if params.fips {
		pipelineName += "-fips"
		tagEnvironment["FIPS"] = value{raw: "yes"}
		if params.centos6 {
			pipelineTitleSegment = "CentOS 6 FIPS release artifacts"
		} else {
			pipelineTitleSegment = "FIPS release artifacts"
		}
	}

	p := newKubePipeline(pipelineName)
	p.Environment = map[string]value{
		"RUNTIME": value{raw: "go1.15.5"},
	}
	p.Trigger = triggerTag
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = []volume{
		volumeDocker,
	}
	p.Services = []service{
		{
			Name:  "Start Docker",
			Image: "docker:dind",
			Volumes: []volumeRef{
				volumeRefDocker,
			},
		},
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": value{fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: tagCheckoutCommands(params.fips),
		},
		{
			Name:        fmt.Sprintf("Build %s", pipelineTitleSegment),
			Image:       "docker",
			Environment: tagEnvironment,
			Volumes: []volumeRef{
				volumeRefDocker,
			},
			Commands: tagBuildCommands(params),
		},
		{
			Name:     fmt.Sprintf("Copy %s", pipelineTitleSegment),
			Image:    "docker",
			Commands: tagCopyArtifactCommands(params),
		},
		{
			Name:  "Upload to S3",
			Image: "plugins/s3",
			Settings: map[string]value{
				"bucket":       value{fromSecret: "AWS_S3_BUCKET"},
				"access_key":   value{fromSecret: "AWS_ACCESS_KEY_ID"},
				"secret_key":   value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
				"region":       value{raw: "us-west-2"},
				"source":       value{raw: "/go/artifacts/*"},
				"target":       value{raw: "teleport/tag/${DRONE_TAG##v}"},
				"strip_prefix": value{raw: "/go/artifacts/"},
			},
		},
	}
	return p
}

// tagDownloadArtifactCommands generates a set of commands to download appropriate artifacts for creating a package as part of a tag build
func tagDownloadArtifactCommands(params tagBuildType) []string {
	commands := []string{
		`export VERSION=$(cat /go/.version.txt)`,
		`if [[ "${DRONE_TAG}" != "" ]]; then export S3_PATH="tag/$${DRONE_TAG##v}/"; else export S3_PATH="tag/"; fi`,
	}
	artifactOSS := true
	artifactEnterprise := true

	artifactType := fmt.Sprintf("%s-%s", params.os, params.arch)
	if params.fips {
		artifactType += "-fips"
		artifactOSS = false
	}

	if artifactOSS {
		commands = append(commands,
			fmt.Sprintf(`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}teleport-v$${VERSION}-%s-bin.tar.gz /go/artifacts/`, artifactType),
		)
	}
	if artifactEnterprise {
		commands = append(commands,
			fmt.Sprintf(`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}teleport-ent-v$${VERSION}-%s-bin.tar.gz /go/artifacts/`, artifactType),
		)
	}
	return commands
}

// tagCopyPackageArtifactCommands generates a set of commands to find and copy built package artifacts as part of a tag build
func tagCopyPackageArtifactCommands(params tagBuildType, packageType string) []string {
	commands := []string{
		`cd /go/src/github.com/gravitational/teleport`,
	}
	if !params.fips {
		commands = append(commands, fmt.Sprintf(`find build -maxdepth 1 -iname "teleport*.%s*" -print -exec cp {} /go/artifacts \;`, packageType))
	}
	commands = append(commands, fmt.Sprintf(`find e/build -maxdepth 1 -iname "teleport*.%s*" -print -exec cp {} /go/artifacts \;`, packageType))
	return commands
}

// tagPackagePipeline generates a tag package pipeline for a given combination of os/arch/FIPS
func tagPackagePipeline(packageType string, params tagBuildType) pipeline {
	if packageType == "" {
		panic("packageType must be set")
	}
	if params.os == "" {
		panic("params.os must be set")
	}
	if params.arch == "" {
		panic("params.arch must be set")
	}

	environment := map[string]value{
		"ARCH":             value{raw: params.arch},
		"TMPDIR":           value{raw: "/go"},
		"ENT_TARBALL_PATH": value{raw: "/go/artifacts"},
	}

	dependentPipeline := fmt.Sprintf("build-%s-%s", params.os, params.arch)
	packageBuildCommands := []string{
		`apk add --no-cache bash curl gzip make tar`,
		`cd /go/src/github.com/gravitational/teleport`,
		`export VERSION=$(cat /go/.version.txt)`,
	}

	makeCommand := fmt.Sprintf("make %s", packageType)
	if params.fips {
		dependentPipeline += "-fips"
		environment["FIPS"] = value{raw: "yes"}
		environment["RUNTIME"] = value{raw: "fips"}
		makeCommand = fmt.Sprintf("make -C e %s", packageType)
	} else {
		environment["OSS_TARBALL_PATH"] = value{raw: "/go/artifacts"}
	}

	tagVolumes := []volume{
		volumeDocker,
	}
	tagVolumeRefs := []volumeRef{
		volumeRefDocker,
	}
	if packageType == rpmPackage {
		environment["GNUPG_DIR"] = value{raw: "/tmpfs/gnupg"}
		environment["GPG_RPM_SIGNING_ARCHIVE"] = value{fromSecret: "GPG_RPM_SIGNING_ARCHIVE"}
		packageBuildCommands = append(packageBuildCommands,
			`mkdir -m0700 $GNUPG_DIR`,
			`echo "$GPG_RPM_SIGNING_ARCHIVE" | base64 -d | tar -xzf - -C $GNUPG_DIR`,
			`chown -R root:root $GNUPG_DIR`,
			makeCommand,
			`rm -rf $GNUPG_DIR`,
		)
		tagVolumes = []volume{
			volumeDocker,
			volumeTmpfs,
		}
		tagVolumeRefs = []volumeRef{
			volumeRefDocker,
			volumeRefTmpfs,
		}

	} else if packageType == debPackage {
		packageBuildCommands = append(packageBuildCommands,
			makeCommand,
		)
	}

	pipelineName := fmt.Sprintf("%s-%s", dependentPipeline, packageType)

	p := newKubePipeline(pipelineName)
	p.Trigger = triggerTag
	p.DependsOn = []string{dependentPipeline}
	p.Workspace = workspace{Path: "/go"}
	p.Volumes = tagVolumes
	p.Services = []service{
		{
			Name:    "Start Docker",
			Image:   "docker:dind",
			Volumes: tagVolumeRefs,
		},
	}
	p.Steps = []step{
		{
			Name:  "Check out code",
			Image: "docker:git",
			Environment: map[string]value{
				"GITHUB_PRIVATE_KEY": value{fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: tagCheckoutCommands(params.fips),
		},
		{
			Name:  "Download built tarball artifacts from S3",
			Image: "amazon/aws-cli",
			Environment: map[string]value{
				"AWS_REGION":            value{raw: "us-west-2"},
				"AWS_S3_BUCKET":         value{fromSecret: "AWS_S3_BUCKET"},
				"AWS_ACCESS_KEY_ID":     value{fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
			},
			Commands: tagDownloadArtifactCommands(params),
		},
		{
			Name:        fmt.Sprintf("Build %s artifacts", strings.ToUpper(packageType)),
			Image:       "docker",
			Environment: environment,
			Volumes:     tagVolumeRefs,
			Commands:    packageBuildCommands,
		},
		{
			Name:     fmt.Sprintf("Copy %s artifacts", strings.ToUpper(packageType)),
			Image:    "docker",
			Commands: tagCopyPackageArtifactCommands(params, packageType),
		},
		{
			Name:  "Upload to S3",
			Image: "plugins/s3",
			Settings: map[string]value{
				"bucket":       value{fromSecret: "AWS_S3_BUCKET"},
				"access_key":   value{fromSecret: "AWS_ACCESS_KEY_ID"},
				"secret_key":   value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
				"region":       value{raw: "us-west-2"},
				"source":       value{raw: "/go/artifacts/*"},
				"target":       value{raw: "teleport/tag/${DRONE_TAG##v}"},
				"strip_prefix": value{raw: "/go/artifacts/"},
			},
		},
	}
	return p
}
