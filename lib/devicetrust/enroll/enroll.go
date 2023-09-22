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

package enroll

import (
	"context"
	"runtime"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// Ceremony is the device enrollment ceremony.
// It takes the client role of
// [devicepb.DeviceTrustServiceClient.EnrollDevice].
type Ceremony struct {
	GetDeviceOSType  func() devicepb.OSType
	EnrollDeviceInit func() (*devicepb.EnrollDeviceInit, error)
	SignChallenge    func(chal []byte) (sig []byte, err error)
}

// NewCeremony creates a new ceremony that delegates per-device behavior
// to lib/devicetrust/native.
// If you want to customize a [Ceremony], for example for testing purposes, you
// may create a configure an instance directly, without calling this method.
func NewCeremony() *Ceremony {
	return &Ceremony{
		GetDeviceOSType:  getDeviceOSType,
		EnrollDeviceInit: native.EnrollDeviceInit,
		SignChallenge:    native.SignChallenge,
	}
}

// RunCeremony performs the client-side device enrollment ceremony.
// Equivalent to `NewCeremony().Run()`.
func RunCeremony(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, enrollToken string) (*devicepb.Device, error) {
	return NewCeremony().Run(ctx, devicesClient, enrollToken)
}

// Run performs the client-side device enrollment ceremony.
func (c *Ceremony) Run(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, enrollToken string) (*devicepb.Device, error) {
	// Start by checking the OSType, this lets us exit early with a nicer message
	// for unsupported OSes.
	if c.GetDeviceOSType() != devicepb.OSType_OS_TYPE_MACOS {
		return nil, trace.BadParameter("device enrollment not supported for current OS (%v)", runtime.GOOS)
	}

	init, err := c.EnrollDeviceInit()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	init.Token = enrollToken

	// 1. Init.
	stream, err := devicesClient.EnrollDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: init,
		},
	}); err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	// Unimplemented errors are not expected to happen after this point.

	// 2. Challenge.
	// Only macOS is supported, see the guard at the beginning of the method.
	if err := c.enrollDeviceMacOS(stream, resp); err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err = stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// 3. Success.
	successResp := resp.GetSuccess()
	if successResp == nil {
		return nil, trace.BadParameter("unexpected success payload from server: %T", resp.Payload)
	}
	return successResp.Device, nil
}

func (c *Ceremony) enrollDeviceMacOS(stream devicepb.DeviceTrustService_EnrollDeviceClient, resp *devicepb.EnrollDeviceResponse) error {
	chalResp := resp.GetMacosChallenge()
	if chalResp == nil {
		return trace.BadParameter("unexpected challenge payload from server: %T", resp.Payload)
	}
	sig, err := c.SignChallenge(chalResp.Challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	})
	return trace.Wrap(err)
}

func getDeviceOSType() devicepb.OSType {
	switch runtime.GOOS {
	case "darwin":
		return devicepb.OSType_OS_TYPE_MACOS
	case "linux":
		return devicepb.OSType_OS_TYPE_LINUX
	case "windows":
		return devicepb.OSType_OS_TYPE_WINDOWS
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED
	}
}
