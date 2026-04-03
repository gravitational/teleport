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

package common

import (
	"errors"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows/registry"
)

const (
	// TeleportConnectPoliciesKeyPath is the Windows registry path for Teleport Connect policy settings.
	TeleportConnectPoliciesKeyPath = `SOFTWARE\Policies\Teleport\TeleportConnect`
	// RegistryValueToolsVersion is the policy value name that pins the managed tools version.
	RegistryValueToolsVersion = "ToolsVersion"
	// RegistryValueCDNBaseURL is the policy value name that configures the managed update CDN base URL.
	RegistryValueCDNBaseURL = "CdnBaseUrl"
)

// PolicyValues defines the managed update policy configuration.
type PolicyValues struct {
	// CDNBaseURL is the base URL used to download artifacts.
	CDNBaseURL string
	// Version specifies the enforced application version.
	Version string
}

// ReadRegistryPolicyValues reads system policy values (tools version and CDN base URL) for Teleport Connect.
func ReadRegistryPolicyValues(key registry.Key) (*PolicyValues, error) {
	version, err := ReadRegistryValue(key, TeleportConnectPoliciesKeyPath, RegistryValueToolsVersion)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	url, err := ReadRegistryValue(key, TeleportConnectPoliciesKeyPath, RegistryValueCDNBaseURL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	return &PolicyValues{
		CDNBaseURL: url,
		Version:    version,
	}, nil
}

// ReadRegistryValue reads a registry value.
func ReadRegistryValue(hive registry.Key, pathName string, valueName string) (path string, err error) {
	key, err := registry.OpenKey(hive, pathName, registry.READ)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", trace.NotFound("registry key %s not found", pathName)
		}
		return "", trace.Wrap(err, "opening registry key %s", pathName)
	}

	defer func() {
		if closeErr := key.Close(); closeErr != nil && err == nil {
			err = trace.Wrap(closeErr, "closing registry key %s", pathName)
		}
	}()

	path, _, err = key.GetStringValue(valueName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", trace.NotFound("registry value %s not found in %s", valueName, pathName)
		}
		return "", trace.Wrap(err, "reading registry value %s from %s", valueName, pathName)
	}

	return path, nil
}
