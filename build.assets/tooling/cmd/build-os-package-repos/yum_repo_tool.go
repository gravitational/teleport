/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	// The golang docs are wrong/out of date for this package. Check github instead.
	"github.com/cavaliergopher/rpm"
	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type YumRepoTool struct {
	config       *YumConfig
	s3Manager    *S3manager
	createRepo   *CreateRepo
	gpg          *GPG
	supportedOSs map[string][]string
}

const ArtifactExtension string = ".rpm"

// Instantiates a new yum repo tool instance and performs any required setup/config.
func NewYumRepoTool(config *YumConfig, supportedOSs map[string][]string) (*YumRepoTool, error) {
	cr, err := NewCreateRepo(config.cacheDir)
	if err != nil {
		trace.Wrap(err, "failed to instantiate new CreateRepo instance")
	}

	s3Manager, err := NewS3Manager(config.S3Config)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new s3manager instance")
	}

	gpg, err := NewGPG()
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a new GPG instance")
	}

	return &YumRepoTool{
		config:       config,
		s3Manager:    s3Manager,
		createRepo:   cr,
		gpg:          gpg,
		supportedOSs: supportedOSs,
	}, nil
}

func (yrt *YumRepoTool) Run() error {
	start := time.Now()
	logrus.Infoln("Starting YUM repo build process...")
	logrus.Debugf("Using config: %+v", spew.Sdump(yrt.config))

	isFirstRun, err := yrt.isFirstRun()
	if err != nil {
		return trace.Wrap(err, "failed to determine if YUM repos have been built before")
	}

	if isFirstRun {
		logrus.Warningln("First run or disaster recovery detected, attempting to rebuild existing repos from YUM repository...")

		err = yrt.s3Manager.DownloadExistingRepo()
		if err != nil {
			return trace.Wrap(err, "failed to sync existing repo from S3 bucket")
		}

		// Additional first time setup can be done here, but shouldn't be needed
	} else {
		logrus.Debugf("Not first run of tool, skipping S3 resync")
	}

	// Both Hashicorp and Docker publish their key to this path
	relativeGpgPublicKeyPath := "gpg"
	err = yrt.gpg.WritePublicKeyToFile(path.Join(yrt.config.localBucketPath, relativeGpgPublicKeyPath))
	if err != nil {
		return trace.Wrap(err, "failed to write GPG public key")
	}

	sourceArtifactPaths, err := yrt.getSourceArtifactPaths()
	if err != nil {
		return trace.Wrap(err, "failed to get the file paths of available RPM artifacts")
	}

	// This can be anywhere under repoPath. Hardcoding it rather than putting it in config as it should not change
	// between runs/versions.
	relativeBucketArtifactPath := "RPMs"
	bucketArtifactPaths, err := yrt.copyArtifactsToBucket(sourceArtifactPaths, relativeBucketArtifactPath)
	if err != nil {
		return trace.Wrap(err, "failed to transfer available RPM artifacts to a bucket subdirectory")
	}

	err = yrt.addArtifacts(bucketArtifactPaths, relativeGpgPublicKeyPath)
	if err != nil {
		return trace.Wrap(err, "failed to add artifacts")
	}

	err = yrt.s3Manager.UploadBuiltRepoWithRedirects(ArtifactExtension, relativeBucketArtifactPath)
	if err != nil {
		return trace.Wrap(err, "failed to sync changes to S3 bucket")
	}

	// Future work: add literals to config?
	err = yrt.s3Manager.UploadRedirectURL("index.html", "https://goteleport.com/docs/installation/#linux")
	if err != nil {
		return trace.Wrap(err, "failed to redirect index page to Teleport docs")
	}

	logrus.Infof("YUM repo build process completed in %s", time.Since(start).Round(time.Millisecond))
	return nil
}

func (yrt *YumRepoTool) isFirstRun() (bool, error) {
	yumSyncPath := yrt.config.localBucketPath
	logrus.Debugf("Checking if %q exists...", yumSyncPath)

	files, err := os.ReadDir(yumSyncPath)
	if err != nil {
		return false, trace.Wrap(err, "failed to list files in %q", yumSyncPath)
	}

	logrus.Debugf("Found %d files in %q:", len(files), yumSyncPath)
	for _, file := range files {
		logrus.Debug(file.Name())
	}

	return len(files) == 0, nil
}

func (yrt *YumRepoTool) getSourceArtifactPaths() ([]string, error) {
	artifactPath := yrt.config.artifactPath
	logrus.Infof("Looking for artifacts in %q...", artifactPath)

	fileDirEntries, err := os.ReadDir(artifactPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list files in %q", artifactPath)
	}

	logrus.Infof("Found %d possible artifacts in %q", len(fileDirEntries), artifactPath)

	// This allocates a capacity of the maximum that is possibly needed, but it is probably
	// better than reallocating the underlying array by appending each time
	validArtifactPaths := make([]string, 0, len(fileDirEntries))
	for _, fileDirEntry := range fileDirEntries {
		fileName := fileDirEntry.Name()
		if path.Ext(fileName) != ArtifactExtension {
			logrus.Debugf("The file %q does not have a %q extension, skipping...", fileName, ArtifactExtension)
			continue
		}

		filePath := path.Join(artifactPath, fileName)
		validArtifactPaths = append(validArtifactPaths, filePath)
		logrus.Debugf("Found artifact %q", filePath)
	}

	logrus.Infof("Found %d artifacts", len(validArtifactPaths))
	logrus.Debugf("Source artifact paths: %v", validArtifactPaths)
	return validArtifactPaths, nil
}

func sortArtifactsByArch(artifactPaths []string) (map[string][]string, error) {
	logrus.Info("Determining ISA of targeted artifacts...")

	// Four is probably a decent guess for the number of ISAs we build for. This would cover:
	// i386, x86_64, arm, arm64
	archPackageMap := make(map[string][]string, 4)
	for _, artifactPath := range artifactPaths {
		logrus.Debugf("Attempting to open RPM %q...", artifactPath)
		rpmPackage, err := rpm.Open(artifactPath)
		if err != nil {
			return nil, trace.Wrap(err, "failed to read package %q", artifactPath)
		}

		arch := rpmPackage.Architecture()
		baseArch, err := getBaseArchForArch(arch)
		if err != nil {
			return nil, trace.Wrap(err, "failed to determine base architecture for artifact %q", artifactPath)
		}

		logrus.Debugf("Found %q with ISA %q and base ISA %q", artifactPath, arch, baseArch)
		if rpmPackagePaths, ok := archPackageMap[baseArch]; ok {
			archPackageMap[baseArch] = append(rpmPackagePaths, artifactPath)
		} else {
			archPackageMap[baseArch] = []string{artifactPath}
		}
	}

	logrus.Infof("Found %d ISAs: %v", len(archPackageMap), archPackageMap)

	return archPackageMap, nil
}

// Implementation pulled from https://github.com/rpm-software-management/yum/blob/master/rpmUtils/arch.py#L429
func getBaseArchForArch(arch string) (string, error) {
	archTypes := map[string][]string{
		"i386": {
			"athlon",
			"geode",
			"i686",
			"i586",
			"i486",
			"i386",
		},
		"x86_64": {
			"amd64",
			"ia32e",
			"x86_64",
		},
		// This does not cover ARMv8 and above which have several strange corner cases
		"arm": {
			"armv2",
			"armv3",
			"armv4",
			"armv5",
			"armv6",
			"armv7",
			"arm",
		},
		"aarch64": {
			"arm64",
			"aarch64",
		},
	}

	for baseArch, archTypes := range archTypes {
		for _, archType := range archTypes {
			if strings.HasPrefix(arch, archType) {
				return baseArch, nil
			}
		}
	}

	return "", trace.Errorf("failed to determine base arch for architecture %q", arch)
}

func (yrt *YumRepoTool) addArtifacts(bucketArtifactPaths []string, relativeGpgPublicKeyPath string) error {
	logrus.Info("Adding artifacts to repos...")

	archs, err := sortArtifactsByArch(bucketArtifactPaths)
	if err != nil {
		return trace.Wrap(err, "failed to get artifacts by architecture")
	}

	repoCount := 0
	for os, osVersions := range yrt.supportedOSs {
		osPath := path.Join(yrt.config.localBucketPath, os)
		for _, osVersion := range osVersions {
			for arch, packages := range archs {
				relativeRepoPath := path.Join(
					osVersion,
					"Teleport",
					arch,
					yrt.config.releaseChannel,
					yrt.config.versionChannel,
				)
				repoPath := path.Join(osPath, relativeRepoPath)

				err := yrt.updateRepoWithArtifacts(packages, repoPath)
				if err != nil {
					return trace.Wrap(err, "failed to add artifact for YUM repo %q", relativeRepoPath)
				}

				repoFilePath := filepath.Join(repoPath, "teleport.repo")
				err = yrt.createRepoFile(repoFilePath, os, osVersion, arch, relativeGpgPublicKeyPath)
				if err != nil {
					return trace.Wrap(err, "failed to create repo file for os %q at %q", os, repoFilePath)
				}

				repoCount++
			}
		}
	}

	logrus.Infof("Updated %d repos with %d artifacts", repoCount, len(bucketArtifactPaths))
	return nil
}

func (yrt *YumRepoTool) updateRepoWithArtifacts(packagePaths []string, repoPath string) error {
	logrus.Infof("Updating repo at %q with packages %v", repoPath, packagePaths)

	// A soft copy here will have a significant performance impact, and S3 sync will follow links
	err := yrt.copyArtifactsToRepo(packagePaths, repoPath)
	if err != nil {
		return trace.Wrap(err, "failed to copy artifacts to repo %q", repoPath)
	}

	err = yrt.updateRepoMetadata(repoPath)
	if err != nil {
		return trace.Wrap(err, "failed to update repo %q metadata", repoPath)
	}

	err = yrt.signRepoMetadata(repoPath)
	if err != nil {
		return trace.Wrap(err, "failed to sign repo %q metadata", repoPath)
	}

	logrus.Infof("Finished updating repo %q", repoPath)
	return nil
}

func (yrt *YumRepoTool) copyArtifactsToRepo(artifactPaths []string, repoPath string) error {
	// The "repo_rpms" directory here is arbitrary and not tied to anything else.
	repoArtifactFolder := path.Join(repoPath, "repo_rpms")

	_, err := copyArtifacts(artifactPaths, repoArtifactFolder, false)
	if err != nil {
		return trace.Wrap(err, "failed to copy artifacts %d artifacts to repo directory at %s", len(artifactPaths), repoArtifactFolder)
	}

	return nil
}

// Flattens artifactPaths into one directory and returns the created files in that directory
func (yrt *YumRepoTool) copyArtifactsToBucket(artifactPaths []string, bucketArtifactSubdirectory string) ([]string, error) {
	bucketArtifactFolder := path.Join(yrt.config.localBucketPath, bucketArtifactSubdirectory)

	// A "hard" copy is performed here because the bucket will usually be stored on a non-ephemeral filesystem path.
	// If the artifacts are linked rather than copied then every time the uploaded bucket is synced on future runs
	// the sync will re-download the real artifacts.
	destinationArtifactPaths, err := copyArtifacts(artifactPaths, bucketArtifactFolder, true)
	if err != nil {
		return nil, trace.Wrap(err, "failed to copy artifacts %d artifacts to bucket directory at %s", len(artifactPaths), destinationArtifactPaths)
	}

	return destinationArtifactPaths, nil
}

func copyArtifacts(artifactPaths []string, destinationDirectory string, shouldHardCopy bool) ([]string, error) {
	copyType := "soft"
	if shouldHardCopy {
		copyType = "hard"
	}
	logrus.Debugf("Copying %d artifacts to %q via a %s copy...", len(artifactPaths), destinationDirectory, copyType)

	err := os.MkdirAll(destinationDirectory, 0660)
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure destination directory %q exists", destinationDirectory)
	}

	destinationArtifactPaths := make([]string, len(artifactPaths))
	for i, artifactPath := range artifactPaths {
		artifactDestinationPath := path.Join(destinationDirectory, path.Base(artifactPath))
		if shouldHardCopy {
			hardCopyFile(artifactPath, artifactDestinationPath)
		} else {
			softCopyFile(artifactPath, artifactDestinationPath)
		}
		destinationArtifactPaths[i] = artifactDestinationPath
	}

	logrus.Debugf("Successfully copied %d artifact(s) to %q", len(destinationArtifactPaths), destinationDirectory)
	return destinationArtifactPaths, nil
}

func (yrt *YumRepoTool) updateRepoMetadata(repoPath string) error {
	// Ensure the directory exists
	err := os.MkdirAll(repoPath, 0660)
	if err != nil {
		return trace.Wrap(err, "failed to ensure repo directory %q exists", repoPath)
	}

	err = yrt.createRepo.CreateOrUpdateRepo(repoPath)
	if err != nil {
		return trace.Wrap(err, "failed to update repo metadata for %q", repoPath)
	}

	return nil
}

func (yrt *YumRepoTool) signRepoMetadata(repoPath string) error {
	repomdPath := path.Join(repoPath, "repodata", "repomd.xml")
	err := yrt.gpg.SignFile(repomdPath)
	if err != nil {
		return trace.Wrap(err, "failed to sign file %q", repomdPath)
	}

	return nil
}

// Creates an os-specific ".repo" file for yum-config-manager akin to
// https://rpm.releases.teleport.dev/teleport.repo
func (yrt *YumRepoTool) createRepoFile(filePath, osName, osVersion, arch, relativeGpgPublicKeyPath string) error {
	// Future work: maybe move domain name to config?
	domainName := "yum.releases.teleport.dev"
	sectionName := "teleport"
	// See these for config details:
	// https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/deployment_guide/sec-configuring_yum_and_yum_repositories
	// https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/deployment_guide/sec-using_yum_variables
	repoOptions := map[string]string{
		"name": "Gravitational Teleport packages",
		"baseurl": (&url.URL{
			Scheme: "https",
			Host:   domainName,
			Path: strings.Join(
				[]string{
					osName,
					osVersion,
					"Teleport",
					arch,
					yrt.config.releaseChannel,
					yrt.config.versionChannel,
				},
				"/",
			),
		}).String(),
		"enabled":       "1",
		"gpgcheck":      "1",
		"repo_gpgcheck": "1",
		"gpgkey": (&url.URL{
			Scheme: "https",
			Host:   domainName,
			Path:   relativeGpgPublicKeyPath,
		}).String(),
	}

	// + 2 = repo header line, new line
	repoFileLines := make([]string, 0, len(repoOptions)+2)
	repoFileLines = append(repoFileLines, fmt.Sprintf("[%s]", sectionName))
	for key, value := range repoOptions {
		repoFileLines = append(repoFileLines, fmt.Sprintf("%s=%s", key, value))
	}
	repoFileLines = append(repoFileLines, "")

	repoFileContent := strings.Join(repoFileLines, "\n")
	err := os.WriteFile(filePath, []byte(repoFileContent), 0660)
	if err != nil {
		return trace.Wrap(err, "failed to create repo file at %q", filePath)
	}

	logrus.Infof("Created repo file at %q", filePath)
	logrus.Debugf("Repo file contents:\n%s", repoFileContent)

	return nil
}

// Guaranteed to perform a "copy" operation rather than just linking. Should only be used
// where linking is not acceptable.
func hardCopyFile(src, dest string) error {
	// Implementation is a modified version of method 1 from
	// https://opensource.com/article/18/6/copying-files-go
	start := time.Now()
	logrus.Debugf("Beginning hard file copy from %q to %q...", src, dest)

	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return trace.Wrap(err, "failed to get filesystem data for %q", src)
	}

	if !sourceFileStat.Mode().IsRegular() {
		return trace.Errorf("Source file %q is not a regular file and cannot be copied", src)
	}

	sourceHandle, err := os.Open(src)
	if err != nil {
		return trace.Wrap(err, "failed to open source file %q for reading", src)
	}
	defer sourceHandle.Close()

	destinationHandle, err := os.Create(dest)
	if err != nil {
		return trace.Wrap(err, "failed to open destination file %q for writing", dest)
	}
	defer destinationHandle.Close()

	_, err = io.Copy(destinationHandle, sourceHandle)
	if err != nil {
		return trace.Wrap(err, "failed to copy source file %q to destination file %q", src, dest)
	}

	logrus.Debugf("File transfer from %q to %q completed in %s", src, dest, time.Since(start).Round(time.Millisecond))
	return nil
}

// Copies or links the src file to dest path. The implementation of this function is subject
// to change. If a guaranteed is needed then `hardCopyFile` should be used instead.
func softCopyFile(src, dest string) error {
	// Profiling has shown that disk reads/writes are a significant bottleneck with the
	// APT side of the tool. This will reduce roughly 25GB of read/writes to nearly 0.
	start := time.Now()
	logrus.Debugf("Beginning soft file copy from %q to %q...", src, dest)

	err := os.Symlink(src, dest)
	if err != nil {
		return trace.Wrap(err, "failed to link %q to %q", src, dest)
	}

	logrus.Debugf("File transfer from %q to %q completed in %s", src, dest, time.Since(start).Round(time.Nanosecond))
	return nil
}
