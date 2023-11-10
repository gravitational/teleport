// Copyright 2022 Gravitational, Inc
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/linux"
)

// deviceStateFolderName starts without a "." on Linux systems.
const deviceStateFolderName = "teleport-device"

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func signChallenge(chal []byte) (sig []byte, err error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func solveTPMEnrollChallenge(
	_ *devicepb.TPMEnrollChallenge,
	_ bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func solveTPMAuthnDeviceChallenge(
	_ *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func handleTPMActivateCredential(_, _ string) error {
	return devicetrust.ErrPlatformNotSupported
}

func collectDeviceData(mode CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	osRelease, err := linux.ParseOSRelease()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dmiInfo, err := linux.DMIInfoFromSysfs()
	if err != nil {
		log.WithError(err).Warn("TPM: Failed to read device model and/or serial numbers")
	}
	if errors.Is(err, fs.ErrPermission) {
		switch mode {
		case CollectedDataNeverEscalate, CollectedDataMaybeEscalate:
			log.Debug("TPM: Reading cached DMI info")

			dmiCached, err := readDMIInfoCached()
			if err == nil {
				dmiInfo = dmiCached
				break // from switch
			}

			log.WithError(err).Debug("TPM: Failed to read cached DMI info")
			if mode == CollectedDataNeverEscalate {
				break // from switch
			}

			fallthrough

		case CollectedDataAlwaysEscalate:
			log.Debug("TPM: Running escalated `tsh device dmi-info`")

			dmiInfo, err = readDMIInfoEscalated()
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	// dmiInfo is expected to never be nil, but code defensively just in case.
	var modelIdentifier, reportedAssetTag, systemSerialNumber, baseBoardSerialNumber string
	if dmiInfo != nil {
		modelIdentifier = dmiInfo.ProductName
		reportedAssetTag = dmiInfo.ChassisAssetTag
		systemSerialNumber = dmiInfo.ProductSerial
		baseBoardSerialNumber = dmiInfo.BoardSerial
	}

	u, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.DeviceCollectedData{
		CollectTime:     timestamppb.Now(),
		OsType:          devicepb.OSType_OS_TYPE_LINUX,
		SerialNumber:    firstValidAssetTag(reportedAssetTag, systemSerialNumber, baseBoardSerialNumber),
		ModelIdentifier: modelIdentifier,
		// TODO(codingllama): Write os_id for Linux devices.
		OsVersion:             osRelease.VersionID,
		OsBuild:               osRelease.Version,
		OsUsername:            u.Name,
		ReportedAssetTag:      reportedAssetTag,
		SystemSerialNumber:    systemSerialNumber,
		BaseBoardSerialNumber: baseBoardSerialNumber,
	}, nil
}

func readDMIInfoCached() (*linux.DMIInfo, error) {
	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return nil, trace.Wrap(err, "setting up state dir")
	}

	f, err := os.Open(stateDir.dmiJSONPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	var dmiInfo linux.DMIInfo
	err = json.NewDecoder(f).Decode(&dmiInfo)
	return &dmiInfo, trace.Wrap(err)
}

func readDMIInfoEscalated() (*linux.DMIInfo, error) {
	tshPath, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err, "reading current executable")
	}

	// Run `sudo -v` first to re-authenticate, then run the actual tsh command
	// using `sudo --non-interactive`, so we don't risk getting sudo output
	// mixed with our desired output.
	sudoCmd := exec.Command("/usr/bin/sudo", "-v")
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr
	sudoCmd.Stdin = os.Stdin
	fmt.Println("Determining machine model and serial number, if prompted please type the sudo password")
	if err := sudoCmd.Run(); err != nil {
		return nil, trace.Wrap(err, "running `sudo -v`")
	}

	// Use a context for the cached sudo invocation. Unlike the previous command,
	// this shouldn't require any user input, thus it's expected to run fast.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dmiOut := &bytes.Buffer{}
	dmiCmd := exec.CommandContext(ctx, "/usr/bin/sudo", "-n", tshPath, "device", "dmi-read")
	dmiCmd.Stdout = dmiOut
	if err := dmiCmd.Run(); err != nil {
		return nil, trace.Wrap(err, "running `sudo tsh device dmi-read`")
	}

	// Strip any leading output before the first `{`, just in case.
	val := dmiOut.String()
	if n := strings.Index(val, "{"); n > 0 {
		val = val[n-1:]
	}

	var dmiInfo linux.DMIInfo
	if err := json.Unmarshal([]byte(val), &dmiInfo); err != nil {
		return nil, trace.Wrap(err, "parsing dmi-read output")
	}

	if err := saveDMIInfoToCache(&dmiInfo); err != nil {
		log.WithError(err).Warn("TPM: Failed to write DMI cache")
		// err swallowed on purpose.
	}

	return &dmiInfo, nil
}

func saveDMIInfoToCache(dmiInfo *linux.DMIInfo) error {
	stateDir, err := setupDeviceStateDir(userDirFunc)
	if err != nil {
		return trace.Wrap(err, "setting up state dir")
	}

	f, err := os.OpenFile(stateDir.dmiJSONPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return trace.Wrap(err, "opening dmi.json for write")
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(dmiInfo); err != nil {
		return trace.Wrap(err, "writing dmi.json")
	}
	if err := f.Close(); err != nil {
		return trace.Wrap(err, "closing dmi.json after write")
	}
	log.Debug("TPM: Saved DMI information to local cache")

	return nil
}
