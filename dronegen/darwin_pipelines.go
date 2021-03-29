package main

import (
	"fmt"
	"path/filepath"
)

const boringCryptoBranch = "dev.boringcrypto.go1.16"

// darwinTagPipelines builds all applicable tag pipeline combinations on darwin
func darwinTagPipelines() []pipeline {
	b := buildType{
		os:   "darwin",
		arch: "amd64",
		fips: true,
	}
	return []pipeline{
		newDarwinTagPipeline(b),
		newDarwinTagPackagePipeline(b),
		newDarwinTagTshPackagePipeline(b),
	}
}

// darwinPushPipelines builds all applicable push pipeline combinations on darwin
func darwinPushPipelines() []pipeline {
	return []pipeline{newDarwinPushPipeline(buildType{
		os:   "darwin",
		arch: "amd64",
		fips: true,
	})}
}

// newDarwinPushPipeline generates a push pipeline for a given combination of os/arch/FIPS on darwin
func newDarwinPushPipeline(b buildType) pipeline {
	pipelineName := fmt.Sprintf("push-build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName += "-fips"
	}

	p := newExecPipeline(pipelineName)
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
	}
	p.Platform = b.platform()
	p.Trigger = triggerPush
	p.Workspace = workspace{Path: filepath.Join("/tmp", pipelineName)}

	builder := darwinPushPipelineStepBuilder{
		buildType:          b,
		workspace:          p.Workspace,
		boringCryptoBranch: boringCryptoBranch,
	}

	p.Steps = builder.build()

	return p
}

// newDarwinTagPipeline generates a tag pipeline for a given combination of os/arch/FIPS on darwin
func newDarwinTagPipeline(b buildType) pipeline {
	pipelineName := fmt.Sprintf("build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName += "-fips"
	}

	p := newExecPipeline(pipelineName)
	p.Platform = b.platform()
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
	}
	p.Trigger = triggerTag
	p.Workspace = workspace{Path: filepath.Join("/tmp", pipelineName)}

	builder := darwinTagPipelineStepBuilder{
		buildType:          b,
		workspace:          p.Workspace,
		boringCryptoBranch: boringCryptoBranch,
	}

	p.Steps = builder.build()

	return p
}

// newDarwinTagPackagePipeline generates a tag package pipeline for a given combination of os/arch/FIPS on darwin
func newDarwinTagPackagePipeline(b buildType) pipeline {
	pipelineName := fmt.Sprintf("build-%s-%s-pkg", b.os, b.arch)
	dependentPipeline := fmt.Sprintf("build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName = fmt.Sprintf("build-%s-%s-fips-pkg", b.os, b.arch)
		dependentPipeline = fmt.Sprintf("build-%s-%s-fips", b.os, b.arch)
	}

	p := newExecPipeline(pipelineName)
	p.Platform = b.platform()
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
	}
	p.Trigger = triggerTag
	p.DependsOn = []string{dependentPipeline}
	p.Workspace = workspace{Path: filepath.Join("/tmp", pipelineName)}

	builder := darwinTagPackagePipelineStepBuilder{
		buildType:          b,
		workspace:          p.Workspace,
		boringCryptoBranch: boringCryptoBranch,
	}

	p.Steps = builder.build()

	return p
}

// newDarwinTagTshPackagePipeline generates a tag tsh package pipeline for a given combination of os/arch/FIPS on darwin
func newDarwinTagTshPackagePipeline(b buildType) pipeline {
	pipelineName := fmt.Sprintf("build-%s-%s-pkg-tsh", b.os, b.arch)
	dependentPipeline := fmt.Sprintf("build-%s-%s", b.os, b.arch)
	if b.fips {
		pipelineName = fmt.Sprintf("build-%s-%s-fips-pkg-tsh", b.os, b.arch)
		dependentPipeline = fmt.Sprintf("build-%s-%s-fips", b.os, b.arch)
	}

	p := newExecPipeline(pipelineName)
	p.Platform = b.platform()
	p.Environment = map[string]value{
		"RUNTIME": goRuntime,
	}
	p.Trigger = triggerTag
	p.DependsOn = []string{dependentPipeline}
	p.Workspace = workspace{Path: filepath.Join("/tmp", pipelineName)}

	builder := darwinTagTshPackagePipelineStepBuilder{
		buildType:          b,
		workspace:          p.Workspace,
		boringCryptoBranch: boringCryptoBranch,
	}

	p.Steps = builder.build()

	return p
}

func (r darwinPushPipelineStepBuilder) build() []step {
	return []step{
		setupExecStorage(r.workspace.Path),
		r.checkout(),
		buildGoBuilder(r.workspace, r.boringCryptoBranch),
		pullGoBuilderTarball(r.workspace, r.boringCryptoBranch),
		buildMacArtifacts(r.buildType, r.workspace, r.boringCryptoBranch),
		cleanupExecStorage(r.workspace.Path),
		sendSlackNotification(),
	}
}

// checkout creates a pipeline step to checkout source code for a push build on darwin
func (r darwinPushPipelineStepBuilder) checkout() step {
	return step{
		Name: "Check out code",
		Environment: map[string]value{
			"GITHUB_PRIVATE_KEY":  {fromSecret: "GITHUB_PRIVATE_KEY"},
			"WORKSPACE_DIR":       {raw: r.workspace.Path},
			"TELEPORT_DIR":        {raw: r.workspace.join("go/src/github.com/gravitational/teleport")},
			"GODL_DIR":            {raw: r.workspace.join("go/src/github.com/gravitational/godl")},
			"BORINGCRYPTO_BRANCH": {raw: r.boringCryptoBranch},
		},
		Commands: []string{
			`mkdir -p $TELEPORT_DIR $GODL_DIR`,
			`cd $GODL_DIR`,
			`git clone https://github.com/gravitational/godl.git .`,
			`git checkout $BORINGCRYPTO_BRANCH`,
			`cd $TELEPORT_DIR`,
			`git clone https://github.com/gravitational/teleport.git .`,
			`git checkout ${DRONE_TAG:-$DRONE_COMMIT}`,
			// fetch enterprise submodules
			// suppressing the newline on the end of the private key makes git operations fail on MacOS
			// with an error like 'Load key "/path/.ssh/id_rsa": invalid format'
			`mkdir -m 0700 $WORKSPACE_DIR/.ssh && echo "$GITHUB_PRIVATE_KEY" > $WORKSPACE_DIR/.ssh/id_rsa && chmod 600 $WORKSPACE_DIR/.ssh/id_rsa`,
			`ssh-keyscan -H github.com > $WORKSPACE_DIR/.ssh/known_hosts 2>/dev/null`,
			`chmod 600 $WORKSPACE_DIR/.ssh/known_hosts`,
			`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init e`,
			// this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
			`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init --recursive webassets || true`,
			// set version
			`if [[ "${DRONE_TAG}" != "" ]]; then echo "${DRONE_TAG##v}" > $WORKSPACE_DIR/go/.version.txt; else make print-version > $WORKSPACE_DIR/go/.version.txt; fi; cat $WORKSPACE_DIR/go/.version.txt`,
			`rm -rf $WORKSPACE_DIR/.ssh`,
			`mkdir -p $WORKSPACE_DIR/go/cache`,
		},
	}
}

type darwinPushPipelineStepBuilder struct {
	buildType
	workspace
	boringCryptoBranch string
}

// copyArtifacts creates a pipeline step to copy the built artifacts to the designated location on darwin
func (r darwinTagPipelineStepBuilder) copyArtifacts() step {
	return step{
		Name: "Copy Mac artifacts",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: r.workspace.Path},
		},
		Commands: []string{
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			// copy release archives to artifact directory
			`cp e/teleport-ent*.tar.gz $WORKSPACE_DIR/go/artifacts`,
			// generate checksums (for mac)
			`cd $WORKSPACE_DIR/go/artifacts && for FILE in teleport*.tar.gz; do shasum -a 256 $FILE > $FILE.sha256; done && ls -l`,
		},
	}
}

func (r darwinTagPipelineStepBuilder) build() []step {
	return []step{
		setupExecStorage(r.workspace.Path),
		darwinTagCheckout(r.workspace, r.boringCryptoBranch),
		buildGoBuilder(r.workspace, r.boringCryptoBranch),
		pullGoBuilderTarball(r.workspace, r.boringCryptoBranch),
		buildMacArtifacts(r.buildType, r.workspace, r.boringCryptoBranch),
		r.copyArtifacts(),
		darwinUploadArtifactsToS3(r.workspace.Path),
		cleanupExecStorage(r.workspace.Path),
	}
}

type darwinTagPipelineStepBuilder struct {
	buildType
	workspace
	boringCryptoBranch string
}

func (r darwinTagPackagePipelineStepBuilder) buildArtifacts() step {
	environ := map[string]value{
		"OS":                {raw: r.buildType.os},
		"ARCH":              {raw: r.buildType.arch},
		"APPLE_USERNAME":    {fromSecret: "APPLE_USERNAME"},
		"APPLE_PASSWORD":    {fromSecret: "APPLE_PASSWORD"},
		"BUILDBOX_PASSWORD": {fromSecret: "BUILDBOX_PASSWORD"},
		"ENT_TARBALL_PATH":  {raw: r.workspace.join("go/artifacts")},
		"WORKSPACE_DIR":     {raw: r.workspace.Path},
	}
	if r.buildType.fips {
		environ["RUNTIME"] = value{raw: "fips"}
	}
	return step{
		Name:        "Build Mac pkg release artifacts",
		Environment: environ,
		Commands: []string{
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
			`export HOME=/Users/build`,
			// unlock login keychain
			`security unlock-keychain -p $${BUILDBOX_PASSWORD} login.keychain`,
			// show available certificates
			`security find-identity -v`,
			`make -C e pkg VERSION=$VERSION OS=$OS ARCH=$ARCH RUNTIME=$RUNTIME`,
		},
	}
}

// copyArtifacts creates a pipeline step to copy the built artifacts to the designated location on darwin
func (r darwinTagPackagePipelineStepBuilder) copyArtifacts() step {
	return step{
		Name: "Copy Mac pkg artifacts",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: r.workspace.Path},
		},
		Commands: []string{
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			// delete temporary tarball artifacts so we don't re-upload them in the next stage
			`rm -rf $WORKSPACE_DIR/go/artifacts/*.tar.gz`,
			// copy release archives to artifact directory
			`cp e/build/teleport-ent*.pkg $WORKSPACE_DIR/go/artifacts`,
			// generate checksums (for mac)
			`cd $WORKSPACE_DIR/go/artifacts && for FILE in teleport*.pkg; do shasum -a 256 $FILE > $FILE.sha256; done && ls -l`,
		},
	}
}

func (r darwinTagPackagePipelineStepBuilder) build() []step {
	return []step{
		setupExecStorage(r.workspace.Path),
		darwinTagCheckout(r.workspace, r.boringCryptoBranch),
		darwinDownloadTarballFromS3(r.workspace.Path, r.buildType),
		r.buildArtifacts(),
		r.copyArtifacts(),
		darwinUploadArtifactsToS3(r.workspace.Path),
		cleanupExecStorage(r.workspace.Path),
	}
}

type darwinTagPackagePipelineStepBuilder struct {
	buildType
	workspace
	boringCryptoBranch string
}

func (r darwinTagTshPackagePipelineStepBuilder) buildArtifacts() step {
	environ := map[string]value{
		"OS":                {raw: r.buildType.os},
		"ARCH":              {raw: r.buildType.arch},
		"APPLE_USERNAME":    {fromSecret: "APPLE_USERNAME"},
		"APPLE_PASSWORD":    {fromSecret: "APPLE_PASSWORD"},
		"BUILDBOX_PASSWORD": {fromSecret: "BUILDBOX_PASSWORD"},
		"ENT_TARBALL_PATH":  {raw: r.workspace.join("go/artifacts")},
		"WORKSPACE_DIR":     {raw: r.workspace.Path},
	}
	if r.buildType.fips {
		environ["RUNTIME"] = value{raw: "fips"}
	}
	return step{
		Name:        "Build Mac tsh pkg release artifacts",
		Environment: environ,
		Commands: []string{
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
			// set HOME explicitly (as Drone overrides it normally)
			`export HOME=/Users/build`,
			// unlock login keychain
			`security unlock-keychain -p $${BUILDBOX_PASSWORD} login.keychain`,
			// show available certificates
			`security find-identity -v`,
			// build pkg
			`make -C e pkg-tsh VERSION=$VERSION OS=$OS ARCH=$ARCH RUNTIME=$RUNTIME`,
		},
	}
}

// copyArtifacts creates a pipeline step to copy the built tsh package artifacts to the designated location on darwin
func (r darwinTagTshPackagePipelineStepBuilder) copyArtifacts() step {
	return step{
		Name: "Copy Mac tsh pkg artifacts",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: r.workspace.Path},
		},
		Commands: []string{
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			// delete temporary tarball artifacts so we don't re-upload them in the next stage
			`rm -rf $WORKSPACE_DIR/go/artifacts/*.tar.gz`,
			// copy release archives to artifact directory
			`cp e/build/tsh*.pkg $WORKSPACE_DIR/go/artifacts`,
			// generate checksums (for mac)
			`cd $WORKSPACE_DIR/go/artifacts && for FILE in tsh*.pkg; do shasum -a 256 $FILE > $FILE.sha256; done && ls -l`,
		},
	}
}

func (r darwinTagTshPackagePipelineStepBuilder) build() []step {
	return []step{
		setupExecStorage(r.workspace.Path),
		darwinTagCheckout(r.workspace, r.boringCryptoBranch),
		darwinDownloadTarballFromS3(r.workspace.Path, r.buildType),
		r.buildArtifacts(),
		r.copyArtifacts(),
		darwinUploadArtifactsToS3(r.workspace.Path),
		cleanupExecStorage(r.workspace.Path),
	}
}

type darwinTagTshPackagePipelineStepBuilder struct {
	buildType
	workspace
	boringCryptoBranch string
}

// setupExecStorage creates a pipeline step to prepare exec runner's environment
func setupExecStorage(workspaceDir string) step {
	return step{
		Name: "Set up exec runner storage",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspaceDir},
		},
		Commands: []string{
			`mkdir -p $WORKSPACE_DIR`,
			`chmod -R u+rw $WORKSPACE_DIR`,
			`rm -rf $WORKSPACE_DIR/go $WORKSPACE_DIR/.ssh`,
		},
	}
}

func darwinTagCheckout(workspace workspace, boringCryptoBranch string) step {
	return step{
		Name: "Check out code",
		Environment: map[string]value{
			"GITHUB_PRIVATE_KEY":  {fromSecret: "GITHUB_PRIVATE_KEY"},
			"WORKSPACE_DIR":       {raw: workspace.Path},
			"TELEPORT_DIR":        {raw: workspace.join("go/src/github.com/gravitational/teleport")},
			"GODL_DIR":            {raw: workspace.join("go/src/github.com/gravitational/godl")},
			"BORINGCRYPTO_BRANCH": {raw: boringCryptoBranch},
		},
		Commands: []string{
			`mkdir -p $TELEPORT_DIR $GODL_DIR`,
			`cd $GODL_DIR`,
			`git clone https://github.com/gravitational/godl.git .`,
			`git checkout $BORINGCRYPTO_BRANCH`,
			`cd $TELEPORT_DIR`,
			`git clone https://github.com/gravitational/teleport.git .`,
			`git checkout ${DRONE_TAG:-$DRONE_COMMIT}`,
			// fetch enterprise submodules
			// suppressing the newline on the end of the private key makes git operations fail on MacOS
			// with an error like 'Load key "/path/.ssh/id_rsa": invalid format'
			`mkdir -m 0700 $WORKSPACE_DIR/.ssh && echo "$GITHUB_PRIVATE_KEY" > $WORKSPACE_DIR/.ssh/id_rsa && chmod 600 $WORKSPACE_DIR/.ssh/id_rsa`,
			`ssh-keyscan -H github.com > $WORKSPACE_DIR/.ssh/known_hosts 2>/dev/null`,
			`chmod 600 $WORKSPACE_DIR/.ssh/known_hosts`,
			`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init e`,
			// this is allowed to fail because pre-4.3 Teleport versions don't use the webassets submodule
			`GIT_SSH_COMMAND='ssh -i $WORKSPACE_DIR/.ssh/id_rsa -o UserKnownHostsFile=$WORKSPACE_DIR/.ssh/known_hosts -F /dev/null' git submodule update --init --recursive webassets || true`,
			`rm -rf $WORKSPACE_DIR/.ssh`,
			`mkdir -p $WORKSPACE_DIR/go/artifacts $WORKSPACE_DIR/go/cache`,
			`echo "${DRONE_TAG##v}" > $WORKSPACE_DIR/go/.version.txt`,
			`cat $WORKSPACE_DIR/go/.version.txt`,
		},
	}
}

func buildMacArtifacts(b buildType, workspace workspace, boringCryptoBranch string) step {
	environ := map[string]value{
		"GOPATH":  {raw: workspace.join("go")},
		"GOCACHE": {raw: workspace.join("go/cache")},
		// use custom Go compiler
		"GODL_DIR":            {raw: workspace.join("go/src/github.com/gravitational/godl")},
		"OS":                  {raw: b.os},
		"ARCH":                {raw: b.arch},
		"WORKSPACE_DIR":       {raw: workspace.Path},
		"BORINGCRYPTO_BRANCH": {raw: boringCryptoBranch},
	}
	if b.fips {
		environ["FIPS"] = value{raw: "yes"}
	}
	return step{
		Name:        "Build Mac release artifacts",
		Environment: environ,
		Commands: []string{
			`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
			`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
			`make -C e clean release GITTAG=v$VERSION VERSION=$VERSION OS=$OS ARCH=$ARCH FIPS=$FIPS GO_BINARY="$GODL_DIR/build/$BORINGCRYPTO_BRANCH"`,
		},
	}
}

func buildGoBuilder(workspace workspace, boringCryptoBranch string) step {
	return step{
		Name: "Build the boringcrypto Go builder",
		Environment: map[string]value{
			"BORINGCRYPTO_BRANCH": {raw: boringCryptoBranch},
			"WORKSPACE_DIR":       {raw: workspace.Path},
			"GODL_DIR":            {raw: workspace.join("go/src/github.com/gravitational/godl")},
		},
		Commands: []string{
			`cd $GODL_DIR`,
			`mkdir build`,
			`go build -o $GODL_DIR/build/$BORINGCRYPTO_BRANCH $BORINGCRYPTO_BRANCH/main.go`,
		},
	}
}

func pullGoBuilderTarball(workspace workspace, boringCryptoBranch string) step {
	return step{
		Name: "Pull boringcrypto build tarball from S3",
		Environment: fromEnviron(s3Environ, map[string]value{
			"GODL_DIR":            {raw: workspace.join("go/src/github.com/gravitational/godl")},
			"BORINGCRYPTO_BRANCH": {raw: boringCryptoBranch},
		}),
		Commands: []string{
			`mkdir -p $HOME/sdk/$BORINGCRYPTO_BRANCH`,
			`cd $HOME/sdk/$BORINGCRYPTO_BRANCH`,
			`aws s3 cp s3://$AWS_S3_BUCKET/ci/go/$BORINGCRYPTO_BRANCH-darwin-amd64.tar.gz .`,
			`aws s3 cp s3://$AWS_S3_BUCKET/ci/go/$BORINGCRYPTO_BRANCH-darwin-amd64.tar.gz.sha256 .`,
			`$GODL_DIR/build/$BORINGCRYPTO_BRANCH download`,
		},
	}
}

func darwinDownloadTarballFromS3(workspaceDir string, b buildType) step {
	tarballName := fmt.Sprintf("teleport-ent-v$${VERSION}-%s-%s-bin.tar.gz", b.os, b.arch)
	if b.fips {
		tarballName = fmt.Sprintf("teleport-ent-v$${VERSION}-%s-%s-fips-bin.tar.gz", b.os, b.arch)
	}
	return step{
		Name: "Download built tarball artifact from S3",
		Environment: fromEnviron(s3Environ, map[string]value{
			"WORKSPACE_DIR": {raw: workspaceDir},
		}),
		Commands: []string{
			`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
			`export S3_PATH="tag/$${DRONE_TAG##v}/"`,
			fmt.Sprintf(`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}%s $WORKSPACE_DIR/go/artifacts/`, tarballName),
		},
	}
}

func darwinUploadArtifactsToS3(workspaceDir string) step {
	return step{
		Name: "Upload to S3",
		Environment: fromEnviron(s3Environ, map[string]value{
			"WORKSPACE_DIR": {raw: workspaceDir},
		}),
		Commands: []string{
			`cd $WORKSPACE_DIR/go/artifacts`,
			`aws s3 sync . s3://$AWS_S3_BUCKET/teleport/tag/${DRONE_TAG##v}`,
		},
	}
}

func cleanupExecStorage(workspaceDir string) step {
	return step{
		Name: "Clean up exec runner storage (post)",
		Environment: map[string]value{
			"WORKSPACE_DIR": {raw: workspaceDir},
		},
		Commands: []string{
			`chmod -R u+rw $WORKSPACE_DIR`,
			`rm -rf $WORKSPACE_DIR/go $WORKSPACE_DIR/.ssh`,
		},
	}
}

// fromEnviron augments environ with values from orig
func fromEnviron(orig map[string]value, environ map[string]value) map[string]value {
	for key, value := range orig {
		environ[key] = value
	}
	return environ
}

var s3Environ = map[string]value{
	"AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
	"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
	"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
	"AWS_REGION":            {raw: "us-west-2"},
}
