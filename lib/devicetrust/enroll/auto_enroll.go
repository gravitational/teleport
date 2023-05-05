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

package enroll

import (
	"context"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// AutoEnroll attempts to create an auto-enroll token via
// [devicepb.DeviceTrustServiceClient.CreateDeviceEnrollToken] and enrolls the
// device by calling [RunCeremony].
func AutoEnroll(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient) (*devicepb.Device, error) {
	cd, err := collectDeviceData()
	if err != nil {
		return nil, trace.Wrap(err, "collecting device data")
	}

	token, err := devicesClient.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
		DeviceData: cd,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating auto-token")
	}

	dev, err := RunCeremony(ctx, devicesClient, token.Token)
	return dev, trace.Wrap(err)
}
