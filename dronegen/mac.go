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
	"path"
	"path/filepath"
)

const (
	perBuildDir           = "/tmp/build-$DRONE_BUILD_NUMBER-$DRONE_BUILD_CREATED"
	perBuildToolchainsDir = perBuildDir + "/toolchains"
	perBuildCargoDir      = perBuildToolchainsDir + "/cargo"
	perBuildRustupDir     = perBuildToolchainsDir + "/rustup"
)

// escapedPreformatted returns expr wrapped in escaped backticks,
// resulting in Slack "preformatted" string, but safe to use in bash
// without triggering the command expansion.
// This is useful for use in Go backtick literals,
// where backticks can not be escaped in any way.
func escapedPreformatted(expr string) string {
	return fmt.Sprintf("\\`%s\\`", expr)
}

func newDarwinPipeline(name string) pipeline {
	p := newExecPipeline(name)
	p.Workspace.Path = path.Join("/tmp", name)
	p.Concurrency.Limit = 1
	p.Platform = platform{OS: "darwin", Arch: "amd64"}
	return p
}

func darwinConnectDmgPipeline() pipeline {
	b := buildType{os: "darwin", arch: "amd64"}
	toolchainConfig := toolchainConfig{nodejs: true}
	artifactConfig := onlyConnectWithBundledTshApp

	p := newDarwinPipeline("build-darwin-amd64-connect")
	awsConfigPath := filepath.Join(p.Workspace.Path, "credentials")
	p.Trigger = triggerTag
	p.DependsOn = []string{"build-darwin-amd64-pkg-tsh"}
	p.Steps = []step{
		setUpExecStorageStep(p.Workspace.Path),
		{
			Name: "Check out code",
			Environment: map[string]value{
				"WORKSPACE_DIR":      {raw: p.Workspace.Path},
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: darwinTagCheckoutCommands(artifactConfig),
		},
	}
	p.Steps = append(p.Steps,
		installToolchains(p.Workspace.Path, toolchainConfig)...)
	p.Steps = append(p.Steps, []step{
		macAssumeAwsRoleStep(macRoleSettings{
			awsRoleSettings: awsRoleSettings{
				awsAccessKeyID:     value{fromSecret: "AWS_ACCESS_KEY_ID"},
				awsSecretAccessKey: value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
				role:               value{fromSecret: "AWS_ROLE"},
			},
			configPath: awsConfigPath,
		}),
		{
			Name: "Download tsh.pkg artifact from S3",
			Environment: map[string]value{
				"AWS_REGION":                  {raw: "us-west-2"},
				"AWS_S3_BUCKET":               {fromSecret: "AWS_S3_BUCKET"},
				"GITHUB_PRIVATE_KEY":          {fromSecret: "GITHUB_PRIVATE_KEY"},
				"WORKSPACE_DIR":               {raw: p.Workspace.Path},
				"AWS_SHARED_CREDENTIALS_FILE": {raw: awsConfigPath},
			},
			Commands: darwinConnectDownloadArtifactCommands(),
		},
		buildMacArtifactsStep(p.Workspace.Path, b, toolchainConfig, artifactConfig),
		{
			Name: "Copy dmg artifact",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
			},
			Commands: darwinConnectCopyDmgArtifactCommands(),
		},
		{
			Name: "Upload to S3",
			Environment: map[string]value{
				"AWS_S3_BUCKET":               {fromSecret: "AWS_S3_BUCKET"},
				"AWS_REGION":                  {raw: "us-west-2"},
				"WORKSPACE_DIR":               {raw: p.Workspace.Path},
				"AWS_SHARED_CREDENTIALS_FILE": {raw: awsConfigPath},
			},
			Commands: darwinUploadToS3Commands(),
		},
		{
			Name: "Register artifact",
			// Connect's artifact description is automatically generated based on the filename so we pass
			// no packageType and extraQualifications.
			Commands: tagCreateReleaseAssetCommands(b, "", nil),
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
				"RELEASES_CERT": {fromSecret: "RELEASES_CERT"},
				"RELEASES_KEY":  {fromSecret: "RELEASES_KEY"},
			},
		},
		cleanUpToolchainsStep(p.Workspace.Path, toolchainConfig),
		cleanUpExecStorageStep(p.Workspace.Path),
	}...,
	)
	return p
}

func darwinPushPipeline() pipeline {
	b := buildType{os: "darwin", arch: "amd64"}
	toolchainConfig := toolchainConfig{golang: true, rust: true, nodejs: true}
	artifactConfig := binariesWithConnect

	p := newDarwinPipeline("push-build-darwin-amd64")
	p.Trigger = trigger{
		Event:  triggerRef{Include: []string{"push"}, Exclude: []string{"pull_request"}},
		Branch: triggerRef{Include: []string{"master", "branch/*"}},
		Repo:   triggerRef{Include: []string{"gravitational/*"}},
	}
	p.Steps = []step{
		setUpExecStorageStep(p.Workspace.Path),
		{
			Name: "Check out code",
			Environment: map[string]value{
				"WORKSPACE_DIR":      {raw: p.Workspace.Path},
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: pushCheckoutCommandsDarwin(artifactConfig),
		},
	}
	p.Steps = append(p.Steps,
		installToolchains(p.Workspace.Path, toolchainConfig)...)
	p.Steps = append(p.Steps, []step{
		buildMacArtifactsStep(p.Workspace.Path, b, toolchainConfig, artifactConfig),
		cleanUpToolchainsStep(p.Workspace.Path, toolchainConfig),
		cleanUpExecStorageStep(p.Workspace.Path),
		{
			Name:        "Send Slack notification (exec)",
			Environment: map[string]value{"SLACK_WEBHOOK_DEV_TELEPORT": {fromSecret: "SLACK_WEBHOOK_DEV_TELEPORT"}},
			Commands: []string{
				`
export DRONE_BUILD_LINK="${DRONE_SYSTEM_PROTO}://${DRONE_SYSTEM_HOSTNAME}/${DRONE_REPO_OWNER}/${DRONE_REPO_NAME}/${DRONE_BUILD_NUMBER}"
export GOOS=$(go env GOOS)
export GOARCH=$(go env GOARCH)
`,
				fmt.Sprintf(`
curl -sL -X POST -H 'Content-type: application/json' --data "{\"text\":\"Warning: %s artifact build failed for [%s] - please investigate immediately!\nBranch: %s\nCommit: %s\nLink: $DRONE_BUILD_LINK\"}" $SLACK_WEBHOOK_DEV_TELEPORT`,
					escapedPreformatted("${GOOS}-${GOARCH}"),
					escapedPreformatted("${DRONE_REPO_NAME}"),
					escapedPreformatted("${DRONE_BRANCH}"),
					escapedPreformatted("${DRONE_COMMIT_SHA}")),
			},
			When: &condition{Status: []string{"failure"}},
		},
	}...)
	return p
}

func darwinTagPipeline() pipeline {
	b := buildType{
		arch: "amd64",
		os:   "darwin",
	}
	toolchainConfig := toolchainConfig{golang: true, rust: true}
	artifactConfig := onlyBinaries

	p := newDarwinPipeline("build-darwin-amd64")
	awsConfigPath := filepath.Join(p.Workspace.Path, "credentials")
	p.Trigger = triggerTag
	p.DependsOn = []string{tagCleanupPipelineName}
	p.Steps = []step{
		setUpExecStorageStep(p.Workspace.Path),
		{
			Name: "Check out code",
			Environment: map[string]value{
				"WORKSPACE_DIR":      {raw: p.Workspace.Path},
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: darwinTagCheckoutCommands(artifactConfig),
		},
	}
	p.Steps = append(p.Steps,
		installToolchains(p.Workspace.Path, toolchainConfig)...,
	)
	p.Steps = append(p.Steps, []step{
		buildMacArtifactsStep(p.Workspace.Path, b, toolchainConfig, artifactConfig),
		{
			Name: "Copy Mac artifacts",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
			},
			Commands: darwinTagCopyPackageArtifactCommands(),
		},
		macAssumeAwsRoleStep(macRoleSettings{
			awsRoleSettings: awsRoleSettings{
				awsAccessKeyID:     value{fromSecret: "AWS_ACCESS_KEY_ID"},
				awsSecretAccessKey: value{fromSecret: "AWS_SECRET_ACCESS_KEY"},
				role:               value{fromSecret: "AWS_ROLE"},
			},
			configPath: awsConfigPath,
		}),
		{
			Name: "Upload to S3",
			Environment: map[string]value{
				"AWS_S3_BUCKET":               {fromSecret: "AWS_S3_BUCKET"},
				"AWS_REGION":                  {raw: "us-west-2"},
				"WORKSPACE_DIR":               {raw: p.Workspace.Path},
				"AWS_SHARED_CREDENTIALS_FILE": {raw: awsConfigPath},
			},
			Commands: darwinUploadToS3Commands(),
		},
		{
			Name: "Register artifacts",
			// Binaries built by this pipeline don't require extra description, so we don't pass
			// packageType and extraQualifications.
			Commands: tagCreateReleaseAssetCommands(b, "", nil),
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
				"RELEASES_CERT": {fromSecret: "RELEASES_CERT"},
				"RELEASES_KEY":  {fromSecret: "RELEASES_KEY"},
			},
		},
		cleanUpToolchainsStep(p.Workspace.Path, toolchainConfig),
		cleanUpExecStorageStep(p.Workspace.Path),
	}...)
	return p
}

func pushCheckoutCommandsDarwin(artifactConfig darwinArtifactConfig) []string {
	commands := []string{
		`set -u`,
		`mkdir -p $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
		`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
		`git clone https://github.com/gravitational/${DRONE_REPO_NAME}.git .`,
		`git checkout ${DRONE_TAG:-$DRONE_COMMIT}`,
		// suppressing the newline on the end of the private key makes git operations fail on MacOS
		// with an error like 'Load key "/path/.ssh/id_rsa": invalid format'
		`mkdir -m 0700 $WORKSPACE_DIR/.ssh && echo "$GITHUB_PRIVATE_KEY" > $WORKSPACE_DIR/.ssh/id_rsa && chmod 600 $WORKSPACE_DIR/.ssh/id_rsa`,
		`ssh-keyscan -H github.com > $WORKSPACE_DIR/.ssh/known_hosts 2>/dev/null`,
		`chmod 600 $WORKSPACE_DIR/.ssh/known_hosts`,
	}

	// clone github.com/gravitational/webapps for the Teleport Connect source code
	if artifactConfig == binariesWithConnect || artifactConfig == onlyConnectWithBundledTshApp {
		commands = append(commands,
			`mkdir -p $WORKSPACE_DIR/go/src/github.com/gravitational/webapps`,
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/webapps`,
			`git clone https://github.com/gravitational/webapps.git .`,
			`git checkout $($WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build.assets/webapps/webapps-version.sh)`,
			`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init packages/webapps.e`,
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
		)
	}

	commands = append(commands,
		// fetch enterprise submodules
		`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init e`,
		// this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
		`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init --recursive webassets || true`,
		`rm -rf $WORKSPACE_DIR/.ssh`,
		`mkdir -p $WORKSPACE_DIR/go/cache`,
	)

	return commands
}

func setUpExecStorageStep(path string) step {
	return step{
		Name:        "Set up exec runner storage",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: path}},
		Commands: []string{
			"set -u",
			"mkdir -p $WORKSPACE_DIR",
			"chmod -R u+rw $WORKSPACE_DIR",
			"rm -rf $WORKSPACE_DIR/go $WORKSPACE_DIR/.ssh",
		},
	}
}

type toolchainConfig struct {
	golang bool
	rust   bool
	nodejs bool
}

func installToolchains(workspacePath string, config toolchainConfig) (steps []step) {
	if config.golang {
		steps = append(steps, installGoToolchainStep())
	}

	if config.rust {
		steps = append(steps, installRustToolchainStep(workspacePath))
	}

	if config.nodejs {
		steps = append(steps, installNodeToolchainStep(workspacePath))
	}

	return steps
}

func installGoToolchainStep() step {
	return step{
		Name: "Install Go Toolchain",
		Environment: map[string]value{
			"RUNTIME": goRuntime,
		},
		Commands: []string{
			`set -u`,
			`mkdir -p ` + perBuildToolchainsDir,
			`curl --silent -O https://dl.google.com/go/$RUNTIME.darwin-amd64.tar.gz`,
			`tar -C  ` + perBuildToolchainsDir + ` -xzf $RUNTIME.darwin-amd64.tar.gz`,
			`rm -rf $RUNTIME.darwin-amd64.tar.gz`,
		},
	}
}

func installRustToolchainStep(path string) step {
	return step{
		Name:        "Install Rust Toolchain",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: path}},
		Commands: []string{
			`set -u`,
			`export PATH=/Users/$(whoami)/.cargo/bin:$PATH`, // use the system-installed rustup to install our custom Rust version
			`mkdir -p ` + perBuildToolchainsDir,
			`export RUST_VERSION=$(make -C $WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build.assets print-rust-version)`,
			`export CARGO_HOME=` + perBuildCargoDir,
			`export RUST_HOME=$CARGO_HOME`,
			`export RUSTUP_HOME=` + perBuildRustupDir,
			`rustup toolchain install $RUST_VERSION`,
		},
	}
}

func installNodeToolchainStep(workspacePath string) step {
	return step{
		Name:        "Install Node Toolchain",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		Commands: []string{
			`set -u`,
			`export NODE_VERSION=$(make -C $WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build.assets print-node-version)`,
			`export TOOLCHAIN_DIR=` + perBuildToolchainsDir,
			`export NODE_DIR=$TOOLCHAIN_DIR/node-v$NODE_VERSION-darwin-x64`,
			`mkdir -p $TOOLCHAIN_DIR`,
			`curl --silent -O https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-darwin-x64.tar.gz`,
			`tar -C $TOOLCHAIN_DIR -xzf node-v$NODE_VERSION-darwin-x64.tar.gz`,
			`rm -f node-v$NODE_VERSION-darwin-x64.tar.gz`,
			`export PATH=$NODE_DIR/bin:$PATH`,
			`corepack enable yarn`,
			`echo Node reporting version $(node --version)`,
			`echo Yarn reporting version $(yarn --version)`,
		},
	}
}

func configureToolchainsCommands(config toolchainConfig) []string {
	commands := []string{
		// HOME needs to be set to the actual home directory of a macOS user rather than the temporary
		// directory that Drone sets it to by default. This way we're able to unlock Keychain which is
		// needed for Connect signing.
		//
		// Hence, the toolchains are not installed within the temporary home dir but a separate
		// TOOLCHAIN_DIR. Every pipeline in this file follows this pattern even though technically we
		// need to unlock Keychain only for the build-darwin-amd64-connect pipeline.
		`export HOME=/Users/$(whoami)`,
		`export TOOLCHAIN_DIR=` + perBuildToolchainsDir,
	}

	// Configure toolchains in descending order so that Node.js is added to PATH last.
	// We expect that Node.js will add the most packages so we want to avoid any bin conflicts with Go
	// or Rust toolchains.
	if config.nodejs {
		commands = append(commands,
			`export NODE_VERSION=$(make -C $WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build.assets print-node-version)`,
			`export NODE_HOME=$TOOLCHAIN_DIR/node-v$NODE_VERSION-darwin-x64`,
			`export PATH=$NODE_HOME/bin:$PATH`,
		)
	}

	if config.rust {
		commands = append(commands,
			`export RUST_VERSION=$(make -C $WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build.assets print-rust-version)`,
			`export CARGO_HOME=`+perBuildCargoDir,
			`export RUST_HOME=$CARGO_HOME`,
			`export RUSTUP_HOME=`+perBuildRustupDir,
			`export PATH=$CARGO_HOME/bin:/Users/build/.cargo/bin:$PATH`,
			`rustup override set $RUST_VERSION`,
		)
	}

	if config.golang {
		commands = append(commands,
			`export PATH=$TOOLCHAIN_DIR/go/bin:$PATH`,
		)
	}

	return commands
}

func cleanUpToolchainsStep(workspacePath string, config toolchainConfig) step {
	step := step{
		Name:        "Clean up toolchains (post)",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: workspacePath}},
		When: &condition{
			Status: []string{"success", "failure"},
		},
		Commands: []string{
			`set -u`,
		},
	}

	if config.rust {
		step.Commands = append(step.Commands,
			`export PATH=/Users/$(whoami)/.cargo/bin:$PATH`,
			`export CARGO_HOME=`+perBuildCargoDir,
			`export RUST_HOME=$CARGO_HOME`,
			`export RUSTUP_HOME=`+perBuildRustupDir,
			`export RUST_VERSION=$(make -C $WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build.assets print-rust-version)`,
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			// clean up the rust toolchain even though we're about to delete the directory
			// this ensures we don't leave behind a broken link
			`rustup override unset`,
			`rustup toolchain uninstall $RUST_VERSION`,
		)
	}

	step.Commands = append(step.Commands,
		`rm -rf `+perBuildDir,
	)

	return step
}

func cleanUpExecStorageStep(path string) step {
	return step{
		Name:        "Clean up exec runner storage (post)",
		Environment: map[string]value{"WORKSPACE_DIR": {raw: path}},
		Commands: []string{
			`set -u`,
			`chmod -R u+rw $WORKSPACE_DIR`,
			`rm -rf $WORKSPACE_DIR/go $WORKSPACE_DIR/.ssh`,
		},
	}
}

func darwinTagCheckoutCommands(artifactConfig darwinArtifactConfig) []string {
	return append(
		pushCheckoutCommandsDarwin(artifactConfig),
		`mkdir -p $WORKSPACE_DIR/go/artifacts`,
		`echo "${DRONE_TAG##v}" > $WORKSPACE_DIR/go/.version.txt`,
		`cat $WORKSPACE_DIR/go/.version.txt`,
	)
}

// darwinArtifactConfig describes artifacts made by the build step in different macOS pipelines.
//
// On a commit push, we run one pipeline that builds artifacts (darwinPushPipeline). It uses
// binariesWithConnect as the artifact config as it only checks if we can still compile/build the
// artifacts after a commit lands in master.
//
// On a version tag push, we run two pipelines from this file that build artifacts. First we run
// darwinTagPipeline with onlyBinaries as the artifact config. It builds, among others, the tsh
// binary which later gets signed, bundled into tsh.app and packaged into a .pkg file.
//
// After that, we run darwinConnectDmgPipeline with onlyConnectWithBundledTshApp as the artifact
// config. darwinConnectDmgPipeline downloads the signed tsh.app bundle and puts it within Connect's
// own bundle.
type darwinArtifactConfig int

const (
	onlyBinaries darwinArtifactConfig = iota
	binariesWithConnect
	onlyConnectWithBundledTshApp
)

func buildMacArtifactsStep(workspacePath string, b buildType, toolchainConfig toolchainConfig, artifactConfig darwinArtifactConfig) step {
	step := step{
		Name: "Build Mac artifacts",
		Environment: map[string]value{
			"GOPATH":            {raw: path.Join(workspacePath, "/go")},
			"GOCACHE":           {raw: path.Join(workspacePath, "/go/cache")},
			"OS":                {raw: b.os},
			"ARCH":              {raw: b.arch},
			"WORKSPACE_DIR":     {raw: workspacePath},
			"BUILDBOX_PASSWORD": {fromSecret: "BUILDBOX_PASSWORD"},
		},
		Commands: darwinBuildCommands(toolchainConfig, artifactConfig),
	}

	var artifactDesc string
	switch artifactConfig {
	case onlyBinaries:
		artifactDesc = "binaries"
	case binariesWithConnect:
		artifactDesc = "binaries and Teleport Connect"
	case onlyConnectWithBundledTshApp:
		artifactDesc = "Teleport Connect"
	}
	step.Name = step.Name + " (" + artifactDesc + ")"

	if artifactConfig == onlyConnectWithBundledTshApp {
		// These credentials are necessary for the signing and notarization of Teleport Connect, which
		// is built in to the Electron tooling.
		// The rest of the mac artifacts are signed and notarized with gon in the darwin pkg pipeline.
		step.Environment["APPLE_USERNAME"] = value{fromSecret: "APPLE_USERNAME"}
		step.Environment["APPLE_PASSWORD"] = value{fromSecret: "APPLE_PASSWORD"}
	}

	return step
}

func darwinBuildCommands(toolchainConfig toolchainConfig, artifactConfig darwinArtifactConfig) []string {
	commands := []string{
		`set -u`,
	}
	commands = append(commands, configureToolchainsCommands(toolchainConfig)...)

	// Commands for building binaries.
	if artifactConfig == onlyBinaries || artifactConfig == binariesWithConnect {
		commands = append(commands,
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			`build.assets/build-fido2-macos.sh build`,
			`export PKG_CONFIG_PATH="$(build.assets/build-fido2-macos.sh pkg_config_path)"`,
			`make clean release OS=$OS ARCH=$ARCH FIDO2=yes TOUCHID=yes PIV=yes`,
		)
	}

	// Commands for building Teleport Connect.
	if artifactConfig == binariesWithConnect || artifactConfig == onlyConnectWithBundledTshApp {
		commands = append(commands,
			`export VERSION=$(make -C $WORKSPACE_DIR/go/src/github.com/gravitational/teleport print-version)`,
			// BUILD_NUMBER is used by electron-builder to add an extra fourth integer to CFBundleVersion on macOS.
			// This makes the full app version look like this: 9.3.5.12489
			// https://www.electron.build/configuration/configuration.html#Configuration-buildVersion
			`export BUILD_NUMBER=$DRONE_BUILD_NUMBER`,

			// Unlock Keychain so that electron-builder can use developer ID cert for signing.
			`security unlock-keychain -p $${BUILDBOX_PASSWORD} login.keychain`,
			`security find-identity -v`,
			// CSC_NAME tells electron-builder which cert to use for signing when there are multiple certs
			// available.
			// https://www.electron.build/code-signing
			`export CSC_NAME=0FFD3E3413AB4C599C53FBB1D8CA690915E33D83`,
		)

		if artifactConfig == binariesWithConnect {
			commands = append(commands,
				`export CONNECT_TSH_BIN_PATH=$WORKSPACE_DIR/go/src/github.com/gravitational/teleport/build/tsh`,
			)
		}

		if artifactConfig == onlyConnectWithBundledTshApp {
			commands = append(commands,
				// Unpack tsh.pkg.
				`cd $WORKSPACE_DIR/go/src/github.com/gravitational`,
				`pkgutil --expand-full tsh-$${VERSION}.pkg tsh`,
				`export CONNECT_TSH_APP_PATH=$WORKSPACE_DIR/go/src/github.com/gravitational/tsh/Payload/tsh.app`,
			)
		}

		commands = append(commands,
			// Build and package Connect
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/webapps`,
			// c.extraMetadata.version overwrites the version property from package.json to $VERSION
			// https://www.electron.build/configuration/configuration.html#Configuration-extraMetadata
			`yarn install && yarn build-term && yarn package-term -c.extraMetadata.version=$VERSION`,
		)
	}

	return commands
}

func darwinTagCopyPackageArtifactCommands() []string {
	commands := []string{
		`set -u`,
		`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
		// copy release archives to artifact directory
		`cp teleport*.tar.gz $WORKSPACE_DIR/go/artifacts`,
		`cp e/teleport-ent*.tar.gz $WORKSPACE_DIR/go/artifacts`,
		// generate checksums
		`cd $WORKSPACE_DIR/go/artifacts && for FILE in teleport*.tar.gz; do shasum -a 256 $FILE > $FILE.sha256; done && ls -l`,
	}

	return commands
}

func darwinConnectCopyDmgArtifactCommands() []string {
	commands := []string{
		`set -u`,
		// copy dmg to artifact directory
		`cd $WORKSPACE_DIR/go/src/github.com/gravitational/webapps/packages/teleterm/build/release`,
		`cp *.dmg $WORKSPACE_DIR/go/artifacts`,
		// generate checksums
		`cd $WORKSPACE_DIR/go/artifacts && for FILE in *.dmg; do shasum -a 256 "$FILE" > "$FILE.sha256"; done && ls -l`,
	}

	return commands
}

func darwinUploadToS3Commands() []string {
	return []string{
		`set -u`,
		`cd $WORKSPACE_DIR/go/artifacts`,
		`aws s3 sync . s3://$AWS_S3_BUCKET/teleport/tag/${DRONE_TAG##v}`,
	}
}

func darwinConnectDownloadArtifactCommands() []string {
	return []string{
		`set -u`,
		`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
		`export S3_PATH="tag/$${DRONE_TAG##v}/"`,
		// Download tsh.pkg. We're going to extract tsh.app from it which is then packaged within the
		// Teleport Connect bundle.
		`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}tsh-$${VERSION}.pkg $WORKSPACE_DIR/go/src/github.com/gravitational/`,
	}
}
