package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	// The golang docs are wrong/out of date for this package. Check github instead.
	"github.com/cavaliergopher/rpm"
	"github.com/gravitational/trace"
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
	slog.InfoContext(context.Background(), "Starting YUM repo build process")
	slog.DebugContext(context.Background(), "Using providing configuration", "config", yrt.config)

	isFirstRun, err := yrt.isFirstRun()
	if err != nil {
		return trace.Wrap(err, "failed to determine if YUM repos have been built before")
	}

	if isFirstRun {
		slog.WarnContext(context.Background(), "First run or disaster recovery detected, attempting to rebuild existing repos from YUM repository")

		err = yrt.s3Manager.DownloadExistingRepo()
		if err != nil {
			return trace.Wrap(err, "failed to sync existing repo from S3 bucket")
		}

		// Additional first time setup can be done here, but shouldn't be needed
	} else {
		slog.DebugContext(context.Background(), "Not first run of tool, skipping S3 resync")
	}

	// Both Hashicorp and Docker publish their key to this path
	relativeGpgPublicKeyPath := "gpg"
	err = yrt.gpg.WritePublicKeyToFile(filepath.Join(yrt.config.localBucketPath, relativeGpgPublicKeyPath))
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

	slog.InfoContext(context.Background(), "YUM repo build process completed", "build_duration", time.Since(start).Round(time.Millisecond))
	return nil
}

func (yrt *YumRepoTool) isFirstRun() (bool, error) {
	yumSyncPath := yrt.config.localBucketPath
	slog.DebugContext(context.Background(), "Checking if bucket exists", "bucket", yumSyncPath)

	files, err := os.ReadDir(yumSyncPath)
	if err != nil {
		return false, trace.Wrap(err, "failed to list files in %q", yumSyncPath)
	}

	slog.DebugContext(context.Background(), "Found files in bucket", "matching_count", len(files), "bucket", yumSyncPath)
	return len(files) == 0, nil
}

func (yrt *YumRepoTool) getSourceArtifactPaths() ([]string, error) {
	artifactPath := yrt.config.artifactPath
	slog.InfoContext(context.Background(), "Looking for source artifacts", "path", artifactPath)

	fileDirEntries, err := os.ReadDir(artifactPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list files in %q", artifactPath)
	}

	slog.InfoContext(context.Background(), "Found possible artifacts", "found_count", len(fileDirEntries), "path", artifactPath)

	// This allocates a capacity of the maximum that is possibly needed, but it is probably
	// better than reallocating the underlying array by appending each time
	validArtifactPaths := make([]string, 0, len(fileDirEntries))
	for _, fileDirEntry := range fileDirEntries {
		fileName := fileDirEntry.Name()
		if filepath.Ext(fileName) != ArtifactExtension {
			slog.DebugContext(context.Background(), "Skipping file with invalid extension", "file", fileName, "expected", ArtifactExtension, "actual", filepath.Ext(fileName))
			continue
		}

		filePath := filepath.Join(artifactPath, fileName)
		validArtifactPaths = append(validArtifactPaths, filePath)
		slog.DebugContext(context.Background(), "Found artifact", "path", filePath)
	}

	slog.InfoContext(context.Background(), "Found artifacts", "artifact_count", len(validArtifactPaths), "path", validArtifactPaths)
	return validArtifactPaths, nil
}

func sortArtifactsByArch(artifactPaths []string) (map[string][]string, error) {
	slog.InfoContext(context.Background(), "Determining ISA of targeted artifacts")

	// Four is probably a decent guess for the number of ISAs we build for. This would cover:
	// i386, x86_64, arm, arm64
	archPackageMap := make(map[string][]string, 4)
	for _, artifactPath := range artifactPaths {
		slog.DebugContext(context.Background(), "Attempting to open RPM", "path", artifactPath)
		rpmPackage, err := rpm.Open(artifactPath)
		if err != nil {
			return nil, trace.Wrap(err, "failed to read package %q", artifactPath)
		}

		arch := rpmPackage.Architecture()
		baseArch, err := getBaseArchForArch(arch)
		if err != nil {
			return nil, trace.Wrap(err, "failed to determine base architecture for artifact %q", artifactPath)
		}

		slog.DebugContext(context.Background(), "Found matching artifact", "path", artifactPath, "isa", arch, "base_isa", baseArch)
		if rpmPackagePaths, ok := archPackageMap[baseArch]; ok {
			archPackageMap[baseArch] = append(rpmPackagePaths, artifactPath)
		} else {
			archPackageMap[baseArch] = []string{artifactPath}
		}
	}

	slog.InfoContext(context.Background(), "Found ISAs", "isa_count", len(archPackageMap), "isas", archPackageMap)

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
	slog.InfoContext(context.Background(), "Adding artifacts to repos")

	archs, err := sortArtifactsByArch(bucketArtifactPaths)
	if err != nil {
		return trace.Wrap(err, "failed to get artifacts by architecture")
	}

	repoCount := 0
	for os, osVersions := range yrt.supportedOSs {
		osPath := filepath.Join(yrt.config.localBucketPath, os)
		for _, osVersion := range osVersions {
			for arch, packages := range archs {
				relativeRepoPath := filepath.Join(
					osVersion,
					"Teleport",
					arch,
					yrt.config.releaseChannel,
					yrt.config.versionChannel,
				)
				repoPath := filepath.Join(osPath, relativeRepoPath)

				err := yrt.updateRepoWithArtifacts(packages, repoPath)
				if err != nil {
					return trace.Wrap(err, "failed to add artifact for YUM repo %q", relativeRepoPath)
				}

				err = yrt.createRepoFiles(repoPath, os, osVersion, arch, relativeGpgPublicKeyPath)
				if err != nil {
					return trace.Wrap(err, "failed to create repo files")
				}

				repoCount++
			}
		}
	}

	slog.InfoContext(context.Background(), "Updated repos with artifacts", "repo_count", repoCount, "artifact_count", len(bucketArtifactPaths))
	return nil
}

func (yrt *YumRepoTool) createRepoFiles(repoPath, os, osVersion, arch, relativeGpgPublicKeyPath string) error {
	repoFiles := map[string][]string{
		"yum": []string{
			"teleport.repo",
			"teleport-yum.repo",
		},
		"zypper": []string{
			"teleport-zypper.repo",
		},
	}

	for subdomain, filesNames := range repoFiles {
		for _, fileName := range filesNames {
			repoFilePath := filepath.Join(repoPath, fileName)
			repoDomain := fmt.Sprintf("%s.%s", subdomain, yrt.config.domainName)
			err := yrt.createRepoFile(repoFilePath, repoDomain, os, osVersion, arch, relativeGpgPublicKeyPath)
			if err != nil {
				return trace.Wrap(err, "failed to create repo file for os %q at %q", os, repoFilePath)
			}
		}
	}

	return nil
}

func (yrt *YumRepoTool) updateRepoWithArtifacts(packagePaths []string, repoPath string) error {
	slog.InfoContext(context.Background(), "Updating repo at with packages", "repo", repoPath, "packages", packagePaths)

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

	slog.InfoContext(context.Background(), "Finished updating repo", "repo", repoPath)
	return nil
}

func (yrt *YumRepoTool) copyArtifactsToRepo(artifactPaths []string, repoPath string) error {
	// The "repo_rpms" directory here is arbitrary and not tied to anything else.
	repoArtifactFolder := filepath.Join(repoPath, "repo_rpms")

	_, err := copyArtifacts(artifactPaths, repoArtifactFolder, false)
	if err != nil {
		return trace.Wrap(err, "failed to copy artifacts %d artifacts to repo directory at %s", len(artifactPaths), repoArtifactFolder)
	}

	return nil
}

// Flattens artifactPaths into one directory and returns the created files in that directory
func (yrt *YumRepoTool) copyArtifactsToBucket(artifactPaths []string, bucketArtifactSubdirectory string) ([]string, error) {
	bucketArtifactFolder := filepath.Join(yrt.config.localBucketPath, bucketArtifactSubdirectory)

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
	slog.DebugContext(context.Background(), "Copying artifacts", "artifact_count", len(artifactPaths), "destination", destinationDirectory, "copy_type", copyType)

	err := os.MkdirAll(destinationDirectory, 0770)
	if err != nil {
		return nil, trace.Wrap(err, "failed to ensure destination directory %q exists", destinationDirectory)
	}

	destinationArtifactPaths := make([]string, len(artifactPaths))
	for i, artifactPath := range artifactPaths {
		artifactDestinationPath := filepath.Join(destinationDirectory, filepath.Base(artifactPath))
		if shouldHardCopy {
			hardCopyFile(artifactPath, artifactDestinationPath)
		} else {
			softCopyFile(artifactPath, artifactDestinationPath)
		}
		destinationArtifactPaths[i] = artifactDestinationPath
	}

	slog.DebugContext(context.Background(), "Successfully copied artifact(s)", "artifact_count", len(destinationArtifactPaths), "destination", destinationDirectory)
	return destinationArtifactPaths, nil
}

func (yrt *YumRepoTool) updateRepoMetadata(repoPath string) error {
	// Ensure the directory exists
	err := os.MkdirAll(repoPath, 0770)
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
	repomdPath := filepath.Join(repoPath, "repodata", "repomd.xml")
	err := yrt.gpg.SignFile(repomdPath)
	if err != nil {
		return trace.Wrap(err, "failed to sign file %q", repomdPath)
	}

	return nil
}

// Creates an os-specific ".repo" file for yum-config-manager akin to
// https://rpm.releases.teleport.dev/teleport.repo
func (yrt *YumRepoTool) createRepoFile(filePath, domainName, osName, osVersion, arch, relativeGpgPublicKeyPath string) error {
	// Future work: maybe move domain name to config?
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
	err := os.WriteFile(filePath, []byte(repoFileContent), 0770)
	if err != nil {
		return trace.Wrap(err, "failed to create repo file at %q", filePath)
	}

	slog.InfoContext(context.Background(), "Created repo file", "path", filePath)

	return nil
}

// Guaranteed to perform a "copy" operation rather than just linking. Should only be used
// where linking is not acceptable.
func hardCopyFile(src, dest string) error {
	// Implementation is a modified version of method 1 from
	// https://opensource.com/article/18/6/copying-files-go
	start := time.Now()
	slog.DebugContext(context.Background(), "Beginning hard file copy", "source", src, "destination", dest)

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

	slog.DebugContext(context.Background(), "File transfer completed", "source", src, "destination", dest, "copy_duration", time.Since(start).Round(time.Millisecond))
	return nil
}

// Copies or links the src file to dest path. The implementation of this function is subject
// to change. If a guaranteed is needed then `hardCopyFile` should be used instead.
func softCopyFile(src, dest string) error {
	// Profiling has shown that disk reads/writes are a significant bottleneck with the
	// APT side of the tool. This will reduce roughly 25GB of read/writes to nearly 0.
	start := time.Now()
	slog.DebugContext(context.Background(), "Beginning soft file copy", "source", src, "destination", dest)

	err := os.Symlink(src, dest)
	if err != nil {
		return trace.Wrap(err, "failed to link %q to %q", src, dest)
	}

	slog.DebugContext(context.Background(), "File transfer completed", "source", src, "destination", dest, "copy_duration", time.Since(start).Round(time.Nanosecond))
	return nil
}
