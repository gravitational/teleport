/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/services"
)

func TestMFADeviceManagement(t *testing.T) {
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	as, err := NewTestAuthServer(TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clock,
	})
	require.NoError(t, err)
	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)
	// Enable U2F support.
	authPref, err := services.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: teleport.Local,
		U2F: &types.U2F{
			AppID:  "teleport",
			Facets: []string{"teleport"},
		},
	})
	require.NoError(t, err)
	err = as.AuthServer.SetAuthPreference(authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(as.AuthServer, "mfa-user", []string{"role"})
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// No MFA devices should exist for a new user.
	resp, err := cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Devices)

	totpSecrets := make(map[string]string)
	u2fDevices := make(map[string]*mocku2f.Key)

	// Add several MFA devices.
	addTests := []struct {
		desc string
		opts mfaAddTestOpts
	}{
		{
			desc: "add initial TOTP device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "totp-dev",
					Type:       proto.AddMFADeviceRequestInit_TOTP,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// The challenge should be empty for the first device.
					require.Empty(t, cmp.Diff(req, &proto.MFAAuthenticateChallenge{}))
					return &proto.MFAAuthenticateResponse{}
				},
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					totpRegisterChallenge := req.GetTOTP()
					require.NotEmpty(t, totpRegisterChallenge)
					require.Equal(t, totpRegisterChallenge.Algorithm, otp.AlgorithmSHA1.String())
					code, err := totp.GenerateCodeCustom(totpRegisterChallenge.Secret, clock.Now(), totp.ValidateOpts{
						Period:    uint(totpRegisterChallenge.PeriodSeconds),
						Digits:    otp.Digits(totpRegisterChallenge.Digits),
						Algorithm: otp.AlgorithmSHA1,
					})
					require.NoError(t, err)

					totpSecrets["totp-dev"] = totpRegisterChallenge.Secret
					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{
							Code: code,
						}},
					}
				},
				wantDev: func(t *testing.T) *types.MFADevice {
					wantDev, err := services.NewTOTPDevice("totp-dev", totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)
					return wantDev
				},
			},
		},
		{
			desc: "add a U2F device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "u2f-dev",
					Type:       proto.AddMFADeviceRequestInit_U2F,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Respond to challenge using the existing TOTP device.
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					u2fRegisterChallenge := req.GetU2F()
					require.NotEmpty(t, u2fRegisterChallenge)

					mdev, err := mocku2f.Create()
					require.NoError(t, err)
					u2fDevices["u2f-dev"] = mdev
					mresp, err := mdev.RegisterResponse(&u2f.RegisterChallenge{
						Challenge: u2fRegisterChallenge.Challenge,
						AppID:     u2fRegisterChallenge.AppID,
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{Response: &proto.MFARegisterResponse_U2F{U2F: &proto.U2FRegisterResponse{
						RegistrationData: mresp.RegistrationData,
						ClientData:       mresp.ClientData,
					}}}
				},
				wantDev: func(t *testing.T) *types.MFADevice {
					wantDev, err := u2f.NewDevice(
						"u2f-dev",
						&u2f.Registration{
							KeyHandle: u2fDevices["u2f-dev"].KeyHandle,
							PubKey:    u2fDevices["u2f-dev"].PrivateKey.PublicKey,
						},
						clock.Now(),
					)
					require.NoError(t, err)
					return wantDev
				},
			},
		},
	}
	for _, tt := range addTests {
		t.Run(tt.desc, func(t *testing.T) {
			testAddMFADevice(ctx, t, cl, tt.opts)
			// Advance the time to roll TOTP tokens.
			clock.Advance(30 * time.Second)
		})
	}

	// Check that all new devices are registered.
	resp, err = cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	deviceNames := make([]string, 0, len(resp.Devices))
	for _, dev := range resp.Devices {
		deviceNames = append(deviceNames, dev.GetName())
	}
	sort.Strings(deviceNames)
	require.Equal(t, deviceNames, []string{"totp-dev", "u2f-dev"})

	// Delete several of the MFA devices.
	deleteTests := []struct {
		desc string
		opts mfaDeleteTestOpts
	}{
		{
			desc: "delete TOTP device",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "totp-dev",
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Respond to the challenge using the TOTP device being deleted.
					require.NotNil(t, req.TOTP)
					code, err := totp.GenerateCode(totpSecrets["totp-dev"], clock.Now())
					require.NoError(t, err)

					delete(totpSecrets, "totp-dev")

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
			},
		},
		{
			desc: "delete last U2F device",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "u2f-dev",
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.Len(t, req.U2F, 1)
					chal := req.U2F[0]

					mdev := u2fDevices["u2f-dev"]
					mresp, err := mdev.SignResponse(&u2f.AuthenticateChallenge{
						Challenge: chal.Challenge,
						KeyHandle: chal.KeyHandle,
						AppID:     chal.AppID,
					})
					require.NoError(t, err)

					delete(u2fDevices, "u2f-dev")
					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_U2F{U2F: &proto.U2FResponse{
						KeyHandle:  mresp.KeyHandle,
						ClientData: mresp.ClientData,
						Signature:  mresp.SignatureData,
					}}}
				},
			},
		},
	}
	for _, tt := range deleteTests {
		t.Run(tt.desc, func(t *testing.T) {
			testDeleteMFADevice(ctx, t, cl, tt.opts)
		})
	}

	// Check the remaining number of devices
	resp, err = cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Len(t, resp.Devices, len(addTests)-len(deleteTests))
}

type mfaAddTestOpts struct {
	initReq         *proto.AddMFADeviceRequestInit
	authHandler     func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	registerHandler func(*testing.T, *proto.MFARegisterChallenge) *proto.MFARegisterResponse
	wantDev         func(*testing.T) *types.MFADevice
}

func testAddMFADevice(ctx context.Context, t *testing.T, cl *Client, opts mfaAddTestOpts) {
	addStream, err := cl.AddMFADevice(ctx)
	require.NoError(t, err)
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_Init{Init: opts.initReq}})
	require.NoError(t, err)

	authChallenge, err := addStream.Recv()
	require.NoError(t, err)
	authResp := opts.authHandler(t, authChallenge.GetExistingMFAChallenge())
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_ExistingMFAResponse{ExistingMFAResponse: authResp}})
	require.NoError(t, err)

	registerChallenge, err := addStream.Recv()
	require.NoError(t, err)
	registerResp := opts.registerHandler(t, registerChallenge.GetNewMFARegisterChallenge())
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{NewMFARegisterResponse: registerResp}})
	require.NoError(t, err)

	registerAck, err := addStream.Recv()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(registerAck.GetAck(), &proto.AddMFADeviceResponseAck{
		Device: opts.wantDev(t),
	}, cmpopts.IgnoreFields(types.MFADevice{}, "Id")))

	require.NoError(t, addStream.CloseSend())
}

type mfaDeleteTestOpts struct {
	initReq     *proto.DeleteMFADeviceRequestInit
	authHandler func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
}

func testDeleteMFADevice(ctx context.Context, t *testing.T, cl *Client, opts mfaDeleteTestOpts) {
	deleteStream, err := cl.DeleteMFADevice(ctx)
	require.NoError(t, err)
	err = deleteStream.Send(&proto.DeleteMFADeviceRequest{Request: &proto.DeleteMFADeviceRequest_Init{Init: opts.initReq}})
	require.NoError(t, err)

	authChallenge, err := deleteStream.Recv()
	require.NoError(t, err)
	authResp := opts.authHandler(t, authChallenge.GetMFAChallenge())
	err = deleteStream.Send(&proto.DeleteMFADeviceRequest{Request: &proto.DeleteMFADeviceRequest_MFAResponse{MFAResponse: authResp}})
	require.NoError(t, err)

	deleteAck, err := deleteStream.Recv()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(deleteAck.GetAck(), &proto.DeleteMFADeviceResponseAck{}))

	require.NoError(t, deleteStream.CloseSend())
}
