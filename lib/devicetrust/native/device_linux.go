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
	"os/user"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/linux"
)

func enrollDeviceInit() (*devicepb.EnrollDeviceInit, error) {
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

func collectDeviceData() (*devicepb.DeviceCollectedData, error) {
	osRelease, err := linux.ParseOSRelease()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dmiInfo, err := linux.DMIInfoFromSysfs()
	if err != nil {
		log.WithError(err).Warn("TPM: Failed to read device model and/or serial numbers")
	}

	// dmiInfo is expected to never be nil, but code defensively just in case.
	var modelIdentifier, reportedAssetTag, systemSerialNumber, baseBoardSerialNumber string
	if dmiInfo != nil {
		modelIdentifier = dmiInfo.ProductName
		reportedAssetTag = dmiInfo.ChassisAssetTag
		systemSerialNumber = dmiInfo.ProductSerial
		baseBoardSerialNumber = dmiInfo.BoardSerial
	}

	u, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &devicepb.DeviceCollectedData{
		CollectTime:     timestamppb.Now(),
		OsType:          devicepb.OSType_OS_TYPE_LINUX,
		SerialNumber:    firstValidAssetTag(reportedAssetTag, systemSerialNumber, baseBoardSerialNumber),
		ModelIdentifier: modelIdentifier,
		// TODO(codingllama): Write os_id for Linux devices.
		OsVersion:             osRelease.VersionID,
		OsBuild:               osRelease.Version,
		OsUsername:            u.Name,
		ReportedAssetTag:      reportedAssetTag,
		SystemSerialNumber:    systemSerialNumber,
		BaseBoardSerialNumber: baseBoardSerialNumber,
	}, nil
}
