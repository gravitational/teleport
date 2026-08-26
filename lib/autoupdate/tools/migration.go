/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// migrationFilePerms defines the permissions for files created during the migration process.
	migrationFilePerms = 0o755
)

// migrateV1AndUpdateConfig launches migration process and add migrated
// tools to configuration file.
func migrateV1AndUpdateConfig(toolsDir string, tools []string) error {
	if err := UpdateToolsConfig(toolsDir, func(ctc *ClientToolsConfig) error {
		migratedTools, err := migrateV1(toolsDir, tools)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(migratedTools) == 0 {
			return nil
		}

		for _, tool := range migratedTools {
			ctc.AddTool(toolsDir, tool)
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// migrateV1 verifies the tool binary located in the tool's directory.
// If it is a symlink, it reads the target location and generates a tool object
// to be saved in the configuration for backward compatibility.
// If it is a regular binary, a new package folder should be created,
// and the binary should be copied to the new location.
// TODO(vapopov): DELETE in v21.0.0 - the version without caching will no longer be supported.
func migrateV1(toolsDir string, tools []string) (map[string]Tool, error) {
	newPkg := fmt.Sprint(uuid.New().String(), updatePackageSuffixV2)
	migratedTools := make(map[string]Tool)
	for _, tool := range tools {
		path := filepath.Join(toolsDir, tool)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, trace.Wrap(err, "failed to retrieve information for tool %q", tool)
		}

		toolVersion, err := CheckToolVersion(path)
		if trace.IsBadParameter(err) {
			// If we can't identify toolVersion, it is blocked by EDR software or binary
			// is damaged we should continue migration process.
			slog.ErrorContext(context.Background(), "failed to check the toolVersion", "error", err)
			continue
		} else if err != nil {
			return nil, trace.Wrap(err)
		}

		if info.Mode().Type()&os.ModeSymlink != 0 {
			fullPath, err := os.Readlink(path)
			if err != nil {
				return nil, trace.Wrap(err, "failed to read symlink %q", path)
			}
			pkg, relPath, err := extractPackageName(toolsDir, fullPath)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if err := utils.RecursiveCopy(filepath.Join(toolsDir, pkg), filepath.Join(toolsDir, newPkg), nil); err != nil {
				return nil, trace.Wrap(err)
			}
			if t, ok := migratedTools[toolVersion]; ok {
				t.PathMap[tool] = filepath.Join(newPkg, relPath)
			} else {
				migratedTools[toolVersion] = Tool{
					Version: toolVersion,
					OS:      runtime.GOOS,
					Arch:    runtime.GOARCH,
					PathMap: map[string]string{tool: filepath.Join(newPkg, relPath)},
				}
			}
			continue
		}

		// Create new toolVersion of the package and move tools to new destination.
		if t, ok := migratedTools[toolVersion]; ok {
			newPath := filepath.Join(toolsDir, newPkg, tool)
			if err := utils.CopyFile(path, newPath, migrationFilePerms); err != nil {
				return nil, trace.Wrap(err)
			}
			t.PathMap[tool] = filepath.Join(newPkg, tool)
		} else {
			if err := os.Mkdir(filepath.Join(toolsDir, newPkg), migrationFilePerms); err != nil {
				return nil, trace.Wrap(err)
			}
			newPath := filepath.Join(toolsDir, newPkg, tool)
			if err := utils.CopyFile(path, newPath, migrationFilePerms); err != nil {
				return nil, trace.Wrap(err)
			}
			migratedTools[toolVersion] = Tool{
				Version: toolVersion,
				OS:      runtime.GOOS,
				Arch:    runtime.GOARCH,
				PathMap: map[string]string{tool: filepath.Join(newPkg, tool)},
			}
		}

	}

	return migratedTools, nil
}

func extractPackageName(toolsDir string, fullPath string) (string, string, error) {
	rel, err := filepath.Rel(toolsDir, fullPath)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	dir := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(dir) == 2 && strings.HasSuffix(dir[0], updatePackageSuffix) {
		return dir[0], dir[1], nil
	}

	return "", fullPath, nil
}
