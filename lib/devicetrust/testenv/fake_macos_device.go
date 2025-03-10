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

package testenv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/cryptopatch"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust/native"
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
	key, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
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

func (f *FakeMacOSDevice) CollectDeviceData(mode native.CollectDataMode) (*devicepb.DeviceCollectedData, error) {
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
	cd, _ := f.CollectDeviceData(native.CollectedDataAlwaysEscalate)
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
	_ *devicepb.TPMEnrollChallenge,
	_ bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, trace.NotImplemented("mac device does not implement SolveTPMEnrollChallenge")
}

func (d *FakeMacOSDevice) SolveTPMAuthnDeviceChallenge(_ *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, trace.NotImplemented("mac device does not implement SolveTPMAuthnDeviceChallenge")
}
