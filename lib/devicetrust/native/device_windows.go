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
	"crypto/x509"
	"github.com/google/go-attestation/attest"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"os/exec"
	"path"
	"strings"
)

const (
	akFile = "/.teleport-device/attestation.key"
)

func deviceStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return path.Join(home, "teleport-device/"), nil
}

func attestationKeyPath() (string, error) {
	statePath, err := deviceStatePath()
	if err != nil {
		return "", trace.Wrap(err)
	}

	return path.Join(statePath, "attestation.key"), nil
}

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

// getMarshaledEK returns the EK public key in PKIX, ASN.1 DER format.
func getMarshaledEK(tpm *attest.TPM) ([]byte, error) {
	eks, err := tpm.EKs()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(eks) == 0 {
		return nil, trace.BadParameter("no endorsement keys found in tpm")
	}
	// TODO: Marshal EK Certificate instead of key if present.
	encodedEK, err := x509.MarshalPKIXPublicKey(eks[0].Public)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return encodedEK, nil
}

func getOrCreateAK(tpm *attest.TPM) (*attest.AK, error) {
	path, err := attestationKeyPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ref, err := os.ReadFile(path); err == nil {
		ak, err := tpm.LoadAK(ref)
		if err == nil {
			return ak, nil
		}

		return ak, nil
	}
	// If no AK found on disk, create one.
	ak, err := tpm.NewAK(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref, err := ak.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: Check perms
	err = os.WriteFile(path, ref, 0644)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ak, nil
}

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	marshaledEK, err := getMarshaledEK(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ak, err := getOrCreateAK(tpm)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	deviceData, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.EnrollDeviceInit{
		CredentialId: "", // TODO: Fetch cred id
		DeviceData:   deviceData,
		Tpm: &devicepb.TPMEnrollPayload{
			Ek: &devicepb.TPMEnrollPayload_EkKey{
				EkKey: marshaledEK,
			},
			AttestationParameters: &devicepb.TPMAttestationParameters{
				// TODO: Fill this out
			},
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
