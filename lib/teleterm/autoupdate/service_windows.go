// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package autoupdate

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/registry"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/auto_update/v1"
	"github.com/gravitational/teleport/lib/teleterm/autoupdate/common"
)

const (
	// Defined in electron-builder-config.js
	teleportConnectGUID          = "22539266-67e8-54a3-83b9-dfdca7b33ee1"
	teleportConnectKeyPath       = `SOFTWARE\` + teleportConnectGUID
	registryValueInstallLocation = "InstallLocation"
)

// GetInstallationMetadata returns installation metadata of the currently running app instance.
func (s *Service) GetInstallationMetadata(_ context.Context, _ *api.GetInstallationMetadataRequest) (*api.GetInstallationMetadataResponse, error) {
	perMachine, err := isPerMachineInstall()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetInstallationMetadataResponse{IsPerMachineInstall: perMachine}, nil
}

// platformGetConfig retrieves the local auto updates configuration.
func platformGetConfig() (*api.GetConfigResponse, error) {
	perMachine, err := isPerMachineInstall()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	machineValues, err := common.ReadRegistryPolicyValues(registry.LOCAL_MACHINE)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &api.GetConfigResponse{
		CdnBaseUrl: &api.ConfigValue{
			Value:  machineValues.CDNBaseURL,
			Source: api.ConfigSource_CONFIG_SOURCE_POLICY,
		},
		ToolsVersion: &api.ConfigValue{
			Value:  machineValues.Version,
			Source: api.ConfigSource_CONFIG_SOURCE_POLICY,
		},
	}

	// If per-machine config is fully set, there's no need to check other sources.
	perMachineConfigFullySet := machineValues.CDNBaseURL != "" && machineValues.Version != ""
	if perMachineConfigFullySet {
		return config, nil
	}

	if !perMachine {
		userValues, err := common.ReadRegistryPolicyValues(registry.CURRENT_USER)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if machineValues.CDNBaseURL == "" {
			config.CdnBaseUrl.Value = userValues.CDNBaseURL
		}

		if machineValues.Version == "" {
			config.ToolsVersion.Value = userValues.Version
		}
	}

	// Read deprecated env vars. If they are set and the app is installed per-machine, updates must use
	// the standard UAC installer (no privileged updater).
	envVarConfig, err := readConfigFromEnvVars()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.CdnBaseUrl.Value == "" {
		config.CdnBaseUrl = envVarConfig.GetCdnBaseUrl()
	}

	if config.ToolsVersion.Value == "" {
		config.ToolsVersion = envVarConfig.GetToolsVersion()
	}

	return config, nil
}

func isPerMachineInstall() (bool, error) {
	perMachineLocation, err := common.ReadRegistryValue(registry.LOCAL_MACHINE, teleportConnectKeyPath, registryValueInstallLocation)
	if err != nil {
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return false, trace.Wrap(err)
	}

	// tsh is placed in <installation directory>/resources/bin/tsh.exe.
	exePathInPerMachineLocation := filepath.Join(perMachineLocation, "resources", "bin", "tsh.exe")

	return exePath == exePathInPerMachineLocation, nil
}
