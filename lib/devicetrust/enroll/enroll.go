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
	"github.com/gravitational/trace/trail"
	log "github.com/sirupsen/logrus"
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
			log.WithError(err).Debug(
				"Device Trust: Redacting access denied error with user-friendly message")
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
			log.Debugf(
				"Device Trust: Found device %q/%v, id=%q",
				currentDev.AssetTag, devicetrust.FriendlyOSType(currentDev.OsType), currentDev.Id,
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
		log.Debugf(
			"Device Trust: Created enrollment token for device %q/%s",
			currentDev.AssetTag,
			devicetrust.FriendlyOSType(currentDev.OsType))
	}
	token := currentDev.EnrollToken.GetToken()

	// Then proceed onto enrollment.
	enrolled, err := c.Run(ctx, devicesClient, debug, token)
	if err != nil {
		return enrolled, outcome, trace.Wrap(err)
	}

	outcome++ // "0" becomes "Enrolled", "Registered" becomes "RegisteredAndEnrolled".
	return enrolled, outcome, trace.Wrap(err)
}

// Run performs the client-side device enrollment ceremony.
func (c *Ceremony) Run(ctx context.Context, devicesClient devicepb.DeviceTrustServiceClient, debug bool, enrollToken string) (*devicepb.Device, error) {
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
		err = c.enrollDeviceTPM(ctx, stream, resp, debug)
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
	return trace.Wrap(err)
}
