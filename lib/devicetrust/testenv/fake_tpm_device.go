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
	"errors"

	"github.com/google/uuid"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/native"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FakeTPMDevice allows us to exercise EnrollCeremony. To avoid requiring
// dependencies to support a TPM simulator, we currently do not closely emulate
// the behavior of a real windows device.
type FakeTPMDevice struct {
	OSType       devicepb.OSType
	CredentialID string
	SerialNumber string
}

func NewFakeLinuxDevice() *FakeTPMDevice {
	return &FakeTPMDevice{
		OSType:       devicepb.OSType_OS_TYPE_LINUX,
		CredentialID: uuid.NewString(),
		SerialNumber: uuid.NewString(),
	}
}

func NewFakeWindowsDevice() *FakeTPMDevice {
	return &FakeTPMDevice{
		OSType:       devicepb.OSType_OS_TYPE_WINDOWS,
		CredentialID: uuid.NewString(),
		SerialNumber: uuid.NewString(),
	}
}

func (f *FakeTPMDevice) GetDeviceOSType() devicepb.OSType {
	return f.OSType
}

func (f *FakeTPMDevice) CollectDeviceData(mode native.CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       f.OSType,
		SerialNumber: f.SerialNumber,
		// Note: other data points are nice to have, but not mandatory.
	}, nil
}

var validEKKey = []byte("FAKE_VALID_EK_KEY")
var validAttestationParameters = &devicepb.TPMAttestationParameters{
	Public: []byte("FAKE_TPMT_PUBLIC_FOR_AK"),
}

func (f *FakeTPMDevice) EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	cd, _ := f.CollectDeviceData(native.CollectedDataAlwaysEscalate)
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

func (f *FakeTPMDevice) SolveTPMEnrollChallenge(
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

func (f *FakeTPMDevice) SolveTPMAuthnDeviceChallenge(
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

func (f *FakeTPMDevice) SignChallenge(_ []byte) (sig []byte, err error) {
	return nil, errors.New("not implemented for TPM devices")
}

func (f *FakeTPMDevice) GetDeviceCredential() *devicepb.DeviceCredential {
	return &devicepb.DeviceCredential{
		Id: f.CredentialID,
	}
}
