//go:build !darwin && !linux && !windows

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

package native

import (
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
)

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func collectDeviceData(mode CollectDataMode) (*devicepb.DeviceCollectedData, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func signChallenge(chal []byte) (sig []byte, err error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func getDeviceCredential() (*devicepb.DeviceCredential, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func solveTPMEnrollChallenge(
	_ *devicepb.TPMEnrollChallenge,
	_ bool,
) (*devicepb.TPMEnrollChallengeResponse, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func solveTPMAuthnDeviceChallenge(
	_ *devicepb.TPMAuthenticateDeviceChallenge,
) (*devicepb.TPMAuthenticateDeviceChallengeResponse, error) {
	return nil, devicetrust.ErrPlatformNotSupported
}

func handleTPMActivateCredential(_, _ string) error {
	return devicetrust.ErrPlatformNotSupported
}
