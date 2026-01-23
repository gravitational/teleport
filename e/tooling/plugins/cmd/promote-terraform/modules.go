package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport.e/tooling/plugins/internal/terraform/modules"
)

func handleModules(ctx context.Context, localRegistry *registryPaths, moduleFiles []string) error {
	moduleInfos := make([]*modules.FileInfo, 0, len(moduleFiles))
	for _, m := range moduleFiles {
		info, err := modules.ParseFilePath(m)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to parse module file name", "artifact", m)
			return trace.Wrap(err)
		}
		moduleInfos = append(moduleInfos, info)
	}

	moduleStoreDir := filepath.Join(localRegistry.objectStoreDir, "modules")
	if err := repackModules(ctx, moduleInfos, moduleStoreDir); err != nil {
		slog.ErrorContext(ctx, "Failed repacking module artifacts", "error", err)
		return trace.Wrap(err)
	}

	if err := updateModuleRegistry(ctx, localRegistry, moduleInfos); err != nil {
		slog.ErrorContext(ctx, "Failed updating module registry", "error", err)
		return trace.Wrap(err)
	}

	return nil
}

// repackModules organizes and renames modules into a tree structure for upload to S3.
// The subdirectory tree name is based off of the file name as follows:
// Input:  "$srcDir/terraform-module_<namespace>_<name>_<system>_v<version>.tar.gz"
// Output: "$dstDir/modules/:namespace/:name/:system/:version.tar.gz
func repackModules(ctx context.Context, moduleInfos []*modules.FileInfo, dstDir string) error {
	for _, m := range moduleInfos {
		slog.InfoContext(ctx, "Repacking module tarball", "artifact", m.Path)

		newPath := modules.ModuleFilePath(dstDir, m.Namespace, m.Name, m.System, m.Version)
		if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
			return trace.Wrap(err, "failed ensuring output dir for %s exists", newPath)
		}
		if err := os.Rename(m.Path, newPath); err != nil {
			return trace.Wrap(err, "failed to rename file %s to %s", m.Path, newPath)
		}
		slog.InfoContext(ctx, "Module tarball repacked", "new_path", newPath)
	}
	return nil
}

// updateModuleRegistry updates the versions files required by module registry
// protocol for each Terraform module.
// This corresponds with versions request path:
// GET /modules/v1/:namespace/:name/:system/versions
func updateModuleRegistry(ctx context.Context, localRegistry *registryPaths, moduleInfos []*modules.FileInfo) error {
	for _, m := range moduleInfos {
		versionsPath := getModuleVersionsFilePath(localRegistry.moduleRegistryDir, m)
		logger := slog.With("versions", versionsPath)
		versions, err := modules.LoadModulesVersionFile(versionsPath)
		if err != nil {
			if !trace.IsNotFound(err) {
				return trace.Wrap(err, "failed to load versions file")
			}
			logger.InfoContext(ctx, "No versions index found, using empty versions index")
			versions = &modules.Versions{}
		} else {
			logger.InfoContext(ctx, "Loaded existing versions file")
		}

		versions.Append(modules.Version{Version: m.Version})
		if err := versions.Save(versionsPath); err != nil {
			return trace.Wrap(err, "failed to save versions file")
		}
		logger.InfoContext(ctx, "Updated module registry versions file")
	}
	return nil
}

func getModuleVersionsFilePath(registryDir string, info *modules.FileInfo) string {
	return modules.VersionsFilePath(registryDir, info.Namespace, info.Name, info.System)
}
