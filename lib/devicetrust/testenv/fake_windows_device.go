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

package testenv

import (
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// FakeWindowsDevice allows us to exercise EnrollCeremony. To avoid requiring
// dependencies to support a TPM simulator, we currently do not closely emulate
// the behavior of a real windows device.
// TODO(noah): When the underlying implementation in `native/` is refactored to
// share code between Windows & Linux, it will be a good opportunity to refactor
// this implementation to be more realistic.
type FakeWindowsDevice struct {
	CredentialID string
	SerialNumber string
}

func NewFakeWindowsDevice() *FakeWindowsDevice {
	return &FakeWindowsDevice{
		CredentialID: uuid.NewString(),
		SerialNumber: uuid.NewString(),
	}
}

func (f *FakeWindowsDevice) GetDeviceOSType() devicepb.OSType {
	return devicepb.OSType_OS_TYPE_WINDOWS
}

func (f *FakeWindowsDevice) CollectDeviceData() (*devicepb.DeviceCollectedData, error) {
	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_WINDOWS,
		SerialNumber: f.SerialNumber,
	}, nil
}

var validEKKey = []byte("FAKE_VALID_EK_KEY")
var validAttestationParameters = &devicepb.TPMAttestationParameters{
	Public: []byte("FAKE_TPMT_PUBLIC_FOR_AK"),
}

func (f *FakeWindowsDevice) EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	cd, _ := f.CollectDeviceData()
	return &devicepb.EnrollDeviceInit{
		CredentialId: f.CredentialID,
		DeviceData:   cd,
		Tpm: &devicepb.TPMEnrollPayload{
			Ek: &devicepb.TPMEnrollPayload_EkKey{
				EkKey: validEKKey,
			},
			AttestationParameters: validAttestationParameters,
		},
	}, nil
}

func (f *FakeWindowsDevice) SolveTPMEnrollChallenge(
	challenge *devicepb.TPMEnrollChallenge,
	_ bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	// This extremely roughly mimics the actual TPM by using the values
	// provided in the encrypted credential to produce an activation challenge
	// "solution", and uses the provided nonce in a fake platform attestation.
	// This lets us assert from the server that the `SolveTPMEnrollChallenge`
	// is provided all the values from the server by `RunCeremony`.
	solution := append(
		challenge.EncryptedCredential.Secret,
		challenge.EncryptedCredential.CredentialBlob...,
	)
	return &devicepb.TPMEnrollChallengeResponse{
		Solution: solution,
		PlatformParameters: &devicepb.TPMPlatformParameters{
			EventLog: challenge.AttestationNonce,
		},
	}, nil
}

func (f *FakeWindowsDevice) SolveTPMAuthnDeviceChallenge(
	challenge *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	// This fake is similar to the one used in SolveTPMEnrollChallenge except
	// only the PlatformAttestation is faked, as CredentialActivation is not
	// used in device authentication.
	return &devicepb.TPMAuthenticateDeviceChallengeResponse{
		PlatformParameters: &devicepb.TPMPlatformParameters{
			EventLog: challenge.AttestationNonce,
		},
	}, nil
}

func (f *FakeWindowsDevice) SignChallenge(_ []byte) (sig []byte, err error) {
	return nil, trace.NotImplemented("windows does not implement SignChallenge")
}

func (f *FakeWindowsDevice) GetDeviceCredential() *devicepb.DeviceCredential {
	return &devicepb.DeviceCredential{
		Id: f.CredentialID,
	}
}
