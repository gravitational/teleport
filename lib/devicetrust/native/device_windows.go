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
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
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

func getDeviceSerial() (string, error) {
	// ThinkPad P P14s:
	// PS > Get-WmiObject Win32_BIOS | Select -ExpandProperty SerialNumber
	// PF47WND6

	type Win32_BIOS struct {
		SerialNumber string
	}

	var bios []Win32_BIOS
	query := wmi.CreateQuery(&bios, "")
	if err := wmi.Query(query, &bios); err != nil {
		return "", trace.Wrap(err)
	}

	if len(bios) == 0 {
		return "", trace.BadParameter("could not read serial number from Win32_BIOS")
	}

	return bios[0].SerialNumber, nil
}

func getReportedAssetTag() (string, error) {
	// ThinkPad P P14s:
	// PS > Get-WmiObject Win32_SystemEnclosure | Select -ExpandProperty SMBIOSAssetTag
	// winaia_1337

	type Win32_SystemEnclosure struct {
		SMBIOSAssetTag string
	}

	var system []Win32_SystemEnclosure
	query := wmi.CreateQuery(&system, "")
	if err := wmi.Query(query, &system); err != nil {
		return "", trace.Wrap(err)
	}

	if len(system) == 0 {
		return "", trace.BadParameter("could not read asset tag from Win32_SystemEnclosure")
	}

	return system[0].SMBIOSAssetTag, nil
}

func getDeviceModel() (string, error) {
	// ThinkPad P P14s:
	// PS> Get-WmiObject Win32_ComputerSystem | Select -ExpandProperty Model
	// 21J50013US

	type Win32_ComputerSystem struct {
		Model string
	}
	var cs []Win32_ComputerSystem
	query := wmi.CreateQuery(&cs, "")
	if err := wmi.Query(query, &cs); err != nil {
		return "", trace.Wrap(err)
	}

	if len(cs) == 0 {
		return "", trace.BadParameter("could not read model from Win32_ComputerSystem")
	}

	return cs[0].Model, nil
}

func getDeviceBaseBoardSerial() (string, error) {
	// ThinkPad P P14s:
	// PS> Get-WmiObject Win32_BaseBoard | Select -ExpandProperty SerialNumber
	// L1HF2CM03ZT

	type Win32_BaseBoard struct {
		SerialNumber string
	}
	var bb []Win32_BaseBoard
	query := wmi.CreateQuery(&bb, "")
	if err := wmi.Query(query, &bb); err != nil {
		return "", trace.Wrap(err)
	}

	if len(bb) == 0 {
		return "", trace.BadParameter("could not read serial from Win32_BaseBoard")
	}

	return bb[0].SerialNumber, nil
}

func collectDeviceData(_ CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	ctx := context.Background()
	logger := slog.With(teleport.ComponentKey, "TPM")

	logger.DebugContext(ctx, "Collecting device data")

	var g errgroup.Group
	const groupLimit = 4 // arbitrary
	g.SetLimit(groupLimit)

	// Run exec-ed commands concurrently.
	var systemSerial, baseBoardSerial, reportedAssetTag, model string
	for _, spec := range []struct {
		fn   func() (string, error)
		out  *string
		desc string
	}{
		{fn: getDeviceModel, out: &model, desc: "device model"},
		{fn: getDeviceSerial, out: &systemSerial, desc: "system serial"},
		{fn: getDeviceBaseBoardSerial, out: &baseBoardSerial, desc: "base board serial"},
		{fn: getReportedAssetTag, out: &reportedAssetTag, desc: "reported asset tag"},
	} {
		spec := spec
		g.Go(func() error {
			val, err := spec.fn()
			if err != nil {
				logger.DebugContext(ctx, "Failed to fetch device details", "details", spec.desc, "error", err)
				return nil // Swallowed on purpose.
			}

			*spec.out = val
			return nil
		})
	}

	ver := windows.RtlGetVersion()

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
		OsVersion:             fmt.Sprintf("%v.%v.%v", ver.MajorVersion, ver.MinorVersion, ver.BuildNumber),
		OsBuild:               strconv.FormatInt(int64(ver.BuildNumber), 10),
		OsUsername:            u.Username,
		SystemSerialNumber:    systemSerial,
		BaseBoardSerialNumber: baseBoardSerial,
		ReportedAssetTag:      reportedAssetTag,
	}
	logger.DebugContext(ctx, "Device data collected", "device_collected_data", dcd)
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

	slog.DebugContext(context.Background(), "Starting elevated process.")
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
			slog.DebugContext(context.Background(), "Failed to clean up credential activation result", "error", err)
		}
	}()

	solutionBytes, err := os.ReadFile(credActivationPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return solutionBytes, nil
}
