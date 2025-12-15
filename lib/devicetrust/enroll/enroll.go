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

package enroll

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/trail"
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
	SolveTPMEnrollChallenge func(challenge *devicepb.TPMEnrollChallenge, debug bool) (*devicepb.TPMEnrollChallengeResponse, error)
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

// RunAdminOutcome is the outcome of [Ceremony.RunAdmin].
// It is used to communicate the actions performed.
type RunAdminOutcome int

const (
	_ RunAdminOutcome = iota // Zero means nothing happened.
	DeviceEnrolled
	DeviceRegistered
	DeviceRegisteredAndEnrolled
)

// RunAdmin is a more powerful variant of Run: it attempts to register the
// current device, creates an enrollment token and uses that token to call Run.
//
// Must be called by a user capable of performing all actions above, otherwise
// it fails.
//
// Returns the created or enrolled device, an outcome marker and an error. The
// zero outcome means everything failed.
//
// Note that the device may be created and the ceremony can still fail
// afterwards, causing a return similar to "return dev, DeviceRegistered, err"
// (where nothing is "nil").
func (c *Ceremony) RunAdmin(
	ctx context.Context,
	devicesClient devicepb.DeviceTrustServiceClient,
	debug bool,
) (*devicepb.Device, RunAdminOutcome, error) {
	// The init message contains the device collected data.
	init, err := c.EnrollDeviceInit()
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}
	cdd := init.DeviceData
	osType := cdd.OsType
	assetTag := cdd.SerialNumber

	rewordAccessDenied := func(err error, action string) error {
		if trace.IsAccessDenied(trail.FromGRPC(err)) {
			slog.DebugContext(ctx, "Device Trust: Redacting access denied error with user-friendly message", "error", err)
			return trace.AccessDenied(
				"User does not have permissions to %s. Contact your cluster device administrator.",
				action,
			)
		}
		return err
	}

	// Query for current device.
	findResp, err := devicesClient.FindDevices(ctx, &devicepb.FindDevicesRequest{
		IdOrTag: assetTag,
	})
	if err != nil {
		return nil, 0, trace.Wrap(rewordAccessDenied(err, "list devices"))
	}
	var currentDev *devicepb.Device
	for _, dev := range findResp.Devices {
		if dev.OsType == osType {
			currentDev = dev
			slog.DebugContext(ctx, "Device Trust: Found device",
				slog.Group("device",
					slog.String("asset_tag", currentDev.AssetTag),
					slog.String("os", devicetrust.FriendlyOSType(currentDev.OsType)),
					slog.String("id", currentDev.Id),
				),
			)
			break
		}
	}

	// If missing, create the device.
	var outcome RunAdminOutcome
	if currentDev == nil {
		currentDev, err = devicesClient.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
			Device: &devicepb.Device{
				OsType:   osType,
				AssetTag: assetTag,
			},
			CreateEnrollToken: true, // Save an additional RPC.
		})
		if err != nil {
			return nil, outcome, trace.Wrap(rewordAccessDenied(err, "register devices"))
		}
		outcome = DeviceRegistered
	}
	// From here onwards, always return `currentDev` and `outcome`!

	// If missing, create a new enrollment token.
	if currentDev.EnrollToken.GetToken() == "" {
		currentDev.EnrollToken, err = devicesClient.CreateDeviceEnrollToken(ctx, &devicepb.CreateDeviceEnrollTokenRequest{
			DeviceId: currentDev.Id,
		})
		if err != nil {
			return currentDev, outcome, trace.Wrap(rewordAccessDenied(err, "create device enrollment tokens"))
		}
		slog.DebugContext(ctx, "Device Trust: Created enrollment token for device",
			slog.Group("device",
				slog.String("asset_tag", currentDev.AssetTag),
				slog.String("os", devicetrust.FriendlyOSType(currentDev.OsType)),
				slog.String("id", currentDev.Id),
			),
		)
	}
	token := currentDev.EnrollToken.GetToken()

	// Then proceed onto enrollment.
	enrolled, err := c.Run(ctx, devicesClient, debug, token)
	if err != nil {
		return currentDev, outcome, trace.Wrap(err)
	}

	outcome++ // "0" becomes "Enrolled", "Registered" becomes "RegisteredAndEnrolled".
	return enrolled, outcome, trace.Wrap(err)
}

// Run performs the client-side device enrollment ceremony.
func (c *Ceremony) Run(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, debug bool, enrollToken string) (*devicepb.Device, error) {
	init, err := c.EnrollDeviceInit()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	init.Token = enrollToken

	return c.run(ctx, devicesClient, debug, init)
}

func (c *Ceremony) run(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, debug bool, init *devicepb.EnrollDeviceInit) (*devicepb.Device, error) {
	// Sanity check.
	if init.GetToken() == "" {
		return nil, trace.BadParameter("enroll init message lacks enrollment token")
	}

	stream, err := devicesClient.EnrollDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	defer stream.CloseSend()

	// 1. Init.
	if err := stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: init,
		},
	}); err != nil && !errors.Is(err, io.EOF) {
		// [io.EOF] indicates that the server has closed the stream.
		// The client should handle the underlying error on the subsequent Recv call.
		// All other errors are client-side errors and should be returned.
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(devicetrust.HandleUnimplemented(err))
	}
	// Unimplemented errors are not expected to happen after this point.

	// 2. Challenge.
	switch c.GetDeviceOSType() {
	case devicepb.OSType_OS_TYPE_MACOS:
		err = c.enrollDeviceMacOS(stream, resp)
		// err handled below
	case devicepb.OSType_OS_TYPE_LINUX, devicepb.OSType_OS_TYPE_WINDOWS:
		err = c.enrollDeviceTPM(ctx, stream, resp, debug)
		// err handled below
	default:
		// Safety check.
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
	if err != nil && !errors.Is(err, io.EOF) {
		// [io.EOF] indicates that the server has closed the stream.
		// The client should handle the underlying error on the subsequent Recv call.
		// All other errors are client-side errors and should be returned.
		return trace.Wrap(err)
	}
	return nil
}

func (c *Ceremony) enrollDeviceTPM(ctx context.Context, stream devicepb.DeviceTrustService_EnrollDeviceClient, resp *devicepb.EnrollDeviceResponse, debug bool) error {
	challenge := resp.GetTpmChallenge()
	switch {
	case challenge == nil:
		return trace.BadParameter("unexpected challenge payload from server: %T", resp.Payload)
	case challenge.EncryptedCredential == nil:
		return trace.BadParameter("missing encrypted_credential in challenge from server")
	case len(challenge.AttestationNonce) == 0:
		return trace.BadParameter("missing attestation_nonce in challenge from server")
	}

	challengeResponse, err := c.SolveTPMEnrollChallenge(challenge, debug)
	if err != nil {
		return trace.Wrap(err)
	}
	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_TpmChallengeResponse{
			TpmChallengeResponse: challengeResponse,
		},
	})
	// [io.EOF] indicates that the server has closed the stream.
	// The client should handle the underlying error on the subsequent Recv call.
	// All other errors are client-side errors and should be returned.
	if err != nil && !errors.Is(err, io.EOF) {
		return trace.Wrap(err)
	}
	return nil
}
