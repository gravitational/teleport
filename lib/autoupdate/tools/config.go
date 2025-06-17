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
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/gravitational/trace"
)

const (
	configFileName  = ".config.json"
	configFilePerms = 0o644
	// defaultSizeStoredVersion defines how many versions will be stored in the tools
	// directory. Older versions will be cleaned up based on least recently used.
	defaultSizeStoredVersion = 5
)

var (
	// configSync mutex for managed updates config file.
	configSync sync.Mutex
)

// ClientToolsConfig is configuration structure for client tools managed updates.
type ClientToolsConfig struct {
	// Configs stores information about profile and cluster version and mode:
	// `{"profile-name":{"version": "1.2.3", "disabled":false}}`.
	Configs map[string]*Config `json:"configs"`
	// Tools stores information about tools directories per versions:
	// `[{"tool_name": "tsh", "path": "tool-path"}]`.
	Tools Tools `json:"tools"`
}

// Config stores required version and mode for specific cluster.
type Config struct {
	Version  string `json:"version"`
	Disabled bool   `json:"disabled"`
}

// Tool stores tools path per version, each tool might be stored in different path.
type Tool struct {
	Version string            `json:"version"`
	PathMap map[string]string `json:"path"`
	Package string            `json:"package"`
}

type Tools []Tool

// PickVersion lookups the version and re-order by last recently used.
func (p Tools) PickVersion(version string) *Tool {
	for i, tool := range p {
		if tool.Version == version {
			p[0], p[i] = p[i], p[0]
			return &tool
		}
	}
	return nil
}

// HasVersion check that specific version present in collection.
func (p Tools) HasVersion(version string) bool {
	return slices.ContainsFunc(p, func(s Tool) bool {
		return version == s.Version
	})
}

func (u *Updater) loadConfig(currentProfile string) (*Config, error) {
	if currentProfile == "" {
		return nil, nil
	}

	configSync.Lock()
	defer configSync.Unlock()

	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, trace.NotFound("managed updates config file %q not found", configFileName)
	}
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to read managed updates config file %q", configFileName)
	}

	var clientToolsConfig ClientToolsConfig
	if err := json.Unmarshal(data, &clientToolsConfig); err != nil {
		return nil, trace.Wrap(err)
	}

	if config, ok := clientToolsConfig.Configs[currentProfile]; ok {
		return config, nil
	}

	return nil, trace.NotFound("config for profile %q not found", currentProfile)
}

func (u *Updater) loadTools() (Tools, error) {
	configSync.Lock()
	defer configSync.Unlock()

	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, trace.NotFound("managed updates config file %q not found", configFileName)
	}
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to read managed updates config file %q", configFileName)
	}

	var profileConfigs ClientToolsConfig
	if err := json.Unmarshal(data, &profileConfigs); err != nil {
		return nil, trace.Wrap(err)
	}

	return profileConfigs.Tools, nil
}

func (u *Updater) saveTool(tool Tool) error {
	configSync.Lock()
	defer configSync.Unlock()

	clientToolsConfig := &ClientToolsConfig{
		Configs: make(map[string]*Config),
	}
	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if err == nil {
		if err := json.Unmarshal(data, clientToolsConfig); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}

	if len(clientToolsConfig.Tools) >= defaultSizeStoredVersion {
		clientToolsConfig.Tools = append(Tools{tool}, clientToolsConfig.Tools[:defaultSizeStoredVersion-1]...)
	} else {
		clientToolsConfig.Tools = append(Tools{tool}, clientToolsConfig.Tools...)
	}

	jsonData, err := json.Marshal(clientToolsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.WriteFile(filepath.Join(u.toolsDir, configFileName), jsonData, configFilePerms))
}

func (u *Updater) saveConfig(proxyHost string, config *Config) error {
	configSync.Lock()
	defer configSync.Unlock()

	profileConfigs := &ClientToolsConfig{
		Configs: make(map[string]*Config),
	}
	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if err == nil {
		if err := json.Unmarshal(data, profileConfigs); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}

	profileConfigs.Configs[proxyHost] = config

	jsonData, err := json.Marshal(profileConfigs)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.WriteFile(filepath.Join(u.toolsDir, configFileName), jsonData, configFilePerms))
}
