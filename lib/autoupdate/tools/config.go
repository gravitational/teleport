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
	"sync"

	"github.com/gravitational/trace"
)

const (
	configFileName  = ".config.json"
	configFilePerms = 0o644
)

var (
	// configSync mutex for managed updates config file.
	configSync sync.Mutex
)

// ProfileConfigs is configuration structure for client tools managed updates.
type ProfileConfigs struct {
	// Configs holds information about profile and cluster version and mode
	// `{"profile-proxy":{"version": "1.2.3", "disabled":false}}`.
	Configs map[string]*Config `json:"configs"`
	// Versions holds information about directory per version
	// `{"version": "directory"}`.
	Versions map[string]map[string]string `json:"versions"`
}

// Config stores required version and mode for specific cluster.
type Config struct {
	Version  string `json:"version"`
	Disabled bool   `json:"disabled"`
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

	var profileConfigs ProfileConfigs
	if err := json.Unmarshal(data, &profileConfigs); err != nil {
		return nil, trace.Wrap(err)
	}

	if config, ok := profileConfigs.Configs[currentProfile]; ok {
		return config, nil
	}

	return nil, trace.NotFound("config for profile %q not found", currentProfile)
}

func (u *Updater) loadToolsMap(version string) (map[string]string, error) {
	configSync.Lock()
	defer configSync.Unlock()

	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, trace.NotFound("managed updates config file %q not found", configFileName)
	}
	if err != nil {
		return nil, trace.WrapWithMessage(err, "failed to read managed updates config file %q", configFileName)
	}

	var profileConfigs ProfileConfigs
	if err := json.Unmarshal(data, &profileConfigs); err != nil {
		return nil, trace.Wrap(err)
	}

	return profileConfigs.Versions[version], nil
}

func (u *Updater) SaveToolsMap(version string, toolsMap map[string]string) error {
	configSync.Lock()
	defer configSync.Unlock()

	profileConfigs := &ProfileConfigs{
		Configs:  make(map[string]*Config),
		Versions: make(map[string]map[string]string),
	}
	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if err == nil {
		if err := json.Unmarshal(data, profileConfigs); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}

	profileConfigs.Versions[version] = toolsMap

	jsonData, err := json.Marshal(profileConfigs)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.WriteFile(filepath.Join(u.toolsDir, configFileName), jsonData, configFilePerms))
}

func (u *Updater) SaveConfig(currentProfile string, config *Config) error {
	configSync.Lock()
	defer configSync.Unlock()

	profileConfigs := &ProfileConfigs{
		Configs:  make(map[string]*Config),
		Versions: make(map[string]map[string]string),
	}
	data, err := os.ReadFile(filepath.Join(u.toolsDir, configFileName))
	if err == nil {
		if err := json.Unmarshal(data, profileConfigs); err != nil {
			return trace.Wrap(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return trace.Wrap(err)
	}

	profileConfigs.Configs[currentProfile] = config

	jsonData, err := json.Marshal(profileConfigs)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(os.WriteFile(filepath.Join(u.toolsDir, configFileName), jsonData, configFilePerms))
}
