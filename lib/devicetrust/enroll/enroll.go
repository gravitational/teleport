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

	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/native"
)

// Ceremony is the device enrollment ceremony.
// It takes the client role of
// [devicepb.DeviceTrustServiceClient.EnrollDevice].
type Ceremony struct {
	GetDeviceOSType         func() devicepb.OSType
	EnrollDeviceInit        func() (*devicepb.EnrollDeviceInit, error)
	SignChallenge           func(chal []byte) (sig []byte, err error)
	SolveTPMEnrollChallenge func(ctx context.Context, challenge *devicepb.TPMEnrollChallenge) (*devicepb.TPMEnrollChallengeResponse, error)
}

// NewCeremony creates a new ceremony that delegates per-device behavior
// to lib/devicetrust/native.
// If you want to customize a [Ceremony], for example for testing purposes, you
// may create a configure an instance directly, without calling this method.
func NewCeremony() *Ceremony {
	return &Ceremony{
		GetDeviceOSType:         native.GetDeviceOSType,
		EnrollDeviceInit:        native.EnrollDeviceInit,
		SignChallenge:           native.SignChallenge,
		SolveTPMEnrollChallenge: native.SolveTPMEnrollChallenge,
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
	osType := c.GetDeviceOSType()
	if !slices.Contains([]devicepb.OSType{
		devicepb.OSType_OS_TYPE_MACOS,
		devicepb.OSType_OS_TYPE_WINDOWS,
	}, osType) {
		return nil, trace.BadParameter(
			"device enrollment not supported for current OS (%s)",
			types.ResourceOSTypeToString(osType),
		)
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
	switch osType {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = c.enrollDeviceMacOS(stream, resp)
		// err handled below
	case devicepb.OSType_OS_TYPE_WINDOWS:
		err = c.enrollDeviceTPM(ctx, stream, resp)
		// err handled below
	default:
		// This should be caught by the OSType guard at start of function.
		panic("no enrollment function provided for os")
	}
	if err != nil {
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

func (c *Ceremony) enrollDeviceTPM(ctx context.Context, stream devicepb.DeviceTrustService_EnrollDeviceClient, resp *devicepb.EnrollDeviceResponse) error {
	challenge := resp.GetTpmChallenge()
	switch {
	case challenge == nil:
		return trace.BadParameter("unexpected challenge payload from server: %T", resp.Payload)
	case challenge.EncryptedCredential == nil:
		return trace.BadParameter("missing encrypted_credential in challenge from server")
	case len(challenge.AttestationNonce) == 0:
		return trace.BadParameter("missing attestation_nonce in challenge from server")
	}

	challengeResponse, err := c.SolveTPMEnrollChallenge(ctx, challenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: challengeResponse,
		},
	})
	return trace.Wrap(err)
}
