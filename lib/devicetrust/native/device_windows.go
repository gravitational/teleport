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
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/windowsexec"
)

// deviceStateFolderName starts with a "." on Windows for backwards
// compatibility, but in practice it does not need to.
const deviceStateFolderName = ".teleport-device"

var windowsDevice = &tpmDevice{
	isElevatedProcess: func() (bool, error) {
		return windows.GetCurrentProcessToken().IsElevated(), nil
	},
	activateCredentialInElevatedChild: activateCredentialInElevatedChild,
}

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return windowsDevice.enrollDeviceInit()
}

func signChallenge(chal []byte) (sig []byte, err error) {
	return nil, errors.New("signChallenge not implemented for TPM devices")
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	return windowsDevice.getDeviceCredential()
}

func solveTPMEnrollChallenge(
	chal *devicepb.TPMEnrollChallenge,
	debug bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return windowsDevice.solveTPMEnrollChallenge(chal, debug)
}

func solveTPMAuthnDeviceChallenge(
	chal *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return windowsDevice.solveTPMAuthnDeviceChallenge(chal)
}

func handleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret string) error {
	return windowsDevice.handleTPMActivateCredential(encryptedCredential, encryptedCredentialSecret)
}

// getDeviceSerial returns the serial number of the device using PowerShell to
// grab the correct WMI objects. Getting it without calling into PS is possible,
// but requires interfacing with the ancient Win32 COM APIs.
func getDeviceSerial() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_BIOS | Select -ExpandProperty SerialNumber",
	)
	// ThinkPad P P14s:
	// PS > Get-WmiObject Win32_BIOS | Select -ExpandProperty SerialNumber
	// PF47WND6
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func getReportedAssetTag() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_SystemEnclosure | Select -ExpandProperty SMBIOSAssetTag",
	)
	// ThinkPad P P14s:
	// PS > Get-WmiObject Win32_SystemEnclosure | Select -ExpandProperty SMBIOSAssetTag
	// winaia_1337
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func getDeviceModel() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_ComputerSystem | Select -ExpandProperty Model",
	)
	// ThinkPad P P14s:
	// PS> Get-WmiObject Win32_ComputerSystem | Select -ExpandProperty Model
	// 21J50013US
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func getDeviceBaseBoardSerial() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_BaseBoard | Select -ExpandProperty SerialNumber",
	)
	// ThinkPad P P14s:
	// PS> Get-WmiObject Win32_BaseBoard | Select -ExpandProperty SerialNumber
	// L1HF2CM03ZT
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func getOSVersion() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty Version",
	)
	// ThinkPad P P14s:
	// PS>  Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty Version
	// 10.0.22621
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func getOSBuildNumber() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty BuildNumber",
	)
	// ThinkPad P P14s:
	// PS>  Get-WmiObject Win32_OperatingSystem | Select -ExpandProperty BuildNumber
	// 22621
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(bytes.TrimSpace(out)), nil
}

func collectDeviceData(_ CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	log.Debug("TPM: Collecting device data.")

	var g errgroup.Group
	const groupLimit = 4 // arbitrary
	g.SetLimit(groupLimit)

	// Run exec-ed commands concurrently.
	var systemSerial, baseBoardSerial, reportedAssetTag, model, osVersion, osBuildNumber string
	for _, spec := range []struct {
		fn   func() (string, error)
		out  *string
		desc string
	}{
		{fn: getDeviceModel, out: &model, desc: "device model"},
		{fn: getOSVersion, out: &osVersion, desc: "os version"},
		{fn: getOSBuildNumber, out: &osBuildNumber, desc: "os build number"},
		{fn: getDeviceSerial, out: &systemSerial, desc: "system serial"},
		{fn: getDeviceBaseBoardSerial, out: &baseBoardSerial, desc: "base board serial"},
		{fn: getReportedAssetTag, out: &reportedAssetTag, desc: "reported asset tag"},
	} {
		spec := spec
		g.Go(func() error {
			val, err := spec.fn()
			if err != nil {
				log.WithError(err).Debugf("TPM: Failed to fetch %v", spec.desc)
				return nil // Swallowed on purpose.
			}

			*spec.out = val
			return nil
		})
	}

	// We want to fetch as much info as possible, so errors are ignored.
	_ = g.Wait()

	u, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err, "fetching user")
	}

	serial := firstValidAssetTag(reportedAssetTag, systemSerial, baseBoardSerial)
	if serial == "" {
		return nil, trace.BadParameter("unable to determine serial number")
	}

	dcd := &devicepb.DeviceCollectedData{
		CollectTime:           timestamppb.Now(),
		OsType:                devicepb.OSType_OS_TYPE_WINDOWS,
		SerialNumber:          serial,
		ModelIdentifier:       model,
		OsVersion:             osVersion,
		OsBuild:               osBuildNumber,
		OsUsername:            u.Username,
		SystemSerialNumber:    systemSerial,
		BaseBoardSerialNumber: baseBoardSerial,
		ReportedAssetTag:      reportedAssetTag,
	}
	log.WithField(
		"device_collected_data", dcd,
	).Debug("TPM: Device data collected.")
	return dcd, nil
}

// activateCredentialInElevated child uses `runas` to trigger a child process
// with elevated privileges. This is necessary because the process must have
// elevated privileges in order to invoke the TPM 2.0 ActivateCredential
// command.
func activateCredentialInElevatedChild(
	encryptedCredential attest.EncryptedCredential,
	credActivationPath string,
	debug bool,
) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, trace.Wrap(err, "determining current executable path")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, trace.Wrap(err, "determining current working directory")
	}

	// Clear up the results of any previous credential activation
	if err := os.Remove(credActivationPath); err != nil {
		err := trace.ConvertSystemError(err)
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "clearing previous credential activation results")
		}
	}

	// Assemble the parameter list. We encoded any binary data in base64.
	// These parameters cause `tsh` to invoke HandleTPMActivateCredential.
	params := []string{
		"device",
		"tpm-activate-credential",
		"--encrypted-credential",
		base64.StdEncoding.EncodeToString(encryptedCredential.Credential),
		"--encrypted-credential-secret",
		base64.StdEncoding.EncodeToString(encryptedCredential.Secret),
	}
	if debug {
		params = append(params, "--debug")
	}

	log.Debug("Starting elevated process.")
	// https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecutew
	err = windowsexec.RunAsAndWait(
		exe,
		cwd,
		time.Second*10,
		params,
	)
	if err != nil {
		return nil, trace.Wrap(err, "invoking ShellExecute")
	}

	// Ensure we clean up the results of the execution once we are done with
	// it.
	defer func() {
		if err := os.Remove(credActivationPath); err != nil {
			log.WithError(err).Debug("Failed to clean up credential activation result")
		}
	}()

	solutionBytes, err := os.ReadFile(credActivationPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return solutionBytes, nil
}
