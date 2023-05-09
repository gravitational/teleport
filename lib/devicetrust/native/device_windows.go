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
	deviceStateFolderName  = "teleport-device"
	attestationKeyFileName = "attestation.key"
)

// Ensures that device state directory exists with the correct permissions and:
// - If it does not exist, creates it.
// - If it exists with the wrong permissions, errors.
// ~/teleport-device/attestation.key
func setupDeviceStateDir(getHomeDir func() (string, error)) (string, error) {
	home, err := getHomeDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	deviceStateDirPath := path.Join(home, deviceStateFolderName)
	keyPath := path.Join(deviceStateDirPath, attestationKeyFileName)

	stat, err := os.Stat(deviceStateDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If it doesn't exist, we can create it and return as we know
			// the perms are correct as we created it.
			if err := os.Mkdir(deviceStateDirPath, 700); err != nil {
				return "", trace.Wrap(err)
			}
			return keyPath, nil
		}
		return "", trace.Wrap(err)
	}

	// As it already exists, we need to check the directory's perms
	if !stat.IsDir() {
		return "", trace.BadParameter("path %q is not a directory", deviceStateDirPath)
	}
	if stat.Mode().Perm() != 700 {
		return "", trace.BadParameter("path %q has incorrect permissions, expected 700")
	}

	// Now check if the Attestation Key exists. If it doesn't, don't create it.
	// If it does, we need to check its perms.
	stat, err = os.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return keyPath, nil
		}
		return "", trace.Wrap(err)
	}
	if stat.Mode().Perm() != 600 {
		return "", trace.BadParameter("path %q has incorrect permissions, expected 600")
	}

	return keyPath, nil
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

// loadOrCreateAK attempts to load an AK from disk. A NotFound error will be
// returned if no such file exists.
func loadAK(
	tpm *attest.TPM,
	persistencePath string,
) (*attest.AK, error) {
	ref, err := os.ReadFile(persistencePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	ak, err := tpm.LoadAK(ref)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ak, nil
}

func createAndSaveAK(
	tpm *attest.TPM,
	persistencePath string,
) (*attest.AK, error) {
	ak, err := tpm.NewAK(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Write it to the well-known location on disk
	ref, err := ak.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = os.WriteFile(persistencePath, ref, 600)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ak, nil
}

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	akPath, err := setupDeviceStateDir(os.UserHomeDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	// Try to load an existing AK in the case of re-enrollment, but, if the
	// AK does not exist, create one and persist it.
	ak, err := loadAK(tpm, akPath)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		ak, err = createAndSaveAK(tpm, akPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	defer ak.Close(tpm)

	deviceData, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	marshaledEK, err := getMarshaledEK(tpm)
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
			AttestationParameters: devicetrust.AttestationParametersToProto(
				ak.AttestationParameters(),
			),
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
	akPath, err := setupDeviceStateDir(os.UserHomeDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tpm, err := openTPM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tpm.Close()

	// Attempt to load the AK from well-known location, do not create one if
	// it does not exist - this would be invalid for solving a challenge.
	ak, err := loadAK(tpm, akPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer ak.Close(tpm)

	// First perform the credential activation challenge provided by the
	// auth server.
	activationSolution, err := ak.ActivateCredential(
		tpm,
		devicetrust.EncryptedCredentialFromProto(challenge.EncryptedCredential),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Next perform a platform attestation using the AK.
	platformsParams, err := tpm.AttestPlatform(
		ak,
		challenge.AttestationNonce,
		&attest.PlatformAttestConfig{
			EventLog: nil,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.TPMEnrollChallengeResponse{
		Solution: activationSolution,
		PlatformParameters: devicetrust.PlatformParametersToProto(
			platformsParams,
		),
	}, nil
}

// signChallenge is not implemented on windows as TPM platform attestation
// is used instead.
func signChallenge(_ []byte) (sig []byte, err error) {
	return nil, devicetrust.ErrPlatformNotSupported
}
