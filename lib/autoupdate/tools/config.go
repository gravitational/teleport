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
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// configFileVersion identifies the version of the configuration file
	// might be used for future migrations.
	configFileVersion = "v1"
	// lockFileName is file used for locking update process in parallel.
	lockFileName = ".lock"
	// configFileName is the configuration file used to store versions for known hosts
	// and the installed versions of client tools.
	configFileName  = ".config.json"
	configFilePerms = 0o644
	// defaultSizeStoredVersion defines how many versions will be stored in the tools
	// directory. Older versions will be cleaned up based on least recently used.
	defaultSizeStoredVersion = 3
)

// ClientToolsConfig is configuration structure for client tools managed updates.
type ClientToolsConfig struct {
	// Version determines version of configuration file (to support future extensions).
	Version string `json:"version"`
	// Configs stores information about profile and cluster version and mode:
	// `{"profile-name":{"version": "1.2.3", "disabled":false}}`.
	Configs map[string]*ClusterConfig `json:"configs"`
	// Tools stores information about tools directories per versions:
	// `[{"tool_name": "tsh", "path": "tool-path", "version": "tool-version"}]`.
	Tools []Tool `json:"tools"`
	// MaxTools defines the maximum number of tools allowed in the tools directory.
	// Any tools exceeding this limit will be removed during the next installation.
	MaxTools int `json:"max_tools"`
}

// AddTool adds a tool to the collection in the configuration, always placing it at the top.
// The collection size is limited by the `defaultSizeStoredVersion` constant.
func (ctc *ClientToolsConfig) AddTool(toolsDir string, tool Tool) {
	for _, t := range ctc.Tools {
		if t.IsEqual(toolsDir, tool.Version, tool.OS, tool.Arch) {
			maps.Copy(t.PathMap, tool.PathMap)
			return
		}
	}
	if ctc.MaxTools <= 0 {
		ctc.MaxTools = defaultSizeStoredVersion
	}
	if len(ctc.Tools) >= ctc.MaxTools {
		ctc.Tools = append([]Tool{tool}, ctc.Tools[:ctc.MaxTools-1]...)
	} else {
		ctc.Tools = append([]Tool{tool}, ctc.Tools...)
	}
}

// SetConfig sets the version and mode flag for a specific host.
func (ctc *ClientToolsConfig) SetConfig(proxy string, version string, disabled bool) {
	if config, ok := ctc.Configs[proxy]; ok {
		config.Disabled = disabled
		config.Version = version
	} else {
		ctc.Configs[proxy] = &ClusterConfig{Version: version, Disabled: disabled}
	}
}

// SelectVersion lookups the version and re-order by last recently used.
func (ctc *ClientToolsConfig) SelectVersion(toolsDir, version, os, arch string) *Tool {
	for i, tool := range ctc.Tools {
		if tool.IsEqual(toolsDir, version, os, arch) {
			ctc.Tools = append([]Tool{tool}, append(ctc.Tools[:i], ctc.Tools[i+1:]...)...)
			return &tool
		}
	}
	return nil
}

// HasVersion check that specific version present in collection.
func (ctc *ClientToolsConfig) HasVersion(toolsDir, version, os, arch string) bool {
	return slices.ContainsFunc(ctc.Tools, func(tool Tool) bool {
		return tool.IsEqual(toolsDir, version, os, arch)
	})
}

// ClusterConfig stores required version and mode for specific cluster.
type ClusterConfig struct {
	Version  string `json:"version"`
	Disabled bool   `json:"disabled"`
}

// Tool stores tools path per version, each tool might be stored in different path.
type Tool struct {
	// Version is the version of the tools (tsh, tctl) as defined in the PathMap.
	Version string `json:"version"`
	// OS is the operating system of the installed package.
	OS string `json:"os"`
	// Arch is architecture of the installed package.
	Arch string `json:"arch"`
	// PathMap stores the relative path (within the tools directory) for each tool binary.
	// For example: {"tctl": "package-id/tctl"}.
	PathMap map[string]string `json:"path"`
}

// PackageNames returns the package names extracted from the tool path map.
func (t *Tool) PackageNames() []string {
	var packageNames []string
	for _, path := range t.PathMap {
		dir := strings.SplitN(path, string(filepath.Separator), 2)
		if len(dir) > 0 {
			packageNames = append(packageNames, dir[0])
		}
	}
	return packageNames
}

// IsEqual verifies that specific tool matches version, operating system and architecture.
func (t *Tool) IsEqual(toolsDir, version, os, arch string) bool {
	// If OS version is not defined in configuration we should check it by binary headers.
	// TODO(vapopov): DELETE IN v21.0.0 - OS and Arch must be supported for all version.
	if t.OS == "" {
		path, ok := t.PathMap[DefaultClientTools()[0]]
		if !ok {
			return false
		}
		info, err := buildinfo.ReadFile(filepath.Join(toolsDir, path))
		if err != nil {
			slog.WarnContext(context.Background(), "Failed to read build info.", "error", err)
			return version == t.Version
		}
		var binOS, binArch string
		for _, s := range info.Settings {
			switch s.Key {
			case "GOOS":
				binOS = s.Value
			case "GOARCH":
				binArch = s.Value
			}
		}
		if binOS == "" || binArch == "" {
			return version == t.Version
		}
		// macOS binaries are always built with universal architecture support (arm64, amd64).
		if runtime.GOOS == constants.DarwinOS {
			return version == t.Version && os == binOS
		}
		return version == t.Version && os == binOS && arch == binArch
	}

	return version == t.Version && os == t.OS && arch == t.Arch
}

// GetToolsConfig reads the configuration file for client tools managed updates,
// and acquires a filesystem lock until the configuration is read and deserialized.
func GetToolsConfig(toolsDir string) (ctc *ClientToolsConfig, err error) {
	unlock, err := utils.FSWriteLock(filepath.Join(toolsDir, lockFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, unlock())
	}()

	ctc = &ClientToolsConfig{
		Configs: make(map[string]*ClusterConfig),
	}
	data, err := os.ReadFile(filepath.Join(toolsDir, configFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, trace.Wrap(err)
	}
	if data != nil {
		if err := json.Unmarshal(data, ctc); err != nil {
			// If the configuration file content is corrupted, tools execution should not fail.
			// Instead, we should proceed and re-install the required version.
			slog.WarnContext(context.Background(), "failed to unmarshal config file", "error", err)
		}
	}

	return ctc, nil
}

// UpdateToolsConfig creates or opens the configuration file for client tools managed updates,
// and acquires a filesystem lock until the configuration is written and closed.
func UpdateToolsConfig(toolsDir string, update func(ctc *ClientToolsConfig) error) (err error) {
	unlock, err := utils.FSWriteLock(filepath.Join(toolsDir, lockFileName))
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err = trace.NewAggregate(err, unlock())
	}()

	ctc := &ClientToolsConfig{
		Version: configFileVersion,
		Configs: make(map[string]*ClusterConfig),
	}
	data, err := os.ReadFile(filepath.Join(toolsDir, configFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}
	if data != nil {
		if err := json.Unmarshal(data, ctc); err != nil {
			// If the configuration file content is corrupted, tools execution should not fail.
			// Instead, we should proceed and re-install the required version.
			slog.WarnContext(context.Background(), "failed to unmarshal config file", "error", err)
		}
	}

	// Perform update values before configuration file is going to be written.
	if err := update(ctc); err != nil {
		return trace.Wrap(err)
	}

	jsonData, err := json.Marshal(ctc)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(
		os.WriteFile(filepath.Join(toolsDir, configFileName), jsonData, configFilePerms),
	)
}
