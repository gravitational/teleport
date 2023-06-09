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
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otlpcommonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	otlpresourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	otlptracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestMFADeviceManagement(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)
	clock := srv.Clock().(clockwork.FakeClock)

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	const webOrigin = "https://localhost" // matches RPID above
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(srv.Auth(), "mfa-user", []string{"role"}, nil)
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// No MFA devices should exist for a new user.
	resp, err := cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	require.Empty(t, resp.Devices)

	// Add one device of each kind
	devs := addOneOfEachMFADevice(t, cl, clock, webOrigin)

	// Run AddMFADevice tests, including adding additional devices and failures.
	webKey2, err := mocku2f.Create()
	require.NoError(t, err)
	webKey2.PreferRPID = true
	const webDev2Name = "webauthn2"
	const pwdlessDevName = "pwdless"

	addTests := []struct {
		desc string
		opts mfaAddTestOpts
	}{
		{
			desc: "fail TOTP auth challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "fail-dev",
					DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)

					// Respond to challenge using an unregistered TOTP device,
					// which should fail the auth challenge.
					badDev, err := totp.Generate(totp.GenerateOpts{Issuer: "Teleport", AccountName: user.GetName()})
					require.NoError(t, err)
					code, err := totp.GenerateCode(badDev.Secret(), clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkAuthErr: require.Error,
			},
		},
		{
			desc: "fail a TOTP registration challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "fail-dev",
					DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
				},
				authHandler:  devs.totpAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, req *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					totpRegisterChallenge := req.GetTOTP()
					require.NotEmpty(t, totpRegisterChallenge)
					require.Equal(t, totpRegisterChallenge.Algorithm, otp.AlgorithmSHA1.String())
					// Use the wrong secret for registration, causing server
					// validation to fail.
					code, err := totp.GenerateCodeCustom(base32.StdEncoding.EncodeToString([]byte("wrong-secret")), clock.Now(), totp.ValidateOpts{
						Period:    uint(totpRegisterChallenge.PeriodSeconds),
						Digits:    otp.Digits(totpRegisterChallenge.Digits),
						Algorithm: otp.AlgorithmSHA1,
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{TOTP: &proto.TOTPRegisterResponse{
							Code: code,
						}},
					}
				},
				checkRegisterErr: require.Error,
			},
		},
		{
			desc: "add a second webauthn device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: webDev2Name,
					DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				},
				authHandler:  devs.webAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					ccr, err := webKey2.SignCredentialCreation(webOrigin, wanlib.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: require.NoError,
			},
		},
		{
			desc: "fail a webauthn auth challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "webauthn-1512000",
					DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				},
				authHandler: func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, challenge.WebauthnChallenge) // webauthn enabled

					// Sign challenge with an unknown device.
					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true
					key.IgnoreAllowedCredentials = true
					resp, err := key.SignAssertion(webOrigin, wanlib.CredentialAssertionFromProto(challenge.WebauthnChallenge))
					require.NoError(t, err)
					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
						},
					}
				},
				checkAuthErr: func(t require.TestingT, err error, i ...interface{}) {
					require.Error(t, err)
					require.Equal(t, codes.PermissionDenied, status.Code(err))
				},
			},
		},
		{
			desc: "fail a webauthn registration challenge",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: "webauthn-1512000",
					DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				},
				authHandler:  devs.webAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					require.NotNil(t, challenge.GetWebauthn())

					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true

					ccr, err := key.SignCredentialCreation(
						"http://badorigin.com" /* origin */, wanlib.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)
					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: func(t require.TestingT, err error, i ...interface{}) {
					require.Error(t, err)
					require.Equal(t, codes.InvalidArgument, status.Code(err))
				},
			},
		},
		{
			desc: "add passwordless device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName:  pwdlessDevName,
					DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
					DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
				},
				authHandler:  devs.webAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					require.NotNil(t, challenge.GetWebauthn(), "WebAuthn challenge cannot be nil")

					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true
					key.SetPasswordless()

					ccr, err := key.SignCredentialCreation(webOrigin, wanlib.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: require.NoError,
				assertRegisteredDev: func(t *testing.T, dev *types.MFADevice) {
					// Do a few simple device checks - lib/auth/webauthn goes in depth.
					require.NotNil(t, dev.GetWebauthn(), "WebAuthnDevice cannot be nil")
					require.True(t, true, dev.GetWebauthn().ResidentKey, "ResidentKey should be set to true")
				},
			},
		},
	}
	for _, tt := range addTests {
		t.Run(tt.desc, func(t *testing.T) {
			testAddMFADevice(ctx, t, cl, tt.opts)
		})
	}

	// Check that all new devices are registered.
	resp, err = cl.GetMFADevices(ctx, &proto.GetMFADevicesRequest{})
	require.NoError(t, err)
	deviceNames := make([]string, 0, len(resp.Devices))
	deviceIDs := make(map[string]string)
	for _, dev := range resp.Devices {
		deviceNames = append(deviceNames, dev.GetName())
		deviceIDs[dev.GetName()] = dev.Id
	}
	sort.Strings(deviceNames)
	require.Equal(t, deviceNames, []string{pwdlessDevName, devs.TOTPName, devs.WebName, webDev2Name})

	// Delete several of the MFA devices.
	deleteTests := []struct {
		desc string
		opts mfaDeleteTestOpts
	}{
		{
			desc: "fail to delete an unknown device",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: "unknown-dev",
				},
				authHandler: devs.totpAuthHandler,
				checkErr:    require.Error,
			},
		},
		{
			desc: "fail a TOTP auth challenge",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.TOTPName,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, req.TOTP)

					// Respond to challenge using an unregistered TOTP device,
					// which should fail the auth challenge.
					badDev, err := totp.Generate(totp.GenerateOpts{Issuer: "Teleport", AccountName: user.GetName()})
					require.NoError(t, err)
					code, err := totp.GenerateCode(badDev.Secret(), clock.Now())
					require.NoError(t, err)

					return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{TOTP: &proto.TOTPResponse{
						Code: code,
					}}}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "fail a webauthn auth challenge",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.WebName,
				},
				authHandler: func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					require.NotNil(t, challenge.WebauthnChallenge)

					// Sign challenge with an unknown device.
					key, err := mocku2f.Create()
					require.NoError(t, err)
					key.PreferRPID = true
					key.IgnoreAllowedCredentials = true
					resp, err := key.SignAssertion(webOrigin, wanlib.CredentialAssertionFromProto(challenge.WebauthnChallenge))
					require.NoError(t, err)
					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
						},
					}
				},
				checkErr: require.Error,
			},
		},
		{
			desc: "delete TOTP device by name",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.TOTPName,
				},
				authHandler: devs.totpAuthHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "delete pwdless device by name",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: pwdlessDevName,
				},
				authHandler: devs.webAuthHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "delete webauthn device by name",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.WebName,
				},
				authHandler: devs.webAuthHandler,
				checkErr:    require.NoError,
			},
		},
		{
			desc: "delete webauthn device by ID",
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: deviceIDs[webDev2Name],
				},
				authHandler: func(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					resp, err := webKey2.SignAssertion(
						webOrigin, wanlib.CredentialAssertionFromProto(challenge.WebauthnChallenge))
					require.NoError(t, err)
					return &proto.MFAAuthenticateResponse{
						Response: &proto.MFAAuthenticateResponse_Webauthn{
							Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
						},
					}
				},
				checkErr: require.NoError,
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
	require.Empty(t, resp.Devices)
}

type mfaDevices struct {
	clock     clockwork.Clock
	webOrigin string

	TOTPName, TOTPSecret string
	WebName              string
	WebKey               *mocku2f.Key
}

func (d *mfaDevices) totpAuthHandler(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
	require.NotNil(t, challenge.TOTP)

	if c, ok := d.clock.(clockwork.FakeClock); ok {
		c.Advance(30 * time.Second)
	}
	code, err := totp.GenerateCode(d.TOTPSecret, d.clock.Now())
	require.NoError(t, err)
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{
				Code: code,
			},
		},
	}
}

func (d *mfaDevices) webAuthHandler(t *testing.T, challenge *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
	require.NotNil(t, challenge.WebauthnChallenge)

	resp, err := d.WebKey.SignAssertion(
		d.webOrigin, wanlib.CredentialAssertionFromProto(challenge.WebauthnChallenge))
	require.NoError(t, err)
	return &proto.MFAAuthenticateResponse{
		Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wanlib.CredentialAssertionResponseToProto(resp),
		},
	}
}

func addOneOfEachMFADevice(t *testing.T, cl *Client, clock clockwork.Clock, origin string) mfaDevices {
	const totpName = "totp-dev"
	const webName = "webauthn-dev"
	devs := mfaDevices{
		clock:     clock,
		webOrigin: origin,
		TOTPName:  totpName,
		WebName:   webName,
	}

	var err error
	devs.WebKey, err = mocku2f.Create()
	require.NoError(t, err)
	devs.WebKey.PreferRPID = true

	// Add MFA devices of all kinds.
	ctx := context.Background()
	for _, test := range []struct {
		name string
		opts mfaAddTestOpts
	}{
		{
			name: "TOTP device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: totpName,
					DeviceType: proto.DeviceType_DEVICE_TYPE_TOTP,
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Empty for first device.
					return &proto.MFAAuthenticateResponse{}
				},
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					require.NotEmpty(t, challenge.GetTOTP())
					require.Equal(t, challenge.GetTOTP().Algorithm, otp.AlgorithmSHA1.String())

					devs.TOTPSecret = challenge.GetTOTP().Secret
					code, err := totp.GenerateCodeCustom(devs.TOTPSecret, clock.Now(), totp.ValidateOpts{
						Period:    uint(challenge.GetTOTP().PeriodSeconds),
						Digits:    otp.Digits(challenge.GetTOTP().Digits),
						Algorithm: otp.AlgorithmSHA1,
					})
					require.NoError(t, err)

					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_TOTP{
							TOTP: &proto.TOTPRegisterResponse{
								Code: code,
							},
						},
					}
				},
				checkRegisterErr: require.NoError,
				assertRegisteredDev: func(t *testing.T, got *types.MFADevice) {
					want, err := services.NewTOTPDevice(totpName, devs.TOTPSecret, clock.Now())
					want.Id = got.Id
					require.NoError(t, err)
					require.Empty(t, cmp.Diff(want, got))
				},
			},
		},
		{
			name: "Webauthn device",
			opts: mfaAddTestOpts{
				initReq: &proto.AddMFADeviceRequestInit{
					DeviceName: webName,
					DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
				},
				authHandler:  devs.totpAuthHandler,
				checkAuthErr: require.NoError,
				registerHandler: func(t *testing.T, challenge *proto.MFARegisterChallenge) *proto.MFARegisterResponse {
					require.NotNil(t, challenge.GetWebauthn())

					ccr, err := devs.WebKey.SignCredentialCreation(origin, wanlib.CredentialCreationFromProto(challenge.GetWebauthn()))
					require.NoError(t, err)
					return &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
						},
					}
				},
				checkRegisterErr: require.NoError,
				assertRegisteredDev: func(t *testing.T, got *types.MFADevice) {
					// MFADevice device asserted in its entirety by lib/auth/webauthn
					// tests, a simple check suffices here.
					require.Equal(t, devs.WebKey.KeyHandle, got.GetWebauthn().CredentialId)
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			testAddMFADevice(ctx, t, cl, test.opts)
		})
	}
	return devs
}

type mfaAddTestOpts struct {
	initReq             *proto.AddMFADeviceRequestInit
	authHandler         func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkAuthErr        require.ErrorAssertionFunc
	registerHandler     func(*testing.T, *proto.MFARegisterChallenge) *proto.MFARegisterResponse
	checkRegisterErr    require.ErrorAssertionFunc
	assertRegisteredDev func(*testing.T, *types.MFADevice)
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
	opts.checkAuthErr(t, err)
	if err != nil {
		return
	}
	registerResp := opts.registerHandler(t, registerChallenge.GetNewMFARegisterChallenge())
	err = addStream.Send(&proto.AddMFADeviceRequest{Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{NewMFARegisterResponse: registerResp}})
	require.NoError(t, err)

	registerAck, err := addStream.Recv()
	opts.checkRegisterErr(t, err)
	if err != nil {
		return
	}
	if opts.assertRegisteredDev != nil {
		opts.assertRegisteredDev(t, registerAck.GetAck().GetDevice())
	}

	require.NoError(t, addStream.CloseSend())
}

type mfaDeleteTestOpts struct {
	initReq     *proto.DeleteMFADeviceRequestInit
	authHandler func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	checkErr    require.ErrorAssertionFunc
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
	opts.checkErr(t, err)
	if err != nil {
		return
	}
	deleted := deleteAck.GetAck().GetDevice()
	require.NotNil(t, deleted, "deleted device in ack message is nil")
	require.NotEmpty(t, deleted.Id, "deleted device.Id in ack message is empty")
	require.NotEmpty(t, deleted.GetName(), "deleted device.Name in ack message is empty")
	// opts.initReq.DeviceName can be either the device name or ID, so check if
	// either matches the deleted device.
	wantName := []string{
		deleted.Id,
		deleted.GetName(),
	}
	require.Contains(t, wantName, opts.initReq.DeviceName)
	require.NoError(t, deleteStream.CloseSend())
}

func TestDeleteLastMFADevice(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Enable MFA support.
	authSpec := &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	}
	authPref, err := types.NewAuthPreference(*authSpec)

	const webOrigin = "https://localhost" // matches RPID above
	require.NoError(t, err)
	auth := srv.Auth()
	err = auth.SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a fake user.
	user, _, err := CreateUserAndRole(auth, "mfa-user", []string{"role"}, nil)
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Add devices
	devs := addOneOfEachMFADevice(t, cl, srv.Clock(), webOrigin)

	tests := []struct {
		name         string
		secondFactor constants.SecondFactorType
		opts         mfaDeleteTestOpts
	}{
		{
			name:         "NOK sf=OTP trying to delete last OTP device",
			secondFactor: constants.SecondFactorOTP,
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.TOTPName,
				},
				authHandler: devs.totpAuthHandler,
				checkErr:    require.Error,
			},
		},
		{
			name:         "NOK sf=Webauthn trying to delete last Webauthn device",
			secondFactor: constants.SecondFactorWebauthn,
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.WebName,
				},
				authHandler: devs.webAuthHandler,
				checkErr:    require.Error,
			},
		},
		{
			name:         "OK delete OTP device",
			secondFactor: constants.SecondFactorOn,
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.TOTPName,
				},
				authHandler: devs.totpAuthHandler,
				checkErr:    require.NoError,
			},
		},
		{
			name:         "NOK sf=on trying to delete last MFA device",
			secondFactor: constants.SecondFactorOn,
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.WebName,
				},
				authHandler: devs.webAuthHandler,
				checkErr:    require.Error,
			},
		},
		{
			name:         "OK sf=optional delete last device (webauthn)",
			secondFactor: constants.SecondFactorOptional,
			opts: mfaDeleteTestOpts{
				initReq: &proto.DeleteMFADeviceRequestInit{
					DeviceName: devs.WebName,
				},
				authHandler: devs.webAuthHandler,
				checkErr:    require.NoError,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Update second factor settings, if necessary.
			cap, err := auth.GetAuthPreference(ctx)
			require.NoError(t, err)
			if cap.GetSecondFactor() != test.secondFactor {
				authSpec.SecondFactor = test.secondFactor
				newCAP, err := types.NewAuthPreference(*authSpec)
				require.NoError(t, err)
				require.NoError(t, auth.SetAuthPreference(ctx, newCAP))
			}

			testDeleteMFADevice(ctx, t, cl, test.opts)
		})
	}
}

func TestCreateAppSession_deviceExtensions(t *testing.T) {
	ctx := context.Background()
	testServer := newTestTLSServer(t)
	authServer := testServer.Auth()

	// Create an user for testing.
	user, _, err := CreateUserAndRole(authServer, "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")

	// Register an application.
	app, err := types.NewAppV3(
		types.Metadata{
			Name: "llamaapp",
		}, types.AppSpecV3{
			URI:        "http://localhost:8080",
			PublicAddr: "llamaapp.example.com",
		})
	require.NoError(t, err, "NewAppV3 failed")
	appServer, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err, "NewAppServerV3FromApp failed")
	_, err = authServer.UpsertApplicationServer(ctx, appServer)
	require.NoError(t, err, "UpsertApplicationServer failed")

	wantExtensions := &tlsca.DeviceExtensions{
		DeviceID:     "device1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	tests := []struct {
		name       string
		modifyUser func(u *TestIdentity)
		assertCert func(t *testing.T, cert *x509.Certificate)
	}{
		{
			name: "no device extensions",
			// Absence of errors is enough here, this is mostly to make sure the base
			// scenario works.
		},
		{
			name: "user with device extensions",
			modifyUser: func(u *TestIdentity) {
				lu := u.I.(authz.LocalUser)
				lu.Identity.DeviceExtensions = *wantExtensions
				u.I = lu
			},
			assertCert: func(t *testing.T, cert *x509.Certificate) {
				gotIdentity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
				require.NoError(t, err, "FromSubject failed")

				if diff := cmp.Diff(*wantExtensions, gotIdentity.DeviceExtensions, protocmp.Transform()); diff != "" {
					t.Errorf("DeviceExtensions mismatch (-want +got)\n%s", diff)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := TestUser(user.GetName())
			if test.modifyUser != nil {
				test.modifyUser(&u)
			}

			userClient, err := testServer.NewClient(u)
			require.NoError(t, err, "NewClient failed")

			session, err := userClient.CreateAppSession(ctx, types.CreateAppSessionRequest{
				Username:    user.GetName(),
				PublicAddr:  app.GetPublicAddr(),
				ClusterName: testServer.ClusterName(),
			})
			require.NoError(t, err, "CreateAppSession failed")

			block, _ := pem.Decode(session.GetTLSCert())
			require.NotNil(t, block, "Decode failed")
			gotCert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err, "ParserCertificate failed")

			if test.assertCert != nil {
				test.assertCert(t, gotCert)
			}
		})
	}
}

func TestGenerateUserCerts_deviceExtensions(t *testing.T) {
	ctx := context.Background()
	testServer := newTestTLSServer(t)

	// Create an user for testing.
	user, _, err := CreateUserAndRole(testServer.Auth(), "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")

	_, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err, "GenerateKeyPair failed")

	wantExtensions := &tlsca.DeviceExtensions{
		DeviceID:     "device1",
		AssetTag:     "assettag1",
		CredentialID: "credentialid1",
	}

	tests := []struct {
		name       string
		modifyUser func(u *TestIdentity)
		assertCert func(t *testing.T, cert *x509.Certificate)
	}{
		{
			name: "no device extensions",
			// Absence of errors is enough here, this is mostly to make sure the base
			// scenario works.
		},
		{
			name: "user with device extensions",
			modifyUser: func(u *TestIdentity) {
				lu := u.I.(authz.LocalUser)
				lu.Identity.DeviceExtensions = *wantExtensions
				u.I = lu
			},
			assertCert: func(t *testing.T, cert *x509.Certificate) {
				gotIdentity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
				require.NoError(t, err, "FromSubject failed")

				if diff := cmp.Diff(*wantExtensions, gotIdentity.DeviceExtensions, protocmp.Transform()); diff != "" {
					t.Errorf("DeviceExtensions mismatch (-want +got)\n%s", diff)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := TestUser(user.GetName())
			if test.modifyUser != nil {
				test.modifyUser(&u)
			}

			userClient, err := testServer.NewClient(u)
			require.NoError(t, err, "NewClient failed")

			resp, err := userClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
				PublicKey: pub,
				Username:  user.GetName(),
				Expires:   testServer.Clock().Now().Add(1 * time.Hour),
			})
			require.NoError(t, err, "GenerateUserCerts failed")

			block, _ := pem.Decode(resp.TLS)
			require.NotNil(t, block, "Decode failed")
			gotCert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err, "ParserCertificate failed")

			if test.assertCert != nil {
				test.assertCert(t, gotCert)
			}
		})
	}
}

func TestGenerateUserCerts_deviceAuthz(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise, // required for Device Trust.
	})

	testServer := newTestTLSServer(t)

	ctx := context.Background()
	clock := testServer.Clock()
	clusterName := testServer.ClusterName()
	authServer := testServer.Auth()

	// Create a user for testing.
	user, role, err := CreateUserAndRole(testServer.Auth(), "llama", []string{"llama"}, nil)
	require.NoError(t, err, "CreateUserAndRole failed")
	username := user.GetName()

	// Make sure MFA is required for this user.
	roleOpt := role.GetOptions()
	roleOpt.RequireMFAType = types.RequireMFAType_SESSION
	role.SetOptions(roleOpt)

	err = authServer.UpsertRole(ctx, role)
	require.NoError(t, err)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "mynode",
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err = authServer.UpsertNode(ctx, node)
	require.NoError(t, err)

	// Create clients with and without device extensions.
	clientWithoutDevice, err := testServer.NewClient(TestUser(username))
	require.NoError(t, err, "NewClient failed")

	clientWithDevice, err := testServer.NewClient(
		TestUserWithDeviceExtensions(username, tlsca.DeviceExtensions{
			DeviceID:     "deviceid1",
			AssetTag:     "assettag1",
			CredentialID: "credentialid1",
		}))
	require.NoError(t, err, "NewClient failed")

	// updateAuthPref is a helper used throughout the test.
	updateAuthPref := func(t *testing.T, modify func(ap types.AuthPreference)) {
		authPref, err := authServer.GetAuthPreference(ctx)
		require.NoError(t, err, "GetAuthPreference failed")

		modify(authPref)

		require.NoError(t,
			authServer.SetAuthPreference(ctx, authPref),
			"SetAuthPreference failed")
	}

	// Register MFA devices for the user.
	// Required to issue certificates with MFA.
	const rpID = "localhost"
	const origin = "https://" + rpID + ":3080" // matches RPID.
	updateAuthPref(t, func(authPref types.AuthPreference) {
		authPref.SetSecondFactor(constants.SecondFactorOptional)
		authPref.SetWebauthn(&types.Webauthn{
			RPID: "localhost",
		})
	})
	mfaDevices := addOneOfEachMFADevice(t, clientWithoutDevice, clock, origin)

	// Create a public key for UserCertsRequest.
	_, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err, "GenerateKeyPair failed")

	expires := clock.Now().Add(1 * time.Hour)
	sshReq := proto.UserCertsRequest{
		PublicKey:      pub,
		Username:       username,
		Expires:        expires,
		RouteToCluster: clusterName,
		NodeName:       "mynode",
		Usage:          proto.UserCertsRequest_SSH,
		SSHLogin:       "llama",
	}
	appReq := proto.UserCertsRequest{
		PublicKey:      pub,
		Username:       username,
		Expires:        expires,
		RouteToCluster: clusterName,
		Usage:          proto.UserCertsRequest_App,
		RouteToApp: proto.RouteToApp{
			Name:        "hello",
			SessionID:   "mysessionid",
			PublicAddr:  "hello.cluster.dev",
			ClusterName: clusterName,
		},
	}
	winReq := proto.UserCertsRequest{
		PublicKey:      pub,
		Username:       username,
		Expires:        expires,
		RouteToCluster: clusterName,
		Usage:          proto.UserCertsRequest_WindowsDesktop,
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: "mydesktop",
			Login:          username,
		},
	}

	assertSuccess := func(t *testing.T, err error) {
		assert.NoError(t, err, "GenerateUserCerts error mismatch")
	}
	assertAccessDenied := func(t *testing.T, err error) {
		assert.True(t, trace.IsAccessDenied(err), "GenerateUserCerts error mismatch, got=%v (%T), want trace.AccessDeniedError", err, err)
	}

	// generateCertsMFA is used to run the MFA-aware, streaming certificate
	// issuance ceremony.
	generateCertsMFA := func(t *testing.T, client *Client, req proto.UserCertsRequest) (cert *proto.SingleUseUserCert, err error) {
		defer func() {
			// Translate gRPC to trace errors, as our clients do.
			err = trail.FromGRPC(err)
		}()

		stream, err := client.GenerateUserSingleUseCerts(ctx)
		if err != nil {
			return nil, err
		}

		if err := stream.Send(&proto.UserSingleUseCertsRequest{
			Request: &proto.UserSingleUseCertsRequest_Init{
				Init: &req,
			},
		}); err != nil {
			return nil, err
		}
		resp, err := stream.Recv()
		if err != nil {
			return nil, err
		}
		mfaResp := mfaDevices.webAuthHandler(t, resp.GetMFAChallenge())

		if err := stream.Send(&proto.UserSingleUseCertsRequest{
			Request: &proto.UserSingleUseCertsRequest_MFAResponse{
				MFAResponse: mfaResp,
			},
		}); err != nil {
			return nil, err
		}
		resp, err = stream.Recv()
		if err != nil {
			return nil, err
		}
		cert = resp.GetCert()
		if cert == nil {
			return nil, fmt.Errorf("unexpected response, cert is nil: %v", resp)
		}
		return cert, nil
	}

	tests := []struct {
		name                     string
		clusterDeviceMode        string
		client                   *Client
		req                      proto.UserCertsRequest
		skipGenerateUserCertsRPC bool // aka non-MFA issuance.
		skipSingleUseCertsRPC    bool // aka MFA/streaming issuance.
		assertErr                func(t *testing.T, err error)
	}{
		{
			name:              "mode=optional without extensions",
			clusterDeviceMode: constants.DeviceTrustModeOptional,
			client:            clientWithoutDevice,
			req:               sshReq,
			assertErr:         assertSuccess,
		},
		{
			name:              "mode=optional with extensions",
			clusterDeviceMode: constants.DeviceTrustModeOptional,
			client:            clientWithDevice,
			req:               sshReq,
			assertErr:         assertSuccess,
		},
		{
			name:              "nok: mode=required without extensions",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithoutDevice,
			req:               sshReq,
			assertErr:         assertAccessDenied,
		},
		{
			name:              "mode=required with extensions",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithDevice,
			req:               sshReq,
			assertErr:         assertSuccess,
		},
		{
			name:                  "mode=required ignores App Access requests (non-MFA)",
			clusterDeviceMode:     constants.DeviceTrustModeRequired,
			client:                clientWithoutDevice,
			req:                   appReq,
			skipSingleUseCertsRPC: true,
			assertErr:             assertSuccess,
		},
		{
			// Tracked here because, if this changes, then the scenario should be the
			// same as the one above.
			name:                     "GenerateUserSingleUseCerts does not allow App usage",
			clusterDeviceMode:        constants.DeviceTrustModeRequired,
			client:                   clientWithoutDevice,
			req:                      appReq,
			skipGenerateUserCertsRPC: true,
			assertErr: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "app access certificates", "GenerateUserSingleUseCerts expected to fail for usage=App")
			},
		},
		{
			name:              "mode=required ignores Desktop Access requests",
			clusterDeviceMode: constants.DeviceTrustModeRequired,
			client:            clientWithoutDevice,
			req:               winReq,
			assertErr:         assertSuccess,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			updateAuthPref(t, func(ap types.AuthPreference) {
				ap.SetDeviceTrust(&types.DeviceTrust{
					Mode: test.clusterDeviceMode,
				})
			})

			// Test the unary, non-MFA endpoint.
			if !test.skipGenerateUserCertsRPC {
				t.Run("GenerateUserCerts", func(t *testing.T) {
					_, err := test.client.GenerateUserCerts(ctx, test.req)
					test.assertErr(t, err)
				})
			}

			// Test the streaming, MFA-aware endpoint.
			if !test.skipSingleUseCertsRPC {
				t.Run("GenerateUserSingleUseCerts", func(t *testing.T) {
					_, err := generateCertsMFA(t, test.client, test.req)
					test.assertErr(t, err)
				})
			}
		})
	}
}

func TestGenerateUserSingleUseCert(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise, // required for IP pinning.
		TestFeatures:  modules.GetModules().Features(),
	})

	ctx := context.Background()
	srv := newTestTLSServer(t)
	clock := srv.Clock()
	userCertTTL := 12 * time.Hour
	userCertExpires := clock.Now().Add(userCertTTL)

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	const webOrigin = "https://localhost" // matches RPID above
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: "node-a",
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	kube, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: "kube-a",
	}, types.KubernetesClusterSpecV3{})

	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "kube-a", "kube-a")
	require.NoError(t, err)
	_, err = srv.Auth().UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)
	// Register a database.
	db, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "db-a",
	}, types.DatabaseServerSpecV3{
		Protocol: "postgres",
		URI:      "localhost",
		Hostname: "localhost",
		HostID:   "localhost",
	})
	require.NoError(t, err)

	_, err = srv.Auth().UpsertDatabaseServer(ctx, db)
	require.NoError(t, err)

	desktop, err := types.NewWindowsDesktopV3("desktop", nil, types.WindowsDesktopSpecV3{
		Addr:   "localhost",
		HostID: "test",
	})
	require.NoError(t, err)

	require.NoError(t, srv.Auth().CreateWindowsDesktop(ctx, desktop))

	leaf, err := types.NewRemoteCluster("leaf")
	require.NoError(t, err)

	// create remote cluster
	require.NoError(t, srv.Auth().CreateRemoteCluster(leaf))

	// Create a fake user.
	user, role, err := CreateUserAndRole(srv.Auth(), "mfa-user", []string{"role"}, nil)
	require.NoError(t, err)
	// Make sure MFA is required for this user.
	roleOpt := role.GetOptions()
	roleOpt.RequireMFAType = types.RequireMFAType_SESSION
	role.SetDatabaseUsers(types.Allow, []string{types.Wildcard})
	role.SetDatabaseLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	role.SetDatabaseNames(types.Allow, []string{types.Wildcard})
	role.SetWindowsLogins(types.Allow, []string{"role"})
	role.SetWindowsDesktopLabels(types.Allow, types.Labels{types.Wildcard: {types.Wildcard}})
	role.SetOptions(roleOpt)
	err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)
	testUser := TestUser(user.GetName())
	testUser.TTL = userCertTTL
	cl, err := srv.NewClient(testUser)
	require.NoError(t, err)

	// Register MFA devices for the fake user.
	registered := addOneOfEachMFADevice(t, cl, clock, webOrigin)

	// Fetch MFA device IDs.
	devs, err := srv.Auth().Services.GetMFADevices(ctx, user.GetName(), false)
	require.NoError(t, err)
	var webDevID string
	for _, dev := range devs {
		if dev.GetWebauthn() != nil {
			webDevID = dev.Id
			break
		}
	}

	_, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	// Used for device trust tests.
	wantDeviceExtensions := tlsca.DeviceExtensions{
		DeviceID:     "device-id1",
		AssetTag:     "device-assettag1",
		CredentialID: "device-credentialid1",
	}

	tests := []struct {
		desc      string
		newClient func() (*Client, error) // optional, makes a new client for the test.
		opts      generateUserSingleUseCertTestOpts
	}{
		{
			desc: "ssh using webauthn",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:  clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:    proto.UserCertsRequest_SSH,
					NodeName: "node-a",
					SSHLogin: "role",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					sshCertBytes := c.GetSSH()
					require.NotEmpty(t, sshCertBytes)

					cert, err := sshutils.ParseCertificate(sshCertBytes)
					require.NoError(t, err)

					require.Equal(t, webDevID, cert.Extensions[teleport.CertExtensionMFAVerified])
					require.Equal(t, userCertExpires.Format(time.RFC3339), cert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionLoginIP]).IsLoopback())
					require.Equal(t, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()), cert.ValidBefore)
				},
			},
		},
		{
			desc: "ssh - adjusted expiry",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:  clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:    proto.UserCertsRequest_SSH,
					NodeName: "node-a",
					SSHLogin: "role",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetSSH()
					require.NotEmpty(t, crt)

					cert, err := sshutils.ParseCertificate(crt)
					require.NoError(t, err)

					require.Equal(t, webDevID, cert.Extensions[teleport.CertExtensionMFAVerified])
					require.Equal(t, userCertExpires.Format(time.RFC3339), cert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionLoginIP]).IsLoopback())
					require.Equal(t, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()), cert.ValidBefore)
				},
			},
		},
		{
			desc: "k8s",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:           clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-a",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageKubeOnly}, identity.Usage)
					require.Equal(t, "kube-a", identity.KubernetesCluster)
				},
			},
		},
		{
			desc: "db",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
						Database:    "db-a",
					},
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, clock.Now().Add(teleport.UserSingleUseCertTTL), cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
					require.Equal(t, identity.RouteToDatabase.ServiceName, "db-a")
				},
			},
		},
		{
			desc: "db with ttl limit disabled",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry should *not* be adjusted to single user cert TTL,
					// since ttl limiting is disabled when requester is a local proxy tunnel.
					// It *should* be adjusted to the user cert ttl though.
					Expires: clock.Now().Add(1000 * time.Hour),
					Usage:   proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
					},
					RequesterName: proto.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL,
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, userCertExpires, cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
					require.Equal(t, identity.RouteToDatabase.ServiceName, "db-a")
				},
			},
		},
		{
			desc: "kube with ttl limit disabled",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry should *not* be adjusted to single user cert TTL,
					// since ttl limiting is disabled when requester is a local proxy.
					// It *should* be adjusted to the user cert ttl though.
					Expires:           clock.Now().Add(1000 * time.Hour),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-a",
					RequesterName:     proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY,
				},
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				checkInitErr: require.NoError,
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, userCertExpires, cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageKubeOnly}, identity.Usage)
					require.Equal(t, identity.KubernetesCluster, "kube-a")
				},
			},
		},
		{
			desc: "desktops",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_WindowsDesktop,
					RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
						WindowsDesktop: "desktop",
						Login:          "role",
					},
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageWindowsDesktopOnly}, identity.Usage)
				},
			},
		},
		{
			desc: "fail - wrong usage",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_All,
					NodeName:  "node-a",
				},
				checkInitErr: require.Error,
			},
		},
		{
			desc: "fail - mfa challenge fail",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_SSH,
					NodeName:  "node-a",
					SSHLogin:  "role",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Return no challenge response.
					return &proto.MFAAuthenticateResponse{}
				},
				checkAuthErr: require.Error,
			},
		},
		{
			desc: "device extensions copied SSH cert",
			newClient: func() (*Client, error) {
				u := TestUser(user.GetName())
				u.TTL = 1 * time.Hour

				// Add device extensions to the fake user's identity.
				localUser := u.I.(authz.LocalUser)
				localUser.Identity.DeviceExtensions = wantDeviceExtensions
				u.I = localUser

				return srv.NewClient(u)
			},
			opts: generateUserSingleUseCertTestOpts{
				// Same as SSH options. Nothing special here.
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_SSH,
					NodeName:  "node-a",
					SSHLogin:  "role",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					// SSH certificate.
					sshRaw := c.GetSSH()
					require.NotEmpty(t, sshRaw, "Got empty single-use SSH certificate")

					sshCert, err := sshutils.ParseCertificate(sshRaw)
					require.NoError(t, err, "ParseCertificate failed")

					gotSSH := tlsca.DeviceExtensions{
						DeviceID:     sshCert.Extensions[teleport.CertExtensionDeviceID],
						AssetTag:     sshCert.Extensions[teleport.CertExtensionDeviceAssetTag],
						CredentialID: sshCert.Extensions[teleport.CertExtensionDeviceCredentialID],
					}
					if diff := cmp.Diff(wantDeviceExtensions, gotSSH, protocmp.Transform()); diff != "" {
						t.Errorf("SSH DeviceExtensions mismatch (-want +got)\n%s", diff)
					}
				},
			},
		},
		{
			desc: "device extensions copied TLS cert",
			newClient: func() (*Client, error) {
				u := TestUser(user.GetName())
				u.TTL = 1 * time.Hour

				// Add device extensions to the fake user's identity.
				localUser := u.I.(authz.LocalUser)
				localUser.Identity.DeviceExtensions = wantDeviceExtensions
				u.I = localUser

				return srv.NewClient(u)
			},
			opts: generateUserSingleUseCertTestOpts{
				// Same as Database options. Nothing special here.
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-a",
					},
				},
				checkInitErr: require.NoError,
				authHandler:  registered.webAuthHandler,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_YES, required)
				},
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					// TLS certificate.
					tlsRaw := c.GetTLS()
					require.NotEmpty(t, tlsRaw, "Got empty single-use TLS certificate")

					block, _ := pem.Decode(tlsRaw)
					require.NotNil(t, block, "Decode failed (TLS PEM)")
					tlsCert, err := x509.ParseCertificate(block.Bytes)
					require.NoError(t, err, "ParseCertificate failed")

					singleUseIdentity, err := tlsca.FromSubject(tlsCert.Subject, tlsCert.NotAfter)
					require.NoError(t, err, "FromSubject failed")
					gotTLS := singleUseIdentity.DeviceExtensions
					if diff := cmp.Diff(wantDeviceExtensions, gotTLS, protocmp.Transform()); diff != "" {
						t.Errorf("TLS DeviceExtensions mismatch (-want +got)\n%s", diff)
					}
				},
			},
		},
		{
			desc: "fail - mfa not required when RBAC prevents access",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_SSH,
					NodeName:  "node-a",
					SSHLogin:  "llama", // not an allowed login which prevents access
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_NO, required)
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Return no challenge response.
					return &proto.MFAAuthenticateResponse{}
				},
				checkAuthErr: func(t require.TestingT, err error, i ...interface{}) {
					require.ErrorIs(t, err, io.EOF, i...)
				},
			},
		},
		{
			desc: "mfa unspecified when no SSHLogin provided",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_SSH,
					NodeName:  "node-a",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authHandler: func(t *testing.T, req *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse {
					// Return no challenge response.
					return &proto.MFAAuthenticateResponse{}
				},
				checkAuthErr: require.Error,
			},
		},
		{
			desc: "k8s in leaf cluster",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:           clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:             proto.UserCertsRequest_Kubernetes,
					KubernetesCluster: "kube-b",
					RouteToCluster:    "leaf",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, cert.NotAfter, clock.Now().Add(teleport.UserSingleUseCertTTL))

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageKubeOnly}, identity.Usage)
					require.Equal(t, "kube-b", identity.KubernetesCluster)
				},
			},
		},
		{
			desc: "db in leaf cluster",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires: clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:   proto.UserCertsRequest_Database,
					RouteToDatabase: proto.RouteToDatabase{
						ServiceName: "db-b",
						Database:    "db-b",
					},
					RouteToCluster: "leaf",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					crt := c.GetTLS()
					require.NotEmpty(t, crt)

					cert, err := tlsca.ParseCertificatePEM(crt)
					require.NoError(t, err)
					require.Equal(t, clock.Now().Add(teleport.UserSingleUseCertTTL), cert.NotAfter)

					identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
					require.NoError(t, err)
					require.Equal(t, webDevID, identity.MFAVerified)
					require.Equal(t, userCertExpires, identity.PreviousIdentityExpires)
					require.True(t, net.ParseIP(identity.LoginIP).IsLoopback())
					require.Equal(t, []string{teleport.UsageDatabaseOnly}, identity.Usage)
					require.Equal(t, identity.RouteToDatabase.ServiceName, "db-b")
				},
			},
		},
		{
			desc: "ssh in leaf node",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					// This expiry is longer than allowed, should be
					// automatically adjusted.
					Expires:        clock.Now().Add(2 * teleport.UserSingleUseCertTTL),
					Usage:          proto.UserCertsRequest_SSH,
					NodeName:       "node-b",
					SSHLogin:       "role",
					RouteToCluster: "leaf",
				},
				checkInitErr: require.NoError,
				mfaRequiredHandler: func(t *testing.T, required proto.MFARequired) {
					require.Equal(t, proto.MFARequired_MFA_REQUIRED_UNSPECIFIED, required)
				},
				authHandler:  registered.webAuthHandler,
				checkAuthErr: require.NoError,
				validateCert: func(t *testing.T, c *proto.SingleUseUserCert) {
					sshCertBytes := c.GetSSH()
					require.NotEmpty(t, sshCertBytes)

					cert, err := sshutils.ParseCertificate(sshCertBytes)
					require.NoError(t, err)

					require.Equal(t, webDevID, cert.Extensions[teleport.CertExtensionMFAVerified])
					require.Equal(t, userCertExpires.Format(time.RFC3339), cert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
					require.True(t, net.ParseIP(cert.Extensions[teleport.CertExtensionLoginIP]).IsLoopback())
					require.Equal(t, uint64(clock.Now().Add(teleport.UserSingleUseCertTTL).Unix()), cert.ValidBefore)
				},
			},
		},
		{
			desc: "fail - app access not supported",
			opts: generateUserSingleUseCertTestOpts{
				initReq: &proto.UserCertsRequest{
					PublicKey: pub,
					Username:  user.GetName(),
					Expires:   clock.Now().Add(teleport.UserSingleUseCertTTL),
					Usage:     proto.UserCertsRequest_App,
				},
				checkInitErr: require.Error,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			testClient := cl
			if tt.newClient != nil {
				var err error
				testClient, err = tt.newClient()
				require.NoError(t, err, "newClient failed")
			}

			testGenerateUserSingleUseCert(ctx, t, testClient, tt.opts)
		})
	}
}

type generateUserSingleUseCertTestOpts struct {
	initReq            *proto.UserCertsRequest
	checkInitErr       require.ErrorAssertionFunc
	authHandler        func(*testing.T, *proto.MFAAuthenticateChallenge) *proto.MFAAuthenticateResponse
	mfaRequiredHandler func(*testing.T, proto.MFARequired)
	checkAuthErr       require.ErrorAssertionFunc
	validateCert       func(*testing.T, *proto.SingleUseUserCert)
}

func testGenerateUserSingleUseCert(ctx context.Context, t *testing.T, cl *Client, opts generateUserSingleUseCertTestOpts) {
	stream, err := cl.GenerateUserSingleUseCerts(ctx)
	require.NoError(t, err)
	err = stream.Send(&proto.UserSingleUseCertsRequest{Request: &proto.UserSingleUseCertsRequest_Init{Init: opts.initReq}})
	require.NoError(t, err)

	authChallenge, err := stream.Recv()
	opts.checkInitErr(t, err)
	if err != nil {
		return
	}

	challenge := authChallenge.GetMFAChallenge()
	opts.mfaRequiredHandler(t, challenge.MFARequired)

	authResp := opts.authHandler(t, challenge)
	err = stream.Send(&proto.UserSingleUseCertsRequest{Request: &proto.UserSingleUseCertsRequest_MFAResponse{MFAResponse: authResp}})
	if challenge.MFARequired == proto.MFARequired_MFA_REQUIRED_NO && err != nil {
		require.ErrorIs(t, err, io.EOF, "Want the server to close the stream when MFA is not required")
		return
	} else {
		require.NoError(t, err)
	}

	certs, err := stream.Recv()
	opts.checkAuthErr(t, err)
	if err != nil {
		return
	}
	opts.validateCert(t, certs.GetCert())

	require.NoError(t, stream.CloseSend())
}

var requireMFATypes = []types.RequireMFAType{
	types.RequireMFAType_OFF,
	types.RequireMFAType_SESSION,
	types.RequireMFAType_SESSION_AND_HARDWARE_KEY,
	types.RequireMFAType_HARDWARE_KEY_TOUCH,
}

func TestIsMFARequired(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Register an SSH node.
	node := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: uuid.NewString(),
		},
		Spec: types.ServerSpecV2{
			Hostname: "node-a",
		},
	}
	_, err := srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	// Create a fake user.
	user, role, err := CreateUserAndRole(srv.Auth(), "no-mfa-user", []string{"no-mfa-user"}, nil)
	require.NoError(t, err)

	for _, authPrefRequireMFAType := range requireMFATypes {
		authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
			Type:           constants.Local,
			SecondFactor:   constants.SecondFactorOptional,
			RequireMFAType: authPrefRequireMFAType,
			Webauthn: &types.Webauthn{
				RPID: "teleport",
			},
		})
		require.NoError(t, err)
		err = srv.Auth().SetAuthPreference(ctx, authPref)
		require.NoError(t, err)

		for _, roleRequireMFAType := range requireMFATypes {
			// If role or auth pref have "hardware_key_touch", expect not required.
			expectRequired := !(roleRequireMFAType == types.RequireMFAType_HARDWARE_KEY_TOUCH || authPrefRequireMFAType == types.RequireMFAType_HARDWARE_KEY_TOUCH)
			// Otherwise, if auth pref or role require session MFA, expect required.
			expectRequired = expectRequired && (roleRequireMFAType.IsSessionMFARequired() || authPrefRequireMFAType.IsSessionMFARequired())

			t.Run(fmt.Sprintf("authPref=%v/role=%v/expect=%v", authPrefRequireMFAType, roleRequireMFAType, expectRequired), func(t *testing.T) {
				roleOpt := role.GetOptions()
				roleOpt.RequireMFAType = roleRequireMFAType
				role.SetOptions(roleOpt)
				err = srv.Auth().UpsertRole(ctx, role)
				require.NoError(t, err)

				cl, err := srv.NewClient(TestUser(user.GetName()))
				require.NoError(t, err)

				resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
					Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
						Login: user.GetName(),
						Node:  "node-a",
					}},
				})
				require.NoError(t, err)
				require.Equal(t, expectRequired, resp.Required)
			})
		}
	}
}

func TestIsMFARequiredUnauthorized(t *testing.T) {
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Enable MFA support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "teleport",
		},
	})
	require.NoError(t, err)
	err = srv.Auth().SetAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Register an SSH node.
	node1 := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node1",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"a": "b"},
		},
		Spec: types.ServerSpecV2{
			Hostname: "node1",
			Addr:     "localhost:3022",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node1)
	require.NoError(t, err)

	// Register another SSH node with a duplicate hostname.
	node2 := &types.ServerV2{
		Kind:    types.KindNode,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "node2",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{"a": "c"},
		},
		Spec: types.ServerSpecV2{
			Hostname: "node1",
			Addr:     "localhost:3022",
		},
	}
	_, err = srv.Auth().UpsertNode(ctx, node2)
	require.NoError(t, err)

	user, role, err := CreateUserAndRole(srv.Auth(), "alice", []string{"alice"}, nil)
	require.NoError(t, err)

	// Require MFA.
	roleOpt := role.GetOptions()
	roleOpt.RequireMFAType = types.RequireMFAType_SESSION
	role.SetOptions(roleOpt)
	role.SetNodeLabels(types.Allow, map[string]utils.Strings{"a": []string{"c"}})
	err = srv.Auth().UpsertRole(ctx, role)
	require.NoError(t, err)

	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	// Call the endpoint for an authorized login. The user is only authorized
	// for the 2nd node, but should still be asked for MFA.
	resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
			Login: "alice",
			Node:  "node1",
		}},
	})
	require.NoError(t, err)
	require.True(t, resp.Required)

	// Call the endpoint for an unauthorized login.
	resp, err = cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
		Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
			Login: "bob",
			Node:  "node1",
		}},
	})

	// When unauthorized, expect a silent `false`.
	require.NoError(t, err)
	require.False(t, resp.Required)
}

func TestIsMFARequired_NodeMatch(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Register an SSH node.
	node, err := types.NewServerWithLabels(uuid.NewString(), types.KindNode, types.ServerSpecV2{
		Hostname:    "node-a",
		Addr:        "127.0.0.1:3022",
		PublicAddrs: []string{"node.example.com:3022", "localhost:3022"},
	}, map[string]string{"foo": "bar"})
	require.NoError(t, err)
	_, err = srv.Auth().UpsertNode(ctx, node)
	require.NoError(t, err)

	// Create a fake user with per session mfa required for all nodes.
	role, err := CreateRole(ctx, srv.Auth(), "mfa-user", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			Logins:     []string{"mfa-user"},
			NodeLabels: types.Labels{types.Wildcard: utils.Strings{types.Wildcard}},
		},
	})
	require.NoError(t, err)

	user, err := CreateUser(srv.Auth(), "mfa-user", role)
	require.NoError(t, err)

	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, tc := range []struct {
		desc string
		// IsMFARequired only expects a host name or ip without the port.
		node        string
		expectMatch require.BoolAssertionFunc
	}{
		{
			desc:        "OK uuid match",
			node:        node.GetName(),
			expectMatch: require.True,
		},
		{
			desc:        "OK host name match",
			node:        node.GetHostname(),
			expectMatch: require.True,
		},
		{
			desc:        "OK addr match",
			node:        node.GetAddr(),
			expectMatch: require.True,
		},
		{
			desc:        "OK public addr 1 match",
			node:        "node.example.com",
			expectMatch: require.True,
		},
		{
			desc:        "OK public addr 2 match",
			node:        "localhost",
			expectMatch: require.True,
		},
		{
			desc:        "NOK label match",
			node:        "foo",
			expectMatch: require.False,
		},
		{
			desc:        "NOK unknown ip",
			node:        "1.2.3.4",
			expectMatch: require.False,
		},
		{
			desc:        "NOK unknown addr",
			node:        "unknown.example.com",
			expectMatch: require.False,
		},
	} {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			resp, err := cl.IsMFARequired(ctx, &proto.IsMFARequiredRequest{
				Target: &proto.IsMFARequiredRequest_Node{Node: &proto.NodeLogin{
					Login: user.GetName(),
					Node:  tc.node,
				}},
			})
			require.NoError(t, err)
			tc.expectMatch(t, resp.Required)
		})
	}
}

// testOriginDynamicStored tests setting a ResourceWithOrigin via the server
// API always results in the resource being stored with OriginDynamic.
func testOriginDynamicStored(t *testing.T, setWithOrigin func(*Client, string) error, getStored func(*Server) (types.ResourceWithOrigin, error)) {
	srv := newTestTLSServer(t)

	// Create a fake user.
	user, _, err := CreateUserAndRole(srv.Auth(), "configurer", []string{}, nil)
	require.NoError(t, err)
	cl, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, origin := range types.OriginValues {
		t.Run(fmt.Sprintf("setting with origin %q", origin), func(t *testing.T) {
			err := setWithOrigin(cl, origin)
			require.NoError(t, err)

			stored, err := getStored(srv.Auth())
			require.NoError(t, err)
			require.Equal(t, stored.Origin(), types.OriginDynamic)
		})
	}
}

func TestAuthPreferenceOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *Client, origin string) error {
		authPref := types.DefaultAuthPreference()
		authPref.SetOrigin(origin)
		return cl.SetAuthPreference(ctx, authPref)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetAuthPreference(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestClusterNetworkingConfigOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *Client, origin string) error {
		netConfig := types.DefaultClusterNetworkingConfig()
		netConfig.SetOrigin(origin)
		return cl.SetClusterNetworkingConfig(ctx, netConfig)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetClusterNetworkingConfig(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestSessionRecordingConfigOriginDynamic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	setWithOrigin := func(cl *Client, origin string) error {
		recConfig := types.DefaultSessionRecordingConfig()
		recConfig.SetOrigin(origin)
		return cl.SetSessionRecordingConfig(ctx, recConfig)
	}

	getStored := func(asrv *Server) (types.ResourceWithOrigin, error) {
		return asrv.GetSessionRecordingConfig(ctx)
	}

	testOriginDynamicStored(t, setWithOrigin, getStored)
}

func TestGenerateHostCerts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	pubTLS, err := PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	certs, err := clt.GenerateHostCerts(ctx, &proto.HostCertsRequest{
		HostID:   "Admin",
		Role:     types.RoleAdmin,
		NodeName: "foo",
		// Ensure that 0.0.0.0 gets replaced with the RemoteAddr of the client
		AdditionalPrincipals: []string{"0.0.0.0"},
		PublicSSHKey:         pub,
		PublicTLSKey:         pubTLS,
	})
	require.NoError(t, err)
	require.NotNil(t, certs)
}

// TestInstanceCertAndControlStream attempts to generate an instance cert via the
// assertion API and use it to handle an inventory ping via the control stream.
func TestInstanceCertAndControlStream(t *testing.T) {
	const assertionID = "test-assertion"
	const serverID = "test-server"
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := newTestTLSServer(t)

	roles := []types.SystemRole{
		types.RoleNode,
		types.RoleAuth,
		types.RoleProxy,
	}

	clt, err := srv.NewClient(TestServerID(types.RoleNode, serverID))
	require.NoError(t, err)
	defer clt.Close()

	priv, pub, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	pubTLS, err := PrivateKeyToPublicKeyTLS(priv)
	require.NoError(t, err)

	req := proto.HostCertsRequest{
		HostID:       serverID,
		Role:         types.RoleInstance,
		PublicSSHKey: pub,
		PublicTLSKey: pubTLS,
		SystemRoles:  roles,
		// assertion ID is omitted initially to test
		// the failure case
	}

	// request should fail since clt only holds RoleNode
	_, err = clt.GenerateHostCerts(ctx, &req)
	require.True(t, trace.IsAccessDenied(err))

	// perform assertions
	for _, role := range roles {
		func() {
			clt, err := srv.NewClient(TestServerID(role, serverID))
			require.NoError(t, err)
			defer clt.Close()

			err = clt.UnstableAssertSystemRole(ctx, proto.UnstableSystemRoleAssertion{
				ServerID:    serverID,
				AssertionID: assertionID,
				SystemRole:  role,
			})
			require.NoError(t, err)
		}()
	}

	// set assertion ID
	req.UnstableSystemRoleAssertionID = assertionID

	// assertion should allow us to generate certs
	certs, err := clt.GenerateHostCerts(ctx, &req)
	require.NoError(t, err)

	// make an instance client
	instanceCert, err := tls.X509KeyPair(certs.TLS, priv)
	require.NoError(t, err)
	instanceClt := srv.NewClientWithCert(instanceCert)

	// instance cert can self-renew without assertions
	req.UnstableSystemRoleAssertionID = ""
	_, err = instanceClt.GenerateHostCerts(ctx, &req)
	require.NoError(t, err)

	stream, err := instanceClt.InventoryControlStream(ctx)
	require.NoError(t, err)
	defer stream.Close()

	err = stream.Send(ctx, proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: roles,
	})
	require.NoError(t, err)

	select {
	case msg := <-stream.Recv():
		_, ok := msg.(proto.DownstreamInventoryHello)
		require.True(t, ok)
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for downstream hello")
	}

	// fire off a ping in the background
	pingErr := make(chan error, 1)
	go func() {
		defer close(pingErr)
		// get an admin client so that we can test pings
		clt, err := srv.NewClient(TestAdmin())
		if err != nil {
			pingErr <- err
			return
		}
		defer clt.Close()

		_, err = clt.PingInventory(ctx, proto.InventoryPingRequest{
			ServerID: serverID,
		})
		pingErr <- err
	}()

	// wait for the ping
	select {
	case msg := <-stream.Recv():
		ping, ok := msg.(proto.DownstreamInventoryPing)
		require.True(t, ok)
		err = stream.Send(ctx, proto.UpstreamInventoryPong{
			ID: ping.ID,
		})
		require.NoError(t, err)
	case <-time.After(time.Second * 5):
		t.Fatalf("timeout waiting for downstream ping")
	}

	// ensure that bg ping routine was successful
	require.NoError(t, <-pingErr)
}

func TestNodesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// node1 and node2 will be added to default namespace
	node1, err := types.NewServerWithLabels("node1", types.KindNode, types.ServerSpecV2{}, nil)
	require.NoError(t, err)
	node2, err := types.NewServerWithLabels("node2", types.KindNode, types.ServerSpecV2{}, nil)
	require.NoError(t, err)

	t.Run("CreateNode", func(t *testing.T) {
		// Initially expect no nodes to be returned.
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Empty(t, nodes)

		// Create nodes.
		_, err = clt.UpsertNode(ctx, node1)
		require.NoError(t, err)

		_, err = clt.UpsertNode(ctx, node2)
		require.NoError(t, err)
	})

	// Run NodeGetters in nested subtests to allow parallelization.
	t.Run("NodeGetters", func(t *testing.T) {
		t.Run("GetNodes", func(t *testing.T) {
			t.Parallel()
			// Get all nodes
			nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			require.Len(t, nodes, 2)
			require.Empty(t, cmp.Diff([]types.Server{node1, node2}, nodes,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// GetNodes should not fail if namespace is empty
			_, err = clt.GetNodes(ctx, "")
			require.NoError(t, err)
		})
		t.Run("GetNode", func(t *testing.T) {
			t.Parallel()
			// Get Node
			node, err := clt.GetNode(ctx, apidefaults.Namespace, "node1")
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(node1, node,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// GetNode should fail if node name isn't provided
			_, err = clt.GetNode(ctx, apidefaults.Namespace, "")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())

			// GetNode should fail if namespace isn't provided
			_, err = clt.GetNode(ctx, "", "node1")
			require.IsType(t, &trace.BadParameterError{}, err.(*trace.TraceErr).OrigError())
		})
	})

	t.Run("DeleteNode", func(t *testing.T) {
		// Make sure can't delete with empty namespace or name.
		err = clt.DeleteNode(ctx, apidefaults.Namespace, "")
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		err = clt.DeleteNode(ctx, "", node1.GetName())
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		// Delete node.
		err = clt.DeleteNode(ctx, apidefaults.Namespace, node1.GetName())
		require.NoError(t, err)

		// Expect node not found
		_, err := clt.GetNode(ctx, apidefaults.Namespace, "node1")
		require.IsType(t, trace.NotFound(""), err)
	})

	t.Run("DeleteAllNodes", func(t *testing.T) {
		// Make sure can't delete with empty namespace.
		err = clt.DeleteAllNodes(ctx, "")
		require.Error(t, err)
		require.IsType(t, trace.BadParameter(""), err)

		// Delete nodes
		err = clt.DeleteAllNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)

		// Now expect no nodes to be returned.
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Empty(t, nodes)
	})
}

func TestLocksCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	now := srv.Clock().Now()
	lock1, err := types.NewLock("lock1", types.LockSpecV2{
		Target: types.LockTarget{
			User: "user-A",
		},
		Expires: &now,
	})
	require.NoError(t, err)
	lock1.SetCreatedBy(string(types.RoleAdmin))
	lock1.SetCreatedAt(now)

	lock2, err := types.NewLock("lock2", types.LockSpecV2{
		Target: types.LockTarget{
			Node: "node",
		},
		Message: "node compromised",
	})
	require.NoError(t, err)
	lock2.SetCreatedBy(string(types.RoleAdmin))
	lock2.SetCreatedAt(now)

	t.Run("CreateLock", func(t *testing.T) {
		// Initially expect no locks to be returned.
		locks, err := clt.GetLocks(ctx, false)
		require.NoError(t, err)
		require.Empty(t, locks)

		// Create locks.
		err = clt.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		err = clt.UpsertLock(ctx, lock2)
		require.NoError(t, err)
	})

	// Run LockGetters in nested subtests to allow parallelization.
	t.Run("LockGetters", func(t *testing.T) {
		t.Run("GetLocks", func(t *testing.T) {
			t.Parallel()
			locks, err := clt.GetLocks(ctx, false)
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))
		})
		t.Run("GetLocks with targets", func(t *testing.T) {
			t.Parallel()
			// Match both locks with the targets.
			locks, err := clt.GetLocks(ctx, false, lock1.Target(), lock2.Target())
			require.NoError(t, err)
			require.Len(t, locks, 2)
			require.Empty(t, cmp.Diff([]types.Lock{lock1, lock2}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Match only one of the locks.
			roleTarget := types.LockTarget{Role: "role-A"}
			locks, err = clt.GetLocks(ctx, false, lock1.Target(), roleTarget)
			require.NoError(t, err)
			require.Len(t, locks, 1)
			require.Empty(t, cmp.Diff([]types.Lock{lock1}, locks,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Match none of the locks.
			locks, err = clt.GetLocks(ctx, false, roleTarget)
			require.NoError(t, err)
			require.Empty(t, locks)
		})
		t.Run("GetLock", func(t *testing.T) {
			t.Parallel()
			// Get one of the locks.
			lock, err := clt.GetLock(ctx, lock1.GetName())
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(lock1, lock,
				cmpopts.IgnoreFields(types.Metadata{}, "ID")))

			// Attempt to get a nonexistent lock.
			_, err = clt.GetLock(ctx, "lock3")
			require.Error(t, err)
			require.True(t, trace.IsNotFound(err))
		})
	})

	t.Run("UpsertLock", func(t *testing.T) {
		// Get one of the locks.
		lock, err := clt.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Empty(t, lock.Message())

		msg := "cluster maintenance"
		lock1.SetMessage(msg)
		err = clt.UpsertLock(ctx, lock1)
		require.NoError(t, err)

		lock, err = clt.GetLock(ctx, lock1.GetName())
		require.NoError(t, err)
		require.Equal(t, msg, lock.Message())
	})

	t.Run("DeleteLock", func(t *testing.T) {
		// Delete lock.
		err = clt.DeleteLock(ctx, lock1.GetName())
		require.NoError(t, err)

		// Expect lock not found.
		_, err := clt.GetLock(ctx, lock1.GetName())
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

// TestApplicationServersCRUD tests application server operations.
func TestApplicationServersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple app servers.
	app1, err := types.NewAppV3(types.Metadata{Name: "app-1"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server1, err := types.NewAppServerV3FromApp(app1, "server-1", "server-1")
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{Name: "app-2"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server2, err := types.NewAppServerV3FromApp(app2, "server-2", "server-2")
	require.NoError(t, err)
	app3, err := types.NewAppV3(types.Metadata{Name: "app-3"},
		types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server3, err := types.NewAppServerV3FromApp(app3, "server-3", "server-3")
	require.NoError(t, err)

	// Initially we expect no app servers.
	out, err := clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Register all app servers.
	_, err = clt.UpsertApplicationServer(ctx, server1)
	require.NoError(t, err)
	_, err = clt.UpsertApplicationServer(ctx, server2)
	require.NoError(t, err)
	_, err = clt.UpsertApplicationServer(ctx, server3)
	require.NoError(t, err)

	// Fetch all app servers.
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server1, server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update an app server.
	server1.Metadata.Description = "description"
	_, err = clt.UpsertApplicationServer(ctx, server1)
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server1, server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete an app server.
	err = clt.DeleteApplicationServer(ctx, server1.GetNamespace(), server1.GetHostID(), server1.GetName())
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server2, server3}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete all app servers.
	err = clt.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	out, err = clt.GetApplicationServers(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

// TestAppsCRUD tests application resource operations.
func TestAppsCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple apps.
	app1, err := types.NewAppV3(types.Metadata{
		Name:   "app1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)
	app2, err := types.NewAppV3(types.Metadata{
		Name:   "app2",
		Labels: map[string]string{types.OriginLabel: types.OriginOkta}, // This should be overwritten
	}, types.AppSpecV3{
		URI: "localhost2",
	})
	require.NoError(t, err)

	// Initially we expect no apps.
	out, err := clt.GetApps(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create both apps.
	err = clt.CreateApp(ctx, app1)
	require.NoError(t, err)
	err = clt.CreateApp(ctx, app2)
	require.NoError(t, err)

	// Fetch all apps.
	app2.SetOrigin(types.OriginDynamic)
	out, err = clt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app1, app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific app.
	app, err := clt.GetApp(ctx, app2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(app2, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch an app that doesn't exist.
	_, err = clt.GetApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same app.
	err = clt.CreateApp(ctx, app1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update an app.
	app1.Metadata.Description = "description"
	err = clt.UpdateApp(ctx, app1)
	require.NoError(t, err)
	app, err = clt.GetApp(ctx, app1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(app1, app,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete an app.
	err = clt.DeleteApp(ctx, app1.GetName())
	require.NoError(t, err)
	out, err = clt.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete an app that doesn't exist.
	err = clt.DeleteApp(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all apps.
	err = clt.DeleteAllApps(ctx)
	require.NoError(t, err)
	out, err = clt.GetApps(ctx)
	require.NoError(t, err)
	require.Len(t, out, 0)
}

// TestAppServersCRUD tests application server resource operations.
func TestAppServersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	// Create an app server, expected origin dynamic.
	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	app1, err := types.NewAppV3(types.Metadata{
		Name: "app-dynamic",
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)

	appServer1, err := types.NewAppServerV3FromApp(app1, "app-dynamic", "hostID")
	require.NoError(t, err)

	_, err = clt.UpsertApplicationServer(ctx, appServer1)
	require.NoError(t, err)

	resources, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 1)

	appServer := resources.Resources[0].(types.AppServer)
	require.Empty(t, cmp.Diff(appServer, appServer1,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	require.NoError(t, clt.DeleteApplicationServer(ctx, apidefaults.Namespace, "hostID", appServer1.GetName()))

	resources, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Empty(t, resources.Resources)

	// Try to create app servers with Okta labels as a non-Okta role.
	app2, err := types.NewAppV3(types.Metadata{
		Name:   "app-okta",
		Labels: map[string]string{types.OriginLabel: types.OriginOkta},
	}, types.AppSpecV3{
		URI: "localhost1",
	})
	require.NoError(t, err)

	appServer2, err := types.NewAppServerV3FromApp(app2, "app-okta", "hostID")
	require.NoError(t, err)

	_, err = clt.UpsertApplicationServer(ctx, appServer2)
	require.ErrorIs(t, err, trace.BadParameter("only the Okta role can create app servers and apps with an Okta origin"))

	delete(app2.Metadata.Labels, types.OriginLabel)
	appServer2.SetOrigin(types.OriginOkta)

	_, err = clt.UpsertApplicationServer(ctx, appServer2)
	require.ErrorIs(t, err, trace.BadParameter("only the Okta role can create app servers and apps with an Okta origin"))

	// Create an app server with Okta labels using the Okta role.
	clt, err = srv.NewClient(TestBuiltin(types.RoleOkta))
	require.NoError(t, err)

	app2.SetOrigin(types.OriginOkta)
	appServer2.SetOrigin(types.OriginOkta)
	_, err = clt.UpsertApplicationServer(ctx, appServer2)
	require.NoError(t, err)

	resources, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Len(t, resources.Resources, 1)

	appServer2.SetOrigin(types.OriginOkta)
	app2.SetOrigin(types.OriginOkta)
	appServer = resources.Resources[0].(types.AppServer)
	require.Empty(t, cmp.Diff(appServer, appServer2,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	require.NoError(t, clt.DeleteApplicationServer(ctx, apidefaults.Namespace, "hostID", appServer2.GetName()))

	resources, err = clt.ListResources(ctx, proto.ListResourcesRequest{
		ResourceType: types.KindAppServer,
		Limit:        apidefaults.DefaultChunkSize,
	})
	require.NoError(t, err)
	require.Empty(t, resources.Resources)
}

// TestDatabasesCRUD tests database resource operations.
func TestDatabasesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create a couple databases.
	db1, err := types.NewDatabaseV3(types.Metadata{
		Name:   "db1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	db2, err := types.NewDatabaseV3(types.Metadata{
		Name:   "db2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "localhost:3306",
	})
	require.NoError(t, err)

	// Initially we expect no databases.
	out, err := clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))

	// Create both databases.
	err = clt.CreateDatabase(ctx, db1)
	require.NoError(t, err)
	err = clt.CreateDatabase(ctx, db2)
	require.NoError(t, err)

	// Fetch all databases.
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Fetch a specific database.
	db, err := clt.GetDatabase(ctx, db2.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db2, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to fetch a database that doesn't exist.
	_, err = clt.GetDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Try to create the same database.
	err = clt.CreateDatabase(ctx, db1)
	require.IsType(t, trace.AlreadyExists(""), err)

	// Update a database.
	db1.Metadata.Description = "description"
	err = clt.UpdateDatabase(ctx, db1)
	require.NoError(t, err)
	db, err = clt.GetDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(db1, db,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a database.
	err = clt.DeleteDatabase(ctx, db1.GetName())
	require.NoError(t, err)
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a database that doesn't exist.
	err = clt.DeleteDatabase(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all databases.
	err = clt.DeleteAllDatabases(ctx)
	require.NoError(t, err)
	out, err = clt.GetDatabases(ctx)
	require.NoError(t, err)
	require.Len(t, out, 0)
}

// TestDatabaseServicesCRUD tests DatabaseService resource operations.
func TestDatabaseServicesCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create two DatabaseServices.
	db1, err := types.NewDatabaseServiceV1(types.Metadata{
		Name:   "db1",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"prod"},
				},
			},
		},
	})
	require.NoError(t, err)

	db2, err := types.NewDatabaseServiceV1(types.Metadata{
		Name:   "db2",
		Labels: map[string]string{types.OriginLabel: types.OriginDynamic},
	}, types.DatabaseServiceSpecV1{
		ResourceMatchers: []*types.DatabaseResourceMatcher{
			{
				Labels: &types.Labels{
					"env": []string{"stg"},
				},
			},
		},
	})
	require.NoError(t, err)

	// Initially we expect no DatabaseServices.
	listServicesResp, err := clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err := types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, out)

	// Create both DatabaseServices.
	_, err = clt.UpsertDatabaseService(ctx, db1)
	require.NoError(t, err)
	_, err = clt.UpsertDatabaseService(ctx, db2)
	require.NoError(t, err)

	// Fetch all DatabaseServices.
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseService{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update a DatabaseService.
	db1.Spec.ResourceMatchers[0] = &types.DatabaseResourceMatcher{
		Labels: &types.Labels{
			"env": []string{"notprod"},
		},
	}

	_, err = clt.UpsertDatabaseService(ctx, db1)
	require.NoError(t, err)
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseService{db1, db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a DatabaseService.
	err = clt.DeleteDatabaseService(ctx, db1.GetName())
	require.NoError(t, err)
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseService{db2}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a DatabaseService that doesn't exist.
	err = clt.DeleteDatabaseService(ctx, "doesnotexist")
	require.IsType(t, trace.NotFound(""), err)

	// Delete all DatabaseServices.
	err = clt.DeleteAllDatabaseServices(ctx)
	require.NoError(t, err)
	listServicesResp, err = clt.ListResources(ctx,
		proto.ListResourcesRequest{
			ResourceType: types.KindDatabaseService,
			Limit:        apidefaults.DefaultChunkSize,
		},
	)
	require.NoError(t, err)
	out, err = types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestSAMLIdPServiceProvidersCRUD tests SAMLIdPServiceProviders resource operations.
func TestSAMLIdPServiceProvidersCRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	// Create two SAML IdP service providers.
	sp1, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp1"),
			EntityID:         "sp1",
		})
	require.NoError(t, err)

	sp2, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp2",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newEntityDescriptor("sp2"),
			EntityID:         "sp2",
		})
	require.NoError(t, err)

	// Initially we expect no service providers.
	listResp, nextKey, err := clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, listResp)

	// Create both service providers
	err = clt.CreateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	err = clt.CreateSAMLIdPServiceProvider(ctx, sp2)
	require.NoError(t, err)

	// Fetch all service providers
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, listResp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Update a service provider.
	sp1.SetEntityDescriptor(newEntityDescriptor("updated-sp1"))
	sp1.SetEntityID("updated-sp1")

	err = clt.UpdateSAMLIdPServiceProvider(ctx, sp1)
	require.NoError(t, err)
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp1, sp2}, listResp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Delete a service provider.
	err = clt.DeleteSAMLIdPServiceProvider(ctx, sp1.GetName())
	require.NoError(t, err)
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, cmp.Diff([]types.SAMLIdPServiceProvider{sp2}, listResp,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
	))

	// Try to delete a service provider that doesn't exist.
	err = clt.DeleteSAMLIdPServiceProvider(ctx, "doesnotexist")
	require.True(t, trace.IsNotFound(err))

	// Delete all service providers.
	err = clt.DeleteAllSAMLIdPServiceProviders(ctx)
	require.NoError(t, err)
	listResp, nextKey, err = clt.ListSAMLIdPServiceProviders(ctx, 200, "")
	require.NoError(t, err)
	require.Empty(t, nextKey)
	require.Empty(t, listResp)
}

func TestListResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	srv := newTestTLSServer(t)

	clt, err := srv.NewClient(TestAdmin())
	require.NoError(t, err)

	testCases := map[string]struct {
		resourceType   string
		createResource func(name string, clt *Client) error
	}{
		"DatabaseServers": {
			resourceType: types.KindDatabaseServer,
			createResource: func(name string, clt *Client) error {
				server, err := types.NewDatabaseServerV3(types.Metadata{
					Name: name,
				}, types.DatabaseServerSpecV3{
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
					Hostname: "localhost",
					HostID:   uuid.New().String(),
				})
				if err != nil {
					return err
				}

				_, err = clt.UpsertDatabaseServer(ctx, server)
				return err
			},
		},
		"ApplicationServers": {
			resourceType: types.KindAppServer,
			createResource: func(name string, clt *Client) error {
				app, err := types.NewAppV3(types.Metadata{
					Name: name,
				}, types.AppSpecV3{
					URI: "localhost",
				})
				if err != nil {
					return err
				}

				server, err := types.NewAppServerV3(types.Metadata{
					Name: name,
				}, types.AppServerSpecV3{
					Hostname: "localhost",
					HostID:   uuid.New().String(),
					App:      app,
				})
				if err != nil {
					return err
				}

				_, err = clt.UpsertApplicationServer(ctx, server)
				return err
			},
		},
		"KubeServer": {
			resourceType: types.KindKubeServer,
			createResource: func(name string, clt *Client) error {
				kube, err := types.NewKubernetesClusterV3(
					types.Metadata{
						Name:   name,
						Labels: map[string]string{"name": name},
					},
					types.KubernetesClusterSpecV3{},
				)
				if err != nil {
					return err
				}

				kubeServer, err := types.NewKubernetesServerV3FromCluster(kube, "_", "_")
				if err != nil {
					return err
				}
				_, err = clt.UpsertKubernetesServer(ctx, kubeServer)
				return err
			},
		},
		"Node": {
			resourceType: types.KindNode,
			createResource: func(name string, clt *Client) error {
				server, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{})
				if err != nil {
					return err
				}

				_, err = clt.UpsertNode(ctx, server)
				return err
			},
		},
		"WindowsDesktops": {
			resourceType: types.KindWindowsDesktop,
			createResource: func(name string, clt *Client) error {
				desktop, err := types.NewWindowsDesktopV3(name, nil,
					types.WindowsDesktopSpecV3{Addr: "_", HostID: "_"})
				if err != nil {
					return err
				}

				return clt.UpsertWindowsDesktop(ctx, desktop)
			},
		},
	}

	for name, test := range testCases {
		name := name
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: test.resourceType,
				Namespace:    apidefaults.Namespace,
				Limit:        100,
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 0)
			require.Empty(t, resp.NextKey)

			// create two resources
			err = test.createResource("foo", clt)
			require.NoError(t, err)
			err = test.createResource("bar", clt)
			require.NoError(t, err)

			resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: test.resourceType,
				Namespace:    apidefaults.Namespace,
				Limit:        100,
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 2)
			require.Empty(t, resp.NextKey)
			require.Empty(t, resp.TotalCount)

			// ListResources should also work when called on auth directly
			resp, err = srv.Auth().ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: test.resourceType,
				Namespace:    apidefaults.Namespace,
				Limit:        100,
			})
			require.NoError(t, err)
			require.Len(t, resp.Resources, 2)
			require.Empty(t, resp.NextKey)
			require.Empty(t, resp.TotalCount)

			// Test types.KindKubernetesCluster
			if test.resourceType == types.KindKubeServer {
				test.resourceType = types.KindKubernetesCluster
				resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType: test.resourceType,
					Namespace:    apidefaults.Namespace,
					Limit:        100,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 2)
				require.Empty(t, resp.NextKey)
				require.Equal(t, 2, resp.TotalCount)
			} else {
				// Test listing with NeedTotalCount flag.
				resp, err = clt.ListResources(ctx, proto.ListResourcesRequest{
					ResourceType:   test.resourceType,
					Limit:          100,
					NeedTotalCount: true,
				})
				require.NoError(t, err)
				require.Len(t, resp.Resources, 2)
				require.Empty(t, resp.NextKey)
				require.Equal(t, 2, resp.TotalCount)
			}
		})
	}

	t.Run("InvalidResourceType", func(t *testing.T) {
		_, err := clt.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: "",
			Namespace:    apidefaults.Namespace,
			Limit:        100,
		})
		require.Error(t, err)
	})
}

func TestCustomRateLimiting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tests := []struct {
		name  string
		burst int
		fn    func(*Client) error
	}{
		{
			name: "RPC ChangeUserAuthentication",
			fn: func(clt *Client) error {
				_, err := clt.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{})
				return err
			},
		},
		{
			name:  "RPC CreateAuthenticateChallenge",
			burst: defaults.LimiterPasswordlessBurst,
			fn: func(clt *Client) error {
				_, err := clt.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{})
				return err
			},
		},
		{
			name: "RPC GetAccountRecoveryToken",
			fn: func(clt *Client) error {
				_, err := clt.GetAccountRecoveryToken(ctx, &proto.GetAccountRecoveryTokenRequest{})
				return err
			},
		},
		{
			name: "RPC StartAccountRecovery",
			fn: func(clt *Client) error {
				_, err := clt.StartAccountRecovery(ctx, &proto.StartAccountRecoveryRequest{})
				return err
			},
		},
		{
			name: "RPC VerifyAccountRecovery",
			fn: func(clt *Client) error {
				_, err := clt.VerifyAccountRecovery(ctx, &proto.VerifyAccountRecoveryRequest{})
				return err
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Create new instance per test case, to troubleshoot which test case
			// specifically failed, otherwise multiple cases can fail from running
			// cases in parallel.
			srv := newTestTLSServer(t)
			clt, err := srv.NewClient(TestNop())
			require.NoError(t, err)

			var attempts int
			if test.burst == 0 {
				attempts = 10 // Good for most tests.
			} else {
				attempts = test.burst
			}

			for i := 0; i < attempts; i++ {
				err = test.fn(clt)
				require.False(t, trace.IsLimitExceeded(err), "got err = %v, want non-IsLimitExceeded", err)
			}

			err = test.fn(clt)
			require.True(t, trace.IsLimitExceeded(err), "got err = %v, want LimitExceeded", err)
		})
	}
}

type mockAuthorizer struct {
	ctx *authz.Context
	err error
}

func (a mockAuthorizer) Authorize(context.Context) (*authz.Context, error) {
	return a.ctx, a.err
}

type mockTraceClient struct {
	err   error
	spans []*otlptracev1.ResourceSpans
}

func (m mockTraceClient) Start(ctx context.Context) error {
	return nil
}

func (m mockTraceClient) Stop(ctx context.Context) error {
	return nil
}

func (m *mockTraceClient) UploadTraces(ctx context.Context, protoSpans []*otlptracev1.ResourceSpans) error {
	m.spans = protoSpans

	return m.err
}

func TestExport(t *testing.T) {
	t.Parallel()
	uploadErr := trace.AccessDenied("failed to upload")

	const user = "user"

	validateResource := func(forwardedFor string, resourceSpan *otlptracev1.ResourceSpans) {
		var forwarded []string
		for _, attribute := range resourceSpan.Resource.Attributes {
			if attribute.Key == forwardedTag {
				forwarded = append(forwarded, attribute.Value.GetStringValue())
			}
		}

		require.Len(t, forwarded, 1)

		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				for _, attribute := range span.Attributes {
					if attribute.Key == forwardedTag {
						forwarded = append(forwarded, attribute.Value.GetStringValue())
					}
				}
			}
		}

		require.Len(t, forwarded, 2)
		for _, value := range forwarded {
			require.Equal(t, forwardedFor, value)
		}
	}

	validateTaggedSpans := func(forwardedFor string) require.ValueAssertionFunc {
		return func(t require.TestingT, i interface{}, i2 ...interface{}) {
			require.NotEmpty(t, i)
			resourceSpans, ok := i.([]*otlptracev1.ResourceSpans)
			require.True(t, ok)

			for _, resourceSpan := range resourceSpans {
				if resourceSpan.Resource != nil {
					validateResource(forwardedFor, resourceSpan)
					return
				}

				for _, scopeSpan := range resourceSpan.ScopeSpans {
					for _, span := range scopeSpan.Spans {
						var foundForwardedTag bool
						for _, attribute := range span.Attributes {
							if attribute.Key == forwardedTag {
								require.False(t, foundForwardedTag)
								foundForwardedTag = true
								require.Equal(t, forwardedFor, attribute.Value.GetStringValue())
							}
						}
						require.True(t, foundForwardedTag)
					}
				}
			}
		}
	}

	testSpans := []*otlptracev1.ResourceSpans{
		{
			Resource: &otlpresourcev1.Resource{
				Attributes: []*otlpcommonv1.KeyValue{
					{
						Key: "test",
						Value: &otlpcommonv1.AnyValue{
							Value: &otlpcommonv1.AnyValue_IntValue{
								IntValue: 1,
							},
						},
					},
					{
						Key: "key",
						Value: &otlpcommonv1.AnyValue{
							Value: &otlpcommonv1.AnyValue_StringValue{
								StringValue: user,
							},
						},
					},
				},
			},
			ScopeSpans: []*otlptracev1.ScopeSpans{
				{
					Spans: []*otlptracev1.Span{
						{
							Name: "with-attributes",
							Attributes: []*otlpcommonv1.KeyValue{
								{
									Key: "test",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_IntValue{
											IntValue: 1,
										},
									},
								},
								{
									Key: "key",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_DoubleValue{
											DoubleValue: 5.0,
										},
									},
								},
							},
						},
						{
							Name:       "with-tag",
							Attributes: []*otlpcommonv1.KeyValue{{Key: forwardedTag, Value: &otlpcommonv1.AnyValue{Value: &otlpcommonv1.AnyValue_StringValue{StringValue: "test"}}}},
						},
						{
							Name: "no-attributes",
						},
					},
				},
			},
		},
		{
			ScopeSpans: []*otlptracev1.ScopeSpans{
				{
					Spans: []*otlptracev1.Span{
						{
							Name: "more-with-attributes",
							Attributes: []*otlpcommonv1.KeyValue{
								{
									Key: "test2",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_IntValue{
											IntValue: 11,
										},
									},
								},
								{
									Key: "key2",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_DoubleValue{
											DoubleValue: 15.0,
										},
									},
								},
							},
						},
						{
							Name: "already-tagged",
							Attributes: []*otlpcommonv1.KeyValue{
								{
									Key: forwardedTag,
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_StringValue{
											StringValue: user,
										},
									},
								},
								{
									Key: "key2",
									Value: &otlpcommonv1.AnyValue{
										Value: &otlpcommonv1.AnyValue_DoubleValue{
											DoubleValue: 15.0,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cases := []struct {
		name              string
		identity          TestIdentity
		errAssertion      require.ErrorAssertionFunc
		uploadedAssertion require.ValueAssertionFunc
		spans             []*otlptracev1.ResourceSpans
		authorizer        authz.Authorizer
		mockTraceClient   mockTraceClient
	}{
		{
			name:              "error when unauthorized",
			identity:          TestNop(),
			errAssertion:      require.Error,
			uploadedAssertion: require.Empty,
			spans:             make([]*otlptracev1.ResourceSpans, 1),
			authorizer:        &mockAuthorizer{err: trace.AccessDenied("unauthorized")},
		},
		{
			name:              "nop for empty spans",
			identity:          TestBuiltin(types.RoleNode),
			errAssertion:      require.NoError,
			uploadedAssertion: require.Empty,
		},
		{
			name:     "failure to forward spans",
			identity: TestBuiltin(types.RoleNode),
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, trail.FromGRPC(trace.Unwrap(err)), uploadErr)
			},
			uploadedAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.NotNil(t, i)
				require.Len(t, i, 1)
			},
			spans:           make([]*otlptracev1.ResourceSpans, 1),
			mockTraceClient: mockTraceClient{err: uploadErr},
		},
		{
			name:              "forwarded spans get tagged for system roles",
			identity:          TestBuiltin(types.RoleProxy),
			errAssertion:      require.NoError,
			spans:             testSpans,
			uploadedAssertion: validateTaggedSpans(fmt.Sprintf("%s.localhost:%s", types.RoleProxy, types.RoleProxy)),
		},
		{
			name:              "forwarded spans get tagged for users",
			identity:          TestUser(user),
			errAssertion:      require.NoError,
			spans:             testSpans,
			uploadedAssertion: validateTaggedSpans(user),
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			as, err := NewTestAuthServer(TestAuthServerConfig{
				Dir:         t.TempDir(),
				Clock:       clockwork.NewFakeClock(),
				TraceClient: &tt.mockTraceClient,
			})
			require.NoError(t, err)

			srv, err := as.NewTestTLSServer()
			require.NoError(t, err)

			t.Cleanup(func() { require.NoError(t, srv.Close()) })

			// Create a fake user.
			_, _, err = CreateUserAndRole(srv.Auth(), user, []string{"role"}, nil)
			require.NoError(t, err)

			// Setup the server
			if tt.authorizer != nil {
				srv.TLSServer.grpcServer.Authorizer = tt.authorizer
				require.NoError(t, err)
			}

			// Get a client for the test identity
			clt, err := srv.NewClient(tt.identity)
			require.NoError(t, err)

			// create a tracing client and forward some traces
			traceClt := tracing.NewClient(clt.APIClient.GetConnection())
			t.Cleanup(func() { require.NoError(t, traceClt.Close()) })
			require.NoError(t, traceClt.Start(ctx))

			tt.errAssertion(t, traceClt.UploadTraces(ctx, tt.spans))
			tt.uploadedAssertion(t, tt.mockTraceClient.spans)
		})
	}
}

// TestGRPCServer_CreateToken tests the handler of the deprecated CreateToken
// RPC.
func TestGRPCServer_CreateToken(t *testing.T) {
	ctx := context.Background()
	server := newTestTLSServer(t)

	// Allow us to directly invoke the deprecated gRPC methods with
	// authentication.
	user := TestAdmin()
	ctx = authz.ContextWithUser(ctx, user.I)

	// Test default expiry is applied.
	t.Run("undefined-expiry", func(t *testing.T) {
		tokenName := "undefined-expiry"
		roles := types.SystemRoles{types.RoleNode}
		token, err := types.NewProvisionToken(
			tokenName,
			roles,
			time.Time{},
		)
		require.NoError(t, err)
		_, err = server.TLSServer.grpcServer.CreateToken(
			ctx, token.(*types.ProvisionTokenV2),
		)
		require.NoError(t, err)
		token, err = server.TLSServer.grpcServer.GetToken(
			ctx, &types.ResourceRequest{Name: tokenName},
		)
		require.NoError(t, err)
		require.Equal(t, tokenName, token.GetName())
		ttl := token.Expiry().Sub(server.Clock().Now())
		defaultTTL := defaults.ProvisioningTokenTTL
		require.LessOrEqual(t, ttl, defaultTTL)
		require.GreaterOrEqual(t, ttl, defaultTTL-(time.Second*10))
	})

	// Test set expiry is applied.
	t.Run("set-expiry", func(t *testing.T) {
		tokenName := "set-expiry"
		roles := types.SystemRoles{types.RoleNode}
		ttl := time.Hour * 24
		token, err := types.NewProvisionToken(
			tokenName,
			roles,
			server.Clock().Now().Add(ttl),
		)
		require.NoError(t, err)
		_, err = server.TLSServer.grpcServer.CreateToken(
			ctx, token.(*types.ProvisionTokenV2),
		)
		require.NoError(t, err)
		token, err = server.TLSServer.grpcServer.GetToken(
			ctx, &types.ResourceRequest{Name: tokenName},
		)
		require.NoError(t, err)
		require.Equal(t, tokenName, token.GetName())
		actualTTL := token.Expiry().Sub(server.Clock().Now())
		require.LessOrEqual(t, actualTTL, ttl)
		require.GreaterOrEqual(t, actualTTL, ttl-(time.Second*10))
	})

	// Test expiry in past is changed to default
	t.Run("past-expiry", func(t *testing.T) {
		tokenName := "past-expiry"
		roles := types.SystemRoles{types.RoleNode}
		token, err := types.NewProvisionToken(
			tokenName,
			roles,
			server.Clock().Now().Add(-1*time.Hour),
		)
		require.NoError(t, err)
		_, err = server.TLSServer.grpcServer.CreateToken(
			ctx, token.(*types.ProvisionTokenV2),
		)
		require.NoError(t, err)
		token, err = server.TLSServer.grpcServer.GetToken(
			ctx, &types.ResourceRequest{Name: tokenName},
		)
		require.NoError(t, err)
		require.Equal(t, tokenName, token.GetName())
		ttl := token.Expiry().Sub(server.Clock().Now())
		defaultTTL := defaults.ProvisioningTokenTTL
		require.LessOrEqual(t, ttl, defaultTTL)
		require.GreaterOrEqual(t, ttl, defaultTTL-(time.Second*10))
	})
}

// TestGRPCServer_UpsertToken tests the handler of the deprecated CreateToken
// RPC.
func TestGRPCServer_UpsertToken(t *testing.T) {
	ctx := context.Background()
	server := newTestTLSServer(t)

	// Allow us to directly invoke the deprecated gRPC methods with
	// authentication.
	user := TestAdmin()
	ctx = authz.ContextWithUser(ctx, user.I)

	// Test default expiry is applied.
	t.Run("undefined-expiry", func(t *testing.T) {
		tokenName := "undefined-expiry"
		roles := types.SystemRoles{types.RoleNode}
		token, err := types.NewProvisionToken(
			tokenName,
			roles,
			time.Time{},
		)
		require.NoError(t, err)
		_, err = server.TLSServer.grpcServer.UpsertToken(
			ctx, token.(*types.ProvisionTokenV2),
		)
		require.NoError(t, err)
		token, err = server.TLSServer.grpcServer.GetToken(
			ctx, &types.ResourceRequest{Name: tokenName},
		)
		require.NoError(t, err)
		require.Equal(t, tokenName, token.GetName())
		ttl := token.Expiry().Sub(server.Clock().Now())
		defaultTTL := defaults.ProvisioningTokenTTL
		require.LessOrEqual(t, ttl, defaultTTL)
		require.GreaterOrEqual(t, ttl, defaultTTL-(time.Second*10))
	})

	// Test set expiry is applied.
	t.Run("set-expiry", func(t *testing.T) {
		tokenName := "set-expiry"
		roles := types.SystemRoles{types.RoleNode}
		ttl := time.Hour * 24
		token, err := types.NewProvisionToken(
			tokenName,
			roles,
			server.Clock().Now().Add(ttl),
		)
		require.NoError(t, err)
		_, err = server.TLSServer.grpcServer.UpsertToken(
			ctx, token.(*types.ProvisionTokenV2),
		)
		require.NoError(t, err)
		token, err = server.TLSServer.grpcServer.GetToken(
			ctx, &types.ResourceRequest{Name: tokenName},
		)
		require.NoError(t, err)
		require.Equal(t, tokenName, token.GetName())
		actualTTL := token.Expiry().Sub(server.Clock().Now())
		require.LessOrEqual(t, actualTTL, ttl)
		require.GreaterOrEqual(t, actualTTL, ttl-(time.Second*10))
	})

	// Test expiry in past is changed to default
	t.Run("past-expiry", func(t *testing.T) {
		tokenName := "past-expiry"
		roles := types.SystemRoles{types.RoleNode}
		token, err := types.NewProvisionToken(
			tokenName,
			roles,
			server.Clock().Now().Add(-1*time.Hour),
		)
		require.NoError(t, err)
		_, err = server.TLSServer.grpcServer.UpsertToken(
			ctx, token.(*types.ProvisionTokenV2),
		)
		require.NoError(t, err)
		token, err = server.TLSServer.grpcServer.GetToken(
			ctx, &types.ResourceRequest{Name: tokenName},
		)
		require.NoError(t, err)
		require.Equal(t, tokenName, token.GetName())
		ttl := token.Expiry().Sub(server.Clock().Now())
		defaultTTL := defaults.ProvisioningTokenTTL
		require.LessOrEqual(t, ttl, defaultTTL)
		require.GreaterOrEqual(t, ttl, defaultTTL-(time.Second*10))
	})
}

// TestSAMLValidation tests that SAML validation does not perform an HTTP
// request if the calling user does not have permissions to create or update
// a SAML connector.
func TestSAMLValidation(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestFeatures: modules.Features{SAML: true},
	})

	// minimal entity_descriptor to pass validation. not actually valid
	const minimalEntityDescriptor = `
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="http://example.com">
  <md:IDPSSODescriptor>
    <md:SingleSignOnService Location="http://example.com" />
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`

	allowSAMLUpsert := types.RoleConditions{
		Rules: []types.Rule{{
			Resources: []string{types.KindSAML},
			Verbs:     []string{types.VerbCreate, types.VerbUpdate},
		}},
	}

	testCases := []struct {
		desc               string
		allow              types.RoleConditions
		entityDescriptor   string
		entityServerCalled bool
		assertErr          func(error) bool
	}{
		{
			desc:               "access denied",
			allow:              types.RoleConditions{},
			entityServerCalled: false,
			assertErr:          trace.IsAccessDenied,
		},
		{
			desc:               "validation failure",
			allow:              allowSAMLUpsert,
			entityDescriptor:   "", // validation fails with no issuer
			entityServerCalled: true,
			assertErr:          trace.IsBadParameter,
		},
		{
			desc:               "access permitted",
			allow:              allowSAMLUpsert,
			entityDescriptor:   minimalEntityDescriptor,
			entityServerCalled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			server := newTestTLSServer(t)
			// Create an http server to serve the entity descriptor url
			entityServerCalled := false
			entityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				entityServerCalled = true
				_, err := w.Write([]byte(tc.entityDescriptor))
				require.NoError(t, err)
			}))

			role, err := CreateRole(ctx, server.Auth(), "test_role", types.RoleSpecV6{Allow: tc.allow})
			require.NoError(t, err)
			user, err := CreateUser(server.Auth(), "test_user", role)
			require.NoError(t, err)

			connector, err := types.NewSAMLConnector("test_connector", types.SAMLConnectorSpecV2{
				AssertionConsumerService: "http://localhost:65535/acs", // not called
				EntityDescriptorURL:      entityServer.URL,
				AttributesToRoles: []types.AttributeMapping{
					// not used. can be any name, value but role must exist
					{Name: "groups", Value: "admin", Roles: []string{role.GetName()}},
				},
			})
			require.NoError(t, err)

			client, err := server.NewClient(TestUser(user.GetName()))
			require.NoError(t, err)

			err = client.UpsertSAMLConnector(ctx, connector)

			if tc.assertErr != nil {
				require.Error(t, err)
				require.True(t, tc.assertErr(err), "UpsertSAMLConnector error type mismatch. got: %T", trace.Unwrap(err))
			} else {
				require.NoError(t, err)
			}
			if tc.entityServerCalled {
				require.True(t, entityServerCalled, "entity_descriptor_url was not called")
			} else {
				require.False(t, entityServerCalled, "entity_descriptor_url was called")
			}
		})
	}
}

func newEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `
<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`

func TestGRPCServer_GetInstallers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	server := newTestTLSServer(t)
	grpc := server.TLSServer.grpcServer

	user := TestAdmin()
	ctx = authz.ContextWithUser(ctx, user.I)

	tests := []struct {
		name               string
		inputInstallers    map[string]string
		expectedInstallers map[string]string
	}{
		{
			name: "default installers only",
			expectedInstallers: map[string]string{
				installers.InstallerScriptName:          installers.DefaultInstaller.GetScript(),
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
		{
			name: "default and custom installers",
			inputInstallers: map[string]string{
				"my-custom-installer": "echo test",
			},
			expectedInstallers: map[string]string{
				"my-custom-installer":                   "echo test",
				installers.InstallerScriptName:          installers.DefaultInstaller.GetScript(),
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
		{
			name: "override default installer",
			inputInstallers: map[string]string{
				installers.InstallerScriptName: "echo test",
			},
			expectedInstallers: map[string]string{
				installers.InstallerScriptName:          "echo test",
				installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller.GetScript(),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				_, err := grpc.DeleteAllInstallers(ctx, &emptypb.Empty{})
				require.NoError(t, err)
			})

			for name, script := range tc.inputInstallers {
				installer, err := types.NewInstallerV1(name, script)
				require.NoError(t, err)
				_, err = grpc.SetInstaller(ctx, installer)
				require.NoError(t, err)
			}

			outputInstallerList, err := grpc.GetInstallers(ctx, &emptypb.Empty{})
			require.NoError(t, err)
			outputInstallers := make(map[string]string, len(tc.expectedInstallers))
			for _, installer := range outputInstallerList.Installers {
				outputInstallers[installer.GetName()] = installer.GetScript()
			}

			require.Equal(t, tc.expectedInstallers, outputInstallers)
		})
	}
}

func TestRoleVersions(t *testing.T) {
	t.Parallel()
	srv := newTestTLSServer(t)

	wildcardLabels := types.Labels{types.Wildcard: {types.Wildcard}}

	newRole := func(spec types.RoleSpecV6) types.Role {
		role, err := types.NewRole("test_rule", spec)
		require.NoError(t, err)
		return role
	}

	role := newRole(types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels:               wildcardLabels,
			AppLabels:                wildcardLabels,
			AppLabelsExpression:      `labels["env"] == "staging"`,
			DatabaseLabelsExpression: `labels["env"] == "staging"`,
			Rules: []types.Rule{
				types.NewRule(types.KindRole, services.RW()),
			},
		},
		Deny: types.RoleConditions{
			KubernetesLabels:               types.Labels{"env": {"prod"}},
			ClusterLabels:                  types.Labels{"env": {"prod"}},
			ClusterLabelsExpression:        `labels["env"] == "prod"`,
			WindowsDesktopLabelsExpression: `labels["env"] == "prod"`,
		},
	})

	user, err := CreateUser(srv.Auth(), "user", role)
	require.NoError(t, err)

	client, err := srv.NewClient(TestUser(user.GetName()))
	require.NoError(t, err)

	for _, tc := range []struct {
		desc             string
		clientVersions   []string
		expectError      bool
		expectedRole     types.Role
		expectDowngraded bool
	}{
		{
			desc: "up to date",
			clientVersions: []string{
				minSupportedLabelExpressionVersion.String(), "13.3.0", "14.0.0-alpha.1", "15.1.2", "",
			},
			expectedRole: role,
		},
		{
			desc:           "bad client versions",
			clientVersions: []string{"Not a version", "13", "13.1"},
			expectError:    true,
		},
		{
			desc:           "label expressions downgraded",
			clientVersions: []string{"13.0.11", "12.4.3", "6.0.0"},
			expectedRole: newRole(
				types.RoleSpecV6{
					Allow: types.RoleConditions{
						// None of the allow labels change
						NodeLabels:               wildcardLabels,
						AppLabels:                wildcardLabels,
						AppLabelsExpression:      `labels["env"] == "staging"`,
						DatabaseLabelsExpression: `labels["env"] == "staging"`,
						Rules: []types.Rule{
							types.NewRule(types.KindRole, services.RW()),
						},
					},
					Deny: types.RoleConditions{
						// These fields don't change
						KubernetesLabels:               types.Labels{"env": {"prod"}},
						ClusterLabelsExpression:        `labels["env"] == "prod"`,
						WindowsDesktopLabelsExpression: `labels["env"] == "prod"`,
						// These all get set to wildcard deny because there is
						// either an allow or deny label expression for them.
						AppLabels:            wildcardLabels,
						DatabaseLabels:       wildcardLabels,
						ClusterLabels:        wildcardLabels,
						WindowsDesktopLabels: wildcardLabels,
					},
				}),
			expectDowngraded: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			for _, clientVersion := range tc.clientVersions {
				t.Run(clientVersion, func(t *testing.T) {
					// setup client metadata
					ctx := context.Background()
					if clientVersion == "" {
						ctx = context.WithValue(ctx, metadata.DisableInterceptors{}, struct{}{})
					} else {
						ctx = metadata.AddMetadataToContext(ctx, map[string]string{
							metadata.VersionKey: clientVersion,
						})
					}

					checkRole := func(gotRole types.Role) {
						t.Helper()
						if tc.expectError {
							return
						}
						require.Empty(t, cmp.Diff(tc.expectedRole, gotRole,
							cmpopts.IgnoreFields(types.RoleV6{}, "Metadata.ID", "Metadata.Labels")))
						// The downgraded label value won't match exactly because it
						// includes the client version, so just check it's not empty
						// and ignore it in the role diff.
						if tc.expectDowngraded {
							require.NotEmpty(t, gotRole.GetMetadata().Labels[types.TeleportDowngradedLabel])
						}
					}
					checkErr := func(err error) {
						t.Helper()
						if tc.expectError {
							require.Error(t, err)
						} else {
							require.NoError(t, err)
						}
					}

					// Test GetRole
					gotRole, err := client.GetRole(ctx, role.GetName())
					checkErr(err)
					checkRole(gotRole)

					// Test GetRoles
					gotRoles, err := client.GetRoles(ctx)
					checkErr(err)
					if !tc.expectError {
						foundTestRole := false
						for _, gotRole := range gotRoles {
							if gotRole.GetName() != role.GetName() {
								continue
							}
							checkRole(gotRole)
							foundTestRole = true
							break
						}
						require.True(t, foundTestRole, "GetRoles result does not include expected role")
					}

					ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()

					// Test WatchEvents
					watcher, err := client.NewWatcher(ctx, types.Watch{Name: "roles", Kinds: []types.WatchKind{{Kind: types.KindRole}}})
					require.NoError(t, err)
					defer watcher.Close()

					// Swallow the init event
					e := <-watcher.Events()
					require.Equal(t, types.OpInit, e.Type)

					// Re-upsert the role so that the watcher sees it, do this
					// on the auth server directly to avoid the
					// TeleportDowngradedLabel check in ServerWithRoles
					require.NoError(t, srv.Auth().UpsertRole(ctx, role))

					gotRole, err = func() (types.Role, error) {
						for {
							select {
							case <-watcher.Done():
								return nil, watcher.Error()
							case e := <-watcher.Events():
								if gotRole, ok := e.Resource.(types.Role); ok && gotRole.GetName() == role.GetName() {
									return gotRole, nil
								}
							}
						}
					}()
					checkErr(err)
					checkRole(gotRole)

					if !tc.expectError {
						// Try to re-upsert the role we got. If it was
						// downgraded, it should be rejected due to the
						// TeleportDowngradedLabel
						err = client.UpsertRole(ctx, gotRole)
						if tc.expectDowngraded {
							require.Error(t, err)
						} else {
							require.NoError(t, err)
						}
					}
				})
			}
		})
	}
}
