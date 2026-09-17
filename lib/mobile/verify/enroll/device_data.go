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

package enroll

import (
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// DeviceCollectedData mirrors the iOS-relevant subset of
// teleport.devicetrust.v1.DeviceCollectedData. The wrapper is required
// because gomobile only exports types from the bound package.
//
// Fields don't start with an acronym to dodge a gomobile bug
// (https://github.com/golang/go/issues/32008): VersionOS, not OSVersion.
type DeviceCollectedData struct {
	SerialNumber       string
	ModelIdentifier    string
	VersionOS          string
	BuildOS            string
	SystemSerialNumber string
}

// toPBDeviceData translates DeviceCollectedData into the proto type.
func toPBDeviceData(d *DeviceCollectedData) (*devicepb.DeviceCollectedData, error) {
	if d == nil {
		d = &DeviceCollectedData{}
	}
	osType, err := osTypeFromModelIdentifier(d.ModelIdentifier)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return devicepb.DeviceCollectedData_builder{
		CollectTime:        timestamppb.Now(),
		OsType:             osType,
		SerialNumber:       d.SerialNumber,
		ModelIdentifier:    d.ModelIdentifier,
		OsVersion:          d.VersionOS,
		OsBuild:            d.BuildOS,
		SystemSerialNumber: d.SystemSerialNumber,
	}.Build(), nil
}

func osTypeFromModelIdentifier(modelIdentifier string) (devicepb.OSType, error) {
	switch model := strings.ToLower(modelIdentifier); {
	case strings.HasPrefix(model, "iphone"):
		return devicepb.OSType_OS_TYPE_IOS, nil
	case strings.HasPrefix(model, "ipad"):
		return devicepb.OSType_OS_TYPE_IPADOS, nil
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED,
			trace.BadParameter("unsupported model identifier %q", modelIdentifier)
	}
}
