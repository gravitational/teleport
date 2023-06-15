//go:build !darwin && !windows

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

package native

import (
	"context"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
)

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func signChallenge(chal []byte) (sig []byte, err error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func solveTPMEnrollChallenge(
	_ context.Context,
	_ *devicepb.TPMEnrollChallenge,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func solveTPMAuthnDeviceChallenge(
	_ *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func handleTPMActivateCredential(_ string, _ string) error {
	return devicetrust.ErrPlatformNotSupported
}
