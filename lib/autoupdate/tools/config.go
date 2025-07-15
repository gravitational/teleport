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
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// configFileVersion identifies the version of the configuration file
	// might be used for future migrations
	configFileVersion = "v1"
	// lockFileName is file used for locking update process in parallel.
	lockFileName = ".lock"
	// configFileName is the configuration file used to store versions for known hosts
	// and installed client tool versions.
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
func (ctc *ClientToolsConfig) AddTool(tool Tool) {
	for _, t := range ctc.Tools {
		if t.Version == tool.Version {
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
func (ctc *ClientToolsConfig) SelectVersion(version string) *Tool {
	for i, tool := range ctc.Tools {
		if tool.Version == version {
			ctc.Tools = append([]Tool{tool}, append(ctc.Tools[:i], ctc.Tools[i+1:]...)...)
			return &tool
		}
	}
	return nil
}

// HasVersion check that specific version present in collection.
func (ctc *ClientToolsConfig) HasVersion(version string) bool {
	return slices.ContainsFunc(ctc.Tools, func(s Tool) bool {
		return version == s.Version
	})
}

// ClusterConfig stores required version and mode for specific cluster.
type ClusterConfig struct {
	Version  string `json:"version"`
	Disabled bool   `json:"disabled"`
}

// Tool stores tools path per version, each tool might be stored in different path.
type Tool struct {
	Version string            `json:"version"`
	PathMap map[string]string `json:"path"`
}

// PackageNames returns the package names extracted from the tool path map.
func (c *Tool) PackageNames() []string {
	var packageNames []string
	for _, path := range c.PathMap {
		dir := strings.SplitN(path, string(filepath.Separator), 2)
		if len(dir) > 0 {
			packageNames = append(packageNames, dir[0])
		}
	}
	return packageNames
}

// newClientToolsConfig creates or opens the configuration file for client tools managed updates,
// and acquires a filesystem lock until the configuration is written and closed.
func newClientToolsConfig(toolsDir string) (ctc *ClientToolsConfig, save func() error, err error) {
	unlock, err := utils.FSWriteLock(filepath.Join(toolsDir, lockFileName))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			err = trace.NewAggregate(err, unlock())
		}
	}()

	ctc = &ClientToolsConfig{
		Version: configFileVersion,
		Configs: make(map[string]*ClusterConfig),
	}
	data, err := os.ReadFile(filepath.Join(toolsDir, configFileName))
	if err == nil {
		if err := json.Unmarshal(data, ctc); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, trace.Wrap(err)
	}

	return ctc, func() error {
		jsonData, err := json.Marshal(ctc)
		if err != nil {
			return trace.NewAggregate(err, unlock())
		}
		return trace.NewAggregate(
			os.WriteFile(filepath.Join(toolsDir, configFileName), jsonData, configFilePerms),
			unlock(),
		)
	}, nil
}
