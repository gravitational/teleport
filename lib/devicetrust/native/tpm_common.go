//go:build linux || windows

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package native

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// `const deviceStateFolderName string` declared separately for each platform.

const (
	attestationKeyFileName       = "attestation.key"
	credentialActivationFileName = "credential-activation"
	dmiJsonFileName              = "dmi.json"
)

type deviceState struct {
	attestationKeyPath       string
	credentialActivationPath string
	dmiJSONPath              string
}

// userDirFunc is used to determine where to save/lookup the device's
// attestation key.
// We use os.UserCacheDir instead of os.UserConfigDir because the latter is
// roaming (which we don't want for device-specific keys).
var userDirFunc = os.UserCacheDir

// setupDeviceStateDir ensures that device state directory exists.
// It returns a struct containing the path of each part of the device state,
// or nil and an error if it was not possible to set up the directory.
func setupDeviceStateDir(getBaseDir func() (string, error)) (*deviceState, error) {
	base, err := getBaseDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deviceStateDirPath := filepath.Join(base, deviceStateFolderName)
	ds := &deviceState{
		attestationKeyPath:       filepath.Join(deviceStateDirPath, attestationKeyFileName),
		credentialActivationPath: filepath.Join(deviceStateDirPath, credentialActivationFileName),
		dmiJSONPath:              filepath.Join(deviceStateDirPath, dmiJsonFileName),
	}

	switch _, err := os.Stat(deviceStateDirPath); {
	case os.IsNotExist(err):
		// If it doesn't exist, we can create it and return as we know
		// the perms are correct as we created it.
		if err := os.MkdirAll(deviceStateDirPath, 0700); err != nil {
			return nil, trace.Wrap(err)
		}
	case err != nil:
		return nil, trace.Wrap(err)
	}

	return ds, nil
}

func firstValidAssetTag(assetTags ...string) string {
	for _, assetTag := range assetTags {
		// Skip empty serials and values with spaces on them.
		//
		// There are many variations of "no value set" used by manufacturers, but
		// looking for a space in the string catches most of them. For example:
		// "Default string", "No Asset Information", "Not Specified", etc.
		if assetTag == "" || strings.Contains(assetTag, " ") {
			continue
		}
		return assetTag
	}
	return ""
}
