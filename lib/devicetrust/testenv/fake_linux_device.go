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
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// FakeLinuxDevice only implements GetDeviceOSType so we can be sure
// this fails in a user friendly manner.
type FakeLinuxDevice struct{}

func NewFakeLinuxDevice() *FakeLinuxDevice {
	return &FakeLinuxDevice{}
}

func (d *FakeLinuxDevice) GetDeviceOSType() devicepb.OSType {
	return devicepb.OSType_OS_TYPE_LINUX
}

func (d *FakeLinuxDevice) CollectDeviceData() (*devicepb.DeviceCollectedData, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) EnrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) SignChallenge(_ []byte) (sig []byte, err error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) SolveTPMEnrollChallenge(_ *devicepb.TPMEnrollChallenge, _ bool) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}

func (d *FakeLinuxDevice) GetDeviceCredential() *devicepb.DeviceCredential {
	return nil
}

func (f *FakeLinuxDevice) SolveTPMAuthnDeviceChallenge(_ *devicepb.TPMAuthenticateDeviceChallenge) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, trace.NotImplemented("linux device fake unimplemented")
}
