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

package testenv

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// FakeMacOSDevice fakes the native methods of a macOS device, as expected by
// the devicetrust packages.
type FakeMacOSDevice struct {
	ID           string
	SerialNumber string
	PubKeyDER    []byte

	privKey *ecdsa.PrivateKey
}

func NewFakeMacOSDevice() (*FakeMacOSDevice, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	pubKeyDER, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, err
	}

	return &FakeMacOSDevice{
		ID:           uuid.NewString(),
		SerialNumber: uuid.NewString(),
		privKey:      key,
		PubKeyDER:    pubKeyDER,
	}, nil
}

func (f *FakeMacOSDevice) CollectDeviceData() (*devicepb.DeviceCollectedData, error) {
	return &devicepb.DeviceCollectedData{
		CollectTime:  timestamppb.Now(),
		OsType:       devicepb.OSType_OS_TYPE_MACOS,
		SerialNumber: f.SerialNumber,
	}, nil
}

func (f *FakeMacOSDevice) GetDeviceCredential() *devicepb.DeviceCredential {
	return &devicepb.DeviceCredential{
		Id:           f.ID,
		PublicKeyDer: f.PubKeyDER,
	}
}

func (f *FakeMacOSDevice) GetDeviceOSType() devicepb.OSType {
	return devicepb.OSType_OS_TYPE_MACOS
}

func (f *FakeMacOSDevice) EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	cd, _ := f.CollectDeviceData()
	return &devicepb.EnrollDeviceInit{
		Token:        "",
		CredentialId: f.ID,
		DeviceData:   cd,
		Macos: &devicepb.MacOSEnrollPayload{
			PublicKeyDer: f.PubKeyDER,
		},
	}, nil
}

func (f *FakeMacOSDevice) SignChallenge(chal []byte) (sig []byte, err error) {
	h := sha256.Sum256(chal)
	return ecdsa.SignASN1(rand.Reader, f.privKey, h[:])
}

func (d *FakeMacOSDevice) SolveTPMEnrollChallenge(
	_ context.Context,
	_ *devicepb.TPMEnrollChallenge,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, trace.NotImplemented("mac device does not implement SolveTPMEnrollChallenge")
}

func (d *FakeMacOSDevice) SolveTPMAuthnDeviceChallenge(_ *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, trace.NotImplemented("mac device does not implement SolveTPMAuthnDeviceChallenge")
}
