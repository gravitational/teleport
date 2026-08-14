package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport.e/tooling/plugins/internal/terraform/registry"
)

func handleProviders(ctx context.Context, args *args, localRegistry *registryPaths, providerFiles []string) error {
	signingEntity, err := loadSigningEntity(ctx, args.signingKeyText)
	if err != nil {
		slog.ErrorContext(ctx, "Failed decoding signing key", "error", err)
		os.Exit(1)
	}

	objectStoreUrl := args.registryURL + "store/"

	versionRecord, newFiles, err := repackProviders(ctx, providerFiles, localRegistry, objectStoreUrl, signingEntity, args.protocolVersions, args.providerNamespace, args.providerName)
	if err != nil {
		slog.ErrorContext(ctx, "Failed repacking provider artifacts", "error", err)
		return trace.Wrap(err)
	}

	err = updateRegistry(ctx, localRegistry, args.providerNamespace, args.providerName, versionRecord, newFiles)
	if err != nil {
		slog.ErrorContext(ctx, "Failed updating provider registry", "error", err)
		return trace.Wrap(err)
	}

	return nil
}

func loadSigningEntity(ctx context.Context, keyText string) (*openpgp.Entity, error) {
	slog.InfoContext(ctx, "Decoding signing key")

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

func repackProviders(
	ctx context.Context,
	providerArtifacts []string,
	localRegistry *registryPaths,
	objectStoreUrl string,
	signingEntity *openpgp.Entity,
	protocolVersions []string,
	providerNamespace,
	providerName string,
) (registry.Version, []string, error) {
	versionRecord := registry.Version{
		Protocols: protocolVersions,
	}

	newFiles := []string{}
	unsetVersion := semver.Version{}

	// buffer to hold the content for the master SHA256SUMS file
	var masterSums bytes.Buffer
	var masterSumPath, masterSigPath string

	for _, providerArtifact := range providerArtifacts {
		slog.InfoContext(ctx, "Found provider tarball", "artifact", providerArtifact)

		registryInfo, err := registry.RepackProvider(ctx, localRegistry.objectStoreDir, providerArtifact, signingEntity)
		if err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "failed repacking provider")
		}

		slog.InfoContext(ctx, "Provider repacked", "path", registryInfo.Zip)
		newFiles = append(newFiles, registryInfo.Zip)

		// version handling
		if versionRecord.Version == unsetVersion {
			versionRecord.Version = registryInfo.Version

			// Standard Format: terraform-provider-NAME_VERSION_SHA256SUMS
			masterSumName := fmt.Sprintf("terraform-provider-%s_%s_SHA256SUMS", providerName, registryInfo.Version)
			masterSigName := masterSumName + ".sig"

			masterSumPath = filepath.Join(localRegistry.objectStoreDir, masterSumName)
			masterSigPath = filepath.Join(localRegistry.objectStoreDir, masterSigName)

		} else if !versionRecord.Version.Equal(registryInfo.Version) {
			return registry.Version{}, nil, trace.Wrap(err, "version mismatch. Expected %s, got %s", versionRecord.Version, registryInfo.Version)
		}

		// Append this artifact's hash to our Master Buffer
		// Format: "checksum  filename" (Standard sha256sum output)
		if _, err := fmt.Fprintf(&masterSums, "%s  %s\n", registryInfo.Sha256String(), filepath.Base(registryInfo.Zip)); err != nil {
			return registry.Version{}, []string{}, trace.Wrap(err, "failed writing master sum")
		}

		// Overwrite the Sum/Sig paths so download.json points to the master files
		registryInfo.Sum = masterSumPath
		registryInfo.Sig = masterSigPath

		downloadInfo, err := registry.NewDownloadFromRepackResult(registryInfo, protocolVersions, objectStoreUrl)
		if err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "failed creating download info record")
		}

		filename, err := downloadInfo.Save(localRegistry.providerRegistryDir, providerNamespace, providerName, registryInfo.Version)
		if err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "Failed saving download info record")
		}
		newFiles = append(newFiles, filename)

		versionRecord.Platforms = append(versionRecord.Platforms, registry.Platform{
			OS:   registryInfo.OS,
			Arch: registryInfo.Arch,
		})
	}

	// write the Master SHA256SUMS file
	if masterSumPath != "" {
		if err := writeMasterFiles(ctx, masterSumPath, masterSigPath, &masterSums, signingEntity); err != nil {
			return registry.Version{}, nil, trace.Wrap(err, "failed writing master files")
		}

		newFiles = append(newFiles, masterSumPath, masterSigPath)
	}

	return versionRecord, newFiles, nil
}

func getVersionsFilePath(registryDir, namespace, provider string) string {
	return registry.VersionsFilePath(registryDir, namespace, provider)
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
	versionsFilePath := getVersionsFilePath(workspace.providerRegistryDir, namespace, provider)

	// Check if the versions file path exists. If not, warn and treat this as a
	// new registry with an empty index.
	versions := registry.Versions{}
	versionsFileStat, err := os.Stat(versionsFilePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return trace.Wrap(err, "failed to stat the version file at %q")
		}

		slog.WarnContext(ctx, "No index found, using empty index", "path", versionsFilePath)
	} else {
		if !versionsFileStat.Mode().Type().IsRegular() {
			return trace.Errorf("the versions fs object at %q is not a regular file", versionsFilePath)
		}

		versions, err = registry.LoadVersionsFile(versionsFilePath)
		if err != nil {
			return trace.Wrap(err, "failed to load versions file from %q", versionsFilePath)
		}

		slog.InfoContext(ctx, "Loaded versions file", "path", versionsFilePath)
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
	semvers := slices.Collect(maps.Keys(versionIndex))
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

func writeMasterFiles(
	ctx context.Context,
	sumPath string,
	sigPath string,
	sums io.Reader,
	signingEntity *openpgp.Entity,
) error {
	slog.InfoContext(ctx, "Writing Master SHA256SUMS", "path", sumPath)

	fSum, err := os.Create(sumPath)
	if err != nil {
		return trace.Wrap(err, "failed creating master sum file")
	}
	defer closeWithLog(ctx, fSum, "master sum file", sumPath)

	fSig, err := os.Create(sigPath)
	if err != nil {
		return trace.Wrap(err, "failed creating master sig file")
	}
	defer closeWithLog(ctx, fSig, "master sig file", sigPath)

	return registry.WriteMasterManifest(ctx, sums, signingEntity, fSum, fSig)
}

func closeWithLog(ctx context.Context, f *os.File, label, path string) {
	if err := f.Close(); err != nil {
		slog.ErrorContext(ctx, "Failed to close file", "path", path, "err", trace.DebugReport(err))
	}
}
