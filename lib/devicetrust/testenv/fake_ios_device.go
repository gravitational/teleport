// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testenv

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// FakeIOSDevice fakes the behavior of an iOS or iPadOS device running the
// Teleport Verify app.
//
// Unlike [FakeMacOSDevice], it doesn't implement [FakeDevice]. Mobile devices
// drive their ceremonies through the public Device Trust service instead of the
// client-side ceremonies in lib/devicetrust.
type FakeIOSDevice struct {
	ID           string
	SerialNumber string
	PubKeyDER    []byte // PKIX, ASN.1 DER form

	osType  devicepb.OSType
	privKey *ecdsa.PrivateKey
}

func NewFakeIOSDevice(osType devicepb.OSType) (*FakeIOSDevice, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	pubKeyDER, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, err
	}

	return &FakeIOSDevice{
		ID:           uuid.NewString(),
		SerialNumber: uuid.NewString(),
		osType:       osType,
		privKey:      key,
		PubKeyDER:    pubKeyDER,
	}, nil
}

// CollectDeviceData returns the device data the app submits to the public
// Device Trust service.
func (f *FakeIOSDevice) CollectDeviceData() *devicepb.DeviceCollectedData {
	return devicepb.DeviceCollectedData_builder{
		CollectTime:  timestamppb.Now(),
		OsType:       f.osType,
		OsVersion:    "26.0.1",
		SerialNumber: f.SerialNumber,
	}.Build()
}

// EnrollDeviceInit returns the public service enrollment init message for the
// device, with token filled in.
func (f *FakeIOSDevice) EnrollDeviceInit(token string) *devicetrustpublicv1pb.EnrollDeviceInit {
	return devicetrustpublicv1pb.EnrollDeviceInit_builder{
		Token:        token,
		CredentialId: f.ID,
		DeviceData:   f.CollectDeviceData(),
		Ios: devicetrustpublicv1pb.IOSEnrollPayload_builder{
			PublicKeyDer: f.PubKeyDER,
		}.Build(),
	}.Build()
}

// SignChallenge signs chal with the device key, the way Secure Enclave would.
func (f *FakeIOSDevice) SignChallenge(chal []byte) ([]byte, error) {
	h := sha256.Sum256(chal)
	return ecdsa.SignASN1(rand.Reader, f.privKey, h[:])
}
