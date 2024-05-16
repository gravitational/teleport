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
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport-plugins/tooling/internal/terraform/registry"
)

func main() {
	args := parseCommandLine()

	localRegistry, err := setupRegistryDirectory(args.registryDirectoryPath)
	if err != nil {
		log.WithError(err).Fatalf("Failed setting up registry file tree")
	}

	log.StandardLogger().SetLevel(logLevel(args))

	signingEntity, err := loadSigningEntity(args.signingKeyText)
	if err != nil {
		log.WithError(err).Fatalf("Failed decoding signing key")
	}

	files, err := getArtifactFiles(args.artifactDirectoryPath)
	if err != nil {
		log.WithError(err).Fatalf("failed to list artifacts in %q", args.artifactDirectoryPath)
	}

	objectStoreUrl := args.registryURL + "store/"

	versionRecord, newFiles, err := repackProviders(files, localRegistry, objectStoreUrl, signingEntity, args.protocolVersions, args.providerNamespace, args.providerName)
	if err != nil {
		log.WithError(err).Fatalf("Failed repacking artifacts")
	}

	err = updateRegistry(context.Background(), localRegistry, args.providerNamespace, args.providerName, versionRecord, newFiles)
	if err != nil {
		log.WithError(err).Fatal("Failed updating registry")
	}
}

func logLevel(args *args) log.Level {
	switch {
	case args.verbosity >= 2:
		return log.TraceLevel

	case args.verbosity == 1:
		return log.DebugLevel

	default:
		return log.InfoLevel
	}
}

// updateRegistry fetches the live registry and adds our new providers to it.
// It's possible for another process to update the `versions` index in the
// bucket while we are modifying it here, and unfortunately AWS doesn't give
// us a nice way to prevent this simply with S3.
//
// We could layer a locking mechanism on top of another AWS service, but for
// now we are relying on GHA to honour its concurrency limits (i.e. 1) to
// serialise access to the live `versions` file.
func updateRegistry(ctx context.Context, workspace *registryPaths, namespace, provider string, newVersion registry.Version, files []string) error {
	versionsFilePath := getVersionsFilePath(workspace.registryDir, namespace, provider)

	// Check if the versions file path exists. If not, warn and treat this as a
	// new registry with an empty index.
	versions := registry.Versions{}
	versionsFileStat, err := os.Stat(versionsFilePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return trace.Wrap(err, "failed to stat the version file at %q")
		}

		log.Warnf("No index found at %q. Using empty index.", versionsFilePath)
	} else {
		if !versionsFileStat.Mode().Type().IsRegular() {
			return trace.Errorf("the versions fs object at %q is not a regular file", versionsFilePath)
		}

		versions, err = registry.LoadVersionsFile(versionsFilePath)
		if err != nil {
			return trace.Wrap(err, "failed to load versions file from %q", versionsFilePath)
		}

		log.Infof("Loaded versions file from %q.", versionsFilePath)
	}

	// Index the available version by their semver version, so that we can find the
	// appropriate release if we're overwriting an existing version
	versionIndex := map[semver.Version]registry.Version{}
	for _, v := range versions.Versions {
		versionIndex[v.Version] = v
	}

	// add/overwrite the existing version entry
	versionIndex[newVersion.Version] = newVersion
	versions.Versions = flattenVersionIndex(versionIndex)

	if err = versions.Save(versionsFilePath); err != nil {
		return trace.Wrap(err, "failed saving index file")
	}

	return nil
}

func flattenVersionIndex(versionIndex map[semver.Version]registry.Version) []registry.Version {
	// We want to output a list of semvers with semver ordering, so first we
	// generate a sorted list of semvers
	semvers := maps.Keys(versionIndex)
	semverPtrs := make([]*semver.Version, 0, len(semvers)) // Pointer array is required by the sort function

	for i := range semvers {
		semverPtrs = append(semverPtrs, &semvers[i])
	}
	semver.Sort(semverPtrs)

	// Now we can simply walk the index using the sorted key list and we have
	// our sorted output list
	providerVersions := make([]registry.Version, 0, len(semvers))
	for _, semverPtr := range semverPtrs {
		providerVersions = append(providerVersions, versionIndex[*semverPtr])
	}

	return providerVersions
}

func repackProviders(providerArtifacts []string, localRegistry *registryPaths, objectStoreUrl string, signingEntity *openpgp.Entity, protocolVersions []string, providerNamespace, providerName string) (registry.Version, []string, error) {
	versionRecord := registry.Version{
		Protocols: protocolVersions,
	}

	newFiles := []string{}
	unsetVersion := semver.Version{}

	for _, providerArtifact := range providerArtifacts {
		log.Infof("Found provider tarball %s", providerArtifact)

		registryInfo, err := registry.RepackProvider(localRegistry.objectStoreDir, providerArtifact, signingEntity)
		if err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "failed repacking provider")
		}

		log.Infof("Provider repacked to %s", registryInfo.Zip)
		newFiles = append(newFiles, registryInfo.Zip, registryInfo.Sum, registryInfo.Sig)

		if versionRecord.Version == unsetVersion {
			versionRecord.Version = registryInfo.Version
		} else if !versionRecord.Version.Equal(registryInfo.Version) {
			return registry.Version{}, nil, trace.Wrap(err, "version mismatch. Expected %s, got %s", versionRecord.Version, registryInfo.Version)
		}

		downloadInfo, err := registry.NewDownloadFromRepackResult(registryInfo, protocolVersions, objectStoreUrl)
		if err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "failed creating download info record")
		}

		filename, err := downloadInfo.Save(localRegistry.registryDir, providerNamespace, providerName, registryInfo.Version)
		if err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "Failed saving download info record")
		}
		newFiles = append(newFiles, filename)

		versionRecord.Platforms = append(versionRecord.Platforms, registry.Platform{
			OS:   registryInfo.OS,
			Arch: registryInfo.Arch,
		})
	}

	return versionRecord, newFiles, nil
}

func getVersionsFilePath(registryDir, namespace, provider string) string {
	return registry.VersionsFilePath(registryDir, namespace, provider)
}

type registryPaths struct {
	registryDir    string
	objectStoreDir string
}

func setupRegistryDirectory(registryDirectoryPath string) (*registryPaths, error) {
	// Ensure that the working dir and so on exist
	stagingDir := filepath.Join(registryDirectoryPath, "staging")
	err := os.MkdirAll(stagingDir, 0700)
	if err != nil {
		return nil, trace.Wrap(err, "failed ensuring staging dir %s exists", stagingDir)
	}

	registryDir := filepath.Join(registryDirectoryPath, "registry")
	err = os.MkdirAll(registryDir, 0700)
	if err != nil {
		return nil, trace.Wrap(err, "failed ensuring registry output dir %s exists", registryDir)
	}

	objectStoreDir := filepath.Join(registryDirectoryPath, "store")
	err = os.MkdirAll(objectStoreDir, 0700)
	if err != nil {
		return nil, trace.Wrap(err, "failed ensuring registry output dir %s exists", objectStoreDir)
	}

	return &registryPaths{
		registryDir:    registryDir,
		objectStoreDir: objectStoreDir,
	}, nil
}

// Gets all the files in the provided path that are Terraform provider artifacts.
func getArtifactFiles(artifactDirectoryPath string) ([]string, error) {
	fsObjects, err := os.ReadDir(artifactDirectoryPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list file in artifact directory %q", artifactDirectoryPath)
	}

	filePaths := make([]string, 0, len(fsObjects))
	for _, fsObject := range fsObjects {
		fsObjectPath := filepath.Join(artifactDirectoryPath, fsObject.Name())
		if !fsObject.Type().IsRegular() {
			log.Debugf("Skipping non-regular file fs object %q", fsObjectPath)
			continue
		}

		if !registry.IsProviderTarball(fsObjectPath) {
			log.Debugf("Skipping Terraform provider file %q", fsObjectPath)
			continue
		}

		filePaths = append(filePaths, fsObjectPath)
	}

	return filePaths, nil
}

func loadSigningEntity(keyText string) (*openpgp.Entity, error) {
	log.Info("Decoding signing key")

	block, err := armor.Decode(strings.NewReader(keyText))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entity, err := openpgp.ReadEntity(packet.NewReader(block.Body))
	if err != nil {
		return nil, trace.Wrap(err, "failed loading entity from private key")
	}

	return entity, nil
}
