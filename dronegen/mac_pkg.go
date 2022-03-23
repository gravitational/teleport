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
)

func darwinPkgPipeline(name, makeTarget string, pkgGlobs []string) pipeline {
	b := buildType{
		arch: "amd64",
		os:   "darwin",
	}
	p := newDarwinPipeline(name)
	p.Trigger = triggerTag
	p.DependsOn = []string{"build-darwin-amd64"}
	p.Steps = []step{
		setUpExecStorageStep(p.Workspace.Path),
		{
			Name: "Check out code",
			Environment: map[string]value{
				"WORKSPACE_DIR":      {raw: p.Workspace.Path},
				"GITHUB_PRIVATE_KEY": {fromSecret: "GITHUB_PRIVATE_KEY"},
			},
			Commands: darwinTagCheckoutCommands(),
		},
		{
			Name: "Download built tarball artifacts from S3",
			Environment: map[string]value{
				"AWS_REGION":            {raw: "us-west-2"},
				"AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
				"GITHUB_PRIVATE_KEY":    {fromSecret: "GITHUB_PRIVATE_KEY"},
				"WORKSPACE_DIR":         {raw: p.Workspace.Path},
			},
			Commands: darwinTagDownloadArtifactCommands(),
		},
		{
			Name: "Build Mac pkg release artifacts",
			Environment: map[string]value{
				"WORKSPACE_DIR":     {raw: p.Workspace.Path},
				"APPLE_USERNAME":    {fromSecret: "APPLE_USERNAME"},
				"APPLE_PASSWORD":    {fromSecret: "APPLE_PASSWORD"},
				"BUILDBOX_PASSWORD": {fromSecret: "BUILDBOX_PASSWORD"},
				"OSS_TARBALL_PATH":  {raw: "/tmp/build-darwin-amd64-pkg/go/artifacts"},
				"ENT_TARBALL_PATH":  {raw: "/tmp/build-darwin-amd64-pkg/go/artifacts"},
				"OS":                {raw: b.os},
				"ARCH":              {raw: b.arch},
			},
			Commands: darwinTagPackageCommands(makeTarget),
		},
		{
			Name: "Copy Mac pkg artifacts",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
			},
			Commands: darwinTagCopyPkgArtifactCommands(pkgGlobs),
		},
		{
			Name: "Upload to S3",
			Environment: map[string]value{
				"AWS_REGION":            {raw: "us-west-2"},
				"AWS_S3_BUCKET":         {fromSecret: "AWS_S3_BUCKET"},
				"AWS_ACCESS_KEY_ID":     {fromSecret: "AWS_ACCESS_KEY_ID"},
				"AWS_SECRET_ACCESS_KEY": {fromSecret: "AWS_SECRET_ACCESS_KEY"},
				"WORKSPACE_DIR":         {raw: p.Workspace.Path},
			},
			Commands: []string{
				`set -u`,
				`cd $WORKSPACE_DIR/go/artifacts`,
				`aws s3 sync . s3://$AWS_S3_BUCKET/teleport/tag/${DRONE_TAG##v}`,
			},
		},
		{
			Name:     "Register artifacts",
			Commands: tagCreateReleaseAssetCommands(b),
			Failure:  "ignore",
			Environment: map[string]value{
				"WORKSPACE_DIR": {raw: p.Workspace.Path},
				"RELEASES_CERT": value{fromSecret: "RELEASES_CERT_STAGING"},
				"RELEASES_KEY":  value{fromSecret: "RELEASES_KEY_STAGING"},
			},
		},
		cleanUpExecStorageStep(p.Workspace.Path),
	}

	return p
}

func darwinTeleportPkgPipeline() pipeline {
	return darwinPkgPipeline("build-darwin-amd64-pkg", "pkg", []string{"build/teleport*.pkg", "e/build/teleport-ent*.pkg"})
}

func darwinTshPkgPipeline() pipeline {
	return darwinPkgPipeline("build-darwin-amd64-pkg-tsh", "pkg-tsh", []string{"build/tsh*.pkg"})
}

func darwinTagDownloadArtifactCommands() []string {
	return []string{
		`set -u`,
		`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
		`export S3_PATH="tag/$${DRONE_TAG##v}/"`,
		`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}teleport-v$${VERSION}-darwin-amd64-bin.tar.gz $WORKSPACE_DIR/go/artifacts/`,
		`aws s3 cp s3://$AWS_S3_BUCKET/teleport/$${S3_PATH}teleport-ent-v$${VERSION}-darwin-amd64-bin.tar.gz $WORKSPACE_DIR/go/artifacts/`,
	}
}

func darwinTagPackageCommands(target string) []string {
	return []string{
		`set -u`,
		`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
		`export VERSION=$(cat $WORKSPACE_DIR/go/.version.txt)`,
		// set HOME explicitly (as Drone overrides it normally)
		`export HOME=/Users/build`,
		// unlock login keychain
		`security unlock-keychain -p $${BUILDBOX_PASSWORD} login.keychain`,
		// show available certificates
		`security find-identity -v`,
		// build pkg, target is `pkg` for teleport, `pkg-tsh` for tsh
		fmt.Sprintf(`make %s OS=$OS ARCH=$ARCH`, target),
	}
}

func darwinTagCopyPkgArtifactCommands(pkgGlobs []string) []string {
	return []string{
		`set -u`,
		`cd $WORKSPACE_DIR/go/src/github.com/gravitational/teleport`,
		// delete temporary tarball artifacts so we don't re-upload them in the next stage
		`rm -rf $WORKSPACE_DIR/go/artifacts/*.tar.gz`,
		// copy release archives to artifact directory
		fmt.Sprintf(`cp %s $WORKSPACE_DIR/go/artifacts/`, strings.Join(pkgGlobs, " ")),
		// generate checksums (for mac)
		`cd $WORKSPACE_DIR/go/artifacts && for FILE in *.pkg; do shasum -a 256 $FILE > $FILE.sha256; done && ls -l`,
	}
}
