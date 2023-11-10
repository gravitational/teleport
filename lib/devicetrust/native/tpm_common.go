//go:build linux || windows

// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package native

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

const (
	deviceStateFolderName        = ".teleport-device"
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
		if err := os.Mkdir(deviceStateDirPath, 0700); err != nil {
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
