package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport.e/tooling/plugins/internal/terraform/modules"
	"github.com/gravitational/teleport.e/tooling/plugins/internal/terraform/registry"
)

func main() {
	args := parseCommandLine()
	ctx := context.Background()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel(args)})))

	localRegistry, err := setupRegistryDirectory(args.registryDirectoryPath)
	if err != nil {
		slog.ErrorContext(ctx, "Failed setting up registry file tree", "error", err)
		os.Exit(1)
	}

	artifacts, err := getArtifacts(ctx, args.artifactDirectoryPath, args.variant)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to list artifacts directory", "path", args.artifactDirectoryPath, "error", err)
		os.Exit(1)
	}

	if args.publishModules {
		if err := handleModules(ctx, localRegistry, artifacts.modules); err != nil {
			os.Exit(1)
		}
	} else {
		if err := handleProviders(ctx, args, localRegistry, artifacts.providers); err != nil {
			os.Exit(1)
		}
	}
}

func logLevel(args *args) slog.Leveler {
	var level slog.LevelVar

	switch {
	case args.verbosity >= 2:
		level.Set(slog.LevelDebug - 4)
	case args.verbosity == 1:
		level.Set(slog.LevelDebug)
	default:
		level.Set(slog.LevelInfo)
	}

	return &level
}

type registryPaths struct {
	providerRegistryDir string
	objectStoreDir      string
	moduleRegistryDir   string
}

func setupRegistryDirectory(registryDirectoryPath string) (*registryPaths, error) {
	// Ensure that the working dir and so on exist
	registryDir := filepath.Join(registryDirectoryPath, "registry")
	if err := os.MkdirAll(registryDir, 0700); err != nil {
		return nil, trace.Wrap(err, "failed ensuring registry output dir %s exists", registryDir)
	}

	objectStoreDir := filepath.Join(registryDirectoryPath, "store")
	if err := os.MkdirAll(objectStoreDir, 0700); err != nil {
		return nil, trace.Wrap(err, "failed ensuring registry output dir %s exists", objectStoreDir)
	}

	moduleRegistryDir := filepath.Join(registryDirectoryPath, "modules", "v1")
	if err := os.MkdirAll(moduleRegistryDir, 0700); err != nil {
		return nil, trace.Wrap(err, "failed ensuring registry output dir %s exists", moduleRegistryDir)
	}

	return &registryPaths{
		providerRegistryDir: registryDir,
		objectStoreDir:      objectStoreDir,
		moduleRegistryDir:   moduleRegistryDir,
	}, nil
}

type artifactPaths struct {
	providers []string
	modules   []string
}

// Gets all the files in the provided path that are Terraform provider artifacts.
func getArtifacts(ctx context.Context, artifactDirectoryPath string, variant string) (*artifactPaths, error) {
	fsObjects, err := os.ReadDir(artifactDirectoryPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list file in artifact directory %q", artifactDirectoryPath)
	}

	var out artifactPaths
	for _, fsObject := range fsObjects {
		fsObjectPath := filepath.Join(artifactDirectoryPath, fsObject.Name())

		switch {
		case !fsObject.Type().IsRegular():
			slog.DebugContext(ctx, "Skipping non-regular file fs object", "path", fsObjectPath)
			continue
		case registry.IsProviderTarball(fsObjectPath, variant):
			out.providers = append(out.providers, fsObjectPath)
		case modules.IsModuleTarball(fsObjectPath):
			out.modules = append(out.modules, fsObjectPath)
		default:
			slog.DebugContext(ctx, "Skipping unrecognized artifact file", "path", fsObjectPath)
			continue
		}
	}

	return &out, nil
}
