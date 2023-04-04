package main

import "time"

// TODO(camh): Remove this comment when the code is pruned.
// These pipelines replace the drone ones with calls to GitHub Actions. The
// pipelines in mac.go and mac_pkg.go will be removed entirely once these
// pipelines are shown to be reliable.
//
// The GitHub Actions pipeline does not split the build and packaging of
// the various packages and images. One pipeline does it all without using
// intermediate storage on S3. This means if some steps of the package/image
// build fails (likely notarization as that is an external service run by
// Apple), the whole pipeline needs to be re-run. If such failures end up
// being common, we may want to split the pipelines again. But it would
// probably be better value to implement caching, as then the rebuild would
// be almost instant.

// darwinTagPipelineGHA returns a pipeline that kicks off a tagged build of
// the Mac (darwin) release assets on GitHub Actions. The action builds:
// * a tarball of signed teleport binaries (teleport, tsh, tctl, tbot).
// * a package with the Teleport binaries (teleport, tsh, tctl, tbot).
// * a package with the tsh binary.
// * a disk image (dmg) of Teleport Connect containing the signed tsh package.
// These build assets are signed and notarized.
func darwinTagPipelineGHA() pipeline {
	bt := ghaBuildType{
		buildType:    buildType{os: "darwin", arch: "amd64"},
		trigger:      triggerTag,
		pipelineName: "build-darwin-amd64",
		ghaWorkflow:  "release-mac-amd64.yaml",
		srcRefVar:    "DRONE_TAG",
		workflowRef:  "${DRONE_TAG}",
		timeout:      60 * time.Minute,
		slackOnError: true,
		inputs: []string{
			"release-artifacts=true",
			"build-packages=true",
		},
	}
	return ghaBuildPipeline(bt)
}

// darwinPushPipelineGHA returns a pipeline that kicks off a push build of the
// teleport binaries and the teleport connect dmg. The binaries are signed and
// notarized even though we do not release these assets. This tests that the
// signing and notarization process continues to work so we don't wait until
// release time to discover breakage.
func darwinPushPipelineGHA() pipeline {
	bt := ghaBuildType{
		buildType:    buildType{os: "darwin", arch: "amd64"},
		trigger:      triggerPush,
		pipelineName: "push-build-darwin-amd64",
		ghaWorkflow:  "release-mac-amd64.yaml",
		srcRefVar:    "DRONE_COMMIT",
		workflowRef:  "${DRONE_BRANCH}",
		timeout:      60 * time.Minute,
		slackOnError: true,
		inputs: []string{
			"release-artifacts=false",
			"build-packages=false",
		},
	}
	return ghaBuildPipeline(bt)
}
