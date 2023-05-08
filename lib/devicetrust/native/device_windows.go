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
	"github.com/google/go-attestation/attest"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os/exec"
	"strings"
)

func openTPM() (*attest.TPM, error) {
	cfg := &attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
		// TODO: Determine if windows command channel wrapper is necessary
	}

	tpm, err := attest.OpenTPM(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tpm, nil
}

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	cd, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &devicepb.EnrollDeviceInit{
		CredentialId: "", // TODO: Fetch cred id
		DeviceData:   cd,
		Tpm:          &devicepb.TPMEnrollPayload{
			// TODO: Fill this out
		},
	}, nil
}

// getDeviceSerial returns the serial number of the device using PowerShell to
// grab the correct WMI objects. Getting it without calling into PS is possible,
// but requires interfacing with the ancient Win32 COM APIs.
func getDeviceSerial() (string, error) {
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"Get-WmiObject win32_bios | Select -ExpandProperty Serialnumber",
	)
	// Example output from powershell terminal on a non-administrator Lenovo
	// ThinkPad P P14s:
	// PS C:\Users\noahstride> Get-WmiObject win32_bios | Select -ExpandProperty Serialnumber
	// PF47WND6
	out, err := cmd.Output()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return strings.TrimSpace(string(bytes.ReplaceAll(out, []byte(" "), nil))), nil
}

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	serial, err := getDeviceSerial()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: Collect data:
	// - BaseBoard serial?
	// - Model?
	// - OS Version?
	// - OS Build?
	// - Username?
	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_WINDOWS,
		SerialNumber: serial,
	}, nil
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	return nil, nil
}

func solveTPMEnrollChallenge(
	challenge *devicepb.TPMEnrollChallenge,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, nil
}

// signChallenge is not implemented on windows as TPM platform attestation
// is used instead.
func signChallenge(_ []byte) (sig []byte, err error) {
	return nil, devicetrust.ErrPlatformNotSupported
}
